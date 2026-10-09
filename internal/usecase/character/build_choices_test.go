package character_test

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	file "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

func TestEquipmentCategoriesValidateAndProject(t *testing.T) {
	b := build(t).add("fighter", domain.Event{Type: domain.EventClass, Ref: ref(rules.RefClass, "fighter"), Level: 1})
	event := domain.Event{Type: domain.EventClass, Ref: ref(rules.RefClass, "fighter"), Choices: []domain.Answer{
		answer("fighter/starting-equipment/main-hand", "longsword"),
		answer("fighter/starting-equipment/off-hand", "longsword"),
	}}
	invalid := event
	invalid.Choices = append([]domain.Answer{}, event.Choices...)
	invalid.Choices[1] = answer("fighter/starting-equipment/off-hand", "plate-armor")
	if _, err := b.s.Apply(context.Background(), testOwner, b.id, rules.DefaultLocale, b.seq, invalid); err == nil {
		t.Fatal("accepted items outside martial-weapons")
	}
	b.add("two identical martial weapons", event)
	cat, _ := b.s.Catalog(context.Background(), rules.DefaultLocale)
	state, err := domain.Project(b.log(), cat)
	if err != nil {
		t.Fatal(err)
	}
	// Both hands asked for a weapon, so both longswords are wielded.
	count := 0
	for _, stack := range state.Equipment.Equipped {
		if stack.Item == "longsword" {
			count += stack.Count
		}
	}
	if count != 2 || len(state.Equipment.Backpack) != 0 {
		t.Fatalf("longswords wielded = %d, backpack = %v", count, state.Equipment.Backpack)
	}
	resolved, err := domain.ResolvedSelections(b.log(), cat)
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved["fighter/starting-equipment/off-hand"].Options) != 1 {
		t.Fatalf("saved off-hand answer: %+v", resolved["fighter/starting-equipment/off-hand"])
	}
}

func TestFighterArmorAndBowAreSeparateChoices(t *testing.T) {
	b := build(t).add("fighter", domain.Event{Type: domain.EventClass, Ref: ref(rules.RefClass, "fighter"), Level: 1})
	event := domain.Event{Type: domain.EventClass, Ref: ref(rules.RefClass, "fighter"), Choices: []domain.Answer{
		answer("fighter/starting-equipment/body", "leather-armor+longbow+arrow"),
	}}
	if _, err := b.s.Apply(context.Background(), testOwner, b.id, rules.DefaultLocale, b.seq, event); err == nil {
		t.Fatal("accepted armor and bow as one body choice")
	}
	event.Choices = []domain.Answer{answer("fighter/starting-equipment/body", "leather-armor")}
	b.add("armor only", event)
	cat, err := b.s.Catalog(context.Background(), rules.DefaultLocale)
	if err != nil {
		t.Fatal(err)
	}
	state, err := domain.Project(b.log(), cat)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(state.Equipment.Equipped, []domain.ItemStack{{Item: "leather-armor", Count: 1}}) || len(state.Equipment.Backpack) != 0 {
		t.Fatalf("armor choice granted extra gear: %+v", state.Equipment)
	}
	event.Choices = []domain.Answer{answer("fighter/starting-equipment/backup", "longbow+arrow")}
	b.add("bow separately", event)
	state, err = domain.Project(b.log(), cat)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []domain.ItemStack{{Item: "longbow", Count: 1}, {Item: "arrow", Count: 20}} {
		if !slices.Contains(state.Equipment.Backpack, want) {
			t.Errorf("backpack = %+v, missing %+v", state.Equipment.Backpack, want)
		}
	}
}

func TestFightingStyleCannotBeChosenAgain(t *testing.T) {
	b := build(t).add("fighter", domain.Event{Type: domain.EventClass, Ref: ref(rules.RefClass, "fighter"), Level: 1}).
		add("style", domain.Event{Type: domain.EventLevel, Ref: ref(rules.RefClass, "fighter"), Level: 1, Choices: []domain.Answer{answer("fighter-fighting-style/subfeature/0", "fighter-fighting-style-defense")}}).
		add("desired level", domain.Event{Type: domain.EventChange, Changes: []domain.Change{{Path: "identity.desiredLevel", Op: domain.OpSet, Value: domain.IntValue(10)}}}).
		add("champion", domain.Event{Type: domain.EventSubclass, Ref: ref(rules.RefSubclass, "champion"), Level: 3})
	event := domain.Event{Type: domain.EventLevel, Ref: ref(rules.RefClass, "fighter"), Level: 10, Choices: []domain.Answer{answer("additional-fighting-style/subfeature/0", "fighter-fighting-style-defense")}}
	if _, err := b.s.Apply(context.Background(), testOwner, b.id, rules.DefaultLocale, b.seq, event); err == nil {
		t.Fatal("accepted duplicate fighting style")
	}
	event.Choices = []domain.Answer{answer("additional-fighting-style/subfeature/0", "fighter-fighting-style-archery")}
	b.add("different style", event)
}

func TestDragonbornAncestryGrantsBreathWithoutAnotherChoice(t *testing.T) {
	for color, damageType := range map[string]rules.Slug{
		"black": "acid", "blue": "lightning", "brass": "fire", "bronze": "lightning",
		"copper": "acid", "gold": "fire", "green": "poison", "red": "fire",
		"silver": "cold", "white": "cold",
	} {
		t.Run(color, func(t *testing.T) {
			ctx := context.Background()
			b := build(t).add("dragonborn", domain.Event{Type: domain.EventRace, Ref: ref(rules.RefRace, "dragonborn")})
			prompts, err := b.s.Prompts(ctx, testOwner, b.id, rules.DefaultLocale)
			if err != nil {
				t.Fatal(err)
			}
			if !hasOpenPrompt(prompts, "draconic-ancestry/subtrait/0") {
				t.Fatal("ancestry must remain a player choice")
			}
			ancestry := rules.Slug("draconic-ancestry-" + color)
			b.add("ancestry", domain.Event{Type: domain.EventRace, Ref: ref(rules.RefRace, "dragonborn"), Choices: []domain.Answer{
				answer("draconic-ancestry/subtrait/0", ancestry),
			}})
			prompts, err = b.s.Prompts(ctx, testOwner, b.id, rules.DefaultLocale)
			if err != nil {
				t.Fatal(err)
			}
			for _, prompt := range prompts {
				if prompt.Group == domain.GroupRace {
					t.Errorf("unexpected racial choice after ancestry: %s", prompt.Choice.Prompt)
				}
			}
			sheet, err := b.s.Sheet(ctx, testOwner, b.id, rules.DefaultLocale)
			if err != nil {
				t.Fatal(err)
			}
			for _, trait := range []rules.Slug{ancestry, "breath-weapon", "damage-resistance"} {
				if !slices.Contains(sheet.Traits, trait) {
					t.Errorf("missing automatically granted trait %s", trait)
				}
			}
			cat, err := b.s.Catalog(ctx, rules.DefaultLocale)
			if err != nil {
				t.Fatal(err)
			}
			trait, ok := cat.Traits.Get(ancestry)
			if !ok || trait.Specific == nil || !slices.Contains(trait.Specific.DamageResistance, damageType) {
				t.Fatalf("missing ancestry resistance to %s", damageType)
			}
			breath := trait.Specific.BreathWeapon
			if breath == nil || len(breath.From.Options) != 1 {
				t.Fatal("missing automatic breath weapon data")
			}
			damage, ok := breath.From.Options[0].(rules.DamageOption)
			if !ok || damage.Damage.Type != damageType {
				t.Fatalf("breath damage = %#v, want %s", damage, damageType)
			}
		})
	}
}

func TestSpellChoicesThroughCharacterService(t *testing.T) {
	r, err := file.NewRegistry([]string{filepath.Join("..", "..", "..", "data", "pack", "srd-5.1")}, []file.Dependency{{ID: "srd-2014", Version: "^2.0.0"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	s := charuc.NewService(memory.NewCharacterRepository(), memory.NewFolderRepository(), r, nil, slog.New(slog.DiscardHandler))
	c := mustCreateScored(t, s)
	b := (&builder{t: t, s: s, id: c.ID, seq: c.Log.LastSeq()}).add("wizard", domain.Event{Type: domain.EventClass, Ref: ref(rules.RefClass, "wizard"), Level: 1})
	bad := domain.Event{Type: domain.EventLevel, Ref: ref(rules.RefClass, "wizard"), Level: 1, Choices: []domain.Answer{answer("wizard/spell/cantrip/1", "fire-bolt", "fire-bolt", "cure-wounds")}}
	if _, err = s.Apply(context.Background(), testOwner, b.id, rules.DefaultLocale, b.seq, bad); err == nil {
		t.Fatal("accepted duplicate/off-list spells")
	}
	b.add("cantrips", domain.Event{Type: domain.EventLevel, Ref: ref(rules.RefClass, "wizard"), Level: 1, Choices: []domain.Answer{answer("wizard/spell/cantrip/1", "fire-bolt", "mage-hand", "light")}})
	b.add("spellbook", domain.Event{Type: domain.EventLevel, Ref: ref(rules.RefClass, "wizard"), Level: 1, Choices: []domain.Answer{answer("wizard/spell/spellbook/1", "alarm", "burning-hands", "charm-person", "color-spray", "comprehend-languages", "detect-magic")}})
	b.add("prepare", domain.Event{Type: domain.EventLevel, Ref: ref(rules.RefClass, "wizard"), Level: 1, Choices: []domain.Answer{answer("wizard/spell/prepared/1", "alarm")}})
	cat, err := s.CharacterCatalog(context.Background(), testOwner, b.id, rules.DefaultLocale)
	if err != nil {
		t.Fatal(err)
	}
	state, err := domain.Project(b.log(), cat)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Spells.Cantrips) != 3 || len(state.Spells.Known) != 6 || len(state.Spells.Prepared) != 1 {
		t.Fatalf("spell projection: %+v", state.Spells)
	}
	saved := b.log()
	var cantripSeq int
	for _, event := range saved.Events {
		for _, choice := range event.Choices {
			if choice.Prompt == "wizard/spell/cantrip/1" {
				cantripSeq = event.Seq
			}
		}
	}
	editable, err := s.PromptsBefore(context.Background(), testOwner, b.id, rules.DefaultLocale, cantripSeq)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, prompt := range editable {
		if prompt.Choice.Prompt == "wizard/spell/cantrip/1" {
			found = true
			if slices.Contains(prompt.Held, rules.Slug("fire-bolt")) {
				t.Fatal("original pick is held against itself")
			}
		}
	}
	if !found {
		t.Fatal("saved cantrip question was not reopened for editing")
	}
	if !reflect.DeepEqual(saved, b.log()) {
		t.Fatal("reading edit choices mutated the saved log")
	}
	b.add("level two", domain.Event{Type: domain.EventChange, Changes: []domain.Change{{Path: "identity.desiredLevel", Op: domain.OpSet, Value: domain.IntValue(2)}}})
	prompts, err := s.Prompts(context.Background(), testOwner, b.id, rules.DefaultLocale)
	if err != nil {
		t.Fatal(err)
	}
	if !hasOpenPrompt(prompts, "wizard/spell/spellbook/2") {
		t.Fatal("level-up did not ask for two book spells")
	}
}

func TestSpellDraftRevisesSeveralAnswersAtomically(t *testing.T) {
	ctx := context.Background()
	r, err := file.NewRegistry([]string{filepath.Join("..", "..", "..", "data", "pack", "srd-5.1")}, []file.Dependency{{ID: "srd-2014", Version: "^2.0.0"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	s := charuc.NewService(memory.NewCharacterRepository(), memory.NewFolderRepository(), r, nil, slog.New(slog.DiscardHandler))
	c := mustCreateScored(t, s)
	b := (&builder{t: t, s: s, id: c.ID, seq: c.Log.LastSeq()}).add("warlock", domain.Event{Type: domain.EventClass, Ref: ref(rules.RefClass, "warlock"), Level: 1}).
		add("cantrips", domain.Event{Type: domain.EventLevel, Ref: ref(rules.RefClass, "warlock"), Level: 1, Choices: []domain.Answer{answer("warlock/spell/cantrip/1", "eldritch-blast", "chill-touch")}}).
		add("spells", domain.Event{Type: domain.EventLevel, Ref: ref(rules.RefClass, "warlock"), Level: 1, Choices: []domain.Answer{answer("warlock/spell/known/1", "charm-person", "hellish-rebuke")}}).
		add("level two", domain.Event{Type: domain.EventChange, Changes: []domain.Change{{Path: "identity.desiredLevel", Op: domain.OpSet, Value: domain.IntValue(2)}}})
	before := b.log()
	edits := map[int]*domain.Event{}
	for _, event := range before.Events {
		for _, choice := range event.Choices {
			switch choice.Prompt {
			case "warlock/spell/cantrip/1":
				event.Choices = []domain.Answer{answer(choice.Prompt, "eldritch-blast", "mage-hand")}
				edits[event.Seq] = &event
			case "warlock/spell/known/1":
				event.Choices = []domain.Answer{answer(choice.Prompt, "charm-person", "unseen-servant")}
				edits[event.Seq] = &event
			}
		}
	}
	added := []domain.Event{{Type: domain.EventLevel, Ref: ref(rules.RefClass, "warlock"), Level: 2, Choices: []domain.Answer{answer("warlock/spell/known/2", "comprehend-languages")}}}
	preview, err := b.s.ReviseBatch(ctx, testOwner, b.id, rules.DefaultLocale, b.seq, edits, added, false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, b.log()) {
		t.Fatal("preview changed the saved log")
	}
	invalid := slices.Clone(added)
	invalid[0].Choices = []domain.Answer{answer("warlock/spell/known/2", "not-a-spell")}
	if _, err := b.s.ReviseBatch(ctx, testOwner, b.id, rules.DefaultLocale, b.seq, edits, invalid, true); err == nil {
		t.Fatal("accepted invalid final acquisition")
	}
	if !reflect.DeepEqual(before, b.log()) {
		t.Fatal("failed batch partially saved edits")
	}
	result, err := b.s.ReviseBatch(ctx, testOwner, b.id, rules.DefaultLocale, b.seq, edits, added, true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(preview.Sheet, result.Sheet) || len(result.Dropped) != 0 {
		t.Fatal("preview and commit disagree")
	}
	if !slices.Contains(result.Sheet.Spells.Cantrips, rules.Slug("mage-hand")) || !slices.Contains(result.Sheet.Spells.Known, rules.Slug("comprehend-languages")) {
		t.Fatal("batch did not apply every answer")
	}
	stale := charuc.WithRevision(ctx, result.Revision-1)
	if _, err := b.s.ReviseBatch(stale, testOwner, b.id, rules.DefaultLocale, result.Seq, edits, nil, true); err == nil {
		t.Fatal("accepted a stale draft")
	}
	after := b.log()
	for seq := range edits {
		if before.Events[seq-1].ID != after.Events[seq-1].ID {
			t.Fatal("replacement changed event identity")
		}
	}
}

func TestCustomSpellsPersistWithoutConsumingNormalAllowances(t *testing.T) {
	b := spellBuilderForTest(t).add("sorcerer", domain.Event{Type: domain.EventClass, Ref: ref(rules.RefClass, "sorcerer"), Level: 1})
	// A level-one sorcerer cannot normally learn Wish or a cleric spell.
	invalid := domain.Event{Type: domain.EventLevel, Ref: ref(rules.RefClass, "sorcerer"), Level: 1, Choices: []domain.Answer{answer("sorcerer/spell/known/1", "wish", "cure-wounds")}}
	if _, err := b.s.Apply(context.Background(), testOwner, b.id, rules.DefaultLocale, b.seq, invalid); err == nil {
		t.Fatal("normal selection accepted custom spells")
	}
	event := domain.Event{Type: domain.EventChange, Choices: []domain.Answer{answer("custom/spell/known", "wish", "cure-wounds", "fireball")}}
	b.add("custom spells", event)
	cat, _ := b.s.Catalog(context.Background(), rules.DefaultLocale)
	state, err := domain.Project(b.log(), cat)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(state.Spells.Known, rules.Slug("wish")) || !slices.Contains(state.Spells.Prepared, rules.Slug("wish")) {
		t.Fatal("custom spell did not project")
	}
	source := state.Spells.Sources[len(state.Spells.Sources)-1]
	if source.Source.String() != "rule:custom-spells" || len(source.Known) != 3 {
		t.Fatalf("custom source lost: %+v", source)
	}
	prompts, err := b.s.Prompts(context.Background(), testOwner, b.id, rules.DefaultLocale)
	if err != nil {
		t.Fatal(err)
	}
	normalOpen := false
	for _, p := range prompts {
		if p.Choice.Prompt == "sorcerer/spell/known/1" {
			normalOpen = true
		}
	}
	if !normalOpen {
		t.Fatal("custom picks silently consumed normal allowance")
	}
	for _, picks := range [][]rules.Slug{{"wish", "wish"}, {"missing-spell"}, {"light"}} {
		replacement := domain.Event{Type: domain.EventChange, Choices: []domain.Answer{{Prompt: "custom/spell/known", Picks: picks}}}
		if _, _, err := charuc.Revise(b.log(), cat, b.seq, &replacement); err == nil {
			t.Fatalf("accepted invalid custom picks %v", picks)
		}
	}
	// Explicit empty replacement removes every custom spell without affecting rules.
	empty := domain.Event{Type: domain.EventChange, Choices: []domain.Answer{answer("custom/spell/known")}}
	log, _, err := charuc.Revise(b.log(), cat, b.seq, &empty)
	if err != nil {
		t.Fatal(err)
	}
	state, err = domain.Project(log, cat)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(state.Spells.Known, rules.Slug("wish")) {
		t.Fatal("removed custom spell remains")
	}
}

func TestSorcererUsesCurrentLevelTotalWithoutReplacementSteps(t *testing.T) {
	b := spellBuilderForTest(t).add("sorcerer", domain.Event{Type: domain.EventClass, Ref: ref(rules.RefClass, "sorcerer"), Level: 1}).
		add("level five", domain.Event{Type: domain.EventChange, Changes: []domain.Change{{Path: "identity.desiredLevel", Op: domain.OpSet, Value: domain.IntValue(5)}}})
	for index, picks := range [][]rules.Slug{{"fireball", "lightning-bolt"}, {"burning-hands"}, {"invisibility"}, {"mirror-image"}, {"shield"}} {
		level := index + 1
		b.add("learn", domain.Event{Type: domain.EventLevel, Ref: ref(rules.RefClass, "sorcerer"), Level: level, Choices: []domain.Answer{{Prompt: rules.Slug(fmt.Sprintf("sorcerer/spell/known/%d", level)), Picks: picks}}})
	}
	prompts, err := b.s.Prompts(context.Background(), testOwner, b.id, rules.DefaultLocale)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range prompts {
		if p.Purpose == "replace" || p.Purpose == "forget" {
			t.Fatal("replacement workflow still offered")
		}
	}
	cat, _ := b.s.Catalog(context.Background(), rules.DefaultLocale)
	state, err := domain.Project(b.log(), cat)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Spells.Known) != 6 || !slices.Contains(state.Spells.Known, rules.Slug("fireball")) || !slices.Contains(state.Spells.Known, rules.Slug("lightning-bolt")) {
		t.Fatalf("wrong current-level selection: %v", state.Spells.Known)
	}
	allowances, err := domain.SpellRules(b.log(), cat)
	if err != nil {
		t.Fatal(err)
	}
	known := 0
	for _, r := range allowances {
		if r.Class == "sorcerer" && r.Purpose == "known" {
			known++
			if r.Count != 6 || r.MaxLevel != 3 || r.ClassLevel != 5 || len(r.ListClasses) != 1 || r.ListClasses[0] != "sorcerer" {
				t.Fatalf("incorrect total: %+v", r)
			}
		}
	}
	if known != 1 {
		t.Fatalf("expected one compact allowance, got %d", known)
	}
}

func TestEarlierSpellAnswersEditAtCurrentClassLevel(t *testing.T) {
	b := spellBuilderForTest(t).add("sorcerer", domain.Event{Type: domain.EventClass, Ref: ref(rules.RefClass, "sorcerer"), Level: 1}).
		add("known", domain.Event{Type: domain.EventLevel, Ref: ref(rules.RefClass, "sorcerer"), Level: 1, Choices: []domain.Answer{answer("sorcerer/spell/known/1", "shield", "magic-missile")}})
	seq := b.seq
	b.add("level five", domain.Event{Type: domain.EventChange, Changes: []domain.Change{{Path: "identity.desiredLevel", Op: domain.OpSet, Value: domain.IntValue(5)}}})
	prompts, err := b.s.PromptsBefore(context.Background(), testOwner, b.id, rules.DefaultLocale, seq)
	if err != nil {
		t.Fatal(err)
	}
	offered := false
	for _, p := range prompts {
		if p.Choice.Prompt == "sorcerer/spell/known/1" {
			offered = slices.Contains(rules.OptionKeys(p.Choice.From), rules.Slug("fireball"))
		}
	}
	if !offered {
		t.Fatal("saved choice still restricted to its historical level")
	}
	cat, _ := b.s.Catalog(context.Background(), rules.DefaultLocale)
	edit := domain.Event{Type: domain.EventLevel, Ref: ref(rules.RefClass, "sorcerer"), Level: 1, Choices: []domain.Answer{answer("sorcerer/spell/known/1", "fireball", "lightning-bolt")}}
	revised, lost, err := charuc.Revise(b.log(), cat, seq, &edit)
	if err != nil {
		t.Fatal(err)
	}
	if len(lost) > 0 {
		t.Fatalf("unrelated events dropped: %+v", lost)
	}
	state, err := domain.Project(revised, cat)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Spells.Known) != 2 || !slices.Contains(state.Spells.Known, rules.Slug("fireball")) {
		t.Fatalf("edit lost: %v", state.Spells.Known)
	}
	edit.Choices = []domain.Answer{answer("sorcerer/spell/known/1", "wish", "fireball")}
	if _, _, err := charuc.Revise(b.log(), cat, seq, &edit); err == nil {
		t.Fatal("current level bounds bypassed")
	}
}

func spellBuilderForTest(t *testing.T) *builder {
	t.Helper()
	registry, err := file.NewRegistry([]string{filepath.Join("..", "..", "..", "data", "pack", "srd-5.1")}, []file.Dependency{{ID: "srd-2014", Version: "^2.0.0"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	service := charuc.NewService(memory.NewCharacterRepository(), memory.NewFolderRepository(), registry, nil, slog.New(slog.DiscardHandler))
	character := mustCreateScored(t, service)
	return &builder{t: t, s: service, id: character.ID, seq: character.Log.LastSeq()}
}

func TestEarlierSpellEditIncludesCurrentSubclassList(t *testing.T) {
	b := spellBuilderForTest(t).add("warlock", domain.Event{Type: domain.EventClass, Ref: ref(rules.RefClass, "warlock"), Level: 1}).
		add("known", domain.Event{Type: domain.EventLevel, Ref: ref(rules.RefClass, "warlock"), Level: 1, Choices: []domain.Answer{answer("warlock/spell/known/1", "charm-person", "hellish-rebuke")}})
	seq := b.seq
	b.add("fiend", domain.Event{Type: domain.EventSubclass, Ref: ref(rules.RefSubclass, "fiend"), Level: 1}).
		add("level five", domain.Event{Type: domain.EventChange, Changes: []domain.Change{{Path: "identity.desiredLevel", Op: domain.OpSet, Value: domain.IntValue(5)}}})
	prompts, err := b.s.PromptsBefore(context.Background(), testOwner, b.id, rules.DefaultLocale, seq)
	if err != nil {
		t.Fatal(err)
	}
	offered := false
	for _, p := range prompts {
		if p.Choice.Prompt == "warlock/spell/known/1" {
			offered = slices.Contains(rules.OptionKeys(p.Choice.From), rules.Slug("fireball"))
		}
	}
	if !offered {
		t.Fatal("current subclass spell missing from earlier edit")
	}
	cat, _ := b.s.Catalog(context.Background(), rules.DefaultLocale)
	edit := domain.Event{Type: domain.EventLevel, Ref: ref(rules.RefClass, "warlock"), Level: 1, Choices: []domain.Answer{answer("warlock/spell/known/1", "fireball", "hellish-rebuke")}}
	revised, _, err := charuc.Revise(b.log(), cat, seq, &edit)
	if err != nil {
		t.Fatal(err)
	}
	state, err := domain.Project(revised, cat)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(state.Spells.Known, rules.Slug("fireball")) {
		t.Fatal("subclass spell lost on replay")
	}
}

func TestHighestSpellLevelLimitAndCustomOverride(t *testing.T) {
	b := spellBuilderForTest(t).add("sorcerer", domain.Event{Type: domain.EventClass, Ref: ref(rules.RefClass, "sorcerer"), Level: 1}).
		add("level five", domain.Event{Type: domain.EventChange, Changes: []domain.Change{{Path: "identity.desiredLevel", Op: domain.OpSet, Value: domain.IntValue(5)}}}).
		add("two third level", domain.Event{Type: domain.EventLevel, Ref: ref(rules.RefClass, "sorcerer"), Level: 1, Choices: []domain.Answer{answer("sorcerer/spell/known/1", "fireball", "lightning-bolt")}})
	cat, _ := b.s.Catalog(context.Background(), rules.DefaultLocale)
	limits, err := domain.SpellRules(b.log(), cat)
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range limits {
		if rule.Class == "sorcerer" && rule.Purpose == "known" {
			if rule.Count != 6 || rule.MaxLevelCount == nil || *rule.MaxLevelCount != 2 {
				t.Fatalf("bad limit: %+v", rule)
			}
		}
	}
	// Aggregate validation must reject a third level-3 choice even in another prompt.
	illegal := domain.Event{Type: domain.EventLevel, Ref: ref(rules.RefClass, "sorcerer"), Level: 2, Choices: []domain.Answer{answer("sorcerer/spell/known/2", "haste")}}
	if _, err := b.s.ReviseBatch(context.Background(), testOwner, b.id, rules.DefaultLocale, b.seq, nil, []domain.Event{illegal}, false); err == nil {
		t.Fatal("third highest-level spell accepted")
	}
	b.add("lower level", domain.Event{Type: domain.EventLevel, Ref: ref(rules.RefClass, "sorcerer"), Level: 2, Choices: []domain.Answer{answer("sorcerer/spell/known/2", "shield")}})
	if _, _, err := charuc.Revise(b.log(), cat, b.seq, &illegal); err == nil {
		t.Fatal("edit bypassed highest-level cap")
	}
	// A persistent extra allowance is separate from the normal class formula.
	b.add("extra", domain.Event{Type: domain.EventChange, Changes: []domain.Change{{Path: "spellLimits.known", Op: domain.OpIncrement, Value: domain.IntValue(1)}}}).
		add("custom", domain.Event{Type: domain.EventChange, Choices: []domain.Answer{answer("custom/spell/known", "haste", "wish")}})
	limits, err = domain.SpellRules(b.log(), cat)
	if err != nil {
		t.Fatal(err)
	}
	extra := false
	for _, rule := range limits {
		if rule.Purpose == "custom-limit" {
			extra = rule.Count == 1 && rule.MaxLevel == 9
		}
		if rule.Class == "sorcerer" && rule.Purpose == "known" && (rule.Count != 6 || *rule.MaxLevelCount != 2) {
			t.Fatal("extra changed class formula")
		}
	}
	if !extra {
		t.Fatal("extra allowance not persisted")
	}
	b.add("level six", domain.Event{Type: domain.EventChange, Changes: []domain.Change{{Path: "identity.desiredLevel", Op: domain.OpSet, Value: domain.IntValue(6)}}})
	limits, err = domain.SpellRules(b.log(), cat)
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range limits {
		if rule.Class == "sorcerer" && rule.Purpose == "known" && (rule.Count != 7 || *rule.MaxLevelCount != 4) {
			t.Fatalf("level six formula wrong: %+v", rule)
		}
	}
}
