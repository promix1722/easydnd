package catalog

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	domain "github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

// Handler serves the compendium.
type Handler struct {
	source domain.Source
	log    *slog.Logger

	// rendered caches the marshalled bytes of each whole collection, keyed by
	// locale and collection name.
	//
	// The compendium is immutable for the lifetime of the process -- it is
	// read from a release directory at startup and never written -- so
	// converting and marshalling it is a once-per-process cost rather than a
	// per-request one. That is what makes serving it from the domain as
	// cheap as serving a static file, while still resolving prose and still
	// supporting ?slugs=.
	rendered sync.Map
}

// New builds the handler over a catalogue source.
func New(source domain.Source, log *slog.Logger) *Handler {
	return &Handler{source: source, log: log}
}

// renderKey identifies one cached rendering.
type renderKey struct {
	locale     rules.Locale
	collection string
}

// The collection names.
//
// They are the names of the generated files without their extension, which is
// the vocabulary manifest.json and data/srd_5.1/ already use. Inventing a
// third spelling of "the collections" is how a manifest and a router drift
// apart.
const (
	CollectionAbilities           = "abilities"
	CollectionSkills              = "skills"
	CollectionAlignments          = "alignments"
	CollectionLanguages           = "languages"
	CollectionConditions          = "conditions"
	CollectionDamageTypes         = "damage-types"
	CollectionMagicSchools        = "magic-schools"
	CollectionWeaponProperties    = "weapon-properties"
	CollectionProficiencies       = "proficiencies"
	CollectionEquipmentCategories = "equipment-categories"
	CollectionRaces               = "races"
	CollectionSubraces            = "subraces"
	CollectionTraits              = "traits"
	CollectionClasses             = "classes"
	CollectionClassLevels         = "class-levels"
	CollectionSubclasses          = "subclasses"
	CollectionFeatures            = "features"
	CollectionBackgrounds         = "backgrounds"
	CollectionFeats               = "feats"
	CollectionEquipment           = "equipment"
	CollectionMagicItems          = "magic-items"
	CollectionSpells              = "spells"
	CollectionTerms               = "terms"
)

// Collections lists every collection, in the order the manifest does.
func Collections() []string {
	return []string{
		CollectionAbilities, CollectionSkills, CollectionAlignments, CollectionLanguages,
		CollectionConditions, CollectionDamageTypes, CollectionMagicSchools,
		CollectionWeaponProperties, CollectionProficiencies, CollectionEquipmentCategories,
		CollectionRaces, CollectionSubraces, CollectionTraits,
		CollectionClasses, CollectionClassLevels, CollectionSubclasses, CollectionFeatures,
		CollectionBackgrounds, CollectionFeats,
		CollectionEquipment, CollectionMagicItems, CollectionSpells,
		CollectionTerms,
	}
}

// entries converts one collection to its response shapes.
//
// Every branch returns a slice of a different type, so the result is []any:
// the alternative is twenty-three near-identical handlers, and the value is
// marshalled immediately either way.
func entries(c converter, collection string) (any, bool) {
	cat := c.cat
	switch collection {
	case CollectionAbilities:
		return mapAll(cat.Abilities.All(), c.ability), true
	case CollectionSkills:
		return mapAll(cat.Skills.All(), c.skill), true
	case CollectionAlignments:
		return mapAll(cat.Alignments.All(), c.alignment), true
	case CollectionLanguages:
		return mapAll(cat.Languages.All(), c.language), true
	case CollectionConditions:
		return mapAll(cat.Conditions.All(), func(v domain.Condition) Entry { return entryOf(v.Entry) }), true
	case CollectionDamageTypes:
		return mapAll(cat.DamageTypes.All(), func(v domain.DamageType) Entry { return entryOf(v.Entry) }), true
	case CollectionMagicSchools:
		return mapAll(cat.MagicSchools.All(), func(v domain.MagicSchool) Entry { return entryOf(v.Entry) }), true
	case CollectionWeaponProperties:
		return mapAll(cat.WeaponProperties.All(), func(v domain.WeaponProperty) Entry { return entryOf(v.Entry) }), true
	case CollectionProficiencies:
		return mapAll(cat.Proficiencies.All(), c.proficiency), true
	case CollectionEquipmentCategories:
		return mapAll(cat.EquipmentCategories.All(), c.equipmentCategory), true
	case CollectionRaces:
		return mapAll(cat.Races.All(), c.race), true
	case CollectionSubraces:
		return mapAll(cat.Subraces.All(), c.subrace), true
	case CollectionTraits:
		return mapAll(cat.Traits.All(), c.trait), true
	case CollectionClasses:
		return mapAll(cat.Classes.All(), c.class), true
	case CollectionClassLevels:
		return c.allClassLevels(), true
	case CollectionSubclasses:
		return mapAll(cat.Subclasses.All(), c.subclass), true
	case CollectionFeatures:
		return mapAll(cat.Features.All(), c.feature), true
	case CollectionBackgrounds:
		return mapAll(cat.Backgrounds.All(), c.background), true
	case CollectionFeats:
		return mapAll(cat.Feats.All(), c.feat), true
	case CollectionEquipment:
		return mapAll(cat.Items.All(), c.item), true
	case CollectionMagicItems:
		return mapAll(cat.MagicItems.All(), c.magicItem), true
	case CollectionSpells:
		return mapAll(cat.Spells.All(), c.spellSummary), true
	case CollectionTerms:
		return mapAll(cat.Terms.All(), c.termEntry), true
	}
	return nil, false
}

// allClassLevels flattens every class's and subclass's advancement rows.
func (c converter) allClassLevels() []ClassLevel {
	var out []ClassLevel
	for _, class := range c.cat.Classes.All() {
		for _, row := range c.cat.ClassLevels(class.Slug) {
			out = append(out, c.classLevel(row))
		}
		for _, subclass := range class.Subclasses {
			for _, row := range c.cat.ClassLevels(subclass) {
				out = append(out, c.classLevel(row))
			}
		}
	}
	return out
}

func mapAll[In, Out any](in []In, f func(In) Out) []Out {
	out := make([]Out, 0, len(in))
	for _, item := range in {
		out = append(out, f(item))
	}
	return out
}

// collectionBytes returns the marshalled whole collection for a locale,
// rendering it on first use.
func (h *Handler) collectionBytes(ctx context.Context, locale rules.Locale, collection string) ([]byte, error) {
	key := renderKey{locale: locale, collection: collection}
	if cached, ok := h.rendered.Load(key); ok {
		return cached.([]byte), nil
	}

	cat, err := h.source.Load(ctx, locale)
	if err != nil {
		return nil, err
	}
	value, ok := entries(converter{cat: cat}, collection)
	if !ok {
		return nil, types.NewNotFoundError("no catalogue collection named %q", collection)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, types.WrapServerError(err, "rendering the %s collection", collection)
	}
	h.rendered.Store(key, raw)
	return raw, nil
}
