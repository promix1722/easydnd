// Package group serves the group resource and the invite links that lead into
// it.
//
// Two URL trees, one package. /v1/groups is the resource; /v1/invites is the
// same service seen from outside a group, by somebody who is not in it yet and
// so cannot address it. Splitting them into two directories would create a
// boundary where there is none -- both call the one usecase, and the invite
// handlers would be two files importing it exactly as these do.
package group

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/middleware"
	domain "github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/user"
	groupuc "github.com/promix1722/easydnd/internal/usecase/group"
)

// Handler serves the group resource.
type Handler struct {
	service *groupuc.Service
	log     *slog.Logger
}

// New builds the handler over the group service.
func New(service *groupuc.Service, log *slog.Logger) *Handler {
	return &Handler{service: service, log: log}
}

// actor returns the whole account the request is authenticated as.
//
// The whole account rather than just its id, because two of these routes may
// have to write the row a guest needs before they can be named in a roster,
// and that row needs a display name. See the group usecase's ensureStored.
//
// Every route here sits behind middleware.RequireSession, so UserFrom
// reporting nothing means a route was declared outside the guarded group --
// a wiring bug, not a runtime condition. The zero user is returned in that
// case deliberately: it is in no group, so a mis-wired route shows an empty
// list rather than somebody else's table.
func (h *Handler) actor(c *gin.Context) user.User {
	account, ok := middleware.UserFrom(c)
	if !ok {
		return user.User{}
	}
	return account
}

// idOf reads the group id from the path.
func idOf(c *gin.Context) domain.ID { return domain.ID(c.Param("id")) }

// targetOf reads the member a request is about from ?user=.
//
// A query parameter rather than a second path segment, so that every resource
// route stays at most one level deep. It is the same shape as
// DELETE /v1/characters/{id}/events?after=N.
func targetOf(c *gin.Context) user.ID { return user.ID(c.Query("user")) }
