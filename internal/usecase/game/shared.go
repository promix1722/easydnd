package game

import (
	"context"

	"github.com/promix1722/easydnd/internal/domain/character"
	domain "github.com/promix1722/easydnd/internal/domain/game"
	"github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// Share puts one of the actor's own characters on a group's table.
//
// Two checks, and they refuse differently on purpose. Not being in the group
// is a 404 on the group, because a stranger must not learn that it exists. Not
// owning the character is a 404 on the character, because a member must not be
// able to probe which character ids are real by trying to share them. Neither
// is a 403: both would confirm something the caller has no business knowing.
//
// Any member may share, including a player -- putting your own character on
// the table is the whole of what a player does here, and it is the missing
// half of a group that this feature exists to add.
func (s *Service) Share(
	ctx context.Context, actor user.ID, id group.ID, c character.ID,
) error {
	if _, err := s.member(ctx, actor, id); err != nil {
		return err
	}
	if _, err := s.owned(ctx, actor, c); err != nil {
		return err
	}

	err := s.shared.Share(ctx, domain.Shared{
		Group:     id,
		Character: c,
		Owner:     actor,
		SharedAt:  s.now(),
	})
	if err != nil {
		return err
	}
	s.log.Info("character shared with group",
		"group_id", string(id), "character_id", string(c), "actor_id", string(actor))
	return nil
}

// owned fetches a character and refuses it to anyone but its owner.
//
// It is character.Service.owned repeated rather than called, because this
// package must not depend on that service -- and because the two are answering
// different questions that happen to share an implementation today. Sharing
// asks "is this yours to give away"; that one asks "is this yours to read".
func (s *Service) owned(
	ctx context.Context, actor user.ID, id character.ID,
) (character.Character, error) {
	c, err := s.characters.Get(ctx, id)
	if err != nil {
		return character.Character{}, err
	}
	if c.Owner != character.OwnerID(actor) {
		return character.Character{}, types.NewNotFoundError("character %q", id).Because("character.notFound")
	}
	return c, nil
}

// Unshare takes a character back off a group's table.
//
// Either the person whose character it is, or a DM. The owner because it is
// theirs and they may withdraw it; a DM because they run the table and
// clearing somebody's forgotten character off it is table management, not a
// reach into another account -- and because the owner may be a guest whose
// session has expired, in which case nobody else could ever remove it.
//
// The game rosters are cleared first and the pool second. That ordering is
// chosen for what a crash leaves behind: a character seated at a game but not
// in the pool is the state the pool exists to prevent, whereas one in the pool
// and seated nowhere is just a character on the table.
func (s *Service) Unshare(
	ctx context.Context, actor user.ID, id group.ID, c character.ID,
) error {
	role, err := s.member(ctx, actor, id)
	if err != nil {
		return err
	}

	if !role.AtLeast(group.RoleDM) {
		// A player may only withdraw their own. Asked of a character that is
		// not theirs, this is the same 404 sharing gives.
		if _, err := s.owned(ctx, actor, c); err != nil {
			return types.NewAccessDeniedError(
				"only the character's owner or a DM may take it off the table")
		}
	}

	if err := s.games.RemoveFromGroupGames(ctx, id, c); err != nil {
		return err
	}
	if err := s.shared.Unshare(ctx, id, c); err != nil {
		return err
	}
	s.log.Info("character unshared from group",
		"group_id", string(id), "character_id", string(c), "actor_id", string(actor))
	return nil
}

// SharedCharacters lists what a group has on its table.
//
// Any member sees all of it: that is the decision this feature was built
// around, and it is what makes the pool a shared table rather than a private
// drop box. A character that has since been deleted is skipped -- see
// summarize.
func (s *Service) SharedCharacters(
	ctx context.Context, actor user.ID, id group.ID, locale rules.Locale,
) ([]character.Summary, error) {
	if _, err := s.member(ctx, actor, id); err != nil {
		return nil, err
	}
	pool, err := s.shared.List(ctx, id)
	if err != nil {
		return nil, err
	}
	ids := make([]character.ID, 0, len(pool))
	for _, sh := range pool {
		ids = append(ids, sh.Character)
	}
	return s.summarize(ctx, ids, locale)
}

// UnshareEverywhere takes a character off every table it is on.
//
// It exists for one caller: deleting a character. It takes no actor because
// there is nobody to authorize -- the character service has already decided
// the caller owns it, and by the time this runs the character is going or
// gone. See the Sharing port in internal/domain/character.
func (s *Service) UnshareEverywhere(ctx context.Context, c character.ID) error {
	groups, err := s.shared.GroupsSharing(ctx, c)
	if err != nil {
		return err
	}
	for _, g := range groups {
		if err := s.games.RemoveFromGroupGames(ctx, g, c); err != nil {
			return err
		}
	}
	return s.shared.UnshareEverywhere(ctx, c)
}
