package catalog

import "github.com/promix1722/easydnd/internal/domain/rules"

// The SRD prints casting time, range and duration as English phrases -- "1
// action", "90 feet", "Up to 1 minute" -- but they are mechanics, not prose.
// Storing the phrase would mean a Russian sheet showing "90 feet", and would
// make "which spells can I cast as a bonus action?" a substring search.
//
// So they are parsed into the structures below, and the phrase is rendered
// back per locale at display time. The value space is small enough to
// enumerate completely: nine distinct casting times, seventeen ranges,
// nineteen durations across all 319 SRD spells.

// TimeUnit is a unit of game time.
type TimeUnit uint8

// The units of game time, shortest first.
const (
	TimeUnitNone TimeUnit = iota
	Round
	Minute
	Hour
	Day
)

// CastingTimeKind is how a spell's casting time is expressed.
type CastingTimeKind uint8

// The ways a spell's casting time is stated.
const (
	CastingTimeNone CastingTimeKind = iota
	// CastAsAction, CastAsBonusAction and CastAsReaction consume part of the
	// caster's turn. These are three siblings, not a hierarchy: a bonus
	// action is not a kind of action.
	CastAsAction
	CastAsBonusAction
	CastAsReaction
	// CastOverTime is a ritual-length casting: 1 minute, 8 hours.
	CastOverTime
)

// CastingTime is how long a spell takes to cast.
type CastingTime struct {
	Kind CastingTimeKind

	// Amount and Unit apply when Kind is CastOverTime. For the three action
	// kinds Amount is 1 and Unit is unset.
	Amount int
	Unit   TimeUnit
}

// SpellRangeKind is how a spell's range is expressed.
type SpellRangeKind uint8

// The ways a spell's range is stated.
const (
	SpellRangeNone SpellRangeKind = iota
	// RangeSelf targets the caster.
	RangeSelf
	// RangeTouch requires physical contact.
	RangeTouch
	// RangeDistance is a measured range; see SpellRange.Distance.
	RangeDistance
	// RangeSight reaches anything the caster can see.
	RangeSight
	// RangeUnlimited has no bound.
	RangeUnlimited
	// RangeSpecial is described in the spell's own text.
	RangeSpecial
)

// SpellRange is how far a spell reaches.
type SpellRange struct {
	Kind SpellRangeKind

	// Distance applies when Kind is RangeDistance. Miles are converted to
	// feet so that comparisons need no unit handling.
	Distance rules.Feet
}

// DurationKind is how a spell's duration is expressed.
type DurationKind uint8

// The ways a spell's duration is stated.
const (
	DurationNone DurationKind = iota
	// DurationInstantaneous resolves and ends immediately.
	DurationInstantaneous
	// DurationTimed lasts a stated span; see Duration.Amount and Unit.
	DurationTimed
	// DurationUntilDispelled lasts until ended by dispel magic or the spell's
	// own terms.
	DurationUntilDispelled
	// DurationSpecial is described in the spell's own text.
	DurationSpecial
)

// Duration is how long a spell's effect lasts.
type Duration struct {
	Kind DurationKind

	// Amount and Unit apply when Kind is DurationTimed.
	Amount int
	Unit   TimeUnit

	// UpTo marks a duration the caster may end early -- the SRD's "Up to 1
	// minute" as opposed to a flat "1 minute". In practice it accompanies
	// concentration, but the two are stated separately and so are stored
	// separately.
	UpTo bool
}

// Components are the verbal, somatic and material requirements of a spell.
type Components struct {
	Verbal   bool
	Somatic  bool
	Material bool
}

// AreaKind is the shape of a spell's area of effect.
type AreaKind uint8

// The area shapes the SRD uses.
const (
	AreaNone AreaKind = iota
	AreaCone
	AreaCube
	AreaCylinder
	AreaLine
	AreaSphere
)

// AreaOfEffect is the region a spell covers.
type AreaOfEffect struct {
	Kind AreaKind
	Size rules.Feet
}

// SpellAttackType distinguishes a spell that makes an attack roll from one
// that forces a saving throw. A spell may do neither.
type SpellAttackType uint8

// The kinds of spell attack.
const (
	SpellAttackNone SpellAttackType = iota
	MeleeSpellAttack
	RangedSpellAttack
)

// SaveEffect is what a successful saving throw achieves.
type SaveEffect uint8

// The outcomes of a successful save.
const (
	SaveEffectNone SaveEffect = iota
	// SaveNegates means a success avoids the effect entirely.
	SaveNegates
	// SaveHalvesDamage means a success takes half damage.
	SaveHalvesDamage
	// SaveOther means the spell's text describes the result.
	SaveOther
)

// SavingThrow is the save a spell forces.
type SavingThrow struct {
	Ability rules.Ability
	Success SaveEffect
}

// SpellScaling is a value that grows with the slot used or, for cantrips,
// with the caster's character level.
//
// The two maps are mutually exclusive in practice: a levelled spell scales by
// slot, a cantrip by character level. Both are nil for a spell that does not
// scale, and that nil is meaningful -- reading a missing scaling table as
// zero damage is exactly the silent error this shape exists to prevent.
type SpellScaling struct {
	// AtSlotLevel is keyed by the spell slot level used, 1..MaxSpellLevel.
	AtSlotLevel map[int]rules.Dice

	// AtCharacterLevel is keyed by the caster's character level, 1..20.
	AtCharacterLevel map[int]rules.Dice
}

// SpellDamage is a spell's damage, typed and scaled.
type SpellDamage struct {
	// Type is a damage-type slug. It is empty for the few spells whose
	// damage type varies with the caster's choice.
	Type rules.Slug

	Scaling SpellScaling
}

// Spell is a spell or cantrip.
type Spell struct {
	Entry

	// Source names the document the spell comes from. Every spell today is
	// "srd-5.1"; the field exists so that content from anywhere else -- a
	// later SRD, a homebrew import -- can sit in the same collection without
	// a model change.
	Source rules.Slug

	// Level is 0 for a cantrip, 1..MaxSpellLevel otherwise.
	Level int

	// School is a magic-school slug.
	School rules.Slug

	CastingTime CastingTime
	Range       SpellRange
	Duration    Duration
	Components  Components

	// Material is the material component's description, in the catalogue's
	// locale. Empty unless Components.Material is set.
	Material string

	// Ritual reports whether the spell can be cast as a ritual, taking ten
	// minutes longer and consuming no slot.
	Ritual bool

	// Concentration reports whether maintaining the spell occupies the
	// caster's concentration, of which only one may be held at a time.
	Concentration bool

	AttackType SpellAttackType

	// Save is nil for spells that force none.
	Save *SavingThrow

	// Damage and Heal are nil for spells that do neither.
	Damage *SpellDamage
	Heal   *SpellScaling

	// Area is nil for single-target spells.
	Area *AreaOfEffect

	// Classes and Subclasses list who has this spell on their list.
	Classes    []rules.Slug
	Subclasses []rules.Slug

	// HigherLevel is the "At Higher Levels" prose, in the catalogue's locale.
	HigherLevel []string
}

// IsCantrip reports whether the spell is a cantrip, which costs no slot and
// scales with character level rather than slot level.
func (s Spell) IsCantrip() bool { return s.Level == 0 }
