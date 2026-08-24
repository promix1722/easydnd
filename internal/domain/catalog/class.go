package catalog

import "github.com/promix1722/easydnd/internal/domain/rules"

// MaxSpellLevel is the highest spell level in the SRD. Spell slot arrays are
// indexed 1..MaxSpellLevel; index 0 is unused so that slots[3] means "3rd
// level" without an off-by-one at every call site.
const MaxSpellLevel = 9

// Class is a character class: barbarian, wizard, rogue and the rest.
type Class struct {
	Entry

	// HitDie is the die size, so 8 means d8. Rolled on level up and pooled as
	// Hit Dice for short rests.
	HitDie int

	// SavingThrows are the two abilities this class is proficient in.
	SavingThrows []rules.Ability

	// Proficiencies are granted unconditionally; ProficiencyOptions holds the
	// prompts, e.g. "choose two skills from this list".
	Proficiencies      []rules.Slug
	ProficiencyOptions []rules.Choice

	// StartingEquipment is granted unconditionally;
	// StartingEquipmentOptions holds the prompts. Both are used only when a
	// character starts at level 1 rather than multiclassing in.
	StartingEquipment        []ItemStack
	StartingEquipmentOptions []rules.Choice

	// Subclasses lists the archetypes available to this class.
	Subclasses []rules.Slug

	// Spellcasting is nil for the four classes that have none. A nil here and
	// an empty struct mean different things, which is why it is a pointer.
	Spellcasting *Spellcasting

	// MultiClassing states what is required to take a level in this class,
	// and what it grants when taken as a second class.
	MultiClassing MultiClassing
}

// Spellcasting describes how a class casts, as opposed to what a particular
// character has prepared.
type Spellcasting struct {
	// Level is the class level at which spellcasting begins: 1 for wizards,
	// 2 for paladins and rangers.
	Level int

	// Ability is the ability that sets spell save DC and spell attack bonus.
	Ability rules.Ability

	// Info is the class's spellcasting rules text, one section per entry, in
	// the catalogue's locale.
	Info []NamedText
}

// NamedText is a titled block of prose in the catalogue's locale.
type NamedText struct {
	Name string
	Desc []string
}

// MultiClassing states the cost and benefit of adding this class.
type MultiClassing struct {
	// Prerequisites must all hold before a character may take a level.
	Prerequisites []Prerequisite

	// Proficiencies are granted on multiclassing in, and are deliberately a
	// smaller set than the class's level-1 proficiencies.
	Proficiencies      []rules.Slug
	ProficiencyOptions []rules.Choice
}

// ClassLevel is one row of a class's advancement table.
type ClassLevel struct {
	// Class is always set. Subclass is set only for rows that belong to an
	// archetype's own table rather than the base class table.
	Class    rules.Slug
	Subclass rules.Slug

	Level int

	// ProficiencyBonus is the number added to rolls the character is
	// proficient in. The SRD calls this the proficiency bonus; it is distinct
	// from rules.Proficiency, which is whether the character is proficient
	// at all.
	ProficiencyBonus int

	// AbilityScoreBonuses is the cumulative count of Ability Score
	// Improvements available by this level.
	AbilityScoreBonuses int

	// Features are gained at this level.
	Features []rules.Slug

	// SpellSlots is indexed by spell level, 1..MaxSpellLevel. Index 0 is
	// unused.
	SpellSlots [MaxSpellLevel + 1]int

	// CantripsKnown and SpellsKnown are zero for classes that prepare from a
	// full list rather than knowing a fixed number.
	CantripsKnown int
	SpellsKnown   int

	// Resources are the class-specific values for this level: rage count, ki
	// points, sneak attack dice, martial arts die, sorcery points and the
	// rest.
	//
	// One generic keyed list covers all twelve classes. The SRD has no
	// umbrella term for these -- "slot" is spell-only -- and the upstream
	// data files all thirty-two of them under a single class_specific bag,
	// so twelve bespoke structs would buy nothing but churn.
	Resources []LevelResource
}

// LevelResource is one class-specific value at one level.
//
// Some resources are counts (ki points: 5), some are dice (sneak attack:
// 3d6), and a few are both. Dice is nil when the resource is a plain count.
type LevelResource struct {
	Key    rules.Slug
	Number int
	Dice   *rules.Dice

	// Text carries the values that are neither a count nor a die: a druid's
	// maximum Wild Shape challenge rating is printed as "1/4", and rounding
	// it to an integer would silently let a level-2 druid turn into a bear.
	Text string
}

// Subclass is a class archetype: Thief, Berserker, School of Evocation.
type Subclass struct {
	Entry

	// Class is the parent class.
	Class rules.Slug

	// Flavor is what the parent class calls its archetypes -- "Roguish
	// Archetype", "Divine Domain", "Sacred Oath".
	//
	// The 2014 rules have no generic word for a subclass; each class names
	// its own. "Subclass" is the right term for the model, but this is the
	// term to put on screen, and it is localizable prose.
	Flavor string

	// Levels lists the class levels at which this subclass grants something.
	Levels []int

	// Spells are granted automatically at the listed levels, as domain and
	// oath spells are.
	Spells []SubclassSpell
}

// SubclassSpell is a spell a subclass grants at a given level, always
// prepared and never counting against the character's limit.
type SubclassSpell struct {
	Spell rules.Slug
	Level int
}

// Feature is something a class or subclass grants at a level: Sneak Attack,
// Cunning Action, Channel Divinity.
//
// A Feature comes from a class; a Trait comes from a race. The SRD keeps them
// in separate collections, and so does this catalogue, because a character
// needs to be able to say which source granted a given entry.
type Feature struct {
	Entry

	// Class is always set. Subclass is set only for archetype features.
	Class    rules.Slug
	Subclass rules.Slug

	// Level is the class level at which the feature is gained.
	Level int

	// Parent is set when this feature refines another, e.g. a Metamagic
	// option under Metamagic.
	Parent rules.Slug

	Prerequisites []Prerequisite

	// Specific carries the structured payload the twenty-five features that
	// have one attach. Nil for the rest, which are prose only.
	Specific *FeatureSpecific
}

// FeatureSpecific is the structured payload of the few features that have
// one. Every field is optional; nil means the feature does not use it.
type FeatureSpecific struct {
	// ExpertiseOptions is the rogue's and bard's prompt to double
	// proficiency in chosen skills.
	ExpertiseOptions *rules.Choice

	// SubfeatureOptions is a prompt to pick from a list of sub-features:
	// Metamagic, Fighting Style, Eldritch Invocations.
	SubfeatureOptions *rules.Choice

	// EnemyTypeOptions and TerrainTypeOptions are the ranger's Favored Enemy
	// and Natural Explorer prompts.
	EnemyTypeOptions   *rules.Choice
	TerrainTypeOptions *rules.Choice

	// Invocations lists the warlock invocations this feature makes available.
	Invocations []rules.Slug
}
