package folder

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
)

// RenameParams is the body of PATCH /v1/folders/{id}.
type RenameParams struct {
	Name string `json:"name"`
}

// Rename handles PATCH /v1/folders/{id}.
//
// The default folder can be renamed like any other. What an account cannot lose
// is the folder, not the word on it -- and a player who would rather call it
// "Active" is not doing anything the model has to prevent.
func (h *Handler) Rename(c *gin.Context) {
	var params RenameParams
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, err)
		return
	}

	renamed, err := h.service.RenameFolder(c.Request.Context(), h.owner(c), idOf(c), params.Name)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, folderOf(renamed))
}
