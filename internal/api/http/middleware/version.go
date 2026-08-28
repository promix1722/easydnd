package middleware

import "github.com/gin-gonic/gin"

// HeaderAppVersion names the release that answered a request. It carries the
// same string GET /v1/version serves and the same one the frontend bundle was
// built with, so a browser can compare the two without asking.
const HeaderAppVersion = "X-App-Version"

// AppVersion stamps the running release onto every response.
//
// This is how a long-lived tab finds out it is running code from a release
// that is no longer deployed. The client compares this header against the
// version baked into its bundle at its single fetch choke point, so any
// request it was going to make anyway is the check -- no polling, no extra
// traffic, and no interval to tune.
//
// It belongs in the global chain rather than on a route, for two reasons. A
// 401 or a 500 is still evidence of which release answered, and a client
// running against a newer API is exactly the client that starts collecting
// them. And the header must be set before c.Next(), so that it survives a
// handler that aborts.
//
// Same-origin only, so no Access-Control-Expose-Headers is needed: the browser
// hides response headers from cross-origin reads, and nginx proxies /v1/
// without touching them.
func AppVersion(version string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set(HeaderAppVersion, version)
		c.Next()
	}
}
