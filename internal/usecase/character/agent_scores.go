package character

import (
	"reflect"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
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
		event := domain.Event{Type: domain.EventChange, Source: domain.GroupAbilities}
		// Catalogue ability order keeps logs deterministic across map iteration.
		for _, ability := range cat.AbilityIDs() {
			if base, ok := bases[ability]; ok {
				event.Changes = append(event.Changes, domain.Change{Path: domain.Path("abilities." + ability.String()), Op: domain.OpSet, Value: domain.IntValue(base)})
			}
		}
		candidate := stripped.Clone()
		if err := candidate.Append(event); err != nil {
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
			if len(validateChanges(cat, event, 0)) > 0 {
				return log
			}
			// The same totals are insufficient if a custom conditional rule
			// changed grants or resources while solving for a base score.
			before := desired
			before.Contributions = nil
			state.Contributions = nil
			if !reflect.DeepEqual(before, state) {
				return log
			}
			return candidate
		}
	}
	return log
}
