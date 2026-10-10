package character

import (
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// identity asks for a name on a log that has nothing in it at all. It exists
// so that a character created without an init event -- a client crash between
// the create call and the first append -- has a way back rather than being
// permanently unreadable.
func (b *promptBuilder) identity() {
	if !b.empty {
		return
	}
	b.out = append(b.out, Prompt{
		Choice: rules.Choice{Prompt: "character/init", Choose: 1, Kind: rules.ChooseText},
		Group:  GroupIdentity,
		Event:  PromptEvent{Type: EventInit},
	})
}

// ruleset asks which rules the character is built under, once.
//
// There is exactly one answer today -- the compendium's own ruleset -- and the
// validator holds every answer to it, so the question is a recorded fact
// rather than a decision. It exists so that a 2024 compendium, when one is
// wired in, meets characters that already say which rules they meant.
//
// Required, with the desired level below it, and the two of them are why the
// identity stage is three questions rather than a name: a character is built
// to a level under a set of rules, and both are decisions the sheet cannot
// derive. A log that predates them -- an import -- reads as unfinished until
// they are answered, which is the honest reading: nothing in it says which
// rules it was written under.
func (b *promptBuilder) ruleset() {
	if b.empty || !b.state.Identity.Ruleset.IsZero() {
		return
	}
	b.out = append(b.out, Prompt{
		Choice: rules.Choice{Prompt: "character/ruleset", Choose: 1, Kind: rules.ChooseText},
		Group:  GroupIdentity,
		Event:  PromptEvent{Type: EventChange},
	})
}

// desiredLevel asks what level the character is being built towards.
//
// It is answered by a change event setting identity.desiredLevel, so like the
// ability scores it closes by state rather than by a recorded answer.
func (b *promptBuilder) desiredLevel() {
	if b.empty || b.state.Identity.DesiredLevel > 0 {
		return
	}
	b.out = append(b.out, Prompt{
		Choice: rules.Choice{Prompt: "character/desired-level", Choose: 1, Kind: rules.ChooseLevel},
		Group:  GroupIdentity,
		Event:  PromptEvent{Type: EventChange},
	})
}

func (b *promptBuilder) abilities() {
	if b.scored {
		return
	}
	b.out = append(b.out, Prompt{
		Choice: rules.Choice{
			Prompt: "character/abilities",
			Choose: len(b.cat.AbilityIDs()),
			Kind:   rules.ChooseAbilityScores,
		},
		Group:       GroupAbilities,
		Event:       PromptEvent{Type: EventChange},
		Recommended: b.abilityPriority(),
	})
}

// abilityPriority is the first class's advice, when it names every ability
// exactly once. Anything else -- a pack that lists four, or one twice -- is
// not an order the six scores can be dealt out by, so it is no advice at all.
func (b *promptBuilder) abilityPriority() []rules.Ability {
	if len(b.state.Identity.Classes) == 0 {
		return nil
	}
	class, ok := b.cat.Classes.Get(b.state.Identity.Classes[0].Class)
	if !ok || len(class.AbilityPriority) != len(b.cat.AbilityIDs()) {
		return nil
	}
	seen := map[rules.Ability]bool{}
	for _, ability := range class.AbilityPriority {
		seen[ability] = true
	}
	if len(seen) != len(class.AbilityPriority) {
		return nil
	}
	return class.AbilityPriority
}

func (b *promptBuilder) race() {
	if b.state.Identity.Race.IsZero() {
		b.out = append(b.out, Prompt{
			Choice: rules.Choice{
				Prompt: "character/race",
				Choose: 1,
				Kind:   rules.ChooseRace,
				From:   rules.OptionSet{Kind: rules.OptionsFromCollection, Collection: rules.RefRace},
			},
			Group: GroupRace,
			Event: PromptEvent{Type: EventRace},
		})
		return
	}

	race, ok := b.cat.Races.Get(b.state.Identity.Race)
	if !ok {
		return
	}
	source := rules.NewRef(rules.RefRace, race.Slug)
	base := Prompt{Group: GroupRace, Source: source, Event: PromptEvent{Type: EventRace, Ref: source}}

	b.addChoice(race.AbilityBonusOptions, base)
	b.addChoice(race.LanguageOptions, optionalPrompt(base))
	b.addChoices(race.ProficiencyOptions, base)

	if len(race.Subraces) > 0 && b.state.Identity.Subrace.IsZero() {
		b.out = append(b.out, Prompt{
			Choice: rules.Choice{
				Prompt: "character/subrace",
				Choose: 1,
				Kind:   rules.ChooseSubrace,
				From:   refOptions(rules.RefSubrace, race.Subraces),
			},
			Group:  GroupRace,
			Source: source,
			// A subrace is optional only in the sense that the SRD's four
			// cover four of nine races; where one exists the rules require
			// picking it.
			Event: PromptEvent{Type: EventSubrace},
		})
	}
	if subrace, ok := b.cat.Subraces.Get(b.state.Identity.Subrace); ok {
		subSource := rules.NewRef(rules.RefSubrace, subrace.Slug)
		b.addChoice(subrace.LanguageOptions, optionalPrompt(Prompt{
			Group:  GroupRace,
			Source: subSource,
			Event:  PromptEvent{Type: EventSubrace, Ref: subSource},
		}))
	}

	// Traits pose prompts of their own, and only once the race that grants
	// them has been chosen -- which is why an answer to one necessarily
	// arrives in a later event than the race event that opened it.
	for _, slug := range b.state.Traits {
		trait, ok := b.cat.Traits.Get(slug)
		if !ok {
			continue
		}
		traitSource := rules.NewRef(rules.RefTrait, trait.Slug)
		traitPrompt := Prompt{
			Group:  GroupRace,
			Source: traitSource,
			Event:  PromptEvent{Type: EventRace, Ref: source},
		}
		b.addChoice(trait.ProficiencyOptions, traitPrompt)
		if trait.Specific != nil {
			b.addChoice(trait.Specific.SpellOptions, traitPrompt)
			b.addChoice(trait.Specific.SubtraitOptions, traitPrompt)
			// BreathWeapon describes the attack granted by ancestry. Its
			// legacy choice-shaped payload is not a player decision.
		}
	}
}

func (b *promptBuilder) background() {
	if b.state.Identity.Background.IsZero() {
		b.out = append(b.out, Prompt{
			Choice: rules.Choice{
				Prompt: "character/background",
				Choose: 1,
				Kind:   rules.ChooseBackground,
				From:   rules.OptionSet{Kind: rules.OptionsFromCollection, Collection: rules.RefBackground},
			},
			Group: GroupBackground,
			Event: PromptEvent{Type: EventBackground},
		})
		return
	}

	background, ok := b.cat.Backgrounds.Get(b.state.Identity.Background)
	if !ok {
		return
	}
	source := rules.NewRef(rules.RefBackground, background.Slug)
	base := Prompt{Group: GroupBackground, Source: source, Event: PromptEvent{Type: EventBackground, Ref: source}}

	b.addChoice(background.LanguageOptions, optionalPrompt(base))

	optional := optionalPrompt(base)
	b.addChoices(background.StartingEquipmentOptions, optional)
}

// personality poses who the character is: the four roleplaying questions and
// an alignment.
//
// These are available from the start, independently of the background.
// A background may suggest answers, but the player can write their own first.
//
// The four are asked as **text**, and the SRD's own d8 tables are not offered.
// A trait is the one thing on a character sheet that is nobody's but the
// player's, and a menu of eight makes it the compendium's. The state behind
// them was always free text -- see State.Identity -- so what changes here is
// only that the prompt stops pretending otherwise: each is answered by the
// change that settles it, exactly as the alignment beside them is, and the
// suggestions remain in the compendium for anybody who wants to read them.
//
// Each is posed only while its answer is unset. There is nothing here to
// compare against an option set, so "answered" is a question about the sheet
// rather than about the log -- which is the same rule the alignment follows,
// and the reason neither of them goes through addChoice.
func (b *promptBuilder) personality() {
	written := func(prompt rules.Slug, kind rules.ChoiceKind, set []string) {
		if len(set) > 0 {
			return
		}
		b.out = append(b.out, Prompt{
			Choice: rules.Choice{
				Prompt: prompt,
				// One, whatever the background's table suggests. The count
				// belonged to a menu -- "pick two of these eight" -- and what
				// is asked now is one answer in the player's own words, which
				// can be as long as it wants to be.
				Choose: 1,
				Kind:   kind,
				// Explicit and empty: there is nothing to pick between, and a
				// client tells this apart from a menu by the absence of one.
				From: rules.OptionSet{Kind: rules.OptionsExplicit},
			},
			Group:    GroupPersonality,
			Optional: true,
			Event:    PromptEvent{Type: EventChange},
		})
	}

	id := b.state.Identity
	written("character/personality-trait", rules.ChoosePersonality, id.PersonalityTraits)
	written("character/ideal", rules.ChooseIdeal, id.Ideals)
	written("character/bond", rules.ChooseBond, id.Bonds)
	written("character/flaw", rules.ChooseFlaw, id.Flaws)

	if b.state.Identity.Alignment.IsZero() {
		b.out = append(b.out, Prompt{
			Choice: rules.Choice{
				Prompt: "character/alignment",
				Choose: 1,
				Kind:   rules.ChooseAlignment,
				From:   rules.OptionSet{Kind: rules.OptionsFromCollection, Collection: rules.RefAlignment},
			},
			Group:    GroupPersonality,
			Optional: true,
			Event:    PromptEvent{Type: EventChange},
		})
	}
}
