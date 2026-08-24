package auth

import (
	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	"github.com/promix1722/easydnd/internal/types"
)

// LoginBegin handles POST /v1/auth/login/begin.
//
// It takes no request body at all, and that is the feature. A discoverable
// credential carries the account handle on the authenticator, so the browser
// can offer the right passkey with nothing typed -- and because we never name
// an account here, the endpoint cannot be used to find out which accounts
// exist.
func (h *Handler) LoginBegin(c *gin.Context) {
	options, ceremony, err := h.svc.BeginLogin(c.Request.Context())
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	h.options(c, options, ceremony)
}

// LoginFinish handles POST /v1/auth/login/finish.
func (h *Handler) LoginFinish(c *gin.Context) {
	ceremony := h.cookies.Ceremony(c)
	if ceremony == "" {
		helpers.FormatError(c, types.NewValidationError("no sign-in is in progress"))
		return
	}

	body, err := ceremonyBody(c)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}

	account, token, err := h.svc.FinishLogin(c.Request.Context(), ceremony, body)
	if err != nil {
		h.cookies.ClearCeremony(c)
		helpers.FormatError(c, err)
		return
	}
	h.establish(c, account, token, h.svc.SessionTTL())
}
