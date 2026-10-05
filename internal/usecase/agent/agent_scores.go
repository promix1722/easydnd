package agent

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

// During import, final totals must survive adding race/ASI/pack observations in
// any order. At save, reconcile ordinary additive rules back to base scores so
// subsequent level-ups work normally. Non-invertible custom rules retain their
// explicit total override rather than changing the source's observed value.
func rebaseAgentScores(log domain.Log, cat *catalog.Catalog) domain.Log {
	desired, err := domain.Project(log, cat)
	if err != nil {
		return log
	}
	targets := map[rules.Ability]int{}
	stripped := log.Clone()
	for i := range stripped.Events {
		changes := []domain.Change{}
		for _, ch := range stripped.Events[i].Changes {
			if strings.HasPrefix(string(ch.Path), "finalAbilities.") {
				a, ok := rules.ParseAbility(strings.TrimPrefix(string(ch.Path), "finalAbilities."))
				if ok {
					targets[a] = desired.Abilities.Score(a)
				}
				continue
			}
			changes = append(changes, ch)
		}
		stripped.Events[i].Changes = changes
	}
	if len(targets) == 0 {
		return log
	}
	// A total with nothing left to say is not an entry in anybody's history.
	stripped, err = domain.Rebuild(slices.DeleteFunc(stripped.Events, func(e domain.Event) bool { return e.Observed && charuc.SaysNothing(e) }))
	if err != nil {
		return log
	}
	// The answer to the ability-scores question, as the builder's own form
	// writes it: one entry, rewritten in place when the totals are solved again.
	at := slices.IndexFunc(stripped.Events, func(e domain.Event) bool {
		return !e.Observed && e.Type == domain.EventChange && slices.ContainsFunc(e.Changes, func(ch domain.Change) bool { return ch.Path == "abilities.method" })
	})
	bases := map[rules.Ability]int{}
	for ability := range targets {
		bases[ability] = 10
	}
	for _, event := range stripped.Events {
		for _, ch := range event.Changes {
			parts := ch.Path.Segments()
			if len(parts) != 2 || parts[0] != "abilities" {
				continue
			}
			ability, ok := rules.ParseAbility(parts[1])
			if !ok {
				continue
			}
			if _, tracked := targets[ability]; !tracked {
				continue
			}
			switch ch.Op {
			case domain.OpSet:
				bases[ability] = ch.Value.Int
			case domain.OpIncrement:
				bases[ability] += ch.Value.Int
			}
		}
	}
	for attempt := 0; attempt < 5; attempt++ {
		event := domain.Event{Type: domain.EventChange, Source: domain.GroupAbilities, Changes: []domain.Change{{Path: "abilities.method", Op: domain.OpSet, Value: domain.SlugValue("manual")}}}
		// Catalogue ability order keeps logs deterministic across map iteration.
		for _, ability := range cat.AbilityIDs() {
			if base, ok := bases[ability]; ok {
				event.Changes = append(event.Changes, domain.Change{Path: domain.Path("abilities." + ability.String()), Op: domain.OpSet, Value: domain.IntValue(base)})
			} else if at >= 0 {
				// A score the sheet did not print keeps what the entry held.
				for _, ch := range stripped.Events[at].Changes {
					if ch.Path == domain.Path("abilities."+ability.String()) {
						event.Changes = append(event.Changes, ch)
					}
				}
			}
		}
		candidate := stripped.Clone()
		if at >= 0 {
			candidate.Events[at].Changes = event.Changes
		} else if err := candidate.Append(event); err != nil {
			return log
		}
		state, err := domain.Project(candidate, cat)
		if err != nil {
			return log
		}
		equal := true
		for ability, want := range targets {
			delta := want - state.Abilities.Score(ability)
			if delta != 0 {
				equal = false
				bases[ability] += delta
			}
		}
		if equal {
			if len(charuc.ValidateChanges(cat, event, 0)) > 0 {
				return log
			}
			// The same totals are insufficient if a custom conditional rule
			// changed grants or resources while solving for a base score.
			before := desired
			before.Contributions = nil
			before.Abilities.Method = state.Abilities.Method
			before.Abilities.Scores = state.Abilities.Scores
			state.Contributions = nil
			if !reflect.DeepEqual(before, state) {
				return log
			}
			return candidate
		}
	}
	return log
}

// settleScores keeps the build at the totals the sheet prints.
//
// A tool writes a printed total as finalAbilities.<ability>; this takes it off
// the character and onto the session, then solves the base scores that reach
// every total held there. Only a build no base score can reach -- a custom
// rule that is not additive -- keeps the total pinned over it.
func (a *Agent) settleScores(s *AgentSession, cat *catalog.Catalog) {
	for _, e := range s.Log.Events {
		for _, ch := range e.Changes {
			if rest, ok := strings.CutPrefix(string(ch.Path), "finalAbilities."); ok && ch.Value.Kind == domain.ValueInt {
				if ability, ok := rules.ParseAbility(rest); ok {
					if s.scores == nil {
						s.scores = map[rules.Ability]int{}
					}
					s.scores[ability] = ch.Value.Int
				}
			}
		}
	}
	if len(s.scores) == 0 {
		return
	}
	log := s.Log.Clone()
	for _, ability := range cat.AbilityIDs() {
		if total, ok := s.scores[ability]; ok {
			log = setObserved(log, domain.Change{Path: domain.Path("finalAbilities." + ability.String()), Op: domain.OpSet, Value: domain.IntValue(total)}, "")
		}
	}
	s.Log = rebaseAgentScores(log, cat)
}

// overridePath reports that a path is a printed number or list laid over what
// the build derives, rather than a fact the build is made of.
func overridePath(path string) bool {
	for _, prefix := range []string{"skills.", "savingThrows.", "status.", "base.", "proficiencies"} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// nativeLog is the draft without the printed values imported on top of it:
// the character as its build alone makes it.
//
// Prompts are asked of this view. A sheet's skills are usually written down
// before its choices are answered, and a prompt computed over them would see
// every skill as already held and refuse the pick that explains it.
func nativeLog(log domain.Log) domain.Log {
	out := log.Clone()
	for i := range out.Events {
		if out.Events[i].Observed {
			out.Events[i].Changes = slices.DeleteFunc(out.Events[i].Changes, func(ch domain.Change) bool { return overridePath(string(ch.Path)) })
		}
	}
	return out
}

// pruneAgentOverrides drops every imported value the build already produces.
//
// A value written over the build -- AC 14, because the user asked for it kept
// -- is a fact about the character only while the build says otherwise. Once
// the build computes the same number it is a copy of the build's own
// arithmetic, and left in place it would pin the sheet against every later
// edit. Each is compared with what the build alone computes at the same path.
// A path the build has no value for is kept: nothing says it repeats.
func pruneAgentOverrides(log domain.Log, cat *catalog.Catalog) domain.Log {
	native, err := domain.Project(nativeLog(log), cat)
	if err != nil {
		return log
	}
	computed := sheetValues(native)
	out := log.Clone()
	for i := range out.Events {
		if !out.Events[i].Observed || out.Events[i].Type != domain.EventChange {
			continue
		}
		out.Events[i].Changes = slices.DeleteFunc(out.Events[i].Changes, func(change domain.Change) bool {
			value, known := computed[string(change.Path)]
			return overridePath(string(change.Path)) && known && produces(value, change)
		})
	}
	// An observation with nothing left to say is not an entry in anybody's
	// history; the log is renumbered without it.
	events := slices.DeleteFunc(out.Events, func(e domain.Event) bool { return e.Observed && charuc.SaysNothing(e) })
	rebuilt, err := domain.Rebuild(events)
	if err != nil {
		return log
	}
	return rebuilt
}

// produces reports whether a value the build computes is what a printed
// change would set. A list the sheet adds to is produced when the build
// already holds every entry of it.
func produces(computed any, change domain.Change) bool {
	switch change.Value.Kind {
	case domain.ValueInt:
		return computed == change.Value.Int
	case domain.ValueBool:
		return computed == change.Value.Bool
	case domain.ValueString:
		return fmt.Sprint(computed) == change.Value.Str
	case domain.ValueSlug:
		return fmt.Sprint(computed) == change.Value.Slug.String()
	case domain.ValueSlugList:
		held, ok := computed.([]rules.Slug)
		if !ok || change.Op != domain.OpAdd && len(held) != len(change.Value.Slugs) {
			return false
		}
		return !slices.ContainsFunc(change.Value.Slugs, func(slug rules.Slug) bool { return !slices.Contains(held, slug) })
	}
	return false
}

// sameScores reports that two logs project the same six ability totals.
func sameScores(x, y domain.Log, cat *catalog.Catalog) bool {
	before, errX := domain.Project(x, cat)
	after, errY := domain.Project(y, cat)
	return errX == nil && errY == nil && maps.Equal(before.Abilities.Scores, after.Abilities.Scores)
}

// assignAbilities answers the ability-score prompts a sheet cannot answer: an
// Ability Score Improvement nobody itemised, a half-elf's two +1s.
//
// A sheet prints six totals, never how they were reached. Left open, such a
// prompt keeps the whole total in the base score, and the owner who answers it
// later in the builder gets the improvement twice -- a level 10 fighter's
// printed Strength 20 becomes 22. Answering it here costs the totals nothing:
// settleScores holds them where the sheet has them and moves the bases down.
//
// ponytail: where the points go is a guess -- the highest base score first, so
// the bases end up as flat as a point buy -- because the sheet does not say.
// A sheet format that itemises its improvements would want them read instead.
func (a *Agent) assignAbilities(ctx context.Context, s *AgentSession, cat *catalog.Catalog) (any, error) {
	bases := map[rules.Ability]int{}
	for _, e := range s.Log.Events {
		for _, ch := range e.Changes {
			if rest, ok := strings.CutPrefix(string(ch.Path), "abilities."); ok && ch.Value.Kind == domain.ValueInt {
				if ability, ok := rules.ParseAbility(rest); ok {
					bases[ability] = ch.Value.Int
				}
			}
		}
	}
	if len(s.scores) == 0 || len(bases) == 0 {
		return nil, fmt.Errorf("the sheet's ability totals are not in the draft yet: give them to plan_import first")
	}
	// take picks the ability with the most base score to give up, among the
	// ones offered, and never takes a base under 8: no player starts lower.
	take := func(offered []rules.Ability, not []rules.Ability) (rules.Ability, bool) {
		best, found := rules.Ability(""), false
		for _, ability := range cat.AbilityIDs() {
			if slices.Contains(offered, ability) && !slices.Contains(not, ability) && bases[ability] > 8 && (!found || bases[ability] > bases[best]) {
				best, found = ability, true
			}
		}
		if found {
			bases[best]--
		}
		return best, found
	}
	assigned := []map[string]any{}
	// An answer opens the next prompt (the improvement's "scores or a feat"
	// branch, then its two points), so the open prompts are read again after
	// each one; tried keeps a refused prompt from being asked forever.
	tried := map[rules.Slug]bool{}
	for {
		prompts, err := domain.Prompts(nativeLog(s.Log), cat)
		if err != nil {
			return nil, err
		}
		i := slices.IndexFunc(prompts, func(p domain.Prompt) bool {
			return !p.Optional && !tried[p.Choice.Prompt] && (p.Choice.Kind == rules.ChooseAbilityScores || p.Choice.Kind == rules.ChooseAbilityBonus)
		})
		if i < 0 {
			break
		}
		p := prompts[i]
		tried[p.Choice.Prompt] = true
		picks := []string{}
		if p.Choice.Kind == rules.ChooseAbilityScores {
			// "Scores or a feat": the sheet's feats are imported by name
			// before this, so an improvement still open is the scores.
			picks = []string{p.Choice.Prompt.String() + "/0"}
		} else {
			offered, chosen := []rules.Ability{}, []rules.Ability{}
			for _, option := range p.Choice.From.Options {
				if bonus, ok := option.(rules.AbilityBonusOption); ok {
					offered = append(offered, bonus.Ability)
				}
			}
			for range p.Choice.Choose {
				not := chosen
				if p.Choice.Repeatable {
					not = nil
				}
				ability, ok := take(offered, not)
				if !ok {
					break
				}
				chosen = append(chosen, ability)
				picks = append(picks, ability.String())
			}
			if len(picks) < p.Choice.Choose {
				continue
			}
		}
		if _, err := a.answer(ctx, s, cat, agentArgs{Prompt: p.Choice.Prompt.String(), Picks: picks}); err != nil {
			continue
		}
		if p.Choice.Kind == rules.ChooseAbilityBonus {
			assigned = append(assigned, map[string]any{"prompt": localKey(cat, p.Choice.Prompt.String()), "picks": picks})
		}
	}
	open, err := a.openPrompts(s, cat)
	if err != nil {
		return nil, err
	}
	return map[string]any{"assigned": assigned, "open": open, "next": "The ability totals are unchanged: these answers only say which part of each total is an improvement. The sheet does not print that, so say in the summary that the improvements were placed by inference."}, nil
}
