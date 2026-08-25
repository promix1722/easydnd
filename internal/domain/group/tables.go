package group

import "context"

// Tables is the port through which deleting a group reaches whatever else is
// hung off it.
//
// A group's table of shared characters and the games played at it live in
// another aggregate -- see internal/domain/game -- and both outlive the group
// unless something tells them not to. As with character.Sharing, the arrow
// points outward from the thing being deleted: a group knows that something
// may be hanging off it and wants telling, and not what.
//
// Nothing is *unreachable* without this. Every route into a game resolves the
// group behind it first and refuses a caller who is not a member, so a game
// whose group is gone answers 404 to everybody already. This port is what
// stops those rows accumulating in memory rather than what stops them being
// read, and it may be nil in a build that has neither.
type Tables interface {
	// DeleteForGroup removes everything belonging to the group: the games
	// first, then the characters shared with it. It is not an error for there
	// to be none.
	DeleteForGroup(ctx context.Context, id ID) error
}
