package game

import (
	"context"
	"slices"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/character"
	domain "github.com/promix1722/easydnd/internal/domain/game"
	"github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// Create opens a game at a group's table.
//
// DM or owner. Running the game is what a DM is, and a player opening one
// would be a player deciding what the table is doing -- the same reasoning
// that puts renaming the group at this rank.
func (s *Service) Create(
	ctx context.Context, actor user.ID, id group.ID, name string,
) (domain.Game, error) {
	role, err := s.member(ctx, actor, id)
	if err != nil {
		return domain.Game{}, err
	}
	if !role.AtLeast(group.RoleDM) {
		return domain.Game{}, types.NewAccessDeniedError("only the owner or a DM may open a game")
	}
	clean, err := validateName(name)
	if err != nil {
		return domain.Game{}, err
	}
	gameID, err := newGameID()
	if err != nil {
		return domain.Game{}, err
	}

	g := domain.Game{
		ID:        gameID,
		Group:     id,
		Name:      clean,
		CreatedBy: actor,
		CreatedAt: s.now(),
	}
	if err := s.games.Create(ctx, g); err != nil {
		return domain.Game{}, err
	}
	s.log.Info("game created",
		"game_id", string(gameID), "group_id", string(id), "actor_id", string(actor))
	return g, nil
}

// At is one game together with the name of the table it sits at.
//
// The name is joined in on read rather than stored on the game, for the reason
// group.Member.DisplayName is: there is one place that says what a group is
// called, and a copy here would be a second answer that drifts when somebody
// renames the table.
type At struct {
	Game      domain.Game
	GroupName string
}

// Mine returns every game at every table the actor sits at, newest first.
//
// This is what makes a game a thing somebody has rather than a thing a group
// has. It takes no group id because a player who is at three tables wants one
// list, and asking them which table before showing them anything is asking
// them to remember where a game lives in order to find it.
//
// One query per group rather than one query overall. That is honest against
// the in-memory store this runs on, and it is the shape a SQL sibling would
// replace with a join -- the port stays the same either way.
func (s *Service) Mine(ctx context.Context, actor user.ID) ([]At, error) {
	memberships, err := s.groups.ListFor(ctx, actor)
	if err != nil {
		return nil, err
	}
	out := make([]At, 0)
	for _, m := range memberships {
		games, err := s.games.ListFor(ctx, m.Group.ID)
		if err != nil {
			return nil, err
		}
		for _, g := range games {
			out = append(out, At{Game: g, GroupName: m.Group.Name})
		}
	}
	// Newest first across every table, with the id breaking a tie so two games
	// opened in one clock tick do not come back in group order.
	slices.SortFunc(out, func(a, b At) int {
		if !a.Game.CreatedAt.Equal(b.Game.CreatedAt) {
			return b.Game.CreatedAt.Compare(a.Game.CreatedAt)
		}
		return strings.Compare(string(a.Game.ID), string(b.Game.ID))
	})
	return out, nil
}

// Get returns a game, the caller's rank at its table, and its roster.
//
// The rank comes back because every screen has to decide which controls to
// draw, and without it each of them would ask for the group separately -- the
// same reasoning behind the Role field on the group DTO.
func (s *Service) Get(
	ctx context.Context, actor user.ID, id domain.ID, locale rules.Locale,
) (domain.Game, group.Role, []character.Summary, error) {
	g, role, err := s.at(ctx, actor, id)
	if err != nil {
		return domain.Game{}, "", nil, err
	}
	roster, err := s.games.Characters(ctx, id)
	if err != nil {
		return domain.Game{}, "", nil, err
	}
	ids := make([]character.ID, 0, len(roster))
	for _, e := range roster {
		ids = append(ids, e.Character)
	}
	summaries, err := s.summarize(ctx, ids, locale)
	if err != nil {
		return domain.Game{}, "", nil, err
	}
	return g, role, summaries, nil
}

// at resolves a game and the actor's rank at the table it sits on, and is the
// only way any read or write in this package reaches a game.
//
// A game is reached through its group, so the refusal is the group's: somebody
// who is not at the table gets a 404 on the game, which tells them neither
// that the game exists nor that the group does.
func (s *Service) at(
	ctx context.Context, actor user.ID, id domain.ID,
) (domain.Game, group.Role, error) {
	g, err := s.games.ByID(ctx, id)
	if err != nil {
		return domain.Game{}, "", err
	}
	role, err := s.member(ctx, actor, g.Group)
	if err != nil {
		if types.IsNotFound(err) {
			return domain.Game{}, "", types.NewNotFoundError("game %q", id).Because("game.notFound")
		}
		return domain.Game{}, "", err
	}
	return g, role, nil
}

// dm resolves a game and refuses it to anybody who does not run the table.
func (s *Service) dm(
	ctx context.Context, actor user.ID, id domain.ID, what string,
) (domain.Game, error) {
	g, role, err := s.at(ctx, actor, id)
	if err != nil {
		return domain.Game{}, err
	}
	if !role.AtLeast(group.RoleDM) {
		return domain.Game{}, types.NewAccessDeniedError("only the owner or a DM may %s", what)
	}
	return g, nil
}

// Rename changes a game's name. DM or owner.
func (s *Service) Rename(
	ctx context.Context, actor user.ID, id domain.ID, name string,
) (domain.Game, error) {
	g, err := s.dm(ctx, actor, id, "rename a game")
	if err != nil {
		return domain.Game{}, err
	}
	clean, err := validateName(name)
	if err != nil {
		return domain.Game{}, err
	}
	if err := s.games.Rename(ctx, id, clean); err != nil {
		return domain.Game{}, err
	}
	g.Name = clean
	return g, nil
}

// Delete removes a game. DM or owner.
//
// It takes nothing with it. The characters were never the game's -- they are
// the players' and they stay on the group's table, which is the difference
// between this and deleting a folder.
func (s *Service) Delete(ctx context.Context, actor user.ID, id domain.ID) error {
	if _, err := s.dm(ctx, actor, id, "delete a game"); err != nil {
		return err
	}
	if err := s.games.Delete(ctx, id); err != nil {
		return err
	}
	s.log.Info("game deleted", "game_id", string(id), "actor_id", string(actor))
	return nil
}

// AddCharacters seats characters at a game.
//
// DM or owner: choosing who is in tonight's game is running it.
//
// Every character seated here must be on the group's table, because the whole
// point of the table is that everybody at it can read what is in front of
// them -- a roster carrying a name nobody but its owner may open would be a
// leak the DM caused by accident.
//
// So a character that is not on the table yet is put there, provided it is the
// actor's own. That is what makes seating your own character one action rather
// than two: sharing is not a separate decision from bringing it to the game,
// it is the same decision seen from the table's side. Somebody else's
// unshared character is still refused -- a DM may run the table, but they do
// not get to publish another player's character on their behalf.
func (s *Service) AddCharacters(
	ctx context.Context, actor user.ID, id domain.ID, cs []character.ID,
) error {
	g, err := s.dm(ctx, actor, id, "change a game's roster")
	if err != nil {
		return err
	}
	if len(cs) == 0 {
		return types.NewValidationError("name at least one character to seat")
	}

	for _, c := range cs {
		onTable, err := s.shared.IsShared(ctx, g.Group, c)
		if err != nil {
			return err
		}
		if onTable {
			continue
		}
		if _, err := s.owned(ctx, actor, c); err != nil {
			return types.NewValidationError("character %q is not on this group's table", c)
		}
		err = s.shared.Share(ctx, domain.Shared{
			Group:     g.Group,
			Character: c,
			Owner:     actor,
			SharedAt:  s.now(),
		})
		if err != nil {
			return err
		}
		s.log.Info("character shared by being seated",
			"group_id", string(g.Group), "character_id", string(c), "actor_id", string(actor))
	}

	if err := s.games.AddCharacters(ctx, id, cs, s.now()); err != nil {
		return err
	}
	s.log.Info("characters added to game",
		"game_id", string(id), "count", len(cs), "actor_id", string(actor))
	return nil
}

// RemoveCharacter takes a character out of a game, leaving it on the group's
// table. DM or owner.
func (s *Service) RemoveCharacter(
	ctx context.Context, actor user.ID, id domain.ID, c character.ID,
) error {
	if _, err := s.dm(ctx, actor, id, "change a game's roster"); err != nil {
		return err
	}
	return s.games.RemoveCharacter(ctx, id, c)
}

// DeleteForGroup removes everything a group had: its games first, then its
// table.
//
// It exists for one caller -- deleting a group -- and takes no actor for the
// reason UnshareEverywhere does not: the group service has already decided the
// caller owns it.
func (s *Service) DeleteForGroup(ctx context.Context, id group.ID) error {
	if err := s.games.DeleteForGroup(ctx, id); err != nil {
		return err
	}
	return s.shared.ClearGroup(ctx, id)
}
