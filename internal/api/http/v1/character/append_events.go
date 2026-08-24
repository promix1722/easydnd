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
	c.JSON(http.StatusOK, WriteResponse{Seq: seq, Sheet: sheetOf(sheet)})
}
