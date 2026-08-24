package character

import "github.com/promix1722/easydnd/internal/domain/rules"

// traitSenses maps a racial trait to the sense it grants.
//
// This table exists because the SRD data does not carry it. A Trait has a
// slug, prose and an optional TraitSpecific, and none of them holds a range:
// "You have superior vision in dark and dim conditions. You can see in dim
// light within 60 feet of you as if it were bright light" is a paragraph, and
// the 60 lives inside the sentence. Every other number on the sheet is read
// from the compendium; these two are read from here, and that is a gap in the
// upstream data rather than a modelling choice.
//
// Keeping it small and explicit is deliberate. The alternative -- parsing
// "60 feet" out of the description -- would be locale-dependent, which is
// precisely the mistake docs/dnd.md describes when it says rule strings are
// mechanics and belong structured.
var traitSenses = map[rules.Slug]Sense{
	"darkvision":          {Kind: Darkvision, Distance: 60},
	"superior-darkvision": {Kind: Darkvision, Distance: 120},
}

// sensesFor returns the senses a set of traits grants, strongest first for
// each kind, with duplicates collapsed.
//
// A dwarf who is also somehow granted superior darkvision has one darkvision,
// at the better range -- senses do not stack, they supersede.
func sensesFor(traits []rules.Slug) []Sense {
	best := make(map[SenseKind]rules.Feet)
	var order []SenseKind
	for _, trait := range traits {
		sense, ok := traitSenses[trait]
		if !ok {
			continue
		}
		if got, seen := best[sense.Kind]; !seen {
			order = append(order, sense.Kind)
			best[sense.Kind] = sense.Distance
		} else if sense.Distance > got {
			best[sense.Kind] = sense.Distance
		}
	}
	out := make([]Sense, 0, len(order))
	for _, kind := range order {
		out = append(out, Sense{Kind: kind, Distance: best[kind]})
	}
	return out
}
