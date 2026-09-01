package character

import (
	"slices"

	"github.com/promix1722/easydnd/internal/domain/rules"
)

// oneList collapses a choice between branches into the single list those
// branches add up to, where they do add up to one.
//
// The rogue's first Expertise is the case, and the only one in SRD 5.1. The
// book says "two of your skill proficiencies, or one of your skill
// proficiencies and your proficiency with thieves' tools", and the compendium
// transcribes that faithfully: choose 1 of two branches, one of which is a
// bundle. Asked that way it is two questions -- pick a branch, then pick
// inside it -- to express one: choose 2 from your skills plus thieves' tools.
// The two are the same set of legal answers, because thieves' tools cannot be
// picked twice.
//
// That the flat form is the right one is not a guess. The *same feature* at
// sixth level, and both of the bard's, are already flat in the data -- choose
// 2 over the eighteen skills and thieves' tools, no branches. The nesting at
// first level is an inconsistency in the transcription rather than a rule, and
// this is where it is reconciled: the compendium keeps saying what the book
// says, and the application decides how to ask it.
//
// # Both sides call this, and that is not optional
//
// Prompts asks with it and Project reads with it. Projection resolves an
// answer by walking the catalogue's own shape (answers.chosen), so a flattened
// question and a nested reader would store skill-stealth against a prompt
// whose options are branches, FindOption would fail to resolve it, and the
// Expertise would silently not apply.
//
// # The guard
//
// Everything that is not exactly this shape comes back untouched, which is
// eighteen starting-equipment prompts and the monk's tools. The fighter's "a
// martial weapon and a shield, or two martial weapons" is the one to keep in
// mind: its pool is an equipment *category* rather than a list of references,
// and flattening it would make "a shield and a shield" a legal answer.
func oneList(c *rules.Choice) *rules.Choice {
	if c == nil || c.Choose != 1 || c.From.Kind != rules.OptionsExplicit {
		return c
	}
	if len(c.From.Options) < 2 {
		return c
	}

	var pool []rules.Option
	var extras []rules.Option
	total := 0
	for i, option := range c.From.Options {
		branch, fixed, ok := branchOf(option)
		if !ok {
			return c
		}
		// Every branch draws from the same pool. Two branches offering
		// different things are a real choice between them, not one list.
		if i == 0 {
			pool = branch.From.Options
		} else if !samePool(pool, branch.From.Options) {
			return c
		}
		// And every branch yields as many entries as the last, counting what
		// it grants outright. A branch worth fewer picks than its sibling is
		// a decision about how much you get, which a flat list cannot state.
		count := branch.Choose + len(fixed)
		if i == 0 {
			total = count
		} else if count != total {
			return c
		}
		extras = append(extras, fixed...)
	}
	if total < 1 {
		return c
	}

	options := slices.Clone(pool)
	for _, extra := range extras {
		if !slices.ContainsFunc(options, func(o rules.Option) bool {
			return rules.OptionKey(o) == rules.OptionKey(extra)
		}) {
			options = append(options, extra)
		}
	}

	// The prompt id is the outer one, because that is the question being
	// asked and what an answer must point at. The branch ids stop existing.
	flat := *c
	flat.Choose = total
	flat.From = rules.OptionSet{Kind: rules.OptionsExplicit, Options: options}
	return &flat
}

// branchOf splits one option of a branch selector into the choice it poses and
// the entries it grants outright, and reports whether it is a branch at all.
//
// A bare nested choice grants nothing; a bundle grants its references and must
// contain exactly one choice. Anything else -- a plain reference standing
// beside branches, a nested choice over a category rather than a list -- is
// not a shape this can add up.
func branchOf(o rules.Option) (choice rules.Choice, fixed []rules.Option, ok bool) {
	switch opt := o.(type) {
	case rules.NestedOption:
		if !listsRefs(opt.Choice) {
			return rules.Choice{}, nil, false
		}
		return opt.Choice, nil, true
	case rules.BundleOption:
		var nested *rules.Choice
		for _, item := range opt.Items {
			switch item := item.(type) {
			case rules.NestedOption:
				if nested != nil || !listsRefs(item.Choice) {
					return rules.Choice{}, nil, false
				}
				inner := item.Choice
				nested = &inner
			case rules.RefOption:
				fixed = append(fixed, item)
			default:
				return rules.Choice{}, nil, false
			}
		}
		if nested == nil {
			return rules.Choice{}, nil, false
		}
		return *nested, fixed, true
	}
	return rules.Choice{}, nil, false
}

// listsRefs reports whether a choice draws from an inline list of catalogue
// references, which is the only pool two branches can be compared over.
func listsRefs(c rules.Choice) bool {
	if c.From.Kind != rules.OptionsExplicit || len(c.From.Options) == 0 || c.Choose < 1 {
		return false
	}
	for _, option := range c.From.Options {
		if _, ok := option.(rules.RefOption); !ok {
			return false
		}
	}
	return true
}

func samePool(a, b []rules.Option) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if rules.OptionKey(a[i]) != rules.OptionKey(b[i]) {
			return false
		}
	}
	return true
}
