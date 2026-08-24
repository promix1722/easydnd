package catalog

import "github.com/promix1722/easydnd/internal/domain/rules"

// ItemStack is a quantity of one item. It is the unit of a starting equipment
// list and of a character's inventory.
type ItemStack struct {
	Item  rules.Slug
	Count int
}

// Item is a piece of mundane equipment: a weapon, a suit of armor, a tool, a
// vehicle or a piece of gear.
//
// The five sub-shapes are pointers rather than a flattened struct because
// "this armor has no Strength minimum" and "this backpack is not armor at
// all" must not look the same. Exactly one is non-nil for any item the SRD
// classifies; all five are nil for the handful it leaves uncategorised.
type Item struct {
	Entry

	// Category is the top-level equipment category: weapon, armor, tools,
	// adventuring-gear, mounts-and-vehicles.
	Category rules.Slug

	// Cost is the list price. Weight is in pounds; zero means the SRD gives
	// none, which is common for trinkets.
	Cost   rules.Coins
	Weight float64

	Weapon  *Weapon
	Armor   *Armor
	Gear    *Gear
	Tool    *Tool
	Vehicle *Vehicle
}

// WeaponCategory distinguishes simple from martial weapons, which decides who
// is proficient with it.
type WeaponCategory uint8

// The weapon categories.
const (
	WeaponCategoryNone WeaponCategory = iota
	SimpleWeapon
	MartialWeapon
)

// WeaponRange distinguishes melee from ranged weapons.
type WeaponRange uint8

// The weapon ranges.
const (
	WeaponRangeNone WeaponRange = iota
	MeleeWeapon
	RangedWeapon
)

// Weapon is the weapon-specific part of an Item.
type Weapon struct {
	Category WeaponCategory
	Range    WeaponRange

	// Damage is nil for the two weapons whose damage the SRD describes in
	// prose instead (the net).
	Damage *rules.Damage

	// TwoHandedDamage is set only for versatile weapons, and is the damage
	// die used when wielded in two hands.
	TwoHandedDamage *rules.Damage

	// NormalRange and LongRange apply to ranged weapons; ThrowNormal and
	// ThrowLong apply to thrown melee weapons. Zero means not applicable.
	NormalRange rules.Feet
	LongRange   rules.Feet
	ThrowNormal rules.Feet
	ThrowLong   rules.Feet

	// Properties are weapon property slugs: finesse, light, versatile.
	Properties []rules.Slug
}

// ArmorCategory groups armor by how much it weighs the wearer down.
type ArmorCategory uint8

// The armor categories. Shield is a category of its own in the SRD data.
const (
	ArmorCategoryNone ArmorCategory = iota
	LightArmor
	MediumArmor
	HeavyArmor
	Shield
)

// Armor is the armor-specific part of an Item.
type Armor struct {
	Category ArmorCategory

	// BaseAC is the armor class before any Dexterity modifier. For a shield
	// it is the bonus added rather than a base.
	BaseAC int

	// AddsDexBonus reports whether the wearer's Dexterity modifier applies.
	// MaxDexBonus caps it, and is nil when uncapped -- the distinction
	// between light armor (uncapped) and medium (capped at +2).
	AddsDexBonus bool
	MaxDexBonus  *int

	// StrengthMinimum is the Strength score below which the armor slows the
	// wearer. Zero means no requirement.
	StrengthMinimum int

	// StealthDisadvantage reports whether wearing it imposes disadvantage on
	// Dexterity (Stealth) checks.
	StealthDisadvantage bool
}

// Gear is the adventuring-gear part of an Item.
type Gear struct {
	// GearCategory is the sub-category: standard-gear, ammunition,
	// holy-symbols and so on.
	GearCategory rules.Slug

	// Quantity is how many the listed price buys, e.g. 20 arrows.
	Quantity int

	// Contents are the items a pack contains, e.g. a Burglar's Pack.
	Contents []ItemStack
}

// Tool is the tool part of an Item.
type Tool struct {
	// ToolCategory is the sub-category: artisan's tools, gaming sets,
	// musical instruments.
	ToolCategory rules.Slug
}

// Vehicle is the mount-or-vehicle part of an Item.
type Vehicle struct {
	// VehicleCategory is the sub-category as the SRD prints it.
	VehicleCategory rules.Slug

	// Speed is how fast it moves; SpeedUnit names the unit, since vehicles
	// are given in miles per hour and mounts in feet per round.
	Speed     float64
	SpeedUnit string

	// Capacity is the carrying capacity as prose, because the SRD states it
	// inconsistently across vehicle types.
	Capacity string
}

// Rarity is a magic item's scarcity, which governs its price and the level at
// which it is appropriate.
type Rarity uint8

// The rarities, least scarce first. RarityVaries covers items whose rarity
// depends on which variant is meant.
const (
	RarityNone Rarity = iota
	RarityCommon
	RarityUncommon
	RarityRare
	RarityVeryRare
	RarityLegendary
	RarityArtifact
	RarityVaries
)

// MagicItem is an enchanted item. Its mechanical effects live in Desc rather
// than in structured fields: the SRD describes them in prose, and inventing a
// schema for effects that are applied by a DM would be a schema nothing can
// fill.
type MagicItem struct {
	Entry

	Category rules.Slug
	Rarity   Rarity

	// Variants lists the concrete items this entry generalises, e.g. the
	// specific +1/+2/+3 weapons under a generic entry. IsVariant marks an
	// entry that is itself one of those.
	Variants  []rules.Slug
	IsVariant bool
}
