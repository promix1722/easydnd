package character

import (
	"context"
	"io"
	"time"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

// SheetImporter reads a character sheet exported by another tool.
//
// The port is declared here rather than in the adapter that satisfies it, for
// the same reason catalog.Source is: the application states what it needs, and
// internal/app picks what provides it. A usecase importing an adapter would
// point the dependency arrow backwards, and `make lint/layers` exists because
// that is easy to do by accident.
//
// The clock is a parameter so that an import is reproducible: the same file
// imported twice produces the same log.
type SheetImporter interface {
	Import(r io.Reader, cat *catalog.Catalog, at time.Time) (domain.Log, ImportReport, error)
}

// ImportEntry is one line of an ImportReport.
type ImportEntry struct {
	// Field is the exporting tool's own path for the value.
	Field string

	// Detail says what was there and why it did not survive.
	Detail string
}

// ImportReport is everything an import could not carry across.
//
// It is not a failure list. SRD 5.1 publishes one background and one feat, so
// a sheet from a tool with the full rules will always leave something behind;
// the report is what makes that visible instead of silent. It travels all the
// way to the browser for exactly that reason.
type ImportReport struct {
	// Unresolved names something SRD 5.1 does not publish.
	Unresolved []ImportEntry

	// Skipped is real data the model has no home for.
	Skipped []ImportEntry

	// Open lists the prompts the import left for the player to answer.
	Open []rules.Slug
}

// Import creates a character from an exported sheet.
//
// An imported character arrives with its choices *unanswered*. A foreign sheet
// records what a character is, not what was chosen, so reconstructing the
// choices would mean inventing them; instead the numbers land as an opening
// state and every prompt stays open. The client should send the player to the
// build screen afterwards, not the sheet.
func (s *Service) Import(
	ctx context.Context, owner domain.OwnerID, locale rules.Locale, r io.Reader,
) (domain.Character, ImportReport, error) {
	if s.importer == nil {
		return domain.Character{}, ImportReport{}, types.NewNotImplementedError(
			"importing sheets is not configured")
	}

	cat, err := s.catalog.Load(ctx, locale)
	if err != nil {
		return domain.Character{}, ImportReport{}, err
	}

	log, report, err := s.importer.Import(r, cat, s.now())
	if err != nil {
		return domain.Character{}, ImportReport{}, err
	}
	if err := validateImported(cat, log); err != nil {
		return domain.Character{}, ImportReport{}, err
	}

	created, err := s.repo.Create(ctx, owner)
	if err != nil {
		return domain.Character{}, ImportReport{}, err
	}
	if err := s.repo.Append(ctx, created.ID, 0, log.Events...); err != nil {
		return domain.Character{}, ImportReport{}, err
	}

	stored, err := s.repo.Get(ctx, created.ID)
	if err != nil {
		return domain.Character{}, ImportReport{}, err
	}
	return stored, report, nil
}

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
func validateImported(cat *catalog.Catalog, log domain.Log) error {
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
