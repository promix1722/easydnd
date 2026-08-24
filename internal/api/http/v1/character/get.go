package character

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
)

// Get handles GET /v1/characters/{id}, returning the log itself.
//
// The log rather than the sheet, because the sheet has its own route: this is
// the record, and /sheet is what the record means.
func (h *Handler) Get(c *gin.Context) {
	character, err := h.service.Get(c.Request.Context(), h.owner(c), idOf(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, characterOf(character))
}
