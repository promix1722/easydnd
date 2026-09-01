package character

import (
	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// grant is everything one step of character building hands the character.
//
// It exists so that Project and Prompts derive "what does taking this level
// give me?" from one place. Two implementations of that question is two
// chances to disagree, and a disagreement shows up as a proficiency the sheet
// shows but the prompt never offered, or the reverse.
type grant struct {
	Proficiencies []rules.Slug
	Choices       []rules.Choice
	Equipment     []catalog.ItemStack
	Purse         rules.Coins
}

func (g *grant) add(other grant) {
	g.Proficiencies = append(g.Proficiencies, other.Proficiencies...)
	g.Choices = append(g.Choices, other.Choices...)
	g.Equipment = append(g.Equipment, other.Equipment...)
}

// classGrant returns what a level in a class hands the character.
//
// The three cases are genuinely different, and conflating them is the
// classic multiclassing bug:
//
//   - The very first class at level 1 grants its full proficiency list, its
//     proficiency choices, and starting equipment.
//   - A class taken later at level 1 grants only the smaller multiclassing
//     proficiency set, and no equipment at all -- you do not get a second
//     adventuring kit for deciding to dabble in wizardry.
//   - Any level after the first grants that level's features and nothing
//     else; the proficiencies came with level 1.
//
// first reports whether this is the character's opening class.
func classGrant(class catalog.Class, level int, first bool) grant {
	switch {
	case level > 1:
		return grant{}
	case first:
		return grant{
			Proficiencies: class.Proficiencies,
			Choices:       class.ProficiencyOptions,
			Equipment:     class.StartingEquipment,
		}
	default:
		return grant{
			Proficiencies: class.MultiClassing.Proficiencies,
			Choices:       class.MultiClassing.ProficiencyOptions,
		}
	}
}

// subclassLevel is the class level at which a class chooses its subclass.
//
// It is derived rather than tabulated: a subclass's advancement rows begin at
// the level it is chosen, so the lowest row is the answer. That yields 3 for
// the thief, 2 for the evocation wizard and 1 for the life cleric, all
// correct, and it keeps working for a homebrew class without a code change.
//
// Subclass.Levels is deliberately not used: it is empty for all twelve SRD
// entries, so trusting it would report that no class ever picks a subclass.
func subclassLevel(cat *catalog.Catalog, class catalog.Class) int {
	best := 0
	for _, slug := range class.Subclasses {
		for _, row := range cat.ClassLevels(slug) {
			if row.Level > 0 && (best == 0 || row.Level < best) {
				best = row.Level
			}
		}
	}
	return best
}

// grantsAbilityScoreImprovement reports whether reaching a level in a class
// grants an Ability Score Improvement.
//
// ClassLevel.AbilityScoreBonuses is cumulative, not per-level: a rogue reads
// 1 at level 4 and still 1 at level 5. So the improvement is granted where
// the count *increases*, which yields [4,8,10,12,16,19] for the rogue and
// [4,6,8,12,14,16,19] for the fighter -- both correct, and both derived
// rather than hardcoded.
func grantsAbilityScoreImprovement(cat *catalog.Catalog, class rules.Slug, level int) bool {
	if level < 1 {
		return false
	}
	row, ok := cat.ClassLevel(class, level)
	if !ok {
		return false
	}
	previous := 0
	if level > 1 {
		if before, ok := cat.ClassLevel(class, level-1); ok {
			previous = before.AbilityScoreBonuses
		}
	}
	return row.AbilityScoreBonuses > previous
}

// featuresThrough returns every feature a class or subclass has granted up to
// and including a level, in level order.
func featuresThrough(cat *catalog.Catalog, owner rules.Slug, level int) []rules.Slug {
	var out []rules.Slug
	for _, row := range cat.ClassLevels(owner) {
		if row.Level > level {
			break
		}
		out = append(out, row.Features...)
	}
	return out
}
