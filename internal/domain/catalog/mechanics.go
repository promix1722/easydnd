package catalog

import "github.com/promix1722/easydnd/internal/domain/rules"

// Mechanics is compiled pack policy. A loaded pack always carries one: the
// loader refuses a rules context with no core policy.
type Mechanics struct {
	Core               CoreRules
	Actions            []ActionDefinition
	Resources          []ResourceDefinition
	Rules              []RuleDefinition
	Casting            map[rules.Slug]CastingProfile
	SpellBenefits      []SpellBenefit
	ChoiceRequirements []ChoiceRequirement
}

type CoreRules struct {
	MinScore, MaxScore, AbilityScoreIncrease, BaseArmorClass, SpellSaveBase int
	HitPointFirst, HitPointLater                                            rules.Expression
	HitDiceRecovery                                                         []RecoveryPolicy
	SpellSlotRecovery                                                       []RecoveryPolicy
	MaxLevel                                                                int
	Proficiency                                                             rules.Expression
	AbilityModifier                                                         rules.Expression
	StandardArray                                                           []int
	PointBuyBudget                                                          int
	PointCosts                                                              map[int]int
	MulticlassSlots                                                         map[int]map[int]int
	Senses                                                                  map[rules.Slug]SenseGrant
}

type SenseGrant struct {
	Kind     string
	Distance int
}

type CastingProfile struct {
	List                    rules.Slug    // optional spell-list class, defaulting to the casting class
	Ability                 rules.Ability // optional override, required for subclass casting
	Selection               string        // known, prepared, or spellbook
	PrepareDivisor          int
	BookStart, BookPerLevel int
	ReplaceKnown            bool
	ExpandedSubclass        bool

	Kind                   string // shared or independent
	Numerator, Denominator int
	Rounding               string // floor or ceil, applied per class contribution
	StartsAt               int
	Resource               rules.Slug
}

type Rational struct{ Numerator, Denominator int }

type ResourceRow struct {
	Rational  *Rational
	Boolean   *bool
	From      int
	Capacity  int
	Dice      string
	SlotLevel int
	Text      string
}

type RecoveryPolicy struct {
	When      *rules.Expression
	Trigger   string
	Operation string // all, amount, or budget
	Amount    rules.Expression
	Budget    string // shared allocation budget, e.g. hit-dice
}

type ResourceDefinition struct {
	RequiresSubclass bool
	Entry
	Owner        rules.Ref
	MinimumLevel int
	Kind         string // pool or parameter
	Input        string
	Rows         []ResourceRow
	Capacity     *rules.Expression
	Recovery     []RecoveryPolicy
	SharedKey    rules.Slug
	Combine      string // max or sum for explicitly shared pools
	Group        string
}

type RuleDefinition struct {
	ID           rules.Slug
	Owner        rules.Ref
	MinimumLevel int
	When         *rules.Expression
	Choices      []rules.Choice
	Effects      []Effect
	Manual       bool
}

type Effect struct {
	Op     string
	Target string
	Ref    rules.Ref
	Value  rules.Expression
}

// ActionDefinition describes eligibility and atomic resource costs. Outcomes
// such as attack rolls are resolved at the table and recorded as event facts.
//
// Owner is zero for an action open to everybody -- Dash, Hide -- and Kind is
// the wire name of the part of a turn it takes, as on ActionTag.
type ActionDefinition struct {
	Entry
	Kind         string
	Owner        rules.Ref
	MinimumLevel int
	When         *rules.Expression
	Costs        []ResourceCost
	Manual       bool
}
type ResourceCost struct {
	Resource rules.Slug
	Amount   rules.Expression
}

// SpellBenefit describes a source-specific acquisition or automatic grant.
// Level is a class level for class features and character level for racial traits.
type SpellBenefit struct {
	Ability                  rules.Ability
	ID                       rules.Slug
	Owner                    rules.Ref
	Class                    rules.Slug
	Level, Count, SpellLevel int
	Mode                     string // cantrip, known, prepared, arcanum, mastery, or spellbook
	From                     string // class, any, or book
	Spells                   []rules.Slug
	CountsKnown              bool

	// List is the class whose spell list the picks are drawn from when it is
	// not Class: a Nature cleric's cantrip is a druid's. Class stays "whose
	// level-up poses this".
	List rules.Slug
	// ListFrom names a prompt whose answer is that class instead, for a feat
	// that lets the player choose it: Magic Initiate. Nothing is offered
	// until it is answered.
	ListFrom rules.Slug
	// Schools and Ritual narrow the picks: Fey Touched's divination or
	// enchantment, Ritual Caster's rituals.
	Schools []rules.Slug
	Ritual  bool
}

// ChoiceRequirement keeps conditional starting-equipment offers in pack policy.
type ChoiceRequirement struct {
	Prompt, Pick   rules.Slug
	AnyProficiency []rules.Slug
}
