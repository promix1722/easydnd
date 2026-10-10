package agent

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// prepareReview handles the prepare_review tool.
func (a *Agent) prepareReview(ctx context.Context, s *AgentSession, cat *catalog.Catalog, args agentArgs) (any, error) {
	if err := spokenIn(s.Locale, args.Text); err != nil {
		return nil, err
	}
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
			return map[string]any{"ready": false, "remainingChoices": agentPrompts(cat, required, false, s.decided()), "next": "Resolve these choices from the source or call ask_user with a focused question and suggested answers. Do not send the user to the editor. Only set allow_incomplete if the user explicitly chooses to leave choices incomplete."}, nil
		}
	}
	// Whatever is still unanswered is the owner's to decide about, not the
	// model's to pass over: optional questions included.
	// Refused until the owner has actually been asked: a second call is
	// not consent.
	// allow_incomplete does not skip this: it is the model's word that
	// the owner chose to leave things open, and the owner was not asked.
	// The builder's own extra-spell questions are not counted -- they
	// are always open, and no character is unfinished for them.
	//
	// What the owner has been asked about is theirs already, whatever
	// they answered and whenever they were asked. It used to count only
	// if the question came after this refusal: an owner asked about a
	// blank alignment who said "leave it blank" was asked again a
	// message later, because the refusal that followed did not know.
	open, err := a.openPrompts(s, cat)
	if err != nil {
		return nil, err
	}
	open = slices.DeleteFunc(open, func(entry map[string]any) bool { return entry["purpose"] == "custom" })
	unasked := slices.DeleteFunc(slices.Clone(open), func(entry map[string]any) bool { return entry["asked"] != nil })
	if len(unasked) > 0 && !s.Unattended {
		s.offered = true
		return map[string]any{"ready": false, "unanswered": unasked, "userAnswers": s.answers(), "next": "The user has not been asked about these. Do not finish yet: call ask_user once and offer to settle them, naming each in the user's language and never by its id, with the answers \"Fill them in for me\", \"One by one\" and \"Leave them blank\". Fill them in: answer each from the sheet or the description. One by one: one ask_user per entry, with its options as prepared answers (for a written one such as a personality trait, offer a few suggestions that suit the character). Leave them blank: prepare_review again. What the user has already answered is in userAnswers, is not listed here and is never asked again."}, nil
	}
	// The draft goes to its owner as a build, not as a build with the
	// sheet's numbers pinned on top. What the build reproduces is dropped
	// here, and with it the checklist entry that asked for it.
	pruned := pruneAgentOverrides(s.Log, cat)
	s.expected = slices.DeleteFunc(s.expected, func(key string) bool {
		return !strings.Contains(key, ":") && overridePath(key) && !pathWritten(pruned, key)
	})
	s.Log = pruned
	if err := setStatus(s, "review"); err != nil {
		return nil, err
	}
	addAgentEvent(s, "assistant", args.Text, "", nil)
	// Nobody was asked, so the handoff says what was left for the owner.
	if s.Unattended && len(open) > 0 {
		s.Events[len(s.Events)-1].Data = raw(map[string]any{"open": open})
	}
	return map[string]bool{"ready": true}, nil
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
