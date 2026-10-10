package character

import "github.com/promix1722/easydnd/internal/domain/catalog"

// Project replays a log into the character it describes.
//
// A log holds build decisions only. What a character has spent at a table --
// slots, hit dice, uses of a feature -- is the game's to keep, beside the seat
// (see usecase/game), and never an entry here.
func Project(log Log, cat *catalog.Catalog) (State, error) {
	if err := log.Validate(); err != nil {
		return State{}, err
	}
	s, err := projectBuild(log, cat)
	if err != nil {
		return s, err
	}
	err = actionOffers(&s, cat)
	return s, err
}
