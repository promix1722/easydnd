package character

import (
	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// applyRace resolves the race and subrace: bonuses, speed, size, languages,
// traits and the proficiencies they grant.
func (p *projector) applyRace() {
	race, ok := p.cat.Races.Get(p.state.Identity.Race)
	if !ok {
		return
	}
	p.state.Base.Size = race.Size
	if race.Speed > 0 {
		p.state.Base.Speeds = append(p.state.Base.Speeds, Speed{Kind: Walking, Distance: race.Speed})
	}
	p.addLanguages(race.Languages...)
	p.addLanguages(p.answers.slugs(race.LanguageOptions)...)
	p.proficiencies = append(p.proficiencies, race.StartingProficiencies...)
	for _, choice := range race.ProficiencyOptions {
		p.proficiencies = append(p.proficiencies, p.answers.slugs(&choice)...)
	}
	p.state.Traits = append(p.state.Traits, race.Traits...)

	if subrace, ok := p.cat.Subraces.Get(p.state.Identity.Subrace); ok {
		p.state.Traits = append(p.state.Traits, subrace.Traits...)
		p.proficiencies = append(p.proficiencies, subrace.StartingProficiencies...)
		p.addLanguages(p.answers.slugs(subrace.LanguageOptions)...)
	}

	// Traits carry prompts of their own -- the half-elf's two skills come
	// from Skill Versatility, not from the race entry.
	for _, slug := range p.state.Traits {
		trait, ok := p.cat.Traits.Get(slug)
		if !ok {
			continue
		}
		p.proficiencies = append(p.proficiencies, trait.Proficiencies...)
		p.proficiencies = append(p.proficiencies, p.answers.slugs(trait.ProficiencyOptions)...)
		if trait.Specific != nil {
			p.state.Traits = append(p.state.Traits, p.answers.slugs(trait.Specific.SubtraitOptions)...)
		}
	}
	p.state.Base.Senses = sensesFor(p.state.Traits)
	if len(p.cat.Mechanics.Core.Senses) > 0 {
		p.state.Base.Senses = packSenses(p.state.Traits, p.cat)
	}
}

// applyBackground resolves the background's proficiencies, languages,
// equipment and roleplaying picks.
func (p *projector) applyBackground() {
	background, ok := p.cat.Backgrounds.Get(p.state.Identity.Background)
	if !ok {
		return
	}
	p.proficiencies = append(p.proficiencies, background.StartingProficiencies...)
	p.addLanguages(p.answers.slugs(background.LanguageOptions)...)
	p.addStacks(background.StartingEquipment)
	if background.StartingGold.Amount > 0 {
		p.addCoins(background.StartingGold)
	}
	if !background.Feature.IsZero() {
		p.state.Features = append(p.state.Features, background.Feature)
	}

	// Nothing here writes the roleplaying lines. They are the player's own
	// words now, arriving as changes to identity.personalityTraits and its
	// three siblings, and this used to *assign* them from the picked
	// suggestion -- so choosing a background after writing them would have
	// wiped what was written.
}

// applyClasses resolves every class the character has levels in: hit points,
// Hit Dice, proficiencies, features and starting equipment.
func (p *projector) applyClasses() {
	for i, taken := range p.state.Identity.Classes {
		class, ok := p.cat.Classes.Get(taken.Class)
		if !ok {
			continue
		}
		first := i == 0
		g := classGrant(class, 1, first)
		p.proficiencies = append(p.proficiencies, g.Proficiencies...)
		for _, choice := range g.Choices {
			p.proficiencies = append(p.proficiencies, p.answers.slugs(&choice)...)
		}
		p.addStacks(g.Equipment)

		// The first class grants its saving throws; a class taken later does
		// not, which is the 2014 multiclassing rule.
		if first {
			for _, ability := range class.SavingThrows {
				p.setSavingThrow(ability)
			}
		}

		if class.HitDie > 0 {
			p.addHitPoints(class.HitDie, taken.Level, first)
		}
		p.addHitDice(class.HitDie, taken.Level)

		features := featuresThrough(p.cat, taken.Class, taken.Level)
		if !taken.Subclass.IsZero() {
			features = append(features, featuresThrough(p.cat, taken.Subclass, taken.Level)...)
		}
		p.state.Features = append(p.state.Features, features...)
		p.applyFeaturePrompts(features)

		if row, ok := p.cat.ClassLevel(taken.Class, taken.Level); ok {
			p.addClassResources(row)
		}
	}
}

// applyFeaturePrompts collects what the answered prompts on a feature grant.
// Expertise is the one that changes a number rather than adding to a list.
func (p *projector) applyFeaturePrompts(features []rules.Slug) {
	for _, slug := range features {
		feature, ok := p.cat.Features.Get(slug)
		if !ok || feature.Specific == nil {
			continue
		}
		if feature.Specific.ExpertiseOptions != nil {
			// Through oneList, because that is how Prompts asked it.
			p.expertise = append(p.expertise, p.answers.slugs(oneList(feature.Specific.ExpertiseOptions))...)
		}
		// A favored enemy and a favored terrain are sub-features under their
		// own field names: held like any other, so a second tier cannot
		// choose the first one's again.
		for _, picked := range []*rules.Choice{feature.Specific.SubfeatureOptions, feature.Specific.EnemyTypeOptions, feature.Specific.TerrainTypeOptions} {
			p.state.Features = append(p.state.Features, p.answers.slugs(picked)...)
		}
	}
}

// addHitPoints adds the hit points a class's levels contribute.
//
// The first level of the character's first class takes the full hit die; every
// level after that takes the SRD's fixed average, which for a die of size d is
// d/2 + 1. Both are increased by the Constitution modifier, and the modifier
// is read after the input changes and racial bonuses have landed.
//
// Rolling is deliberately not an option here: Project is documented as having
// no randomness, and a projection that rolled would give a different sheet on
// every read. A rolled hit point total is recorded as a change event, which
// overrides this.
func (p *projector) addHitPoints(hitDie, level int, first bool) {
	if hitDie <= 0 || level < 1 {
		return
	}
	core := p.cat.Mechanics.Core
	vars := variables(p.state, p.cat)
	vars["hitDie"] = hitDie
	firstHP, err := core.HitPointFirst.Eval(vars)
	if err != nil {
		p.err = err
		return
	}
	laterHP, err := core.HitPointLater.Eval(vars)
	if err != nil {
		p.err = err
		return
	}
	gained := level * laterHP
	if first {
		gained += firstHP - laterHP
	}
	p.state.Base.HitPoints.Max += gained
}

func (p *projector) addHitDice(hitDie, level int) {
	if hitDie <= 0 || level < 1 {
		return
	}
	dice := rules.Dice{Terms: []rules.DiceTerm{{Count: level, Faces: hitDie}}}
	p.state.Resources.HitDice = append(p.state.Resources.HitDice, Pool{
		Max:      level,
		Recharge: OnLongRest,
		Dice:     &dice,
	})
}

func (p *projector) addClassResources(row catalog.ClassLevel) {
	for _, resource := range row.Resources {
		pool := Pool{Key: resource.Key, Max: resource.Number}
		if resource.Dice != nil {
			dice := *resource.Dice
			pool.Dice = &dice
		}
		p.state.Resources.Class = append(p.state.Resources.Class, pool)
	}
}

// applyEquipmentChoices resolves the starting-equipment prompts into stacks.
//
// Everything chosen lands in the backpack, whatever slot the question was
// asked for. Putting it on is not this function's business and not a build's:
// a kit choice "for the main hand" used to be worn, which read well for one
// sword and badly for everything else -- see AutoEquip, which does it once,
// at the end, one item to a slot.
func (p *projector) applyEquipmentChoices() {
	class, ok := p.cat.Classes.Get(p.firstClass())
	if ok {
		for _, choice := range class.StartingEquipmentOptions {
			p.addChosenEquipment(choice)
		}
	}
	if background, ok := p.cat.Backgrounds.Get(p.state.Identity.Background); ok {
		for _, choice := range background.StartingEquipmentOptions {
			p.addChosenEquipment(choice)
		}
	}
}

func (p *projector) addChosenEquipment(choice rules.Choice) {
	p.answers.chosen(p.cat.ResolveChoice(choice), func(o rules.Option) {
		switch opt := o.(type) {
		case rules.RefOption:
			if opt.Ref.Kind == rules.RefItem || opt.Ref.Kind == rules.RefMagicItem {
				p.addStacks([]catalog.ItemStack{{Item: opt.Ref.Slug, Count: max(opt.Count, 1)}})
			}
		case rules.MoneyOption:
			p.addCoins(opt.Coins)
		}
	})
}

func (p *projector) firstClass() rules.Slug {
	if len(p.state.Identity.Classes) == 0 {
		return ""
	}
	return p.state.Identity.Classes[0].Class
}

// addStacks carries stacks in the backpack. An equipment pack is carried as
// what is in it -- a dungeoneer's pack is a backpack, a crowbar, ten torches
// -- because that is what a player reaches for; the pack itself is only how
// the book sells them together.
func (p *projector) addStacks(stacks []catalog.ItemStack) {
	for _, stack := range stacks {
		if stack.Item.IsZero() {
			continue
		}
		if it, ok := p.cat.Items.Get(stack.Item); ok && it.Gear != nil && len(it.Gear.Contents) > 0 {
			for _, inside := range it.Gear.Contents {
				p.addStacks([]catalog.ItemStack{{Item: inside.Item, Count: max(inside.Count, 1) * max(stack.Count, 1)}})
			}
			continue
		}
		p.state.Equipment.Backpack = append(p.state.Equipment.Backpack, ItemStack{
			Item:  stack.Item,
			Count: max(stack.Count, 1),
		})
	}
}

func (p *projector) addCoins(coins rules.Coins) {
	if coins.Amount == 0 {
		return
	}
	if p.state.Equipment.Purse == nil {
		p.state.Equipment.Purse = make(rules.Purse)
	}
	p.state.Equipment.Purse[coins.Unit] += coins.Amount
}
