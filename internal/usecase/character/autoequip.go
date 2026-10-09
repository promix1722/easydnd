package character

import (
	"context"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// AutoEquip puts one suitable item in each slot of a character who has
// nothing on, as one ordinary change entry in the log.
//
// It is the end of a build and of an import: neither equips anything on its
// way, so this is the one moment a new character gets dressed. A character
// with anything equipped already is left exactly as it is, which is what makes
// it safe to call again -- pressing Finish twice changes nothing the second
// time. See domain.AutoEquip for what "suitable" means.
func (s *Service) AutoEquip(ctx context.Context, owner domain.OwnerID, id domain.ID, locale rules.Locale) error {
	character, cat, err := s.load(ctx, owner, id, locale)
	if err != nil {
		return err
	}
	state, err := domain.Project(character.Log, domain.WithCustomCatalog(character.Log, cat))
	if err != nil {
		return err
	}
	changes := domain.AutoEquip(state, cat)
	if len(changes) == 0 {
		return nil
	}
	_, err = s.Apply(ctx, owner, id, locale, character.Log.LastSeq(), domain.Event{Type: domain.EventChange, Changes: changes})
	return err
}
