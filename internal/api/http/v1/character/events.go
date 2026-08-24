package character

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
)

// EventsResponse is the body of GET /v1/characters/{id}/events.
type EventsResponse struct {
	Seq    int     `json:"seq"`
	Events []Event `json:"events"`
}

// Events handles GET /v1/characters/{id}/events.
func (h *Handler) Events(c *gin.Context) {
	character, err := h.service.Get(c.Request.Context(), h.owner(c), idOf(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	events := make([]Event, 0, character.Log.Len())
	for _, e := range character.Log.Events {
		events = append(events, eventOf(e))
	}
	c.JSON(http.StatusOK, EventsResponse{Seq: character.Log.LastSeq(), Events: events})
}
