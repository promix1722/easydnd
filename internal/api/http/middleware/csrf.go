package middleware

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	"github.com/promix1722/easydnd/internal/types"
)

// HeaderSecFetchSite is the browser's own account of where a request came
// from. Chromium and Firefox send it; Safari does not, so it can only ever be
// used to reject, never to admit.
const HeaderSecFetchSite = "Sec-Fetch-Site"

// SameOrigin rejects state-changing requests that did not come from our own
// pages.
//
// The session cookie is SameSite=Lax, which already withholds it from a
// cross-site POST. This is the second lock, and it is nearly free:
//
//   - Origin must match one of the relying party's own origins. This is the
//     strongest check and the one that does the work.
//   - X-Request-Id must be present. web/src/lib/api/client.ts already sends it
//     on every call, and an HTML form -- the only cross-site POST that carries
//     cookies without a preflight -- cannot set a custom header at all.
//   - Sec-Fetch-Site, where the browser offers it, must say same-origin.
//
// Safe methods are exempt: they change nothing, and a GET that did would be
// the actual bug.
func SameOrigin(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}

		if origin := c.GetHeader("Origin"); origin != "" && !slices.Contains(allowedOrigins, origin) {
			helpers.FormatError(c, types.NewAccessDeniedError("request origin is not allowed"))
			return
		}

		if site := c.GetHeader(HeaderSecFetchSite); site != "" && site != "same-origin" {
			helpers.FormatError(c, types.NewAccessDeniedError("cross-site request rejected").Because("request.crossSite"))
			return
		}

		if c.GetHeader(HeaderRequestID) == "" {
			helpers.FormatError(c, types.NewAccessDeniedError(
				"missing %s; state-changing requests must come from the application", HeaderRequestID))
			return
		}

		c.Next()
	}
}

// NoStore stops anything between us and the browser from keeping a response.
//
// Applied to the auth routes, whose bodies say who someone is. nginx is not
// configured with proxy_cache today, so this guards against a future
// configuration and against intermediaries we do not control.
func NoStore() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}
