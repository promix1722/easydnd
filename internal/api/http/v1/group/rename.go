package group

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
)

// RenameParams is the body of PATCH /v1/groups/{id}.
type RenameParams struct {
	Name string `json:"name"`
}

// Rename handles PATCH /v1/groups/{id}.
func (h *Handler) Rename(c *gin.Context) {
	var params RenameParams
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, err)
		return
	}

	ctx := c.Request.Context()
	actor := h.actor(c).ID
	if _, err := h.service.Rename(ctx, actor, idOf(c), params.Name); err != nil {
		helpers.FormatError(c, err)
		return
	}
	// Read back rather than patch the value in hand: the response is the whole
	// group, and the roster may have moved while this request was in flight.
	g, role, members, err := h.service.Get(ctx, actor, idOf(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, groupOf(g, role, members))
}
