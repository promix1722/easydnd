// Package catalog serves the SRD compendium.
//
// The response shapes are declared here rather than reused from
// internal/adapter/catalog/file. That package's wire types are the *storage*
// format: they are language-neutral by design, carrying no names and no
// descriptions, because prose lives in a sibling i18n tree and is merged in
// at load time. Serving them would push the per-key locale fallback into the
// browser and would make regenerating the data a breaking API change.
//
// Reusing them would also have the inbound adapter import an outbound one.
// depguard denies adapter -> api but not api -> adapter, so that would pass
// lint while breaking the rule both are supposed to follow, and a convention
// lint cannot catch is the one that rots.
package catalog

// Entry is the part every catalogue entry has, already in the requested
// locale.
type Entry struct {
	Slug string   `json:"slug"`
	Name string   `json:"name"`
	Desc []string `json:"desc,omitempty"`
}

// Choice is a prompt: choose N of these.
//
// It is the same recursive grammar the compendium stores, with one addition:
// every option carries the key an answer names it by. The client never
// computes that key, it echoes what the server sent -- which is what keeps
// the rule "a bundle is named by its position" in one place.
type Choice struct {
	Prompt string    `json:"prompt"`
	Choose int       `json:"choose"`
	Kind   string    `json:"kind"`
	From   OptionSet `json:"from"`
}

// OptionSet is the pool a Choice draws from.
type OptionSet struct {
	// Kind is "explicit", "equipment-category" or "collection".
	Kind string `json:"kind"`

	Options []Option `json:"options,omitempty"`

	// Category is set when Kind is "equipment-category"; Collection when it
	// is "collection". Both mean "resolve this against the catalogue" --
	// there, an entry's own slug is the key.
	Category   string `json:"category,omitempty"`
	Collection string `json:"collection,omitempty"`
}

// Option is one selectable answer.
//
// The ten option kinds are flattened into one struct with a discriminator
// rather than a union, for the same reason the event log is: it keeps the
// client's rendering a switch on one field.
type Option struct {
	// Key is what an answer names this option by.
	Key  string `json:"key"`
	Kind string `json:"kind"`

	// Ref and Count are set for "ref".
	Ref   string `json:"ref,omitempty"`
	Count int    `json:"count,omitempty"`

	// Choice is set for "nested"; Items for "bundle".
	Choice *Choice  `json:"choice,omitempty"`
	Items  []Option `json:"items,omitempty"`

	// Ability and Bonus are set for "ability-bonus"; Ability and Minimum for
	// "score-minimum".
	Ability string `json:"ability,omitempty"`
	Bonus   int    `json:"bonus,omitempty"`
	Minimum int    `json:"minimum,omitempty"`

	// Damage is set for "damage", Cost for "money", Size for "size".
	Damage *Damage `json:"damage,omitempty"`
	Cost   *Cost   `json:"cost,omitempty"`
	Size   string  `json:"size,omitempty"`

	// Text is the resolved prose for a "text" option, an action's name, or a
	// damage option's note. It is resolved here because the compendium
	// stores only a key into the locale bundle, and an option a player
	// cannot read is an option they cannot pick.
	Text string `json:"text,omitempty"`

	// Alignments narrows which alignments an ideal suits; empty means any.
	Alignments []string `json:"alignments,omitempty"`

	// Recharge is set for "action".
	Recharge string `json:"recharge,omitempty"`
}

// Damage is a dice expression and its type.
type Damage struct {
	Dice string `json:"dice"`
	Type string `json:"type,omitempty"`
}

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

// Prerequisite is a condition on taking an entry.
type Prerequisite struct {
	Kind         string `json:"kind"`
	Ability      string `json:"ability,omitempty"`
	MinimumScore int    `json:"minimumScore,omitempty"`
	Level        int    `json:"level,omitempty"`
	Ref          string `json:"ref,omitempty"`
}

// The per-collection shapes. Each embeds Entry, so every response has a slug,
// a name and a description in the same place.

// Ability is one of the six ability scores.
type Ability struct {
	Entry
	FullName string   `json:"fullName,omitempty"`
	Skills   []string `json:"skills,omitempty"`
}

// Skill is one of the eighteen skills.
type Skill struct {
	Entry
	Ability string `json:"ability"`
}

// Alignment is one of the nine.
type Alignment struct {
	Entry
	Abbreviation string `json:"abbreviation,omitempty"`
}

// Language is a tongue.
type Language struct {
	Entry
	Type            string   `json:"type,omitempty"`
	Script          string   `json:"script,omitempty"`
	TypicalSpeakers []string `json:"typicalSpeakers,omitempty"`
}

// Proficiency is one named proficiency.
type Proficiency struct {
	Entry
	Type      string `json:"type,omitempty"`
	Reference string `json:"reference,omitempty"`
}

// EquipmentCategory groups items.
type EquipmentCategory struct {
	Entry
	Items []string `json:"items,omitempty"`
}

// Race is a player character race.
type Race struct {
	Entry
	Speed                 int            `json:"speed"`
	Size                  string         `json:"size,omitempty"`
	AbilityBonuses        []AbilityBonus `json:"abilityBonuses,omitempty"`
	AbilityBonusOptions   *Choice        `json:"abilityBonusOptions,omitempty"`
	Languages             []string       `json:"languages,omitempty"`
	LanguageOptions       *Choice        `json:"languageOptions,omitempty"`
	StartingProficiencies []string       `json:"startingProficiencies,omitempty"`
	ProficiencyOptions    []Choice       `json:"proficiencyOptions,omitempty"`
	Traits                []string       `json:"traits,omitempty"`
	Subraces              []string       `json:"subraces,omitempty"`
	AgeDesc               []string       `json:"ageDesc,omitempty"`
	AlignmentDesc         []string       `json:"alignmentDesc,omitempty"`
	SizeDesc              []string       `json:"sizeDesc,omitempty"`
	LanguageDesc          []string       `json:"languageDesc,omitempty"`
}

// Subrace is a variant of a race.
type Subrace struct {
	Entry
	Race                  string         `json:"race"`
	AbilityBonuses        []AbilityBonus `json:"abilityBonuses,omitempty"`
	Traits                []string       `json:"traits,omitempty"`
	StartingProficiencies []string       `json:"startingProficiencies,omitempty"`
	LanguageOptions       *Choice        `json:"languageOptions,omitempty"`
}

// Trait is a racial trait.
type Trait struct {
	Entry
	Races              []string `json:"races,omitempty"`
	Subraces           []string `json:"subraces,omitempty"`
	Proficiencies      []string `json:"proficiencies,omitempty"`
	ProficiencyOptions *Choice  `json:"proficiencyOptions,omitempty"`
	BreathWeapon       *Choice  `json:"breathWeapon,omitempty"`
	SpellOptions       *Choice  `json:"spellOptions,omitempty"`
	SubtraitOptions    *Choice  `json:"subtraitOptions,omitempty"`
	DamageResistance   []string `json:"damageResistance,omitempty"`
}

// Spellcasting describes how a class casts.
type Spellcasting struct {
	Level   int         `json:"level"`
	Ability string      `json:"ability"`
	Info    []NamedText `json:"info,omitempty"`
}

// NamedText is a titled block of prose.
type NamedText struct {
	Name string   `json:"name"`
	Desc []string `json:"desc,omitempty"`
}

// MultiClassing states what a class costs and grants on multiclassing in.
type MultiClassing struct {
	Prerequisites      []Prerequisite `json:"prerequisites,omitempty"`
	Proficiencies      []string       `json:"proficiencies,omitempty"`
	ProficiencyOptions []Choice       `json:"proficiencyOptions,omitempty"`
}

// Class is a character class.
type Class struct {
	Entry
	HitDie                   int            `json:"hitDie"`
	SavingThrows             []string       `json:"savingThrows,omitempty"`
	Proficiencies            []string       `json:"proficiencies,omitempty"`
	ProficiencyOptions       []Choice       `json:"proficiencyOptions,omitempty"`
	StartingEquipment        []ItemStack    `json:"startingEquipment,omitempty"`
	StartingEquipmentOptions []Choice       `json:"startingEquipmentOptions,omitempty"`
	Subclasses               []string       `json:"subclasses,omitempty"`
	Spellcasting             *Spellcasting  `json:"spellcasting,omitempty"`
	MultiClassing            *MultiClassing `json:"multiClassing,omitempty"`

	// SubclassLevel is the class level at which the subclass is chosen. It is
	// derived from where the subclass's own advancement rows begin, and is
	// served because a client would otherwise have to derive it too.
	SubclassLevel int `json:"subclassLevel,omitempty"`
}

// LevelResource is one class resource at one level.
type LevelResource struct {
	Key    string `json:"key"`
	Number int    `json:"number,omitempty"`
	Dice   string `json:"dice,omitempty"`
	Text   string `json:"text,omitempty"`
}

// ClassLevel is one row of a class's advancement table.
type ClassLevel struct {
	Class               string          `json:"class"`
	Subclass            string          `json:"subclass,omitempty"`
	Level               int             `json:"level"`
	ProficiencyBonus    int             `json:"proficiencyBonus,omitempty"`
	AbilityScoreBonuses int             `json:"abilityScoreBonuses,omitempty"`
	Features            []string        `json:"features,omitempty"`
	SpellSlots          map[string]int  `json:"spellSlots,omitempty"`
	CantripsKnown       int             `json:"cantripsKnown,omitempty"`
	SpellsKnown         int             `json:"spellsKnown,omitempty"`
	Resources           []LevelResource `json:"resources,omitempty"`
}

// Subclass is a class's archetype.
type Subclass struct {
	Entry
	Class  string `json:"class"`
	Flavor string `json:"flavor,omitempty"`
}

// Feature is a class or subclass feature.
type Feature struct {
	Entry
	Class              string   `json:"class,omitempty"`
	Subclass           string   `json:"subclass,omitempty"`
	Level              int      `json:"level,omitempty"`
	Prerequisites      []string `json:"prerequisites,omitempty"`
	ExpertiseOptions   *Choice  `json:"expertiseOptions,omitempty"`
	SubfeatureOptions  *Choice  `json:"subfeatureOptions,omitempty"`
	EnemyTypeOptions   *Choice  `json:"enemyTypeOptions,omitempty"`
	TerrainTypeOptions *Choice  `json:"terrainTypeOptions,omitempty"`
}

// Background is where a character came from.
type Background struct {
	Entry
	StartingProficiencies    []string    `json:"startingProficiencies,omitempty"`
	LanguageOptions          *Choice     `json:"languageOptions,omitempty"`
	StartingEquipment        []ItemStack `json:"startingEquipment,omitempty"`
	StartingEquipmentOptions []Choice    `json:"startingEquipmentOptions,omitempty"`
	StartingGold             *Cost       `json:"startingGold,omitempty"`
	Feature                  string      `json:"feature,omitempty"`
	PersonalityTraits        *Choice     `json:"personalityTraits,omitempty"`
	Ideals                   *Choice     `json:"ideals,omitempty"`
	Bonds                    *Choice     `json:"bonds,omitempty"`
	Flaws                    *Choice     `json:"flaws,omitempty"`
}

// Feat is an optional talent.
type Feat struct {
	Entry
	Prerequisites []Prerequisite `json:"prerequisites,omitempty"`
}

// Weapon is the weapon part of an item.
type Weapon struct {
	Category            string   `json:"category,omitempty"`
	Range               string   `json:"range,omitempty"`
	Damage              *Damage  `json:"damage,omitempty"`
	TwoHandedDamage     *Damage  `json:"twoHandedDamage,omitempty"`
	NormalRange         int      `json:"normalRange,omitempty"`
	LongRange           int      `json:"longRange,omitempty"`
	ThrowNormalRange    int      `json:"throwNormalRange,omitempty"`
	ThrowLongRange      int      `json:"throwLongRange,omitempty"`
	Properties          []string `json:"properties,omitempty"`
	CategoryDescription string   `json:"categoryDescription,omitempty"`
}

// Armor is the armor part of an item.
type Armor struct {
	Category            string `json:"category,omitempty"`
	BaseAC              int    `json:"baseAC"`
	AddsDexBonus        bool   `json:"addsDexBonus"`
	MaxDexBonus         *int   `json:"maxDexBonus,omitempty"`
	StrengthMinimum     int    `json:"strengthMinimum,omitempty"`
	StealthDisadvantage bool   `json:"stealthDisadvantage,omitempty"`
}

// Gear is the adventuring-gear part of an item.
type Gear struct {
	GearCategory string      `json:"gearCategory,omitempty"`
	Quantity     int         `json:"quantity,omitempty"`
	Contents     []ItemStack `json:"contents,omitempty"`
}

// Tool is the tool part of an item.
type Tool struct {
	ToolCategory string `json:"toolCategory,omitempty"`
}

// Vehicle is the vehicle part of an item.
type Vehicle struct {
	VehicleCategory string  `json:"vehicleCategory,omitempty"`
	Speed           float64 `json:"speed,omitempty"`
	SpeedUnit       string  `json:"speedUnit,omitempty"`
	Capacity        string  `json:"capacity,omitempty"`
}

// Item is a piece of equipment.
type Item struct {
	Entry
	Category string   `json:"category,omitempty"`
	Cost     *Cost    `json:"cost,omitempty"`
	Weight   float64  `json:"weight,omitempty"`
	Weapon   *Weapon  `json:"weapon,omitempty"`
	Armor    *Armor   `json:"armor,omitempty"`
	Gear     *Gear    `json:"gear,omitempty"`
	Tool     *Tool    `json:"tool,omitempty"`
	Vehicle  *Vehicle `json:"vehicle,omitempty"`
}

// MagicItem is a magical piece of equipment.
type MagicItem struct {
	Entry
	Category string   `json:"category,omitempty"`
	Rarity   string   `json:"rarity,omitempty"`
	Variant  bool     `json:"variant,omitempty"`
	Variants []string `json:"variants,omitempty"`
}

// Spell is a spell.
//
// The collection endpoint serves the summary half of this -- slug, name,
// level, school, classes -- because 319 spells at full fidelity is a payload
// nobody needs in order to build a character. Full detail comes from the same
// endpoint with ?slugs=.
type Spell struct {
	Entry
	Level         int               `json:"level"`
	School        string            `json:"school,omitempty"`
	Classes       []string          `json:"classes,omitempty"`
	Subclasses    []string          `json:"subclasses,omitempty"`
	Ritual        bool              `json:"ritual,omitempty"`
	Concentration bool              `json:"concentration,omitempty"`
	CastingTime   *RuleValue        `json:"castingTime,omitempty"`
	Range         *RuleValue        `json:"range,omitempty"`
	Duration      *RuleValue        `json:"duration,omitempty"`
	Components    *Components       `json:"components,omitempty"`
	AttackType    string            `json:"attackType,omitempty"`
	SavingThrow   *SavingThrow      `json:"savingThrow,omitempty"`
	AreaOfEffect  *Area             `json:"areaOfEffect,omitempty"`
	Damage        *SpellDamage      `json:"damage,omitempty"`
	Healing       map[string]string `json:"healing,omitempty"`
	HigherLevel   []string          `json:"higherLevel,omitempty"`
}

// RuleValue is a structured rule string: a casting time, a range, a duration.
//
// The SRD writes these as prose -- "1 action", "90 feet", "Up to 1 minute" --
// and the compendium stores them structured so that a Russian sheet does not
// read "90 feet". The client renders them per locale.
type RuleValue struct {
	Kind     string `json:"kind"`
	Amount   int    `json:"amount,omitempty"`
	Unit     string `json:"unit,omitempty"`
	Distance int    `json:"distance,omitempty"`
	UpTo     bool   `json:"upTo,omitempty"`
}

// Components is a spell's verbal, somatic and material requirements.
type Components struct {
	Verbal   bool   `json:"verbal,omitempty"`
	Somatic  bool   `json:"somatic,omitempty"`
	Material bool   `json:"material,omitempty"`
	Consumed bool   `json:"consumed,omitempty"`
	Text     string `json:"text,omitempty"`
}

// SavingThrow is the save a spell calls for.
type SavingThrow struct {
	Ability string `json:"ability"`
	Success string `json:"success,omitempty"`
}

// Area is a spell's area of effect.
type Area struct {
	Shape string `json:"shape"`
	Size  int    `json:"size"`
}

// SpellDamage is a spell's damage and how it scales.
type SpellDamage struct {
	Type        string            `json:"type,omitempty"`
	AtSlotLevel map[string]string `json:"atSlotLevel,omitempty"`
	AtCharLevel map[string]string `json:"atCharacterLevel,omitempty"`
}

// Term is prose the choice grammar points at by key.
type Term struct {
	Entry
}
