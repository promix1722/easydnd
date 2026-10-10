package character

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
)

// Visibility is who may read a character besides its owner and its groups.
type Visibility struct {
	// Public opens the sheet to anybody signed in who has its link.
	Public bool `json:"public"`
}

// GetVisibility handles GET /v1/characters/{id}/visibility.
func (h *Handler) GetVisibility(c *gin.Context) {
	character, err := h.service.Get(c.Request.Context(), h.owner(c), idOf(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, Visibility{Public: character.Public})
}

// SetVisibility handles PUT /v1/characters/{id}/visibility.
//
// Its own route for the reason the folder has one: it is a thing about a
// stored character that changes without an event, and a general PATCH would
// read as an invitation to patch what only the log may change.
func (h *Handler) SetVisibility(c *gin.Context) {
	var params Visibility
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, err)
		return
	}
	if err := h.service.SetPublic(c.Request.Context(), h.owner(c), idOf(c), params.Public); err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, params)
}
