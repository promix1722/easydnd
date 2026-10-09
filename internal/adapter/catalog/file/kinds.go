package file

import (
	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// The wire vocabulary: every enum the on-disk format spells as a string.
//
// These constants are exported because the tools that write a pack use them
// and this package reads them. Sharing the spelling is what stops a writer and
// the loader from disagreeing about, say, whether a two-handed weapon's
// category is "martial" or "Martial" -- a mismatch that would surface as a
// silently uncategorised weapon rather than as an error.

// Option kinds.
const (
	OptionRef          = "ref"
	OptionNested       = "nested"
	OptionBundle       = "bundle"
	OptionAbilityBonus = "ability-bonus"
	OptionDamage       = "damage"
	OptionMoney        = "money"
	OptionSize         = "size"
	OptionText         = "text"
	OptionAction       = "action"
	OptionScoreMinimum = "score-minimum"
)

// Option set kinds.
const (
	OptionSetExplicit          = "explicit"
	OptionSetEquipmentCategory = "equipment-category"
	OptionSetCollection        = "collection"
)

// Prerequisite kinds.
const (
	PrerequisiteAbility = "ability"
	PrerequisiteLevel   = "level"
	PrerequisiteEntry   = "entry"
)

// Casting time kinds.
const (
	CastAction      = "action"
	CastBonusAction = "bonus-action"
	CastReaction    = "reaction"
	CastOverTime    = "over-time"
)

// Spell range kinds.
const (
	RangeSelf      = "self"
	RangeTouch     = "touch"
	RangeDistance  = "distance"
	RangeSight     = "sight"
	RangeUnlimited = "unlimited"
	RangeSpecial   = "special"
)

// Duration kinds.
const (
	DurationInstantaneous  = "instantaneous"
	DurationTimed          = "timed"
	DurationUntilDispelled = "until-dispelled"
	DurationSpecial        = "special"
)

// Time units.
const (
	UnitRound  = "round"
	UnitMinute = "minute"
	UnitHour   = "hour"
	UnitDay    = "day"
)

// Spell attack types.
const (
	AttackMelee  = "melee"
	AttackRanged = "ranged"
)

// Saving throw outcomes.
const (
	SaveNegates = "negates"
	SaveHalf    = "half"
	SaveOther   = "other"
)

// Area of effect shapes.
const (
	AreaCone     = "cone"
	AreaCube     = "cube"
	AreaCylinder = "cylinder"
	AreaLine     = "line"
	AreaSphere   = "sphere"
)

// ActionKinds are the parts of a turn an action tag or a standalone action
// may name.
var ActionKinds = []string{"action", "bonus-action", "reaction", "free-action"}

// Weapon categories and ranges.
const (
	WeaponSimple  = "simple"
	WeaponMartial = "martial"
	WeaponMelee   = "melee"
	WeaponRanged  = "ranged"
)

// Armor categories.
const (
	ArmorLight  = "light"
	ArmorMedium = "medium"
	ArmorHeavy  = "heavy"
	ArmorShield = "shield"
)

// Equipment slots: where an item is worn or wielded.
const (
	SlotHead     = "head"
	SlotNeck     = "neck"
	SlotBack     = "back"
	SlotBody     = "body"
	SlotArms     = "arms"
	SlotWaist    = "waist"
	SlotFeet     = "feet"
	SlotRing     = "ring"
	SlotMainHand = "main-hand"
	SlotOffHand  = "off-hand"
)

// Magic item rarities.
const (
	RarityCommon    = "common"
	RarityUncommon  = "uncommon"
	RarityRare      = "rare"
	RarityVeryRare  = "very-rare"
	RarityLegendary = "legendary"
	RarityArtifact  = "artifact"
	RarityVaries    = "varies"
)

// Language types.
const (
	LanguageStandard = "standard"
	LanguageExotic   = "exotic"
)

// Proficiency types.
const (
	ProficiencyArmor              = "armor"
	ProficiencyWeapons            = "weapons"
	ProficiencyArtisansTools      = "artisans-tools"
	ProficiencyGamingSets         = "gaming-sets"
	ProficiencyMusicalInstruments = "musical-instruments"
	ProficiencyOtherTools         = "other-tools"
	ProficiencyVehicles           = "vehicles"
	ProficiencySkills             = "skills"
	ProficiencySavingThrows       = "saving-throws"
)

var optionSetKinds = map[string]rules.OptionSetKind{
	OptionSetExplicit:          rules.OptionsExplicit,
	OptionSetEquipmentCategory: rules.OptionsFromEquipmentCategory,
	OptionSetCollection:        rules.OptionsFromCollection,
}

var prerequisiteKinds = map[string]catalog.PrerequisiteKind{
	PrerequisiteAbility: catalog.PrerequisiteAbility,
	PrerequisiteLevel:   catalog.PrerequisiteLevel,
	PrerequisiteEntry:   catalog.PrerequisiteEntry,
}

var castingTimeKinds = map[string]catalog.CastingTimeKind{
	CastAction:      catalog.CastAsAction,
	CastBonusAction: catalog.CastAsBonusAction,
	CastReaction:    catalog.CastAsReaction,
	CastOverTime:    catalog.CastOverTime,
}

var spellRangeKinds = map[string]catalog.SpellRangeKind{
	RangeSelf:      catalog.RangeSelf,
	RangeTouch:     catalog.RangeTouch,
	RangeDistance:  catalog.RangeDistance,
	RangeSight:     catalog.RangeSight,
	RangeUnlimited: catalog.RangeUnlimited,
	RangeSpecial:   catalog.RangeSpecial,
}

var durationKinds = map[string]catalog.DurationKind{
	DurationInstantaneous:  catalog.DurationInstantaneous,
	DurationTimed:          catalog.DurationTimed,
	DurationUntilDispelled: catalog.DurationUntilDispelled,
	DurationSpecial:        catalog.DurationSpecial,
}

var timeUnits = map[string]catalog.TimeUnit{
	UnitRound:  catalog.Round,
	UnitMinute: catalog.Minute,
	UnitHour:   catalog.Hour,
	UnitDay:    catalog.Day,
}

var attackTypes = map[string]catalog.SpellAttackType{
	AttackMelee:  catalog.MeleeSpellAttack,
	AttackRanged: catalog.RangedSpellAttack,
}

var saveEffects = map[string]catalog.SaveEffect{
	SaveNegates: catalog.SaveNegates,
	SaveHalf:    catalog.SaveHalvesDamage,
	SaveOther:   catalog.SaveOther,
}

var areaKinds = map[string]catalog.AreaKind{
	AreaCone:     catalog.AreaCone,
	AreaCube:     catalog.AreaCube,
	AreaCylinder: catalog.AreaCylinder,
	AreaLine:     catalog.AreaLine,
	AreaSphere:   catalog.AreaSphere,
}

var weaponCategories = map[string]catalog.WeaponCategory{
	WeaponSimple:  catalog.SimpleWeapon,
	WeaponMartial: catalog.MartialWeapon,
}

var weaponRanges = map[string]catalog.WeaponRange{
	WeaponMelee:  catalog.MeleeWeapon,
	WeaponRanged: catalog.RangedWeapon,
}

var armorCategories = map[string]catalog.ArmorCategory{
	ArmorLight:  catalog.LightArmor,
	ArmorMedium: catalog.MediumArmor,
	ArmorHeavy:  catalog.HeavyArmor,
	ArmorShield: catalog.Shield,
}

var slots = map[string]catalog.Slot{
	SlotHead: catalog.SlotHead,
	SlotNeck: catalog.SlotNeck,
	SlotBack: catalog.SlotBack,
	SlotBody: catalog.SlotBody,
	SlotArms: catalog.SlotArms,
	// "hands" was its own slot once. Archived SRD releases and the private
	// 2014 pack still write it, so it stays readable, as arms.
	"hands":      catalog.SlotArms,
	SlotWaist:    catalog.SlotWaist,
	SlotFeet:     catalog.SlotFeet,
	SlotRing:     catalog.SlotRing,
	SlotMainHand: catalog.SlotMainHand,
	SlotOffHand:  catalog.SlotOffHand,
}

var rarities = map[string]catalog.Rarity{
	RarityCommon:    catalog.RarityCommon,
	RarityUncommon:  catalog.RarityUncommon,
	RarityRare:      catalog.RarityRare,
	RarityVeryRare:  catalog.RarityVeryRare,
	RarityLegendary: catalog.RarityLegendary,
	RarityArtifact:  catalog.RarityArtifact,
	RarityVaries:    catalog.RarityVaries,
}

var languageTypes = map[string]catalog.LanguageType{
	LanguageStandard: catalog.LanguageStandard,
	LanguageExotic:   catalog.LanguageExotic,
}

var proficiencyTypes = map[string]catalog.ProficiencyType{
	ProficiencyArmor:              catalog.ProficiencyArmor,
	ProficiencyWeapons:            catalog.ProficiencyWeapons,
	ProficiencyArtisansTools:      catalog.ProficiencyArtisansTools,
	ProficiencyGamingSets:         catalog.ProficiencyGamingSets,
	ProficiencyMusicalInstruments: catalog.ProficiencyMusicalInstruments,
	ProficiencyOtherTools:         catalog.ProficiencyOtherTools,
	ProficiencyVehicles:           catalog.ProficiencyVehicles,
	ProficiencySkills:             catalog.ProficiencySkills,
	ProficiencySavingThrows:       catalog.ProficiencySavingThrows,
}
