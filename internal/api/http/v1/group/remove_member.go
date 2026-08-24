package group

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
)

// RemoveMember handles DELETE /v1/groups/{id}/members?user=U.
//
// This is also how somebody leaves: leaving is removing yourself. One route
// rather than two, so that the rule stopping an owner from walking away from
// their own group cannot be sidestepped by calling the other one.
func (h *Handler) RemoveMember(c *gin.Context) {
	err := h.service.RemoveMember(c.Request.Context(), h.actor(c).ID, idOf(c), targetOf(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
