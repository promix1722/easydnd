// Package folder serves the folder resource.
//
// A folder is a named place one account files its characters, and nothing more
// than that. It is emphatically not a group of players: it has exactly one
// owner, it shares nothing with anybody, and the word "group" is kept out of
// this package so that the two ideas cannot be confused when the other one
// arrives.
//
// One exported handler per file, named after the action, with its request and
// response types beside it -- the same shape as the character resource next
// door.
package folder

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/middleware"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

// Handler serves the folder resource.
//
// It holds the character service rather than a folder service of its own,
// because the two rules that make folders more than a table -- every character
// is in one, and deleting one deletes the characters in it -- reach into both
// stores. Splitting the service would not split the rules; it would only make
// one of them call the other.
type Handler struct {
	service *charuc.Service
	log     *slog.Logger
}

// New builds the handler over the character service.
func New(service *charuc.Service, log *slog.Logger) *Handler {
	return &Handler{service: service, log: log}
}

// owner returns the account the request is authenticated as.
//
// The same function as the character handler's, and deliberately a copy rather
// than something shared: it is four lines, and the alternative is a package
// that exists to hold four lines two handlers each read once.
func (h *Handler) owner(c *gin.Context) domain.OwnerID {
	account, ok := middleware.UserFrom(c)
	if !ok {
		return ""
	}
	return domain.OwnerID(account.ID)
}

// idOf reads the folder id from the path.
func idOf(c *gin.Context) domain.FolderID { return domain.FolderID(c.Param("id")) }
