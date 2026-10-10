package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

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
