package rules

import (
	"fmt"
	"strings"
)

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
// Every key is derived from what the option *is*, never from where it sits.
// The bundle above is "shortbow+arrow". That is what makes a stored answer
// legible where it is read back -- a log row, a support question, a hand-
// written fixture -- rather than a "#0" that means nothing without the option
// list beside it. It is also what makes an answer *editable*: a person can
// write one and be right.
//
// It used to be positional where an option had no identity, and the argument
// for that was stability across a catalogue regeneration. Deriving the key
// from the contents is strictly better on exactly that ground: reordering a
// class's equipment options no longer renames anybody's stored answer, where
// a positional key changed under them.
//
// What position *did* guarantee is uniqueness within a set, and a derived key
// cannot. That guarantee moved to a check over the whole compendium -- see
// TestEveryOptionSetHasUniqueKeys -- which fails the build rather than
// letting FindOption resolve the wrong option in silence. The index parameter
// is gone with it: with no position in scope, this function cannot quietly
// fall back to one.
//
// Prompts emits these and Project resolves them, so both must call this
// function rather than each deriving a key of its own: two implementations of
// "which option is this?" is two chances to disagree, and the disagreement
// would surface as a proficiency that silently vanishes.
func OptionKey(o Option) Slug {
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
		// What the branch draws from, which is the only thing about a nested
		// choice a player could recognise: "martial-weapons", "skills",
		// "feat". Its prompt id would do as an identifier and did, but a
		// prompt id is a path -- "fighter/starting-equipment/1/0/0" -- and a
		// path in an answer is the same unreadable thing as a position.
		return nestedKey(opt.Choice)
	case BundleOption:
		return bundleKey(opt)
	case SizeOption:
		return Slug(opt.Size.String())
	case ActionOption:
		return opt.Key
	case DamageOption:
		// Draconic ancestry distinguishes its options by the prose naming the
		// dragon where it has it; the damage type names the rest, and is what
		// the SRD's own table is keyed on.
		if opt.Notes != "" {
			return opt.Notes
		}
		return opt.Damage.Type
	case MoneyOption:
		return Slug(fmt.Sprintf("%d-%s", opt.Coins.Amount, opt.Coins.Unit))
	case ScoreMinimumOption:
		return Slug(fmt.Sprintf("%s-%d", opt.Ability.Slug(), opt.Minimum))
	}
	return ""
}

// bundleJoin separates the parts of a bundle's key. It is not a character any
// slug contains, so a composed key can always be read back apart.
const bundleJoin = "+"

// bundleKey names several things granted together by the things themselves:
// a shortbow and twenty arrows is "shortbow+arrow". The count is deliberately
// not in it -- a bundle is identified by what is in it, and "twenty" is a fact
// about the arrows rather than about which option this is.
func bundleKey(opt BundleOption) Slug {
	parts := make([]string, 0, len(opt.Items))
	for _, item := range opt.Items {
		if key := OptionKey(item); key != "" {
			parts = append(parts, key.String())
		}
	}
	return Slug(strings.Join(parts, bundleJoin))
}

// nestedKey names a branch by the pool its answers come from: the equipment
// category, or the collection. "A martial weapon and a shield" is
// "martial-weapons+shield", which is the branch as the rules describe it.
//
// A branch that lists its options inline has no pool to name, and there is
// nothing in the option list that names it either -- the monk's "one artisan's
// tool or one musical instrument" is two inline lists of the same kind, and
// the words that tell them apart are in SRD prose the compendium does not
// carry. Those fall back to the branch's own prompt, which is unique by
// construction (TestPromptIDsAreGloballyUnique) and stable across a
// regeneration. It is the one identifier here that is not a name, and it is
// the honest answer to "what else would you call it?".
func nestedKey(c Choice) Slug {
	if c.From.Category != "" {
		return c.From.Category
	}
	if c.From.Collection != RefNone {
		return Slug(c.From.Collection.String())
	}
	return c.Prompt
}

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
	for _, option := range set.Options {
		out = append(out, OptionKey(option))
	}
	return out
}

// FindOption returns the option in a set with the given key, and whether it
// was found. It is the inverse of OptionKey and the only way Project should
// resolve a stored answer.
func FindOption(set OptionSet, key Slug) (Option, bool) {
	for _, option := range set.Options {
		if OptionKey(option) == key {
			return option, true
		}
	}
	return nil, false
}
