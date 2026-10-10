package rules

// Ability is one of the six ability scores.
//
// The SRD calls these "ability scores", never "attributes" -- the word
// "attribute" does not appear anywhere in SRD 5.1. The naming is followed here
// so that a reader with the rulebook open finds the same words in the code.
type Ability string

const (
	AbilityNone  Ability = ""
	Strength     Ability = "str"
	Dexterity    Ability = "dex"
	Constitution Ability = "con"
	Intelligence Ability = "int"
	Wisdom       Ability = "wis"
	Charisma     Ability = "cha"
)

func Abilities() []Ability {
	return []Ability{Strength, Dexterity, Constitution, Intelligence, Wisdom, Charisma}
}
func (a Ability) String() string {
	if a == AbilityNone {
		return "none"
	}
	return string(a)
}
func (a Ability) Slug() Slug { return Slug(a) }
func ParseAbility(s string) (Ability, bool) {
	for _, a := range Abilities() {
		if string(a) == s {
			return a, true
		}
	}
	return AbilityNone, false
}

// Modifier returns the ability modifier for a score, which the SRD defines as
// the score minus 10, halved and rounded down.
//
// Rounding down must hold for scores below 10 too: a score of 7 is -2, not -1.
// Go's integer division truncates toward zero, so the negative branch is
// computed explicitly rather than relying on (score-10)/2.
func Modifier(score int) int {
	diff := score - 10
	if diff >= 0 {
		return diff / 2
	}
	return -((-diff + 1) / 2)
}

// Size is a creature's size category.
type Size uint8

// The size categories, smallest first.
const (
	SizeNone Size = iota
	Tiny
	Small
	Medium
	Large
	Huge
	Gargantuan
)

var sizeNames = map[Size]string{
	SizeNone:   "none",
	Tiny:       "tiny",
	Small:      "small",
	Medium:     "medium",
	Large:      "large",
	Huge:       "huge",
	Gargantuan: "gargantuan",
}

// String returns the size's wire name, or "unknown" outside the enumeration.
func (s Size) String() string {
	if name, ok := sizeNames[s]; ok {
		return name
	}
	return "unknown"
}

// ParseSize maps a wire name to its Size. The second result reports whether
// the name was recognised.
func ParseSize(s string) (Size, bool) {
	for size, name := range sizeNames {
		if name == s && size != SizeNone {
			return size, true
		}
	}
	return SizeNone, false
}

// Proficiency is how strongly a character is trained in a skill, saving throw
// or tool.
//
// This is an enumeration rather than a bool because Expertise (rogues, bards)
// doubles the proficiency bonus and Jack of All Trades halves it. A bool
// cannot express either, and discovering that late means rewriting every
// caller.
//
// Note the distinction the SRD draws and DND.md blurred: this is *being
// proficient*. The number added to a d20 roll is the "proficiency bonus", a
// separate value derived from character level.
type Proficiency uint8

// The proficiency levels, in increasing order of training.
const (
	NotProficient Proficiency = iota
	HalfProficient
	Proficient
	Expertise
)

var proficiencyNames = map[Proficiency]string{
	NotProficient:  "none",
	HalfProficient: "half",
	Proficient:     "proficient",
	Expertise:      "expertise",
}

// String returns the level's wire name, or "unknown" outside the enumeration.
func (p Proficiency) String() string {
	if name, ok := proficiencyNames[p]; ok {
		return name
	}
	return "unknown"
}

// ParseProficiency maps a wire name to its Proficiency. The second result
// reports whether the name was recognised.
//
// "none" parses, unlike the zero value of the other enumerations in this
// package: NotProficient is a real answer -- it is what a change event says to
// take a proficiency away again -- rather than the absence of one.
func ParseProficiency(s string) (Proficiency, bool) {
	for level, name := range proficiencyNames {
		if name == s {
			return level, true
		}
	}
	return NotProficient, false
}

// Apply returns the number this proficiency level contributes given the
// character's proficiency bonus.
func (p Proficiency) Apply(proficiencyBonus int) int {
	switch p {
	case HalfProficient:
		return proficiencyBonus / 2
	case Proficient:
		return proficiencyBonus
	case Expertise:
		return proficiencyBonus * 2
	case NotProficient:
		return 0
	default:
		return 0
	}
}

// Feet is a distance in feet: speeds, weapon ranges, spell ranges and the
// reach of a sense such as darkvision.
type Feet int
