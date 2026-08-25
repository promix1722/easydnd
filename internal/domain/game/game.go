// Package game holds the game aggregate: one sitting at a group's table, and
// the characters brought to it.
//
// This is the first aggregate in the codebase that *refers to* a character
// without owning one. A folder holds characters and belongs to the same
// account they do; a group holds people and holds no characters at all. A game
// holds neither -- it names characters that belong to other people, at a table
// that belongs to a third, and every hard thing about this package follows
// from that.
//
// It also owns the group's shared character pool, in Shared below, rather than
// leaving that to internal/domain/group. The two are one invariant: a game's
// roster is a subset of the pool its group shares, and an aggregate that could
// only see one half would let the halves disagree. It is the same test
// internal/domain/character/folder.go applies when it keeps Folder in the
// character package -- a folder has no meaning apart from characters, and a
// game roster has none apart from the pool it draws from.
//
// # Why "game"
//
// Not "session": that word is spent several times over on the thing that
// proves a request belongs to an account -- auth.Session, RequireSession, the
// session cookie, session_secret -- and once more on the browser's
// sessionStorage. Not "party" either: docs/dnd.md reserves that for the
// in-fiction band of adventurers, which is a thing the rules talk about and
// this is not. A game is one sitting, run by a DM, which is what a player
// means when they say "are you coming to the game on Thursday".
//
// # Why none of this is stored
//
// Everything here points at a character id, and a character id is a
// process-local counter that dies with the process -- see the memory
// repository. A table in Postgres would therefore be full of ids naming
// nothing by the next morning, which is the argument 00003_groups.sql already
// makes for why a group holds no characters. So the store is in memory, beside
// the characters it names, and the pool and the games are empty after a
// restart. That is a cost, written down rather than hidden: the group survives
// and the table it sat at does not. When characters become durable this
// package moves with them, and the ports below are what make that one adapter
// rather than a rewrite.
//
// It is an inner layer: the standard library, and the character, group and
// account aggregates. Nothing points back at it -- character and group are
// both unaware this package exists, which is what keeps the sideways
// dependency legal and acyclic.
package game

import (
	"time"

	"github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/user"
)

// ID identifies a game. Opaque base64url text minted by the usecase, exactly
// as a group id is: a game id travels in a URL that gets pasted into a chat
// window, so it must not say how many games exist or let a stranger address
// the next one.
type ID string

// String returns the identifier's text.
func (id ID) String() string { return string(id) }

// IsZero reports whether the identifier is unset.
func (id ID) IsZero() bool { return id == "" }

// Name length bounds, matching the group's so that the two name fields cannot
// disagree about what is acceptable at the same table.
const (
	MinNameLen = 1
	MaxNameLen = 64
)

// Shared is one character a player has put on a group's table.
//
// Sharing is what makes a character visible to anybody but its owner, and it
// is the only thing that does. It grants *reading* and nothing else: the owner
// remains the only account that can write to the log, which is enforced a
// layer up by character.Service.owned, untouched by any of this.
//
// Owner is recorded here rather than read back from the character, because
// every question this type has to answer -- may this person unshare it, whose
// was it -- must stay answerable when the character itself is gone. After a
// restart the pool is empty anyway, but within one process a character can be
// deleted while a roster still names it, and a row that could only describe
// itself by fetching the thing it describes would have nothing to say.
type Shared struct {
	Group     group.ID
	Character character.ID
	Owner     user.ID
	SharedAt  time.Time
}

// Game is one sitting at a group's table.
//
// The roster is not a field here, for the reason group.Group does not carry
// its members: most of what reads a game -- the list screen, a permission
// check, a rename -- does not need it, and the one screen that does asks for
// it by name.
type Game struct {
	ID    ID
	Group group.ID
	Name  string

	// CreatedBy records who opened the game. It is history, not authority:
	// what anybody may do to a game is decided by their rank in its group, so
	// this field must never be consulted to decide it. A DM who opens a game
	// and is then demoted does not keep any power over it.
	CreatedBy user.ID
	CreatedAt time.Time
}

// Entry is one character on a game's roster.
//
// An id and a timestamp, never a snapshot of the character. A copy taken when
// the character was added would show the DM a sheet the player had since
// changed, and the whole event-sourced design exists so that there is exactly
// one answer to what a character currently is.
type Entry struct {
	Character character.ID
	AddedAt   time.Time
}
