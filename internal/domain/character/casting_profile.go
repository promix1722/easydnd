package character

import (
	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// castingProfile resolves the selected archetype before the base class. The
// owner identifies the advancement table and spell choices; the character's
// level and class identity still belong to the parent class.
func castingProfile(cat *catalog.Catalog, taken ClassLevel) (rules.Slug, catalog.CastingProfile, bool) {
	if subclass, ok := cat.Subclasses.Get(taken.Subclass); ok && subclass.Class == taken.Class {
		if profile, ok := cat.Mechanics.Casting[taken.Subclass]; ok {
			return taken.Subclass, profile, true
		}
	}
	profile, ok := cat.Mechanics.Casting[taken.Class]
	return taken.Class, profile, ok
}

func castingAbility(cat *catalog.Catalog, class rules.Slug, profile catalog.CastingProfile) rules.Ability {
	if profile.Ability != "" {
		return profile.Ability
	}
	if entry, ok := cat.Classes.Get(class); ok && entry.Spellcasting != nil {
		return entry.Spellcasting.Ability
	}
	return ""
}
