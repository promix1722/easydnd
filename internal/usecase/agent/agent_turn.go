package agent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/types"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

// pull re-reads the character, and reports whether the player has changed it
// since the agent last did. The character is theirs and open in the builder
// the whole time; the session's log is only the agent's working copy of it.
func (a *Agent) pull(ctx context.Context, s *AgentSession) (bool, error) {
	c, err := a.service.Repository().Get(ctx, s.CharacterID)
	if err != nil {
		return false, err
	}
	if !a.made(s, c) {
		return false, types.NewNotFoundError("character not found")
	}
	if c.Revision == s.characterRevision {
		return false, nil
	}
	s.Log, s.characterRevision = c.Log.Clone(), c.Revision
	return true, nil
}

// made reports whether c is a character this session may write to: its
// owner's. The id a session holds is durable, as the session is, so the
// check is ownership and nothing more.
func (a *Agent) made(s *AgentSession, c domain.Character) bool {
	return c.Owner == s.Owner
}

// push writes the working copy back, refusing if the player got there first.
func (a *Agent) push(ctx context.Context, s *AgentSession) error {
	if err := a.service.Repository().Commit(ctx, s.CharacterID, s.characterRevision, s.Log); err != nil {
		return err
	}
	c, err := a.service.Repository().Get(ctx, s.CharacterID)
	if err != nil {
		return err
	}
	s.Log, s.characterRevision = c.Log.Clone(), c.Revision
	return nil
}

// call runs one tool against the stored character and stores what it changed.
func (a *Agent) call(ctx context.Context, s *AgentSession, name string, arguments []byte) (any, error) {
	held := s.Log
	edited, err := a.pull(ctx, s)
	if types.IsNotFound(err) {
		_ = setStatus(s, "failed")
		addAgentEvent(s, "status", "failed", "", nil)
	}
	if err != nil {
		return nil, err
	}
	if edited {
		// The owner's numbers win over the sheet's -- when they changed a
		// number. A rename is no reason to forget the printed totals: without
		// them the next race or improvement lands on top of bases that were
		// solved to already include it.
		if cat, catErr := a.Catalog(ctx, *s); catErr != nil || !sameScores(held, s.Log, cat) {
			s.scores = nil
		}
		addAgentEvent(s, "edit", "", "", nil)
		if name != "get_build_context" {
			return nil, fmt.Errorf("not run: the player edited the character in the builder. Call get_build_context, treat what it shows as authoritative and preserve their changes, then send this again if it is still needed")
		}
	}
	before := s.Log.Clone()
	result, err := a.tool(ctx, s, name, arguments)
	cat, catErr := a.Catalog(ctx, *s)
	if catErr == nil {
		a.settleScores(s, cat)
	}
	if !reflect.DeepEqual(before, s.Log) {
		// The limits Service.Apply holds a log to, which these tools write
		// around. The refusal goes to the model like any other tool error.
		if catErr == nil {
			if limitErr := charuc.CheckSheet(before, s.Log, cat, a.service.Limits()); limitErr != nil {
				s.Log = before
				return nil, limitErr
			}
		}
		if pushErr := a.push(ctx, s); pushErr != nil {
			s.Log = before
			return nil, pushErr
		}
	}
	return result, err
}

func (a *Agent) worker() {
	defer a.wg.Done()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-a.wake:
		}
		for a.ctx.Err() == nil {
			// A turn this process is still unwinding is not taken again: its
			// late tool call must not run beside the next turn's.
			a.local.Lock()
			busy := slices.Collect(maps.Keys(a.running))
			a.local.Unlock()
			rec, ok, err := a.store.Claim(a.ctx, a.instance, a.config.Timeout+leaseMargin, busy)
			if err != nil && a.ctx.Err() == nil {
				a.service.Logger().Warn("AI wizard claim failed", "error", err)
			}
			if err != nil || !ok {
				break
			}
			ctx, cancel := context.WithTimeout(a.ctx, a.config.Timeout)
			a.local.Lock()
			a.running[rec.ID] = cancel
			a.local.Unlock()
			a.signal()
			a.turn(ctx, cancel, rec)
			cancel()
			a.local.Lock()
			delete(a.running, rec.ID)
			a.local.Unlock()
			a.signal()
		}
	}
}

// turn is one claimed turn. The session it works on is this goroutine's own:
// the lease is what keeps every other writer out, so nothing here is locked.
func (a *Agent) turn(ctx context.Context, cancel context.CancelFunc, rec Record) {
	s, err := decode(rec)
	if err == nil {
		s.Files, err = a.store.Files(ctx, s.ID)
	}
	if err != nil {
		a.service.Logger().Error("AI wizard session unreadable", "session", rec.ID, "error", err)
		return
	}
	// save stores the session as it stands and reports whether the turn is
	// still this process's to continue.
	save := func(pending []AgentCall, last bool) bool {
		// A turn that ran out of time, or a process that is stopping, still
		// has to say how it ended.
		saving, done := context.WithTimeout(context.WithoutCancel(ctx), storeTimeout)
		defer done()
		// ponytail: the whole document is rewritten after every tool call (up
		// to 2 MiB of transcript); store Input as rows if it shows.
		rec := encode(s, pending)
		if !last {
			// A question or a review ends the turn in the middle of a batch,
			// and the status goes with the turn's last save, not this one: a
			// session stored as no longer running has given up its lease, and
			// the last save -- the one that moves the revision the page is
			// waiting on -- would be refused.
			rec.Status = "running"
		}
		ok, err := a.store.Save(saving, rec, a.instance)
		if err != nil {
			a.service.Logger().Error("AI wizard save failed", "session", s.ID, "error", err)
		}
		if err != nil || !ok {
			cancel()
			return false
		}
		s.count = len(s.Events)
		return true
	}
	// Before the model is asked anything: a chat whose character has been
	// deleted has nothing to build, and a request would be spent finding
	// that out.
	if c, err := a.service.Repository().Get(ctx, s.CharacterID); err != nil && ctx.Err() == nil || err == nil && !a.made(s, c) {
		a.service.Logger().Warn("AI wizard session has no character of its own", "session", s.ID, "character", s.CharacterID, "error", err)
		_ = setStatus(s, "failed")
		s.Revision++
		addAgentEvent(s, "status", "failed", "", nil)
		save(nil, true)
		return
	}
	held := true
	response, err := a.model.Respond(ctx, AgentRequest{Input: s.Input, Files: s.Files, Locale: s.Locale.String(), Session: s.ID, Unattended: s.Unattended}, func(delta string) {
		if !held || len(s.Events) >= 10000 {
			return
		}
		e := AgentEvent{ID: len(s.Events) + 1, Kind: "delta", Text: delta}
		if ok, err := a.store.Append(ctx, s.ID, a.instance, s.Generation, e); err != nil || !ok {
			// Stopped, answered or taken over while the model was typing.
			held = false
			cancel()
			return
		}
		s.Events = append(s.Events, e)
		s.count = len(s.Events)
	})
	if !held {
		return
	}
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		if a.ctx.Err() != nil {
			// The process is stopping. The turn is not lost with it: back in
			// the queue, it is the next process's, or this one's restarted.
			_ = setStatus(s, "queued")
		} else {
			// The owner is told only that it failed; why is here.
			a.service.Logger().Error("AI wizard model request failed", "session", s.ID, "error", err)
			_ = setStatus(s, "failed")
			s.Revision++
			addAgentEvent(s, "status", "failed", "", nil)
		}
		save(nil, true)
		return
	}
	// Commit a complete response before executing any tool. Partial streamed
	// arguments are never interpreted. The transcript is the operation ledger.
	s.Input = append(s.Input, response.Output...)
	a.service.Logger().Info("AI wizard model request", "session", s.ID, "inputTokens", response.Usage.Input, "cachedTokens", response.Usage.Cached, "outputTokens", response.Usage.Output)
	addAgentEvent(s, "response", response.Text, "", nil)
	s.Turns++
	if s.operations == nil {
		s.operations = map[string]agentOperation{}
	}
	for i, call := range response.Calls {
		hash := sha256.Sum256([]byte(call.Name + "\x00" + call.Arguments))
		op, known := s.operations[call.ID]
		if known && op.Hash != hash {
			op = agentOperation{Hash: hash, Error: true, Result: raw(map[string]string{"error": "operation id reused with different arguments"})}
		} else if !known {
			var result any
			// Every call still gets an output -- the provider rejects a
			// transcript with an unanswered call -- but a response is one
			// bounded batch, and nothing runs after the call that ended the turn.
			switch {
			case ctx.Err() != nil:
				err = errInterrupted
			case i >= 32:
				err = fmt.Errorf("too many tool calls in one response; send the rest again")
			case s.Status != "running":
				err = fmt.Errorf("not run: the turn had already ended with a question or a review; send it again if it is still needed")
			case len(call.Arguments) > 128<<10:
				err = fmt.Errorf("tool arguments too large")
			default:
				result, err = a.call(ctx, s, call.Name, []byte(call.Arguments))
			}
			op = agentOperation{Hash: hash, Result: raw(result)}
			if err != nil {
				a.service.Logger().Warn("AI wizard tool rejected", "tool", call.Name, "error", err)
				op.Error = true
				op.Result = agentError(err)
			}
			// The only record of what a model actually asked for. Debug, because
			// the arguments are a player's character sheet.
			a.service.Logger().Debug("AI wizard tool call", "session", s.ID, "tool", call.Name, "arguments", clip(call.Arguments), "result", clip(string(op.Result)))
			s.operations[call.ID] = op
		}
		s.Input = append(s.Input, raw(map[string]any{"type": "function_call_output", "call_id": call.ID, "output": string(op.Result)}))
		var args agentArgs
		_ = json.Unmarshal([]byte(call.Arguments), &args)
		summary := args.Query
		if summary == "" {
			summary = args.Path
		}
		if summary == "" {
			summary = args.Ref
		}
		if summary == "" {
			summary = args.Name
		}
		if op.Error {
			summary = "invalid"
		}
		addAgentEvent(s, "tool", summary, call.Name, nil)
		if op.Error {
			s.Events[len(s.Events)-1].Data = op.Result
		}
		// Batches and answers publish their own progress, one line per write.
		if tool := agentToolName(call.Name, args); !op.Error && !known && (tool == "resolve_import_facts" || tool == "upsert_custom_option" || tool == "revise_choice") {
			a.recordProgress(ctx, s, tool, args, op.Result)
		}
		s.Events[len(s.Events)-1].Source = args.Source
		s.Events[len(s.Events)-1].Assumption = args.Assumption
		// Each call's writes are stored, and so shown, as it finishes, not
		// when the batch does.
		if !save(response.Calls[i+1:], false) {
			return
		}
	}

	s.Revision++
	if s.Status == "running" {
		if len(response.Calls) == 0 {
			// The request demands a tool call, so this is a response cut short.
			// It is not a question: waiting would leave the owner with nothing
			// to answer. Paused has a Resume button.
			_ = setStatus(s, "paused")
			addAgentEvent(s, "status", "paused", "", nil)
		} else if s.Turns >= a.config.MaxTurns || len(s.Input) > 500 || len(s.Events) > 10000 || transcriptSize(s.Input) > 2<<20 {
			_ = setStatus(s, "paused")
			addAgentEvent(s, "status", "budget", "", nil)
		} else {
			_ = setStatus(s, "queued")
		}
	}
	save(nil, true)
}

// agentError is what a rejected call tells the model: the message, and
// whatever would make its next attempt different -- the fields a validator
// named, the entries a name could have meant. A bare "some answers are not
// valid" is how a model ends up preserving the content as custom instead.
func agentError(err error) json.RawMessage {
	out := map[string]any{"error": err.Error()}
	var invalid *types.FieldValidationError
	if errors.As(err, &invalid) {
		fields := []map[string]any{}
		for _, f := range invalid.Fields {
			fields = append(fields, map[string]any{"field": f.Field, "rule": f.Rule, "reason": f.Reason, "args": f.Args})
		}
		out["fields"] = fields
	}
	var unsettled *candidatesError
	if errors.As(err, &unsettled) {
		out["candidates"] = unsettled.Candidates
	}
	return raw(out)
}
