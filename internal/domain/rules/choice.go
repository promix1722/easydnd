package rules

// The SRD asks the player to choose things constantly: two skills from a
// class list, a martial weapon or a shield, one extra language, a +1 to any
// two abilities. Those prompts nest -- an equipment option can itself be a
// choice between bundles -- so the grammar below is recursive.
//
// It is a transcription of the option grammar in the vendored Zod schemas
// (docs/reference_srd_5.1/data/5e-database-2014-en/schemas/_common.ts), which
// is the authoritative spec for both the shape and the optionality.

// ChoiceKind names what a prompt is choosing, so a UI can render "pick two
// skills" differently from "pick a weapon" without inspecting the options.
type ChoiceKind uint8

// The kinds of prompt the SRD poses.
const (
	ChooseNothing ChoiceKind = iota
	ChooseProficiency
	ChooseLanguage
	ChooseEquipment
	ChooseAbilityBonus
	ChooseSpell
	ChooseTrait
	ChooseFeature
	ChooseIdeal
	ChooseExpertise
	ChooseDamage
	ChooseAction
	ChooseAttack
	ChooseText
	ChoosePersonality
	ChooseBond
	ChooseFlaw
	ChooseAbility

	// The kinds below have no counterpart in the SRD data. They name the
	// prompts a character-building flow must pose that no catalogue entry
	// asks -- "which race?", "which class do you gain a level in?" -- so
	// that those steps can be expressed as Choices rather than as a second,
	// parallel vocabulary the UI would have to learn.
	//
	// They exist for the reason the enum exists at all: so a client can
	// render "pick a race" differently from "pick two skills" without
	// inspecting the option set. Folding them into ChooseNothing would
	// defeat that.
	ChooseRace
	ChooseSubrace
	ChooseBackground
	ChooseClass
	ChooseSubclass
	ChooseLevel
	ChooseAlignment
	ChooseAbilityScores
)

var choiceKindNames = map[ChoiceKind]string{
	ChooseNothing:      "none",
	ChooseProficiency:  "proficiency",
	ChooseLanguage:     "language",
	ChooseEquipment:    "equipment",
	ChooseAbilityBonus: "ability-bonus",
	ChooseSpell:        "spell",
	ChooseTrait:        "trait",
	ChooseFeature:      "feature",
	ChooseIdeal:        "ideal",
	ChooseExpertise:    "expertise",
	ChooseDamage:       "damage",
	ChooseAction:       "action",
	ChooseAttack:       "attack",
	ChooseText:         "text",
	ChoosePersonality:  "personality",
	ChooseBond:         "bond",
	ChooseFlaw:         "flaw",
	ChooseAbility:      "ability",

	ChooseRace:          "race",
	ChooseSubrace:       "subrace",
	ChooseBackground:    "background",
	ChooseClass:         "class",
	ChooseSubclass:      "subclass",
	ChooseLevel:         "level",
	ChooseAlignment:     "alignment",
	ChooseAbilityScores: "ability-scores",
}

// String returns the kind's wire name, or "unknown" outside the enumeration.
func (k ChoiceKind) String() string {
	if name, ok := choiceKindNames[k]; ok {
		return name
	}
	return "unknown"
}

// ParseChoiceKind maps a wire name to its ChoiceKind. The second result
// reports whether the name was recognised.
func ParseChoiceKind(s string) (ChoiceKind, bool) {
	for kind, name := range choiceKindNames {
		if name == s {
			return kind, true
		}
	}
	return ChooseNothing, false
}

// Choice is "choose Choose of these", and nests arbitrarily deep.
type Choice struct {
	// Prompt identifies this prompt within its owning entry, e.g.
	// "fighter/starting-equipment/1". It is load-bearing: a character's log
	// records answers against it, so it must stay stable across catalogue
	// regenerations or stored characters lose their choices.
	Prompt Slug

	Choose int
	Kind   ChoiceKind
	From   OptionSet
}

// OptionSetKind distinguishes how an option set names its members.
type OptionSetKind uint8

// The three ways the SRD data enumerates a set of options.
const (
	// OptionsExplicit lists every option inline.
	OptionsExplicit OptionSetKind = iota
	// OptionsFromEquipmentCategory means "any item in this category", e.g.
	// any martial weapon. The set is resolved against the catalogue.
	OptionsFromEquipmentCategory
	// OptionsFromCollection means "any member of this collection", used for
	// open-ended picks such as any language.
	OptionsFromCollection
)

// OptionSet is the pool a Choice draws from.
type OptionSet struct {
	Kind OptionSetKind

	// Options is populated when Kind is OptionsExplicit.
	Options []Option

	// Category is populated when Kind is OptionsFromEquipmentCategory.
	Category Slug

	// Collection is populated when Kind is OptionsFromCollection, naming
	// which catalogue collection to draw from.
	Collection RefKind
}

// Option is one selectable answer to a Choice.
//
// The interface is sealed -- optionKind is unexported, so only the types below
// can implement it. That makes a type switch over an Option exhaustive by
// construction, which matters because a silently unhandled option type shows
// up as a missing proficiency rather than as an error.
type Option interface {
	optionKind() OptionKind
}

// OptionKind is the discriminator carried by every Option implementation.
type OptionKind uint8

// The option kinds, one per option_type in the SRD schemas.
const (
	OptionKindRef OptionKind = iota
	OptionKindNested
	OptionKindBundle
	OptionKindAbilityBonus
	OptionKindDamage
	OptionKindMoney
	OptionKindSize
	OptionKindText
	OptionKindAction
	OptionKindScoreMinimum
)

// RefOption selects a catalogue entry, optionally more than one of it: the
// two daggers in a rogue's starting kit are Count 2.
type RefOption struct {
	Ref   Ref
	Count int
}

func (RefOption) optionKind() OptionKind { return OptionKindRef }

// NestedOption is an option that is itself a prompt: "a martial weapon and a
// shield, or two martial weapons" makes the second branch a nested choice.
type NestedOption struct {
	Choice Choice
}

func (NestedOption) optionKind() OptionKind { return OptionKindNested }

// BundleOption is several things granted together as one selectable answer.
type BundleOption struct {
	Items []Option
}

func (BundleOption) optionKind() OptionKind { return OptionKindBundle }

// AbilityBonusOption raises one ability score, as the half-elf's "+1 to two
// abilities of your choice" does.
type AbilityBonusOption struct {
	Ability Ability
	Bonus   int
}

func (AbilityBonusOption) optionKind() OptionKind { return OptionKindAbilityBonus }

// DamageOption is a damage expression offered as a choice, used by draconic
// ancestry and similar traits.
type DamageOption struct {
	Damage Damage
	// Notes points at prose in the locale bundle rather than carrying text,
	// because everything in this package is language-neutral.
	Notes Slug
}

func (DamageOption) optionKind() OptionKind { return OptionKindDamage }

// MoneyOption grants coin, used by backgrounds that offer gold instead of
// equipment.
type MoneyOption struct {
	Coins Coins
}

func (MoneyOption) optionKind() OptionKind { return OptionKindMoney }

// SizeOption selects a creature size.
type SizeOption struct {
	Size Size
}

func (SizeOption) optionKind() OptionKind { return OptionKindSize }

// TextOption is a free-standing prose option with no mechanical payload, such
// as a background's suggested ideal. The text lives in the locale bundle under
// Key.
type TextOption struct {
	Key Slug
	// Alignments narrows which alignments an ideal suits; empty means any.
	Alignments []Slug
}

func (TextOption) optionKind() OptionKind { return OptionKindText }

// ActionOption is a granted action, e.g. a breath weapon usable once per
// rest. Name refers to prose in the locale bundle.
type ActionOption struct {
	Key   Slug
	Count int
	// Recharge names when the action's uses come back; empty means at will.
	Recharge Slug
}

func (ActionOption) optionKind() OptionKind { return OptionKindAction }

// ScoreMinimumOption is a prerequisite disguised as an option: the
// multiclassing rules express "Strength 13 or higher" this way.
type ScoreMinimumOption struct {
	Ability Ability
	Minimum int
}

func (ScoreMinimumOption) optionKind() OptionKind { return OptionKindScoreMinimum }
