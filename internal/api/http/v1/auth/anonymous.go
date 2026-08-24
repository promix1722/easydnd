package auth

import (
	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
)

// Anonymous handles POST /v1/auth/anonymous.
//
// It is the way in for somebody who will not, or cannot, make a passkey: one
// request, no ceremony, no body, and a session that names nothing stored. The
// account endpoints beside it all write or read a row; this one writes
// nothing at all, which is the entire point.
//
// Deliberately unguarded, like the login routes: there is nobody to
// authenticate yet. It still sits inside the /v1 group's SameOrigin check, so
// a cross-site page cannot mint a session in somebody's browser.
//
// Note what this endpoint does not have: a rate limit. Signing is cheap and
// stateless, so the tokens themselves are not the concern -- but each one can
// then fill the character store, which is bounded by nothing. There is no rate
// limiter anywhere in this service yet; when one arrives, this route wants it.
func (h *Handler) Anonymous(c *gin.Context) {
	guest, token, err := h.svc.SignInAnonymously(c.Request.Context())
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	h.establish(c, guest, token, h.svc.GuestSessionTTL())
}
