package auth

import (
	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	"github.com/promix1722/easydnd/internal/types"
)

// RegisterBegin handles POST /v1/auth/register/begin.
//
// Like its twin at login/begin, it takes no request body -- there is nothing
// left to send. There has never been a username or an email here, and the
// display name the operating system's passkey prompt needs is now minted by
// the usecase alongside the account id. So the client cannot tell us anything
// about the account it is about to create, which is what lets one button in
// the browser mean both "sign me in" and "sign me up": it does not have to
// know which it is doing before it asks.
//
// Nothing is stored here. The new account travels inside the sealed ceremony
// cookie and is written only when an attestation verifies, so an abandoned
// sign-up leaves no record behind and this endpoint cannot be used to fill the
// account store.
func (h *Handler) RegisterBegin(c *gin.Context) {
	options, ceremony, err := h.svc.BeginRegistration(c.Request.Context())
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	h.options(c, options, ceremony)
}

// RegisterFinish handles POST /v1/auth/register/finish.
func (h *Handler) RegisterFinish(c *gin.Context) {
	ceremony := h.cookies.Ceremony(c)
	if ceremony == "" {
		helpers.FormatError(c, types.NewValidationError("no registration is in progress").Because("auth.noCeremony"))
		return
	}

	body, err := ceremonyBody(c)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}

	account, token, err := h.svc.FinishRegistration(c.Request.Context(), ceremony, body)
	if err != nil {
		// The ceremony is over either way. Leaving the cookie in place would
		// let the client retry a challenge that has already been judged.
		h.cookies.ClearCeremony(c)
		helpers.FormatError(c, err)
		return
	}
	h.establish(c, account, token, h.svc.SessionTTL())
}
