package middleware

import (
	"errors"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	"github.com/promix1722/easydnd/internal/logging"
	"github.com/promix1722/easydnd/internal/types"
)

// Recovery turns a panic into a logged 500 travelling through FormatError, so
// panics produce the same error envelope as every other failure.
//
// gin.Recovery would do the job, but it writes through gin's own logger rather
// than slog and does not go through our error mapper.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}

			log := logging.FromContext(c.Request.Context())

			// A client that hung up mid-response is not our bug and must not
			// page anyone; there is also no connection left to reply on.
			if isBrokenPipe(r) {
				log.Warn("client disconnected", "route", c.FullPath(), "panic", r)
				c.Abort()
				return
			}

			log.Error("panic recovered",
				"panic", r,
				"route", c.FullPath(),
				"stack", string(debug.Stack()),
			)
			helpers.FormatError(c, types.NewServerError("panic: %v", r))
		}()

		c.Next()
	}
}

func isBrokenPipe(r any) bool {
	err, ok := r.(error)
	if !ok {
		return false
	}
	if errors.Is(err, net.ErrClosed) || errors.Is(err, http.ErrAbortHandler) {
		return true
	}
	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		return false
	}
	var sysErr *os.SyscallError
	if !errors.As(opErr, &sysErr) {
		return false
	}
	msg := strings.ToLower(sysErr.Error())
	return strings.Contains(msg, "broken pipe") || strings.Contains(msg, "connection reset by peer")
}
