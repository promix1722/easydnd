package catalog

import (
	"slices"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/rules"
)

// SpellFilter is one search over the spell collection.
//
// It lives in the domain because "which spells can I cast as a bonus action?"
// is a rules question, and the structured CastingTime exists precisely so it
// is not a substring search. Every zero field matches everything, so the zero
// filter is the whole collection.
type SpellFilter struct {
	// Name matches case-insensitively anywhere in the localized name.
	Name string

	// Level matches exactly; nil matches every level. A pointer because 0 is
	// a real level -- the cantrips.
	Level *int

	// School and Class are slugs; empty matches everything.
	School rules.Slug
	Class  rules.Slug

	// CastingTime is a CastingTimeKind wire name -- "action", "bonus-action",
	// "reaction", "over-time". Empty matches everything.
	CastingTime string

	// Concentration and Ritual match the flag when set; nil matches both.
	Concentration *bool
	Ritual        *bool

	// Material filters on the material component; false selects the spells
	// castable without one, which is the filter people actually use.
	Material *bool
}

// Matches reports whether the spell satisfies every set field.
func (f SpellFilter) Matches(s Spell) bool {
	if f.Name != "" && !strings.Contains(strings.ToLower(s.Name), strings.ToLower(f.Name)) {
		return false
	}
	if f.Level != nil && s.Level != *f.Level {
		return false
	}
	if f.School != "" && s.School != f.School {
		return false
	}
	if f.Class != "" && !slices.Contains(s.Classes, f.Class) {
		return false
	}
	if f.CastingTime != "" && s.CastingTime.Kind.String() != f.CastingTime {
		return false
	}
	if f.Concentration != nil && s.Concentration != *f.Concentration {
		return false
	}
	if f.Ritual != nil && s.Ritual != *f.Ritual {
		return false
	}
	if f.Material != nil && s.Components.Material != *f.Material {
		return false
	}
	return true
}
