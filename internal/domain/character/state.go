package character

import "github.com/promix1722/easydnd/internal/domain/rules"

// State is the current picture of a character: a pure fold of the Log against
// a catalogue.
//
// It is never the source of truth and is never stored. Everything in it is
// either seeded by an init event, chosen by a later event, or derived from the
// two -- which is what DND.md means by "autocalculated", and why recomputing
// it from scratch must always be safe.
type State struct {
	Identity     Identity
	Base         Base
	Abilities    Abilities
	Skills       Skills
	SavingThrows SavingThrows
	Status       Status
	Equipment    Equipment
	Resources    Resources
	Spells       Spellbook
	Actions      []Action

	// Feats are taken in place of an Ability Score Improvement.
	Feats []rules.Slug

	// Traits come from the race and subrace; Features come from classes and
	// subclasses. They are separate because the catalogue keeps them
	// separate: a merged list could not answer "what did my race give me?".
	Traits   []rules.Slug
	Features []rules.Slug

	// Conditions are currently applied statuses: prone, poisoned, and so on.
	Conditions []rules.Slug

	// Proficiencies are the armor, weapon, tool and vehicle proficiencies the
	// character holds. Skill and saving-throw proficiencies are deliberately
	// not repeated here: those have typed homes in Skills and SavingThrows,
	// where a bonus is computed from them. What is left is the sheet's
	// "Other proficiencies & languages" box, which is a list and nothing
	// more.
	Proficiencies []rules.Slug
}

// Identity is who the character is, as opposed to what they can do.
type Identity struct {
	Name       string
	Alignment  rules.Slug
	Race       rules.Slug
	Subrace    rules.Slug
	Background rules.Slug

	// Classes is a slice because a character may multiclass. Order is the
	// order taken; the first is the class the character started as.
	Classes []ClassLevel

	// Experience is the experience-point total, and it is *recorded* rather
	// than acted on. Level comes from the log -- a level event is what makes
	// a character third level -- so nothing here derives a level from this
	// number and crossing a threshold advances nobody. A table playing
	// milestones leaves it at zero and loses nothing; a table counting XP has
	// somewhere to keep the count. Set by a change event, like every other
	// value no rule computes.
	Experience int

	// PersonalityTraits, Ideals, Bonds and Flaws are the player's own text,
	// seeded from the background's suggestions but freely edited. They are
	// never localized: the player wrote them.
	PersonalityTraits []string
	Ideals            []string
	Bonds             []string
	Flaws             []string
}

// Level returns the character level: the sum of levels across all classes,
// which is what drives proficiency bonus and feat progression.
func (i Identity) Level() int {
	total := 0
	for _, c := range i.Classes {
		total += c.Level
	}
	return total
}

// ClassLevel is how many levels the character has taken in one class.
type ClassLevel struct {
	Class    rules.Slug
	Subclass rules.Slug
	Level    int
}

// HitPoints tracks a character's health.
//
// Max is the hit point maximum -- the SRD's term; "max HP" appears nowhere in
// it. Temporary hit points are a separate pool that absorbs damage first and
// never stacks.
type HitPoints struct {
	Current   int
	Max       int
	Temporary int
}

// SpeedKind is a mode of movement.
type SpeedKind uint8

// The movement modes. The SRD calls these speeds, not movement.
const (
	SpeedNone SpeedKind = iota
	Walking
	Flying
	Climbing
	Swimming
	Burrowing
)

// Speed is one movement mode and its rate.
type Speed struct {
	Kind     SpeedKind
	Distance rules.Feet
}

// SenseKind is a special mode of perception.
type SenseKind uint8

// The senses. Darkvision is a sense, not a speed -- a distinction worth
// keeping, since at least one popular sheet exporter files it under movement.
const (
	SenseNone SenseKind = iota
	Darkvision
	Blindsight
	Tremorsense
	Truesight
)

// Sense is one special sense and its range.
type Sense struct {
	Kind     SenseKind
	Distance rules.Feet
}

// DeathSaves tracks the three-and-three tally rolled while dying.
type DeathSaves struct {
	Successes int
	Failures  int
}

// Base is the character's fundamental physical state.
type Base struct {
	HitPoints HitPoints
	Speeds    []Speed
	Senses    []Sense
	Size      rules.Size

	// Languages are the tongues the character speaks, reads and writes.
	Languages []rules.Slug

	// Exhaustion is the current exhaustion level, 0 to 6.
	Exhaustion int

	DeathSaves DeathSaves

	// Inspiration is the binary the DM grants and the player spends.
	Inspiration bool
}

// Abilities holds the six ability scores.
//
// Modifiers are deliberately absent: they are a pure function of the score
// (see rules.Modifier), and storing a derived value is how a sheet ends up
// internally inconsistent.
type Abilities struct {
	// Scores are the base scores as generated -- point buy, standard array
	// or rolled -- with every racial and Ability Score Improvement bonus
	// already folded in by Project. An init event records the base; the
	// final numbers are always derived, so choosing a different race changes
	// the sheet rather than requiring the player to re-enter six numbers.
	Scores map[rules.Ability]int

	// Method is how the scores were generated: "point-buy", "standard-array",
	// "rolled" or "manual". It is recorded because the log cannot otherwise
	// say -- six numbers look the same however they were arrived at -- and
	// going back a step has to re-open the editor the player was using
	// rather than guess.
	Method rules.Slug
}

// Score returns the ability's score, or 10 when unset -- the SRD's default
// for a creature with no stated score.
func (a Abilities) Score(ability rules.Ability) int {
	if score, ok := a.Scores[ability]; ok {
		return score
	}
	return 10
}

// Modifier returns the ability's modifier.
func (a Abilities) Modifier(ability rules.Ability) int {
	return rules.Modifier(a.Score(ability))
}

// SkillState is how trained the character is in one skill, and the resulting
// bonus.
type SkillState struct {
	// Proficiency is whether and how strongly the character is trained.
	Proficiency rules.Proficiency

	// Bonus is the total added to a check: ability modifier plus whatever
	// Proficiency contributes. Derived.
	Bonus int
}

// Skills maps each skill slug to the character's training in it.
//
// Every skill in the compendium is present, not only the trained ones: an
// untrained skill is a real answer -- NotProficient, at the bare ability
// modifier -- rather than a missing key. A sheet is read to find out what to
// roll, and the skills nothing trained are the ones that question is asked
// about most.
type Skills struct {
	BySkill map[rules.Slug]SkillState
}

// SavingThrowState is the character's training in one saving throw.
type SavingThrowState struct {
	Proficient bool

	// Bonus is the total added to the save. Derived.
	Bonus int
}

// SavingThrows maps each ability to the character's save with it. DND.md
// states these are autocalculated, and they are: proficiency comes from the
// class, the bonus from the ability modifier plus the proficiency bonus.
type SavingThrows struct {
	ByAbility map[rules.Ability]SavingThrowState
}

// SpellcastingSummary is the at-a-glance casting block for one class.
type SpellcastingSummary struct {
	Class   rules.Slug
	Ability rules.Ability

	// SaveDC is 8 + proficiency bonus + ability modifier.
	SaveDC int

	// AttackBonus is proficiency bonus + ability modifier. The SRD uses both
	// "spell attack bonus" and "spell attack modifier" for this.
	AttackBonus int
}

// Status is the derived headline block of a character sheet: the numbers a
// player reads off constantly.
type Status struct {
	ArmorClass int
	Initiative int

	// ProficiencyBonus is the number added to anything the character is
	// proficient in, derived from character level.
	ProficiencyBonus int

	PassivePerception int

	// Spellcasting is a slice, not a single value: a multiclassed
	// cleric/wizard has two casting abilities, two save DCs and two attack
	// bonuses, and collapsing them to one would be wrong for exactly the
	// characters most likely to need the summary.
	Spellcasting []SpellcastingSummary
}

// CustomItem is a homebrew or DM-granted item with no catalogue entry.
type CustomItem struct {
	Name        string
	Description string
	Weight      float64
	Cost        rules.Coins
}

// ItemStack is a quantity of one item in a character's possession.
//
// Custom is nil for catalogue items, which are the common case. When it is
// set, Item is zero and the stack describes something the catalogue has never
// heard of -- which SRD 5.1 makes routine, since it publishes one background
// and one feat.
type ItemStack struct {
	Item   rules.Slug
	Count  int
	Custom *CustomItem
}

// Equipment is everything the character carries.
type Equipment struct {
	// Equipped is worn or wielded and contributes to armor class and
	// actions; Backpack is carried; Loot is party treasure not yet divided.
	Equipped []ItemStack
	Backpack []ItemStack
	Loot     []ItemStack

	Purse rules.Purse
}

// Recharge is when a spent resource comes back.
type Recharge uint8

// The recovery points the SRD defines.
const (
	RechargeNone Recharge = iota
	OnShortRest
	OnLongRest
	OnDawn
	AtWill
)

// Pool is a consumable resource: a number of uses, some spent.
type Pool struct {
	// Key names the resource, e.g. "ki-points" or "rage". Zero for the
	// anonymous pools in Resources.SpellSlots.
	Key rules.Slug

	Max      int
	Used     int
	Recharge Recharge

	// Dice is set for pools measured in dice rather than uses, such as Hit
	// Dice or a bard's Bardic Inspiration die.
	Dice *rules.Dice
}

// Available returns how many uses remain.
func (p Pool) Available() int {
	if p.Max-p.Used < 0 {
		return 0
	}
	return p.Max - p.Used
}

// MaxSpellLevel is the highest spell level a character can hold slots for.
const MaxSpellLevel = 9

// Resources is everything the character spends and regains.
//
// DND.md called this "slots". In the SRD a slot is specifically a spell slot;
// there is no umbrella term for ki points, rage uses, sorcery points or
// superiority dice, so this uses the neutral word and keeps SpellSlots as one
// named member.
type Resources struct {
	// SpellSlots is indexed by spell level, 1..MaxSpellLevel. Index 0 is
	// unused so that SpellSlots[3] means third-level slots.
	SpellSlots [MaxSpellLevel + 1]Pool

	// HitDice is one pool per class, since a multiclassed character's dice
	// are different sizes and are spent separately.
	HitDice []Pool

	// Class holds every other class resource, keyed by name. One generic
	// list covers all thirty-two the SRD defines.
	Class []Pool
}

// Spellbook is what the character knows and has ready.
type Spellbook struct {
	// Cantrips are always available and cost no slot.
	Cantrips []rules.Slug

	// Known is the spells the character has learned. Prepared is the subset
	// currently ready to cast. For classes that prepare from a full list
	// (clerics, druids) Known is empty and only Prepared is meaningful; for
	// classes that know a fixed set (sorcerers, warlocks) the two are equal.
	Known    []rules.Slug
	Prepared []rules.Slug

	// Ability is the spellcasting ability. For a multiclassed caster this is
	// the primary one; per-class detail lives in Status.Spellcasting.
	Ability rules.Ability
}

// ActionKind is which part of a turn an action consumes.
//
// These are siblings, not a hierarchy. A bonus action is not a kind of
// action: a turn grants one of each, and spending one does not spend another.
type ActionKind uint8

// The parts of a turn.
const (
	ActionKindNone ActionKind = iota
	MainAction
	BonusAction
	Reaction
	FreeAction
	LegendaryAction
)

// ActionSource is where an action came from.
//
// DND.md calls out the dual provenance directly: an action may be derived
// from equipment or a spell, or stored in the list outright.
type ActionSource uint8

// The provenances an action can have.
const (
	ActionSourceNone ActionSource = iota
	// Derived actions are recomputed on every projection: an equipped
	// longsword produces its attack, a prepared spell produces its casting.
	// Editing one has no effect, because the next projection overwrites it.
	Derived
	// Manual actions are stored in the log directly, for things no rule
	// derives -- a DM's ad-hoc grant, or a reminder the player wants on the
	// sheet.
	Manual
)

// Action is something the character can do on their turn.
type Action struct {
	Source ActionSource

	// Origin names what produced the action: the item, the spell, the
	// feature. Zero for a manual action with no catalogue basis.
	Origin rules.Ref

	Kind ActionKind

	// Name is display text. For a derived action it is copied from the
	// origin entry and is therefore already in the catalogue's locale; for a
	// manual action the player wrote it.
	Name string

	// Range is how far the action reaches. Zero when not applicable.
	Range rules.Feet

	// ToHit is the attack bonus. Nil for actions that make no attack roll,
	// which is not the same as an attack bonus of zero.
	ToHit *int

	// Damage is nil for actions that deal none.
	Damage *rules.Damage

	// Uses points at the pool this action spends, e.g. a Channel Divinity
	// charge. Zero when the action is unlimited.
	Uses rules.Slug

	// Notes is the reminder text a player keeps on the sheet.
	Notes string
}
