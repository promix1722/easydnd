package character

import (
	"slices"
	"testing"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

func subclassCastingCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	base := spellCatalog(t)
	var levels []catalog.ClassLevel
	for _, class := range base.Classes.Slugs() {
		levels = append(levels, base.ClassLevels(class)...)
	}
	for _, tc := range []struct{ class, subclass rules.Slug }{
		{"fighter", "homebrew/knight"}, {"rogue", "homebrew/trickster"},
	} {
		for level := 3; level <= 8; level++ {
			row := catalog.ClassLevel{Class: tc.class, Subclass: tc.subclass, Level: level, CantripsKnown: 2, SpellsKnown: 3}
			row.SpellSlots[1] = 2
			if level >= 4 {
				row.SpellsKnown = 4
				row.SpellSlots[1] = 3
			}
			if level >= 7 {
				row.SpellsKnown = 5
				row.SpellSlots[1] = 4
				row.SpellSlots[2] = 2
			}
			if level >= 8 {
				row.SpellsKnown = 6
			}
			levels = append(levels, row)
		}
	}
	cat := catalog.New(rules.LocaleEN, levels)
	cat.Classes, cat.Spells, cat.Mechanics = base.Classes, base.Spells, base.Mechanics
	cat.Subclasses = catalog.NewCollection([]catalog.Subclass{
		{Entry: catalog.Entry{Slug: "homebrew/knight"}, Class: "fighter"},
		{Entry: catalog.Entry{Slug: "homebrew/trickster"}, Class: "rogue"},
	})
	for _, owner := range []rules.Slug{"homebrew/knight", "homebrew/trickster"} {
		cat.Mechanics.Casting[owner] = catalog.CastingProfile{Selection: "known", List: "wizard", Ability: rules.Intelligence, Kind: "shared", Numerator: 1, Denominator: 3, Rounding: "floor", StartsAt: 3, ReplaceKnown: true}
	}
	return cat
}

func TestSubclassCastingAcquisitionAndAbility(t *testing.T) {
	cat := subclassCastingCatalog(t)
	for _, tc := range []struct{ class, subclass rules.Slug }{
		{"fighter", "homebrew/knight"}, {"rogue", "homebrew/trickster"},
	} {
		t.Run(tc.subclass.String(), func(t *testing.T) {
			for level := 2; level <= 8; level++ {
				state := spellState(tc.class, level)
				state.Identity.Classes[0].Subclass = tc.subclass
				saved, sources := answerSpellPrompts(t, state, cat)
				if level == 2 {
					if len(sources) != 0 {
						t.Fatal("subclass casts before its start level")
					}
					continue
				}
				if len(sources) != 1 {
					t.Fatalf("sources = %v", sources)
				}
				source := sources[0]
				row, _ := cat.ClassLevel(tc.subclass, level)
				if source.Class != tc.class || source.Source != rules.NewRef(rules.RefSubclass, tc.subclass) || source.Ability != rules.Intelligence {
					t.Fatalf("subclass ownership or ability lost: %+v", source)
				}
				if len(source.Known) != row.SpellsKnown || len(source.Cantrips) != row.CantripsKnown || len(source.Prepared) != row.SpellsKnown {
					t.Fatalf("level %d acquisition = %+v, want %d known, %d cantrips", level, source, row.SpellsKnown, row.CantripsKnown)
				}
				for _, slug := range append(slices.Clone(source.Known), source.Cantrips...) {
					spell, _ := cat.Spells.Get(slug)
					if !slices.Contains(spell.Classes, rules.Slug("wizard")) {
						t.Fatalf("non-Wizard spell %s", slug)
					}
				}
				state.Identity.Classes[0].Subclass = ""
				prompts, sources := spellChoices(state, cat, saved, false)
				if len(sources) != 0 {
					t.Fatalf("base class inherited subclass casting: %v", sources)
				}
				for _, prompt := range prompts {
					if prompt.Purpose != "custom" {
						t.Fatalf("base class offers subclass choices: %v", prompt)
					}
				}
			}
		})
	}
}

func TestSubclassSlotsAndMulticlassContribution(t *testing.T) {
	cat := subclassCastingCatalog(t)
	knight := ClassLevel{Class: "fighter", Subclass: "homebrew/knight", Level: 4}
	slots, _ := spellSlots(cat, []ClassLevel{knight})
	if slots[1].Max != 3 {
		t.Fatalf("single subclass used multiclass table: %v", slots)
	}
	classes := []ClassLevel{knight, {Class: "wizard", Level: 2}}
	if got := casterLevel(cat, classes); got != 3 {
		t.Fatalf("caster level = %d", got)
	}
	slots, _ = spellSlots(cat, classes)
	if slots[1].Max != 4 || slots[2].Max != 2 {
		t.Fatalf("multiclass slots = %v", slots)
	}
	classes = []ClassLevel{{Class: "fighter", Subclass: "homebrew/knight", Level: 2}, {Class: "wizard", Level: 2}}
	if got := casterLevel(cat, classes); got != 2 {
		t.Fatalf("premature subclass contribution = %d", got)
	}
	slots, _ = spellSlots(cat, []ClassLevel{{Class: "rogue", Subclass: "homebrew/knight", Level: 4}})
	if slots[1].Max != 0 {
		t.Fatal("subclass from another class grants slots")
	}
	summaries := spellcastingSummaries(cat, []ClassLevel{knight}, Abilities{Scores: map[rules.Ability]int{rules.Intelligence: 16}}, 2)
	if len(summaries) != 1 || summaries[0].Class != "fighter" || summaries[0].Ability != rules.Intelligence || summaries[0].SaveDC != 13 || summaries[0].AttackBonus != 5 {
		t.Fatalf("casting summary = %+v", summaries)
	}
}

func TestEquipmentExpressionFlags(t *testing.T) {
	cat := spellCatalog(t)
	for _, tc := range []struct {
		items         []ItemStack
		armor, shield int
	}{
		{nil, 0, 0},
		{[]ItemStack{{Item: "leather-armor", Count: 1}}, 1, 0},
		{[]ItemStack{{Item: "shield", Count: 1}}, 0, 1},
		{[]ItemStack{{Item: "chain-mail", Count: 1}, {Item: "shield", Count: 1}}, 1, 1},
		{[]ItemStack{{Item: "shield", Count: 0}, {Item: "dagger", Count: 1}}, 0, 0},
	} {
		state := State{Equipment: Equipment{Equipped: tc.items, Backpack: []ItemStack{{Item: "chain-mail", Count: 1}}}}
		vars := variables(state, cat)
		if vars["equipped:armor"] != tc.armor || vars["equipped:shield"] != tc.shield {
			t.Fatalf("equipment %v: %v", tc.items, vars)
		}
	}
}
