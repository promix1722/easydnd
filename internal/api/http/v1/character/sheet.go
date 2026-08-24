package character

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
)

// Sheet handles GET /v1/characters/{id}/sheet.
//
// This is the read path the whole event-sourced design exists to serve: the
// log folded against the compendium, in the negotiated locale.
func (h *Handler) Sheet(c *gin.Context) {
	state, err := h.service.Sheet(c.Request.Context(), h.owner(c), idOf(c), helpers.Locale(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, sheetOf(state))
}
