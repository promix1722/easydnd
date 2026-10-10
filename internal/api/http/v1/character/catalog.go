package character

import (
	"github.com/gin-gonic/gin"
	"github.com/promix1722/easydnd/internal/api/http/helpers"
	catalogapi "github.com/promix1722/easydnd/internal/api/http/v1/catalog"
)

// Catalog handles GET /v1/characters/{id}/catalog/{collection}.
func (h *Handler) Catalog(c *gin.Context) {
	cat, err := h.service.CharacterCatalog(c.Request.Context(), h.owner(c), idOf(c), helpers.Locale(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	catalogapi.ServeCollection(c, cat)
}

// SpellSearch handles POST /v1/characters/{id}/catalog/spells/search.
func (h *Handler) SpellSearch(c *gin.Context) {
	cat, err := h.service.CharacterCatalog(c.Request.Context(), h.owner(c), idOf(c), helpers.Locale(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	catalogapi.ServeSpellSearch(c, cat)
}
