package character

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

// maxExportBytes caps an uploaded sheet.
//
// The reference export is 20 KB and a heavily played character is a small
// multiple of that, so a megabyte is generous. The cap exists because the body
// is read straight into a decoder: without it, an authenticated client can ask
// the server to buffer whatever it likes.
const maxExportBytes = 1 << 20

// ImportEntry is one line of an import report.
type ImportEntry struct {
	Field  string `json:"field"`
	Detail string `json:"detail"`
}

// ImportReport travels to the browser because the player is the only one who
// can judge what a missing item or an unmapped background is worth. Dropping
// it server-side would make the import look lossless when it is not.
type ImportReport struct {
	Unresolved []ImportEntry `json:"unresolved"`
	Skipped    []ImportEntry `json:"skipped"`
	Open       []string      `json:"open"`
}

// ImportResponse is what an imported character looks like.
//
// It mirrors CreateResponse, plus the report. Seq is the sequence the log now
// ends at, so a client can post to it immediately without re-reading.
type ImportResponse struct {
	ID     string       `json:"id"`
	Seq    int          `json:"seq"`
	Sheet  Sheet        `json:"sheet"`
	Report ImportReport `json:"report"`
}

// Import handles POST /v1/characters/import.
//
// The body is the exported sheet itself rather than a wrapper object. An
// import has exactly one input, and asking a client to base64 a file into a
// JSON field would be ceremony that buys nothing.
//
// An imported character arrives with every prompt unanswered, so a client
// should send the player to the build screen rather than the sheet.
//
// `?folder=` files it, defaulting to the caller's default folder. It is in the
// query rather than the body because the body is the export.
func (h *Handler) Import(c *gin.Context) {
	body := http.MaxBytesReader(c.Writer, c.Request.Body, maxExportBytes)
	defer body.Close()

	ctx := c.Request.Context()
	locale := helpers.Locale(c)
	owner := h.owner(c)

	imported, report, err := h.service.Import(ctx, owner, folderOf(c), locale, body)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}

	sheet, err := h.service.Sheet(ctx, owner, imported.ID, locale)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}

	c.JSON(http.StatusCreated, ImportResponse{
		ID:     imported.ID.String(),
		Seq:    imported.Log.LastSeq(),
		Sheet:  SheetOf(sheet),
		Report: reportOf(report),
	})
}

// reportOf converts the usecase's report for the wire.
func reportOf(in charuc.ImportReport) ImportReport {
	out := ImportReport{
		Unresolved: importEntries(in.Unresolved),
		Skipped:    importEntries(in.Skipped),
		Open:       make([]string, 0, len(in.Open)),
	}
	for _, prompt := range in.Open {
		out.Open = append(out.Open, prompt.String())
	}
	return out
}

// importEntries never returns nil, so the field encodes as [] rather than
// null. A client rendering a list should not have to special-case the empty
// case twice.
func importEntries(in []charuc.ImportEntry) []ImportEntry {
	out := make([]ImportEntry, 0, len(in))
	for _, e := range in {
		out = append(out, ImportEntry{Field: e.Field, Detail: e.Detail})
	}
	return out
}
