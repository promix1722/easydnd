package character

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

type agentArgs struct {
	AllowIncomplete bool            `json:"allow_incomplete"`
	Options         []string        `json:"options"`
	Offset          int             `json:"offset"`
	Definition      string          `json:"definition"`
	Kind            string          `json:"kind"`
	Query           string          `json:"query"`
	Level           *int            `json:"level"`
	Ref             string          `json:"ref"`
	Prompt          string          `json:"prompt"`
	Picks           []string        `json:"picks"`
	Sequence        int             `json:"sequence"`
	Path            string          `json:"path"`
	Value           json.RawMessage `json:"value"`
	Assumption      string          `json:"assumption"`
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	Source          string          `json:"source"`
	Text            string          `json:"text"`
}

func agentValue(b json.RawMessage) (domain.Value, error) {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return domain.Value{}, err
	}
	switch x := v.(type) {
	case string:
		return domain.StringValue(x), nil
	case float64:
		if x != float64(int(x)) {
			break
		}
		return domain.IntValue(int(x)), nil
	case bool:
		return domain.BoolValue(x), nil
	case []any:
		out := []rules.Slug{}
		for _, v := range x {
			s, ok := v.(string)
			if !ok {
				return domain.Value{}, fmt.Errorf("expected string list")
			}
			out = append(out, rules.Slug(s))
		}
		return domain.SlugListValue(out), nil
	}
	return domain.Value{}, fmt.Errorf("unsupported value")
}
func (a *Agent) tool(ctx context.Context, s *AgentSession, name string, b []byte) (any, error) {
	var args agentArgs
	if err := json.Unmarshal(b, &args); err != nil {
		return nil, err
	}
	cat, err := a.Catalog(ctx, *s)
	if err != nil {
		return nil, err
	}
	switch name {
	case "get_build_context", "reconcile_import":
		state, err := domain.Project(s.Log, cat)
		if err != nil {
			return nil, err
		}
		prompts, err := domain.Prompts(s.Log, cat)
		return map[string]any{"sheet": state, "prompts": compactAgentPrompts(prompts), "history": s.Log, "manual": s.Manual, "assumptions": s.Assumptions, "rules": cat.Lock}, err
	case "read_source":
		for _, f := range s.Files {
			if f.Name == args.Name {
				if f.MIME == "application/json" || f.MIME == "text/plain" {
					return string(f.Data), nil
				}
				return map[string]string{"attachment": f.Name, "hint": "Read the image or PDF already attached to the conversation."}, nil
			}
		}
		return nil, fmt.Errorf("source not found")
	case "search_catalog":
		return a.search(ctx, s, args.Kind, args.Query, args.Level)
	case "get_option_details":
		ref, ok := rules.ParseRef(args.Ref)
		if !ok {
			return nil, fmt.Errorf("invalid ref")
		}
		for _, c := range catalogCandidates(cat, ref.Kind.String()) {
			if c.Ref == ref.Canonical() {
				if source, ok := a.service.catalog.(interface {
					CustomExample(context.Context, pack.Lock, rules.Ref) (any, error)
				}); ok {
					example, _ := source.CustomExample(ctx, s.Log.RulesLock(), ref)
					return map[string]any{"option": c, "customExample": example}, nil
				}
				return c, nil
			}
		}
		return nil, fmt.Errorf("option not found")
	case "list_choice_options":
		prompts, err := domain.Prompts(s.Log, cat)
		if err != nil {
			return nil, err
		}
		for _, p := range prompts {
			if p.Choice.Prompt.String() == args.Prompt {
				choice := cat.ResolveChoice(p.Choice)
				keys := rules.OptionKeys(choice.From)
				if choice.From.Kind == rules.OptionsFromCollection {
					for _, candidate := range catalogCandidates(cat, choice.From.Collection.String()) {
						ref, _ := rules.ParseRef(candidate.Ref)
						keys = append(keys, ref.Slug)
					}
				}
				start := min(max(0, args.Offset), len(keys))
				end := min(start+60, len(keys))
				return map[string]any{"prompt": compactAgentPrompts([]domain.Prompt{p}), "keys": keys[start:end], "offset": start, "total": len(keys), "nextOffset": end}, nil
			}
		}
		return nil, fmt.Errorf("prompt not open")
	case "ask_user":
		if strings.TrimSpace(args.Text) == "" {
			return nil, fmt.Errorf("question required")
		}
		if len(args.Options) > 6 {
			return nil, fmt.Errorf("offer at most six concise answers")
		}
		for _, option := range args.Options {
			if strings.TrimSpace(option) == "" || len(option) > 300 {
				return nil, fmt.Errorf("invalid answer option")
			}
		}
		s.Status = "waiting"
		addAgentEvent(s, "question", args.Text, "", nil)
		s.Events[len(s.Events)-1].Options = append([]string(nil), args.Options...)
		return map[string]bool{"waiting": true}, nil
	case "prepare_review":
		state, err := domain.Project(s.Log, cat)
		if err != nil {
			return nil, err
		}
		if state.Identity.Name == "…" || strings.TrimSpace(state.Identity.Name) == "" {
			return nil, fmt.Errorf("name missing")
		}
		if !args.AllowIncomplete {
			prompts, err := domain.Prompts(s.Log, cat)
			if err != nil {
				return nil, err
			}
			required := []domain.Prompt{}
			for _, prompt := range prompts {
				if !prompt.Optional {
					required = append(required, prompt)
				}
			}
			if len(required) > 0 {
				return map[string]any{"ready": false, "remainingChoices": compactAgentPrompts(required), "next": "Resolve these choices from the source or call ask_user with a focused question and suggested answers. Do not send the user to the editor. Only set allow_incomplete if the user explicitly chooses to leave choices incomplete."}, nil
			}
		}
		s.Status = "review"
		addAgentEvent(s, "assistant", args.Text, "", nil)
		return map[string]bool{"ready": true}, nil
	case "upsert_custom_option":
		if args.Definition != "" {
			compiler, ok := a.service.catalog.(interface {
				CompilePrivate(context.Context, pack.Lock, string, []byte, rules.Locale) (*catalog.Catalog, error)
			})
			if !ok {
				return nil, fmt.Errorf("private pack compiler unavailable")
			}
			compiled, err := compiler.CompilePrivate(ctx, s.Log.RulesLock(), s.ID, []byte(args.Definition), s.Locale)
			if err != nil {
				return nil, err
			}
			log := s.Log.Clone()
			log.Events[0].RulesLock = compiled.Lock.Clone()
			if err := validateImported(compiled, log); err != nil {
				return nil, err
			}
			s.Log = log
			return map[string]any{"automated": true, "lock": compiled.Lock, "namespace": "import-" + s.ID}, nil
		}

		if args.ID == "" || args.Kind == "" || strings.TrimSpace(args.Name) == "" || len(args.Description) > 16000 {
			return nil, fmt.Errorf("id, kind, name and bounded description required")
		}
		entry := AgentManual{ID: args.ID, Kind: args.Kind, Name: args.Name, Description: args.Description, Source: args.Source}
		found := false
		for i, v := range s.Manual {
			if v.ID == entry.ID {
				s.Manual[i] = entry
				found = true
				break
			}
		}
		if !found {
			s.Manual = append(s.Manual, entry)
		}
		// Preserve unknown mechanics explicitly as manual content. Never turn prose
		// into executable rules or silently substitute a similarly named entity.
		note := "import.manual:" + entry.ID + "\n" + entry.Kind + ": " + entry.Name + "\n" + entry.Description + "\n" + entry.Source
		log := s.Log.Clone()
		replaced := false
		for i, e := range log.Events {
			if e.Type == domain.EventNote && strings.HasPrefix(e.Note, "import.manual:"+entry.ID+"\n") {
				log.Events[i].Note = note
				replaced = true
			}
		}
		if !replaced {
			_ = log.Append(domain.Event{Type: domain.EventNote, Note: note})
		}
		s.Log = log
		return map[string]any{"manual": entry, "automated": false}, nil
	case "resolve_import_facts":
		log := s.Log.Clone()
		if args.Ref != "" {
			ref, ok := rules.ParseRef(args.Ref)
			if !ok {
				return nil, fmt.Errorf("invalid canonical ref")
			}
			types := map[rules.RefKind]domain.EventType{rules.RefRace: domain.EventRace, rules.RefSubrace: domain.EventSubrace, rules.RefBackground: domain.EventBackground, rules.RefClass: domain.EventClass, rules.RefSubclass: domain.EventSubclass, rules.RefFeat: domain.EventFeat}
			typ, ok := types[ref.Kind]
			if !ok {
				return nil, fmt.Errorf("use a choice for this reference or preserve a manual entry")
			}
			level := 0
			if args.Level != nil {
				level = *args.Level
				if level < 1 || level > maxLevel(cat) {
					return nil, fmt.Errorf("invalid level")
				}
			}
			// Structural observations replace their previous observation instead of
			// appending another class grant each time reconciliation is repeated.
			replaced := false
			for i, e := range log.Events {
				if e.Type == typ && (typ != domain.EventClass || e.Ref == ref) {
					log.Events[i].Ref = ref
					log.Events[i].Level = level
					replaced = true
					break
				}
			}
			if !replaced {
				_ = log.Append(domain.Event{Type: typ, Ref: ref, Level: level})
			}
		} else {
			v, err := agentValue(args.Value)
			if err != nil {
				return nil, err
			}
			if !allowedFactPath(args.Path) {
				return nil, fmt.Errorf("unsupported fact path; preserve as manual content")
			}
			change := domain.Change{Path: domain.Path(args.Path), Op: domain.OpSet, Value: v}
			replaced := false
			for i, ch := range log.Events[0].Changes {
				if ch.Path == change.Path {
					log.Events[0].Changes[i] = change
					replaced = true
				}
			}
			if !replaced {
				log.Events[0].Changes = append(log.Events[0].Changes, change)
			}
		}
		if err := validateImported(cat, log); err != nil {
			return nil, err
		}
		projected, err := domain.Project(log, cat)
		if err != nil {
			return nil, err
		}
		if projected.Identity.Level() > maxLevel(cat) {
			return nil, fmt.Errorf("total class levels exceed this ruleset")
		}
		if errs := validateChanges(cat, log.Events[0], 0); len(errs) > 0 {
			return nil, fmt.Errorf("invalid observed fields: %v", errs)
		}
		if sub, ok := cat.Subraces.Get(projected.Identity.Subrace); ok && sub.Race != projected.Identity.Race {
			return nil, fmt.Errorf("subrace does not belong to the observed race")
		}
		if args.Assumption != "" {
			s.Assumptions = append(s.Assumptions, args.Assumption)
		}
		s.Log = log
		return map[string]bool{"applied": true}, nil
	case "answer_choice", "revise_choice":
		prompts, err := domain.Prompts(s.Log, cat)
		if err != nil {
			return nil, err
		}
		var event domain.Event
		found := false
		for _, p := range prompts {
			if p.Choice.Prompt.String() == args.Prompt {
				event = domain.Event{Type: p.Event.Type, Ref: p.Event.Ref, Level: p.Event.Level}
				found = true
				break
			}
		}
		if name == "revise_choice" && args.Sequence > 1 && args.Sequence <= len(s.Log.Events) {
			event = s.Log.Events[args.Sequence-1]
			if event.Type == domain.EventNote {
				return nil, fmt.Errorf("this entry is import metadata, not a choice")
			}
			event.Seq = 0
			found = true
		}
		if !found {
			return nil, fmt.Errorf("choice not open; re-read context")
		}
		picks := []rules.Slug{}
		for _, v := range args.Picks {
			picks = append(picks, rules.Slug(v))
		}
		event.Choices = []domain.Answer{{Prompt: rules.Slug(args.Prompt), Picks: picks}}
		if strings.HasPrefix(args.Prompt, "character/") && args.Ref != "" {
			event.Choices = nil
		}
		if args.Ref != "" {
			ref, ok := rules.ParseRef(args.Ref)
			if !ok {
				return nil, fmt.Errorf("invalid ref")
			}
			event.Ref = ref
		}
		if name == "revise_choice" {
			log, dropped, err := Revise(s.Log, cat, args.Sequence, &event)
			if err != nil {
				return nil, err
			}
			s.Log = log
			return map[string]any{"dropped": dropped}, nil
		}
		events := []domain.Event{event}
		if err := validateAndAttribute(s.Log, cat, events); err != nil {
			return nil, err
		}
		log := s.Log.Clone()
		if err := log.Append(events...); err != nil {
			return nil, err
		}
		if _, err := domain.Project(log, cat); err != nil {
			return nil, err
		}
		s.Log = log
		return map[string]bool{"applied": true}, nil
	default:
		return nil, fmt.Errorf("unknown tool")
	}
}
func allowedFactPath(path string) bool {
	for _, prefix := range []string{"identity.", "finalAbilities.", "base.hitPoints.", "status.", "skills.", "savingThrows.", "equipment.", "base.languages"} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func compactAgentPrompts(prompts []domain.Prompt) []map[string]any {
	out := []map[string]any{}
	for _, p := range prompts {
		out = append(out, map[string]any{"id": p.Choice.Prompt, "choose": p.Choice.Choose, "kind": p.Choice.Kind.String(), "collection": p.Choice.From.Collection.String(), "source": p.Source.String(), "purpose": p.Purpose, "optional": p.Optional, "event": p.Event, "held": p.Held, "blocked": p.Blocked, "heldOnly": p.HeldOnly, "upTo": p.UpTo})
	}
	return out
}
