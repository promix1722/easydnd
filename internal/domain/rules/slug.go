// Package rules holds the value objects shared by every D&D entity: identity,
// the ability/size/coin enumerations, dice, money, and the recursive "choose N
// of these" grammar the SRD uses for proficiencies and starting equipment.
//
// This is the innermost layer. It imports the standard library and nothing
// else: no gin, no net/http, no database/sql, and no JSON or database struct
// tags. Serialization and persistence details belong to the adapters, so that
// changing either one cannot ripple inward.
//
// Nothing here knows about a specific character or a specific catalogue entry.
// internal/domain/catalog and internal/domain/character both import this
// package; it imports neither.
package rules

import "strings"

// Slug is the stable identity of a catalogue entry, in the SRD's own
// lower-kebab form: "acid-arrow", "half-elf", "cunning-action".
//
// A slug is deliberately language-neutral. Display names live in the locale
// bundles and change per language; the slug never does, which is what lets a
// stored character keep meaning after the catalogue is regenerated or a new
// translation lands.
type Slug string

// String returns the slug's text.
func (s Slug) String() string { return string(s) }

// IsZero reports whether the slug is unset.
func (s Slug) IsZero() bool { return s == "" }

// RefKind names which catalogue collection a Ref points into. It exists so a
// reference carries its own type: "cunning-action" alone is ambiguous between
// a feature and a trait, and resolving the wrong collection is the kind of bug
// that surfaces as a silently missing action three layers away.
type RefKind uint8

// The catalogue collections a Ref can address.
const (
	RefNone RefKind = iota
	RefAbility
	RefSkill
	RefAlignment
	RefLanguage
	RefCondition
	RefDamageType
	RefMagicSchool
	RefWeaponProperty
	RefProficiency
	RefEquipmentCategory
	RefRace
	RefSubrace
	RefTrait
	RefClass
	RefSubclass
	RefFeature
	RefBackground
	RefFeat
	RefItem
	RefMagicItem
	RefSpell
)

var refKindNames = map[RefKind]string{
	RefNone:              "none",
	RefAbility:           "ability",
	RefSkill:             "skill",
	RefAlignment:         "alignment",
	RefLanguage:          "language",
	RefCondition:         "condition",
	RefDamageType:        "damage-type",
	RefMagicSchool:       "magic-school",
	RefWeaponProperty:    "weapon-property",
	RefProficiency:       "proficiency",
	RefEquipmentCategory: "equipment-category",
	RefRace:              "race",
	RefSubrace:           "subrace",
	RefTrait:             "trait",
	RefClass:             "class",
	RefSubclass:          "subclass",
	RefFeature:           "feature",
	RefBackground:        "background",
	RefFeat:              "feat",
	RefItem:              "item",
	RefMagicItem:         "magic-item",
	RefSpell:             "spell",
}

// String returns the kind's wire name, or "unknown" for a value outside the
// enumeration.
func (k RefKind) String() string {
	if name, ok := refKindNames[k]; ok {
		return name
	}
	return "unknown"
}

// ParseRefKind maps a wire name back to its RefKind. The second result reports
// whether the name was recognised.
func ParseRefKind(s string) (RefKind, bool) {
	for kind, name := range refKindNames {
		if name == s {
			return kind, true
		}
	}
	return RefNone, false
}

// Ref is a typed pointer into the catalogue.
//
// References are always slugs, never Go pointers. That is what keeps the
// on-disk data acyclic, lets a *Catalog be shared immutably across requests,
// and lets a character's log survive a catalogue regeneration.
type Ref struct {
	Kind RefKind
	Slug Slug
}

// NewRef builds a Ref of the given kind.
func NewRef(kind RefKind, slug Slug) Ref { return Ref{Kind: kind, Slug: slug} }

// IsZero reports whether the reference is unset.
func (r Ref) IsZero() bool { return r.Kind == RefNone || r.Slug == "" }

// String renders the reference as "kind:slug", the form used in log entries
// and error messages.
func (r Ref) String() string { return r.Kind.String() + ":" + r.Slug.String() }

// ParseRef reads the "kind:slug" form produced by Ref.String. The second
// result reports whether the text was well formed.
func ParseRef(s string) (Ref, bool) {
	kindText, slug, found := strings.Cut(s, ":")
	if !found || slug == "" {
		return Ref{}, false
	}
	kind, ok := ParseRefKind(kindText)
	if !ok || kind == RefNone {
		return Ref{}, false
	}
	return Ref{Kind: kind, Slug: Slug(slug)}, true
}
