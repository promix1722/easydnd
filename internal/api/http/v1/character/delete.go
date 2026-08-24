package character

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
)

// Delete handles DELETE /v1/characters/{id}.
func (h *Handler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), h.owner(c), idOf(c)); err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
