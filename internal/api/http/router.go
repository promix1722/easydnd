// Package httpapi wires gin routes to handlers.
//
// The directory is internal/api/http but the package is named httpapi, so that
// files here can import net/http without every file in the package -- and
// every importer of it -- needing an alias.
//
// This is the only tree in the codebase that imports gin. Handlers translate
// between HTTP and the application layer; they pass c.Request.Context() and
// plain values inward, never a *gin.Context.
package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	"github.com/promix1722/easydnd/internal/api/http/middleware"
	"github.com/promix1722/easydnd/internal/api/http/v1/auth"
	catalogapi "github.com/promix1722/easydnd/internal/api/http/v1/catalog"
	characterapi "github.com/promix1722/easydnd/internal/api/http/v1/character"
	groupapi "github.com/promix1722/easydnd/internal/api/http/v1/group"
	"github.com/promix1722/easydnd/internal/api/http/v1/system"
	"github.com/promix1722/easydnd/internal/config"
	"github.com/promix1722/easydnd/internal/types"
)

// Handlers is the set of inbound adapters the router needs. internal/app
// builds it.
type Handlers struct {
	System    *system.Handler
	Auth      *auth.Handler
	Catalog   *catalogapi.Handler
	Character *characterapi.Handler
	Group     *groupapi.Handler
	// Authenticator resolves the session cookie for the guarded routes. It is
	// the same object Auth is built over; the router takes it separately
	// because middleware and handler need different halves of it.
	Authenticator middleware.Authenticator
}

// NewRouter builds the engine and declares the complete route table. Keeping
// every route visible in one file is deliberate: it is the API's index.
func NewRouter(cfg *config.Config, log *slog.Logger, h Handlers) (*gin.Engine, error) {
	cookies := helpers.CookieOptions{Secure: cfg.Auth.SecureCookies}
	r := gin.New()

	// gin defaults to trusting 0.0.0.0/0 with ForwardedByClientIP on, which
	// lets any client forge X-Forwarded-For and poison ClientIP in the access
	// log and in anything built on it later. We only ever sit behind our own
	// reverse proxy on loopback.
	if err := r.SetTrustedProxies(cfg.HTTP.TrustedProxies); err != nil {
		return nil, fmt.Errorf("set trusted proxies: %w", err)
	}
	r.ForwardedByClientIP = true
	r.HandleMethodNotAllowed = true

	// Order matters: RequestID runs first so that Recovery and the access log
	// can both reach the request-scoped logger it installs.
	r.Use(
		middleware.RequestID(log),
		middleware.Recovery(),
		middleware.RequestLogger(),
	)

	r.NoRoute(func(c *gin.Context) {
		helpers.FormatError(c, types.NewNotFoundError("no route for %s %s",
			c.Request.Method, c.Request.URL.Path))
	})
	r.NoMethod(func(c *gin.Context) {
		c.AbortWithStatus(http.StatusMethodNotAllowed)
	})

	// SameOrigin covers the whole of /v1 rather than just the auth routes:
	// every state-changing request now travels with a session cookie, so
	// every one of them is a CSRF target. Safe methods pass through untouched.
	v1 := r.Group("/v1", middleware.SameOrigin(cfg.Auth.RPOrigins))
	{
		// DEPLOY-CRITICAL: deploy/deploy.sh polls /v1/version for the release
		// SHA and rolls the release back if it does not appear.
		v1.GET("/version", h.System.Version)
		v1.GET("/health", h.System.Health)

		// Sign-in. NoStore because these bodies say who someone is.
		authRoutes := v1.Group("/auth", middleware.NoStore())
		{
			authRoutes.POST("/register/begin", h.Auth.RegisterBegin)
			authRoutes.POST("/register/finish", h.Auth.RegisterFinish)
			authRoutes.POST("/login/begin", h.Auth.LoginBegin)
			authRoutes.POST("/login/finish", h.Auth.LoginFinish)
			// The way in for a visitor with no passkey and no intention of
			// making one. It issues a session that names no account, so it
			// needs no ceremony and no second call.
			authRoutes.POST("/anonymous", h.Auth.Anonymous)
			// Unguarded on purpose: signing out has to work even when the
			// session is already unusable.
			authRoutes.POST("/logout", h.Auth.Logout)

			// Which sign-in buttons to draw. Unguarded because a signed-out
			// visitor is precisely who needs the answer.
			authRoutes.GET("/providers", h.Auth.Providers)

			// Federated sign-in. Both of these are GETs reached by a plain
			// link, not by fetch: the browser has to leave this origin as a
			// top-level navigation for any of it to work.
			//
			// The `sso` segment is deliberate. A bare /v1/auth/:provider/...
			// would put a wildcard where `register` and `login` are static,
			// which is an ambiguity every route added here later would have
			// to reason about.
			sso := authRoutes.Group("/sso")
			{
				sso.GET("/:provider/start", h.Auth.SSOStart)
				// UNGUARDED, AND IT MUST BE: this is the request that
				// establishes a session, so there is none to require. It is
				// guarded instead by the sealed flight cookie and the `state`
				// inside it -- see the handler.
				sso.GET("/:provider/callback", h.Auth.SSOCallback)

				// /link is NOT in the guarded group, and that is deliberate.
				// It is a top-level navigation, so RequireSession's JSON
				// envelope would land on screen as a page of braces when a
				// cookie has expired; the handler resolves the session
				// itself and redirects instead. /unlink is a fetch, so the
				// envelope is exactly right for it.
				sso.GET("/:provider/link", h.Auth.SSOLink)
				sso.POST("/:provider/unlink",
					middleware.RequireSession(h.Authenticator, cookies), h.Auth.SSOUnlink)
			}

			guarded := authRoutes.Group("", middleware.RequireSession(h.Authenticator, cookies))
			guarded.GET("/me", h.Auth.Me)
		}

		// Every resource route sits inside this group, as the comment that
		// stood here before them insisted: a character sheet is somebody's,
		// and an endpoint added one line too high has no authentication at
		// all.
		//
		// The compendium is inside it too, though it is nobody's. It is
		// shared, read-only SRD data and guarding it protects nothing --
		// but this application has no page that reads it without being
		// signed in, so the choice is between a guard that costs nothing
		// and an unauthenticated surface that earns nothing. If a public
		// compendium browser ever wants it, move the two lines out and say
		// in a comment that it is deliberate.
		authed := v1.Group("", middleware.RequireSession(h.Authenticator, cookies))
		{
			authed.GET("/catalog", h.Catalog.Manifest)
			authed.GET("/catalog/:collection", h.Catalog.Collection)

			// Characters. Every route is at most one level deep: a
			// sub-resource under an addressed parent, and never a
			// sub-resource of that.
			authed.GET("/characters", h.Character.List)
			authed.POST("/characters", h.Character.Create)
			// Import is a sibling of create, not a sub-resource of a
			// character: it is what makes one.
			authed.POST("/characters/import", h.Character.Import)
			authed.GET("/characters/:id", h.Character.Get)
			authed.DELETE("/characters/:id", h.Character.Delete)
			authed.GET("/characters/:id/sheet", h.Character.Sheet)
			authed.GET("/characters/:id/prompts", h.Character.Prompts)
			authed.GET("/characters/:id/events", h.Character.Events)
			authed.POST("/characters/:id/events", h.Character.AppendEvents)
			authed.DELETE("/characters/:id/events", h.Character.TruncateEvents)
			// One entry of that log, addressed by position -- which is what
			// Seq means. Addressing a member of a sub-resource collection is
			// not a third level: there is no route below these two, and
			// there will not be one.
			authed.PUT("/characters/:id/events/:seq", h.Character.ReplaceEvent)
			authed.DELETE("/characters/:id/events/:seq", h.Character.DeleteEvent)

			// Groups. A group is the first thing here that belongs to more
			// than one person, so unlike a character it is reached by rank
			// rather than by ownership -- but the guard is the same, and for
			// the same reason.
			authed.GET("/groups", h.Group.List)
			authed.POST("/groups", h.Group.Create)
			authed.GET("/groups/:id", h.Group.Get)
			authed.PATCH("/groups/:id", h.Group.Rename)
			authed.DELETE("/groups/:id", h.Group.Delete)
			authed.POST("/groups/:id/invites", h.Group.CreateInvite)
			// One member is addressed by ?user= rather than by a second path
			// segment. Either would be consistent with the routes above --
			// events/:seq addresses a member of a collection the same way --
			// but this is the shape TruncateEvents already uses, and a member
			// is named by an opaque account id rather than by position.
			authed.PATCH("/groups/:id/members", h.Group.SetMemberRole)
			authed.DELETE("/groups/:id/members", h.Group.RemoveMember)

			// Invites are a separate tree on purpose. Somebody redeeming a
			// link is not yet in the group and cannot name it -- the token
			// carries the id -- so there is no addressed parent to hang these
			// off. Both take the token in the body and never in the URL: our
			// own access log records only the path, but nginx in front of it
			// logs the whole request line, and this token is usable for a day.
			authed.POST("/invites/preview", h.Group.PreviewInvite)
			authed.POST("/invites/accept", h.Group.AcceptInvite)
		}
	}

	return r, nil
}
