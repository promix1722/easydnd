package file

import (
	"encoding/json"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

// The mechanics files, relative to the data directory. Exported so whatever
// writes a pack uses exactly the names this package reads.
const (
	FileAbilities           = "abilities.json"
	FileSkills              = "skills.json"
	FileAlignments          = "alignments.json"
	FileLanguages           = "languages.json"
	FileConditions          = "conditions.json"
	FileDamageTypes         = "damage-types.json"
	FileMagicSchools        = "magic-schools.json"
	FileWeaponProperties    = "weapon-properties.json"
	FileProficiencies       = "proficiencies.json"
	FileEquipmentCategories = "equipment-categories.json"
	FileRaces               = "races.json"
	FileSubraces            = "subraces.json"
	FileTraits              = "traits.json"
	FileClasses             = "classes.json"
	FileClassLevels         = "class-levels.json"
	FileSubclasses          = "subclasses.json"
	FileFeatures            = "features.json"
	FileBackgrounds         = "backgrounds.json"
	FileFeats               = "feats.json"
	FileEquipment           = "equipment.json"
	FileMagicItems          = "magic-items.json"
	FileSpells              = "spells.json"
	FileTerms               = "terms.json"
)

// LocaleDir is the subdirectory holding the per-locale prose bundles.
const LocaleDir = "i18n"

// MechanicsFiles lists every language-neutral data file, in load order.
// A pack directory holds this exact set, and the loader reads it by name.
func MechanicsFiles() []string {
	return []string{
		FileAbilities, FileSkills, FileAlignments, FileLanguages,
		FileConditions, FileDamageTypes, FileMagicSchools, FileWeaponProperties,
		FileProficiencies, FileEquipmentCategories,
		FileRaces, FileSubraces, FileTraits,
		FileClasses, FileClassLevels, FileSubclasses, FileFeatures,
		FileBackgrounds, FileFeats,
		FileEquipment, FileMagicItems, FileSpells,
	}
}

// ProseFiles lists every file readBundles reads: the mechanics files, whose
// prose is keyed by the same slugs, plus terms.json.
//
// terms.json is the odd one out. It has no mechanics counterpart -- a term is
// prose and nothing else -- so it must be listed here rather than inferred
// from MechanicsFiles, and Catalog.Terms is built from the bundle's own keys.
//
// Exported because a pack holds exactly this set, once per locale. It is the
// same argument MechanicsFiles and the wire types make: one definition that
// the writer and the reader share cannot drift.
func ProseFiles() []string {
	return append(MechanicsFiles(), FileTerms)
}

// buildCatalog converts one rules context -- every pack's rows merged per
// collection, and the prose already resolved for the locale -- into a Catalog.
//
// Rows arrive as decoded JSON values because that is what merging and
// qualifying slugs across packs works on; each collection is encoded once
// more here so that it lands in its wire type through that type's own
// decoder, which is where the format's rules live.
func buildCatalog(entities map[string][]any, prose map[string]Bundle, ruleset string, locale rules.Locale) (*catalog.Catalog, error) {
	abilities, err := rows[AbilityScore](entities, FileAbilities)
	if err != nil {
		return nil, err
	}
	skills, err := rows[Skill](entities, FileSkills)
	if err != nil {
		return nil, err
	}
	alignments, err := rows[Named](entities, FileAlignments)
	if err != nil {
		return nil, err
	}
	languages, err := rows[Language](entities, FileLanguages)
	if err != nil {
		return nil, err
	}
	conditions, err := rows[Named](entities, FileConditions)
	if err != nil {
		return nil, err
	}
	damageTypes, err := rows[Named](entities, FileDamageTypes)
	if err != nil {
		return nil, err
	}
	magicSchools, err := rows[Named](entities, FileMagicSchools)
	if err != nil {
		return nil, err
	}
	weaponProperties, err := rows[Named](entities, FileWeaponProperties)
	if err != nil {
		return nil, err
	}
	proficiencies, err := rows[Proficiency](entities, FileProficiencies)
	if err != nil {
		return nil, err
	}
	equipmentCategories, err := rows[EquipmentCategory](entities, FileEquipmentCategories)
	if err != nil {
		return nil, err
	}
	races, err := rows[Race](entities, FileRaces)
	if err != nil {
		return nil, err
	}
	subraces, err := rows[Subrace](entities, FileSubraces)
	if err != nil {
		return nil, err
	}
	traits, err := rows[Trait](entities, FileTraits)
	if err != nil {
		return nil, err
	}
	classes, err := rows[Class](entities, FileClasses)
	if err != nil {
		return nil, err
	}
	classLevels, err := rows[ClassLevel](entities, FileClassLevels)
	if err != nil {
		return nil, err
	}
	subclasses, err := rows[Subclass](entities, FileSubclasses)
	if err != nil {
		return nil, err
	}
	features, err := rows[Feature](entities, FileFeatures)
	if err != nil {
		return nil, err
	}
	backgrounds, err := rows[Background](entities, FileBackgrounds)
	if err != nil {
		return nil, err
	}
	feats, err := rows[Feat](entities, FileFeats)
	if err != nil {
		return nil, err
	}
	equipment, err := rows[Item](entities, FileEquipment)
	if err != nil {
		return nil, err
	}
	magicItems, err := rows[MagicItem](entities, FileMagicItems)
	if err != nil {
		return nil, err
	}
	spells, err := rows[Spell](entities, FileSpells)
	if err != nil {
		return nil, err
	}

	c := &conv{where: "<pack>"}
	levels := mapEach(classLevels, func(w ClassLevel) catalog.ClassLevel { return c.classLevel(w) })

	built := catalog.New(locale, levels)
	built.Ruleset = ruleset
	built.Abilities = catalog.NewCollection(mapBundle(abilities, prose[collectionOf(FileAbilities)], c.abilityScore))
	built.Skills = catalog.NewCollection(mapBundle(skills, prose[collectionOf(FileSkills)], c.skill))
	built.Alignments = catalog.NewCollection(mapBundle(alignments, prose[collectionOf(FileAlignments)], c.alignment))
	built.Languages = catalog.NewCollection(mapBundle(languages, prose[collectionOf(FileLanguages)], c.language))
	built.Conditions = catalog.NewCollection(mapNamed[catalog.Condition](conditions, prose[collectionOf(FileConditions)]))
	built.DamageTypes = catalog.NewCollection(mapNamed[catalog.DamageType](damageTypes, prose[collectionOf(FileDamageTypes)]))
	built.MagicSchools = catalog.NewCollection(mapNamed[catalog.MagicSchool](magicSchools, prose[collectionOf(FileMagicSchools)]))
	built.WeaponProperties = catalog.NewCollection(mapNamed[catalog.WeaponProperty](weaponProperties, prose[collectionOf(FileWeaponProperties)]))
	built.Proficiencies = catalog.NewCollection(mapBundle(proficiencies, prose[collectionOf(FileProficiencies)], c.proficiency))
	built.EquipmentCategories = catalog.NewCollection(mapBundle(equipmentCategories, prose[collectionOf(FileEquipmentCategories)],
		func(w EquipmentCategory, b Bundle) catalog.EquipmentCategory {
			return catalog.EquipmentCategory{Entry: entry(w.Slug, b), Items: slugs(w.Items)}
		}))
	built.Races = catalog.NewCollection(mapBundle(races, prose[collectionOf(FileRaces)], c.race))
	built.Subraces = catalog.NewCollection(mapBundle(subraces, prose[collectionOf(FileSubraces)], c.subrace))
	built.Traits = catalog.NewCollection(mapBundle(traits, prose[collectionOf(FileTraits)], c.trait))
	built.Classes = catalog.NewCollection(mapBundle(classes, prose[collectionOf(FileClasses)], c.class))
	built.Subclasses = catalog.NewCollection(mapBundle(subclasses, prose[collectionOf(FileSubclasses)], c.subclass))
	built.Features = catalog.NewCollection(mapBundle(features, prose[collectionOf(FileFeatures)], c.feature))
	built.Backgrounds = catalog.NewCollection(mapBundle(backgrounds, prose[collectionOf(FileBackgrounds)], c.background))
	built.Feats = catalog.NewCollection(mapBundle(feats, prose[collectionOf(FileFeats)], c.feat))
	built.Items = catalog.NewCollection(mapBundle(equipment, prose[collectionOf(FileEquipment)], c.item))
	built.MagicItems = catalog.NewCollection(mapBundle(magicItems, prose[collectionOf(FileMagicItems)], c.magicItem))
	built.Spells = catalog.NewCollection(mapBundle(spells, prose[collectionOf(FileSpells)], c.spell))
	built.Terms = catalog.NewCollection(termsOf(prose[collectionOf(FileTerms)]))

	if err := c.Err(); err != nil {
		return nil, err
	}
	return built, nil
}

// collectionOf is a data file's collection name: the key its rows and its
// prose are held under.
func collectionOf(file string) string { return strings.TrimSuffix(file, ".json") }

// rows decodes one collection into its wire type.
func rows[T any](entities map[string][]any, file string) ([]T, error) {
	values := entities[collectionOf(file)]
	if values == nil {
		values = []any{}
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return nil, types.WrapServerError(err, "encoding %s", file)
	}
	var out []T
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, types.NewValidationError("<pack>/%s: %v", file, err)
	}
	return out, nil
}

// mapEach applies f to every element.
func mapEach[In, Out any](in []In, f func(In) Out) []Out {
	out := make([]Out, 0, len(in))
	for _, item := range in {
		out = append(out, f(item))
	}
	return out
}

// mapBundle applies a two-argument conversion, threading the locale bundle.
func mapBundle[In, Out any](in []In, b Bundle, f func(In, Bundle) Out) []Out {
	out := make([]Out, 0, len(in))
	for _, item := range in {
		out = append(out, f(item, b))
	}
	return out
}

// termsOf turns a prose bundle into a collection.
//
// Every other collection is driven by its mechanics file, with the bundle
// supplying names and descriptions for slugs the file already listed. Terms
// have no mechanics file, so the bundle's keys are the collection.
func termsOf(b Bundle) []catalog.Term {
	out := make([]catalog.Term, 0, len(b))
	for slug := range b {
		out = append(out, catalog.Term{Entry: entry(slug, b)})
	}
	return out
}

// mapNamed converts the collections whose every attribute is prose.
func mapNamed[Out ~struct{ catalog.Entry }](in []Named, b Bundle) []Out {
	out := make([]Out, 0, len(in))
	for _, item := range in {
		out = append(out, Out{entry(item.Slug, b)})
	}
	return out
}
