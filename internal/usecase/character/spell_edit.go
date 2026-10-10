package character

import (
	"slices"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// Spell edits use today's class-level spell pool even when the edited answer
// was recorded before level-up. Counts and held picks still come from the
// answer's original prefix. The context is read-only; synthetic levels never
// enter the saved log, and non-spell revisions retain strict prefix semantics.
func spellEditContext(prefix, current domain.Log, cat *catalog.Catalog, event domain.Event) (domain.Log, error) {
	if len(event.Choices) == 0 {
		return prefix, nil
	}
	selections, err := domain.ResolvedSelections(current, cat)
	if err != nil {
		return domain.Log{}, err
	}
	for _, answer := range event.Choices {
		if selections[answer.Prompt].Kind != rules.ChooseSpell {
			return prefix, nil
		}
	}
	state, err := domain.Project(current, cat)
	if err != nil {
		return domain.Log{}, err
	}
	before, err := domain.Project(prefix, cat)
	if err != nil {
		return domain.Log{}, err
	}
	context := domain.Log{Events: slices.Clone(prefix.Events)}
	for _, taken := range state.Identity.Classes {
		for _, old := range before.Identity.Classes {
			if old.Class != taken.Class {
				continue
			}
			if !taken.Subclass.IsZero() && old.Subclass != taken.Subclass {
				if err := context.Append(domain.Event{Type: domain.EventSubclass, Ref: rules.NewRef(rules.RefSubclass, taken.Subclass), Level: taken.Level}); err != nil {
					return domain.Log{}, err
				}
			}
			if old.Level < taken.Level {
				if err := context.Append(domain.Event{Type: domain.EventLevel, Ref: rules.NewRef(rules.RefClass, taken.Class), Level: taken.Level}); err != nil {
					return domain.Log{}, err
				}
			}
		}
	}
	return context, nil
}
