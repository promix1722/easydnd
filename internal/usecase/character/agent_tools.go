package character

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

type agentArgs struct {
	Scores          map[string]*int `json:"scores"`
	Skills          map[string]*int `json:"skills"`
	Saves           map[string]*int `json:"saves"`
	Spells          []string        `json:"spells"`
	Prepared        []string        `json:"prepared"`
	Proficient      []string        `json:"proficient"`
	Expertise       []string        `json:"expertise"`
	Coins           map[string]*int `json:"coins"`
	Alignment       string          `json:"alignment"`
	Traits          []string        `json:"personality_traits"`
	Ideals          []string        `json:"ideals"`
	Bonds           []string        `json:"bonds"`
	Flaws           []string        `json:"flaws"`
	HitPoints       *int            `json:"hit_points"`
	ArmorClass      *int            `json:"armor_class"`
	Facts           []agentArgs     `json:"facts"`
	Answers         []agentArgs     `json:"answers"`
	Items           []agentArgs     `json:"items"`
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

// countedName is an item written with its quantity: "10 Rations (1 day)",
// "2x Dagger", "Playing Card Set x2".
var countedName = regexp.MustCompile(`^(?:([0-9]+)\s*[x×]?\s+(.+)|(.+?)\s*[x×]\s*([0-9]+))$`)

var abilityNames = map[string]string{"strength": "str", "dexterity": "dex", "constitution": "con", "intelligence": "int", "wisdom": "wis", "charisma": "cha"}

// structuralEvents are the catalogue entries a character is made of. Each is
// one event naming the entry, which is why a sheet's "Sorcerer 3" is written
// as a fact rather than answered as a pick.
var structuralEvents = map[rules.RefKind]domain.EventType{rules.RefRace: domain.EventRace, rules.RefSubrace: domain.EventSubrace, rules.RefBackground: domain.EventBackground, rules.RefClass: domain.EventClass, rules.RefSubclass: domain.EventSubclass, rules.RefFeat: domain.EventFeat}

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

// agentToolName settles which handler a call means. The fact and answer tools
// each take one entry or a batch, and a model sends either shape to either
// name.
func agentToolName(name string, args agentArgs) string {
	switch {
	case name == "resolve_import_facts" && len(args.Facts) > 0:
		return "import_facts"
	case name == "import_facts" && len(args.Facts) == 0 && (args.Ref != "" || args.Path != "" || args.Kind != ""):
		return "resolve_import_facts"
	case name == "answer_choice" && len(args.Answers) > 0:
		return "answer_choices"
	case name == "answer_choices" && len(args.Answers) == 0 && args.Prompt != "":
		return "answer_choice"
	}
	return name
}

func (a *Agent) tool(ctx context.Context, s *AgentSession, name string, b []byte) (any, error) {
	var args agentArgs
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&args); err != nil {
		return nil, err
	}
	cat, err := a.Catalog(ctx, *s)
	if err != nil {
		return nil, err
	}
	switch agentToolName(name, args) {
	case "plan_import":
		if len(args.Expected) > 300 {
			return nil, fmt.Errorf("list at most 300 documented source facts")
		}
		rejected := []map[string]string{}
		for _, original := range args.Expected {
			key, ok := a.checklistKey(ctx, s, cat, strings.TrimSpace(original))
			if !ok {
				rejected = append(rejected, map[string]string{"key": original, "reason": "not tracked: the checklist takes fact paths, custom:<id>, and kind:Name for race, subrace, class, subclass, background, feat, feature, trait, spell or item"})
				continue
			}
			if !slices.Contains(s.expected, key) {
				s.expected = append(s.expected, key)
			}
		}
		// The sheet's numbers are taken down first and all at once, in a shape
		// that has a slot for each. A number asked for one fact at a time is a
		// number a model skips.
		//
		// Scores and level are what the character is built from, and are
		// written. Everything else is what the build should come out as, and is
		// only kept to compare against.
		failed := []map[string]any{}
		fail := func(path string, err error) {
			a.service.log.Warn("AI wizard tool rejected", "tool", "plan_import", "fact", path, "error", err)
			failure := map[string]any{}
			_ = json.Unmarshal(agentError(err), &failure)
			failure["path"] = path
			failed = append(failed, failure)
		}
		inputs := []agentArgs{}
		for _, ability := range []string{"str", "dex", "con", "int", "wis", "cha"} {
			if value := args.Scores[ability]; value != nil {
				inputs = append(inputs, agentArgs{Path: "finalAbilities." + ability, Value: json.RawMessage(strconv.Itoa(*value)), Source: args.Source})
			}
		}
		if args.Level != nil {
			inputs = append(inputs, agentArgs{Path: "identity.desiredLevel", Value: json.RawMessage(strconv.Itoa(*args.Level)), Source: args.Source})
		}
		for _, unit := range slices.Sorted(maps.Keys(args.Coins)) {
			if value := args.Coins[unit]; value != nil {
				inputs = append(inputs, agentArgs{Path: "equipment.purse." + unit, Value: json.RawMessage(strconv.Itoa(*value)), Source: args.Source})
			}
		}
		// What the sheet says about who the character is has a slot each too:
		// asked for as a separate fact, the second personality trait was the
		// one a model left out. An empty slot writes nothing.
		for _, text := range []struct{ path, value string }{{"identity.name", args.Name}, {"identity.alignment", args.Alignment}} {
			if strings.TrimSpace(text.value) != "" {
				inputs = append(inputs, agentArgs{Path: text.path, Value: raw(text.value), Source: args.Source})
			}
		}
		for _, list := range []struct {
			path   string
			values []string
		}{{"identity.personalityTraits", args.Traits}, {"identity.ideals", args.Ideals}, {"identity.bonds", args.Bonds}, {"identity.flaws", args.Flaws}} {
			if values := slices.DeleteFunc(slices.Clone(list.values), func(v string) bool { return strings.TrimSpace(v) == "" }); len(values) > 0 {
				inputs = append(inputs, agentArgs{Path: list.path, Value: raw(values), Source: args.Source})
			}
		}
		for _, fact := range inputs {
			result, err := a.importFact(ctx, s, cat, fact)
			if err != nil {
				fail(fact.Path, err)
				continue
			}
			if path := result["path"].(string); strings.HasPrefix(path, "finalAbilities.") && !slices.Contains(s.expected, path) {
				s.expected = append(s.expected, path)
			}
			// The level is announced with the class it belongs to.
			if fact.Path != "identity.desiredLevel" {
				a.recordProgress(ctx, s, "resolve_import_facts", fact, raw(result))
			}
		}
		if s.printed == nil {
			s.printed = map[string]int{}
		}
		note := func(path string, value *int) {
			if value == nil {
				return
			}
			path, err := a.factPath(ctx, s, cat, path)
			if err == nil && !knownFactPath(cat, path) {
				err = fmt.Errorf("unknown fact path %q", path)
			}
			if err != nil {
				fail(path, err)
				return
			}
			s.printed[path] = *value
		}
		note("base.hitPoints.max", args.HitPoints)
		note("status.armorClass", args.ArmorClass)
		note("base.speed", args.Speed)
		for _, skill := range slices.Sorted(maps.Keys(args.Skills)) {
			note("skills."+skill+".bonus", args.Skills[skill])
		}
		for _, ability := range slices.Sorted(maps.Keys(args.Saves)) {
			note("savingThrows."+ability+".bonus", args.Saves[ability])
		}
		for _, spell := range args.Spells {
			if spell = strings.TrimSpace(spell); spell != "" && !slices.Contains(s.spells, spell) {
				s.spells = append(s.spells, spell)
				s.expected = append(s.expected, "spell:"+spell)
			}
		}
		if len(args.Prepared) > 0 {
			s.prepared = args.Prepared
		}
		out := map[string]any{"expected": localChecklist(s.expected), "rejected": rejected, "errors": failed, "proficient": printedProficiencies(s.Log, cat, s.printed), "next": "proficient is what the printed bonuses imply. Once race, class, subclass and background are set, assign_skills puts exactly those skills into the prompts that offer them, and assign_spells does the same for the sheet's spells. Resend any unmatched inventory item with set_inventory, by one of its candidate refs."}
		if len(args.Items) > 0 {
			out["inventory"] = a.setInventory(ctx, s, cat, args.Items)
		}
		if out["open"], err = a.openPrompts(s, cat); err != nil {
			return nil, err
		}
		return out, nil
	case "import_facts":
		if len(args.Facts) == 0 || len(args.Facts) > 300 {
			return nil, fmt.Errorf("provide 1-300 facts in the facts array")
		}
		failures := []map[string]any{}
		resolved := []any{}
		applied := 0
		for i, fact := range args.Facts {
			result, err := a.importFact(ctx, s, cat, fact)
			if err != nil {
				a.service.log.Warn("AI wizard tool rejected", "tool", "import_facts", "fact", fact.Path+fact.Ref+fact.Name, "error", err)
				failure := map[string]any{}
				_ = json.Unmarshal(agentError(err), &failure)
				failure["index"], failure["path"], failure["ref"], failure["name"] = i, fact.Path, fact.Ref, fact.Name
				failures = append(failures, failure)
				continue
			}
			applied++
			if _, named := result["ref"]; named {
				resolved = append(resolved, result)
			}
			a.recordProgress(ctx, s, "resolve_import_facts", fact, raw(result))
		}
		open, err := a.openPrompts(s, cat)
		if err != nil {
			return nil, err
		}
		return map[string]any{"applied": applied, "resolved": resolved, "errors": failures, "open": open, "differences": a.differences(s, cat)}, nil
	case "resolve_import_facts":
		return a.importFact(ctx, s, cat, args)
	case "get_build_context":
		state, err := domain.Project(s.Log, cat)
		if err != nil {
			return nil, err
		}
		open, err := a.openPrompts(s, cat)
		if err != nil {
			return nil, err
		}
		answered := []map[string]any{}
		for _, e := range s.Log.Events {
			for _, answer := range e.Choices {
				answered = append(answered, map[string]any{"seq": e.Seq, "prompt": answer.Prompt, "picks": localKeys(cat, answer.Picks)})
			}
		}
		custom := []map[string]any{}
		for _, c := range domain.CustomOptions(s.Log) {
			custom = append(custom, map[string]any{"id": c.ID, "kind": c.Kind, "name": c.Name, "ref": c.Reference, "selected": c.Selected})
		}
		return map[string]any{"sheet": agentSheet(state), "differences": a.differences(s, cat), "prompts": open, "answered": answered, "custom": custom, "checklistMissing": localChecklist(a.missing(ctx, s, cat, state))}, nil
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
		kind := args.Kind
		if mapped, ok := naturalKinds[kind]; ok {
			kind = mapped
		}
		if catalogCandidates(cat, kind) == nil {
			return nil, fmt.Errorf("unknown kind %q; use spell, race, subrace, class, subclass, background, feat, feature, trait, item, magic-item, language, skill, proficiency or alignment", args.Kind)
		}
		candidates, err := a.search(ctx, s, kind, args.Query, args.Level, nil, false)
		for i := range candidates {
			candidates[i].Details = nil
		}
		return candidates, err
	case "get_option_details":
		found, err := a.resolve(ctx, s, cat, nil, args.Ref, nil, false)
		if err != nil {
			return nil, err
		}
		if source, ok := a.service.catalog.(interface {
			CustomExample(context.Context, pack.Lock, rules.Ref) (any, error)
		}); ok {
			ref, _ := rules.ParseRef(found.Ref)
			example, _ := source.CustomExample(ctx, s.Log.RulesLock(), ref)
			return map[string]any{"option": found, "customExample": example}, nil
		}
		return found, nil
	case "list_choice_options":
		prompts, err := domain.Prompts(nativeLog(s.Log), cat)
		if err != nil {
			return nil, err
		}
		for _, p := range prompts {
			if promptNamed(cat, p, args.Prompt) {
				options := promptOptions(cat, p, catalogNames(cat))
				start := min(max(0, args.Offset), len(options))
				end := min(start+60, len(options))
				return map[string]any{"prompt": args.Prompt, "options": options[start:end], "offset": start, "total": len(options), "nextOffset": end}, nil
			}
		}
		return nil, fmt.Errorf("prompt not open")
	case "ask_user":
		if s.Unattended {
			return nil, fmt.Errorf("the user asked not to be asked anything: leave this open, import what the sources state, and call prepare_review")
		}
		if strings.TrimSpace(args.Text) == "" {
			return nil, fmt.Errorf("question required")
		}
		if len(args.Options) > 10 {
			return nil, fmt.Errorf("offer at most ten concise answers")
		}
		for _, option := range args.Options {
			if strings.TrimSpace(option) == "" || len(option) > 300 {
				return nil, fmt.Errorf("invalid answer option")
			}
		}
		s.Status = "waiting"
		s.asked = s.offered
		addAgentEvent(s, "question", args.Text, "", nil)
		s.Events[len(s.Events)-1].Options = append([]string(nil), args.Options...)
		return map[string]bool{"waiting": true}, nil
	case "prepare_review":
		if len(s.Files) > 0 && len(s.expected) == 0 {
			return nil, fmt.Errorf("read all source pages and call plan_import with every documented fact before review")
		}
		state, err := domain.Project(s.Log, cat)
		if err != nil {
			return nil, err
		}
		// The checklist is a reminder, not a lock. It catches the inventory a
		// model forgot; it must not hold a finished draft hostage over an entry
		// the model worded in a way nothing can satisfy, because the way out of
		// that is inventing custom content until the list goes quiet. So it
		// refuses once per distinct list, and the same list again passes.
		if missing := a.missing(ctx, s, cat, state); len(missing) > 0 && !slices.Equal(missing, s.nudged) {
			s.nudged = missing
			return map[string]any{"ready": false, "missingSourceFacts": localChecklist(missing), "next": "These checklist entries are not in the draft. Import the ones the sheet really documents, by name: race, subrace, class, subclass, background and feat with import_facts; a spell with assign_spells {spells} (never import_facts). Features and traits are never imported. Entries that only restate what the build already shows need nothing: call prepare_review again and it will pass. Do not create custom entries to satisfy this list."}, nil
		}
		if !s.Unattended && (state.Identity.Name == "…" || strings.TrimSpace(state.Identity.Name) == "") {
			return map[string]any{"ready": false, "next": `The character has no name. If the sheet prints one, write it: import_facts {"facts":[{"path":"identity.name","value":"<the name>"}]}. If it does not, do not invent a placeholder: call ask_user "What is the character called?" with three or four names that suit the character as answers, write the reply the same way, then prepare_review again.`}, nil
		}
		if !args.AllowIncomplete && !s.Unattended {
			prompts, err := domain.Prompts(nativeLog(s.Log), cat)
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
				return map[string]any{"ready": false, "remainingChoices": agentPrompts(cat, required, false), "next": "Resolve these choices from the source or call ask_user with a focused question and suggested answers. Do not send the user to the editor. Only set allow_incomplete if the user explicitly chooses to leave choices incomplete."}, nil
			}
		}
		// Whatever is still unanswered is the owner's to decide about, not the
		// model's to pass over: optional questions included, once.
		// Refused until the owner has actually been asked: a second call is
		// not consent.
		// allow_incomplete does not skip this: it is the model's word that
		// the owner chose to leave things open, and the owner was not asked.
		// The builder's own extra-spell questions are not counted -- they
		// are always open, and no character is unfinished for them.
		open, err := a.openPrompts(s, cat)
		if err != nil {
			return nil, err
		}
		open = slices.DeleteFunc(open, func(entry map[string]any) bool { return entry["purpose"] == "custom" })
		if len(open) > 0 && !s.Unattended && !(s.offered && s.asked) {
			s.offered = true
			return map[string]any{"ready": false, "unanswered": open, "next": "These are still unanswered. Do not finish yet: call ask_user and offer to settle them, naming them in plain words, with the answers \"Fill them in for me\", \"One by one\" and \"Leave them blank\". Fill them in: answer each from the sheet or the description. One by one: one ask_user per entry, with its options as prepared answers (for a written one such as a personality trait, offer a few suggestions that suit the character). Leave them blank: prepare_review again."}, nil
		}
		// The draft goes to its owner as a build, not as a build with the
		// sheet's numbers pinned on top. What the build reproduces is dropped
		// here, and with it the checklist entry that asked for it.
		pruned := pruneAgentOverrides(s.Log, cat)
		s.expected = slices.DeleteFunc(s.expected, func(key string) bool {
			return !strings.Contains(key, ":") && overridePath(key) && !pathWritten(pruned, key)
		})
		s.Log = pruned
		s.Status = "review"
		addAgentEvent(s, "assistant", args.Text, "", nil)
		// Nobody was asked, so the handoff says what was left for the owner.
		if s.Unattended && len(open) > 0 {
			s.Events[len(s.Events)-1].Data = raw(map[string]any{"open": open})
		}
		return map[string]bool{"ready": true}, nil
	case "upsert_custom_option":
		return a.customOption(ctx, s, cat, args)
	case "assign_skills":
		return a.assignSkills(ctx, s, cat, args)
	case "assign_spells":
		return a.assignSpells(ctx, s, cat, args)
	case "assign_abilities":
		return a.assignAbilities(ctx, s, cat)
	case "answer_choices":
		if len(args.Answers) > 100 {
			return nil, fmt.Errorf("provide 1-100 answers in the answers array")
		}
		results := []map[string]any{}
		for _, answer := range args.Answers {
			// An answer can add a custom entry or open the prompt the next one
			// names, so each is judged against the catalogue as it now stands.
			if cat, err = a.Catalog(ctx, *s); err != nil {
				return nil, err
			}
			picks, err := a.answer(ctx, s, cat, answer)
			if err != nil {
				a.service.log.Warn("AI wizard tool rejected", "tool", "answer_choices", "prompt", answer.Prompt, "error", err)
				failure := map[string]any{}
				_ = json.Unmarshal(agentError(err), &failure)
				failure["prompt"] = answer.Prompt
				results = append(results, failure)
				continue
			}
			results = append(results, map[string]any{"prompt": answer.Prompt, "applied": true, "picks": localKeys(cat, picks)})
		}
		open, err := a.openPrompts(s, cat)
		if err != nil {
			return nil, err
		}
		return map[string]any{"results": results, "open": open}, nil
	case "answer_choice":
		picks, err := a.answer(ctx, s, cat, args)
		if err != nil {
			return nil, err
		}
		return map[string]any{"applied": true, "picks": localKeys(cat, picks)}, nil
	case "revise_choice":
		if args.Sequence <= 1 || args.Sequence > len(s.Log.Events) {
			return nil, fmt.Errorf("choice not open; re-read context")
		}
		event := s.Log.Events[args.Sequence-1]
		if event.Type == domain.EventNote {
			return nil, fmt.Errorf("this entry is import metadata, not a choice")
		}
		event.Seq = 0
		if args.Prompt != "" {
			// The prompt an entry answered is closed. It reopens on the log as it
			// stood with the entry's own answers taken back out, which is where
			// its options can be read to resolve a pick written as a name.
			before := domain.Log{Events: slices.Clone(s.Log.Events[:args.Sequence])}
			before.Events[args.Sequence-1].Choices = nil
			prompts, err := domain.Prompts(nativeLog(before), cat)
			if err != nil {
				return nil, err
			}
			picks := []rules.Slug{}
			for _, pick := range args.Picks {
				key := rules.Slug(pick)
				if i := slices.IndexFunc(prompts, func(p domain.Prompt) bool { return promptNamed(cat, p, args.Prompt) }); i >= 0 {
					if key, err = a.resolvePick(ctx, s, cat, cat.ResolveChoice(prompts[i].Choice), pick); err != nil {
						return nil, err
					}
				}
				picks = append(picks, key)
			}
			id := rules.Slug(args.Prompt)
			if i := slices.IndexFunc(prompts, func(p domain.Prompt) bool { return promptNamed(cat, p, args.Prompt) }); i >= 0 {
				id = prompts[i].Choice.Prompt
			}
			event.Choices = []domain.Answer{{Prompt: id, Picks: picks}}
		}
		if args.Ref != "" {
			found, err := a.resolve(ctx, s, cat, nil, args.Ref, nil, false)
			if err != nil {
				return nil, err
			}
			event.Ref, _ = rules.ParseRef(found.Ref)
			if strings.HasPrefix(args.Prompt, "character/") {
				event.Choices = nil
			}
		}
		log, dropped, err := Revise(s.Log, cat, args.Sequence, &event)
		if err != nil {
			return nil, err
		}
		s.Log = log
		return map[string]any{"dropped": dropped}, nil
	case "set_inventory":
		if len(args.Items) == 0 || len(args.Items) > 300 {
			return nil, fmt.Errorf("list 1-300 items as {name, count, placement}")
		}
		return a.setInventory(ctx, s, cat, args.Items), nil
	default:
		return nil, fmt.Errorf("unknown tool")
	}
}

// proficientFrom reads off which skills and saves a sheet is proficient in
// from the bonuses it prints.
//
// A bonus is the ability's modifier, plus the proficiency bonus once for a
// proficiency and twice for expertise. A sheet marks proficiency with a dot a
// model reads unreliably; the arithmetic it cannot get wrong, so it is done
// here and handed back as the list to build towards.
func proficientFrom(log domain.Log, cat *catalog.Catalog, bonuses map[string]int) (skills, expertise []rules.Slug, saves []string) {
	state, err := domain.Project(log, cat)
	if err != nil || state.Identity.DesiredLevel < 1 {
		return nil, nil, nil
	}
	proficiency := 2 + (state.Identity.DesiredLevel-1)/4
	if e := cat.Mechanics.Core.Proficiency; e.Op != "" {
		if n, err := e.Eval(rules.Variables{"level": state.Identity.DesiredLevel}); err == nil {
			proficiency = n
		}
	}
	for _, path := range slices.Sorted(maps.Keys(bonuses)) {
		parts := strings.Split(path, ".")
		switch parts[0] {
		case "skills":
			skill, ok := cat.Skills.Get(rules.Slug(parts[1]))
			if !ok {
				continue
			}
			switch over := bonuses[path] - state.Abilities.Modifier(skill.Ability); {
			case over >= 2*proficiency:
				expertise = append(expertise, skill.Slug)
				fallthrough
			case over >= proficiency:
				skills = append(skills, skill.Slug)
			}
		case "savingThrows":
			if ability, ok := rules.ParseAbility(parts[1]); ok && bonuses[path]-state.Abilities.Modifier(ability) >= proficiency {
				saves = append(saves, parts[1])
			}
		}
	}
	return skills, expertise, saves
}

func printedProficiencies(log domain.Log, cat *catalog.Catalog, bonuses map[string]int) map[string][]string {
	skills, expertise, saves := proficientFrom(log, cat, bonuses)
	named := func(slugs []rules.Slug) []string {
		out := []string{}
		for _, slug := range slugs {
			skill, _ := cat.Skills.Get(slug)
			out = append(out, skill.Name)
		}
		return out
	}
	return map[string][]string{"skills": named(skills), "expertise": named(expertise), "saves": append([]string{}, saves...)}
}

// promptSlot is a prompt with the places it has, what it would take, and what
// it has been given.
type promptSlot struct {
	prompt domain.Prompt
	offers map[rules.Slug]bool
	picks  []rules.Slug
}

// matchPicks puts each wanted option into a prompt that offers it, and returns
// the ones no prompt had room for.
//
// A sheet says what a character has, never which source gave what, and the
// sources overlap: one option is on two prompts' lists and another on only
// one. This is augmenting-path matching -- an option takes a free place in a
// prompt that offers it, or the place of one that can move somewhere else --
// so a prompt is not filled with options that had other homes while the one
// option only it offers is left out.
func matchPicks(slots []*promptSlot, wanted []rules.Slug) []rules.Slug {
	var place func(rules.Slug, map[*promptSlot]bool) bool
	place = func(option rules.Slug, seen map[*promptSlot]bool) bool {
		for _, slot := range slots {
			if !slot.offers[option] || seen[slot] {
				continue
			}
			seen[slot] = true
			if len(slot.picks) < slot.prompt.Choice.Choose {
				slot.picks = append(slot.picks, option)
				return true
			}
			for i, other := range slot.picks {
				if place(other, seen) {
					slot.picks[i] = option
					return true
				}
			}
		}
		return false
	}
	unplaced := []rules.Slug{}
	for _, option := range wanted {
		if !place(option, map[*promptSlot]bool{}) {
			unplaced = append(unplaced, option)
		}
	}
	return unplaced
}

var spellNameNoise = regexp.MustCompile(`\([^)]*\)|\[[^\]]*\]|^\s*\S+['’]s\s+`)

// bareSpellName is a printed spell name without what a sheet adds to it: a
// parenthesised note ("(R)", "(ritual)") and the leading owner's name the SRD
// drops ("Tasha's", "Melf's").
func bareSpellName(printed string) string {
	return strings.TrimSpace(spellNameNoise.ReplaceAllString(printed, " "))
}

// assignSpells distributes the sheet's cantrips and spells over the build's
// spell prompts -- the class's per-level picks, a racial cantrip -- and keeps
// what is left as spells known outside the build's count.
//
// It is the same puzzle as the skills. An Arcane Trickster's Mage Hand is
// already granted, its other two cantrips fill its own prompt, and the high
// elf's cantrip is then a question the sheet does not answer; a model left to
// it tries Mage Hand in each prompt in turn. Prompts that must be answered are
// filled first, larger ones before smaller, so the one left open is the one
// worth asking the owner about.
func (a *Agent) assignSpells(ctx context.Context, s *AgentSession, cat *catalog.Catalog, args agentArgs) (any, error) {
	names := s.spells
	if len(args.Spells) > 0 {
		names = args.Spells
	}
	if len(names) == 0 {
		// Not an error: a fighter's sheet lists none, and the procedure calls
		// this for every character.
		return map[string]any{"assigned": []any{}, "next": "The sheet lists no spells; nothing to assign. If it does list some, pass spells [names]."}, nil
	}
	// The assignment is done whole, so the spell picks made so far are taken
	// back first. See assignSkills for why they are removed outright.
	kept := slices.DeleteFunc(slices.Clone(s.Log.Events), func(e domain.Event) bool {
		return !e.Observed && len(e.Changes) == 0 && len(e.Choices) == 1 && strings.Contains(e.Choices[0].Prompt.String(), "/spell/")
	})
	log, err := domain.Rebuild(kept)
	if err != nil {
		return nil, err
	}
	if _, err := domain.Project(log, cat); err != nil {
		return nil, err
	}
	s.Log = log

	native := nativeLog(s.Log)
	state, err := domain.Project(native, cat)
	if err != nil {
		return nil, err
	}
	wanted, unknown := []rules.Slug{}, []map[string]any{}
	for _, printed := range names {
		// A sheet says "Tasha's Hideous Laughter" or "Detect Magic (R)" where
		// the rules say "Hideous Laughter" and "Detect Magic", so the name is
		// tried again without its owner and its notes. Nothing looser: every
		// spell there is is a candidate here, and "a name inside the printed
		// one" would turn Cause Fear into Fear.
		found, err := a.resolve(ctx, s, cat, []string{"spell"}, printed, nil, false)
		if bare := bareSpellName(printed); err != nil && bare != printed {
			found, err = a.resolve(ctx, s, cat, []string{"spell"}, bare, nil, false)
		}
		if err != nil {
			failure := map[string]any{}
			_ = json.Unmarshal(agentError(err), &failure)
			failure["name"] = printed
			unknown = append(unknown, failure)
			continue
		}
		if ref, _ := rules.ParseRef(found.Ref); !agentHolds(state, cat, ref) && !slices.Contains(wanted, ref.Slug) {
			wanted = append(wanted, ref.Slug)
		}
	}
	prompts, err := domain.Prompts(native, cat)
	if err != nil {
		return nil, err
	}
	offered := func(p domain.Prompt) map[rules.Slug]bool {
		offers := map[rules.Slug]bool{}
		for _, key := range rules.OptionKeys(cat.ResolveChoice(p.Choice).From) {
			if slices.Contains(wanted, key) && !slices.Contains(p.Held, key) {
				offers[key] = true
			}
		}
		return offers
	}
	slots := []*promptSlot{}
	for _, p := range prompts {
		if p.Choice.Kind == rules.ChooseSpell && p.Purpose != "custom" && !p.Optional {
			if offers := offered(p); len(offers) > 0 {
				slots = append(slots, &promptSlot{prompt: p, offers: offers})
			}
		}
	}
	// A class learns only so many spells of the highest level it can cast, and
	// that allowance is counted over all its prompts together. A sheet with
	// more -- a wizard's scribed scroll -- would have every prompt holding one
	// refused, so the surplus is kept out of the prompts and stays known.
	limits, err := domain.SpellRules(native, cat)
	if err != nil {
		return nil, err
	}
	for _, limit := range limits {
		if limit.MaxLevelCount == nil {
			continue
		}
		taken := 0
		for _, key := range wanted {
			if spell, ok := cat.Spells.Get(key); !ok || spell.Level != limit.MaxLevel {
				continue
			}
			counted := false
			for _, slot := range slots {
				if slot.prompt.Source != limit.Source || slot.prompt.Purpose != limit.Purpose || !slot.offers[key] {
					continue
				}
				if !counted {
					counted, taken = true, taken+1
				}
				if taken > *limit.MaxLevelCount {
					delete(slot.offers, key)
				}
			}
		}
	}
	slices.SortStableFunc(slots, func(x, y *promptSlot) int { return y.prompt.Choice.Choose - x.prompt.Choice.Choose })
	extra := matchPicks(slots, wanted)

	nameOf := catalogNames(cat)
	assigned, partial := []map[string]any{}, []map[string]any{}
	answer := func(p domain.Prompt, picks []rules.Slug) bool {
		names, keys := []string{}, []string{}
		for _, pick := range picks {
			names, keys = append(names, nameOf(rules.NewRef(rules.RefSpell, pick))), append(keys, pick.String())
		}
		entry := map[string]any{"prompt": localKey(cat, p.Choice.Prompt.String()), "picks": names}
		if len(picks) < p.Choice.Choose && !p.UpTo {
			entry["choose"] = p.Choice.Choose
			partial = append(partial, entry)
			return false
		}
		if _, err := a.answer(ctx, s, cat, agentArgs{Prompt: p.Choice.Prompt.String(), Picks: keys, Source: args.Source}); err != nil {
			_ = json.Unmarshal(agentError(err), &entry)
			partial = append(partial, entry)
			return false
		}
		assigned = append(assigned, entry)
		return true
	}
	for _, slot := range slots {
		// A prompt the sheet does not fill stays open, but the spells that
		// were headed for it are still the character's: they are kept as
		// known rather than dropped with the unanswered prompt.
		if !answer(slot.prompt, slot.picks) {
			extra = append(extra, slot.picks...)
		}
	}
	// A prompt that must be answered and that none of the sheet's remaining
	// spells fits is a question the sheet leaves open.
	for _, p := range prompts {
		if p.Choice.Kind == rules.ChooseSpell && p.Purpose != "custom" && !p.Optional && !slices.ContainsFunc(slots, func(slot *promptSlot) bool { return slot.prompt.Choice.Prompt == p.Choice.Prompt }) {
			partial = append(partial, map[string]any{"prompt": localKey(cat, p.Choice.Prompt.String()), "picks": []string{}, "choose": p.Choice.Choose})
		}
	}
	// Preparation is chosen from what is now known, and takes the sheet's
	// spells again rather than what is left of them.
	marked, unprepared := []rules.Slug{}, []string{}
	for _, printed := range s.prepared {
		if found, err := a.resolve(ctx, s, cat, []string{"spell"}, printed, nil, true); err == nil {
			ref, _ := rules.ParseRef(found.Ref)
			marked = append(marked, ref.Slug)
		}
	}
	if prompts, err = domain.Prompts(nativeLog(s.Log), cat); err != nil {
		return nil, err
	}
	for _, p := range prompts {
		if p.Choice.Kind != rules.ChooseSpell || p.Purpose == "custom" || !p.Optional || !p.UpTo {
			continue
		}
		picks := []rules.Slug{}
		for key := range offered(p) {
			picks = append(picks, key)
		}
		// The ones the sheet marks as prepared first; a sheet that marks none
		// gets the first few by name, which is a guess the owner can change.
		slices.SortFunc(picks, func(x, y rules.Slug) int {
			if px, py := slices.Contains(marked, x), slices.Contains(marked, y); px != py {
				if px {
					return -1
				}
				return 1
			}
			return strings.Compare(x.String(), y.String())
		})
		// Everything this prompt offers is already the character's to prepare
		// -- a cleric's whole list, a wizard's book -- so what does not fit is
		// simply not prepared, not an extra spell known from nowhere.
		extra = slices.DeleteFunc(extra, func(key rules.Slug) bool { return slices.Contains(picks, key) })
		for _, key := range picks[min(len(picks), p.Choice.Choose):] {
			unprepared = append(unprepared, nameOf(rules.NewRef(rules.RefSpell, key)))
		}
		if picks = picks[:min(len(picks), p.Choice.Choose)]; len(picks) > 0 {
			answer(p, picks)
		}
	}
	beyond := []string{}
	if len(extra) > 0 {
		refs := []string{}
		for _, key := range extra {
			refs = append(refs, rules.NewRef(rules.RefSpell, key).Canonical())
			beyond = append(beyond, nameOf(rules.NewRef(rules.RefSpell, key)))
		}
		if _, err := a.knowSpells(ctx, s, cat, refs); err != nil {
			return nil, err
		}
	}
	open, err := a.openPrompts(s, cat)
	if err != nil {
		return nil, err
	}
	return map[string]any{"assigned": assigned, "partial": partial, "beyond": beyond, "unprepared": unprepared, "unknown": unknown, "open": open, "next": "partial prompts are ones the sheet's spells do not fill: the build grants a spell the sheet does not list, so ask_user which, offering to leave it open; the picks shown there are kept as known meanwhile. beyond are spells the sheet lists past what the build's prompts take; they are kept as known. unprepared are spells on the class's list past its preparation limit: nothing to do. unknown are names the rules do not have or cannot tell apart: pass the right candidate's ref to assign_spells, or keep it with upsert_custom_option kind spell if none is it."}, nil
}

// assignSkills distributes the sheet's proficient skills over the prompts that
// grant skills -- the class's, the background's, the race's -- and its
// expertise over the prompts that double one.
//
// A sheet says which skills a character has, never which source gave which.
// Finding a legal split is a matching problem: Sleight of Hand is not on the
// sorcerer's list, so it must be the half-elf's, which leaves Insight for the
// sorcerer. A model given the same puzzle picks Intimidation to fill the slot.
// So the puzzle is solved here, whole, replacing any skill picks made so far.
func (a *Agent) assignSkills(ctx context.Context, s *AgentSession, cat *catalog.Catalog, args agentArgs) (any, error) {
	skills, expertise, _ := proficientFrom(s.Log, cat, s.printed)
	named := func(names []string) ([]rules.Slug, error) {
		out := []rules.Slug{}
		for _, name := range names {
			found, err := a.resolve(ctx, s, cat, []string{"skill"}, name, nil, false)
			if err != nil {
				return nil, err
			}
			ref, _ := rules.ParseRef(found.Ref)
			out = append(out, ref.Slug)
		}
		return out, nil
	}
	var err error
	if len(args.Proficient) > 0 {
		if skills, err = named(args.Proficient); err != nil {
			return nil, err
		}
	}
	if len(args.Expertise) > 0 {
		if expertise, err = named(args.Expertise); err != nil {
			return nil, err
		}
	}
	if len(skills) == 0 {
		return nil, fmt.Errorf("no proficient skills known: give plan_import the printed skill bonuses and level, or pass proficient [names]")
	}
	// A skill prompt offers proficiencies, each of which names its skill.
	proficiencyOf, skillOf := map[rules.Slug]rules.Slug{}, map[rules.Slug]rules.Slug{}
	for _, def := range cat.Proficiencies.All() {
		if def.Reference.Kind == rules.RefSkill {
			proficiencyOf[def.Reference.Slug], skillOf[def.Slug] = def.Slug, def.Reference.Slug
		}
	}
	// Take back the skill picks made so far. They are removed outright rather
	// than through Revise: nothing but another skill pick depends on one, and
	// Revise would re-judge every later answer against the sheet's printed
	// lists -- where a language the sheet prints is "already held" by the very
	// answer that chose it.
	kept := slices.DeleteFunc(slices.Clone(s.Log.Events), func(e domain.Event) bool {
		return !e.Observed && len(e.Changes) == 0 && len(e.Choices) == 1 && len(e.Choices[0].Picks) > 0 &&
			!slices.ContainsFunc(e.Choices[0].Picks, func(pick rules.Slug) bool { _, skill := skillOf[pick]; return !skill })
	})
	log, err := domain.Rebuild(kept)
	if err != nil {
		return nil, err
	}
	if _, err := domain.Project(log, cat); err != nil {
		return nil, err
	}
	s.Log = log

	native := nativeLog(s.Log)
	state, err := domain.Project(native, cat)
	if err != nil {
		return nil, err
	}
	prompts, err := domain.Prompts(native, cat)
	if err != nil {
		return nil, err
	}
	slots := []*promptSlot{}
	for _, p := range prompts {
		if p.HeldOnly || p.Choice.Kind != rules.ChooseProficiency {
			continue
		}
		offers := map[rules.Slug]bool{}
		for _, key := range rules.OptionKeys(cat.ResolveChoice(p.Choice).From) {
			if _, skill := skillOf[key]; skill && !slices.Contains(p.Held, key) {
				offers[key] = true
			}
		}
		if len(offers) > 0 {
			slots = append(slots, &promptSlot{prompt: p, offers: offers})
		}
	}
	name := func(proficiency rules.Slug) string {
		skill, _ := cat.Skills.Get(skillOf[proficiency])
		return skill.Name
	}
	wanted := []rules.Slug{}
	for _, skill := range skills {
		// A skill another source already grants needs no prompt.
		if state.Skills.BySkill[skill].Proficiency == rules.NotProficient {
			wanted = append(wanted, proficiencyOf[skill])
		}
	}
	unplaced := []string{}
	for _, proficiency := range matchPicks(slots, wanted) {
		unplaced = append(unplaced, name(proficiency))
	}
	assigned, partial := []map[string]any{}, []map[string]any{}
	answer := func(p domain.Prompt, picks []rules.Slug) {
		names, keys := []string{}, []string{}
		for _, pick := range picks {
			names, keys = append(names, name(pick)), append(keys, pick.String())
		}
		entry := map[string]any{"prompt": localKey(cat, p.Choice.Prompt.String()), "picks": names}
		if len(picks) < p.Choice.Choose {
			entry["choose"] = p.Choice.Choose
			partial = append(partial, entry)
			return
		}
		if _, err := a.answer(ctx, s, cat, agentArgs{Prompt: p.Choice.Prompt.String(), Picks: keys, Source: args.Source}); err != nil {
			_ = json.Unmarshal(agentError(err), &entry)
			partial = append(partial, entry)
			return
		}
		assigned = append(assigned, entry)
	}
	for _, sl := range slots {
		answer(sl.prompt, sl.picks)
	}
	// Expertise doubles a proficiency the character now holds, so its prompts
	// are read only after the proficiencies have landed.
	if len(expertise) > 0 {
		if prompts, err = domain.Prompts(nativeLog(s.Log), cat); err != nil {
			return nil, err
		}
		left := []rules.Slug{}
		for _, skill := range expertise {
			left = append(left, proficiencyOf[skill])
		}
		for _, p := range prompts {
			if !p.HeldOnly {
				continue
			}
			picks := []rules.Slug{}
			for _, key := range rules.OptionKeys(cat.ResolveChoice(p.Choice).From) {
				if len(picks) < p.Choice.Choose && slices.Contains(left, key) && slices.Contains(p.Held, key) {
					picks = append(picks, key)
				}
			}
			if len(picks) == 0 {
				continue
			}
			answer(p, picks)
			left = slices.DeleteFunc(left, func(key rules.Slug) bool { return slices.Contains(picks, key) })
		}
		for _, key := range left {
			unplaced = append(unplaced, name(key)+" (expertise)")
		}
	}
	open, err := a.openPrompts(s, cat)
	if err != nil {
		return nil, err
	}
	return map[string]any{"assigned": assigned, "partial": partial, "unplaced": unplaced, "open": open, "next": "partial prompts are ones the sheet's skills do not fill: complete them with answer_choices or ask_user. unplaced skills are ones no open prompt offers: the sheet has them from a source the build lacks."}, nil
}

// identityKinds are the things a sheet says about who the character is, by the
// word a model uses for them, and the path each is written at.
var identityKinds = map[string]string{"name": "identity.name", "alignment": "identity.alignment", "personality": "identity.personalityTraits", "personalitytraits": "identity.personalityTraits", "ideal": "identity.ideals", "ideals": "identity.ideals", "bond": "identity.bonds", "bonds": "identity.bonds", "flaw": "identity.flaws", "flaws": "identity.flaws"}

// promptPaths are the questions a build asks that a sheet answers with a
// number. A model reads the prompt's id and writes it back as a path, so the
// id is accepted as one.
var promptPaths = map[string]string{
	"character/desired-level": "identity.desiredLevel", "character/abilities": "finalAbilities",
	"character/personality-trait": "identity.personalityTraits", "character/ideal": "identity.ideals",
	"character/bond": "identity.bonds", "character/flaw": "identity.flaws", "character/alignment": "identity.alignment",
}

// setInventory puts a sheet's items on the character, by printed name.
func (a *Agent) setInventory(ctx context.Context, s *AgentSession, cat *catalog.Catalog, items []agentArgs) map[string]any {
	// A sheet's inventory is the whole inventory: the kit a class grants by
	// default is on it already, or was spent long ago.
	if len(s.Files) > 0 {
		for _, placement := range []string{"equipped", "backpack", "loot"} {
			s.Log = clearImportedInventory(s.Log, placement)
		}
	}
	applied, unmatched := []map[string]any{}, []map[string]any{}
	for _, item := range items {
		fact, found, err := a.inventoryFact(ctx, s, cat, item)
		if err == nil {
			_, err = a.importFact(ctx, s, cat, fact)
		}
		if err != nil {
			a.service.log.Warn("AI wizard tool rejected", "tool", "set_inventory", "item", item.Name, "error", err)
			failure := map[string]any{}
			_ = json.Unmarshal(agentError(err), &failure)
			failure["name"] = item.Name
			unmatched = append(unmatched, failure)
			continue
		}
		a.recordProgress(ctx, s, "set_inventory", agentArgs{Name: found.Name, Value: fact.Value}, nil)
		applied = append(applied, map[string]any{"name": item.Name, "ref": found.Ref, "count": json.RawMessage(fact.Value), "placement": strings.Split(fact.Path, ".")[1]})
	}
	return map[string]any{"applied": applied, "unmatched": unmatched, "next": "Resend an unmatched item by one of its candidate refs, or keep it with upsert_custom_option kind item, its count and placement."}
}

// inventoryFact turns one printed item into the stack it is: a catalogue item,
// how many, and where it is carried.
func (a *Agent) inventoryFact(ctx context.Context, s *AgentSession, cat *catalog.Catalog, item agentArgs) (agentArgs, AgentCandidate, error) {
	placement := item.Placement
	if placement == "" {
		placement = "backpack"
	}
	// A sheet says where a thing is in its own words. What is in hand or on
	// the body is equipped; anything else carried is in the pack.
	switch strings.ToLower(placement) {
	case "wielded", "held", "worn", "wearing", "in hand", "hands", "armor", "weapon":
		placement = "equipped"
	case "carried", "pack", "bag", "inventory", "stowed":
		placement = "backpack"
	}
	if !slices.Contains([]string{"equipped", "backpack", "loot"}, placement) {
		return agentArgs{}, AgentCandidate{}, fmt.Errorf("placement is equipped, backpack or loot")
	}
	// A sheet prints "10 Rations" or "Dagger x2", and a model copies the cell
	// whole. The number is the count unless one was given apart from it.
	name, count := item.Name, item.Count
	if match := countedName.FindStringSubmatch(strings.TrimSpace(name)); match != nil && count <= 1 {
		digits := match[1] + match[4]
		name = match[2] + match[3]
		count, _ = strconv.Atoi(digits)
	}
	if count == 0 {
		count = 1
	}
	if count < 1 || count > 100000 {
		return agentArgs{}, AgentCandidate{}, fmt.Errorf("invalid inventory quantity")
	}
	text := name
	if item.Ref != "" {
		text = item.Ref
	}
	found, err := a.resolve(ctx, s, cat, []string{"item", "magic-item"}, text, nil, false)
	if err != nil {
		return agentArgs{}, AgentCandidate{}, err
	}
	ref, _ := rules.ParseRef(found.Ref)
	return agentArgs{Path: "equipment." + placement + "." + ref.Slug.String(), Value: json.RawMessage(strconv.Itoa(count)), Source: item.Source}, found, nil
}

// importFact writes one thing the sheet states: a catalogue entry the
// character is made of, or a printed value at a path.
func (a *Agent) importFact(ctx context.Context, s *AgentSession, cat *catalog.Catalog, args agentArgs) (map[string]any, error) {
	// A model that has just written {kind: "race", name: "Elf"} writes the
	// character's name the same way. It is a value at a path all the same.
	if path := identityKinds[strings.ToLower(args.Kind)]; path != "" && args.Path == "" && args.Ref == "" && args.Name != "" {
		args.Path, args.Value, args.Kind = path, raw(args.Name), ""
	}
	entity := args.Ref != "" || args.Path == "" && args.Kind != "" && args.Name != ""
	if !entity && (args.Path == "" || len(args.Value) == 0) {
		return nil, fmt.Errorf("provide kind plus name (and level for a class) OR path plus value; batches use the facts array")
	}
	var (
		log    domain.Log
		result map[string]any
		err    error
	)
	if entity {
		log, result, err = a.entityFact(ctx, s, cat, args)
	} else {
		log, result, err = a.valueFact(ctx, s, cat, args)
	}
	if err != nil {
		return nil, err
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
	return result, nil
}

// entityScope narrows a name to the entries it can mean on this character. A
// subclass is one of its class's, a subrace one of its race's -- which is what
// makes "Wild Magic" the sorcerer's origin rather than the barbarian's path,
// and what makes the looser match safe.
func entityScope(state domain.State, cat *catalog.Catalog, kind string) (func(rules.Ref) bool, bool) {
	switch kind {
	case "subclass":
		if len(state.Identity.Classes) == 0 {
			return nil, true
		}
		return func(ref rules.Ref) bool {
			sub, ok := cat.Subclasses.Get(ref.Slug)
			return ok && slices.ContainsFunc(state.Identity.Classes, func(c domain.ClassLevel) bool { return c.Class == sub.Class })
		}, true
	case "subrace":
		if state.Identity.Race.IsZero() {
			return nil, true
		}
		return func(ref rules.Ref) bool {
			sub, ok := cat.Subraces.Get(ref.Slug)
			return ok && sub.Race == state.Identity.Race
		}, true
	case "race", "class", "background":
		return nil, true
	}
	return nil, false
}

// entityKind reads the kind a fact names, from its ref when it has one.
func entityKind(args agentArgs) (kind, text string) {
	kind, text = args.Kind, args.Name
	if args.Ref != "" {
		text = args.Ref
		if ref, ok := rules.ParseRef(args.Ref); ok {
			kind = ref.Kind.String()
		} else if prefix, _, cut := strings.Cut(args.Ref, ":"); cut {
			kind = prefix
		}
	}
	if mapped, ok := naturalKinds[kind]; ok {
		kind = mapped
	}
	return kind, text
}

func (a *Agent) entityFact(ctx context.Context, s *AgentSession, cat *catalog.Catalog, args agentArgs) (domain.Log, map[string]any, error) {
	state, err := domain.Project(s.Log, cat)
	if err != nil {
		return domain.Log{}, nil, err
	}
	kind, text := entityKind(args)
	refKind, _ := rules.ParseRefKind(kind)
	typ, ok := structuralEvents[refKind]
	if !ok {
		return domain.Log{}, nil, fmt.Errorf("%q is not a kind import_facts takes by name. By name it takes race, subrace, class, subclass, background and feat. The character's name, alignment, personality traits, ideals, bonds and flaws are values at a path: {\"path\":\"identity.name\",\"value\":...}, identity.alignment, identity.personalityTraits, identity.ideals, identity.bonds, identity.flaws. Spells and options are picked with answer_choices, items go through set_inventory, and racial traits or class features the build grants need nothing", kind)
	}
	scope, loose := entityScope(state, cat, kind)
	found, err := a.resolve(ctx, s, cat, []string{kind}, text, scope, loose)
	if err != nil {
		return domain.Log{}, nil, err
	}
	ref, _ := rules.ParseRef(found.Ref)
	level := 0
	if args.Level != nil {
		level = *args.Level
		if level < 1 || level > maxLevel(cat) {
			return domain.Log{}, nil, fmt.Errorf("invalid level")
		}
	}
	// An entry's level is the level its decision belongs to, which is how the
	// builder files it: a class is chosen at 1 and its subclass when it falls
	// due. The level printed beside the class is the level the character is
	// built towards, a question of its own, answered below. Only a second
	// class needs levels of its own, because one desired level cannot say how
	// they are shared out.
	printed := level
	switch typ {
	case domain.EventClass:
		if args.Level != nil {
			if s.classLevels == nil {
				s.classLevels = map[rules.Slug]int{}
			}
			s.classLevels[ref.Slug] = printed
		}
		if _, only := s.classLevels[ref.Slug]; len(s.classLevels) == 0 || len(s.classLevels) == 1 && only {
			level = 1
		}
	case domain.EventSubclass:
		if sub, ok := cat.Subclasses.Get(ref.Slug); ok {
			if class, ok := cat.Classes.Get(sub.Class); ok {
				level = domain.SubclassLevel(cat, class)
			}
		}
	}
	log := s.Log.Clone()
	if typ == domain.EventClass && len(s.classLevels) > 1 {
		for i, e := range log.Events {
			if printed, ok := s.classLevels[e.Ref.Slug]; ok && e.Type == domain.EventClass && e.Observed {
				log.Events[i].Level = printed
			}
		}
		level = printed
	}
	// Structural observations replace their previous observation instead of
	// appending another class grant each time reconciliation is repeated.
	replaced := false
	for i, e := range log.Events {
		if e.Type != typ || typ == domain.EventClass && e.Ref != ref {
			continue
		}
		replaced = true
		if e.Ref != ref {
			// Answers given to the old entry name it, and would replay as
			// selections of it. Revise judges them against the new one.
			log, _, err = Revise(s.Log, cat, e.Seq, &domain.Event{Type: typ, Ref: ref, Level: level, Source: observedGroup(typ, "")})
			if err != nil {
				return domain.Log{}, nil, err
			}
			break
		}
		// A repeated "Sorcerer" without its level is the same sorcerer.
		if args.Level != nil || typ == domain.EventSubclass {
			log.Events[i].Level = level
		}
		log.Events[i].Observed = true
		log.Events[i].Evidence = args.Source
		log.Events[i].Source = observedGroup(typ, "")
		break
	}
	if !replaced {
		_ = log.Append(domain.Event{Type: typ, Ref: ref, Level: level, Evidence: args.Source, Observed: true, Source: observedGroup(typ, "")})
	}
	if typ == domain.EventRace && found.Score < .99 {
		// "High Elf" names the race and its subrace at once. The race matched
		// loosely; if the same words are exactly one of its subraces, the sheet
		// has said both.
		name := text
		if _, rest, cut := strings.Cut(text, ":"); cut && strings.Count(text, ":") == 1 {
			name = rest
		}
		of := func(candidate rules.Ref) bool {
			sub, ok := cat.Subraces.Get(candidate.Slug)
			return ok && sub.Race == ref.Slug
		}
		if sub, err := a.resolve(ctx, s, cat, []string{"subrace"}, name, of, false); err == nil && sub.Score >= .99 {
			subrace, _ := rules.ParseRef(sub.Ref)
			log.Events = slices.DeleteFunc(log.Events, func(e domain.Event) bool { return e.Type == domain.EventSubrace && e.Observed })
			if log, err = domain.Rebuild(log.Events); err != nil {
				return domain.Log{}, nil, err
			}
			_ = log.Append(domain.Event{Type: domain.EventSubrace, Ref: subrace, Evidence: args.Source, Observed: true, Source: observedGroup(domain.EventSubrace, "")})
		}
	}
	if typ == domain.EventClass && args.Level != nil {
		// The level a character is built towards is a question of its own, and
		// a sheet printing "Sorcerer 3" has answered it. A class named without
		// its level has not: closing the question at level 1 on its behalf is
		// how a third-level sheet was imported as a first-level character.
		total := 0
		for _, held := range s.classLevels {
			total += held
		}
		if projected, err := domain.Project(log, cat); err == nil && projected.Identity.DesiredLevel < total {
			log = setNative(log, domain.Change{Path: "identity.desiredLevel", Op: domain.OpSet, Value: domain.IntValue(total)}, args.Source)
		}
	}
	return log, map[string]any{"applied": true, "ref": found.Ref, "name": found.Name}, nil
}

// factPath rewrites a path the way a model writes it -- skills.stealth.bonus,
// savingThrows.dexterity.bonus, equipment.backpack.Dagger -- into the one the
// pinned catalogue addresses, where a pack's entries carry its namespace.
func (a *Agent) factPath(ctx context.Context, s *AgentSession, cat *catalog.Catalog, path string) (string, error) {
	for prompt, real := range promptPaths {
		if rest, ok := strings.CutPrefix(path, prompt); ok && (rest == "" || strings.HasPrefix(rest, ".")) {
			path = real + rest
		}
	}
	parts := strings.SplitN(path, ".", 3)
	if len(parts) < 2 {
		return path, nil
	}
	switch parts[0] {
	case "skills":
		found, err := a.resolve(ctx, s, cat, []string{"skill"}, parts[1], nil, false)
		if err != nil {
			return "", err
		}
		ref, _ := rules.ParseRef(found.Ref)
		parts[1] = ref.Slug.String()
	case "savingThrows", "finalAbilities":
		if code := abilityNames[strings.ToLower(parts[1])]; code != "" {
			parts[1] = code
		}
	case "equipment":
		if len(parts) == 3 && parts[1] != "purse" {
			found, err := a.resolve(ctx, s, cat, []string{"item", "magic-item"}, parts[2], nil, false)
			if err != nil {
				return "", err
			}
			ref, _ := rules.ParseRef(found.Ref)
			parts[2] = ref.Slug.String()
		}
	}
	return strings.Join(parts, "."), nil
}

// factLists are the paths whose value is a list of catalogue entries, by the
// kind of entry. A sheet's list is added to what the build grants rather than
// replacing it, so the override it leaves is only what the build lacks.
// formLabels are the captions a character sheet prints on its name boxes.
var formLabels = map[string]bool{"character name": true, "player name": true, "name": true, "имя персонажа": true, "имя игрока": true, "имя": true}

var factLists = map[string]string{"base.languages": "language", "proficiencies": "proficiency"}

func (a *Agent) valueFact(ctx context.Context, s *AgentSession, cat *catalog.Catalog, args agentArgs) (domain.Log, map[string]any, error) {
	path, err := a.factPath(ctx, s, cat, args.Path)
	if err != nil {
		return domain.Log{}, nil, err
	}
	v, err := agentValue(args.Value)
	if err != nil {
		return domain.Log{}, nil, err
	}
	if !knownFactPath(cat, path) {
		return domain.Log{}, nil, fmt.Errorf("unsupported fact path %q; supported: identity.name|alignment|personalityTraits|ideals|bonds|flaws, finalAbilities.<ability>, base.hitPoints.max|current|temporary, base.speed, base.size, base.senses.darkvision, base.languages, status.armorClass|initiative|passivePerception, skills.<skill>.proficiency|bonus, savingThrows.<ability>.proficient|bonus, proficiencies, equipment.purse.<coin>. Anything else is kept with upsert_custom_option kind note", args.Path)
	}
	op := domain.OpSet
	switch {
	case strings.HasPrefix(path, "equipment.") && v.Kind == domain.ValueSlugList && len(v.Slugs) > 0:
		return domain.Log{}, nil, fmt.Errorf("list items with set_inventory {items:[{name, count, placement}]}, which resolves their names")
	case path == "base.size" && v.Kind == domain.ValueString:
		v.Str = strings.ToLower(strings.TrimSpace(v.Str))
	case path == "identity.alignment" && v.Kind == domain.ValueString:
		found, err := a.resolve(ctx, s, cat, []string{"alignment"}, v.Str, nil, false)
		if err != nil {
			return domain.Log{}, nil, err
		}
		ref, _ := rules.ParseRef(found.Ref)
		v = domain.SlugValue(ref.Slug)
	case factLists[path] != "":
		if v.Kind == domain.ValueString {
			v = domain.SlugListValue([]rules.Slug{rules.Slug(v.Str)})
		}
		for i, entry := range v.Slugs {
			found, err := a.resolve(ctx, s, cat, []string{factLists[path]}, entry.String(), nil, false)
			if err != nil {
				return domain.Log{}, nil, err
			}
			ref, _ := rules.ParseRef(found.Ref)
			v.Slugs[i] = ref.Slug
		}
		op = domain.OpAdd
	}
	log := s.Log.Clone()
	segments := strings.Split(path, ".")
	if len(s.Files) > 0 && len(segments) == 3 && segments[0] == "equipment" && segments[1] != "purse" {
		log = clearImportedInventory(log, segments[1])
	}
	// A blank is not an answer. Written, it closes the question with nothing
	// in it, and the builder then shows a field somebody "filled in".
	if strings.HasPrefix(path, "identity.") && v.Kind == domain.ValueString && (strings.TrimSpace(v.Str) == "" || strings.TrimSpace(v.Str) == "…") {
		return domain.Log{}, nil, fmt.Errorf("%s has no value to write. Leave it unwritten if the user wants it blank; if it is needed, ask_user for it", path)
	}
	// A blank name box still prints its caption, and that is what gets read.
	if path == "identity.name" && formLabels[strings.ToLower(strings.TrimSpace(v.Str))] {
		return domain.Log{}, nil, fmt.Errorf("%q is the caption of the name box, not a name: the box is blank, so leave the name unwritten", v.Str)
	}
	change := domain.Change{Path: domain.Path(path), Op: op, Value: v}
	if strings.HasPrefix(path, "identity.") {
		return setNative(log, change, args.Source), map[string]any{"applied": true, "path": path}, nil
	}
	return setObserved(log, change, args.Source), map[string]any{"applied": true, "path": path}, nil
}

// setNative writes a fact the builder asks for itself -- the name, the level,
// the personality, the alignment -- as the builder's own answer to it: an
// ordinary entry, not a printed value laid over the build. It rewrites the
// entry that last set the path, so the fact stays one card in the builder
// however many times an import states it.
func setNative(log domain.Log, change domain.Change, source string) domain.Log {
	for i := len(log.Events) - 1; i >= 0; i-- {
		for j, ch := range log.Events[i].Changes {
			if ch.Path == change.Path {
				log.Events[i].Changes[j] = change
				log.Events[i].Observed = false
				return log
			}
		}
	}
	group := domain.GroupPersonality
	if change.Path == "identity.desiredLevel" || change.Path == "identity.name" {
		group = domain.GroupIdentity
	}
	_ = log.Append(domain.Event{Type: domain.EventChange, Changes: []domain.Change{change}, Evidence: source, Source: group})
	return log
}

// setObserved writes a printed value, replacing what the import said about
// that path before and leaving later direct player edits intact.
func setObserved(log domain.Log, change domain.Change, source string) domain.Log {
	for i, e := range log.Events {
		if e.Observed && e.Type == domain.EventChange && len(e.Changes) == 1 && e.Changes[0].Path == change.Path {
			log.Events[i].Changes = []domain.Change{change}
			log.Events[i].Evidence = source
			return log
		}
	}
	_ = log.Append(domain.Event{Type: domain.EventChange, Changes: []domain.Change{change}, Evidence: source, Observed: true, Source: observedGroup(domain.EventChange, string(change.Path))})
	return log
}

// answer applies one answer through the ordinary choice validator, and returns
// the option keys it settled on.
//
// The prompt is judged against the build alone. A sheet's printed skills are
// often written before its choices are answered, and a prompt computed on top
// of them would call every pick "already held".
func (a *Agent) answer(ctx context.Context, s *AgentSession, cat *catalog.Catalog, args agentArgs) ([]rules.Slug, error) {
	if strings.HasPrefix(args.Prompt, "custom/spell/") {
		return a.knowSpells(ctx, s, cat, args.Picks)
	}
	native := nativeLog(s.Log)
	prompts, err := domain.Prompts(native, cat)
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(prompts, func(p domain.Prompt) bool { return promptNamed(cat, p, args.Prompt) })
	if i < 0 {
		return nil, fmt.Errorf("prompt %q is not open; the open prompts are listed in this result", args.Prompt)
	}
	p := prompts[i]
	picks := args.Picks
	if len(picks) == 0 && args.Ref != "" {
		picks = []string{args.Ref}
	}
	if kind, structural := structuralPrompt(p); structural {
		// "Which subclass?" is settled by naming the entry, not by a pick.
		if len(picks) != 1 {
			return nil, fmt.Errorf("name exactly one %s", kind)
		}
		fact := agentArgs{Kind: kind, Name: picks[0], Source: args.Source}
		result, err := a.importFact(ctx, s, cat, fact)
		if err != nil {
			return nil, err
		}
		a.recordProgress(ctx, s, "resolve_import_facts", fact, raw(result))
		ref, _ := rules.ParseRef(result["ref"].(string))
		return []rules.Slug{ref.Slug}, nil
	}
	// "Choose four" answered with three is not an answer: the build reopens
	// the question and the three are left in the log doing nothing.
	if len(picks) < p.Choice.Choose && !p.UpTo {
		return nil, fmt.Errorf("%s takes exactly %d picks; got %d", args.Prompt, p.Choice.Choose, len(picks))
	}
	choice := cat.ResolveChoice(p.Choice)
	keys := make([]rules.Slug, 0, len(picks))
	for _, pick := range picks {
		key, err := a.resolvePick(ctx, s, cat, choice, pick)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	// A feat the sheet names is first written down as a fact, before the
	// question that grants it is open. Answering that question with it is the
	// same feat finding its place, not a second copy: the fact gives way, or
	// the pick is refused as already held and a model goes off to choose a
	// different feat nobody asked for.
	log := s.Log.Clone()
	stated := func(e domain.Event) bool {
		return e.Observed && e.Type == domain.EventFeat && len(e.Choices) == 0 && slices.Contains(keys, e.Ref.Slug)
	}
	if slices.ContainsFunc(log.Events, stated) {
		if log, err = domain.Rebuild(slices.DeleteFunc(log.Events, stated)); err != nil {
			return nil, err
		}
		native = nativeLog(log)
	}
	events := []domain.Event{{Type: p.Event.Type, Ref: p.Event.Ref, Level: p.Event.Level, Choices: []domain.Answer{{Prompt: p.Choice.Prompt, Picks: keys}}}}
	if err := validateAndAttribute(native, cat, events); err != nil {
		return nil, err
	}
	if err := log.Append(events...); err != nil {
		return nil, err
	}
	if _, err := domain.Project(log, cat); err != nil {
		return nil, err
	}
	s.Log = log
	named := []string{}
	for _, key := range keys {
		named = append(named, key.String())
	}
	a.recordProgress(ctx, s, "answer_choice", agentArgs{Prompt: args.Prompt, Picks: named}, nil)
	return keys, nil
}

// knowSpells puts catalogue spells on the character outside any class's or
// race's count: a sheet that lists five spells for a caster who learns three.
//
// The builder already has a place for exactly this -- its custom-source
// prompts, one for cantrips and one for levelled spells -- so the spell stays
// the catalogue's and is an ordinary answer, not a custom entry. The prompt is
// answered once and then closed, so a later spell extends that answer rather
// than being refused for want of an open question.
func (a *Agent) knowSpells(ctx context.Context, s *AgentSession, cat *catalog.Catalog, picks []string) ([]rules.Slug, error) {
	if len(picks) == 0 {
		return nil, fmt.Errorf("name the spells to add")
	}
	state, err := domain.Project(nativeLog(s.Log), cat)
	if err != nil {
		return nil, err
	}
	added, held := []rules.Slug{}, []string{}
	log := s.Log.Clone()
	for _, pick := range picks {
		found, err := a.resolve(ctx, s, cat, []string{"spell"}, pick, nil, false)
		if err != nil {
			return nil, err
		}
		ref, _ := rules.ParseRef(found.Ref)
		// The custom source is for what nothing else grants. A tiefling's
		// Thaumaturgy is already on the sheet, from the tiefling.
		if agentHolds(state, cat, ref) {
			held = append(held, found.Name)
			continue
		}
		prompt := rules.Slug("custom/spell/known")
		if spell, _ := cat.Spells.Get(ref.Slug); spell.Level == 0 {
			prompt = "custom/spell/cantrip"
		}
		i := slices.IndexFunc(log.Events, func(e domain.Event) bool {
			return !e.Observed && len(e.Choices) == 1 && e.Choices[0].Prompt == prompt
		})
		if i < 0 {
			if err := log.Append(domain.Event{Type: domain.EventChange, Source: domain.GroupClass, Choices: []domain.Answer{{Prompt: prompt}}}); err != nil {
				return nil, err
			}
			i = len(log.Events) - 1
		}
		if !slices.Contains(log.Events[i].Choices[0].Picks, ref.Slug) {
			log.Events[i].Choices[0].Picks = append(slices.Clone(log.Events[i].Choices[0].Picks), ref.Slug)
		}
		added = append(added, ref.Slug)
	}
	// One spell the build already grants is no reason to refuse the rest.
	if len(added) == 0 {
		return nil, fmt.Errorf("%s is already granted by the build; it needs no extra entry", strings.Join(held, ", "))
	}
	if err := validateImported(cat, log); err != nil {
		return nil, err
	}
	s.Log = log
	named := []string{}
	for _, key := range added {
		named = append(named, key.String())
	}
	a.recordProgress(ctx, s, "answer_choice", agentArgs{Picks: named}, nil)
	return added, nil
}

// promptNamed matches a prompt by its id, with or without the pack namespace
// a class slug carries into it.
func promptNamed(cat *catalog.Catalog, p domain.Prompt, id string) bool {
	return p.Choice.Prompt.String() == id || localKey(cat, p.Choice.Prompt.String()) == id
}

// structuralPrompt reports that a prompt asks which catalogue entry the
// character is, and of what kind.
func structuralPrompt(p domain.Prompt) (string, bool) {
	if !p.Event.Ref.IsZero() {
		return "", false
	}
	for kind, typ := range structuralEvents {
		if typ == p.Event.Type {
			return kind.String(), true
		}
	}
	return "", false
}

// resolvePick turns one pick as a model writes it -- an option key, a local
// slug, a printed name -- into the key of one of the prompt's own options.
func (a *Agent) resolvePick(ctx context.Context, s *AgentSession, cat *catalog.Catalog, choice rules.Choice, pick string) (rules.Slug, error) {
	pick = strings.TrimSpace(pick)
	offered := map[rules.Ref]bool{}
	kinds := []string{}
	for _, option := range choice.From.Options {
		key := rules.OptionKey(option)
		if key.String() == pick || strings.EqualFold(localKey(cat, key.String()), pick) {
			return key, nil
		}
		switch o := option.(type) {
		case rules.RefOption:
			offered[o.Ref] = true
			if !slices.Contains(kinds, o.Ref.Kind.String()) {
				kinds = append(kinds, o.Ref.Kind.String())
			}
		case rules.AbilityBonusOption:
			if abilityNames[strings.ToLower(pick)] == o.Ability.String() {
				return key, nil
			}
		}
	}
	keep := func(ref rules.Ref) bool { return offered[ref] }
	if choice.From.Kind != rules.OptionsExplicit {
		kinds, keep = []string{choice.From.Collection.String()}, nil
	}
	if len(kinds) == 0 {
		return "", fmt.Errorf("%q is not an option of %s; use one of its option keys", pick, choice.Prompt)
	}
	found, err := a.resolve(ctx, s, cat, kinds, pick, keep, true)
	if err != nil {
		// "No spell named Bless" sends a model looking for another spelling.
		// When the name is a real entry this prompt just does not offer, say
		// that instead: it is already granted, or not on this list.
		if keep != nil {
			if other, otherErr := a.resolve(ctx, s, cat, kinds, pick, nil, true); otherErr == nil {
				return "", fmt.Errorf("%s is not an option of %s: the build already grants it, or this prompt's list does not have it. Do not retry it here", other.Name, choice.Prompt)
			}
		}
		return "", err
	}
	ref, _ := rules.ParseRef(found.Ref)
	return ref.Slug, nil
}

// customOption keeps content the selected rules do not have -- and refuses to
// make a custom copy of content they do.
//
// A sheet prints everything a character has, most of which the build already
// grants. Each of those arriving here would otherwise become a custom entry
// shadowing the real one, which is how an imported rogue ended up with a
// custom Sneak Attack beside the catalogue's.
func (a *Agent) customOption(ctx context.Context, s *AgentSession, cat *catalog.Catalog, args agentArgs) (any, error) {
	state, err := domain.Project(s.Log, cat)
	if err != nil {
		return nil, err
	}
	refKind, _ := rules.ParseRefKind(args.Kind)
	_, structural := structuralEvents[refKind]
	selected := args.Selected == nil || *args.Selected
	if lookup := lookupKinds(args.Kind); args.Kind != "note" && catalogCandidates(cat, lookup[0]) != nil {
		var found AgentCandidate
		known := false
		if args.Ref != "" {
			// A ref the model wrote names something or it is a mistake. Dropping
			// it in silence is how a known spell became a nameless custom one.
			if found, err = a.resolve(ctx, s, cat, lookup, args.Ref, nil, false); err != nil {
				return nil, err
			}
			known = true
		} else if strings.TrimSpace(args.Name) != "" {
			found, known = a.identify(ctx, s, cat, state, args.Kind, args.Name)
		}
		if known && selected {
			ref, _ := rules.ParseRef(found.Ref)
			native := map[string]any{"native": true, "ref": found.Ref, "name": found.Name}
			done := func(reason string) (any, error) {
				s.expected = slices.DeleteFunc(s.expected, func(key string) bool { return key == "custom:"+args.ID })
				native["reason"] = reason
				return native, nil
			}
			switch {
			case agentHolds(state, cat, ref):
				return done("already part of the build; nothing to add")
			case structural:
				fact := agentArgs{Ref: found.Ref, Source: args.Source}
				if args.Kind == "class" {
					fact.Level = args.Level
				}
				result, err := a.importFact(ctx, s, cat, fact)
				if err != nil {
					return nil, err
				}
				a.recordProgress(ctx, s, "resolve_import_facts", fact, raw(result))
				return done("the selected rules have it; imported as the catalogue entry")
			case ref.Kind == rules.RefSpell:
				if _, err := a.knowSpells(ctx, s, cat, []string{found.Ref}); err != nil {
					return nil, err
				}
				return done("the selected rules have it; added as a spell known outside the class's count")
			case ref.Kind == rules.RefItem || ref.Kind == rules.RefMagicItem:
				fact, _, err := a.inventoryFact(ctx, s, cat, agentArgs{Ref: found.Ref, Count: args.Count, Placement: args.Placement, Source: args.Source})
				if err == nil {
					_, err = a.importFact(ctx, s, cat, fact)
				}
				if err != nil {
					return nil, err
				}
				a.recordProgress(ctx, s, "set_inventory", agentArgs{Name: found.Name, Value: fact.Value}, nil)
				return done("the selected rules have it; added to the inventory as the catalogue item")
			}
		}
		// A catalogue entry the character has although nothing offers it -- a
		// spell beyond the class's count, a feature from a level not reached --
		// keeps its identity and is recorded as this character's exception.
		args.Ref = ""
		if known {
			args.Ref = found.Ref
			if ref, _ := rules.ParseRef(found.Ref); args.Kind == "spell" && ref.Kind == rules.RefSpell {
				if spell, ok := cat.Spells.Get(ref.Slug); ok && spell.Level == 0 {
					args.Kind = "cantrip"
				}
			}
		}
	}

	// A custom entry is something the rules lack that a character is built
	// from: a class, a race, a background, a spell, an item. A feature, a
	// trait, a feat or a note the catalogue does not know is not that -- it is
	// what a model writes to quiet a checklist or to record that a field was
	// left blank, and the owner finds it later as junk on the sheet.
	if slices.Contains([]string{"feature", "trait", "feat", "note"}, args.Kind) && args.Ref == "" {
		s.expected = slices.DeleteFunc(s.expected, func(key string) bool { return key == "custom:"+args.ID })
		return map[string]any{"kept": false, "reason": "Not kept: custom entries are only for a class, subclass, race, subrace, background, cantrip, spell or item the selected rules lack. A feature or trait the build grants needs nothing; anything else worth telling the user goes in your message, not on the character."}, nil
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
		for _, taken := range state.Identity.Classes {
			if taken.Class == ref.Slug {
				level := taken.Level
				args.Level = &level
			}
		}
	}
	if args.Parent != "" {
		kind := "class"
		if args.Kind == "subrace" {
			kind = "race"
		}
		// Loose, because a parent is one of a handful of entries and a model
		// writes it as the sheet does: "Sorcerer", not a slug.
		parent, err := a.resolve(ctx, s, cat, []string{kind}, args.Parent, nil, true)
		if err != nil {
			return nil, err
		}
		ref, _ := rules.ParseRef(parent.Ref)
		args.Parent = ref.Slug.String()
	}
	customLog := s.Log
	if len(s.Files) > 0 && args.Kind == "item" && (args.Selected == nil || *args.Selected) {
		customLog = clearImportedInventory(customLog, args.Placement)
	}
	option := domain.CustomOption{ID: args.ID, Kind: args.Kind, Name: args.Name, Description: args.Description, Source: args.Source, Parent: args.Parent, Ability: args.Ability, Mode: args.Mode, Placement: args.Placement, Level: args.Level, HitDie: args.HitDie, Speed: args.Speed, Count: args.Count, Selected: selected, Reference: args.Ref}
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
}

// lookupKinds are the collections a sheet's word for a kind can mean. A sheet
// does not distinguish a class feature from a racial trait, or a cantrip from
// any other spell.
func lookupKinds(kind string) []string {
	if kinds := map[string][]string{"cantrip": {"spell"}, "feature": {"feature", "trait"}, "trait": {"trait", "feature"}, "item": {"item", "magic-item"}}[kind]; kinds != nil {
		return kinds
	}
	if mapped, ok := naturalKinds[kind]; ok {
		return lookupKinds(mapped)
	}
	return []string{kind}
}

// identify finds the catalogue entry a printed name means on this character:
// first among what the build already holds, then within the selected rules.
//
// What the build holds is a small closed set, so a feature printed with its
// dice -- "Sneak Attack (2d6)" -- still finds the feature. Items and spells
// are held to an exact name everywhere: "Silver Dagger" is not the dagger the
// character carries.
func (a *Agent) identify(ctx context.Context, s *AgentSession, cat *catalog.Catalog, state domain.State, kind, name string) (AgentCandidate, bool) {
	lookup := lookupKinds(kind)
	refKind, _ := rules.ParseRefKind(lookup[0])
	_, structural := structuralEvents[refKind]
	decorated := lookup[0] == "feature" || lookup[0] == "trait"
	held := func(ref rules.Ref) bool { return agentHolds(state, cat, ref) }
	if found, err := a.resolve(ctx, s, cat, lookup, name, held, decorated || structural); err == nil && (decorated || structural || found.Score >= .99) {
		return found, true
	}
	scope, loose := entityScope(state, cat, lookup[0])
	if found, err := a.resolve(ctx, s, cat, lookup, name, scope, loose); err == nil && (structural || found.Score >= .99) {
		return found, true
	}
	return AgentCandidate{}, false
}

// agentHolds reports whether the character as built already has an entry,
// whatever granted it.
func agentHolds(state domain.State, cat *catalog.Catalog, ref rules.Ref) bool {
	in := func(list []rules.Slug) bool { return slices.Contains(list, ref.Slug) }
	stacked := func(stacks []domain.ItemStack) bool {
		return slices.ContainsFunc(stacks, func(stack domain.ItemStack) bool { return stack.Item == ref.Slug && stack.Count > 0 })
	}
	skilled := func(skill rules.Slug) bool { return state.Skills.BySkill[skill].Proficiency != rules.NotProficient }
	switch ref.Kind {
	case rules.RefRace:
		return state.Identity.Race == ref.Slug
	case rules.RefSubrace:
		return state.Identity.Subrace == ref.Slug
	case rules.RefBackground:
		return state.Identity.Background == ref.Slug
	case rules.RefAlignment:
		return state.Identity.Alignment == ref.Slug
	case rules.RefClass:
		return slices.ContainsFunc(state.Identity.Classes, func(c domain.ClassLevel) bool { return c.Class == ref.Slug })
	case rules.RefSubclass:
		return slices.ContainsFunc(state.Identity.Classes, func(c domain.ClassLevel) bool { return c.Subclass == ref.Slug })
	case rules.RefFeat:
		return in(state.Feats)
	case rules.RefFeature:
		return in(state.Features)
	case rules.RefTrait:
		return in(state.Traits)
	case rules.RefSpell:
		return in(state.Spells.Cantrips) || in(state.Spells.Known) || in(state.Spells.Prepared)
	case rules.RefItem, rules.RefMagicItem:
		return stacked(state.Equipment.Equipped) || stacked(state.Equipment.Backpack) || stacked(state.Equipment.Loot)
	case rules.RefLanguage:
		return in(state.Base.Languages)
	case rules.RefSkill:
		return skilled(ref.Slug)
	case rules.RefProficiency:
		if def, ok := cat.Proficiencies.Get(ref.Slug); ok && def.Reference.Kind == rules.RefSkill {
			return skilled(def.Reference.Slug)
		}
		return in(state.Proficiencies)
	}
	return false
}

// checklistKey normalizes one coverage entry. Paths are rewritten to the ones
// the catalogue addresses; names stay as written and are resolved when
// coverage is checked, because what "Wild Magic" names depends on the class
// the character turns out to have.
func (a *Agent) checklistKey(ctx context.Context, s *AgentSession, cat *catalog.Catalog, key string) (string, bool) {
	if id, custom := strings.CutPrefix(key, "custom:"); custom {
		return key, id != ""
	}
	if path, err := a.factPath(ctx, s, cat, key); err == nil && knownFactPath(cat, path) {
		return path, true
	}
	if kind, name, ok := strings.Cut(key, ":"); ok {
		if mapped, natural := naturalKinds[kind]; natural {
			key = mapped + ":" + name
		}
	}
	// Only what a character has can be owed. A feature the class grants is
	// covered by the class; one the build does not grant is worth a reminder,
	// because the sheet has something the rules did not give it. A skill or a
	// language is a number or a pick, and listing it would be a debt nothing
	// can pay.
	ref, ok := rules.ParseRef(key)
	_, structural := structuralEvents[ref.Kind]
	switch ref.Kind {
	case rules.RefSpell, rules.RefItem, rules.RefMagicItem, rules.RefFeature, rules.RefTrait:
		structural = true
	}
	return key, ok && structural
}

// missing lists the checklist entries the draft does not yet account for.
//
// A named entry is covered by the character having it, not by which tool put
// it there: a feature the class grants is as covered as one imported by hand.
func (a *Agent) missing(ctx context.Context, s *AgentSession, cat *catalog.Catalog, state domain.State) []string {
	customs := domain.CustomOptions(s.Log)
	out := []string{}
	for _, key := range s.expected {
		covered := false
		switch {
		case strings.HasPrefix(key, "custom:"):
			covered = slices.ContainsFunc(customs, func(c domain.CustomOption) bool { return "custom:"+c.ID == key })
		case !strings.Contains(key, ":"):
			covered = pathWritten(s.Log, key)
			// A printed total is held by the session and solved into the build.
			if rest, ok := strings.CutPrefix(key, "finalAbilities."); ok && !covered {
				_, covered = s.scores[rules.Ability(rest)]
			}
		default:
			kind, name := entityKind(agentArgs{Ref: key})
			// A name may hold a colon of its own -- "spell:Слово силы: смерть" --
			// and is then not a pack:kind:slug reference to be read whole.
			if _, text, cut := strings.Cut(key, ":"); cut {
				if _, isRef := rules.ParseRef(key); !isRef || strings.Count(key, ":") == 1 {
					name = text
				}
			}
			// Nothing imports a feature or a trait by name -- the build
			// grants them or it does not -- so listing one as missing only
			// ever produced a custom copy of it.
			if kind == "feature" || kind == "trait" {
				continue
			}
			covered = slices.ContainsFunc(customs, func(c domain.CustomOption) bool {
				return slices.Contains(lookupKinds(c.Kind), lookupKinds(kind)[0]) && nameScore(name, c.Name, false) >= .99
			})
			held := func(ref rules.Ref) bool { return agentHolds(state, cat, ref) }
			if _, err := a.resolve(ctx, s, cat, lookupKinds(kind), name, held, true); err == nil {
				covered = true
			} else if found, ok := a.identify(ctx, s, cat, state, kind, name); ok && !covered {
				covered = slices.ContainsFunc(customs, func(c domain.CustomOption) bool { return c.Reference == found.Ref })
			}
		}
		if !covered {
			out = append(out, key)
		}
	}
	return out
}

// localChecklist is the checklist as a model wrote it: paths without the
// namespaces the server added to address them.
func localChecklist(keys []string) []string {
	out := make([]string, len(keys))
	for i, key := range keys {
		out[i] = key
		if !strings.Contains(key, ":") {
			out[i] = localPath(key)
		}
	}
	return out
}

// pathWritten reports whether anything in the log sets a fact path. A skill or
// a saving throw named without its leaf is covered by either leaf, and an
// inventory list by any stack in it.
func pathWritten(log domain.Log, key string) bool {
	for _, e := range log.Events {
		for _, ch := range e.Changes {
			path := string(ch.Path)
			if path == key || strings.HasPrefix(path, key+".") && (strings.HasPrefix(key, "skills.") || strings.HasPrefix(key, "savingThrows.") || strings.HasPrefix(key, "equipment.")) {
				return true
			}
		}
	}
	return false
}

type agentOption struct {
	Key  string `json:"key"`
	Name string `json:"name,omitempty"`
}

// catalogNames looks up an entry's display name, reading each collection once.
func catalogNames(cat *catalog.Catalog) func(rules.Ref) string {
	kinds := map[rules.RefKind]map[rules.Slug]string{}
	return func(ref rules.Ref) string {
		if kinds[ref.Kind] == nil {
			kinds[ref.Kind] = map[rules.Slug]string{}
			for _, c := range catalogCandidates(cat, ref.Kind.String()) {
				entry, _ := rules.ParseRef(c.Ref)
				kinds[ref.Kind][entry.Slug] = c.Name
			}
		}
		return kinds[ref.Kind][ref.Slug]
	}
}

// promptOptions lists what a prompt offers: the key an answer is written with
// and, where the option is a catalogue entry, the name a sheet prints.
func promptOptions(cat *catalog.Catalog, p domain.Prompt, nameOf func(rules.Ref) string) []agentOption {
	choice := cat.ResolveChoice(p.Choice)
	out := []agentOption{}
	if choice.From.Kind != rules.OptionsExplicit {
		for _, c := range catalogCandidates(cat, choice.From.Collection.String()) {
			ref, _ := rules.ParseRef(c.Ref)
			out = append(out, agentOption{Key: localKey(cat, ref.Slug.String()), Name: c.Name})
		}
		return out
	}
	for _, option := range choice.From.Options {
		o := agentOption{Key: localKey(cat, rules.OptionKey(option).String())}
		if ref, ok := option.(rules.RefOption); ok {
			o.Name = nameOf(ref.Ref)
		}
		out = append(out, o)
	}
	return out
}

// agentPrompts is the open questions as a model needs them: what is asked,
// how many to pick, how to answer, and the options themselves. A prompt
// without its options costs a round trip per question, which a bounded run
// cannot afford.
func agentPrompts(cat *catalog.Catalog, prompts []domain.Prompt, files bool) []map[string]any {
	nameOf := catalogNames(cat)
	out := []map[string]any{}
	for _, p := range prompts {
		_, structural := structuralPrompt(p)
		byFact := p.Event.Type == domain.EventChange && p.Choice.Kind != rules.ChooseSpell
		// A sheet lists the gear itself. The questions answered in words --
		// personality, ideals, bonds, flaws, alignment -- are listed like any
		// other: left out, a model never learned they were unanswered and
		// never offered to fill them.
		if files && p.Choice.Kind == rules.ChooseEquipment {
			continue
		}
		entry := map[string]any{"id": localKey(cat, p.Choice.Prompt.String()), "choose": p.Choice.Choose, "kind": p.Choice.Kind.String(), "how": "answer_choices"}
		if structural {
			entry["how"] = "import_facts {kind, name} (a class with its printed level), or answer_choices with the entry's name"
		}
		if byFact {
			entry["how"] = "import_facts {path, value}"
			if path := promptPaths[p.Choice.Prompt.String()]; path != "" {
				entry["how"] = "import_facts {path: \"" + path + "\", value}, or plan_import"
			}
		}
		for key, set := range map[string]bool{"optional": p.Optional, "upTo": p.UpTo, "heldOnly": p.HeldOnly, "repeatable": p.Choice.Repeatable} {
			if set {
				entry[key] = true
			}
		}
		if p.Level > 0 {
			entry["level"] = p.Level
		}
		if p.Purpose != "" {
			entry["purpose"] = p.Purpose
		}
		if !p.Source.IsZero() {
			entry["source"] = localKey(cat, p.Source.String())
		}
		if len(p.Held) > 0 {
			entry["held"] = localKeys(cat, p.Held)
		}
		if len(p.Blocked) > 0 {
			entry["blocked"] = localKeys(cat, p.Blocked)
		}
		// The custom-source spell prompts offer every spell there is; a count
		// says as much as the list would.
		if options := promptOptions(cat, p, nameOf); len(options) <= 60 && p.Purpose != "custom" {
			entry["options"] = options
		} else {
			entry["optionCount"] = len(options)
		}
		out = append(out, entry)
	}
	return out
}

func (a *Agent) openPrompts(s *AgentSession, cat *catalog.Catalog) ([]map[string]any, error) {
	prompts, err := domain.Prompts(nativeLog(s.Log), cat)
	if err != nil {
		return nil, err
	}
	// The builder's two "any extra spell" questions are always open and offer
	// every spell there is. Listed, a model tries to close them; extra spells
	// reach them through assign_spells or a custom option's ref instead.
	prompts = slices.DeleteFunc(prompts, func(p domain.Prompt) bool {
		return p.Choice.Kind == rules.ChooseSpell && p.Purpose == "custom"
	})
	return agentPrompts(cat, prompts, len(s.Files) > 0), nil
}

// localKey drops the pinned packs' namespaces from an option key. A key is
// not always a slug -- it can be a prompt id or a bundle of several entries --
// so only a prefix that is a pack's own name is taken off.
func localKey(cat *catalog.Catalog, key string) string {
	for _, release := range cat.Lock.Packs {
		key = strings.ReplaceAll(key, release.ID+"/", "")
	}
	return key
}

func localKeys(cat *catalog.Catalog, keys []rules.Slug) []string {
	out := make([]string, len(keys))
	for i, key := range keys {
		out[i] = localKey(cat, key.String())
	}
	return out
}

func localSlug(slug rules.Slug) string {
	if _, local, qualified := strings.Cut(slug.String(), "/"); qualified {
		return local
	}
	return slug.String()
}

// localPath drops pack namespaces from a path for display. Every tool accepts
// the local form back, so a model never has to carry "dnd-2014/" around.
func localPath(path string) string {
	parts := strings.Split(path, ".")
	for i, part := range parts {
		parts[i] = localSlug(rules.Slug(part))
	}
	return strings.Join(parts, ".")
}

// sheetValues is the projected character keyed by the paths a fact is written
// at, so a printed value and the build's own can be read off the same key.
func sheetValues(state domain.State) map[string]any {
	out := map[string]any{
		"identity.name": state.Identity.Name, "identity.alignment": state.Identity.Alignment,
		"race": state.Identity.Race, "subrace": state.Identity.Subrace, "background": state.Identity.Background,
		"base.hitPoints.max": state.Base.HitPoints.Max, "base.languages": state.Base.Languages,
		"status.armorClass": state.Status.ArmorClass, "status.initiative": state.Status.Initiative,
		"status.passivePerception": state.Status.PassivePerception, "status.proficiencyBonus": state.Status.ProficiencyBonus,
		"proficiencies": state.Proficiencies, "features": state.Features, "traits": state.Traits, "feats": state.Feats,
		"spells.cantrips": state.Spells.Cantrips, "spells.known": state.Spells.Known, "spells.prepared": state.Spells.Prepared,
	}
	classes := []map[string]any{}
	for _, c := range state.Identity.Classes {
		classes = append(classes, map[string]any{"class": localSlug(c.Class), "level": c.Level, "subclass": localSlug(c.Subclass)})
	}
	out["classes"] = classes
	for ability, score := range state.Abilities.Scores {
		out["abilities."+ability.String()] = score
	}
	for skill, s := range state.Skills.BySkill {
		out["skills."+skill.String()+".proficiency"] = s.Proficiency.String()
		out["skills."+skill.String()+".bonus"] = s.Bonus
	}
	for ability, s := range state.SavingThrows.ByAbility {
		out["savingThrows."+ability.String()+".proficient"] = s.Proficient
		out["savingThrows."+ability.String()+".bonus"] = s.Bonus
	}
	for _, speed := range state.Base.Speeds {
		if speed.Kind == domain.Walking {
			out["base.speed"] = int(speed.Distance)
		}
	}
	for _, sense := range state.Base.Senses {
		if sense.Kind == domain.Darkvision {
			out["base.senses.darkvision"] = int(sense.Distance)
		}
	}
	for placement, stacks := range map[string][]domain.ItemStack{"equipped": state.Equipment.Equipped, "backpack": state.Equipment.Backpack, "loot": state.Equipment.Loot} {
		for _, stack := range stacks {
			out["equipment."+placement+"."+stack.Item.String()] = stack.Count
		}
	}
	for unit, count := range state.Equipment.Purse {
		out["equipment.purse."+unit.String()] = count
	}
	return out
}

// agentSheet is sheetValues as a model reads it: local names, no namespaces.
func agentSheet(state domain.State) map[string]any {
	out := map[string]any{}
	for path, value := range sheetValues(state) {
		out[localPath(path)] = localValue(value)
	}
	return out
}

func localValue(value any) any {
	switch v := value.(type) {
	case rules.Slug:
		return localSlug(v)
	case []rules.Slug:
		out := make([]string, len(v))
		for i, slug := range v {
			out[i] = localSlug(slug)
		}
		return out
	}
	return value
}

// differences lists the sheet's printed numbers the draft does not show, each
// beside what the draft computes. It is information for the model -- a skill
// not yet assigned, a level set wrong -- and for the summary it writes. It
// changes nothing by itself: the draft is the build.
func (a *Agent) differences(s *AgentSession, cat *catalog.Catalog) []map[string]any {
	out := []map[string]any{}
	if len(s.printed) == 0 {
		return out
	}
	state, err := domain.Project(s.Log, cat)
	if err != nil {
		return out
	}
	computed := sheetValues(state)
	for _, path := range slices.Sorted(maps.Keys(s.printed)) {
		if value, known := computed[path]; known && value != s.printed[path] {
			out = append(out, map[string]any{"path": localPath(path), "printed": s.printed[path], "computed": value})
		}
	}
	return out
}

// knownFactPath keeps checklist entries addressable. Unknown source fields
// become explicit custom entries instead of impossible review requirements.
func knownFactPath(cat *catalog.Catalog, path string) bool {
	parts := strings.Split(path, ".")
	switch parts[0] {
	case "proficiencies":
		return len(parts) == 1
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
