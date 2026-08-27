package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	"github.com/promix1722/easydnd/internal/api/http/middleware"
	"github.com/promix1722/easydnd/internal/types"
)

// Me handles GET /v1/auth/me.
//
// It sits behind RequireSession, so an anonymous caller gets a 401 with the
// standard error envelope rather than a 200 saying "nobody". The SPA calls
// this once on load and reads that 401 as "show the landing page" -- see
// web/src/lib/auth.
func (h *Handler) Me(c *gin.Context) {
	account, ok := middleware.UserFrom(c)
	if !ok {
		helpers.FormatError(c, types.NewUnauthenticatedError("no session").Because("auth.noSession"))
		return
	}
	c.JSON(http.StatusOK, SessionResponse{User: toWire(account)})
}

// LogoutResponse is the body of POST /v1/auth/logout.
//
// A body, rather than a 204, because web/src/lib/api/client.ts treats an empty
// successful response as a transport fault. Returning JSON everywhere keeps
// that one rule true for every endpoint.
type LogoutResponse struct {
	SignedOut bool `json:"signed_out"`
}

// Logout handles POST /v1/auth/logout.
//
// Clearing the cookie is the whole of it. The token itself stays
// cryptographically valid until it expires -- there is no revocation list to
// add it to, which is the price of a stateless session. Anyone who had already
// captured the cookie keeps it; rotating auth.session_secret is the only lever
// that invalidates outstanding sessions, and it invalidates all of them.
//
// Deliberately unguarded: signing out must work even when the session is
// already unusable, which is exactly when the browser most needs the cookie
// cleared.
func (h *Handler) Logout(c *gin.Context) {
	h.cookies.ClearSession(c)
	h.cookies.ClearCeremony(c)
	c.JSON(http.StatusOK, LogoutResponse{SignedOut: true})
}
