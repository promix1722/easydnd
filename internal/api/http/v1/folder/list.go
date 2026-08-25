package folder

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
)

// ListResponse is the body of GET /v1/folders.
type ListResponse struct {
	Folders []Folder `json:"folders"`
}

// List handles GET /v1/folders.
//
// It never answers with an empty list. An account that has never seen this
// route gets its default folder created by the read, which is what makes "every
// account always has a folder" true without a migration that walks every
// account that already exists.
func (h *Handler) List(c *gin.Context) {
	folders, err := h.service.Folders(c.Request.Context(), h.owner(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	out := make([]Folder, 0, len(folders))
	for _, f := range folders {
		out = append(out, folderOf(f))
	}
	c.JSON(http.StatusOK, ListResponse{Folders: out})
}
