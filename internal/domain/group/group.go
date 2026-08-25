// Package group holds the play-group aggregate: a set of people at one table,
// and the rank each of them plays at.
//
// This is the first aggregate in the codebase that is not owned by exactly one
// account. A character belongs to whoever made it and to nobody else; a group
// belongs to several people at once, at three different ranks, and the whole
// point of it is that they can see each other. Everything awkward about this
// package follows from that one difference.
//
// It is an inner layer: the standard library and the account aggregate, and
// nothing else. No gin, no pgx, and in particular no JWT library -- an invite
// token is declared here as a port in invite.go and implemented by an adapter,
// for the same reason the WebAuthn ceremony is.
package group

import (
	"strings"
	"time"

	"github.com/promix1722/easydnd/internal/domain/user"
)

// ID identifies a group. It is opaque base64url text, minted by the usecase
// exactly as an account id is, and carries no meaning: a group id travels in
// invite links that get pasted into chat windows, so it must not leak the
// name of the table or the number of groups that exist.
type ID string

// Role is a member's rank in one group.
//
// Three ranks, deliberately not a general permission system. A table has a
// person who owns it, people who run it, and people who play at it, and every
// rule in the usecase is expressible as a comparison between two of those.
type Role string

// The ranks, high to low. The string values are the wire form, the storage
// form and the log form all at once, which is why they are lowercase words
// rather than integers: a row that says 'dm' needs no key to read.
const (
	RoleOwner  Role = "owner"
	RoleDM     Role = "dm"
	RolePlayer Role = "player"
)

// Rank orders the hierarchy.
//
// Permission checks compare two ranks rather than enumerating pairs of roles,
// so adding a rank later is one line here instead of a rewrite of every check.
// An unknown role ranks zero, below every real one, so a value that somehow
// survived validation grants nothing rather than everything.
func (r Role) Rank() int {
	switch r {
	case RoleOwner:
		return 3
	case RoleDM:
		return 2
	case RolePlayer:
		return 1
	default:
		return 0
	}
}

// Valid reports whether r is one of the three ranks.
func (r Role) Valid() bool { return r.Rank() > 0 }

// AtLeast reports whether r outranks other, or equals it.
func (r Role) AtLeast(other Role) bool { return r.Rank() >= other.Rank() }

// Name length bounds. They match the CHECK constraint on groups.name, so a
// name the usecase accepts can never be one the database refuses.
const (
	MinNameLen = 1
	MaxNameLen = 64
)

// Group is one table's roster, without the roster.
//
// Members are loaded separately rather than hanging off this struct, because
// almost everything that reads a group -- the list screen, a permission check,
// a rename -- does not need them, and the one screen that does asks for them
// by name.
type Group struct {
	ID   ID
	Name string

	// CreatedBy records who made the group. It is history, not authority:
	// ownership lives in the member rows and moves when it is transferred, so
	// this field must never be consulted to decide what anybody may do.
	CreatedBy user.ID
	CreatedAt time.Time
}

// Member is one person at the table, as the roster screen shows them.
//
// DisplayName is joined in from the account store on read rather than stored
// here. Copying it would mean a rename showed up in some groups and not
// others, and there is exactly one table in this schema that says what a
// person is called.
type Member struct {
	UserID      user.ID
	DisplayName string
	Role        Role
	JoinedAt    time.Time
}

// Anonymous reports that this member is a guest session rather than an
// account.
//
// It is derived from the id rather than stored, because the id is the only
// thing that knows: see user.AnonymousIDPrefix. The roster shows it so that
// the rest of the table can tell that this person will be gone when their
// session expires, and cannot be invited back to the same seat.
func (m Member) Anonymous() bool {
	return strings.HasPrefix(string(m.UserID), user.AnonymousIDPrefix)
}

// Membership is one row of "the groups I am in": a group, and my own rank in
// it. The list screen needs both and would otherwise ask for every roster.
type Membership struct {
	Group Group
	Role  Role
}
