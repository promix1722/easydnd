package character

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	file "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

func spellCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	registry, err := file.NewRegistry([]string{filepath.Join("..", "..", "..", "data", "srd_5.1")}, []file.Dependency{{ID: "srd-2014", Version: "1.0.0"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	cat, err := registry.Load(context.Background(), rules.DefaultLocale)
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func spellState(class rules.Slug, level int) State {
	return State{Identity: Identity{Classes: []ClassLevel{{Class: class, Level: level}}}, Abilities: Abilities{Scores: map[rules.Ability]int{rules.Intelligence: 16, rules.Wisdom: 16, rules.Charisma: 16}}}
}

func answerSpellPrompts(t *testing.T, state State, cat *catalog.Catalog) (answers, []SpellSource) {
	t.Helper()
	a := answers{}
	for round := 0; round < 150; round++ {
		prompts, sources := spellChoices(state, cat, a, false)
		var next *Prompt
		for i := range prompts {
			if !prompts[i].Optional {
				next = &prompts[i]
				break
			}
		}
		if next == nil {
			return a, sources
		}
		var picks []rules.Slug
		for _, key := range rules.OptionKeys(next.Choice.From) {
			if !slices.Contains(next.Held, key) {
				picks = append(picks, key)
			}
			if len(picks) == next.Choice.Choose {
				break
			}
		}
		if len(picks) != next.Choice.Choose {
			t.Fatalf("unanswerable %s: need %d, have %d", next.Choice.Prompt, next.Choice.Choose, len(picks))
		}
		a[next.Choice.Prompt] = picks
	}
	t.Fatal("spell selection failed to settle")
	return nil, nil
}

func TestSpellAcquisitionEveryClassAndLevel(t *testing.T) {
	cat := spellCatalog(t)
	for class, profile := range cat.Mechanics.Casting {
		for level := 1; level <= 20; level++ {
			state := spellState(class, level)
			_, sources := answerSpellPrompts(t, state, cat)
			if level < profile.StartsAt {
				if len(sources) != 0 {
					t.Fatal("premature casting")
				}
				continue
			}
			source := sources[0]
			row, _ := cat.ClassLevel(class, level)
			if len(source.Cantrips) != row.CantripsKnown {
				t.Fatalf("%s %d cantrips: %d want %d", class, level, len(source.Cantrips), row.CantripsKnown)
			}
			if profile.Selection == "known" {
				// Plain class state excludes features; the full projection test covers Secrets.
				if len(source.Known) != row.SpellsKnown {
					t.Fatalf("%s %d known: %d want %d", class, level, len(source.Known), row.SpellsKnown)
				}
			}
			if profile.Selection == "spellbook" && len(source.Spellbook) != 6+2*(level-1) {
				t.Fatalf("wizard %d book: %d", level, len(source.Spellbook))
			}
			if profile.PrepareDivisor > 0 && source.PreparationLimit != level/profile.PrepareDivisor+3 {
				t.Fatalf("%s %d preparation: %d", class, level, source.PreparationLimit)
			}
		}
	}
}

func TestSpellPreparationAndMulticlassEligibility(t *testing.T) {
	cat := spellCatalog(t)
	state := spellState("wizard", 1)
	state.Identity.Classes = append(state.Identity.Classes, ClassLevel{Class: "cleric", Level: 4})
	a, sources := answerSpellPrompts(t, state, cat)
	if len(sources[0].Spellbook) != 6 {
		t.Fatal("wizard level must be independent")
	}
	for _, id := range sources[0].Spellbook {
		s, _ := cat.Spells.Get(id)
		if s.Level != 1 {
			t.Fatal("multiclass slots unlocked wizard spells")
		}
	}
	a[spellPrompt("wizard", "prepared", 1)] = sources[0].Spellbook[:2]
	prompts, sources := spellChoices(state, cat, a, false)
	if len(sources[0].Prepared) != 2 {
		t.Fatal("partial preparation did not settle")
	}
	for _, p := range prompts {
		if p.Choice.Prompt == spellPrompt("wizard", "prepared", 1) {
			t.Fatal("partial preparation reopens")
		}
	}
	a[spellPrompt("wizard", "prepared", 1)] = []rules.Slug{"fireball"}
	_, sources = spellChoices(state, cat, a, false)
	if len(sources[0].Prepared) != 0 {
		t.Fatal("prepared a spell outside the spellbook")
	}
}

func TestSpellBenefitsRespectTheirSource(t *testing.T) {
	cat := spellCatalog(t)
	state := spellState("druid", 9)
	state.Identity.Classes[0].Subclass = "land"
	state.Features = []rules.Slug{"bonus-cantrip", "circle-of-the-land-arctic"}
	_, sources := answerSpellPrompts(t, state, cat)
	if len(sources[0].Cantrips) != 4 || len(sources[0].Prepared) != 8 {
		t.Fatalf("land benefits: %+v", sources[0])
	}
	if slices.Contains(sources[0].Prepared, "lightning-bolt") {
		t.Fatal("arctic gained mountain spells")
	}
	state = spellState("cleric", 7)
	state.Identity.Classes[0].Subclass = "life"
	_, sources = answerSpellPrompts(t, state, cat)
	if len(sources[0].Prepared) != 8 || !slices.Contains(sources[0].Prepared, "guardian-of-faith") {
		t.Fatal("incomplete life domain spells")
	}
	state = spellState("warlock", 1)
	state.Identity.Classes[0].Subclass = "fiend"
	prompts, sources := spellChoices(state, cat, answers{}, false)
	if len(sources[0].Known) != 0 {
		t.Fatal("expanded list was granted automatically")
	}
	found := false
	for _, p := range prompts {
		if p.Purpose == "known" {
			found = slices.Contains(rules.OptionKeys(p.Choice.From), "burning-hands")
		}
	}
	if !found {
		t.Fatal("expanded spell missing from eligible list")
	}
}

func TestLegacySpellReplacementStillProjects(t *testing.T) {
	cat := spellCatalog(t)
	state := spellState("sorcerer", 2)
	a, sources := answerSpellPrompts(t, state, cat)
	old := sources[0].Known[0]
	a[spellPrompt("sorcerer", "forget", 2)] = []rules.Slug{old}

	// Legacy logs remain readable, but no replacement workflow is offered.
	for _, spell := range cat.Spells.All() {
		if spell.Level == 1 && slices.Contains(spell.Classes, rules.Slug("sorcerer")) && !slices.Contains(sources[0].Known, spell.Slug) {
			a[spellPrompt("sorcerer", "replace", 2)] = []rules.Slug{spell.Slug}
			break
		}
	}
	prompts, _ := spellChoices(state, cat, a, false)
	for _, p := range prompts {
		if p.Purpose == "replace" || p.Purpose == "forget" {
			t.Fatal("replacement workflow still offered")
		}
	}

	_, sources = spellChoices(state, cat, a, false)
	if len(sources[0].Known) != 3 || slices.Contains(sources[0].Known, old) {
		t.Fatalf("replacement: %+v", sources[0])
	}
}

func TestConditionalClericEquipment(t *testing.T) {
	cat := spellCatalog(t)
	state := spellState("cleric", 1)
	builder := promptBuilder{cat: cat, state: state, answers: answers{}}
	class, _ := cat.Classes.Get("cleric")
	if !slices.Contains(builder.blockedIn(class.StartingEquipmentOptions[0]), rules.Slug("warhammer")) {
		t.Fatal("warhammer offered without proficiency")
	}
	builder.state.Proficiencies = []rules.Slug{"warhammers", "heavy-armor"}
	if len(builder.blockedIn(class.StartingEquipmentOptions[0])) != 0 || len(builder.blockedIn(class.StartingEquipmentOptions[1])) != 0 {
		t.Fatal("proficient equipment unavailable")
	}
}

func TestHigherLevelSpellBenefitsAndProjectedSources(t *testing.T) {
	cat := spellCatalog(t)
	for _, tc := range []struct {
		class, subclass rules.Slug
		extra           []rules.Slug
	}{
		{"wizard", "evocation", nil}, {"bard", "lore", nil}, {"warlock", "fiend", []rules.Slug{"pact-of-the-tome"}},
	} {
		var log Log
		err := log.Append(Event{Type: EventInit}, Event{Type: EventClass, Ref: rules.NewRef(rules.RefClass, tc.class), Level: 20}, Event{Type: EventSubclass, Ref: rules.NewRef(rules.RefSubclass, tc.subclass), Level: 1})
		if err != nil {
			t.Fatal(err)
		}
		state, err := Project(log, cat)
		if err != nil {
			t.Fatal(err)
		}
		state.Features = append(state.Features, tc.extra...)
		a, sources := answerSpellPrompts(t, state, cat)
		source := sources[0]
		switch tc.class {
		case "wizard":
			if len(source.Spellbook) != 44 || len(source.Mastery) != 2 || len(source.Prepared) != 2 {
				t.Fatalf("wizard benefits: %+v", source)
			}
		case "bard":
			if len(source.Known)+len(source.Cantrips) != 28 {
				t.Fatalf("bard plus Lore and Secrets: %+v", source)
			}
		case "warlock":
			if len(source.Known) != 15 || len(source.Cantrips) != 7 || len(source.Arcanum) != 4 {
				t.Fatalf("warlock benefits: %+v", source)
			}
		}
		for id, picks := range a {
			if err := log.Append(Event{Type: EventChange, Choices: []Answer{{Prompt: id, Picks: picks}}}); err != nil {
				t.Fatal(err)
			}
		}
		projected, err := Project(log, cat)
		if err != nil {
			t.Fatal(err)
		}
		if len(projected.Spells.Sources) == 0 || len(projected.Spells.Known) == 0 {
			t.Fatal("acquired spells absent from sheet")
		}
	}
}
