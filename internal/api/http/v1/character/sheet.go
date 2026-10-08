package character

import (
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	catalogapi "github.com/promix1722/easydnd/internal/api/http/v1/catalog"
	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
)

// Sheet handles GET /v1/characters/{id}/sheet.
//
// This is the read path the whole event-sourced design exists to serve: the
// log folded against the compendium, in the negotiated locale.
func (h *Handler) Sheet(c *gin.Context) {
	state, cat, err := h.service.SheetWithCatalog(c.Request.Context(), h.owner(c), idOf(c), helpers.Locale(c))
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	c.JSON(http.StatusOK, ResolvedSheetOf(state, cat))
}

// nameKinds maps the kind of a reference to the collection that names it, for
// the references a sheet carries as "kind:slug" rather than as a bare slug.
var nameKinds = map[string]string{
	"class": catalogapi.CollectionClasses, "subclass": catalogapi.CollectionSubclasses,
	"race": catalogapi.CollectionRaces, "subrace": catalogapi.CollectionSubraces,
	"background": catalogapi.CollectionBackgrounds, "feat": catalogapi.CollectionFeats,
	"trait": catalogapi.CollectionTraits, "feature": catalogapi.CollectionFeatures,
}

// ResolvedSheetOf is SheetOf with every slug the sheet carries resolved against
// the catalogue it was projected from: a name for the entries a sheet only
// names, the entry itself for those a panel reads more of.
//
// It is what makes drawing a sheet one request. The client used to fetch
// fourteen whole collections to do this lookup itself, the spells collection
// among them. Exported for the game package, as SheetOf is.
func ResolvedSheetOf(s domain.State, cat *catalog.Catalog) Sheet {
	out := SheetOf(s)
	conv := catalogapi.NewConverter(cat)

	// Imported and custom entries were named by the projection; keep those.
	names := map[string]string{}
	name := func(collection, slug string) {
		key := collection + ":" + slug
		if _, named := out.CatalogNames[key]; named || slug == "" {
			return
		}
		if found, ok := conv.Name(collection, slug); ok {
			names[key] = found
		}
	}
	name(catalogapi.CollectionRaces, out.Identity.Race)
	name(catalogapi.CollectionSubraces, out.Identity.Subrace)
	name(catalogapi.CollectionBackgrounds, out.Identity.Background)
	for _, class := range out.Identity.Classes {
		name(catalogapi.CollectionClasses, class.Class)
		name(catalogapi.CollectionSubclasses, class.Subclass)
	}
	for collection, slugs := range map[string][]string{
		catalogapi.CollectionTraits: out.Traits, catalogapi.CollectionFeatures: out.Features,
		catalogapi.CollectionFeats: out.Feats, catalogapi.CollectionLanguages: out.Base.Languages,
	} {
		for _, slug := range slugs {
			name(collection, slug)
		}
	}

	spells := slices.Concat(out.Spells.Cantrips, out.Spells.Known, out.Spells.Prepared)
	for _, source := range out.Spells.Sources {
		spells = slices.Concat(spells, source.Cantrips, source.Known, source.Spellbook,
			source.Prepared, source.Arcanum, source.Mastery)
		if kind, slug, ok := strings.Cut(source.Source, ":"); ok {
			name(nameKinds[kind], slug)
		}
	}
	var items []string
	for _, stack := range slices.Concat(out.Equipment.Equipped, out.Equipment.Backpack, out.Equipment.Loot) {
		items = append(items, stack.Item)
	}

	if len(names) > 0 {
		if out.CatalogNames == nil {
			out.CatalogNames = map[string]string{}
		} else {
			out.CatalogNames = maps.Clone(out.CatalogNames)
		}
		maps.Copy(out.CatalogNames, names)
	}
	resolved := conv.Resolve(out.Proficiencies, items, spells)
	for _, a := range s.Actions {
		if desc := conv.Describe(a.Origin); len(desc) > 0 {
			resolved.Actions = append(resolved.Actions, catalogapi.Entry{Slug: a.Origin.String(), Name: a.Name, Desc: desc})
		}
	}
	out.Catalog = &resolved
	return out
}
