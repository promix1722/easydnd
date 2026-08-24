package group

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	domain "github.com/promix1722/easydnd/internal/domain/group"
)

// CreateInviteParams is the body of POST /v1/groups/{id}/invites.
type CreateInviteParams struct {
	// Role is the rank the link seats its holder at. Empty means player,
	// which is the answer almost every time and the least powerful one to
	// arrive at by omission.
	Role string `json:"role"`
}

// CreateInvite handles POST /v1/groups/{id}/invites. Owner and DMs may.
func (h *Handler) CreateInvite(c *gin.Context) {
	var params CreateInviteParams
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, err)
		return
	}
	role := domain.Role(params.Role)
	if params.Role == "" {
		role = domain.RolePlayer
	}

	invitation, err := h.service.Invite(c.Request.Context(), h.actor(c).ID, idOf(c), role)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusCreated, inviteOf(invitation))
}
