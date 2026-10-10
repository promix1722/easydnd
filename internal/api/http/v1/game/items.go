package game

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	catalogapi "github.com/promix1722/easydnd/internal/api/http/v1/catalog"
	characterapi "github.com/promix1722/easydnd/internal/api/http/v1/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

// ItemParams names what changes hands. To is the receiving entry, and is read
// only by GiveItem.
type ItemParams struct {
	Item  string `json:"item"`
	Count int    `json:"count"`
	To    string `json:"to"`
}

// CoinsParams is a signed number of one coin.
type CoinsParams struct {
	Unit   string `json:"unit"`
	Amount int    `json:"amount"`
}

// GrantItem is the DM giving a seated character an item.
func (h *Handler) GrantItem(c *gin.Context) {
	var params ItemParams
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, types.NewValidationError("invalid item"))
		return
	}
	ctx := c.Request.Context()
	if err := h.service.GrantItem(ctx, h.actor(c), gameOf(c), c.Param("entry"), rules.Slug(params.Item), params.Count); err != nil {
		helpers.FormatError(c, err)
		return
	}
	h.detail(c, ctx, h.actor(c), gameOf(c), http.StatusOK)
}

// AdjustCoins is the DM adding to a seated character's purse, or taking from it.
func (h *Handler) AdjustCoins(c *gin.Context) {
	var params CoinsParams
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, types.NewValidationError("invalid coins"))
		return
	}
	unit, err := rules.ParseCoinUnit(params.Unit)
	if err != nil {
		helpers.FormatError(c, types.NewValidationError("unknown coin"))
		return
	}
	ctx := c.Request.Context()
	if err := h.service.AdjustCoins(ctx, h.actor(c), gameOf(c), c.Param("entry"), unit, params.Amount); err != nil {
		helpers.FormatError(c, err)
		return
	}
	h.detail(c, ctx, h.actor(c), gameOf(c), http.StatusOK)
}

// GiveItem is a player passing one of their own items across the table.
func (h *Handler) GiveItem(c *gin.Context) {
	var params ItemParams
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, types.NewValidationError("invalid item"))
		return
	}
	ctx := c.Request.Context()
	if err := h.service.GiveItem(ctx, h.actor(c), gameOf(c), c.Param("entry"), params.To, rules.Slug(params.Item), params.Count); err != nil {
		helpers.FormatError(c, err)
		return
	}
	h.detail(c, ctx, h.actor(c), gameOf(c), http.StatusOK)
}

// GrantCustomItem is the DM giving a seated character an item written on the
// spot. The body names no id, placement or count: the route adds one new
// item and does nothing else.
func (h *Handler) GrantCustomItem(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
	var params struct {
		Name        string           `json:"name"`
		Description string           `json:"description"`
		Item        *catalogapi.Item `json:"item"`
	}
	if err := c.ShouldBindJSON(&params); err != nil {
		helpers.FormatError(c, types.NewValidationError("invalid item"))
		return
	}
	item, err := characterapi.CustomItemOf(params.Item)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	if err = h.service.GrantCustomItem(c.Request.Context(), h.actor(c), gameOf(c), c.Param("entry"), helpers.Locale(c), params.Name, params.Description, item); err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
