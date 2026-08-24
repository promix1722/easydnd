package catalog

import "github.com/promix1722/easydnd/internal/domain/rules"

// AbilityBonus is a fixed increase to one ability score, as a race or subrace
// grants it.
type AbilityBonus struct {
	Ability rules.Ability
	Bonus   int
}

// Race is a player character race: dwarf, elf, human and the rest.
//
// The 2014 rules call this a race; the 2024 rules rename it to species. This
// project targets 2014, so the older term is correct here.
type Race struct {
	Entry

	// Speed is the base walking speed.
	Speed rules.Feet

	Size rules.Size

	// AbilityBonuses are granted unconditionally. Races that let the player
	// choose instead put the prompt in AbilityBonusOptions.
	AbilityBonuses []AbilityBonus

	// AbilityBonusOptions is the half-elf's "+1 to two other abilities". Nil
	// when the race grants no choice.
	AbilityBonusOptions *rules.Choice

	// Languages are known automatically; LanguageOptions is the extra pick
	// some races offer. Nil when there is none.
	Languages       []rules.Slug
	LanguageOptions *rules.Choice

	// StartingProficiencies are granted unconditionally;
	// ProficiencyOptions holds any prompts, e.g. the half-elf's two skills.
	StartingProficiencies []rules.Slug
	ProficiencyOptions    []rules.Choice

	// Traits are the racial traits this race grants, such as Darkvision.
	// Note these are traits, not class features: the SRD keeps the two in
	// separate collections and so does this catalogue.
	Traits []rules.Slug

	// Subraces lists the available subraces; empty for races without any.
	Subraces []rules.Slug

	// AgeDesc, AlignmentDesc, SizeDesc and LanguageDesc are prose in the
	// catalogue's locale, describing what the numeric fields above summarise.
	AgeDesc       []string
	AlignmentDesc []string
	SizeDesc      []string
	LanguageDesc  []string
}

// Subrace is a variant of a Race, adding bonuses and traits on top of it.
type Subrace struct {
	Entry

	// Race is the parent race this subrace refines.
	Race rules.Slug

	AbilityBonuses []AbilityBonus

	// Traits are added to the parent race's traits, never replacing them.
	Traits []rules.Slug

	StartingProficiencies []rules.Slug
	LanguageOptions       *rules.Choice
}

// Trait is a racial trait: Darkvision, Fey Ancestry, Breath Weapon.
//
// Traits come from a race or subrace. A Feature, by contrast, comes from a
// class or subclass. DND.md originally filed Darkvision under "features"; the
// SRD keeps the two apart, and merging them would make it impossible to say
// which source granted a given entry.
type Trait struct {
	Entry

	// Races and Subraces list which entries grant this trait.
	Races    []rules.Slug
	Subraces []rules.Slug

	// Proficiencies are granted by the trait itself, e.g. Dwarven Combat
	// Training. ProficiencyOptions holds any prompt it poses.
	Proficiencies      []rules.Slug
	ProficiencyOptions *rules.Choice

	// Specific carries the structured payload a handful of traits attach:
	// the dragonborn's breath weapon, the high elf's cantrip pick. Nil for
	// the majority, which are prose only.
	Specific *TraitSpecific
}

// TraitSpecific is the structured payload of the few traits that have one.
// Every field is optional; a nil or zero field means the trait does not use
// that mechanism.
type TraitSpecific struct {
	// BreathWeapon is the dragonborn's, keyed by draconic ancestry.
	BreathWeapon *rules.Choice

	// SpellOptions is a prompt to learn a spell, e.g. the high elf's cantrip.
	SpellOptions *rules.Choice

	// SubtraitOptions is a prompt to pick a further trait.
	SubtraitOptions *rules.Choice

	// DamageResistance lists damage types the trait resists.
	DamageResistance []rules.Slug
}
