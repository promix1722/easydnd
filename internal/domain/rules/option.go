package rules

import "strconv"

// OptionKey returns the slug a character's stored Answer uses to name one
// option of a Choice.
//
// It exists because Option is a sealed interface of ten implementations and
// only two of them -- RefOption and TextOption -- carry anything slug-shaped.
// The rogue's own starting kit proves the gap: rogue/starting-equipment/1 is
// "choose 1" between a BundleOption of a shortbow and twenty arrows and a
// RefOption naming a shortsword, and nothing in the bundle names the bundle.
// Without a total function over Option, half the SRD's prompts would be
// unanswerable.
//
// The result must be stable across a catalogue regeneration, for the same
// reason Choice.Prompt must be: a character's answers point at these strings,
// and an answer that no longer resolves is a choice the character silently
// loses. Where an option has no identity of its own the key is positional,
// which is safe for the same reason the prompt ids already are -- prompt ids
// such as "rogue/starting-equipment/1" are themselves positional, and
// make data/srd/check fails on any reordering of the generated data.
//
// Prompts emits these and Project resolves them, so both must call this
// function rather than each deriving a key of its own: two implementations of
// "which option is this?" is two chances to disagree, and the disagreement
// would surface as a proficiency that silently vanishes.
func OptionKey(o Option, index int) Slug {
	switch opt := o.(type) {
	case RefOption:
		return opt.Ref.Slug
	case TextOption:
		return opt.Key
	case AbilityBonusOption:
		// The half-elf's "+1 to two abilities of your choice" is answered
		// with ability slugs -- "dex", "con" -- which is what a player means
		// and what the sheet already stores.
		return opt.Ability.Slug()
	case NestedOption:
		// A nested choice already has a stable, unique id: its own prompt.
		// Answering the outer prompt with it is what lets the inner prompt
		// appear, which is how the rogue's Expertise works.
		return opt.Choice.Prompt
	case SizeOption:
		return Slug(opt.Size.String())
	case ActionOption:
		if opt.Key != "" {
			return opt.Key
		}
	case DamageOption:
		// Draconic ancestry distinguishes its options by the prose naming the
		// dragon, not by the damage type -- two ancestries share a type.
		if opt.Notes != "" {
			return opt.Notes
		}
	}
	return positionalKey(index)
}

// positionalKey names an option that has no identity of its own: a bundle, a
// pouch of gold, a multiclassing score minimum. The "#" prefix cannot collide
// with a real slug, which is lower-kebab.
func positionalKey(index int) Slug { return Slug("#" + strconv.Itoa(index)) }

// OptionKeys returns the keys of every option in a set, in order.
//
// It is nil for a set that draws from a collection or an equipment category
// rather than listing its members: those are resolved against the catalogue,
// where the entry's own slug is the key.
func OptionKeys(set OptionSet) []Slug {
	if set.Kind != OptionsExplicit {
		return nil
	}
	out := make([]Slug, 0, len(set.Options))
	for i, option := range set.Options {
		out = append(out, OptionKey(option, i))
	}
	return out
}

// FindOption returns the option in a set with the given key, and whether it
// was found. It is the inverse of OptionKey and the only way Project should
// resolve a stored answer.
func FindOption(set OptionSet, key Slug) (Option, bool) {
	for i, option := range set.Options {
		if OptionKey(option, i) == key {
			return option, true
		}
	}
	return nil, false
}
