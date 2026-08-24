package character

import (
	"testing"

	"github.com/promix1722/easydnd/internal/domain/rules"
)

func TestKindOfCasterIsReadFromTheData(t *testing.T) {
	cat := LoadCatalog(t)

	tests := []struct {
		class rules.Slug
		want  casterKind
	}{
		{"bard", fullCaster}, {"cleric", fullCaster}, {"druid", fullCaster},
		{"sorcerer", fullCaster}, {"wizard", fullCaster},
		{"paladin", halfCaster}, {"ranger", halfCaster},
		{"warlock", pactCaster},
		{"barbarian", notACaster}, {"fighter", notACaster},
		{"monk", notACaster}, {"rogue", notACaster},
	}
	for _, tt := range tests {
		if got := kindOfCaster(cat, tt.class); got != tt.want {
			t.Errorf("kindOfCaster(%s) = %d, want %d", tt.class, got, tt.want)
		}
	}
}

// multiclassSlotReference reads one full caster's table as the multiclass
// table. That is only sound while every full caster's table is the same one.
func TestFullCastersShareOneSlotTable(t *testing.T) {
	cat := LoadCatalog(t)

	full := []rules.Slug{"bard", "cleric", "druid", "sorcerer", "wizard"}
	for level := 1; level <= 20; level++ {
		want, ok := cat.ClassLevel(multiclassSlotReference, level)
		if !ok {
			t.Fatalf("%s has no level %d row", multiclassSlotReference, level)
		}
		for _, class := range full {
			got, ok := cat.ClassLevel(class, level)
			if !ok {
				t.Fatalf("%s has no level %d row", class, level)
			}
			if got.SpellSlots != want.SpellSlots {
				t.Errorf("level %d: %s slots = %v, %s = %v",
					level, class, got.SpellSlots, multiclassSlotReference, want.SpellSlots)
			}
		}
	}
}

func TestCasterLevelHalvesHalfCasters(t *testing.T) {
	cat := LoadCatalog(t)

	tests := []struct {
		name    string
		classes []ClassLevel
		want    int
	}{
		{"single full caster", []ClassLevel{{Class: "wizard", Level: 5}}, 5},
		{"single half caster", []ClassLevel{{Class: "paladin", Level: 5}}, 2},
		{"half caster at 1 contributes nothing", []ClassLevel{{Class: "ranger", Level: 1}}, 0},
		{"cleric 1 / wizard 1", []ClassLevel{{Class: "cleric", Level: 1}, {Class: "wizard", Level: 1}}, 2},
		{"paladin 6 / sorcerer 4", []ClassLevel{{Class: "paladin", Level: 6}, {Class: "sorcerer", Level: 4}}, 7},
		{"warlock does not count", []ClassLevel{{Class: "warlock", Level: 5}, {Class: "wizard", Level: 2}}, 2},
		{"no casters", []ClassLevel{{Class: "fighter", Level: 11}}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := casterLevel(cat, tt.classes); got != tt.want {
				t.Errorf("casterLevel() = %d, want %d", got, tt.want)
			}
		})
	}
}

// A single-class caster must read its own table, not the multiclass one. The
// two disagree for half casters, and a paladin whose slots were computed from
// caster level 1 would be missing them entirely at level 2.
func TestSingleClassCasterReadsItsOwnTable(t *testing.T) {
	cat := LoadCatalog(t)

	slots, pact := spellSlots(cat, []ClassLevel{{Class: "paladin", Level: 2}})
	if len(pact) != 0 {
		t.Errorf("paladin has pact pools: %v", pact)
	}
	if slots[1].Max != 2 {
		t.Errorf("paladin 2 first-level slots = %d, want 2", slots[1].Max)
	}

	slots, _ = spellSlots(cat, []ClassLevel{{Class: "wizard", Level: 3}})
	if slots[1].Max != 4 || slots[2].Max != 2 || slots[3].Max != 0 {
		t.Errorf("wizard 3 slots = %d/%d/%d, want 4/2/0",
			slots[1].Max, slots[2].Max, slots[3].Max)
	}
}

// The case the multiclass table exists for: two level-1 casters have the
// slots of a level-2 caster, not of two level-1 casters.
func TestMulticlassCastersShareOneSlotPool(t *testing.T) {
	cat := LoadCatalog(t)

	slots, _ := spellSlots(cat, []ClassLevel{{Class: "cleric", Level: 1}, {Class: "wizard", Level: 1}})
	if slots[1].Max != 3 {
		t.Errorf("cleric 1 / wizard 1 first-level slots = %d, want 3", slots[1].Max)
	}
}

// Pact Magic recovers on a short rest and is a different pool at the same
// spell level, so merging it into SpellSlots would hand a warlock/wizard
// slots they do not have.
func TestPactMagicNeverMergesWithSpellSlots(t *testing.T) {
	cat := LoadCatalog(t)

	slots, pact := spellSlots(cat, []ClassLevel{{Class: "warlock", Level: 3}})
	for level := 1; level <= MaxSpellLevel; level++ {
		if slots[level].Max != 0 {
			t.Errorf("warlock has spell slots at level %d", level)
		}
	}
	if len(pact) != 1 {
		t.Fatalf("warlock 3 pact pools = %d, want 1", len(pact))
	}
	if pact[0].Key != pactMagicKey(2) {
		t.Errorf("pact pool key = %q, want %q", pact[0].Key, pactMagicKey(2))
	}
	if pact[0].Max != 2 {
		t.Errorf("warlock 3 pact slots = %d, want 2", pact[0].Max)
	}
	if pact[0].Recharge != OnShortRest {
		t.Errorf("pact recharge = %d, want OnShortRest", pact[0].Recharge)
	}

	// And in a multiclass the two pools coexist.
	slots, pact = spellSlots(cat, []ClassLevel{{Class: "warlock", Level: 2}, {Class: "wizard", Level: 2}})
	if slots[1].Max != 3 {
		t.Errorf("warlock 2 / wizard 2 first-level spell slots = %d, want 3", slots[1].Max)
	}
	if len(pact) != 1 || pact[0].Max != 2 {
		t.Errorf("pact pools = %v, want one pool of 2", pact)
	}
}

func TestSpellcastingSummariesAreOnePerCastingClass(t *testing.T) {
	cat := LoadCatalog(t)

	abilities := Abilities{Scores: map[rules.Ability]int{
		rules.Wisdom: 16, rules.Intelligence: 14, rules.Charisma: 10,
	}}
	classes := []ClassLevel{
		{Class: "cleric", Level: 1},
		{Class: "wizard", Level: 1},
		{Class: "fighter", Level: 1},
	}
	got := spellcastingSummaries(cat, classes, abilities, 2)
	if len(got) != 2 {
		t.Fatalf("summaries = %d, want 2 (fighter does not cast)", len(got))
	}
	if got[0].Class != "cleric" || got[0].Ability != rules.Wisdom {
		t.Errorf("first summary = %+v, want cleric/wis", got[0])
	}
	// 8 + proficiency 2 + wis modifier 3
	if got[0].SaveDC != 13 || got[0].AttackBonus != 5 {
		t.Errorf("cleric DC/attack = %d/%d, want 13/5", got[0].SaveDC, got[0].AttackBonus)
	}
	if got[1].Class != "wizard" || got[1].SaveDC != 12 {
		t.Errorf("second summary = %+v, want wizard with DC 12", got[1])
	}
}

// A paladin does not cast until 2nd level; a level-1 paladin listing a spell
// save DC would be advertising a number they cannot use.
func TestHalfCasterHasNoSummaryBeforeItsCastingLevel(t *testing.T) {
	cat := LoadCatalog(t)

	abilities := Abilities{Scores: map[rules.Ability]int{rules.Charisma: 16}}
	if got := spellcastingSummaries(cat, []ClassLevel{{Class: "paladin", Level: 1}}, abilities, 2); len(got) != 0 {
		t.Errorf("paladin 1 summaries = %v, want none", got)
	}
	if got := spellcastingSummaries(cat, []ClassLevel{{Class: "paladin", Level: 2}}, abilities, 2); len(got) != 1 {
		t.Errorf("paladin 2 summaries = %v, want one", got)
	}
}
