package group

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
)

// Get handles GET /v1/groups/{id}. Every member may read the roster.
func (h *Handler) Get(c *gin.Context) {
	g, role, members, err := h.service.Get(c.Request.Context(), h.actor(c).ID, idOf(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, groupOf(g, role, members))
}
