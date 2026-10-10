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

	// The item search's own filters; see searchItems.
	ParamWearable = "wearable"
	ParamCategory = "category"
	ParamMagic    = "magic"
)

// maxPageSize bounds one page. Same reasoning as maxSlugFilter: the point is
// to stop abuse, not to constrain use -- the whole collection is 319.
const maxPageSize = 200

var searchParams = []string{
	"pack", "source", ParamQuery, ParamLevel, ParamSchool, ParamClass, ParamCastingTime,
	ParamConcentration, ParamRitual, ParamMaterial, ParamLimit, ParamOffset,
	ParamWearable, ParamCategory, ParamMagic,
}

// spellSearch is a parsed search request: what to match, and which page.
type spellSearch struct {
	filter domain.SpellFilter
	limit  int // 0 means every match, which only the query-string search allows
	offset int
}

// hasSpellSearch reports whether the request carries any search parameter.
// Without one, and without ?slugs=, a request for spells is refused: see
// Collection.
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
	if v := c.Query("pack"); v != "" {
		s.filter.PackIDs = strings.Split(v, ",")
	}
	if v := c.Query("source"); v != "" {
		s.filter.Sources = strings.Split(v, ",")
	}
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

// searchItems answers CollectionItems: equipment and magic items together,
// sorted by name and paged like spells. Of the shared parser's fields it reads
// q, limit and offset, and it has three filters of its own:
//
//   - wearable: whether the item has a slot. The sheet's Equipment tab is the
//     items that have one and its Items tab the ones that do not, so each
//     tab's picker asks for its own half.
//   - category: one equipment category, by slug.
//   - magic: which of the two collections.
//
// The answer carries the categories present under the wearable scope alone --
// the options for the category filter, which would otherwise empty themselves
// as soon as one was picked, and which the client must not derive from data
// it is never sent whole.
func (h *Handler) searchItems(c *gin.Context, search spellSearch) {
	var wearable, magic *bool
	for param, target := range map[string]**bool{ParamWearable: &wearable, ParamMagic: &magic} {
		if raw, ok := c.GetQuery(param); ok {
			value, err := strconv.ParseBool(raw)
			if err != nil {
				helpers.FormatError(c, invalidParam(param))
				return
			}
			*target = &value
		}
	}
	category := c.Query(ParamCategory)

	cat, err := h.source.Load(c.Request.Context(), helpers.Locale(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	q := strings.ToLower(search.filter.Name)
	matches := make([]ItemHit, 0)
	present := map[rules.Slug]bool{}
	// keep applies every filter to one item, noting its category on the way
	// past the wearable scope.
	keep := func(name string, itemCategory rules.Slug, slot domain.Slot, isMagic bool) bool {
		if wearable != nil && *wearable != (slot != domain.SlotNone) {
			return false
		}
		present[itemCategory] = true
		return strings.Contains(strings.ToLower(name), q) &&
			(category == "" || category == itemCategory.String()) &&
			(magic == nil || *magic == isMagic)
	}
	categoryName := func(slug rules.Slug) string {
		entry, _ := cat.EquipmentCategories.Get(slug)
		return entry.Name
	}
	for _, item := range cat.Items.All() {
		if keep(item.Name, item.Category, item.Slot, false) {
			matches = append(matches, ItemHit{
				Slug: item.Slug.String(), Icon: item.Icon, Name: item.Name,
				Category: item.Category.String(), CategoryName: categoryName(item.Category),
				Cost: costOf(item.Cost), Weight: item.Weight,
			})
		}
	}
	for _, item := range cat.MagicItems.All() {
		if keep(item.Name, item.Category, item.Slot, true) {
			matches = append(matches, ItemHit{
				Slug: item.Slug.String(), Icon: item.Icon, Name: item.Name,
				Category: item.Category.String(), CategoryName: categoryName(item.Category),
				Magic: true,
			})
		}
	}
	slices.SortFunc(matches, func(a, b ItemHit) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	categories := make([]ItemCategory, 0, len(present))
	for slug := range present {
		if slug != "" {
			categories = append(categories, ItemCategory{Slug: slug.String(), Name: categoryName(slug)})
		}
	}
	slices.SortFunc(categories, func(a, b ItemCategory) int {
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
	c.JSON(http.StatusOK, ItemSearchResult{Items: append([]ItemHit{}, matches...), Total: total, Categories: categories})
}

// ParseSpellSearch shares validation between scoped and aggregate catalogue searches.
func ParseSpellSearch(c *gin.Context) (domain.SpellFilter, int, int, error) {
	s, err := parseSpellSearch(c)
	return s.filter, s.limit, s.offset, err
}
