package character

import (
	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
)

// validateImported checks what is worth checking about an imported log.
//
// Deliberately not validateAndAttribute: that checks answers against the
// prompts a character has open, and an import produces no answers at all.
// What does need checking is that every catalogue reference resolves, because
// a dangling one projects as a character who simply has no race, with nothing
// saying why.
//
// One-entry-per-selection binds this path vacuously, and it is worth saying
// why rather than leaving the reader to check. An import makes *no*
// selections: the typed events it writes name a race, a class and its levels
// because the export states them outright, and not one of them carries an
// answer -- every prompt is still open when the character arrives. The init
// event carries the whole opening state as changes, and that is one thing
// asserted rather than eight things chosen: "this is what the sheet said".
// Its entries are therefore also the ones the server cannot attribute, and
// they carry no source at all.
//
// The one place the rule bites is the name, which lives in that init event
// alongside numbers the export pinned. Replacing entry 1 replaces all of it
// together, which is right: an imported opening state is a single assertion,
// and half-replacing it would leave a sheet asserting numbers nobody claimed.
//
// The projection is run as well, and that is the substantive check: it is what
// catches a change addressing a path that does not exist. Doing it here means
// a bad import is a 400 rather than a character that cannot be read back.
func ValidateImported(cat *catalog.Catalog, log domain.Log) error {
	cat = domain.WithCustomCatalog(log, cat)
	if err := log.Validate(); err != nil {
		return err
	}
	for i, event := range log.Events {
		if !requiredRef(event) {
			continue
		}
		if err := validateRef(cat, event, i); err != nil {
			return err
		}
	}
	if _, err := domain.Project(log, cat); err != nil {
		return err
	}
	return nil
}
