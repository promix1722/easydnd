package game

import (
	"context"
	"time"

	"github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/group"
)

// SharedRepository is the persistence port for a group's character pool.
//
// The method set follows the same rule group.Repository does: no caller should
// ever have to read the pool, decide something and write it back, so every
// method below is a single statement. Two players sharing at once cannot
// interleave into a state neither asked for.
//
// The last two methods exist because nothing here owns the things it names. A
// character can be deleted and a group can be deleted, and in both cases the
// rows that referred to them have to go without the caller enumerating them
// first -- which it could not do atomically anyway.
type SharedRepository interface {
	// Share puts s in its group's pool. It reports a *types.ValidationError if
	// that character is already shared there; deciding whether a repeat share
	// is harmless is the usecase's call, not storage's to hide.
	Share(ctx context.Context, s Shared) error

	// Unshare takes c out of g's pool, reporting a *types.NotFoundError if it
	// was not in it. It says nothing about games that may still name c -- the
	// usecase owns that ordering, because there is no transaction spanning the
	// two stores.
	Unshare(ctx context.Context, g group.ID, c character.ID) error

	// List returns g's whole pool, most recently shared last so that the order
	// on screen is the order people put characters on the table.
	List(ctx context.Context, g group.ID) ([]Shared, error)

	// IsShared reports whether c is in g's pool.
	//
	// Its own query rather than a scan of what List returns, because this is
	// the hot path: it is the check on the way into every read of somebody
	// else's character.
	IsShared(ctx context.Context, g group.ID, c character.ID) (bool, error)

	// GroupsSharing returns every group c has been shared into.
	//
	// This is what makes "may this person read this character" answerable in
	// one query from the character's side. The alternative -- listing the
	// reader's groups and asking IsShared for each -- is a query per group on
	// the hottest path there is.
	GroupsSharing(ctx context.Context, c character.ID) ([]group.ID, error)

	// UnshareEverywhere removes c from every pool, for when the character
	// itself is deleted. It is not an error for c to be in none.
	UnshareEverywhere(ctx context.Context, c character.ID) error

	// ClearGroup empties g's pool, for when the group is deleted. It is not an
	// error for the pool to be empty.
	ClearGroup(ctx context.Context, g group.ID) error
}

// Repository is the persistence port for games and their rosters.
//
// Shaped by the same one-statement rule as the pool above, with one
// deliberate exception: AddCharacters takes a slice, because "add everyone at
// the table" is a single thing a DM does and must not be a loop of writes that
// can half succeed.
type Repository interface {
	// Create stores g, reporting a *types.ValidationError if the id is taken.
	Create(ctx context.Context, g Game) error

	// ByID returns the game, or a *types.NotFoundError. It says nothing about
	// who may see it -- that is the usecase's decision, and this port is
	// deliberately unable to make it.
	ByID(ctx context.Context, id ID) (Game, error)

	// ListFor returns every game at g's table, most recently created first.
	ListFor(ctx context.Context, g group.ID) ([]Game, error)

	// Rename changes the game's name, reporting a *types.NotFoundError if it
	// does not exist.
	Rename(ctx context.Context, id ID, name string) error

	// Delete removes the game and its roster.
	Delete(ctx context.Context, id ID) error

	// DeleteForGroup removes every game at g's table, for when the group is
	// deleted. It is not an error for there to be none.
	DeleteForGroup(ctx context.Context, g group.ID) error

	// AddCharacters puts cs on id's roster, stamped at. Characters already on
	// it are left where they are rather than re-stamped: adding everyone when
	// most of them are already seated is the common case, and it must not
	// reorder the roster or fail halfway.
	AddCharacters(ctx context.Context, id ID, cs []character.ID, at time.Time) error

	// RemoveCharacter takes c off id's roster, reporting a
	// *types.NotFoundError if it was not on it.
	RemoveCharacter(ctx context.Context, id ID, c character.ID) error

	// Characters returns id's whole roster, in the order characters were
	// added, so that the order on screen does not depend on the storage
	// engine.
	Characters(ctx context.Context, id ID) ([]Entry, error)

	// RemoveFromGroupGames drops c from every game at g's table, for when a
	// character is unshared while it is still seated at one.
	//
	// One method rather than a loop in the usecase because there is no
	// ordering of those writes that leaves a sensible state if it stops
	// halfway: a character on a roster but not in the pool is exactly the
	// state the pool exists to prevent.
	RemoveFromGroupGames(ctx context.Context, g group.ID, c character.ID) error
}
