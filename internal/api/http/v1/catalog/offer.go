package catalog

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	domain "github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

// CollectionSpellFilters is not a collection of entries. It is served on the
// collection routes so that every scope a catalogue is read through -- the
// default one, a character's, a pack selection's -- has it without a route of
// its own.
const CollectionSpellFilters = "spell-filters"

// maxOffer bounds the slugs one search may name. An offer is a subset of the
// spells collection, so this is several catalogues' worth; like maxSlugFilter
// it is there to stop abuse, not to constrain use.
const maxOffer = 5000

// SpellFilterOptions is what the spell filters offer to choose from: the packs
// and books the catalogue's spells come out of, and the schools and classes.
//
// Sent on its own so that no screen downloads the spells to learn what they
// can be filtered by.
type SpellFilterOptions struct {
	Packs   []SpellPack   `json:"packs"`
	Sources []SpellSource `json:"sources"`
	Schools []Entry       `json:"schools"`
	Classes []Entry       `json:"classes"`
}

type SpellPack struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Version  string   `json:"version"`
	Versions []string `json:"versions"`
}

type SpellSource struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	PackID string `json:"packId"`
}

func spellFilterOptions(cat *domain.Catalog) SpellFilterOptions {
	out := SpellFilterOptions{Packs: []SpellPack{}, Sources: []SpellSource{}}
	packs, sources := map[string]bool{}, map[string]bool{}
	for _, spell := range cat.Spells.All() {
		p := spell.Provenance
		if p == nil {
			continue
		}
		if !packs[p.PackID] {
			packs[p.PackID] = true
			out.Packs = append(out.Packs, SpellPack{ID: p.PackID, Title: p.PackTitle, Version: p.Version, Versions: []string{p.Version}})
		}
		for _, source := range p.Sources {
			// SRD membership is an attribution, not a book a pack adds: only
			// the SRD pack itself lists it as a source to filter by.
			attribution := source.ID == "srd-5.1" || strings.HasSuffix(source.ID, ":srd-5.1")
			if sources[source.ID] || attribution && p.PackID != "srd-2014" {
				continue
			}
			sources[source.ID] = true
			out.Sources = append(out.Sources, SpellSource{ID: source.ID, Name: source.Name, PackID: p.PackID})
		}
	}
	out.Schools = mapAll(cat.MagicSchools.All(), func(v domain.MagicSchool) Entry { return nameOf(v.Entry) })
	out.Classes = mapAll(cat.Classes.All(), func(v domain.Class) Entry { return nameOf(v.Entry) })
	return out
}

// nameOf is an entry without its prose: a filter option is a slug and a name.
func nameOf(e domain.Entry) Entry {
	return Entry{Slug: e.Slug.String(), Name: e.Name}
}

// spellOfferRequest is the body of a spell search that has more to say than a
// query string can carry: an offer may name every spell in the rules.
type spellOfferRequest struct {
	Query         string   `json:"q"`
	Level         *int     `json:"level"`
	School        string   `json:"school"`
	Class         string   `json:"class"`
	CastingTime   string   `json:"castingTime"`
	Concentration *bool    `json:"concentration"`
	Ritual        *bool    `json:"ritual"`
	Material      *bool    `json:"material"`
	Packs         []string `json:"packs"`
	Sources       []string `json:"sources"`

	Only *struct {
		Slugs   []string `json:"slugs"`
		Fitting []struct {
			MinLevel int      `json:"minLevel"`
			MaxLevel int      `json:"maxLevel"`
			Classes  []string `json:"classes"`
		} `json:"fitting"`
	} `json:"only"`
	Exclude []string `json:"exclude"`

	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

func slugsOf(in []string) []rules.Slug {
	out := make([]rules.Slug, 0, len(in))
	for _, slug := range in {
		out = append(out, rules.Slug(slug))
	}
	return out
}

// SpellSearch handles POST {catalogue}/spells/search: the spells search of the
// collection route, with the parts that do not fit a URL.
//
// It is what a build screen pages its spell choices through. `only` is the
// offer -- the spells this character may pick, named or described by level and
// class list -- and `exclude` is what is already chosen. The client is never
// sent the whole list to work that out for itself.
func (h *Handler) SpellSearch(c *gin.Context) {
	var body spellOfferRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		helpers.FormatError(c, err)
		return
	}
	if body.Limit < 1 || body.Limit > maxPageSize {
		helpers.FormatError(c, invalidParam(ParamLimit))
		return
	}
	if body.Offset < 0 {
		helpers.FormatError(c, invalidParam(ParamOffset))
		return
	}
	if body.Level != nil && (*body.Level < 0 || *body.Level > domain.MaxSpellLevel) {
		helpers.FormatError(c, invalidParam(ParamLevel))
		return
	}
	named := len(body.Exclude)
	if body.Only != nil {
		named += len(body.Only.Slugs)
	}
	if named > maxOffer {
		helpers.FormatError(c, types.NewFieldValidationError("too many slugs named", types.FieldError{
			Field: SlugsQueryParam, Rule: "max", Reason: "field.slugs.max",
		}))
		return
	}

	search := spellSearch{limit: body.Limit, offset: body.Offset, filter: domain.SpellFilter{
		PackIDs: body.Packs, Sources: body.Sources,
		Name: strings.TrimSpace(body.Query), Level: body.Level,
		School: rules.Slug(body.School), Class: rules.Slug(body.Class), CastingTime: body.CastingTime,
		Concentration: body.Concentration, Ritual: body.Ritual, Material: body.Material,
		Exclude: slugsOf(body.Exclude),
	}}
	if body.Only != nil {
		offer := domain.SpellOffer{Slugs: slugsOf(body.Only.Slugs)}
		for _, fit := range body.Only.Fitting {
			offer.Fitting = append(offer.Fitting, domain.SpellLevels{MinLevel: fit.MinLevel, MaxLevel: fit.MaxLevel, Classes: slugsOf(fit.Classes)})
		}
		search.filter.Only = &offer
	}
	h.searchSpells(c, search)
}

// ServeSpellSearch is SpellSearch over an already authorized catalogue, as
// ServeCollection is Collection.
func ServeSpellSearch(c *gin.Context, cat *domain.Catalog) {
	New(fixedSource{cat}, slog.Default()).SpellSearch(c)
}

// spellFilters serves CollectionSpellFilters.
func (h *Handler) spellFilters(c *gin.Context) {
	cat, err := h.source.Load(c.Request.Context(), helpers.Locale(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, spellFilterOptions(cat))
}
