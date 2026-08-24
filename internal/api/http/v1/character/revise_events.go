package character

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/types"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

// DryRunQueryParam asks for the cost of a change without making it.
const DryRunQueryParam = "dryRun"

// ReplaceEventParams is the body of PUT /v1/characters/{id}/events/{seq}.
//
// The entry is addressed by position, because position is what Seq means.
// ExpectedSeq is the whole log's guard and is required for the same reason it
// is on an append: the log is one record, so a write against a sequence that
// has moved has to be told rather than silently discarding whatever moved it.
type ReplaceEventParams struct {
	ExpectedSeq int   `json:"expectedSeq"`
	Event       Event `json:"event"`
}

// ReplaceEvent handles PUT /v1/characters/{id}/events/{seq}[?dryRun=true].
//
// This is the one mechanism for changing anything a player chose: replace the
// entry that carries the choice, and revalidate what follows. There is no
// append-a-correction special case, because a prompt that has been answered
// is no longer open and re-answering it is rejected.
//
// Seq 1 is replaceable when the replacement is also an init event -- that is
// how a name is changed. Anything else there is a field error on seq.
func (h *Handler) ReplaceEvent(c *gin.Context) {
	var params ReplaceEventParams
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, err)
		return
	}
	event, fields := toEvent(params.Event, 0)
	if len(fields) > 0 {
		helpers.FormatError(c, types.NewFieldValidationError("the events could not be read", fields...))
		return
	}
	// The server stamps the time on a replacement exactly as it does on an
	// append: an entry's At is when it was recorded, and a replacement is a
	// new recording of a new decision.
	event.At = time.Now().UTC()

	h.revise(c, params.ExpectedSeq, &event)
}

// DeleteEvent handles DELETE /v1/characters/{id}/events/{seq}?expectedSeq=M[&dryRun=true].
//
// The same operation as a replacement with nothing to put back: removing a
// level has no replacement entry, so there is no body to carry one. The
// concurrency token travels as a query parameter for the reason the
// truncation route's already does -- a DELETE with a body is legal but
// unevenly supported by proxies and client libraries.
func (h *Handler) DeleteEvent(c *gin.Context) {
	expected, err := intQuery(c, ExpectedSeqQueryParam)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	h.revise(c, expected, nil)
}

// revise runs the shared path behind both routes, committing unless the
// caller asked for a dry run.
func (h *Handler) revise(c *gin.Context, expectedSeq int, replacement *domain.Event) {
	target, err := seqOf(c)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	dryRun, err := boolQuery(c, DryRunQueryParam)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}

	revision, err := h.service.Revise(
		c.Request.Context(), h.owner(c), idOf(c), helpers.Locale(c),
		expectedSeq, target, replacement, !dryRun,
	)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, WriteResponse{
		Seq:     revision.Seq,
		Sheet:   sheetOf(revision.Sheet),
		Dropped: droppedOf(revision.Dropped),
	})
}

// seqOf reads the addressed entry's position from the path.
func seqOf(c *gin.Context) (int, error) {
	value, err := strconv.Atoi(c.Param("seq"))
	if err != nil {
		return 0, types.NewFieldValidationError("the entry cannot be replaced", types.FieldError{
			Field: "seq", Rule: "format", Message: "an entry is addressed by its sequence number",
		})
	}
	return value, nil
}

// boolQuery reads an optional boolean flag.
//
// An unparseable value is an error rather than a false, and the difference
// matters exactly once: ?dryRun=yes silently treated as absent is a change
// the player asked to preview and got instead.
func boolQuery(c *gin.Context, name string) (bool, error) {
	raw := c.Query(name)
	if raw == "" {
		return false, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, types.NewFieldValidationError("a parameter is not a boolean",
			types.FieldError{
				Field: name, Rule: "format", Message: "this parameter must be true or false",
			})
	}
	return value, nil
}

func droppedOf(dropped []charuc.Dropped) []Dropped {
	if len(dropped) == 0 {
		return nil
	}
	out := make([]Dropped, 0, len(dropped))
	for _, d := range dropped {
		entry := Dropped{
			Seq:    d.Seq,
			Type:   d.Type.String(),
			Ref:    refString(d.Ref),
			Level:  d.Level,
			Source: sourceString(d.Source),
			Reason: string(d.Reason),
		}
		for _, lost := range d.Lost {
			entry.Lost = append(entry.Lost, DroppedAnswer{
				Prompt:  lost.Prompt.String(),
				Picks:   slugStrings(lost.Picks),
				Rule:    lost.Rule,
				Message: lost.Message,
			})
		}
		out = append(out, entry)
	}
	return out
}
