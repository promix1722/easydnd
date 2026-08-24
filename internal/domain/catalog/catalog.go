package catalog

import (
	"context"
	"slices"

	"github.com/promix1722/easydnd/internal/domain/rules"
)

// Key returns the entry's slug, satisfying Keyed for every entity that embeds
// Entry.
func (e Entry) Key() rules.Slug { return e.Slug }

// Keyed is anything a Collection can index. Every catalogue entity satisfies
// it by embedding Entry.
type Keyed interface {
	Key() rules.Slug
}

// Collection is an immutable, slug-indexed set of catalogue entries.
//
// Its fields are unexported so a *Catalog handed to a request cannot be
// modified by it: the compendium is loaded once and shared, and a caller that
// could reach the backing map could corrupt every other request.
type Collection[T Keyed] struct {
	items map[rules.Slug]T
	order []rules.Slug
}

// NewCollection indexes items by their slug. Iteration order is the slug
// order, sorted, so output is stable regardless of input order or map
// iteration.
func NewCollection[T Keyed](items []T) Collection[T] {
	c := Collection[T]{items: make(map[rules.Slug]T, len(items)), order: make([]rules.Slug, 0, len(items))}
	for _, item := range items {
		slug := item.Key()
		if _, seen := c.items[slug]; !seen {
			c.order = append(c.order, slug)
		}
		c.items[slug] = item
	}
	slices.Sort(c.order)
	return c
}

// Get returns the entry with the given slug. The second result reports
// whether it exists; a missing slug is a normal outcome for user input, not
// an error.
func (c Collection[T]) Get(slug rules.Slug) (T, bool) {
	item, ok := c.items[slug]
	return item, ok
}

// Has reports whether the slug names an entry in this collection.
func (c Collection[T]) Has(slug rules.Slug) bool {
	_, ok := c.items[slug]
	return ok
}

// All returns every entry in stable slug order. The returned slice is freshly
// allocated, so mutating it cannot affect the catalogue.
func (c Collection[T]) All() []T {
	out := make([]T, 0, len(c.order))
	for _, slug := range c.order {
		out = append(out, c.items[slug])
	}
	return out
}

// Slugs returns every slug in stable order.
func (c Collection[T]) Slugs() []rules.Slug { return slices.Clone(c.order) }

// Len returns the number of entries.
func (c Collection[T]) Len() int { return len(c.items) }

// Catalog is an immutable, locale-resolved SRD compendium.
//
// Callers never think about localization: the prose in every Entry is already
// in this catalogue's locale, having fallen back to rules.DefaultLocale key by
// key wherever a translation was missing. That is the whole point of resolving
// at load time rather than at read time -- the rules math and the application
// layer stay locale-free, and the HTTP layer simply picks the Catalog matching
// the negotiated Accept-Language.
//
// A Catalog is safe for concurrent use and must be treated as read-only.
type Catalog struct {
	locale rules.Locale

	// Ruleset is the rules edition the compendium was generated for, e.g.
	// "2014". It travels with the data rather than being a build-time
	// constant, because a 2024 compendium is a different directory, not a
	// different binary.
	Ruleset string

	Abilities           Collection[AbilityScore]
	Skills              Collection[Skill]
	Alignments          Collection[Alignment]
	Languages           Collection[Language]
	Conditions          Collection[Condition]
	DamageTypes         Collection[DamageType]
	MagicSchools        Collection[MagicSchool]
	WeaponProperties    Collection[WeaponProperty]
	Proficiencies       Collection[ProficiencyDef]
	EquipmentCategories Collection[EquipmentCategory]

	Races    Collection[Race]
	Subraces Collection[Subrace]
	Traits   Collection[Trait]

	Classes    Collection[Class]
	Subclasses Collection[Subclass]
	Features   Collection[Feature]

	Backgrounds Collection[Background]
	Feats       Collection[Feat]

	Items      Collection[Item]
	MagicItems Collection[MagicItem]
	Spells     Collection[Spell]

	// Terms is prose the choice grammar points at by key. It is the only
	// collection with no mechanics file behind it; see Term.
	Terms Collection[Term]

	// classLevels is indexed by class or subclass slug, then by level, since
	// every lookup is "what does a rogue get at 3rd level?" rather than a
	// scan.
	classLevels map[rules.Slug]map[int]ClassLevel
}

// New assembles a Catalog for a locale. It is called by an adapter after the
// mechanics files and the locale overlay have been merged; nothing in the
// application layer constructs one.
func New(locale rules.Locale, levels []ClassLevel) *Catalog {
	c := &Catalog{locale: locale, classLevels: make(map[rules.Slug]map[int]ClassLevel)}
	for _, level := range levels {
		owner := level.Class
		if !level.Subclass.IsZero() {
			owner = level.Subclass
		}
		if c.classLevels[owner] == nil {
			c.classLevels[owner] = make(map[int]ClassLevel)
		}
		c.classLevels[owner][level.Level] = level
	}
	return c
}

// Locale returns the language this catalogue's prose is in.
func (c *Catalog) Locale() rules.Locale { return c.locale }

// ClassLevel returns the advancement row for a class or subclass at a level.
// The second result reports whether that row exists.
func (c *Catalog) ClassLevel(owner rules.Slug, level int) (ClassLevel, bool) {
	byLevel, ok := c.classLevels[owner]
	if !ok {
		return ClassLevel{}, false
	}
	row, ok := byLevel[level]
	return row, ok
}

// ClassLevels returns every advancement row for a class or subclass, ordered
// by level.
func (c *Catalog) ClassLevels(owner rules.Slug) []ClassLevel {
	byLevel := c.classLevels[owner]
	out := make([]ClassLevel, 0, len(byLevel))
	for _, row := range byLevel {
		out = append(out, row)
	}
	slices.SortFunc(out, func(a, b ClassLevel) int { return a.Level - b.Level })
	return out
}

// Source supplies catalogue data for a locale.
//
// It is the compendium's only I/O boundary. Implementations live under
// internal/adapter; internal/app picks the concrete one, and that assignment
// is what proves conformance at compile time.
type Source interface {
	// Locales reports which locales this source can load, most complete
	// first. rules.DefaultLocale is always among them.
	Locales(ctx context.Context) ([]rules.Locale, error)

	// Load reads and resolves the compendium for one locale. Implementations
	// report a *types.ValidationError for data that does not parse, and a
	// *types.NotFoundError for a locale they do not carry.
	Load(ctx context.Context, locale rules.Locale) (*Catalog, error)
}
