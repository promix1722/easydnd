package character

// The import coordinator owns one isolated draft per session. External model
// calls never hold its mutex; a generation check fences their late results.
import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

type AgentFile struct {
	Name string `json:"name"`
	MIME string `json:"mime"`
	Data []byte `json:"-"`
}
type AgentEvent struct {
	Actions    []string        `json:"actions,omitempty"`
	Files      []AgentFile     `json:"files,omitempty"`
	Options    []string        `json:"options,omitempty"`
	Source     string          `json:"source,omitempty"`
	Assumption string          `json:"assumption,omitempty"`
	ID         int             `json:"id"`
	Kind       string          `json:"kind"`
	Text       string          `json:"text,omitempty"`
	Tool       string          `json:"tool,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"`
}
type AgentManual struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
}
type agentOperation struct {
	Hash   [32]byte
	Result json.RawMessage
	Error  bool
}

type AgentSession struct {
	expected []string
	// nudged is the checklist entries review was last refused over. The same
	// list a second time means the model has seen it and is done.
	nudged []string
	// printed is the sheet's derived numbers -- hit points, armor class, skill
	// and save bonuses -- by fact path, as a model read them. They are what the
	// build is checked against and never part of the character: a number read
	// off a scan is the least reliable thing in an import, and pinning one
	// over a build that computes otherwise makes the sheet wrong with authority.
	printed map[string]int
	// spells is the cantrips and spells the sheet lists, by printed name.
	spells []string
	// scores is the six totals the sheet prints. They are never written to the
	// character: after every tool call the base scores are solved again so the
	// build comes out at them, whatever order race and improvements arrive in.
	// A player editing the character ends that -- their numbers win.
	scores map[rules.Ability]int
	// classLevels is the level the sheet prints beside each class.
	classLevels map[rules.Slug]int
	// characterRevision is the stored character's revision as the agent last
	// read or wrote it. Any other value means the player edited it meanwhile.
	characterRevision int
	operations        map[string]agentOperation
	ID                string            `json:"id"`
	Folder            domain.FolderID   `json:"folder"`
	Status            string            `json:"status"`
	Revision          int               `json:"revision"`
	CharacterID       domain.ID         `json:"characterId,omitempty"`
	Events            []AgentEvent      `json:"events"`
	Files             []AgentFile       `json:"files"`
	Manual            []AgentManual     `json:"manual"`
	Assumptions       []string          `json:"assumptions"`
	Owner             domain.OwnerID    `json:"-"`
	Locale            rules.Locale      `json:"-"`
	Log               domain.Log        `json:"-"`
	Generation        int               `json:"-"`
	Input             []json.RawMessage `json:"-"`
	Turns             int               `json:"-"`
	busy              bool
	cancel            context.CancelFunc
}
type AgentCall struct{ ID, Name, Arguments string }
type AgentResponse struct {
	Text   string
	Output []json.RawMessage
	Calls  []AgentCall
}
type AgentRequest struct {
	Input  []json.RawMessage
	Files  []AgentFile
	Locale string
}
type AgentModel interface {
	Respond(context.Context, AgentRequest, func(string)) (AgentResponse, error)
}
type AgentConfig struct {
	Workers, MaxTurns, MaxSessions int
	Timeout                        time.Duration
}
type Agent struct {
	mu       sync.Mutex
	sessions map[string]*AgentSession
	service  *Service
	model    AgentModel
	config   AgentConfig
	wake     chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewAgent(service *Service, model AgentModel, cfg AgentConfig) *Agent {
	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}
	if cfg.MaxTurns <= 0 {
		cfg.MaxTurns = 40
	}
	if cfg.MaxSessions <= 0 {
		cfg.MaxSessions = 100
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := &Agent{sessions: map[string]*AgentSession{}, service: service, model: model, config: cfg, wake: make(chan struct{}, 1), ctx: ctx, cancel: cancel}
	for i := 0; i < cfg.Workers; i++ {
		a.wg.Add(1)
		go a.worker()
	}
	return a
}
func (a *Agent) Close()         { a.cancel(); a.wg.Wait() }
func (a *Agent) Enabled() bool  { return a.model != nil }
func raw(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
func (a *Agent) signal() {
	select {
	case a.wake <- struct{}{}:
	default:
	}
}
func addAgentEvent(s *AgentSession, kind, text, tool string, data any) {
	e := AgentEvent{ID: len(s.Events) + 1, Kind: kind, Text: text, Tool: tool}
	if data != nil {
		e.Data = raw(data)
	}
	s.Events = append(s.Events, e)
}
func (a *Agent) owned(owner domain.OwnerID, id string) (*AgentSession, error) {
	s := a.sessions[id]
	if s == nil || s.Owner != owner {
		return nil, types.NewNotFoundError("import session not found").Because("agent.notFound")
	}
	return s, nil
}
func copyAgentSession(s *AgentSession) AgentSession {
	out := *s
	out.Events = append([]AgentEvent{}, s.Events...)
	for i := range out.Events {
		out.Events[i].Data = append(json.RawMessage(nil), out.Events[i].Data...)
		out.Events[i].Files = agentFileLabels(out.Events[i].Files)
		out.Events[i].Actions = append([]string(nil), out.Events[i].Actions...)
		out.Events[i].Options = append([]string(nil), out.Events[i].Options...)
	}
	out.Files = append([]AgentFile{}, s.Files...)
	for i := range out.Files {
		out.Files[i].Data = nil
	}
	out.expected = append([]string(nil), s.expected...)
	out.Manual = append([]AgentManual{}, s.Manual...)
	out.Assumptions = append([]string{}, s.Assumptions...)
	out.Log = s.Log.Clone()
	out.Input = nil
	out.operations = nil
	out.cancel = nil
	return out
}
func (a *Agent) Create(ctx context.Context, owner domain.OwnerID, folder domain.FolderID, locale rules.Locale, files []AgentFile, instructions string, selected ...pack.Lock) (AgentSession, error) {
	if a.model == nil {
		return AgentSession{}, types.NewNotImplementedError("agent is not configured").Because("agent.disabled")
	}
	if (len(files) == 0 && strings.TrimSpace(instructions) == "") || len(files) > 8 || len(instructions) > 16000 {
		return AgentSession{}, types.NewValidationError("invalid import input").Because("agent.files")
	}
	total := 0
	names := map[string]bool{}
	for _, f := range files {
		total += len(f.Data)
		if len(f.Data) == 0 || len(f.Name) > 200 || names[f.Name] {
			return AgentSession{}, types.NewValidationError("invalid attachment").Because("agent.files")
		}
		names[f.Name] = true
	}
	if total > 20<<20 {
		return AgentSession{}, types.NewValidationError("attachments too large").Because("agent.files")
	}
	folder, err := a.service.resolveFolder(ctx, owner, folder)
	if err != nil {
		return AgentSession{}, err
	}
	var cat *catalog.Catalog
	if len(selected) > 0 && !selected[0].IsZero() {
		if a.service.packAccess == nil {
			return AgentSession{}, types.NewAccessDeniedError("pack selection unavailable")
		}
		if err := a.service.packAccess.AuthorizeLock(ctx, user.ID(owner), selected[0], pack.Lock{}); err != nil {
			return AgentSession{}, err
		}
		cat, err = catalog.LoadLocked(ctx, a.service.catalog, locale, selected[0])
	} else {
		cat, err = a.service.catalog.Load(ctx, locale)
	}
	if err != nil {
		return AgentSession{}, err
	}
	var token [16]byte
	if _, err = rand.Read(token[:]); err != nil {
		return AgentSession{}, err
	}
	ownedFiles := append([]AgentFile(nil), files...)
	for i := range ownedFiles {
		ownedFiles[i].Data = append([]byte(nil), files[i].Data...)
	}
	s := &AgentSession{ID: hex.EncodeToString(token[:]), Owner: owner, Folder: folder, Locale: locale, Status: "queued", Revision: 1, Files: ownedFiles, Events: []AgentEvent{}, Manual: []AgentManual{}, Assumptions: []string{}}
	// The same entries the builder writes for a new character: its name, then
	// the rules it is built under. The wizard's character is an ordinary one
	// from its first message, so it opens in the ordinary builder.
	e := initEvent(NewCharacter{Name: "…"})
	e.RulesLock = cat.Lock.Clone()
	log := domain.Log{}
	_ = log.Append(e,
		domain.Event{Type: domain.EventChange, Source: domain.GroupIdentity, Changes: []domain.Change{{Path: "identity.ruleset", Op: domain.OpSet, Value: domain.SlugValue(rules.Slug(cat.Ruleset))}}},
		domain.Event{Type: domain.EventNote, Note: "import.session:" + s.ID})
	s.Input = append(s.Input, raw(map[string]any{"role": "user", "content": instructions + "\nCreate this single character from the description and any attached sources, using the selected rules lock. Read the build context first."}))
	addAgentEvent(s, "user", instructions, "", nil)
	s.Events[len(s.Events)-1].Files = agentFileLabels(files)
	// The repository can commit the initial log atomically. Do not fall back to
	// Create + Commit, which leaves an empty character after a failed write.
	repo, ok := a.service.repo.(interface {
		CreateWithLog(context.Context, domain.OwnerID, domain.FolderID, domain.Log) (domain.Character, error)
	})
	if !ok {
		return AgentSession{}, types.NewNotImplementedError("atomic character creation unavailable")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.sessions) >= a.config.MaxSessions {
		return AgentSession{}, types.NewValidationError("session capacity reached").Because("agent.capacity")
	}
	created, err := repo.CreateWithLog(ctx, owner, folder, log)
	if err != nil {
		return AgentSession{}, err
	}
	s.CharacterID, s.Log, s.characterRevision = created.ID, created.Log.Clone(), created.Revision
	a.sessions[s.ID] = s
	a.signal()
	return copyAgentSession(s), nil
}

// pull re-reads the character, and reports whether the player has changed it
// since the agent last did. The character is theirs and open in the builder
// the whole time; the session's log is only the agent's working copy of it.
func (a *Agent) pull(ctx context.Context, s *AgentSession) (bool, error) {
	c, err := a.service.repo.Get(ctx, s.CharacterID)
	if err != nil {
		return false, err
	}
	if c.Revision == s.characterRevision {
		return false, nil
	}
	s.Log, s.characterRevision = c.Log.Clone(), c.Revision
	return true, nil
}

// push writes the working copy back, refusing if the player got there first.
func (a *Agent) push(ctx context.Context, s *AgentSession) error {
	if err := a.service.repo.Commit(ctx, s.CharacterID, s.characterRevision, s.Log, "", nil); err != nil {
		return err
	}
	c, err := a.service.repo.Get(ctx, s.CharacterID)
	if err != nil {
		return err
	}
	s.Log, s.characterRevision = c.Log.Clone(), c.Revision
	return nil
}

// call runs one tool against the stored character and stores what it changed.
func (a *Agent) call(ctx context.Context, s *AgentSession, name string, arguments []byte) (any, error) {
	edited, err := a.pull(ctx, s)
	if types.IsNotFound(err) {
		s.Status = "failed"
		addAgentEvent(s, "status", "failed", "", nil)
	}
	if err != nil {
		return nil, err
	}
	if edited {
		s.scores = nil
		addAgentEvent(s, "edit", "", "", nil)
		if name != "get_build_context" {
			return nil, fmt.Errorf("not run: the player edited the character in the builder. Call get_build_context, treat what it shows as authoritative and preserve their changes, then send this again if it is still needed")
		}
	}
	before := s.Log.Clone()
	result, err := a.tool(ctx, s, name, arguments)
	if cat, catErr := a.Catalog(ctx, *s); catErr == nil {
		a.settleScores(s, cat)
	}
	if !reflect.DeepEqual(before, s.Log) {
		if pushErr := a.push(ctx, s); pushErr != nil {
			s.Log = before
			return nil, pushErr
		}
	}
	return result, err
}
func (a *Agent) Get(owner domain.OwnerID, id string) (AgentSession, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	s, err := a.owned(owner, id)
	if err != nil {
		return AgentSession{}, err
	}
	return copyAgentSession(s), nil
}
func (a *Agent) List(owner domain.OwnerID) []AgentSession {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := []AgentSession{}
	for _, s := range a.sessions {
		if s.Owner == owner {
			v := copyAgentSession(s)
			v.Events = nil
			v.Files = nil
			v.Input = nil
			out = append(out, v)
		}
	}
	return out
}
func (a *Agent) invalidate(s *AgentSession) {
	s.Generation++
	if s.cancel != nil {
		s.cancel()
	}
	s.Revision++
}
func (a *Agent) Control(owner domain.OwnerID, id, action, text string, revision int) (AgentSession, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	s, err := a.owned(owner, id)
	if err != nil {
		return AgentSession{}, err
	}
	if s.Revision != revision {
		return AgentSession{}, types.NewValidationError("session changed").Because("agent.changed")
	}
	switch action {
	case "stop":
		a.invalidate(s)
		s.Status = "paused"
		addAgentEvent(s, "status", "paused", "", nil)
	case "resume", "retry":
		if len(s.Input) > 500 || transcriptSize(s.Input) > 2<<20 {
			return AgentSession{}, types.NewValidationError("session conversation limit reached").Because("agent.limit")
		}
		a.invalidate(s)
		s.Status = "queued"
		s.Turns = 0
		a.signal()
	case "message":
		if strings.TrimSpace(text) == "" || len(text) > 16000 || len(s.Input) > 500 || transcriptSize(s.Input) > 2<<20 {
			return AgentSession{}, types.NewValidationError("invalid message")
		}
		a.invalidate(s)
		s.Input = append(s.Input, raw(map[string]any{"role": "user", "content": text}))
		addAgentEvent(s, "user", text, "", nil)
		s.Status = "queued"
		s.Turns = 0
		a.signal()
	case "discard":
		// Discarding the chat discards what it made. Leaving the chat any other
		// way leaves the character where it is.
		if err := a.service.Delete(a.ctx, owner, s.CharacterID); err != nil && !types.IsNotFound(err) {
			return AgentSession{}, err
		}
		a.invalidate(s)
		delete(a.sessions, id)
	default:
		return AgentSession{}, types.NewValidationError("unknown session control")
	}
	return copyAgentSession(s), nil
}
func (a *Agent) Catalog(ctx context.Context, s AgentSession) (*catalog.Catalog, error) {
	cat, err := catalog.LoadLocked(ctx, a.service.catalog, s.Locale, s.Log.RulesLock())
	return domain.WithCustomCatalog(s.Log, cat), err
}
func (a *Agent) Sheet(ctx context.Context, s AgentSession) (domain.State, error) {
	cat, err := a.Catalog(ctx, s)
	if err != nil {
		return domain.State{}, err
	}
	return domain.Project(s.Log, cat)
}
func (a *Agent) worker() {
	defer a.wg.Done()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-a.wake:
		}
		for {
			if a.ctx.Err() != nil {
				return
			}
			a.mu.Lock()
			var s *AgentSession
			for _, v := range a.sessions {
				if v.Status == "queued" && !v.busy {
					s = v
					break
				}
			}
			if s == nil {
				a.mu.Unlock()
				break
			}
			s.busy = true
			s.Status = "running"
			s.Revision++
			gen := s.Generation
			ctx, cancel := context.WithTimeout(a.ctx, a.config.Timeout)
			s.cancel = cancel
			snap := copyAgentSession(s)
			snap.Input = append([]json.RawMessage(nil), s.Input...)
			snap.Files = append([]AgentFile(nil), s.Files...)
			a.mu.Unlock()
			a.signal()
			a.run(ctx, &snap, gen)
			cancel()
			a.mu.Lock()
			if current := a.sessions[snap.ID]; current != nil {
				current.busy = false
				current.cancel = nil
				if current.Status == "queued" {
					a.signal()
				}
			}
			a.mu.Unlock()
		}
	}
}
func (a *Agent) run(ctx context.Context, snap *AgentSession, gen int) {
	response, err := a.model.Respond(ctx, AgentRequest{Input: snap.Input, Files: snap.Files, Locale: snap.Locale.String()}, func(delta string) {
		a.mu.Lock()
		defer a.mu.Unlock()
		s := a.sessions[snap.ID]
		if s != nil && s.Generation == gen && len(s.Events) < 10000 {
			addAgentEvent(s, "delta", delta, "", nil)
		}
	})
	a.mu.Lock()
	defer a.mu.Unlock()
	s := a.sessions[snap.ID]
	if s == nil || s.Generation != gen {
		return
	}
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		s.Status = "failed"
		s.Revision++
		addAgentEvent(s, "status", "failed", "", nil)
		return
	}
	// Commit a complete response before executing any tool. Partial streamed
	// arguments are never interpreted. The transcript is the operation ledger.
	s.Input = append(s.Input, response.Output...)
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
				a.service.log.Warn("AI wizard tool rejected", "tool", call.Name, "error", err)
				op.Error = true
				op.Result = agentError(err)
			}
			// The only record of what a model actually asked for. Debug, because
			// the arguments are a player's character sheet.
			a.service.log.Debug("AI wizard tool call", "session", s.ID, "tool", call.Name, "arguments", clip(call.Arguments), "result", clip(string(op.Result)))
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
	}

	s.Revision++
	if s.Status == "running" {
		if len(response.Calls) == 0 {
			s.Status = "waiting"
		} else if s.Turns >= a.config.MaxTurns || len(s.Input) > 500 || len(s.Events) > 10000 || transcriptSize(s.Input) > 2<<20 {
			s.Status = "paused"
			addAgentEvent(s, "status", "budget", "", nil)
		} else {
			s.Status = "queued"
			a.signal()
		}
	}
}

func clip(text string) string {
	if len(text) > 6000 {
		return text[:6000] + "…"
	}
	return text
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

func transcriptSize(input []json.RawMessage) int {
	n := 0
	for _, v := range input {
		n += len(v)
	}
	return n
}

// AddFiles extends the same character's source set and fences an in-flight
// response, just like a correction. Files remain private to this session.
func (a *Agent) AddFiles(owner domain.OwnerID, id string, revision int, files []AgentFile, text string) (AgentSession, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	s, err := a.owned(owner, id)
	if err != nil {
		return AgentSession{}, err
	}
	if s.Revision != revision {
		return AgentSession{}, types.NewValidationError("session changed").Because("agent.changed")
	}
	if len(files) == 0 || len(files)+len(s.Files) > 8 || len(text) > 16000 || len(s.Input) > 500 {
		return AgentSession{}, types.NewValidationError("invalid attachments").Because("agent.files")
	}
	total := 0
	names := map[string]bool{}
	for _, f := range s.Files {
		total += len(f.Data)
		names[f.Name] = true
	}
	for _, f := range files {
		total += len(f.Data)
		if len(f.Data) == 0 || len(f.Name) > 200 || names[f.Name] {
			return AgentSession{}, types.NewValidationError("empty or duplicate attachment").Because("agent.files")
		}
		names[f.Name] = true
	}
	if total > 20<<20 {
		return AgentSession{}, types.NewValidationError("attachments too large").Because("agent.files")
	}
	for _, f := range files {
		f.Data = append([]byte(nil), f.Data...)
		s.Files = append(s.Files, f)
	}
	a.invalidate(s)
	s.Status = "queued"
	s.Turns = 0
	s.Input = append(s.Input, raw(map[string]any{"role": "user", "content": "I added more source files for the same character. Read the full source set and preserve my edits.\n" + text}))
	addAgentEvent(s, "user", text, "", nil)
	s.Events[len(s.Events)-1].Files = agentFileLabels(files)
	a.signal()
	return copyAgentSession(s), nil
}

// Attachments belong to the user message which submitted them, never to a
// separate transcript row. Source bytes remain private to the provider input.
func agentFileLabels(files []AgentFile) []AgentFile {
	out := make([]AgentFile, len(files))
	for i, f := range files {
		out[i] = AgentFile{Name: f.Name, MIME: f.MIME}
	}
	return out
}
