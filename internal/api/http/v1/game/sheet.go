package game

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	characterapi "github.com/promix1722/easydnd/internal/api/http/v1/character"
)

// Sheet handles GET /v1/shared/:id/sheet: a character's projected sheet, read
// by somebody at a table it was shared with.
//
// The sheet and nothing else. /v1/characters/:id is a character's log -- the
// record of every decision its owner made and the order they made them in --
// and the table has no business with that; it wants to know what the character
// *is*. That is why this route is named after the projection rather than
// mirroring the character tree, and why there is no /v1/shared/:id beside it.
//
// It renders through the character package's own converter, so a shared sheet
// and your own are produced by one code path and cannot drift into two shapes
// the client would have to tell apart.
func (h *Handler) Sheet(c *gin.Context) {
	state, err := h.service.Sheet(
		c.Request.Context(), h.actor(c), pathCharacterOf(c), helpers.Locale(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, characterapi.SheetOf(state))
}
