package hexsheet

import (
	"io"
	"time"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/character"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

// Importer satisfies charuc.SheetImporter.
//
// It exists so that Import above can stay a plain function returning this
// package's own types -- which is what makes it testable without dragging the
// application layer into the test -- while the application still gets the
// shape it declared. The conversion below is the whole of it.
type Importer struct{}

// NewImporter builds the importer internal/app injects.
func NewImporter() Importer { return Importer{} }

// Compile-time proof that this is what the usecase asked for. The assignment
// in internal/app would catch it too, but a mismatch should fail in the
// package that got it wrong.
var _ charuc.SheetImporter = Importer{}

// Import reads a HexSheet export into a log and the application's report type.
func (Importer) Import(
	r io.Reader, cat *catalog.Catalog, at time.Time,
) (character.Log, charuc.ImportReport, error) {
	log, report, err := Import(r, cat, at)
	if err != nil {
		return character.Log{}, charuc.ImportReport{}, err
	}
	return log, charuc.ImportReport{
		Unresolved: entries(report.Unresolved),
		Skipped:    entries(report.Skipped),
		Open:       report.Open,
	}, nil
}

func entries(in []Entry) []charuc.ImportEntry {
	if len(in) == 0 {
		return nil
	}
	out := make([]charuc.ImportEntry, 0, len(in))
	for _, e := range in {
		out = append(out, charuc.ImportEntry{Field: e.Field, Detail: e.Detail})
	}
	return out
}
