// Package game implements the play-session usecases: the characters a group
// has put on its table, and the games run from them.
//
// Two authorization models already exist in this codebase and this package
// needs a third. A character has an owner and the rule is "is this yours"
// (character.Service.owned). A group has a roster and three ranks and the rule
// is "does your rank outrank what you are trying to do" (group.Service.member).
// Neither answers the question this package is here to ask: *may you read
// somebody else's character*. That is `readable`, and it is the only thing in
// the codebase that widens a character's visibility beyond its owner.
//
// It is deliberately the only thing. character.Service.owned is untouched, so
// every path of the character service that writes to a log is still owner-only
// by construction rather than by anybody remembering to check -- and the new
// rule is a separate function returning a separate answer, which is what stops
// the two from being confused later.
//
// One file here does write to a character its actor does not own: items.go,
// what a table hands over. It is granted by a seat at a game, not by
// `readable`, and it writes a backpack count or a coin and nothing else.
//
// The service owns both aggregates for the reason character.Service owns
// folders as well as characters: a game's roster may only contain characters
// its group shares, so the two invariants are one, and splitting them into two
// services would mean one reaching into the other to keep it.
//
// It depends on the game, group, character, catalogue and account domains, and
// on internal/types. It never sees a *gin.Context and it never sees a JWT.
package game

import (
	"context"
	"log/slog"
	"time"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/character"
	domain "github.com/promix1722/easydnd/internal/domain/game"
	"github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// Service holds the game and sharing usecases. Every dependency arrives
// through the constructor; there are no package-level singletons.
type Service struct {
	games      domain.Repository
	shared     domain.SharedRepository
	groups     group.Repository
	characters character.Repository
	catalog    catalog.Source
	log        *slog.Logger

	// clock is injected so a test can predict the timestamps a share and a
	// roster entry are stamped with. Nil means the real clock; see now.
	clock func() time.Time

	// superadmin reports whether an account may read every sheet. Nil means
	// nobody may.
	superadmin func(context.Context, user.ID) bool

	limits types.Limits
}

// SetLimits replaces the limits this service enforces; it starts with
// types.DefaultLimits.
func (s *Service) SetLimits(l types.Limits) { s.limits = l }

// SetSuperadmin installs the predicate readable consults last.
func (s *Service) SetSuperadmin(is func(context.Context, user.ID) bool) { s.superadmin = is }

// NewService wires a Service over the two new stores and the three aggregates
// they refer to.
//
// It takes the group store rather than the group service because the only
// thing it needs is a rank, which is one query -- and because a usecase
// reaching into another usecase is a dependency this architecture does not
// have anywhere else. It takes the character store and the catalogue for the
// same reason: character.Summarize and character.Project are pure functions of
// a log and a compendium, so rendering a shared sheet needs neither the
// character service nor its ownership rule, which is exactly the rule that
// must not apply here.
func NewService(
	games domain.Repository,
	shared domain.SharedRepository,
	groups group.Repository,
	characters character.Repository,
	source catalog.Source,
	log *slog.Logger,
) *Service {
	return &Service{
		limits:     types.DefaultLimits,
		games:      games,
		shared:     shared,
		groups:     groups,
		characters: characters,
		catalog:    source,
		log:        log,
	}
}

// now reads the clock, defaulting to the real one.
func (s *Service) now() time.Time {
	if s.clock != nil {
		return s.clock()
	}
	return time.Now().UTC()
}

// member resolves the actor's rank in a group, and is the only way any read or
// write in this package reaches one.
//
// A non-member gets a NotFoundError rather than an AccessDeniedError, for
// exactly the reason group.member and character.owned do: a 403 on somebody
// else's id confirms that the id exists, which turns a guessable identifier
// into an enumeration oracle. A member who lacks a right gets AccessDenied,
// because they are standing at the table and hiding it from them would teach
// them nothing.
func (s *Service) member(
	ctx context.Context, actor user.ID, id group.ID,
) (group.Role, error) {
	role, err := s.groups.MemberRole(ctx, id, actor)
	if err != nil {
		if types.IsNotFound(err) {
			return "", types.NewNotFoundError("group %q", id).Because("group.notFound")
		}
		return "", err
	}
	return role, nil
}

// readable fetches a character the actor is allowed to see, and is the only
// function in the codebase that lets anybody but an owner see one.
//
// Four ways in: you own it, its owner opened it, it is shared into a group
// you run -- as its DM or its owner -- or you are a superadmin, who may read
// every sheet and still change none. A player at that table reads it only
// once it is opened. The
// refusal is a NotFoundError in every case, matching character.owned exactly
// -- a character id is a short counter, and a 403 on one that is not yours
// would say it exists.
//
// The membership check runs over the groups the *character* is shared into
// rather than the groups the actor is in, because the first list is almost
// always one or two long and the second can be any length. Either would be
// correct; this one is a query per share rather than a query per group.
func (s *Service) readable(
	ctx context.Context, actor user.ID, id character.ID,
) (character.Character, error) {
	c, err := s.characters.Get(ctx, id)
	if err != nil {
		return character.Character{}, err
	}
	if c.Owner == character.OwnerID(actor) {
		return c, nil
	}
	// Its owner opened it: anybody signed in who has the link may read it,
	// whatever table they do or do not sit at. Still only a read -- every
	// write goes through the character service, which asks who owns it.
	if c.Public {
		return c, nil
	}

	groups, err := s.shared.GroupsSharing(ctx, id)
	if err != nil {
		return character.Character{}, err
	}
	for _, g := range groups {
		// Sharing seats a character at a table; it does not open the sheet
		// to the table. Only whoever runs it reads a closed one.
		if role, err := s.groups.MemberRole(ctx, g, actor); err == nil {
			if role.AtLeast(group.RoleDM) {
				return c, nil
			}
		} else if !types.IsNotFound(err) {
			return character.Character{}, err
		}
	}
	// Asked last, so the common reads above never pay for the account lookup.
	if s.superadmin != nil && s.superadmin(ctx, actor) {
		return c, nil
	}
	return character.Character{}, types.NewNotFoundError("character %q", id).Because("character.notFound")
}

// Sheet projects a character the actor is allowed to read.
//
// This is the whole point of sharing: a DM opens it to run the character, and
// a player opens a friend's to see what they are playing beside. It is read
// only: being able to read a sheet never grants a write to it. The writes
// this package does make are items.go's, and a seat at a game grants those.
func (s *Service) Sheet(
	ctx context.Context, actor user.ID, id character.ID, locale rules.Locale,
) (character.State, error) {
	state, _, err := s.SheetWithCatalog(ctx, actor, id, locale)
	return state, err
}

// SheetWithCatalog is Sheet plus the catalogue it was projected against. See
// the character service's method of the same name.
func (s *Service) SheetWithCatalog(
	ctx context.Context, actor user.ID, id character.ID, locale rules.Locale,
) (character.State, *catalog.Catalog, error) {
	c, err := s.readable(ctx, actor, id)
	if err != nil {
		return character.State{}, nil, err
	}
	cat, err := catalog.LoadLocked(ctx, s.catalog, locale, c.Log.RulesLock())
	if err != nil {
		return character.State{}, nil, err
	}
	state, err := character.Project(c.Log, cat)
	return state, cat, err
}

// summarize folds a set of character ids into the short form a roster shows,
// skipping any that no longer exist.
//
// Skipping rather than erroring is deliberate and load-bearing. These stores
// name characters they do not own: a character can be deleted mid-process, and
// after a restart every id in a pool names nothing at all. A listing that
// failed on the first dead id would turn one stale row into a broken screen,
// so a character that is gone is simply not at the table any more.
func (s *Service) summarize(
	ctx context.Context, ids []character.ID, locale rules.Locale,
) ([]character.Summary, error) {
	out := make([]character.Summary, 0, len(ids))
	for _, id := range ids {
		c, err := s.characters.Get(ctx, id)
		if err != nil {
			if types.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		cat, err := catalog.LoadLocked(ctx, s.catalog, locale, c.Log.RulesLock())
		if err != nil {
			return nil, err
		}
		sum := character.Summarize(c.ID, c.Owner, c.Folder, c.Log, cat)
		sum.Public = c.Public
		out = append(out, sum)
	}
	return out, nil
}

// CharacterCatalog follows the same read authorization as a shared sheet.
func (s *Service) CharacterCatalog(ctx context.Context, actor user.ID, id character.ID, locale rules.Locale) (*catalog.Catalog, error) {
	c, err := s.readable(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	return catalog.LoadLocked(ctx, s.catalog, locale, c.Log.RulesLock())
}
