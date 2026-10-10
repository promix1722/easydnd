package game

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	catalogapi "github.com/promix1722/easydnd/internal/api/http/v1/catalog"
	characterapi "github.com/promix1722/easydnd/internal/api/http/v1/character"
	"github.com/promix1722/easydnd/internal/domain/catalog"
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
	state, cat, err := h.service.SheetWithCatalog(
		c.Request.Context(), h.actor(c), pathCharacterOf(c), helpers.Locale(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, characterapi.ResolvedSheetOf(state, cat))
}

func (h *Handler) Catalog(c *gin.Context) {
	cat, err := h.service.CharacterCatalog(c.Request.Context(), h.actor(c), pathCharacterOf(c), helpers.Locale(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	// The item search here is what a DM hands out from, and a character's own
	// custom items are not on offer: they are already that character's, and
	// nobody else's rules hold them. Asked for by name they still answer, which
	// is how a shared sheet draws them.
	if c.Param("collection") == catalogapi.CollectionItems {
		offered := *cat
		offered.Items = catalog.NewCollection(slices.DeleteFunc(cat.Items.All(), func(item catalog.Item) bool { return item.Manual }))
		cat = &offered
	}
	catalogapi.ServeCollection(c, cat)
}
