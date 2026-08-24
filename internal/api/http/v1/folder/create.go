package folder

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
)

// CreateParams is the body of POST /v1/folders.
type CreateParams struct {
	Name string `json:"name"`
}

// Create handles POST /v1/folders.
func (h *Handler) Create(c *gin.Context) {
	var params CreateParams
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, err)
		return
	}

	created, err := h.service.CreateFolder(c.Request.Context(), h.owner(c), params.Name)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusCreated, folderOf(created))
}
