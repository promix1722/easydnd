package catalog

// The wire names for every enum the catalogue defines.
//
// The domain owns these because an enum that cannot name its own values
// forces every adapter to invent a spelling, and two adapters inventing
// separately is how "martial" becomes "Martial" in one of them. rules already
// works this way -- ChoiceKind, Proficiency and Ability all carry String --
// and these were simply never needed until something outside the file adapter
// had to serialize them.
//
// The spellings match internal/adapter/catalog/file/kinds.go exactly, and
// TestDomainNamesMatchTheWireVocabulary in that package pins the agreement so
// the two cannot drift apart unnoticed.

var prerequisiteKindNames = map[PrerequisiteKind]string{
	PrerequisiteNone:    "none",
	PrerequisiteAbility: "ability",
	PrerequisiteLevel:   "level",
	PrerequisiteEntry:   "entry",
}

// String returns the kind's wire name, or "unknown" outside the enumeration.
func (k PrerequisiteKind) String() string { return name(prerequisiteKindNames, k) }

var castingTimeKindNames = map[CastingTimeKind]string{
	CastingTimeNone:   "none",
	CastAsAction:      "action",
	CastAsBonusAction: "bonus-action",
	CastAsReaction:    "reaction",
	CastOverTime:      "over-time",
}

// String returns the kind's wire name, or "unknown" outside the enumeration.
func (k CastingTimeKind) String() string { return name(castingTimeKindNames, k) }

var spellRangeKindNames = map[SpellRangeKind]string{
	SpellRangeNone: "none",
	RangeSelf:      "self",
	RangeTouch:     "touch",
	RangeDistance:  "distance",
	RangeSight:     "sight",
	RangeUnlimited: "unlimited",
	RangeSpecial:   "special",
}

// String returns the kind's wire name, or "unknown" outside the enumeration.
func (k SpellRangeKind) String() string { return name(spellRangeKindNames, k) }

var durationKindNames = map[DurationKind]string{
	DurationNone:           "none",
	DurationInstantaneous:  "instantaneous",
	DurationTimed:          "timed",
	DurationUntilDispelled: "until-dispelled",
	DurationSpecial:        "special",
}

// String returns the kind's wire name, or "unknown" outside the enumeration.
func (k DurationKind) String() string { return name(durationKindNames, k) }

var timeUnitNames = map[TimeUnit]string{
	TimeUnitNone: "none",
	Round:        "round",
	Minute:       "minute",
	Hour:         "hour",
	Day:          "day",
}

// String returns the unit's wire name, or "unknown" outside the enumeration.
func (u TimeUnit) String() string { return name(timeUnitNames, u) }

var spellAttackTypeNames = map[SpellAttackType]string{
	SpellAttackNone:   "",
	MeleeSpellAttack:  "melee",
	RangedSpellAttack: "ranged",
}

// String returns the attack type's wire name, or "unknown" outside the
// enumeration. The zero value is the empty string, because a spell that makes
// no attack roll should serialize as an absent field rather than as "none".
func (t SpellAttackType) String() string { return name(spellAttackTypeNames, t) }

var saveEffectNames = map[SaveEffect]string{
	SaveEffectNone:   "",
	SaveNegates:      "negates",
	SaveHalvesDamage: "half",
	SaveOther:        "other",
}

// String returns the outcome's wire name, or "unknown" outside the
// enumeration.
func (e SaveEffect) String() string { return name(saveEffectNames, e) }

var areaKindNames = map[AreaKind]string{
	AreaNone:     "none",
	AreaCone:     "cone",
	AreaCube:     "cube",
	AreaCylinder: "cylinder",
	AreaLine:     "line",
	AreaSphere:   "sphere",
}

// String returns the shape's wire name, or "unknown" outside the enumeration.
func (k AreaKind) String() string { return name(areaKindNames, k) }

var weaponCategoryNames = map[WeaponCategory]string{
	WeaponCategoryNone: "",
	SimpleWeapon:       "simple",
	MartialWeapon:      "martial",
}

// String returns the category's wire name, or "unknown" outside the
// enumeration.
func (c WeaponCategory) String() string { return name(weaponCategoryNames, c) }

var weaponRangeNames = map[WeaponRange]string{
	WeaponRangeNone: "",
	MeleeWeapon:     "melee",
	RangedWeapon:    "ranged",
}

// String returns the range's wire name, or "unknown" outside the enumeration.
func (r WeaponRange) String() string { return name(weaponRangeNames, r) }

var armorCategoryNames = map[ArmorCategory]string{
	ArmorCategoryNone: "",
	LightArmor:        "light",
	MediumArmor:       "medium",
	HeavyArmor:        "heavy",
	Shield:            "shield",
}

// String returns the category's wire name, or "unknown" outside the
// enumeration.
func (c ArmorCategory) String() string { return name(armorCategoryNames, c) }

var rarityNames = map[Rarity]string{
	RarityNone:      "",
	RarityCommon:    "common",
	RarityUncommon:  "uncommon",
	RarityRare:      "rare",
	RarityVeryRare:  "very-rare",
	RarityLegendary: "legendary",
	RarityArtifact:  "artifact",
	RarityVaries:    "varies",
}

// String returns the rarity's wire name, or "unknown" outside the
// enumeration.
func (r Rarity) String() string { return name(rarityNames, r) }

var languageTypeNames = map[LanguageType]string{
	LanguageTypeNone: "",
	LanguageStandard: "standard",
	LanguageExotic:   "exotic",
}

// String returns the type's wire name, or "unknown" outside the enumeration.
func (t LanguageType) String() string { return name(languageTypeNames, t) }

var proficiencyTypeNames = map[ProficiencyType]string{
	ProficiencyTypeNone:           "",
	ProficiencyArmor:              "armor",
	ProficiencyWeapons:            "weapons",
	ProficiencyArtisansTools:      "artisans-tools",
	ProficiencyGamingSets:         "gaming-sets",
	ProficiencyMusicalInstruments: "musical-instruments",
	ProficiencyOtherTools:         "other-tools",
	ProficiencyVehicles:           "vehicles",
	ProficiencySkills:             "skills",
	ProficiencySavingThrows:       "saving-throws",
}

// String returns the type's wire name, or "unknown" outside the enumeration.
func (t ProficiencyType) String() string { return name(proficiencyTypeNames, t) }

// name looks up an enum's wire name, reporting anything outside the
// enumeration rather than returning an empty string that would serialize as a
// missing field.
func name[T comparable](names map[T]string, value T) string {
	if got, ok := names[value]; ok {
		return got
	}
	return "unknown"
}
