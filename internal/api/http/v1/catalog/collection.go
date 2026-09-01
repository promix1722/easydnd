package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

// SlugsQueryParam narrows a collection to named entries.
const SlugsQueryParam = "slugs"

// maxSlugFilter bounds how many entries one request may name.
//
// Without a bound, ?slugs= is an unbounded loop over client-supplied input.
// The limit is generous -- far more than any screen needs -- because the
// point is to stop abuse, not to constrain use.
const maxSlugFilter = 200

// Collection handles GET /v1/catalog/{collection}.
//
// The whole collection is served by default. ?slugs=a,b narrows it, which is
// what a sheet uses to render the four spells a character has prepared
// without pulling all 319.
//
// Spells and magic items are served in summary at collection level -- slug,
// name, level, school, classes -- and in full only when named. That is the
// one place where asking for everything and asking for something return
// different shapes, and it is worth it: the full spell list is an order of
// magnitude larger than anything a build flow reads.
func (h *Handler) Collection(c *gin.Context) {
	ctx := c.Request.Context()
	locale := helpers.Locale(c)
	name := c.Param("collection")

	filter, err := parseSlugs(c.Query(SlugsQueryParam))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}

	if len(filter) == 0 {
		// Spells answer search parameters -- filtered, sorted, paged, in an
		// envelope. ?slugs= wins when both are sent: naming entries is the
		// more specific request.
		if name == CollectionSpells && hasSpellSearch(c) {
			search, err := parseSpellSearch(c)
			if err != nil {
				helpers.FormatError(c, err)
				return
			}
			h.searchSpells(c, search)
			return
		}
		raw, err := h.collectionBytes(ctx, locale, name)
		if err != nil {
			helpers.FormatError(c, err)
			return
		}
		// Already-marshalled bytes: the conversion happened once, on the
		// first request for this collection in this locale.
		c.Data(http.StatusOK, gin.MIMEJSON, raw)
		return
	}

	selected, err := h.selected(ctx, locale, name, filter)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, selected)
}

func parseSlugs(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) > maxSlugFilter {
		return nil, types.NewFieldValidationError("too many slugs requested", types.FieldError{
			Field: SlugsQueryParam, Rule: "max",
			Reason: "field.slugs.max",
		})
	}
	return out, nil
}

// selected returns the named entries of a collection.
//
// Spells are special-cased to full fidelity, because naming a spell is
// exactly the request that wants its casting time and damage. Everything else
// is filtered out of the same rendering the whole-collection path serves, so
// the two cannot disagree about an entry's shape.
func (h *Handler) selected(
	ctx context.Context, locale rules.Locale, name string, slugs []string,
) (any, error) {
	cat, err := h.source.Load(ctx, locale)
	if err != nil {
		return nil, err
	}
	conv := converter{cat: cat}

	if name == CollectionSpells {
		out := make([]Spell, 0, len(slugs))
		for _, slug := range slugs {
			if spell, ok := cat.Spells.Get(rules.Slug(slug)); ok {
				out = append(out, conv.spell(spell))
			}
		}
		return out, nil
	}

	raw, err := h.collectionBytes(ctx, locale, name)
	if err != nil {
		return nil, err
	}
	var all []json.RawMessage
	if err := json.Unmarshal(raw, &all); err != nil {
		return nil, types.WrapServerError(err, "re-reading the %s collection", name)
	}
	wanted := make(map[string]bool, len(slugs))
	for _, slug := range slugs {
		wanted[slug] = true
	}
	out := make([]json.RawMessage, 0, len(slugs))
	for _, entry := range all {
		var probe struct {
			Slug string `json:"slug"`
		}
		if err := json.Unmarshal(entry, &probe); err != nil {
			continue
		}
		if wanted[probe.Slug] {
			out = append(out, entry)
		}
	}
	return out, nil
}

// countOf reports how many entries a rendered collection holds.
func countOf(value any) int {
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Slice {
		return 0
	}
	return v.Len()
}
