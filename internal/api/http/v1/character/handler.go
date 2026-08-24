package character

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/middleware"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

// Handler serves the character resource.
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
// Every character route sits behind middleware.RequireSession, so the account
// is always present; UserFrom reporting otherwise means a route was declared
// outside the guarded group, which is a wiring bug rather than a runtime
// condition. Returning the zero OwnerID in that case is deliberate: it owns
// nothing, so a mis-wired route lists an empty party rather than somebody
// else's.
func (h *Handler) owner(c *gin.Context) domain.OwnerID {
	account, ok := middleware.UserFrom(c)
	if !ok {
		return ""
	}
	return domain.OwnerID(account.ID)
}

// idOf reads the character id from the path.
func idOf(c *gin.Context) domain.ID { return domain.ID(c.Param("id")) }

// FolderQueryParam names the query parameter that narrows a listing to one
// folder, and that says which folder a new character is filed in.
//
// Creating states its folder in the body, where the rest of the character's
// opening state already is. Importing cannot: the body of an import is the
// exported sheet itself, so the only place left to put it is the query.
const FolderQueryParam = "folder"

// folderOf reads the folder from the query, if the caller named one.
//
// The zero value is not an error and does not mean "no folder": it means the
// caller did not choose, and the application layer resolves it to their
// default. A character is never filed nowhere.
func folderOf(c *gin.Context) domain.FolderID {
	return domain.FolderID(c.Query(FolderQueryParam))
}
