package catalog

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	domain "github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

// The spell search parameters. Only the spells collection reads them: it is
// the one collection large enough to page and rich enough to filter, and the
// browser screen behind it is the one place in the client that searches.
const (
	ParamQuery         = "q"
	ParamLevel         = "level"
	ParamSchool        = "school"
	ParamClass         = "class"
	ParamCastingTime   = "castingTime"
	ParamConcentration = "concentration"
	ParamRitual        = "ritual"
	ParamMaterial      = "material"
	ParamLimit         = "limit"
	ParamOffset        = "offset"
)

// maxPageSize bounds one page. Same reasoning as maxSlugFilter: the point is
// to stop abuse, not to constrain use -- the whole collection is 319.
const maxPageSize = 200

var searchParams = []string{
	ParamQuery, ParamLevel, ParamSchool, ParamClass, ParamCastingTime,
	ParamConcentration, ParamRitual, ParamMaterial, ParamLimit, ParamOffset,
}

// spellSearch is a parsed search request: what to match, and which page.
type spellSearch struct {
	filter domain.SpellFilter
	limit  int // 0 means everything
	offset int
}

// hasSpellSearch reports whether the request carries any search parameter.
// Their absence keeps the whole-collection path -- and its byte cache --
// exactly as it was.
func hasSpellSearch(c *gin.Context) bool {
	for _, param := range searchParams {
		if _, ok := c.GetQuery(param); ok {
			return true
		}
	}
	return false
}

func parseSpellSearch(c *gin.Context) (spellSearch, error) {
	var s spellSearch
	s.filter.Name = strings.TrimSpace(c.Query(ParamQuery))
	s.filter.School = rules.Slug(c.Query(ParamSchool))
	s.filter.Class = rules.Slug(c.Query(ParamClass))
	s.filter.CastingTime = c.Query(ParamCastingTime)

	if raw, ok := c.GetQuery(ParamLevel); ok {
		level, err := strconv.Atoi(raw)
		if err != nil || level < 0 || level > domain.MaxSpellLevel {
			return s, invalidParam(ParamLevel)
		}
		s.filter.Level = &level
	}
	for param, target := range map[string]**bool{
		ParamConcentration: &s.filter.Concentration,
		ParamRitual:        &s.filter.Ritual,
		ParamMaterial:      &s.filter.Material,
	} {
		if raw, ok := c.GetQuery(param); ok {
			value, err := strconv.ParseBool(raw)
			if err != nil {
				return s, invalidParam(param)
			}
			*target = &value
		}
	}
	if raw, ok := c.GetQuery(ParamLimit); ok {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > maxPageSize {
			return s, invalidParam(ParamLimit)
		}
		s.limit = limit
	}
	if raw, ok := c.GetQuery(ParamOffset); ok {
		offset, err := strconv.Atoi(raw)
		if err != nil || offset < 0 {
			return s, invalidParam(ParamOffset)
		}
		s.offset = offset
	}
	return s, nil
}

func invalidParam(param string) error {
	return types.NewFieldValidationError("invalid "+param, types.FieldError{
		Field: param, Rule: "invalid",
		Reason: "field." + param + ".invalid",
	})
}

// searchSpells answers the spells collection filtered, sorted and paged.
//
// The response is an envelope rather than the bare array the plain collection
// path serves, because a page is meaningless without the total behind it --
// the client's "load more" and its count line both read it. Sorting is level
// then localized name, the order the browse screen shows, and it is stable
// across pages because the catalogue is immutable for the life of the
// process.
func (h *Handler) searchSpells(c *gin.Context, search spellSearch) {
	cat, err := h.source.Load(c.Request.Context(), helpers.Locale(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}

	matches := make([]domain.Spell, 0)
	for _, spell := range cat.Spells.All() {
		if search.filter.Matches(spell) {
			matches = append(matches, spell)
		}
	}
	slices.SortFunc(matches, func(a, b domain.Spell) int {
		if a.Level != b.Level {
			return a.Level - b.Level
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	total := len(matches)
	if search.offset < len(matches) {
		matches = matches[search.offset:]
	} else {
		matches = nil
	}
	if search.limit > 0 && search.limit < len(matches) {
		matches = matches[:search.limit]
	}

	conv := converter{cat: cat}
	out := SpellSearchResult{Spells: make([]Spell, 0, len(matches)), Total: total}
	for _, spell := range matches {
		out.Spells = append(out.Spells, conv.spellSummary(spell))
	}
	c.JSON(http.StatusOK, out)
}
