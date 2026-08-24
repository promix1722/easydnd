// Package catalog holds the SRD 5.1 compendium: the static reference entities
// a character is built from -- races, classes, backgrounds, spells, equipment
// and the rest -- together with the port that supplies them.
//
// This is the innermost layer. It imports the standard library and
// internal/domain/rules, and nothing else: no gin, no net/http, no
// database/sql, and no JSON or database struct tags. Reading the data off disk
// is an adapter's job, which is why Source is an interface declared here and
// implemented under internal/adapter.
//
// Everything is one package because these entities are densely
// cross-referential -- a class points at features, features at spells, spells
// at damage types -- and splitting them would buy import cycles rather than
// isolation. References are always rules.Slug values, never Go pointers, so
// the graph stays acyclic and a *Catalog can be shared immutably.
//
// The compendium is read-only at runtime. It is regenerated from the vendored
// SRD dump by cmd/srdgen, never edited by the application.
package catalog

import "github.com/promix1722/easydnd/internal/domain/rules"

// Entry is the identity and prose every catalogue entity carries.
//
// Name and Desc are already resolved for the locale of the Catalog holding
// this entry: prose is merged in at load time, so nothing downstream of the
// adapter has to carry a locale or perform a fallback. Where a locale has no
// translation for a key, these hold the English text.
type Entry struct {
	// Slug is the stable, language-neutral identity.
	Slug rules.Slug

	// Name is the display name in the catalogue's locale.
	Name string

	// Desc is the description, one string per paragraph. The SRD's own data
	// is paragraph-split this way; joining it would lose list formatting that
	// several entries depend on.
	Desc []string
}

// Prerequisite is a condition that must hold before an entry applies: a
// minimum ability score, a level, or another entry already taken.
//
// Exactly one field is meaningful, selected by Kind. This is a small enough
// union that a struct beats an interface -- prerequisites are compared and
// copied far more often than they are switched on.
type Prerequisite struct {
	Kind PrerequisiteKind

	// Ability and MinimumScore apply when Kind is PrerequisiteAbility.
	Ability      rules.Ability
	MinimumScore int

	// Level applies when Kind is PrerequisiteLevel.
	Level int

	// Ref applies when Kind is PrerequisiteEntry.
	Ref rules.Ref
}

// PrerequisiteKind selects which fields of a Prerequisite are meaningful.
type PrerequisiteKind uint8

// The kinds of prerequisite the SRD states.
const (
	PrerequisiteNone PrerequisiteKind = iota
	// PrerequisiteAbility is "Strength 13 or higher".
	PrerequisiteAbility
	// PrerequisiteLevel is "5th level".
	PrerequisiteLevel
	// PrerequisiteEntry is "the Extra Attack feature".
	PrerequisiteEntry
)
