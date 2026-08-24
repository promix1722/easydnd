package middleware

import (
	"context"
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/logging"
	"github.com/promix1722/easydnd/internal/types"
)

// ContextKeyUser is the gin context key under which RequireSession parks the
// authenticated account.
const ContextKeyUser = "auth_user"

// Authenticator resolves a session token to the account behind it. The auth
// usecase satisfies it; declaring it here rather than importing the concrete
// service keeps the middleware testable with a stub.
type Authenticator interface {
	Session(ctx context.Context, token string) (user.User, error)
}

// RequireSession rejects a request that carries no usable session.
//
// It is the guard every future resource route belongs behind. Failures exit
// through helpers.FormatError like every other error in the API, so the client
// gets the same envelope it already knows how to read -- with a 401, which is
// what tells the SPA to show the landing page rather than an error.
func RequireSession(auth Authenticator, cookies helpers.CookieOptions) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := cookies.Session(c)
		if token == "" {
			helpers.FormatError(c, types.NewUnauthenticatedError("no session"))
			return
		}

		account, err := auth.Session(c.Request.Context(), token)
		if err != nil {
			// A cookie that no longer works is worse than no cookie: the
			// browser would keep sending it on every request. Clear it here
			// so a signed-out visitor stops carrying a dead token around.
			cookies.ClearSession(c)
			helpers.FormatError(c, err)
			return
		}

		SetUser(c, account)
		c.Next()
	}
}

// SetUser parks an authenticated account on the request, and tags the
// request-scoped logger with it so every later line is attributable -- the
// same trick RequestID plays with the correlation id.
func SetUser(c *gin.Context, account user.User) {
	c.Set(ContextKeyUser, account)

	ctx := logging.IntoContext(c.Request.Context(),
		logging.FromContext(c.Request.Context()).With(slog.String("user_id", string(account.ID))))
	c.Request = c.Request.WithContext(ctx)
}

// UserFrom returns the account RequireSession resolved.
//
// The second return is false only when the handler was reached without the
// middleware, which is a wiring bug rather than a runtime condition; handlers
// behind RequireSession may treat it as always present.
func UserFrom(c *gin.Context) (user.User, bool) {
	value, exists := c.Get(ContextKeyUser)
	if !exists {
		return user.User{}, false
	}
	account, ok := value.(user.User)
	return account, ok
}
