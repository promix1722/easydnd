package character

import (
	"slices"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// CustomOption preserves a player's own definition without asserting rules
// that are missing. Nil numbers are unknown, not inferred defaults.
type CustomOption struct {
	ID, Kind, Name, Description, Source, Parent, Ability, Mode, Placement, Reference string
	Level, HitDie, Speed                                                             *int
	Count                                                                            int
	Selected                                                                         bool
}

func (c CustomOption) Slug() rules.Slug {
	if ref, ok := rules.ParseRef(c.Reference); ok {
		return ref.Slug
	}
	return rules.Slug("custom-" + c.ID)
}
func CustomOptions(log Log) []CustomOption {
	out := []CustomOption{}
	for _, e := range log.Events {
		if e.Custom != nil {
			i := slices.IndexFunc(out, func(c CustomOption) bool { return c.ID == e.Custom.ID })
			if i < 0 {
				out = append(out, *e.Custom)
			} else {
				out[i] = *e.Custom
			}
		}
	}
	return out
}

// WithCustomCatalog makes a request-local overlay. The shared catalogue is
// immutable; character definitions never enter another character's choices.
func WithCustomCatalog(log Log, base *catalog.Catalog) *catalog.Catalog {
	options := CustomOptions(log)
	if len(options) == 0 || base == nil {
		return base
	}
	cat := *base
	for _, c := range options {
		if ref, ok := rules.ParseRef(c.Reference); ok && ref.Slug.String() != "custom-"+c.ID {
			continue
		}
		entry := catalog.Entry{Slug: c.Slug(), Name: c.Name, Desc: []string{c.Description}, Manual: true}
		switch c.Kind {
		case "class":
			v := catalog.Class{Entry: entry}
			if c.HitDie != nil {
				v.HitDie = *c.HitDie
			}
			cat.Classes = catalog.NewCollection(append(cat.Classes.All(), v))
		case "race":
			v := catalog.Race{Entry: entry}
			if c.Speed != nil {
				v.Speed = rules.Feet(*c.Speed)
			}
			cat.Races = catalog.NewCollection(append(cat.Races.All(), v))
		case "subrace":
			cat.Subraces = catalog.NewCollection(append(cat.Subraces.All(), catalog.Subrace{Entry: entry, Race: rules.Slug(c.Parent)}))
			races := cat.Races.All()
			for i := range races {
				if races[i].Slug.String() == c.Parent {
					races[i].Subraces = append(slices.Clone(races[i].Subraces), c.Slug())
				}
			}
			cat.Races = catalog.NewCollection(races)
		case "background":
			cat.Backgrounds = catalog.NewCollection(append(cat.Backgrounds.All(), catalog.Background{Entry: entry}))
		case "subclass":
			cat.Subclasses = catalog.NewCollection(append(cat.Subclasses.All(), catalog.Subclass{Entry: entry, Class: rules.Slug(c.Parent)}))
			classes := cat.Classes.All()
			for i := range classes {
				if classes[i].Slug.String() == c.Parent {
					classes[i].Subclasses = append(slices.Clone(classes[i].Subclasses), c.Slug())
				}
			}
			cat.Classes = catalog.NewCollection(classes)
		case "feat":
			cat.Feats = catalog.NewCollection(append(cat.Feats.All(), catalog.Feat{Entry: entry}))
		case "feature":
			cat.Features = catalog.NewCollection(append(cat.Features.All(), catalog.Feature{Entry: entry, Class: rules.Slug(c.Parent)}))
		case "trait":
			cat.Traits = catalog.NewCollection(append(cat.Traits.All(), catalog.Trait{Entry: entry}))
		case "spell", "cantrip":
			level := -1
			if c.Kind == "cantrip" {
				level = 0
			} else if c.Level != nil {
				level = *c.Level
			}
			cat.Spells = catalog.NewCollection(append(cat.Spells.All(), catalog.Spell{Entry: entry, Level: level}))
		case "item":
			cat.Items = catalog.NewCollection(append(cat.Items.All(), catalog.Item{Entry: entry}))
		}
	}
	return &cat
}
func (p *projector) customStructure(c CustomOption) {
	if !c.Selected {
		return
	}
	switch c.Kind {
	case "race":
		p.state.Identity.Race = c.Slug()
		p.state.Identity.Subrace = ""
	case "subrace":
		p.state.Identity.Subrace = c.Slug()
	case "background":
		p.state.Identity.Background = c.Slug()
	case "class":
		level := 1
		if c.Level != nil {
			level = *c.Level
		}
		p.state.Identity.Classes = []ClassLevel{{Class: c.Slug(), Level: level}}
	case "subclass":
		p.setSubclass(c.Slug())
	}
}
func (p *projector) customDetails(log Log) {
	p.state.CustomOptions = CustomOptions(log)
	if p.state.CatalogNames == nil {
		p.state.CatalogNames = map[string]string{}
	}
	for _, c := range p.state.CustomOptions {
		collection := map[string]string{"class": "classes", "race": "races", "subrace": "subraces", "subclass": "subclasses", "background": "backgrounds", "spell": "spells", "cantrip": "spells", "item": "equipment", "feat": "feats", "feature": "features", "trait": "traits"}[c.Kind]
		p.state.CatalogNames[collection+":"+c.Slug().String()] = c.Name
		if !c.Selected {
			continue
		}
		if c.Kind == "class" || c.Kind == "subclass" {
			if ability, ok := rules.ParseAbility(c.Ability); ok {
				class := c.Slug()
				if c.Kind == "subclass" {
					class = rules.Slug(c.Parent)
				}
				bonus := p.state.Status.ProficiencyBonus + p.state.Abilities.Modifier(ability)
				if !slices.ContainsFunc(p.state.Status.Spellcasting, func(existing SpellcastingSummary) bool { return existing.Class == class && existing.Ability == ability }) {
					p.state.Status.Spellcasting = append(p.state.Status.Spellcasting, SpellcastingSummary{Class: class, Ability: ability, SaveDC: 8 + bonus, AttackBonus: bonus})
				}
			}
		}
		if c.Kind == "spell" || c.Kind == "cantrip" {
			source := SpellSource{Source: rules.NewRef(rules.RefRule, rules.Slug("custom-spells-"+c.Parent+"-"+c.Ability)), Class: rules.Slug(c.Parent), Ability: rules.Ability(c.Ability)}
			if p.cat.Races.Has(rules.Slug(c.Parent)) {
				source.Class = ""
				source.Source = rules.NewRef(rules.RefRace, rules.Slug(c.Parent))
			}
			if c.Kind == "cantrip" {
				source.Cantrips = []rules.Slug{c.Slug()}
			} else if c.Mode == "spellbook" {
				source.Spellbook = []rules.Slug{c.Slug()}
			} else if c.Mode == "prepared" || c.Mode == "granted" {
				source.Prepared = []rules.Slug{c.Slug()}
			} else {
				source.Known = []rules.Slug{c.Slug()}
				source.Prepared = []rules.Slug{c.Slug()}
			}
			index := slices.IndexFunc(p.state.Spells.Sources, func(existing SpellSource) bool {
				return existing.Source == source.Source && existing.Class == source.Class && existing.Ability == source.Ability
			})
			if index < 0 {
				p.state.Spells.Sources = append(p.state.Spells.Sources, source)
			} else {
				existing := &p.state.Spells.Sources[index]
				existing.Cantrips = uniqueSlugs(append(existing.Cantrips, source.Cantrips...))
				existing.Known = uniqueSlugs(append(existing.Known, source.Known...))
				existing.Prepared = uniqueSlugs(append(existing.Prepared, source.Prepared...))
				existing.Spellbook = uniqueSlugs(append(existing.Spellbook, source.Spellbook...))
			}
		}

		switch c.Kind {
		case "cantrip":
			p.state.Spells.Cantrips = uniqueSlugs(append(p.state.Spells.Cantrips, c.Slug()))
		case "spell":
			if c.Mode == "prepared" || c.Mode == "granted" {
				p.state.Spells.Prepared = uniqueSlugs(append(p.state.Spells.Prepared, c.Slug()))
			} else {
				p.state.Spells.Known = uniqueSlugs(append(p.state.Spells.Known, c.Slug()))
			}
		case "feat":
			p.state.Feats = uniqueSlugs(append(p.state.Feats, c.Slug()))
		case "feature":
			p.state.Features = uniqueSlugs(append(p.state.Features, c.Slug()))
		case "trait":
			p.state.Traits = uniqueSlugs(append(p.state.Traits, c.Slug()))
		case "item":
			stack := ItemStack{Item: c.Slug(), Count: max(c.Count, 1)}
			list := &p.state.Equipment.Backpack
			if c.Placement == "equipped" {
				list = &p.state.Equipment.Equipped
			}
			if c.Placement == "loot" {
				list = &p.state.Equipment.Loot
			}
			*list = slices.DeleteFunc(*list, func(old ItemStack) bool { return old.Item == stack.Item })
			*list = append(*list, stack)
		}
	}
}
func uniqueSlugs(values []rules.Slug) []rules.Slug {
	out := []rules.Slug{}
	for _, v := range values {
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}
