package auth

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	"github.com/promix1722/easydnd/internal/api/http/middleware"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/logging"
	"github.com/promix1722/easydnd/internal/types"
	authuc "github.com/promix1722/easydnd/internal/usecase/auth"
)

// ProvidersResponse lists the external providers this deployment offers.
//
// The client needs it to decide whether to render a "Continue with Google"
// button at all: the credentials are optional configuration, so a button that
// is always shown would be a dead end on a deployment that never set them.
type ProvidersResponse struct {
	Providers []Provider `json:"providers"`
}

// Provider is the wire form of one external provider.
type Provider struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// providerNames maps a provider id to what a button should say. It lives here,
// in the transport layer, because it is presentation: the domain's Provider is
// an identifier, not a label.
var providerNames = map[user.Provider]string{
	user.ProviderGoogle: "Google",
}

// Providers handles GET /v1/auth/providers.
//
// Unguarded: a signed-out visitor is exactly who needs to know which sign-in
// buttons to show. It reveals only what the sign-in page would reveal anyway.
func (h *Handler) Providers(c *gin.Context) {
	available := h.svc.Providers()
	out := make([]Provider, 0, len(available))
	for _, id := range available {
		name, ok := providerNames[id]
		if !ok {
			name = string(id)
		}
		out = append(out, Provider{ID: string(id), Name: name})
	}
	c.JSON(http.StatusOK, ProvidersResponse{Providers: out})
}

// SSOStart handles GET /v1/auth/sso/:provider/start.
//
// A redirect rather than JSON, and a GET rather than a POST, because it is
// reached by a plain link: the browser must leave this origin under its own
// steam. Fetching it instead would follow the 302 as an XHR, land the whole
// Google consent page in a JavaScript string, and set no cookie anywhere.
func (h *Handler) SSOStart(c *gin.Context) {
	h.ssoStart(c, nil)
}

// SSOLink handles GET /v1/auth/sso/:provider/link.
//
// Same flow, one difference: it records whose account the resulting identity
// attaches to. That decision is made here, while we know who is asking, and
// sealed into the flight -- not at the callback, where the only evidence would
// be whichever session happened to be open.
//
// It resolves the session itself instead of sitting behind RequireSession,
// because this is a top-level navigation: the middleware answers an expired
// cookie with the standard JSON envelope, which as a navigation response would
// replace the whole application with a page of braces. Guarded just as
// tightly, reported as a redirect.
func (h *Handler) SSOLink(c *gin.Context) {
	account, err := h.currentAccount(c)
	if err != nil {
		h.failSSO(c, user.Provider(c.Param("provider")), err, "session_expired",
			c.Query("return_to"))
		return
	}
	h.ssoStart(c, &account.ID)
}

func (h *Handler) ssoStart(c *gin.Context, linkTo *user.ID) {
	provider := user.Provider(c.Param("provider"))
	returnTo := c.Query("return_to")

	redirect, flight, err := h.svc.StartSSO(c.Request.Context(), provider, returnTo, linkTo)
	if err != nil {
		// A redirect rather than an envelope, for the same reason the callback
		// uses one: an unknown or unconfigured provider reached by a link is
		// still a navigation, and 404 JSON on screen is not an answer.
		h.failSSO(c, provider, err, startFailureCode(err), returnTo)
		return
	}

	h.cookies.SetFlight(c, flight, h.svc.CeremonyTTL())
	c.Redirect(http.StatusFound, redirect)
}

// currentAccount resolves the session cookie, or reports why it could not.
func (h *Handler) currentAccount(c *gin.Context) (user.User, error) {
	token := h.cookies.Session(c)
	if token == "" {
		return user.User{}, types.NewUnauthenticatedError("no session").Because("auth.noSession")
	}
	account, err := h.svc.Session(c.Request.Context(), token)
	if err != nil {
		// A cookie that no longer works is worse than no cookie: the browser
		// would keep sending it. Same reasoning as middleware.RequireSession.
		h.cookies.ClearSession(c)
		return user.User{}, err
	}
	return account, nil
}

// startFailureCode distinguishes the one failure a visitor can act on.
func startFailureCode(err error) string {
	if types.IsNotFound(err) {
		return "unknown_provider"
	}
	return "sign_in_failed"
}

// SSOCallback handles GET /v1/auth/sso/:provider/callback.
//
// It cannot sit behind RequireSession: it is the request that establishes the
// session. What guards it instead is the sealed flight cookie and the `state`
// it carries, which is why neither is optional.
//
// Every exit is a redirect back into the SPA, success or failure. The API has
// no HTML to render, and a JSON error body at the end of a top-level
// navigation would replace the application with a page of braces. Failures
// therefore land on the client with ?auth_error=, and the reason goes to the
// log rather than into the URL -- a message rendered from a query parameter is
// a way to put chosen text on somebody else's page.
func (h *Handler) SSOCallback(c *gin.Context) {
	provider := user.Provider(c.Param("provider"))
	flight := h.cookies.Flight(c)

	// Spent either way, and cleared before anything can fail: a sealed
	// attempt must not survive to be presented a second time.
	h.cookies.ClearFlight(c)

	// Google reports a refusal in the query string rather than by failing.
	// "access_denied" is somebody clicking Cancel, which is not an error
	// worth alarming anyone about.
	if reason := c.Query("error"); reason != "" {
		h.failSSO(c, provider, types.NewUnauthenticatedError("provider returned %q", reason),
			providerRefusalCode(reason), "")
		return
	}

	// Present but unusable is not the same as absent, so the session is read
	// leniently here: a stale cookie must not turn a fresh sign-in into a
	// failure. Only a link consults the result, and only to require a match.
	var sessionUserID user.ID
	if token := h.cookies.Session(c); token != "" {
		if account, err := h.svc.Session(c.Request.Context(), token); err == nil {
			sessionUserID = account.ID
		}
	}

	account, token, returnTo, err := h.svc.FinishSSO(
		c.Request.Context(), provider, flight, c.Query("state"), c.Query("code"), sessionUserID)
	if err != nil {
		// FinishSSO reports the landing path even when it fails, so long as it
		// got far enough to open the flight.
		h.failSSO(c, provider, err, "sign_in_failed", returnTo)
		return
	}

	h.cookies.SetSession(c, token, h.svc.SessionTTL())
	logging.FromContext(c.Request.Context()).Info("federated sign-in complete",
		"user_id", account.ID, "provider", provider)
	c.Redirect(http.StatusFound, returnTo)
}

// failSSO logs the real reason and sends the browser back with a coarse code.
//
// It lands on the page the attempt was started from where that is known, so a
// link that fails returns to the account screen rather than dumping somebody
// on the party list with no explanation. The path is re-sanitised here because
// on the paths where the flight could not be opened it is a raw query
// parameter, which must never be allowed to name another site.
func (h *Handler) failSSO(
	c *gin.Context,
	provider user.Provider,
	err error,
	code, returnTo string,
) {
	logging.FromContext(c.Request.Context()).Warn("federated sign-in failed",
		"provider", provider, "code", code, "error", err)

	landing := authuc.SafeReturnTo(returnTo)
	separator := "?"
	if strings.Contains(landing, "?") {
		separator = "&"
	}
	c.Redirect(http.StatusFound, landing+separator+"auth_error="+url.QueryEscape(code))
}

// providerRefusalCode narrows what the provider said to something this
// application has words for.
//
// The value is the provider's, which makes it attacker-influenced by way of a
// crafted callback URL; passing it into our own query string unfiltered would
// let it choose the key the client looks up. Only the one refusal worth
// distinguishing survives.
func providerRefusalCode(reason string) string {
	if reason == "access_denied" {
		return "access_denied"
	}
	return "sign_in_failed"
}

// UnlinkParams names the identity to detach.
type UnlinkParams struct {
	// Subject identifies which link to remove. It is required rather than
	// implied because an account may hold more than one identity from the
	// same provider, and guessing which to drop is not a guess worth making.
	Subject string `json:"subject"`
}

// SSOUnlink handles POST /v1/auth/sso/:provider/unlink.
//
// A POST, unlike the rest of this file, because it changes something and must
// therefore travel through the CSRF guard the safe methods are exempt from.
func (h *Handler) SSOUnlink(c *gin.Context) {
	account, ok := middleware.UserFrom(c)
	if !ok {
		helpers.FormatError(c, types.NewUnauthenticatedError("no session").Because("auth.noSession"))
		return
	}

	var params UnlinkParams
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, err)
		return
	}
	if params.Subject == "" {
		helpers.FormatError(c, types.NewFieldValidationError("subject is required",
			types.FieldError{Field: "subject", Rule: "required"}))
		return
	}

	updated, err := h.svc.Unlink(
		c.Request.Context(), account.ID, user.Provider(c.Param("provider")), params.Subject)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, SessionResponse{User: toWire(updated)})
}
