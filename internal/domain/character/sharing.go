package character

import "context"

// Sharing is the port through which deleting a character reaches whatever else
// is holding a reference to it.
//
// It exists because a character can be offered to a group -- see
// internal/domain/game -- and that offer outlives the character unless
// something tells it not to. The dependency points this way round on purpose:
// a character still knows nothing about groups or games, only that *something*
// may be referring to it and wants telling. Reverse it and character.Project
// would be one import away from knowing who is looking at it.
//
// It is deliberately one method. This is not a general "observe a character"
// hook: the only event anything outside this aggregate needs to hear about is
// the one that makes an id stop meaning anything.
//
// Implementations may be nil, in which case deleting a character tells nobody
// -- correct for a build with no sharing in it, and the same accommodation
// SheetImporter gets for the same reason.
type Sharing interface {
	// UnshareEverywhere removes id from every collection referring to it. It
	// is not an error for there to be none.
	UnshareEverywhere(ctx context.Context, id ID) error
}
