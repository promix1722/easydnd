package catalog

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
)

// CollectionInfo names one collection and how many entries it holds.
type CollectionInfo struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// ManifestResponse is the body of GET /v1/catalog.
//
// It is the compendium's index: what ruleset this is, which locales exist,
// and what can be fetched. A client reads it once and knows every URL under
// /v1/catalog without a hardcoded list.
type ManifestResponse struct {
	Ruleset     string           `json:"ruleset"`
	Locale      string           `json:"locale"`
	Locales     []string         `json:"locales"`
	Collections []CollectionInfo `json:"collections"`
}

// Manifest handles GET /v1/catalog.
func (h *Handler) Manifest(c *gin.Context) {
	ctx := c.Request.Context()
	locale := helpers.Locale(c)

	cat, err := h.source.Load(ctx, locale)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	available, err := h.source.Locales(ctx)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}

	locales := make([]string, 0, len(available))
	for _, l := range available {
		locales = append(locales, l.String())
	}

	conv := converter{cat: cat}
	collections := make([]CollectionInfo, 0, len(Collections()))
	for _, name := range Collections() {
		value, ok := entries(conv, name)
		if !ok {
			continue
		}
		collections = append(collections, CollectionInfo{Name: name, Count: countOf(value)})
	}

	c.JSON(http.StatusOK, ManifestResponse{
		Ruleset:     cat.Ruleset,
		Locale:      cat.Locale().String(),
		Locales:     locales,
		Collections: collections,
	})
}
