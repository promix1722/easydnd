// Package game serves the games at a group's table and the characters the
// group has shared with each other.
//
// Three URL trees, one package, for the reason the group package serves both
// /v1/groups and /v1/invites: they are one service seen from three angles, and
// splitting them into three directories would create a boundary where there is
// none.
//
//   - /v1/groups/:id/characters is the table itself, addressed through the
//     group whose table it is.
//   - /v1/games is the games. They are not under /v1/groups/:id because a
//     game's own sub-collection would then be two segments deep, which the
//     route conventions in docs/backend.md do not allow.
//   - /v1/shared/:id/sheet is one shared character, read by somebody who does
//     not own it. It hangs off nothing, because what grants the read is "some
//     group we are both in" and naming any one group in the URL would be a
//     lie about why it was allowed.
package game

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/middleware"
	"github.com/promix1722/easydnd/internal/domain/character"
	domain "github.com/promix1722/easydnd/internal/domain/game"
	"github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/user"
	gameuc "github.com/promix1722/easydnd/internal/usecase/game"
)

// CharacterQueryParam names the character a roster or table operation acts on.
//
// A query parameter rather than a second path segment, matching the way
// /v1/groups/:id/members addresses one member with ?user=: a character is
// named by an opaque id rather than by position, and the parent is already
// addressed.
const CharacterQueryParam = "character"

// Handler serves games and the shared table.
type Handler struct {
	service *gameuc.Service
	log     *slog.Logger
}

// New builds the handler over the game service.
func New(service *gameuc.Service, log *slog.Logger) *Handler {
	return &Handler{service: service, log: log}
}

// actor returns the account id the request is authenticated as.
//
// Every route here sits behind middleware.RequireSession, so UserFrom
// reporting nothing means a route was declared outside the guarded group -- a
// wiring bug, not a runtime condition. The zero id is returned in that case
// deliberately: it is in no group and owns no character, so a mis-wired route
// shows an empty table rather than somebody else's.
func (h *Handler) actor(c *gin.Context) user.ID {
	account, ok := middleware.UserFrom(c)
	if !ok {
		return ""
	}
	return account.ID
}

// groupOf reads the addressed group id.
func groupOf(c *gin.Context) group.ID { return group.ID(c.Param("id")) }

// gameOf reads the addressed game id.
func gameOf(c *gin.Context) domain.ID { return domain.ID(c.Param("id")) }

// pathCharacterOf reads the addressed character id from the path.
func pathCharacterOf(c *gin.Context) character.ID { return character.ID(c.Param("id")) }

// targetOf reads the character a roster or table operation acts on.
func targetOf(c *gin.Context) character.ID {
	return character.ID(c.Query(CharacterQueryParam))
}
