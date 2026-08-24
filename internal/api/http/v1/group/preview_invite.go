package group

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
)

// InviteParams carries an invite token.
//
// The token is in the body and never in the URL, and that is a security
// decision rather than a stylistic one: nginx logs the full request line, so a
// token in a path or a query string would sit in the access log in cleartext
// and stay usable for a day. The client keeps it in a URL *fragment*, which no
// browser ever sends to any server.
type InviteParams struct {
	Token string `json:"token"`
}

// PreviewInvite handles POST /v1/invites/preview.
//
// It reads a link without acting on it, so the recipient can see whose group
// they are being asked to join before they join it.
func (h *Handler) PreviewInvite(c *gin.Context) {
	var params InviteParams
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, err)
		return
	}

	preview, err := h.service.PreviewInvite(c.Request.Context(), h.actor(c).ID, params.Token)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, previewOf(preview))
}
