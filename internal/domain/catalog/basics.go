package catalog

import "github.com/promix1722/easydnd/internal/domain/rules"

// AbilityScore is one of the six abilities as a catalogue entry, carrying its
// full name and the skills it governs.
type AbilityScore struct {
	Entry

	// Ability is the enum this entry describes.
	Ability rules.Ability

	// FullName is the unabbreviated name ("Dexterity") in the catalogue's
	// locale; Entry.Name holds the abbreviation the SRD data uses ("DEX").
	FullName string

	// Skills lists the skills this ability governs.
	Skills []rules.Slug
}

// Skill is a proficiency applied to ability checks, such as Perception or
// Stealth.
type Skill struct {
	Entry

	// Ability is the ability a check with this skill rolls against. It is
	// fixed by the rules, not by the character: Perception is always Wisdom.
	Ability rules.Ability
}

// Alignment is one of the nine moral and ethical positions.
type Alignment struct {
	Entry

	// Abbreviation is the two-letter short form ("CG", "LN").
	Abbreviation string
}

// LanguageType distinguishes the SRD's two language groups.
type LanguageType uint8

// The language groups.
const (
	LanguageTypeNone LanguageType = iota
	// LanguageStandard is Common, Dwarvish, Elvish and the rest that any
	// character may know.
	LanguageStandard
	// LanguageExotic is Draconic, Deep Speech and the others that normally
	// require permission or a background reason.
	LanguageExotic
)

// Language is a tongue a character can speak, read and write.
type Language struct {
	Entry

	Type LanguageType

	// Script names the writing system, e.g. "Dwarvish". Empty for languages
	// with no written form.
	Script string

	// TypicalSpeakers is prose in the catalogue's locale.
	TypicalSpeakers []string
}

// Condition is a status such as Prone, Poisoned or Unconscious. Its Desc holds
// the mechanical effects, which are applied by the DM rather than computed.
type Condition struct {
	Entry
}

// DamageType is a kind of damage: acid, bludgeoning, fire and so on.
type DamageType struct {
	Entry
}

// MagicSchool is one of the eight schools of magic.
type MagicSchool struct {
	Entry
}

// Term is a free-standing piece of prose with no mechanics of its own.
//
// The choice grammar refers to prose by key rather than carrying text, because
// internal/domain/rules is language-neutral: a background's suggested ideals
// are rules.TextOption values holding only a Key, a draconic ancestry's
// DamageOption points its Notes at one, and a granted ActionOption names one.
// Every such key resolves here, so a client can render the option a player is
// being asked to pick without a second vocabulary for "prose that is not an
// entry".
//
// Terms have no mechanics file. They exist only in the locale bundles, which
// is why the collection is built from the bundle's own keys rather than from
// a list of slugs the way every other collection is.
type Term struct {
	Entry
}

// WeaponProperty is a tag such as Finesse, Light or Two-Handed. The property's
// rules text lives in Desc; the mechanical consequences are applied where the
// weapon is used.
type WeaponProperty struct {
	Entry
}

// ProficiencyType groups what a proficiency applies to.
type ProficiencyType uint8

// The kinds of thing a character can be proficient with.
const (
	ProficiencyTypeNone ProficiencyType = iota
	ProficiencyArmor
	ProficiencyWeapons
	ProficiencyArtisansTools
	ProficiencyGamingSets
	ProficiencyMusicalInstruments
	ProficiencyOtherTools
	ProficiencyVehicles
	ProficiencySkills
	ProficiencySavingThrows
)

// ProficiencyDef is a single named proficiency a class, race or background can
// grant, e.g. "Skill: Stealth" or "Light Armor".
type ProficiencyDef struct {
	Entry

	Type ProficiencyType

	// Reference points at what the proficiency is in -- the skill, the item,
	// the equipment category. Zero when the proficiency stands alone.
	Reference rules.Ref

	// Classes and Races list the entries that grant this proficiency. They
	// are denormalised from the grantors so a UI can answer "who gets this?"
	// without scanning every class.
	Classes []rules.Slug
	Races   []rules.Slug
}

// EquipmentCategory groups items, e.g. "Martial Melee Weapons". A Choice may
// draw its options from a category rather than listing them.
type EquipmentCategory struct {
	Entry

	// Items lists every member of the category.
	Items []rules.Slug
}
