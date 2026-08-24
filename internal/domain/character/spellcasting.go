package character

import (
	"strconv"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// casterKind is how a class contributes to spellcasting.
type casterKind uint8

const (
	notACaster casterKind = iota
	// fullCaster gains slots at every level: bard, cleric, druid, sorcerer,
	// wizard.
	fullCaster
	// halfCaster gains them at half rate: paladin and ranger.
	halfCaster
	// pactCaster uses Pact Magic, which is a separate pool that never merges
	// with spell slots: the warlock.
	pactCaster
)

// multiclassSlotReference is the class whose advancement table is used as the
// multiclass spellcaster table.
//
// The PHB prints that table separately, and it is not in the SRD data. It is
// however identical to any full caster's own table, so rather than
// transcribing twenty rows of numbers that nothing would check, the table is
// read from the compendium. The constant is safe precisely because
// TestFullCastersShareOneSlotTable pins that all five full casters agree; if a
// future compendium disagreed, that test fails rather than a wizard's slots
// quietly becoming a bard's.
const multiclassSlotReference rules.Slug = "wizard"

// pactMagicKeyPrefix names the Resources.Class pools that hold Pact Magic.
//
// Warlock slots cannot live in Resources.SpellSlots: that is a single array
// indexed by spell level, and a warlock/wizard has two independent pools at
// overlapping levels, recovering on different rests. Pact Magic is a class
// resource in every sense the model already has a word for, so it is stored
// as one, with the slot level in the key.
const pactMagicKeyPrefix = "pact-magic-level-"

// pactMagicKey names the pool holding pact slots of a given spell level.
func pactMagicKey(level int) rules.Slug {
	return rules.Slug(pactMagicKeyPrefix + strconv.Itoa(level))
}

// kindOfCaster classifies a class from its own advancement table.
//
// The classification is read from the data rather than from a list of slugs,
// because the slug list is the thing most likely to be wrong when a homebrew
// or 2024 compendium arrives. At the class's top level, a full caster holds
// slots at nine spell levels, a half caster at five, and a Pact Magic caster
// at exactly one -- the warlock's, which is the whole reason Pact Magic needs
// separating.
func kindOfCaster(cat *catalog.Catalog, class rules.Slug) casterKind {
	class20, ok := cat.ClassLevel(class, 20)
	if !ok {
		return notACaster
	}
	levels := 0
	for level := 1; level <= MaxSpellLevel; level++ {
		if class20.SpellSlots[level] > 0 {
			levels++
		}
	}
	switch {
	case levels == 0:
		return notACaster
	case levels == 1:
		return pactCaster
	case levels >= 9:
		return fullCaster
	default:
		return halfCaster
	}
}

// casterLevel is the level at which a multiclassed character reads the
// multiclass spellcaster table: full-caster levels count whole, half-caster
// levels count halved and rounded down, and Pact Magic does not count at all.
func casterLevel(cat *catalog.Catalog, classes []ClassLevel) int {
	total := 0
	for _, c := range classes {
		switch kindOfCaster(cat, c.Class) {
		case fullCaster:
			total += c.Level
		case halfCaster:
			total += c.Level / 2
		case pactCaster, notACaster:
		}
	}
	return total
}

// spellSlots returns the character's spell slots, and the Pact Magic pools
// that never merge with them.
//
// A single casting class reads its own table, which is what makes a level-3
// wizard's slots exactly the compendium's wizard row rather than a
// reconstruction of it. Only a genuine multiclass caster goes through the
// multiclass table, because the two disagree: a paladin 2 has slots, but
// contributes only caster level 1.
func spellSlots(cat *catalog.Catalog, classes []ClassLevel) ([MaxSpellLevel + 1]Pool, []Pool) {
	var slots [MaxSpellLevel + 1]Pool
	var pact []Pool

	var casting []ClassLevel
	for _, c := range classes {
		switch kindOfCaster(cat, c.Class) {
		case pactCaster:
			row, ok := cat.ClassLevel(c.Class, c.Level)
			if !ok {
				continue
			}
			for level := 1; level <= MaxSpellLevel; level++ {
				if row.SpellSlots[level] > 0 {
					pact = append(pact, Pool{
						Key:      pactMagicKey(level),
						Max:      row.SpellSlots[level],
						Recharge: OnShortRest,
					})
				}
			}
		case fullCaster, halfCaster:
			casting = append(casting, c)
		case notACaster:
		}
	}

	var row catalog.ClassLevel
	var ok bool
	switch len(casting) {
	case 0:
		return slots, pact
	case 1:
		row, ok = cat.ClassLevel(casting[0].Class, casting[0].Level)
	default:
		row, ok = cat.ClassLevel(multiclassSlotReference, casterLevel(cat, casting))
	}
	if !ok {
		return slots, pact
	}
	for level := 1; level <= MaxSpellLevel; level++ {
		if row.SpellSlots[level] > 0 {
			slots[level] = Pool{Max: row.SpellSlots[level], Recharge: OnLongRest}
		}
	}
	return slots, pact
}

// spellcastingSummaries builds the at-a-glance casting block for each class
// that casts, in the order the classes were taken.
//
// A multiclassed cleric/wizard gets two: two abilities, two save DCs and two
// attack bonuses. Collapsing them to one would be wrong for exactly the
// characters most likely to need the summary.
func spellcastingSummaries(
	cat *catalog.Catalog,
	classes []ClassLevel,
	abilities Abilities,
	proficiencyBonus int,
) []SpellcastingSummary {
	var out []SpellcastingSummary
	for _, c := range classes {
		class, ok := cat.Classes.Get(c.Class)
		if !ok || class.Spellcasting == nil {
			continue
		}
		// Paladins and rangers do not cast until 2nd level; listing a save DC
		// for a level-1 paladin would be a number they cannot use.
		if c.Level < class.Spellcasting.Level {
			continue
		}
		modifier := abilities.Modifier(class.Spellcasting.Ability)
		out = append(out, SpellcastingSummary{
			Class:       c.Class,
			Ability:     class.Spellcasting.Ability,
			SaveDC:      8 + proficiencyBonus + modifier,
			AttackBonus: proficiencyBonus + modifier,
		})
	}
	return out
}
