package helpers

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/promix1722/easydnd/internal/config"
)

// Cookie names. The prefixes are not decoration: a browser enforces them.
//
//   - __Host- requires Secure, Path=/ and no Domain attribute, which makes the
//     cookie unsettable by any other host -- including a subdomain that someone
//     else ends up controlling.
//   - __Secure- requires Secure but permits a narrower Path, which the ceremony
//     cookie wants so it never leaves /v1/auth.
//
// Both prefixes imply Secure, so development -- which serves the Vite dev
// server over plain HTTP -- uses the bare names instead.
const (
	sessionCookieBase  = "easydnd_session"
	ceremonyCookieBase = "easydnd_ceremony"
	flightCookieBase   = "easydnd_sso"

	sessionCookiePath  = "/"
	ceremonyCookiePath = "/v1/auth"
	flightCookiePath   = "/v1/auth/sso"
)

// CookieOptions decides the attributes every auth cookie carries. It is built
// once in internal/app and handed to the handlers, so the attributes cannot
// drift between the places that set them.
type CookieOptions struct {
	// Secure marks cookies Secure and switches on the name prefixes. Derived
	// from the environment, never from a request.
	Secure      bool
	Development bool
	namespace   string
}

// NewCookieOptions isolates development servers even when their browser URLs
// share a hostname. Cookies do not isolate ports; the API listen address does.
func NewCookieOptions(cfg *config.Config) CookieOptions {
	o := CookieOptions{Secure: cfg.Auth.SecureCookies, Development: cfg.Env == config.EnvDevelopment}
	if o.Development && cfg.HTTP.Port != "" {
		sum := sha256.Sum256([]byte(cfg.HTTP.Addr()))
		o.namespace = "_dev_" + hex.EncodeToString(sum[:8])
	}
	return o
}

const HeaderDevelopmentSession = "X-EasyDnD-Dev-Session"

// The selector is not a credential: the selected cookie still needs a valid
// signature. Ignore the header entirely outside development.
func (o CookieOptions) sessionName(c *gin.Context) string {
	name := o.SessionCookieName()
	if o.Development {
		scope := c.GetHeader(HeaderDevelopmentSession)
		if decoded, err := hex.DecodeString(scope); err == nil && len(decoded) == 16 {
			return name + "_" + scope
		}
	}
	return name
}

// SessionCookieName is the name the session cookie goes out under.
func (o CookieOptions) SessionCookieName() string {
	if o.Secure {
		return "__Host-" + sessionCookieBase + o.namespace
	}
	return sessionCookieBase + o.namespace
}

// CeremonyCookieName is the name the in-flight ceremony cookie goes out under.
func (o CookieOptions) CeremonyCookieName() string {
	if o.Secure {
		return "__Secure-" + ceremonyCookieBase + o.namespace
	}
	return ceremonyCookieBase + o.namespace
}

// FlightCookieName is the name the in-flight SSO cookie goes out under.
func (o CookieOptions) FlightCookieName() string {
	if o.Secure {
		return "__Secure-" + flightCookieBase + o.namespace
	}
	return flightCookieBase + o.namespace
}

// SetSession writes the session cookie.
//
// SameSite is Lax rather than Strict so that arriving from an external link --
// a shared character sheet, a bookmark opened from a chat app -- still finds
// the visitor signed in. Lax still withholds the cookie from cross-site POSTs,
// which is the case that matters.
func (o CookieOptions) SetSession(c *gin.Context, token string, ttl time.Duration) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     o.sessionName(c),
		Value:    token,
		Path:     sessionCookiePath,
		MaxAge:   int(ttl.Seconds()),
		Secure:   o.Secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSession expires the session cookie.
func (o CookieOptions) ClearSession(c *gin.Context) { o.ClearSessionNamed(c, o.sessionName(c)) }

// SiblingSessions lists, by name, every other session cookie of this server's
// own namespace that the request carries -- development only, where each
// account switch mints a new selector and so a new cookie. Cookies do not
// isolate ports, so on a shared development hostname those pile up across
// restarts until the Cookie header is too long for the proxy in front; the
// middleware verifies each one and clears the dead. Other servers' namespaces
// are left alone: their tokens cannot be checked here and may well be live.
func (o CookieOptions) SiblingSessions(c *gin.Context) map[string]string {
	if !o.Development {
		return nil
	}
	own, prefix := o.sessionName(c), o.SessionCookieName()
	var siblings map[string]string
	for _, cookie := range c.Request.Cookies() {
		if cookie.Name == own || !strings.HasPrefix(cookie.Name, prefix) {
			continue
		}
		if siblings == nil {
			siblings = map[string]string{}
		}
		siblings[cookie.Name] = cookie.Value
	}
	return siblings
}

// ClearSessionNamed expires a session cookie by name, with the attributes
// SetSession gave it, which is what makes the browser match the two.
func (o CookieOptions) ClearSessionNamed(c *gin.Context, name string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     sessionCookiePath,
		MaxAge:   -1,
		Secure:   o.Secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// SetCeremony writes the in-flight ceremony cookie.
//
// Strict here, unlike the session: a ceremony is only ever completed by the
// script that started it, so there is no navigation case to accommodate. The
// short Path keeps it out of every other request the browser makes.
func (o CookieOptions) SetCeremony(c *gin.Context, token string, ttl time.Duration) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     o.CeremonyCookieName(),
		Value:    token,
		Path:     ceremonyCookiePath,
		MaxAge:   int(ttl.Seconds()),
		Secure:   o.Secure,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearCeremony expires the ceremony cookie. It runs as soon as a ceremony
// resolves, so a sealed challenge cannot be presented twice.
func (o CookieOptions) ClearCeremony(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     o.CeremonyCookieName(),
		Value:    "",
		Path:     ceremonyCookiePath,
		MaxAge:   -1,
		Secure:   o.Secure,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// SetFlight writes the in-flight SSO cookie.
//
// Lax, and this is the one attribute in this file that the flow will not work
// without. The OAuth callback is a top-level GET navigation arriving from
// accounts.google.com -- a cross-site request by every definition a browser
// uses. Lax is sent on exactly that; Strict, which the ceremony cookie next
// door uses quite correctly, is withheld. Tightening this to Strict to match
// its neighbour would break every Google sign-in with "no sign-in is in
// progress" and nothing in the logs to say why, so there is a test asserting
// it stays Lax.
//
// What Lax gives up, state and PKCE take back: the callback is bound to the
// attempt that started it by a value the client never saw.
func (o CookieOptions) SetFlight(c *gin.Context, token string, ttl time.Duration) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     o.FlightCookieName(),
		Value:    token,
		Path:     flightCookiePath,
		MaxAge:   int(ttl.Seconds()),
		Secure:   o.Secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearFlight expires the in-flight SSO cookie. It runs as soon as a callback
// resolves, either way, so a sealed attempt cannot be presented twice.
func (o CookieOptions) ClearFlight(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     o.FlightCookieName(),
		Value:    "",
		Path:     flightCookiePath,
		MaxAge:   -1,
		Secure:   o.Secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// Flight reads the in-flight SSO token, empty if absent.
func (o CookieOptions) Flight(c *gin.Context) string {
	value, err := c.Cookie(o.FlightCookieName())
	if err != nil {
		return ""
	}
	return value
}

// Session reads the session token, empty if absent.
func (o CookieOptions) Session(c *gin.Context) string {
	value, err := c.Cookie(o.sessionName(c))
	if err != nil {
		return ""
	}
	return value
}

// Ceremony reads the in-flight ceremony token, empty if absent.
func (o CookieOptions) Ceremony(c *gin.Context) string {
	value, err := c.Cookie(o.CeremonyCookieName())
	if err != nil {
		return ""
	}
	return value
}
