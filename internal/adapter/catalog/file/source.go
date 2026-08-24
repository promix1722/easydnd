package file

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

// The mechanics files, relative to the data directory. Exported so cmd/srdgen
// writes exactly the names this package reads.
const (
	FileManifest            = "manifest.json"
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
// cmd/srdgen writes this exact set, and the manifest counts them.
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

// proseFiles lists every file readBundles reads: the mechanics files, whose
// prose is keyed by the same slugs, plus terms.json.
//
// terms.json is the odd one out. It has no mechanics counterpart -- a term is
// prose and nothing else -- so it must be listed here rather than inferred
// from MechanicsFiles, and Catalog.Terms is built from the bundle's own keys.
func proseFiles() []string {
	return append(MechanicsFiles(), FileTerms)
}

// Source loads the compendium from a directory of JSON files.
//
// Loading is lazy and cached: the first request for a locale reads and
// converts it, and every later request gets the same *catalog.Catalog. That is
// safe because a Catalog is immutable, and it matters because converting all
// 1,300 entries is not something to redo per request.
type Source struct {
	dir string

	mu     sync.Mutex
	loaded map[rules.Locale]*catalog.Catalog
}

// NewSource returns a Source reading from dir.
func NewSource(dir string) *Source {
	return &Source{dir: dir, loaded: make(map[rules.Locale]*catalog.Catalog)}
}

// Dir returns the directory this source reads from.
func (s *Source) Dir() string { return s.dir }

// Locales reports which locales the directory carries, ordered as
// rules.SupportedLocales orders them so the most complete comes first.
func (s *Source) Locales(_ context.Context) ([]rules.Locale, error) {
	entries, err := os.ReadDir(filepath.Join(s.dir, LocaleDir))
	if err != nil {
		return nil, types.WrapServerError(err, "reading locales from %s", s.dir)
	}
	present := make(map[rules.Locale]bool, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			present[rules.Locale(e.Name())] = true
		}
	}
	out := make([]rules.Locale, 0, len(present))
	for _, known := range rules.SupportedLocales() {
		if present[known] {
			out = append(out, known)
		}
	}
	if len(out) == 0 {
		return nil, types.NewNotFoundError("no supported locales in %s", filepath.Join(s.dir, LocaleDir))
	}
	return out, nil
}

// Load reads and resolves the compendium for one locale.
func (s *Source) Load(_ context.Context, locale rules.Locale) (*catalog.Catalog, error) {
	if !locale.IsSupported() {
		return nil, types.NewNotFoundError("unsupported locale %q", locale)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if cached, ok := s.loaded[locale]; ok {
		return cached, nil
	}

	built, err := s.build(locale)
	if err != nil {
		return nil, err
	}
	s.loaded[locale] = built
	return built, nil
}

// bundles holds the resolved prose for one locale, one Bundle per collection.
type bundles map[string]Bundle

// get returns the bundle for a mechanics file, or an empty one.
func (b bundles) get(file string) Bundle {
	if got, ok := b[file]; ok {
		return got
	}
	return Bundle{}
}

// build does the actual work of Load, without the cache.
func (s *Source) build(locale rules.Locale) (*catalog.Catalog, error) {
	prose, err := s.prose(locale)
	if err != nil {
		return nil, err
	}

	// Mechanics first: every file is read before anything is converted, so a
	// missing file is reported as a missing file rather than as a dangling
	// reference three collections later.
	abilities, err := read[AbilityScore](s.dir, FileAbilities)
	if err != nil {
		return nil, err
	}
	skills, err := read[Skill](s.dir, FileSkills)
	if err != nil {
		return nil, err
	}
	alignments, err := read[Named](s.dir, FileAlignments)
	if err != nil {
		return nil, err
	}
	languages, err := read[Language](s.dir, FileLanguages)
	if err != nil {
		return nil, err
	}
	conditions, err := read[Named](s.dir, FileConditions)
	if err != nil {
		return nil, err
	}
	damageTypes, err := read[Named](s.dir, FileDamageTypes)
	if err != nil {
		return nil, err
	}
	magicSchools, err := read[Named](s.dir, FileMagicSchools)
	if err != nil {
		return nil, err
	}
	weaponProperties, err := read[Named](s.dir, FileWeaponProperties)
	if err != nil {
		return nil, err
	}
	proficiencies, err := read[Proficiency](s.dir, FileProficiencies)
	if err != nil {
		return nil, err
	}
	equipmentCategories, err := read[EquipmentCategory](s.dir, FileEquipmentCategories)
	if err != nil {
		return nil, err
	}
	races, err := read[Race](s.dir, FileRaces)
	if err != nil {
		return nil, err
	}
	subraces, err := read[Subrace](s.dir, FileSubraces)
	if err != nil {
		return nil, err
	}
	traits, err := read[Trait](s.dir, FileTraits)
	if err != nil {
		return nil, err
	}
	classes, err := read[Class](s.dir, FileClasses)
	if err != nil {
		return nil, err
	}
	classLevels, err := read[ClassLevel](s.dir, FileClassLevels)
	if err != nil {
		return nil, err
	}
	subclasses, err := read[Subclass](s.dir, FileSubclasses)
	if err != nil {
		return nil, err
	}
	features, err := read[Feature](s.dir, FileFeatures)
	if err != nil {
		return nil, err
	}
	backgrounds, err := read[Background](s.dir, FileBackgrounds)
	if err != nil {
		return nil, err
	}
	feats, err := read[Feat](s.dir, FileFeats)
	if err != nil {
		return nil, err
	}
	equipment, err := read[Item](s.dir, FileEquipment)
	if err != nil {
		return nil, err
	}
	magicItems, err := read[MagicItem](s.dir, FileMagicItems)
	if err != nil {
		return nil, err
	}
	spells, err := read[Spell](s.dir, FileSpells)
	if err != nil {
		return nil, err
	}

	manifest, err := ReadManifest(s.dir)
	if err != nil {
		return nil, err
	}

	c := &conv{where: s.dir}
	levels := mapEach(classLevels, func(w ClassLevel) catalog.ClassLevel { return c.classLevel(w) })

	built := catalog.New(locale, levels)
	built.Ruleset = manifest.Ruleset
	built.Abilities = catalog.NewCollection(mapBundle(abilities, prose.get(FileAbilities), c.abilityScore))
	built.Skills = catalog.NewCollection(mapBundle(skills, prose.get(FileSkills), c.skill))
	built.Alignments = catalog.NewCollection(mapBundle(alignments, prose.get(FileAlignments), c.alignment))
	built.Languages = catalog.NewCollection(mapBundle(languages, prose.get(FileLanguages), c.language))
	built.Conditions = catalog.NewCollection(mapNamed[catalog.Condition](conditions, prose.get(FileConditions)))
	built.DamageTypes = catalog.NewCollection(mapNamed[catalog.DamageType](damageTypes, prose.get(FileDamageTypes)))
	built.MagicSchools = catalog.NewCollection(mapNamed[catalog.MagicSchool](magicSchools, prose.get(FileMagicSchools)))
	built.WeaponProperties = catalog.NewCollection(mapNamed[catalog.WeaponProperty](weaponProperties, prose.get(FileWeaponProperties)))
	built.Proficiencies = catalog.NewCollection(mapBundle(proficiencies, prose.get(FileProficiencies), c.proficiency))
	built.EquipmentCategories = catalog.NewCollection(mapBundle(equipmentCategories, prose.get(FileEquipmentCategories),
		func(w EquipmentCategory, b Bundle) catalog.EquipmentCategory {
			return catalog.EquipmentCategory{Entry: entry(w.Slug, b), Items: slugs(w.Items)}
		}))
	built.Races = catalog.NewCollection(mapBundle(races, prose.get(FileRaces), c.race))
	built.Subraces = catalog.NewCollection(mapBundle(subraces, prose.get(FileSubraces), c.subrace))
	built.Traits = catalog.NewCollection(mapBundle(traits, prose.get(FileTraits), c.trait))
	built.Classes = catalog.NewCollection(mapBundle(classes, prose.get(FileClasses), c.class))
	built.Subclasses = catalog.NewCollection(mapBundle(subclasses, prose.get(FileSubclasses), c.subclass))
	built.Features = catalog.NewCollection(mapBundle(features, prose.get(FileFeatures), c.feature))
	built.Backgrounds = catalog.NewCollection(mapBundle(backgrounds, prose.get(FileBackgrounds), c.background))
	built.Feats = catalog.NewCollection(mapBundle(feats, prose.get(FileFeats), c.feat))
	built.Items = catalog.NewCollection(mapBundle(equipment, prose.get(FileEquipment), c.item))
	built.MagicItems = catalog.NewCollection(mapBundle(magicItems, prose.get(FileMagicItems), c.magicItem))
	built.Spells = catalog.NewCollection(mapBundle(spells, prose.get(FileSpells), c.spell))
	built.Terms = catalog.NewCollection(termsOf(prose.get(FileTerms)))

	if err := c.Err(); err != nil {
		return nil, err
	}
	return built, nil
}

// prose reads the locale's bundles and merges the default locale underneath
// them, so a partial translation falls back key by key.
func (s *Source) prose(locale rules.Locale) (bundles, error) {
	fallback, err := s.readBundles(rules.DefaultLocale, true)
	if err != nil {
		return nil, err
	}
	if locale == rules.DefaultLocale {
		return fallback, nil
	}
	// A locale directory that does not exist is not an error: it means
	// nothing is translated yet, and everything falls back.
	preferred, err := s.readBundles(locale, false)
	if err != nil {
		return nil, err
	}
	merged := make(bundles, len(fallback))
	for file, base := range fallback {
		merged[file] = resolve(preferred[file], base)
	}
	return merged, nil
}

// readBundles reads every prose file for one locale. When required is false a
// missing file or directory yields an empty bundle rather than an error.
func (s *Source) readBundles(locale rules.Locale, required bool) (bundles, error) {
	out := make(bundles, len(proseFiles()))
	for _, file := range proseFiles() {
		path := filepath.Join(s.dir, LocaleDir, locale.String(), file)
		raw, err := os.ReadFile(path)
		if err != nil {
			if !required && errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, types.WrapServerError(err, "reading %s", path)
		}
		var bundle Bundle
		if err := json.Unmarshal(raw, &bundle); err != nil {
			return nil, types.NewValidationError("%s: %v", path, err)
		}
		out[file] = bundle
	}
	return out, nil
}

// read decodes one JSON array file into a slice.
func read[T any](dir, file string) ([]T, error) {
	path := filepath.Join(dir, file)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, types.WrapServerError(err, "reading %s", path)
	}
	var out []T
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, types.NewValidationError("%s: %v", path, err)
	}
	return out, nil
}

// ReadManifest returns the data directory's manifest.
func ReadManifest(dir string) (Manifest, error) {
	path := filepath.Join(dir, FileManifest)
	raw, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, types.WrapServerError(err, "reading %s", path)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Manifest{}, types.NewValidationError("%s: %v", path, err)
	}
	return m, nil
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
