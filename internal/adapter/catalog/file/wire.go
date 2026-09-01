// Package file loads the SRD compendium from a directory of JSON files.
//
// It is the only place in the project where catalogue JSON struct tags exist.
// The domain forbids them, so the shapes below mirror the domain types with
// their enums flattened to strings and their optional fields made pointers.
//
// The wire types are exported because cmd/srdgen builds them and marshals
// them, while this package unmarshals them. Sharing one definition is what
// guarantees the generator and the loader cannot drift: a change to the format
// is a compile error in both, not a runtime surprise in one.
//
// # Layout
//
// Mechanics are language-neutral and live in the directory root. Prose lives
// under i18n/<locale>/ keyed by the same slug, so adding a translation never
// touches a mechanics file and a partial locale falls back per key.
//
//	data/srd_5.1/
//	  manifest.json
//	  spells.json  races.json  classes.json  ...
//	  i18n/en/spells.json  i18n/ru/spells.json  ...
package file

// Manifest describes a data directory: what produced it, and what it holds.
type Manifest struct {
	// Ruleset is the rules edition, e.g. "2014".
	Ruleset string `json:"ruleset"`

	// Source names where the data was derived from.
	Source string `json:"source"`

	// Locales lists the locale directories present under i18n/.
	Locales []string `json:"locales"`

	// Counts maps each mechanics file to its entry count. It exists so a
	// truncated write is caught at load rather than showing up as a spell
	// that mysteriously does not exist.
	Counts map[string]int `json:"counts"`
}

// Ref is a typed reference to another entry, written as "kind:slug".
type Ref string

// Cost is a price in one coin denomination.
type Cost struct {
	Amount int    `json:"amount"`
	Unit   string `json:"unit"`
}

// AbilityBonus is a fixed increase to one ability.
type AbilityBonus struct {
	Ability string `json:"ability"`
	Bonus   int    `json:"bonus"`
}

// ItemStack is a quantity of one item.
type ItemStack struct {
	Item  string `json:"item"`
	Count int    `json:"count"`
}

// Prerequisite is a condition that must hold before an entry applies.
type Prerequisite struct {
	Kind string `json:"kind"`

	Ability      string `json:"ability,omitempty"`
	MinimumScore int    `json:"minimumScore,omitempty"`
	Level        int    `json:"level,omitempty"`
	Ref          Ref    `json:"ref,omitempty"`
}

// Choice is "choose N of these", and nests.
type Choice struct {
	Prompt string    `json:"prompt"`
	Choose int       `json:"choose"`
	Kind   string    `json:"kind"`
	From   OptionSet `json:"from"`
}

// OptionSet is the pool a Choice draws from.
type OptionSet struct {
	Kind string `json:"kind"`

	Options []Option `json:"options,omitempty"`

	Category string `json:"category,omitempty"`

	Collection string `json:"collection,omitempty"`
}

// Option is one selectable answer.
//
// The domain models this as a sealed interface; on the wire it is a tagged
// struct, because a discriminated union is what JSON can express and a type
// switch on decode is what turns it back into the interface.
type Option struct {
	Kind string `json:"kind"`

	Ref   Ref `json:"ref,omitempty"`
	Count int `json:"count,omitempty"`

	Choice *Choice  `json:"choice,omitempty"`
	Items  []Option `json:"items,omitempty"`

	Ability string `json:"ability,omitempty"`
	Bonus   int    `json:"bonus,omitempty"`
	Minimum int    `json:"minimum,omitempty"`

	Dice       string `json:"dice,omitempty"`
	DamageType string `json:"damageType,omitempty"`

	Cost *Cost `json:"cost,omitempty"`

	Size string `json:"size,omitempty"`

	Key        string   `json:"key,omitempty"`
	Alignments []string `json:"alignments,omitempty"`
	Recharge   string   `json:"recharge,omitempty"`
}

// AbilityScore is one of the six abilities.
type AbilityScore struct {
	Slug   string   `json:"slug"`
	Skills []string `json:"skills,omitempty"`
}

// Skill is a proficiency applied to ability checks.
type Skill struct {
	Slug    string `json:"slug"`
	Ability string `json:"ability"`
}

// Named is an entry whose every attribute is prose: conditions, damage types,
// magic schools, weapon properties and alignments all reduce to this.
type Named struct {
	Slug string `json:"slug"`
}

// Language is a tongue a character can know.
type Language struct {
	Slug string `json:"slug"`
	Type string `json:"type"`
}

// Proficiency is a single named proficiency a class, race or background grants.
type Proficiency struct {
	Slug      string   `json:"slug"`
	Type      string   `json:"type"`
	Reference Ref      `json:"reference,omitempty"`
	Classes   []string `json:"classes,omitempty"`
	Races     []string `json:"races,omitempty"`
}

// EquipmentCategory groups items.
type EquipmentCategory struct {
	Slug  string   `json:"slug"`
	Items []string `json:"items,omitempty"`
}

// Race is a player character race.
type Race struct {
	Slug                  string         `json:"slug"`
	Speed                 int            `json:"speed"`
	Size                  string         `json:"size"`
	AbilityBonuses        []AbilityBonus `json:"abilityBonuses,omitempty"`
	AbilityBonusOptions   *Choice        `json:"abilityBonusOptions,omitempty"`
	Languages             []string       `json:"languages,omitempty"`
	LanguageOptions       *Choice        `json:"languageOptions,omitempty"`
	StartingProficiencies []string       `json:"startingProficiencies,omitempty"`
	ProficiencyOptions    []Choice       `json:"proficiencyOptions,omitempty"`
	Traits                []string       `json:"traits,omitempty"`
	Subraces              []string       `json:"subraces,omitempty"`
}

// Subrace is a variant of a Race.
type Subrace struct {
	Slug                  string         `json:"slug"`
	Race                  string         `json:"race"`
	AbilityBonuses        []AbilityBonus `json:"abilityBonuses,omitempty"`
	Traits                []string       `json:"traits,omitempty"`
	StartingProficiencies []string       `json:"startingProficiencies,omitempty"`
	LanguageOptions       *Choice        `json:"languageOptions,omitempty"`
}

// Trait is a racial trait.
type Trait struct {
	Slug               string   `json:"slug"`
	Races              []string `json:"races,omitempty"`
	Subraces           []string `json:"subraces,omitempty"`
	Proficiencies      []string `json:"proficiencies,omitempty"`
	ProficiencyOptions *Choice  `json:"proficiencyOptions,omitempty"`

	BreathWeapon     *Choice  `json:"breathWeapon,omitempty"`
	SpellOptions     *Choice  `json:"spellOptions,omitempty"`
	SubtraitOptions  *Choice  `json:"subtraitOptions,omitempty"`
	DamageResistance []string `json:"damageResistance,omitempty"`
}

// Class is a character class.
type Class struct {
	Slug                     string      `json:"slug"`
	HitDie                   int         `json:"hitDie"`
	SavingThrows             []string    `json:"savingThrows,omitempty"`
	Proficiencies            []string    `json:"proficiencies,omitempty"`
	ProficiencyOptions       []Choice    `json:"proficiencyOptions,omitempty"`
	StartingEquipment        []ItemStack `json:"startingEquipment,omitempty"`
	StartingEquipmentOptions []Choice    `json:"startingEquipmentOptions,omitempty"`
	Subclasses               []string    `json:"subclasses,omitempty"`

	SpellcastingLevel   int    `json:"spellcastingLevel,omitempty"`
	SpellcastingAbility string `json:"spellcastingAbility,omitempty"`

	MulticlassPrerequisites []Prerequisite `json:"multiclassPrerequisites,omitempty"`
	MulticlassProficiencies []string       `json:"multiclassProficiencies,omitempty"`
	MulticlassOptions       []Choice       `json:"multiclassOptions,omitempty"`
}

// ClassLevel is one row of a class advancement table.
type ClassLevel struct {
	Class    string `json:"class"`
	Subclass string `json:"subclass,omitempty"`
	Level    int    `json:"level"`

	ProficiencyBonus    int `json:"proficiencyBonus"`
	AbilityScoreBonuses int `json:"abilityScoreBonuses,omitempty"`

	Features []string `json:"features,omitempty"`

	// SpellSlots is keyed by spell level as a string, because JSON object
	// keys are strings and a sparse map is smaller than a ten-element array
	// that is empty for two thirds of all rows.
	SpellSlots    map[string]int `json:"spellSlots,omitempty"`
	CantripsKnown int            `json:"cantripsKnown,omitempty"`
	SpellsKnown   int            `json:"spellsKnown,omitempty"`

	Resources []LevelResource `json:"resources,omitempty"`
}

// LevelResource is one class-specific value at one level.
type LevelResource struct {
	Key    string `json:"key"`
	Number int    `json:"number,omitempty"`
	Dice   string `json:"dice,omitempty"`
	Text   string `json:"text,omitempty"`
}

// Subclass is a class archetype.
type Subclass struct {
	Slug   string          `json:"slug"`
	Class  string          `json:"class"`
	Levels []int           `json:"levels,omitempty"`
	Spells []SubclassSpell `json:"spells,omitempty"`
}

// SubclassSpell is a spell a subclass grants at a level.
type SubclassSpell struct {
	Spell string `json:"spell"`
	Level int    `json:"level"`
}

// Feature is something a class or subclass grants at a level.
type Feature struct {
	Slug          string         `json:"slug"`
	Class         string         `json:"class,omitempty"`
	Subclass      string         `json:"subclass,omitempty"`
	Level         int            `json:"level,omitempty"`
	Parent        string         `json:"parent,omitempty"`
	Prerequisites []Prerequisite `json:"prerequisites,omitempty"`

	ExpertiseOptions   *Choice  `json:"expertiseOptions,omitempty"`
	SubfeatureOptions  *Choice  `json:"subfeatureOptions,omitempty"`
	EnemyTypeOptions   *Choice  `json:"enemyTypeOptions,omitempty"`
	TerrainTypeOptions *Choice  `json:"terrainTypeOptions,omitempty"`
	Invocations        []string `json:"invocations,omitempty"`
}

// Background is where a character came from before adventuring.
type Background struct {
	Slug                     string      `json:"slug"`
	StartingProficiencies    []string    `json:"startingProficiencies,omitempty"`
	LanguageOptions          *Choice     `json:"languageOptions,omitempty"`
	StartingEquipment        []ItemStack `json:"startingEquipment,omitempty"`
	StartingEquipmentOptions []Choice    `json:"startingEquipmentOptions,omitempty"`
	StartingGold             *Cost       `json:"startingGold,omitempty"`
	Feature                  string      `json:"feature,omitempty"`

	PersonalityTraits *Choice `json:"personalityTraits,omitempty"`
	Ideals            *Choice `json:"ideals,omitempty"`
	Bonds             *Choice `json:"bonds,omitempty"`
	Flaws             *Choice `json:"flaws,omitempty"`
}

// Feat is an optional talent.
type Feat struct {
	Slug          string         `json:"slug"`
	Prerequisites []Prerequisite `json:"prerequisites,omitempty"`
}

// Item is a piece of mundane equipment.
type Item struct {
	Slug     string  `json:"slug"`
	Category string  `json:"category"`
	Cost     Cost    `json:"cost"`
	Weight   float64 `json:"weight,omitempty"`

	Weapon  *Weapon  `json:"weapon,omitempty"`
	Armor   *Armor   `json:"armor,omitempty"`
	Gear    *Gear    `json:"gear,omitempty"`
	Tool    *Tool    `json:"tool,omitempty"`
	Vehicle *Vehicle `json:"vehicle,omitempty"`
}

// Weapon is the weapon-specific part of an Item.
type Weapon struct {
	Category string `json:"category"`
	Range    string `json:"range"`

	Dice       string `json:"dice,omitempty"`
	DamageType string `json:"damageType,omitempty"`

	TwoHandedDice       string `json:"twoHandedDice,omitempty"`
	TwoHandedDamageType string `json:"twoHandedDamageType,omitempty"`

	NormalRange int `json:"normalRange,omitempty"`
	LongRange   int `json:"longRange,omitempty"`
	ThrowNormal int `json:"throwNormal,omitempty"`
	ThrowLong   int `json:"throwLong,omitempty"`

	Properties []string `json:"properties,omitempty"`
}

// Armor is the armor-specific part of an Item.
type Armor struct {
	Category            string `json:"category"`
	BaseAC              int    `json:"baseAC"`
	AddsDexBonus        bool   `json:"addsDexBonus"`
	MaxDexBonus         *int   `json:"maxDexBonus,omitempty"`
	StrengthMinimum     int    `json:"strengthMinimum,omitempty"`
	StealthDisadvantage bool   `json:"stealthDisadvantage,omitempty"`
}

// Gear is the adventuring-gear part of an Item.
type Gear struct {
	GearCategory string      `json:"gearCategory,omitempty"`
	Quantity     int         `json:"quantity,omitempty"`
	Contents     []ItemStack `json:"contents,omitempty"`
}

// Tool is the tool part of an Item.
type Tool struct {
	ToolCategory string `json:"toolCategory,omitempty"`
}

// Vehicle is the mount-or-vehicle part of an Item.
type Vehicle struct {
	VehicleCategory string  `json:"vehicleCategory,omitempty"`
	Speed           float64 `json:"speed,omitempty"`
	SpeedUnit       string  `json:"speedUnit,omitempty"`
	Capacity        string  `json:"capacity,omitempty"`
}

// MagicItem is an enchanted item.
type MagicItem struct {
	Slug      string   `json:"slug"`
	Category  string   `json:"category,omitempty"`
	Rarity    string   `json:"rarity,omitempty"`
	Variants  []string `json:"variants,omitempty"`
	IsVariant bool     `json:"isVariant,omitempty"`
}

// Spell is a spell or cantrip.
type Spell struct {
	Slug   string `json:"slug"`
	Source string `json:"source,omitempty"`
	Level  int    `json:"level"`
	School string `json:"school"`

	CastingTime CastingTime `json:"castingTime"`
	Range       SpellRange  `json:"range"`
	Duration    Duration    `json:"duration"`
	Components  Components  `json:"components"`

	Ritual        bool `json:"ritual,omitempty"`
	Concentration bool `json:"concentration,omitempty"`

	AttackType string       `json:"attackType,omitempty"`
	Save       *SavingThrow `json:"save,omitempty"`

	Damage *SpellDamage  `json:"damage,omitempty"`
	Heal   *SpellScaling `json:"heal,omitempty"`
	Area   *AreaOfEffect `json:"area,omitempty"`

	Classes    []string `json:"classes,omitempty"`
	Subclasses []string `json:"subclasses,omitempty"`
}

// CastingTime is how long a spell takes to cast.
type CastingTime struct {
	Kind   string `json:"kind"`
	Amount int    `json:"amount,omitempty"`
	Unit   string `json:"unit,omitempty"`
}

// SpellRange is how far a spell reaches.
type SpellRange struct {
	Kind     string `json:"kind"`
	Distance int    `json:"distance,omitempty"`
}

// Duration is how long a spell's effect lasts.
type Duration struct {
	Kind   string `json:"kind"`
	Amount int    `json:"amount,omitempty"`
	Unit   string `json:"unit,omitempty"`
	UpTo   bool   `json:"upTo,omitempty"`
}

// Components are a spell's verbal, somatic and material requirements.
type Components struct {
	Verbal   bool `json:"verbal,omitempty"`
	Somatic  bool `json:"somatic,omitempty"`
	Material bool `json:"material,omitempty"`
}

// SavingThrow is the save a spell forces.
type SavingThrow struct {
	Ability string `json:"ability"`
	Success string `json:"success,omitempty"`
}

// SpellScaling is a value that grows with slot level or character level.
type SpellScaling struct {
	AtSlotLevel      map[string]string `json:"atSlotLevel,omitempty"`
	AtCharacterLevel map[string]string `json:"atCharacterLevel,omitempty"`
}

// SpellDamage is a spell's damage, typed and scaled.
type SpellDamage struct {
	Type    string       `json:"type,omitempty"`
	Scaling SpellScaling `json:"scaling"`
}

// AreaOfEffect is the region a spell covers.
type AreaOfEffect struct {
	Kind string `json:"kind"`
	Size int    `json:"size"`
}
