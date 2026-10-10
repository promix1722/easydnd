package character

import (
	"context"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/types"
)

// Limits is the limits this service enforces, for the AI Wizard, which writes
// logs of its own and holds them to the same numbers.
func (s *Service) Limits() types.Limits { return s.limits }

// SetLimits replaces the limits this service enforces; it starts with
// types.DefaultLimits.
func (s *Service) SetLimits(l types.Limits) { s.limits = l }

// CheckCharacterLimit refuses once owner holds as many characters as they may.
// Every path that creates one calls it: Create, CopyCharacter and the AI
// Wizard's import.
//
// ponytail: count, then insert -- two creates racing can both pass and leave
// the owner a few over. If that matters, guard the INSERT the way
// postgres/agent.go's Create does.
func (s *Service) CheckCharacterLimit(ctx context.Context, owner domain.OwnerID) error {
	_, held, err := s.repo.Search(ctx, domain.Query{Owners: []domain.OwnerID{owner}, Limit: 1})
	if err != nil {
		return err
	}
	if held >= s.limits.Characters {
		return types.LimitReached("characters", s.limits.Characters)
	}
	return nil
}

// CheckSheet refuses a log that grew past a per-character limit. Only growth
// is refused: a character already over a number -- one made before the number
// was lowered -- can still be edited, trimmed and levelled.
func CheckSheet(before, after domain.Log, cat *catalog.Catalog, l types.Limits) error {
	if n := after.Len(); n > l.CharacterEvents && n > before.Len() {
		return types.LimitReached("characterEvents", l.CharacterEvents)
	}
	options, notes := customCounts(after)
	if options > l.CharacterCustomOptions || notes > l.CharacterNotes {
		wasOptions, wasNotes := customCounts(before)
		if notes > l.CharacterNotes && notes > wasNotes {
			return types.LimitReached("characterNotes", l.CharacterNotes)
		}
		if options > l.CharacterCustomOptions && options > wasOptions {
			return types.LimitReached("characterCustomOptions", l.CharacterCustomOptions)
		}
	}
	// Items exist only in the projection -- one event may add many -- so the
	// new log is projected, and the old one only when the new one is over.
	items, err := itemStacks(after, cat)
	if err != nil {
		return err
	}
	if items > l.CharacterItems {
		was, err := itemStacks(before, cat)
		if err != nil {
			return err
		}
		if items > was {
			return types.LimitReached("characterItems", l.CharacterItems)
		}
	}
	return nil
}

func customCounts(log domain.Log) (options, notes int) {
	for _, o := range domain.CustomOptions(log) {
		options++
		if o.Kind == "note" {
			notes++
		}
	}
	return options, notes
}

func itemStacks(log domain.Log, cat *catalog.Catalog) (int, error) {
	state, err := domain.Project(log, cat)
	if err != nil {
		return 0, err
	}
	e := state.Equipment
	return len(e.Equipped) + len(e.Backpack) + len(e.Loot), nil
}
