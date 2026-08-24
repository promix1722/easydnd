package catalog

import "github.com/promix1722/easydnd/internal/domain/rules"

// Background is where a character came from before adventuring: Acolyte,
// Urchin, Soldier.
//
// SRD 5.1 publishes exactly one background (Acolyte), so anything beyond a
// starter experience will need homebrew entries in the same shape.
type Background struct {
	Entry

	// StartingProficiencies are granted unconditionally.
	StartingProficiencies []rules.Slug

	// LanguageOptions is the extra language pick most backgrounds offer. Nil
	// when there is none.
	LanguageOptions *rules.Choice

	// StartingEquipment is granted unconditionally;
	// StartingEquipmentOptions holds any prompts. StartingGold is the coin
	// granted alongside.
	StartingEquipment        []ItemStack
	StartingEquipmentOptions []rules.Choice
	StartingGold             rules.Coins

	// Feature is the background's special feature, e.g. Shelter of the
	// Faithful.
	Feature rules.Slug

	// PersonalityTraits, Ideals, Bonds and Flaws are the roleplaying prompts
	// a player rolls or picks from. They are Choices rather than plain lists
	// because the SRD states how many to take.
	PersonalityTraits rules.Choice
	Ideals            rules.Choice
	Bonds             rules.Choice
	Flaws             rules.Choice
}

// Feat is an optional talent taken in place of an Ability Score Improvement.
//
// SRD 5.1 publishes exactly one feat (Grappler). As with backgrounds, real
// use will mean homebrew.
type Feat struct {
	Entry

	Prerequisites []Prerequisite
}
