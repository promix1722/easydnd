package character

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	domain "github.com/promix1722/easydnd/internal/domain/character"
)

// SetFolderParams is the body of PUT /v1/characters/{id}/folder.
type SetFolderParams struct {
	// Folder is the folder to file the character in. Empty means the
	// caller's default folder, which is how a client moves something back
	// without having to look its id up first.
	Folder string `json:"folder"`
}

// SetFolder handles PUT /v1/characters/{id}/folder.
//
// A folder is the one thing about a stored character that changes without an
// event, so it gets a route of its own rather than a PATCH on the character.
// That is not style: a general PATCH on /v1/characters/{id} would read as an
// invitation to patch a name, a level or a score, and the log is the only way
// any of those can change. A route named after the single mutable field cannot
// be misread that way.
func (h *Handler) SetFolder(c *gin.Context) {
	var params SetFolderParams
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, err)
		return
	}

	err := h.service.MoveCharacter(c.Request.Context(), h.owner(c), idOf(c),
		domain.FolderID(params.Folder))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
