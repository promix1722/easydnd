package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/logging"
)

// RequestLogger emits one structured access-log line per request, using the
// request-scoped logger installed by RequestID.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		status := c.Writer.Status()
		attrs := []any{
			slog.String("method", c.Request.Method),
			// FullPath is the matched route pattern, so it stays
			// low-cardinality; the concrete path goes in its own field.
			slog.String("route", c.FullPath()),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", status),
			slog.Int("bytes", c.Writer.Size()),
			slog.Duration("latency", time.Since(start)),
			slog.String("client_ip", c.ClientIP()),
		}

		log := logging.FromContext(c.Request.Context())
		switch {
		case status >= 500:
			log.Error("http request", attrs...)
		case status >= 400:
			log.Warn("http request", attrs...)
		default:
			log.Info("http request", attrs...)
		}
	}
}
