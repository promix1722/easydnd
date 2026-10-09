package character

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
)

// AutoEquip handles POST /v1/characters/:id/auto-equip.
//
// The build screen calls it when the player presses Finish. It answers 204
// whether or not it wore anything: a character already dressed is not an
// error, and the sheet the client opens next is what shows the result.
func (h *Handler) AutoEquip(c *gin.Context) {
	if err := h.service.AutoEquip(c.Request.Context(), h.owner(c), idOf(c), helpers.Locale(c)); err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
