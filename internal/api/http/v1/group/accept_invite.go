package group

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
)

// AcceptInvite handles POST /v1/invites/accept.
//
// Accepting a link that has already been accepted succeeds and changes
// nothing. The links are reusable by design, so a second click, a refresh or a
// duplicated tab must not read as an error -- and must not silently re-rank
// somebody who is already seated.
func (h *Handler) AcceptInvite(c *gin.Context) {
	var params InviteParams
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, err)
		return
	}

	ctx := c.Request.Context()
	actor := h.actor(c)
	joined, err := h.service.Accept(ctx, actor, params.Token)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	g, role, members, err := h.service.Get(ctx, actor.ID, joined.ID)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, groupOf(g, role, members))
}
