package group

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
)

// ListResponse is the body of GET /v1/groups.
type ListResponse struct {
	Groups []Summary `json:"groups"`
}

// List handles GET /v1/groups.
func (h *Handler) List(c *gin.Context) {
	memberships, err := h.service.List(c.Request.Context(), h.actor(c).ID)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	out := make([]Summary, 0, len(memberships))
	for _, m := range memberships {
		out = append(out, summaryOf(m))
	}
	c.JSON(http.StatusOK, ListResponse{Groups: out})
}
