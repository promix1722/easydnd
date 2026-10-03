package catalog

import "github.com/promix1722/easydnd/internal/domain/rules"

// ResolveChoice expands equipment categories without changing option identities.
// Both prompts and projection use this tree; category names remain on explicit
// sets because nested option keys are part of stored character history.
func (c *Catalog) ResolveChoice(choice rules.Choice) rules.Choice {
	if choice.From.Kind == rules.OptionsFromEquipmentCategory {
		choice.From.Kind = rules.OptionsExplicit
		choice.From.Collection = rules.RefItem
		choice.From.Options = []rules.Option{}
		if category, ok := c.EquipmentCategories.Get(choice.From.Category); ok {
			for _, slug := range category.Items {
				if c.Items.Has(slug) {
					choice.From.Options = append(choice.From.Options, rules.RefOption{Ref: rules.NewRef(rules.RefItem, slug), Count: 1})
				}
			}
		}
		// Choosing several weapons does not require different weapon types.
		choice.Repeatable = choice.Choose > 1
	}
	if choice.From.Kind == rules.OptionsFromCollection {
		var slugs []rules.Slug
		switch choice.From.Collection {
		case rules.RefLanguage:
			slugs = c.Languages.Slugs()
		case rules.RefFeat:
			slugs = c.Feats.Slugs()
		case rules.RefFeature:
			slugs = c.Features.Slugs()
		case rules.RefTrait:
			slugs = c.Traits.Slugs()
		case rules.RefSpell:
			slugs = c.Spells.Slugs()
		case rules.RefProficiency:
			slugs = c.Proficiencies.Slugs()
		default:
			return choice
		}
		choice.From.Kind = rules.OptionsExplicit
		choice.From.Options = []rules.Option{}
		for _, slug := range slugs {
			choice.From.Options = append(choice.From.Options, rules.RefOption{Ref: rules.NewRef(choice.From.Collection, slug), Count: 1})
		}
	}
	options := make([]rules.Option, 0, len(choice.From.Options))
	for _, option := range choice.From.Options {
		options = append(options, c.resolveOption(option))
	}
	choice.From.Options = options
	return choice
}

func (c *Catalog) resolveOption(option rules.Option) rules.Option {
	switch o := option.(type) {
	case rules.NestedOption:
		o.Choice = c.ResolveChoice(o.Choice)
		return o
	case rules.BundleOption:
		items := make([]rules.Option, 0, len(o.Items))
		for _, item := range o.Items {
			items = append(items, c.resolveOption(item))
		}
		o.Items = items
		return o
	default:
		return option
	}
}
