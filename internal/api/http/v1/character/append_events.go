package character

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// AppendEventsParams is the body of POST /v1/characters/{id}/events.
type AppendEventsParams struct {
	// ExpectedSeq is the sequence the client believes the log ends at.
	//
	// It is required rather than optional. The whole log is one record, so
	// two clients editing one character would otherwise read, modify and
	// write the same blob, and the later write would discard the earlier
	// silently.
	ExpectedSeq int     `json:"expectedSeq"`
	Events      []Event `json:"events"`
}

// WriteResponse is what any write to the log returns: where the log now ends,
// and what the character now looks like.
//
// The sheet comes back with the write so that a build step is one round trip
// rather than two. It is also the reason this application needs no cache
// invalidation on the client: the response *is* the invalidation.
type WriteResponse struct {
	Seq   int   `json:"seq"`
	Sheet Sheet `json:"sheet"`

	// Dropped is what a replacement cost, and is present only on the routes
	// that can cost anything. An append and a truncation never populate it:
	// an append adds, and a truncation's cost is the range the caller named.
	Dropped []Dropped `json:"dropped,omitempty"`
}

// Dropped is one entry a replacement did not leave alone.
//
// Seq is its position in the log as the client last saw it, *before* the
// replacement -- the whole point is to name rows still on screen, and after
// the rebuild half of them have moved and some are gone.
type Dropped struct {
	Seq    int    `json:"seq"`
	Type   string `json:"type"`
	Ref    string `json:"ref,omitempty"`
	Level  int    `json:"level,omitempty"`
	Source string `json:"source,omitempty"`

	// Reason is one of "not-offered" (nothing offers this entry any more),
	// "answers-dropped" (the entry stands, minus some picks) or "empty" (the
	// entry was nothing but answers and they all went). Only the first and
	// last remove a row.
	Reason string `json:"reason"`

	// Lost names the answers that did not survive, in the same rule
	// vocabulary a rejected append reports.
	Lost []DroppedAnswer `json:"lost,omitempty"`
}

// DroppedAnswer is one answer a replacement invalidated.
type DroppedAnswer struct {
	Prompt string   `json:"prompt"`
	Picks  []string `json:"picks,omitempty"`
	Rule   string   `json:"rule"`
	// Reason is a message key, like the error envelope's. No prose leaves the
	// Go side; the client owns the words. See helpers.ErrorBody.
	Reason string `json:"reason,omitempty"`
}

// AppendEvents handles POST /v1/characters/{id}/events.
func (h *Handler) AppendEvents(c *gin.Context) {
	var params AppendEventsParams
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, err)
		return
	}

	events, err := toEvents(params.Events)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	// The server stamps the time. An event's At is when it was recorded, and
	// letting a client set it would make the log's own ordering arguable.
	now := time.Now().UTC()
	for i := range events {
		events[i].At = now
	}

	ctx := c.Request.Context()
	id := idOf(c)
	locale := helpers.Locale(c)

	seq, err := h.service.Apply(ctx, h.owner(c), id, locale, params.ExpectedSeq, events...)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	h.writeResponse(c, id, locale, seq)
}

// writeResponse returns the sequence and the freshly projected sheet.
func (h *Handler) writeResponse(
	c *gin.Context, id domain.ID, locale rules.Locale, seq int,
) {
	sheet, err := h.service.Sheet(c.Request.Context(), h.owner(c), id, locale)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, WriteResponse{Seq: seq, Sheet: SheetOf(sheet)})
}
