package character

import (
	"fmt"
	"slices"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

func (b *promptBuilder) classes() {
	if len(b.state.Identity.Classes) == 0 {
		b.out = append(b.out, Prompt{
			Choice: rules.Choice{
				Prompt: "character/class",
				Choose: 1,
				Kind:   rules.ChooseClass,
				From:   rules.OptionSet{Kind: rules.OptionsFromCollection, Collection: rules.RefClass},
			},
			Group: GroupClass,
			Event: PromptEvent{Type: EventClass, Level: 1},
		})
		return
	}

	for i, taken := range b.state.Identity.Classes {
		class, ok := b.cat.Classes.Get(taken.Class)
		if !ok {
			continue
		}
		source := rules.NewRef(rules.RefClass, taken.Class)
		// Level 1, not the level the character has reached. What follows is
		// the grant a class makes when it is *entered* -- its skills, its
		// starting kit -- so a fifth-level barbarian's unanswered skill choice
		// belongs to the level that offered it and files under it. Stamping
		// the current level instead filed every one of them at the end of the
		// class story, under a level that granted nothing.
		base := Prompt{
			Group:  GroupClass,
			Source: source,
			Level:  1,
			Event:  PromptEvent{Type: EventClass, Ref: source, Level: 1},
		}

		g := classGrant(class, 1, i == 0)
		b.addChoices(g.Choices, base)

		if i == 0 {
			optional := base
			optional.Optional = true
			b.addChoices(class.StartingEquipmentOptions, optional)
		}

		b.subclass(class, taken)
		b.levelPrompts(class, taken)
	}
}

// subclass offers the subclass prompt at the level the class's own
// advancement rows say it is due.
func (b *promptBuilder) subclass(class catalog.Class, taken ClassLevel) {
	if !taken.Subclass.IsZero() || len(class.Subclasses) == 0 {
		return
	}
	due := subclassLevel(b.cat, class)
	if due == 0 || taken.Level < due {
		return
	}
	b.out = append(b.out, Prompt{
		Choice: rules.Choice{
			Prompt: rules.Slug(fmt.Sprintf("%s/subclass", class.Slug)),
			Choose: 1,
			Kind:   rules.ChooseSubclass,
			From:   refOptions(rules.RefSubclass, class.Subclasses),
		},
		Group:  GroupClass,
		Source: rules.NewRef(rules.RefClass, class.Slug),
		Level:  due,
		Event:  PromptEvent{Type: EventSubclass, Level: due},
	})
}

// levelPrompts walks the levels already taken in a class and adds whatever
// each one still has open: the prompts its features pose, and the Ability
// Score Improvement.
func (b *promptBuilder) levelPrompts(class catalog.Class, taken ClassLevel) {
	source := rules.NewRef(rules.RefClass, class.Slug)
	for level := 1; level <= taken.Level; level++ {
		row, ok := b.cat.ClassLevel(class.Slug, level)
		if !ok {
			continue
		}
		features := row.Features
		if !taken.Subclass.IsZero() {
			if subRow, ok := b.cat.ClassLevel(taken.Subclass, level); ok {
				features = append(slices.Clone(features), subRow.Features...)
			}
		}
		for _, slug := range features {
			b.featurePrompts(slug, level, source)
		}
		if grantsAbilityScoreImprovement(b.cat, class.Slug, level) {
			b.abilityScoreImprovement(class.Slug, level)
		}
	}
}

func (b *promptBuilder) featurePrompts(slug rules.Slug, level int, class rules.Ref) {
	feature, ok := b.cat.Features.Get(slug)
	if !ok || feature.Specific == nil {
		return
	}
	p := Prompt{
		Group:  GroupClass,
		Source: rules.NewRef(rules.RefFeature, slug),
		Level:  level,
		Event:  PromptEvent{Type: EventLevel, Ref: class, Level: level},
	}
	expertise := p
	expertise.HeldOnly = true
	// oneList, because the rogue's first Expertise is transcribed as a choice
	// between branches and is one list of two. Project reads it through the
	// same function, or an answer to the flat question would not resolve
	// against the nested shape.
	b.addChoice(oneList(feature.Specific.ExpertiseOptions), expertise)
	b.addChoice(feature.Specific.SubfeatureOptions, p)
	b.addChoice(feature.Specific.EnemyTypeOptions, p)
	b.addChoice(feature.Specific.TerrainTypeOptions, p)
}

// abilityScoreImprovement builds the "+2 to one ability, +1 to two, or a
// feat" prompt.
//
// It is synthesised because the SRD data does not carry it: the feature row
// is bare, and only the cumulative AbilityScoreBonuses count marks the level.
// The shape is the compendium's own nested-choice idiom -- choose one branch,
// the branch carries the picks -- so a client that can already render the
// rogue's Expertise can render this with no new code.
func (b *promptBuilder) abilityScoreImprovement(class rules.Slug, level int) {
	prompt := asiPrompt(class, level)
	scores := rules.Choice{
		Prompt: prompt + "/0",
		Choose: abilityScoreIncrease(b.cat),
		Kind:   rules.ChooseAbilityBonus,
		From:   rules.OptionSet{Kind: rules.OptionsExplicit, Options: abilityBonusOptions(b.cat.AbilityIDs())},
		// Two points rather than two scores: both may go into one ability,
		// which is the "+2 to one" half of the rule. This is the only choice
		// in the game that says so -- a half-elf's two look identical and are
		// "two *different* scores" -- which is why it is stated here rather
		// than inferred from the kind.
		Repeatable: true,
	}
	feat := rules.Choice{
		Prompt: prompt + "/1",
		Choose: 1,
		Kind:   rules.ChooseFeature,
		From:   rules.OptionSet{Kind: rules.OptionsFromCollection, Collection: rules.RefFeat},
	}
	b.addChoice(&rules.Choice{
		Prompt: prompt,
		Choose: 1,
		Kind:   rules.ChooseAbilityScores,
		From: rules.OptionSet{Kind: rules.OptionsExplicit, Options: []rules.Option{
			rules.NestedOption{Choice: scores},
			rules.NestedOption{Choice: feat},
		}},
	}, Prompt{
		Group:  GroupClass,
		Source: rules.NewRef(rules.RefClass, class),
		Level:  level,
		Event:  PromptEvent{Type: EventLevel, Ref: rules.NewRef(rules.RefClass, class), Level: level},
	})
}

// asiPrompt is the id of the Ability Score Improvement prompt for one class
// at one level. Prompts emits it and Project reads it, so it lives in one
// place: two spellings of the same synthesised id is an answer that resolves
// on the way in and vanishes on the way out.
func asiPrompt(class rules.Slug, level int) rules.Slug {
	return rules.Slug(fmt.Sprintf("%s/ability-score-improvement/%d", class, level))
}

// abilityBonusOptions is "+1 to any ability", once per ability. Picking the
// same ability twice is the "+2 to one" half of the rule, which the choice
// above allows by being Repeatable.
func abilityBonusOptions(abilities []rules.Ability) []rules.Option {
	out := make([]rules.Option, 0, len(abilities))
	for _, ability := range abilities {
		out = append(out, rules.AbilityBonusOption{Ability: ability, Bonus: 1})
	}
	return out
}

// featLevel is the class level a feat was taken at, or zero when it was not
// taken at one -- a rule owned by something else, or a feat a race gave.
//
// A feat's own question belongs to the level that brought the feat: Slasher's
// "+1 to Strength or Dexterity" is part of what fourth level asked. Without a
// level it reads as belonging to no level at all, and a build screen draws it
// above first level, ahead of the improvement that opened it.
//
// A feature's question belongs to the level the feature is gained at, which
// its own row says: Student of War's tool is asked with third level.
func (b *promptBuilder) featLevel(owner rules.Ref) int {
	if owner.Kind == rules.RefFeature {
		feature, _ := b.cat.Features.Get(owner.Slug)
		return feature.Level
	}
	if owner.Kind != rules.RefFeat {
		return 0
	}
	for _, taken := range b.state.Identity.Classes {
		for level := 1; level <= taken.Level; level++ {
			if slices.Contains(b.answers.picks(asiPrompt(taken.Class, level)+"/1"), owner.Slug) {
				return level
			}
		}
	}
	return 0
}

func abilityScoreIncrease(cat *catalog.Catalog) int {
	return cat.Mechanics.Core.AbilityScoreIncrease
}
