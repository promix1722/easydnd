package catalog

import "github.com/promix1722/easydnd/internal/domain/rules"

// Resolved is the catalogue entries one response refers to, in the shapes the
// collection routes serve them.
//
// A sheet carries slugs. Sending what they mean with it is what keeps a client
// from downloading whole collections to name a dozen things -- the spells
// collection alone is every spell in the rules with its artwork inlined.
// Skills are the exception to "only what is referenced": there are eighteen
// and a sheet draws every one.
type Resolved struct {
	Skills        []Skill       `json:"skills"`
	Proficiencies []Proficiency `json:"proficiencies,omitempty"`
	Equipment     []Item        `json:"equipment,omitempty"`
	MagicItems    []MagicItem   `json:"magicItems,omitempty"`
	Spells        []Spell       `json:"spells,omitempty"`
}

// Resolve looks the named entries up. A slug the catalogue does not define --
// a custom item, an entry a pack migration dropped -- is skipped, and the
// client falls back to the slug as it always has. Item slugs are asked of
// both item collections, because a stack does not say which it came from.
func (c Converter) Resolve(proficiencies, items, spells []string) Resolved {
	cat := c.inner.cat
	out := Resolved{Skills: mapAll(cat.Skills.All(), c.inner.skill)}
	for _, slug := range distinct(proficiencies) {
		if v, ok := cat.Proficiencies.Get(slug); ok {
			out.Proficiencies = append(out.Proficiencies, c.inner.proficiency(v))
		}
	}
	for _, slug := range distinct(items) {
		if v, ok := cat.Items.Get(slug); ok {
			out.Equipment = append(out.Equipment, c.inner.item(v))
		}
		if v, ok := cat.MagicItems.Get(slug); ok {
			out.MagicItems = append(out.MagicItems, c.inner.magicItem(v))
		}
	}
	for _, slug := range distinct(spells) {
		if v, ok := cat.Spells.Get(slug); ok {
			out.Spells = append(out.Spells, c.inner.spellSummary(v))
		}
	}
	return out
}

// Name is the localized name of one entry of a collection that a sheet needs
// nothing else from.
func (c Converter) Name(collection, slug string) (string, bool) {
	cat, s := c.inner.cat, rules.Slug(slug)
	switch collection {
	case CollectionRaces:
		v, ok := cat.Races.Get(s)
		return v.Name, ok
	case CollectionSubraces:
		v, ok := cat.Subraces.Get(s)
		return v.Name, ok
	case CollectionClasses:
		v, ok := cat.Classes.Get(s)
		return v.Name, ok
	case CollectionSubclasses:
		v, ok := cat.Subclasses.Get(s)
		return v.Name, ok
	case CollectionBackgrounds:
		v, ok := cat.Backgrounds.Get(s)
		return v.Name, ok
	case CollectionTraits:
		v, ok := cat.Traits.Get(s)
		return v.Name, ok
	case CollectionFeatures:
		v, ok := cat.Features.Get(s)
		return v.Name, ok
	case CollectionFeats:
		v, ok := cat.Feats.Get(s)
		return v.Name, ok
	case CollectionLanguages:
		v, ok := cat.Languages.Get(s)
		return v.Name, ok
	}
	return "", false
}

func distinct(slugs []string) []rules.Slug {
	seen := map[string]bool{}
	out := make([]rules.Slug, 0, len(slugs))
	for _, slug := range slugs {
		if slug != "" && !seen[slug] {
			seen[slug] = true
			out = append(out, rules.Slug(slug))
		}
	}
	return out
}
