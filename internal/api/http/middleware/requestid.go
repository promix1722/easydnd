// Package middleware holds the gin middleware. This is the outermost ring of
// the inbound adapter; nothing inside internal/usecase or internal/domain may
// import it.
package middleware

import (
	"crypto/rand"
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	"github.com/promix1722/easydnd/internal/logging"
)

// HeaderRequestID is the correlation-id header, read from the request when the
// caller supplies one and always echoed on the response.
const HeaderRequestID = "X-Request-Id"

// RequestID adopts or mints a correlation id and stows a logger tagged with it
// on the request context, so every later log line in the request correlates.
//
// crypto/rand.Text gives 128+ bits of randomness from the standard library,
// which is all a correlation id needs -- no UUID dependency required.
func RequestID(base *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderRequestID)
		if id == "" {
			id = rand.Text()
		}

		c.Writer.Header().Set(HeaderRequestID, id)
		c.Set(helpers.ContextKeyRequestID, id)

		ctx := logging.IntoContext(c.Request.Context(), base.With(slog.String("request_id", id)))
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
