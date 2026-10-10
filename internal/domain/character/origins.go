package character

import (
	"slices"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// Origins says where each trait and feature a character holds came from: the
// class or subclass whose level granted it, the feature it was picked under,
// the feat, background or race behind it.
//
// A sheet lists "Parry" beside "Second Wind", and only one of them is something
// every fighter has. The nearest source wins: a maneuver names Maneuvers rather
// than the Battle Master, because the pick is the thing a player would go back
// to change. An entry nothing here accounts for -- an imported one -- is absent.
func Origins(s State, cat *catalog.Catalog) map[rules.Ref]rules.Ref {
	out := map[rules.Ref]rules.Ref{}
	offers := func(c *rules.Choice, kind rules.RefKind, owner rules.Ref) {
		if c == nil {
			return
		}
		for _, key := range rules.OptionKeys(c.From) {
			if entry := rules.NewRef(kind, key); entry != owner {
				out[entry] = owner
			}
		}
	}

	// Broadest first, so that a nearer source overwrites it.
	for _, slug := range s.Features {
		feature, _ := cat.Features.Get(slug)
		switch {
		case !feature.Subclass.IsZero():
			out[rules.NewRef(rules.RefFeature, slug)] = rules.NewRef(rules.RefSubclass, feature.Subclass)
		case !feature.Class.IsZero():
			out[rules.NewRef(rules.RefFeature, slug)] = rules.NewRef(rules.RefClass, feature.Class)
		}
	}
	if background, ok := cat.Backgrounds.Get(s.Identity.Background); ok && !background.Feature.IsZero() {
		out[rules.NewRef(rules.RefFeature, background.Feature)] = rules.NewRef(rules.RefBackground, background.Slug)
	}
	for _, slug := range s.Traits {
		trait, _ := cat.Traits.Get(slug)
		switch {
		case slices.Contains(trait.Subraces, s.Identity.Subrace):
			out[rules.NewRef(rules.RefTrait, slug)] = rules.NewRef(rules.RefSubrace, s.Identity.Subrace)
		case slices.Contains(trait.Races, s.Identity.Race):
			out[rules.NewRef(rules.RefTrait, slug)] = rules.NewRef(rules.RefRace, s.Identity.Race)
		}
	}
	for _, r := range cat.Mechanics.Rules {
		if active, err := activeRule(s, cat, r); err != nil || !active {
			continue
		}
		for _, e := range r.Effects {
			if e.Op == "grant" {
				out[e.Ref] = r.Owner
			}
		}
		for _, ch := range r.Choices {
			if ch.From.Kind == rules.OptionsExplicit && ch.Kind != rules.ChooseExpertise {
				for _, option := range ch.From.Options {
					if ref, ok := option.(rules.RefOption); ok {
						out[ref.Ref] = r.Owner
					}
				}
			}
		}
	}
	for _, slug := range s.Traits {
		if trait, ok := cat.Traits.Get(slug); ok && trait.Specific != nil {
			offers(trait.Specific.SubtraitOptions, rules.RefTrait, rules.NewRef(rules.RefTrait, slug))
		}
	}
	for _, slug := range s.Features {
		if feature, ok := cat.Features.Get(slug); ok && feature.Specific != nil {
			owner := rules.NewRef(rules.RefFeature, slug)
			offers(feature.Specific.SubfeatureOptions, rules.RefFeature, owner)
			offers(feature.Specific.EnemyTypeOptions, rules.RefFeature, owner)
			offers(feature.Specific.TerrainTypeOptions, rules.RefFeature, owner)
		}
	}
	return out
}
