package game

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	"github.com/promix1722/easydnd/internal/domain/character"
)

// ShareParams is the body of POST /v1/groups/:id/characters.
type ShareParams struct {
	CharacterID string `json:"character_id"`
}

// Table handles GET /v1/groups/:id/characters: what the group has on its
// table. Every member sees all of it.
func (h *Handler) Table(c *gin.Context) {
	roster, err := h.service.SharedCharacters(
		c.Request.Context(), h.actor(c), groupOf(c), helpers.Locale(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, TableResponse{Characters: charactersOf(roster)})
}

// Share handles POST /v1/groups/:id/characters: a member puts one of their own
// characters on the table.
//
// Any rank may do this, including a player -- it is the whole of what a player
// does here. Sharing grants a read to everybody at the table and nothing else;
// the character stays editable only by its owner, through the character
// routes, which this package has no counterpart to.
func (h *Handler) Share(c *gin.Context) {
	var params ShareParams
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, err)
		return
	}

	ctx := c.Request.Context()
	actor := h.actor(c)
	id := groupOf(c)
	if err := h.service.Share(ctx, actor, id, character.ID(params.CharacterID)); err != nil {
		helpers.FormatError(c, err)
		return
	}
	// Read the table back rather than rendering the one character here, so the
	// response shape has exactly one code path and the client can replace the
	// list it is holding without merging.
	roster, err := h.service.SharedCharacters(ctx, actor, id, helpers.Locale(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusCreated, TableResponse{Characters: charactersOf(roster)})
}

// Unshare handles DELETE /v1/groups/:id/characters?character=: the character's
// owner takes it back, or a DM clears it off.
func (h *Handler) Unshare(c *gin.Context) {
	ctx := c.Request.Context()
	actor := h.actor(c)
	id := groupOf(c)
	if err := h.service.Unshare(ctx, actor, id, targetOf(c)); err != nil {
		helpers.FormatError(c, err)
		return
	}
	roster, err := h.service.SharedCharacters(ctx, actor, id, helpers.Locale(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, TableResponse{Characters: charactersOf(roster)})
}
