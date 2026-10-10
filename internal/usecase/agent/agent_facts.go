package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

// planImport handles the plan_import tool.
func (a *Agent) planImport(ctx context.Context, s *AgentSession, cat *catalog.Catalog, args agentArgs) (any, error) {
	var err error
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
		a.service.Logger().Warn("AI wizard tool rejected", "tool", "plan_import", "fact", path, "error", err)
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
	if items := a.withAttackWeapons(ctx, s, cat, args.Items, args.Attacks); len(items) > 0 {
		out["inventory"] = a.setInventory(ctx, s, cat, items)
	}
	if out["open"], err = a.openPrompts(s, cat); err != nil {
		return nil, err
	}
	return out, nil
}

// importFacts handles the import_facts tool.
func (a *Agent) importFacts(ctx context.Context, s *AgentSession, cat *catalog.Catalog, args agentArgs) (any, error) {
	if len(args.Facts) == 0 || len(args.Facts) > 300 {
		return nil, fmt.Errorf("provide 1-300 facts in the facts array")
	}
	failures := []map[string]any{}
	resolved := []any{}
	applied := 0
	for i, fact := range args.Facts {
		result, err := a.importFact(ctx, s, cat, fact)
		if err != nil {
			a.service.Logger().Warn("AI wizard tool rejected", "tool", "import_facts", "fact", fact.Path+fact.Ref+fact.Name, "error", err)
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

// identityKinds are the things a sheet says about who the character is, by the
// word a model uses for them, and the path each is written at.
var identityKinds = map[string]string{"name": "identity.name", "alignment": "identity.alignment", "personality": "identity.personalityTraits", "personalitytraits": "identity.personalityTraits", "ideal": "identity.ideals", "ideals": "identity.ideals", "bond": "identity.bonds", "bonds": "identity.bonds", "flaw": "identity.flaws", "flaws": "identity.flaws"}

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
	if err := charuc.ValidateImported(cat, log); err != nil {
		return nil, err
	}
	projected, err := domain.Project(log, cat)
	if err != nil {
		return nil, err
	}
	if projected.Identity.Level() > charuc.MaxLevel(cat) {
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
		if level < 1 || level > charuc.MaxLevel(cat) {
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
			log, _, err = charuc.Revise(s.Log, cat, e.Seq, &domain.Event{Type: typ, Ref: ref, Level: level, Source: charuc.ObservedGroup(typ, "")})
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
		log.Events[i].Source = charuc.ObservedGroup(typ, "")
		break
	}
	if !replaced {
		_ = log.Append(domain.Event{Type: typ, Ref: ref, Level: level, Evidence: args.Source, Observed: true, Source: charuc.ObservedGroup(typ, "")})
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
			_ = log.Append(domain.Event{Type: domain.EventSubrace, Ref: subrace, Evidence: args.Source, Observed: true, Source: charuc.ObservedGroup(domain.EventSubrace, "")})
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
	result := map[string]any{"applied": true, "path": path}
	refs := []string{}
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
		refs = append(refs, found.Ref)
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
			refs = append(refs, found.Ref)
		}
		op = domain.OpAdd
	}
	// The entries a value named, for whoever reads the write back: the
	// transcript shows them by name.
	if len(refs) > 0 {
		result["refs"] = refs
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
		return setNative(log, change, args.Source), result, nil
	}
	return setObserved(log, change, args.Source), result, nil
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
	_ = log.Append(domain.Event{Type: domain.EventChange, Changes: []domain.Change{change}, Evidence: source, Observed: true, Source: charuc.ObservedGroup(domain.EventChange, string(change.Path))})
	return log
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
