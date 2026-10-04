package character

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

var inventoryCount = regexp.MustCompile(`^([0-9]+)\s+(.+)$`)

var customKey = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

type agentArgs struct {
	Scores          map[string]*int `json:"scores"`
	Facts           []agentArgs     `json:"facts"`
	Expected        []string        `json:"expected"`
	Parent          string          `json:"parent"`
	Ability         string          `json:"ability"`
	Mode            string          `json:"mode"`
	Placement       string          `json:"placement"`
	HitDie          *int            `json:"hit_die"`
	Speed           *int            `json:"speed"`
	Count           int             `json:"count"`
	Selected        *bool           `json:"selected"`
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
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&args); err != nil {
		return nil, err
	}
	// Accept both batch and single shapes across the compatible fact tools.
	if name == "resolve_import_facts" && len(args.Facts) > 0 {
		return a.tool(ctx, s, "import_facts", b)
	}
	if name == "import_facts" && len(args.Facts) == 0 && (args.Ref != "" || args.Path != "") {
		return a.tool(ctx, s, "resolve_import_facts", b)
	}
	cat, err := a.Catalog(ctx, *s)
	if err != nil {
		return nil, err
	}
	switch name {
	case "plan_import":
		if len(args.Expected) == 0 || len(args.Expected) > 300 {
			return nil, fmt.Errorf("list 1-300 documented source facts")
		}
		mappings := map[string]string{}
		for _, original := range args.Expected {
			key := original
			if !knownFactPath(cat, key) && !strings.HasPrefix(key, "custom:") {
				ref, refErr := resolveAgentReference(cat, key)
				exists := refErr == nil
				if exists {
					key = ref.Canonical()
				}
				if !exists {
					id := strings.Trim(customKey.ReplaceAllString(key, "-"), "-")
					if id == "" || len(id) > 100 {
						return nil, fmt.Errorf("invalid checklist key")
					}
					key = "custom:" + id
					mappings[original] = key
				}
			}
			found := false
			for _, old := range s.expected {
				if old == key {
					found = true
				}
			}
			if !found {
				s.expected = append(s.expected, key)
			}
		}
		for _, ability := range []string{"str", "dex", "con", "int", "wis", "cha"} {
			value, ok := args.Scores[ability]
			if !ok || value == nil {
				continue
			}
			path := "finalAbilities." + ability
			fact := agentArgs{Path: path, Value: json.RawMessage(strconv.Itoa(*value)), Source: args.Source}
			encoded, _ := json.Marshal(fact)
			result, err := a.tool(ctx, s, "resolve_import_facts", encoded)
			if err != nil {
				return nil, err
			}
			encoded, _ = json.Marshal(result)
			a.recordProgress(ctx, s, "resolve_import_facts", fact, encoded)
			if !slices.Contains(s.expected, path) {
				s.expected = append(s.expected, path)
			}
		}
		return map[string]any{"expected": s.expected, "customKeys": mappings, "next": "Use the returned normalized checklist. Preserve each custom key with upsert_custom_option id equal to the suffix after custom:. Source page labels themselves do not need checklist keys."}, nil
	case "import_facts":
		if len(args.Facts) == 0 || len(args.Facts) > 300 {
			return nil, fmt.Errorf("provide 1-300 facts in the facts array")
		}
		failures := []map[string]string{}
		applied := 0
		for _, fact := range args.Facts {
			value, _ := json.Marshal(fact)
			result, err := a.tool(ctx, s, "resolve_import_facts", value)
			if err != nil {
				failures = append(failures, map[string]string{"path": fact.Path, "ref": fact.Ref, "error": err.Error()})
				continue
			}
			applied++
			encoded, _ := json.Marshal(result)
			a.recordProgress(ctx, s, "resolve_import_facts", fact, encoded)
		}
		return map[string]any{"applied": applied, "errors": failures}, nil
	case "get_build_context", "reconcile_import":
		state, err := domain.Project(s.Log, cat)
		if err != nil {
			return nil, err
		}
		prompts, err := domain.Prompts(s.Log, cat)
		return map[string]any{"sheet": state, "prompts": compactAgentPrompts(prompts), "history": s.Log, "manual": s.Manual, "assumptions": s.Assumptions, "rules": cat.Lock, "sourceChecklist": s.expected}, err
	case "read_source":
		for _, f := range s.Files {
			if strings.EqualFold(f.Name, args.Name) || strings.EqualFold(f.Name, args.Name[strings.LastIndex(args.Name, "/")+1:]) || args.Name == "" && len(s.Files) == 1 {
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
		ref, err := resolveAgentReference(cat, args.Ref)
		if err != nil {
			return nil, err
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
		s.Events[len(s.Events)-1].Actions = []string{"view", "edit"}
		return map[string]bool{"waiting": true}, nil
	case "prepare_review":
		if len(s.Files) > 0 && len(s.expected) == 0 {
			return nil, fmt.Errorf("read all source pages and call plan_import with every documented fact before review")
		}
		missing := []string{}
		for _, key := range s.expected {
			found := false
			for _, e := range s.Log.Events {
				expectedRef, isRef := rules.ParseRef(key)
				if isRef && e.Ref == expectedRef {
					found = true
				}
				if ref, ok := rules.ParseRef(key); ok {
					for _, answer := range e.Choices {
						for _, pick := range answer.Picks {
							if pick == ref.Slug {
								found = true
							}
						}
					}
				}
				for _, ch := range e.Changes {
					if string(ch.Path) == key || strings.Count(key, ".") == 1 && (strings.HasPrefix(key, "skills.") || strings.HasPrefix(key, "savingThrows.")) && strings.HasPrefix(string(ch.Path), key+".") {
						found = true
					}
				}
				customRef, customIsRef := rules.Ref{}, false
				if e.Custom != nil {
					customRef, customIsRef = rules.ParseRef(e.Custom.Reference)
				}
				if e.Custom != nil && ("custom:"+e.Custom.ID == key || isRef && customIsRef && customRef == expectedRef) {
					found = true
				}
			}
			if !found {
				missing = append(missing, key)
			}
		}
		if len(missing) > 0 {
			return map[string]any{"ready": false, "missingSourceFacts": missing, "next": "Import every documented fact before review. allow_incomplete only exempts undocumented historical choices."}, nil
		}
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
		s.Events[len(s.Events)-1].Actions = []string{"view", "edit", "save"}
		return map[string]bool{"ready": true}, nil
	case "upsert_custom_option":
		// Reuse an unambiguous exact catalogue identity when the model omitted ref.
		// This preserves the spelling/name without turning known content into a new entity.
		if args.Ref == "" && args.Kind != "note" {
			kind := args.Kind
			if kind == "cantrip" {
				kind = "spell"
			}
			candidates, searchErr := a.search(ctx, s, kind, args.Name, nil)
			matches := []AgentCandidate{}
			if searchErr == nil {
				for _, candidate := range candidates {
					if candidate.Score == 1 {
						matches = append(matches, candidate)
					}
				}
			}
			if len(matches) == 1 {
				args.Ref = matches[0].Ref
			} else if args.Kind == "race" {
				subraces, searchErr := a.search(ctx, s, "subrace", args.Name, nil)
				exact := []AgentCandidate{}
				if searchErr == nil {
					for _, candidate := range subraces {
						if candidate.Score == 1 {
							exact = append(exact, candidate)
						}
					}
				}
				if len(exact) == 1 {
					ref, _ := rules.ParseRef(exact[0].Ref)
					sub, _ := cat.Subraces.Get(ref.Slug)
					parentArgs := agentArgs{Ref: rules.NewRef(rules.RefRace, sub.Race).Canonical(), Source: args.Source}
					encoded, _ := json.Marshal(parentArgs)
					result, err := a.tool(ctx, s, "resolve_import_facts", encoded)
					if err != nil {
						return nil, err
					}
					encoded, _ = json.Marshal(result)
					a.recordProgress(ctx, s, "resolve_import_facts", parentArgs, encoded)
					args.Kind = "subrace"
					args.Parent = sub.Race.String()
					args.Ref = ref.Canonical()
				}
			}
		}

		// Optional schema values may arrive as zero/empty from a model. They are
		// absence of evidence, never asserted mechanics for unrelated kinds.
		if args.Kind != "class" || args.HitDie != nil && *args.HitDie == 0 {
			args.HitDie = nil
		}
		if args.Kind != "race" {
			args.Speed = nil
		}
		if args.Kind != "class" && args.Kind != "spell" || args.Level != nil && *args.Level == 0 {
			args.Level = nil
		}
		if args.Kind != "class" && args.Kind != "subclass" && args.Kind != "spell" && args.Kind != "cantrip" {
			args.Ability = ""
		}
		if args.Ability == "none" || args.Ability == "unknown" {
			args.Ability = ""
		}
		abilityNames := map[string]string{"strength": "str", "dexterity": "dex", "constitution": "con", "intelligence": "int", "wisdom": "wis", "charisma": "cha"}
		if code := abilityNames[strings.ToLower(args.Ability)]; code != "" {
			args.Ability = code
		}
		if args.Kind != "item" {
			args.Count = 0
			args.Placement = ""
		}
		if args.Kind != "spell" && args.Kind != "cantrip" {
			args.Mode = ""
		}
		if args.Ref != "" {
			found := false
			if ref, refErr := resolveAgentReference(cat, args.Ref); refErr == nil {
				for _, candidate := range catalogCandidates(cat, ref.Kind.String()) {
					if candidate.Ref == ref.Canonical() {
						found = true
						args.Ref = ref.Canonical()
						if args.Kind == "spell" && ref.Kind == rules.RefSpell {
							if spell, ok := cat.Spells.Get(ref.Slug); ok && spell.Level == 0 {
								args.Kind = "cantrip"
							}
						}

					}
				}
			}
			if !found {
				args.Ref = ""
			}
		}

		if args.Definition != "" {
			compiler, ok := a.service.catalog.(interface {
				CompilePrivate(context.Context, pack.Lock, string, []byte, rules.Locale) (*catalog.Catalog, error)
			})
			if !ok {
				return nil, fmt.Errorf("private pack compiler unavailable")
			}
			compiled, err := compiler.CompilePrivate(ctx, s.Log.RulesLock(), s.ID, []byte(args.Definition), s.Locale)
			if err != nil && (args.ID == "" || args.Kind == "" || strings.TrimSpace(args.Name) == "") {
				return nil, err
			}
			if err == nil {
				log := s.Log.Clone()
				log.Events[0].RulesLock = compiled.Lock.Clone()
				if err := validateImported(compiled, log); err != nil {
					return nil, err
				}
				s.Log = log
				return map[string]any{"automated": true, "lock": compiled.Lock, "namespace": "import-" + s.ID}, nil
			}
		}

		if args.ID == "" || args.Kind == "" || strings.TrimSpace(args.Name) == "" || len(args.Description) > 16000 {
			return nil, fmt.Errorf("id, kind, name and bounded description required")
		}
		if args.Kind == "class" && args.Level == nil && args.Ref != "" {
			ref, _ := rules.ParseRef(args.Ref)
			state, _ := domain.Project(s.Log, cat)
			for _, taken := range state.Identity.Classes {
				if taken.Class == ref.Slug {
					level := taken.Level
					args.Level = &level
				}
			}
		}
		if args.Parent != "" {
			parentRef := args.Parent
			if _, ok := rules.ParseRef(parentRef); !ok {
				kind := "class"
				if args.Kind == "subrace" {
					kind = "race"
				}
				parentRef = kind + ":" + args.Parent
			}
			if parent, err := resolveAgentReference(cat, parentRef); err == nil {
				args.Parent = parent.Slug.String()
			}
		}
		customLog := s.Log
		if len(s.Files) > 0 && args.Kind == "item" && (args.Selected == nil || *args.Selected) {
			customLog = clearImportedInventory(customLog, args.Placement)
		}
		option := domain.CustomOption{ID: args.ID, Kind: args.Kind, Name: args.Name, Description: args.Description, Source: args.Source, Parent: args.Parent, Ability: args.Ability, Mode: args.Mode, Placement: args.Placement, Level: args.Level, HitDie: args.HitDie, Speed: args.Speed, Count: args.Count, Selected: true, Reference: args.Ref}
		if args.Selected != nil {
			option.Selected = *args.Selected
		}
		if option.Reference != "" && slices.Contains([]string{"class", "race", "subrace", "background", "subclass"}, option.Kind) {
			state, _ := domain.Project(s.Log, cat)
			ref, _ := rules.ParseRef(option.Reference)
			held := ref.Slug == state.Identity.Race && option.Kind == "race" || ref.Slug == state.Identity.Subrace && option.Kind == "subrace" || ref.Slug == state.Identity.Background && option.Kind == "background"
			for _, taken := range state.Identity.Classes {
				held = held || option.Kind == "class" && taken.Class == ref.Slug || option.Kind == "subclass" && taken.Subclass == ref.Slug
			}
			if held {
				option.Selected = false
			}
		}
		log, err := upsertCustom(customLog, cat, option)
		if err != nil {
			return nil, err
		}
		s.Log = log
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

		return map[string]any{"manual": entry, "automated": false}, nil
	case "resolve_import_facts":
		if args.Ref == "" && (args.Path == "" || len(args.Value) == 0) {
			return nil, fmt.Errorf("provide ref plus optional level OR path plus value; batches use the facts array")
		}
		log := s.Log.Clone()
		segments := strings.Split(args.Path, ".")
		if len(s.Files) > 0 && len(segments) == 3 && segments[0] == "equipment" && segments[1] != "purse" {
			log = clearImportedInventory(log, segments[1])
		}
		if args.Ref != "" {
			ref, err := resolveAgentReference(cat, args.Ref)
			if err != nil {
				return nil, err
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
					log.Events[i].Observed = true
					log.Events[i].Evidence = args.Source
					log.Events[i].Source = observedGroup(typ, "")
					replaced = true
					break
				}
			}
			if !replaced {
				_ = log.Append(domain.Event{Type: typ, Ref: ref, Level: level, Evidence: args.Source, Observed: true, Source: observedGroup(typ, "")})
			}
		} else {
			v, err := agentValue(args.Value)
			if err != nil {
				return nil, err
			}
			if len(segments) == 2 && segments[0] == "equipment" && slices.Contains([]string{"equipped", "backpack", "loot"}, segments[1]) && v.Kind == domain.ValueSlugList && len(v.Slugs) > 0 {
				counts := map[rules.Slug]int{}
				for _, entry := range v.Slugs {
					count := 1
					explicitCount := false
					name := entry.String()
					if match := inventoryCount.FindStringSubmatch(name); match != nil {
						explicitCount = true
						count, _ = strconv.Atoi(match[1])
						name = match[2]
					}
					slug := rules.Slug(name)
					if !cat.Items.Has(slug) && !cat.MagicItems.Has(slug) {
						candidates, err := a.search(ctx, s, "item", name, nil)
						if err != nil {
							return nil, err
						}
						matches := []AgentCandidate{}
						for _, candidate := range candidates {
							if candidate.Score == 1 {
								matches = append(matches, candidate)
							}
						}
						if len(matches) != 1 {
							return nil, fmt.Errorf("inventory entry %q needs an exact catalogue slug or a typed custom item with its quantity", entry)
						}
						ref, _ := rules.ParseRef(matches[0].Ref)
						slug = ref.Slug
					}
					if count < 1 || count > 100000 {
						return nil, fmt.Errorf("invalid inventory quantity")
					}
					if explicitCount {
						counts[slug] = max(counts[slug], count)
					} else {
						counts[slug] += count
					}
				}
				// Rewrite source observations, retaining later ordinary player edits.
				before := s.Log.Clone()
				reset := args
				reset.Value = json.RawMessage("[]")
				encoded, _ := json.Marshal(reset)
				if _, err := a.tool(ctx, s, "resolve_import_facts", encoded); err != nil {
					s.Log = before
					return nil, err
				}
				oldPaths := []string{}
				for _, e := range before.Events {
					if !e.Observed {
						continue
					}
					for _, ch := range e.Changes {
						if strings.HasPrefix(string(ch.Path), args.Path+".") {
							oldPaths = append(oldPaths, string(ch.Path))
						}
					}
				}
				values := map[string]int{}
				for _, path := range oldPaths {
					values[path] = 0
				}
				for slug, count := range counts {
					values[args.Path+"."+slug.String()] = count
				}
				paths := []string{}
				for path := range values {
					paths = append(paths, path)
				}
				slices.Sort(paths)
				for _, path := range paths {
					fact := args
					fact.Path = path
					fact.Value = json.RawMessage(strconv.Itoa(values[path]))
					encoded, _ := json.Marshal(fact)
					if _, err := a.tool(ctx, s, "resolve_import_facts", encoded); err != nil {
						s.Log = before
						return nil, err
					}
				}
				return map[string]any{"applied": true, "counts": counts}, nil
			}
			if args.Path == "base.size" && v.Kind == domain.ValueString {
				v.Str = strings.ToLower(strings.TrimSpace(v.Str))
			}
			if !allowedFactPath(args.Path) {
				return nil, fmt.Errorf("unsupported fact path; preserve as manual content")
			}
			change := domain.Change{Path: domain.Path(args.Path), Op: domain.OpSet, Value: v}
			replaced := false
			// Update the observation, leaving later direct player edits intact.
			for i, e := range log.Events {
				if e.Observed && e.Type == domain.EventChange && len(e.Changes) == 1 && e.Changes[0].Path == change.Path {
					log.Events[i].Changes = []domain.Change{change}
					log.Events[i].Evidence = args.Source
					replaced = true
					break
				}
			}
			if args.Path == "identity.name" {
				for i, ch := range log.Events[0].Changes {
					if ch.Path == change.Path {
						log.Events[0].Changes[i] = change
						replaced = true
					}
				}
			}
			if !replaced {
				_ = log.Append(domain.Event{Type: domain.EventChange, Changes: []domain.Change{change}, Evidence: args.Source, Observed: true, Source: observedGroup(domain.EventChange, args.Path)})
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
	for _, prefix := range []string{"identity.", "finalAbilities.", "base.", "status.", "skills.", "savingThrows.", "equipment.", "base.languages"} {
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

// knownFactPath keeps checklist entries addressable. Unknown source fields
// become explicit custom entries instead of impossible review requirements.
func knownFactPath(cat *catalog.Catalog, path string) bool {
	parts := strings.Split(path, ".")
	switch parts[0] {
	case "identity":
		return len(parts) == 2 && slices.Contains([]string{"name", "alignment", "experience", "desiredLevel", "personalityTraits", "ideals", "bonds", "flaws"}, parts[1])
	case "finalAbilities":
		if len(parts) != 2 {
			return false
		}
		_, ok := rules.ParseAbility(parts[1])
		return ok
	case "base":
		return path == "base.languages" || path == "base.size" || path == "base.speed" || len(parts) == 3 && (parts[1] == "hitPoints" && slices.Contains([]string{"max", "current", "temporary"}, parts[2]) || parts[1] == "speeds" && parts[2] == "walking" || parts[1] == "senses" && parts[2] == "darkvision")
	case "status":
		return len(parts) == 2 && slices.Contains([]string{"armorClass", "initiative", "passivePerception", "proficiencyBonus"}, parts[1])
	case "skills":
		return len(parts) >= 2 && len(parts) <= 3 && cat.Skills.Has(rules.Slug(parts[1])) && (len(parts) == 2 || slices.Contains([]string{"proficiency", "bonus"}, parts[2]))
	case "savingThrows":
		if len(parts) < 2 || len(parts) > 3 {
			return false
		}
		_, ok := rules.ParseAbility(parts[1])
		return ok && (len(parts) == 2 || slices.Contains([]string{"proficient", "bonus"}, parts[2]))
	case "equipment":
		if len(parts) == 3 && parts[1] == "purse" {
			_, err := rules.ParseCoinUnit(parts[2])
			return err == nil
		}
		return len(parts) >= 2 && len(parts) <= 3 && slices.Contains([]string{"equipped", "backpack", "loot"}, parts[1]) && (len(parts) == 2 || cat.Items.Has(rules.Slug(parts[2])) || cat.MagicItems.Has(rules.Slug(parts[2])))
	}
	return false
}

func clearImportedInventory(log domain.Log, placement string) domain.Log {
	if placement == "" {
		placement = "backpack"
	}
	if !slices.Contains([]string{"equipped", "backpack", "loot"}, placement) {
		return log
	}
	path := domain.Path("equipment." + placement)
	for _, event := range log.Events {
		for _, change := range event.Changes {
			if change.Path == path && change.Op == domain.OpSet {
				return log
			}
		}
	}
	updated := log.Clone()
	_ = updated.Append(domain.Event{Type: domain.EventChange, Observed: true, Changes: []domain.Change{{Path: path, Op: domain.OpSet, Value: domain.SlugListValue(nil)}}})
	return updated
}
