package character_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	catalogfile "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

func TestImportNameEditRetainsObservedClassScoresAndInventory(t *testing.T) {
	svc := newService(t)
	model := modelFunc(func(context.Context, charuc.AgentRequest, func(string)) (charuc.AgentResponse, error) {
		return charuc.AgentResponse{Calls: []charuc.AgentCall{
			{ID: "n", Name: "resolve_import_facts", Arguments: `{"path":"identity.name","value":"Imported"}`},
			{ID: "c", Name: "resolve_import_facts", Arguments: `{"ref":"class:sorcerer","level":3}`},
			{ID: "r", Name: "resolve_import_facts", Arguments: `{"ref":"race:half-elf"}`},
			{ID: "a", Name: "resolve_import_facts", Arguments: `{"path":"finalAbilities.cha","value":17}`},
			{ID: "i", Name: "resolve_import_facts", Arguments: `{"path":"equipment.backpack.dagger","value":2}`},
			{ID: "coins", Name: "resolve_import_facts", Arguments: `{"path":"equipment.purse.gp","value":2}`},
			{ID: "done", Name: "prepare_review", Arguments: `{"text":"Ready","allow_incomplete":true}`},
		}}, nil
	})
	a := charuc.NewAgent(svc, model, charuc.AgentConfig{Workers: 1})
	defer a.Close()
	session, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, nil, "Create a sorcerer")
	if err != nil {
		t.Fatal(err)
	}
	session = waitAgent(t, a, session.ID, func(s charuc.AgentSession) bool { return s.Status == "review" })
	before, err := a.Sheet(context.Background(), session)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := svc.Get(context.Background(), testOwner, session.CharacterID)
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.Revise(charuc.WithRevision(context.Background(), stored.Revision), testOwner, stored.ID, rules.DefaultLocale, stored.Log.LastSeq(), 1, &domain.Event{Type: domain.EventInit, Changes: []domain.Change{{Path: "identity.name", Op: domain.OpSet, Value: domain.StringValue("Renamed")}}}, true)
	if err != nil {
		t.Fatal(err)
	}
	before.Identity.Name = "Renamed"
	if !reflect.DeepEqual(before, result.Sheet) {
		t.Fatalf("renaming erased import facts:\nbefore=%+v\nafter=%+v", before, result.Sheet)
	}
	if len(result.Dropped) != 0 {
		t.Fatalf("renaming dropped imports: %+v", result.Dropped)
	}
	session, _ = a.Get(testOwner, session.ID)
	saved, err := a.Get(testOwner, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	c, err := svc.Get(context.Background(), testOwner, saved.CharacterID)
	if err != nil {
		t.Fatal(err)
	}
	var classFound bool
	for _, e := range c.Log.Events {
		if e.Type == domain.EventClass {
			classFound = true
			if !e.Observed || e.Source != domain.GroupClass {
				t.Fatal("class hidden from editor")
			}
		}
	}
	if !classFound {
		t.Fatal("class disappeared at save")
	}
}

func TestCustomOptionsRoundTripRevisionAndIsolation(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	c := mustCreateScored(t, svc)
	option := domain.CustomOption{ID: "criminal", Kind: "background", Name: "Criminal", Description: "Source skill descriptions", Source: "PDF page 1", Selected: true}
	result, err := svc.UpsertCustomOption(charuc.WithRevision(ctx, c.Revision), testOwner, c.ID, rules.DefaultLocale, option)
	if err != nil {
		t.Fatal(err)
	}
	if result.Sheet.Identity.Background != "custom-criminal" || len(result.Sheet.CustomOptions) != 1 {
		t.Fatalf("custom selection lost: %+v", result.Sheet)
	}
	scoped, err := svc.CharacterCatalog(ctx, testOwner, c.ID, rules.DefaultLocale)
	if err != nil {
		t.Fatal(err)
	}
	if !scoped.Backgrounds.Has("custom-criminal") {
		t.Fatal("custom absent from scoped catalogue")
	}
	other := mustCreateScored(t, svc)
	otherCat, _ := svc.CharacterCatalog(ctx, testOwner, other.ID, rules.DefaultLocale)
	if otherCat.Backgrounds.Has("custom-criminal") {
		t.Fatal("custom leaked to another character")
	}
	if _, err = svc.UpsertCustomOption(charuc.WithRevision(ctx, c.Revision), testOwner, c.ID, rules.DefaultLocale, option); err == nil {
		t.Fatal("stale write accepted")
	}
	if _, err = svc.UpsertCustomOption(ctx, "intruder", c.ID, rules.DefaultLocale, option); err == nil {
		t.Fatal("foreign edit accepted")
	}
	option.Reference = "background:custom-criminal" // Legacy self-match must remain editable.
	option.Name = "Edited Criminal"
	option.Description = "Updated source detail"
	result, err = svc.UpsertCustomOption(charuc.WithRevision(ctx, result.Revision), testOwner, c.ID, rules.DefaultLocale, option)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Sheet.CustomOptions) != 1 || result.Sheet.CustomOptions[0].Name != option.Name || result.Sheet.CustomOptions[0].Reference != "" {
		t.Fatal("editing duplicated definition")
	}
	option.Selected = false
	result, err = svc.UpsertCustomOption(charuc.WithRevision(ctx, result.Revision), testOwner, c.ID, rules.DefaultLocale, option)
	if err != nil {
		t.Fatal(err)
	}
	if result.Sheet.Identity.Background != "" {
		t.Fatal("custom selection cannot be removed")
	}
}

func TestCustomSpellPreservesCanonicalIdentityAndUnknownClassMechanics(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	c := mustCreateScored(t, svc)
	level := 3
	result, err := svc.UpsertCustomOption(ctx, testOwner, c.ID, rules.DefaultLocale, domain.CustomOption{ID: "unknown-class", Kind: "class", Name: "Unpublished Class", Level: &level, Selected: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Sheet.Resources.HitDice) > 0 || result.Sheet.Base.HitPoints.Max != 0 {
		t.Fatal("invented mechanics for unknown hit die")
	}
	result, err = svc.UpsertCustomOption(charuc.WithRevision(ctx, result.Revision), testOwner, c.ID, rules.DefaultLocale, domain.CustomOption{ID: "extra", Kind: "spell", Name: "Fireball", Reference: "spell:fireball", Parent: "custom-unknown-class", Ability: "int", Mode: "known", Selected: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Sheet.Spells.Known) != 1 || result.Sheet.Spells.Known[0] != "fireball" || len(result.Sheet.Spells.Sources) != 1 {
		t.Fatalf("lost spell identity/source: %+v", result.Sheet.Spells)
	}
}

func TestSourceChecklistBlocksPrematureReviewAndBatchRetainsValidFacts(t *testing.T) {
	var turn int
	var blocked bool
	model := modelFunc(func(_ context.Context, request charuc.AgentRequest, _ func(string)) (charuc.AgentResponse, error) {
		turn++
		if turn == 1 {
			return charuc.AgentResponse{Calls: []charuc.AgentCall{
				{ID: "plan", Name: "plan_import", Arguments: `{"expected":["identity.name","class:sorcerer","race:half-elf","finalAbilities.str","custom:criminal"]}`},
				{ID: "batch", Name: "import_facts", Arguments: `{"facts":[{"path":"identity.name","value":"Source hero"},{"ref":"class:sorcerer","level":3},{"ref":"srd-2014:race:half-elf"},{"path":"finalAbilities.str","value":8},{"path":"base.unsupported","value":30}]}`},
				{ID: "early", Name: "prepare_review", Arguments: `{"text":"Ready","allow_incomplete":true}`},
			}}, nil
		}
		for _, raw := range request.Input {
			if strings.Contains(string(raw), "missingSourceFacts") && strings.Contains(string(raw), "custom:criminal") {
				blocked = true
			}
		}
		return charuc.AgentResponse{Calls: []charuc.AgentCall{
			{ID: "custom", Name: "upsert_custom_option", Arguments: `{"id":"criminal","kind":"background","name":"Criminal","hit_die":0,"level":0,"description":"Source background"}`},
			{ID: "done", Name: "prepare_review", Arguments: `{"text":"Ready","allow_incomplete":true}`},
		}}, nil
	})
	svc := newService(t)
	a := charuc.NewAgent(svc, model, charuc.AgentConfig{Workers: 1})
	defer a.Close()
	s, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), "Preserve documented source facts")
	if err != nil {
		t.Fatal(err)
	}
	s = waitAgent(t, a, s.ID, func(s charuc.AgentSession) bool { return s.Status == "review" })
	if !blocked {
		t.Fatal("partial historical choices bypassed missing source facts")
	}
	sheet, err := a.Sheet(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if sheet.Identity.Name != "Source hero" || sheet.Abilities.Score(rules.Ability("str")) != 8 || sheet.Identity.Background != "custom-criminal" {
		t.Fatalf("batch/custom facts lost: %+v", sheet.Identity)
	}
	if len(s.Manual) != 1 {
		t.Fatal("failed custom data leaked into manual entries")
	}
	count := 0
	for _, e := range s.Events {
		if e.Kind == "progress" {
			count++
		}
	}
	if count != 5 {
		t.Fatalf("progress includes rejected writes: %d", count)
	}
}

func TestSourceIdentityReuseSubclassOrderAndInventoryCounts(t *testing.T) {
	svc := newService(t)
	model := modelFunc(func(context.Context, charuc.AgentRequest, func(string)) (charuc.AgentResponse, error) {
		return charuc.AgentResponse{Calls: []charuc.AgentCall{
			{ID: "plan", Name: "plan_import", Arguments: `{"expected":["identity.name","class:sorcerer","race:half-elf","spell:message"]}`},
			{ID: "sub", Name: "upsert_custom_option", Arguments: `{"id":"wild-magic","kind":"subclass","name":"Wild Magic","parent":"class:sorcerer"}`},
			{ID: "facts", Name: "import_facts", Arguments: `{"facts":[{"path":"identity.name","value":"Source hero"},{"ref":"class:sorcerer","level":3},{"ref":"race:half-elf"},{"path":"equipment.equipped.dagger","value":2}]}`},
			{ID: "class", Name: "upsert_custom_option", Arguments: `{"id":"class-details","kind":"class","name":"Sorcerer","description":"Source wording"}`},
			{ID: "race", Name: "upsert_custom_option", Arguments: `{"id":"race-details","kind":"race","name":"Half-Elf","description":"Source wording"}`},
			{ID: "dagger", Name: "upsert_custom_option", Arguments: `{"id":"daggers","kind":"item","name":"Dagger","count":2,"placement":"equipped"}`},
			{ID: "message", Name: "upsert_custom_option", Arguments: `{"id":"message","kind":"spell","name":"Message","parent":"sorcerer","ability":"cha"}`},
			{ID: "duplicate", Name: "upsert_custom_option", Arguments: `{"id":"duplicate","kind":"item","name":"Duplicate dagger","count":2,"selected":false}`},
			{ID: "done", Name: "prepare_review", Arguments: `{"text":"Ready","allow_incomplete":true}`},
		}}, nil
	})
	a := charuc.NewAgent(svc, model, charuc.AgentConfig{Workers: 1})
	defer a.Close()
	s, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), "Preserve source")
	if err != nil {
		t.Fatal(err)
	}
	s = waitAgent(t, a, s.ID, func(s charuc.AgentSession) bool { return s.Status == "review" })
	sheet, err := a.Sheet(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if sheet.Identity.Race != "half-elf" || sheet.Identity.Classes[0].Class != "sorcerer" || sheet.Identity.Classes[0].Level != 3 || sheet.Identity.Classes[0].Subclass != "custom-wild-magic" {
		t.Fatalf("source identities/order lost: %+v", sheet.Identity)
	}
	if len(sheet.Equipment.Equipped) != 1 || sheet.Equipment.Equipped[0].Item != "dagger" || sheet.Equipment.Equipped[0].Count != 2 {
		t.Fatalf("inventory duplicated: %+v", sheet.Equipment)
	}
	if len(sheet.Spells.Cantrips) != 1 || sheet.Spells.Cantrips[0] != "message" || len(sheet.Spells.Known) != 0 {
		t.Fatalf("cantrip identity lost: %+v", sheet.Spells)
	}
}

func TestSourceScoresAndInventoryCountsSurviveBatchTools(t *testing.T) {
	svc := newService(t)
	model := &script{turns: [][]charuc.AgentCall{{
		call("plan", "plan_import", `{"expected":["identity.name","class:sorcerer","race:half-elf","item:Dagger"],"scores":{"str":8,"dex":14,"con":15,"int":10,"wis":12,"cha":17}}`),
		call("facts", "resolve_import_facts", `{"facts":[{"path":"identity.name","value":"Source hero"},{"ref":"class:sorcerer","level":3},{"ref":"race:half-elf"}]}`),
		// The sheet prints its gear twice, on two pages, and writes a quantity
		// into the name; a stack is the count it states, not the number of
		// times it is mentioned.
		call("gear", "set_inventory", `{"items":[{"name":"Dagger","count":2,"placement":"equipped"},{"name":"2 Dagger","placement":"equipped"},{"name":"Javelins x4","placement":"equipped"},{"name":"Rope, Hempen (50 feet)"},{"name":"Vorpal Spoon"}]}`),
		call("done", "prepare_review", `{"text":"Ready","allow_incomplete":true}`),
	}}}
	a := charuc.NewAgent(svc, model, charuc.AgentConfig{Workers: 1})
	defer a.Close()
	s, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), "Preserve source")
	if err != nil {
		t.Fatal(err)
	}
	s = waitAgent(t, a, s.ID, func(s charuc.AgentSession) bool { return s.Status == "review" })
	sheet, err := a.Sheet(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	for ability, score := range map[string]int{"str": 8, "dex": 14, "con": 15, "int": 10, "wis": 12, "cha": 17} {
		if sheet.Abilities.Score(rules.Ability(ability)) != score {
			t.Fatalf("lost source score %s: %+v", ability, sheet.Abilities)
		}
	}
	counts := map[rules.Slug]int{}
	for _, stack := range sheet.Equipment.Equipped {
		counts[stack.Item] += stack.Count
	}
	if len(counts) != 2 || counts["dagger"] != 2 || counts["javelin"] != 4 {
		t.Fatalf("quantity or duplicate source page lost: %+v", sheet.Equipment)
	}
	// The class's default kit is not on the sheet, so it is not in the pack.
	if len(sheet.Equipment.Backpack) != 1 || sheet.Equipment.Backpack[0].Item != "rope-hempen-50-feet" {
		t.Fatalf("backpack is not the sheet's: %+v", sheet.Equipment.Backpack)
	}
}

func TestImportedClassUsesSelectedPackNamespaceAndSurvivesEditing(t *testing.T) {
	path := filepath.Join("..", "..", "..", "data", "srd_5.1")
	base, err := catalogfile.LoadPack(path)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := catalogfile.NewRegistry([]string{path}, []catalogfile.Dependency{{ID: "dnd-2014", Version: base.Manifest.Version}}, "", catalogfile.PackFolder{Path: path, ID: "dnd-2014"})
	if err != nil {
		t.Fatal(err)
	}
	svc := charuc.NewService(memory.NewCharacterRepository(), memory.NewFolderRepository(), registry, nil, nil, slog.New(slog.DiscardHandler))
	model := modelFunc(func(context.Context, charuc.AgentRequest, func(string)) (charuc.AgentResponse, error) {
		return charuc.AgentResponse{Calls: []charuc.AgentCall{
			{ID: "plan", Name: "plan_import", Arguments: `{"expected":["identity.name","class:sorcerer","race:half-elf","custom:wild-magic"],"scores":{"cha":17}}`},
			{ID: "facts", Name: "import_facts", Arguments: `{"facts":[{"path":"identity.name","value":"Vas Pup"},{"ref":"srd-2014:class:sorcerer","level":3},{"ref":"class:sorcerer","level":3},{"ref":"race:half-elf"}]}`},
			{ID: "subclass", Name: "upsert_custom_option", Arguments: `{"id":"wild-magic","kind":"subclass","name":"Wild Magic","parent":"class:sorcerer"}`},
			{ID: "message", Name: "upsert_custom_option", Arguments: `{"id":"message","kind":"cantrip","name":"Message","ref":"spell:message","parent":"sorcerer","ability":"cha"}`},
			{ID: "done", Name: "prepare_review", Arguments: `{"text":"Ready","allow_incomplete":true}`},
		}}, nil
	})
	a := charuc.NewAgent(svc, model, charuc.AgentConfig{Workers: 1})
	defer a.Close()
	session, err := a.Create(context.Background(), testOwner, "", rules.DefaultLocale, agentFile(), "Preserve this Sorcerer 3 using the selected pack")
	if err != nil {
		t.Fatal(err)
	}
	session = waitAgent(t, a, session.ID, func(s charuc.AgentSession) bool { return s.Status == "review" })
	before, err := a.Sheet(context.Background(), session)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Identity.Classes) != 1 || before.Identity.Classes[0].Class != "dnd-2014/sorcerer" || before.Identity.Classes[0].Level != 3 || before.Identity.Classes[0].Subclass != "custom-wild-magic" {
		t.Fatalf("missing selected class: %+v", before.Identity)
	}
	if len(before.Spells.Cantrips) != 1 || before.Spells.Cantrips[0] != "dnd-2014/message" {
		t.Fatalf("lost selected pack spell identity: %+v", before.Spells)
	}
	classProgress := 0
	for _, event := range session.Events {
		if event.Kind == "progress" {
			b, _ := json.Marshal(event.Data)
			if strings.Contains(string(b), `"field":"class"`) {
				classProgress++
			}
		}
	}
	if classProgress != 1 {
		t.Fatalf("unselected explicit reference was applied: class progress=%d", classProgress)
	}
	saved, err := a.Get(testOwner, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	character, err := svc.Get(context.Background(), testOwner, saved.CharacterID)
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.Revise(charuc.WithRevision(context.Background(), character.Revision), testOwner, character.ID, rules.DefaultLocale, character.Log.LastSeq(), 1, &domain.Event{Type: domain.EventInit, Changes: []domain.Change{{Path: "identity.name", Op: domain.OpSet, Value: domain.StringValue("Renamed")}}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.Identity.Classes, result.Sheet.Identity.Classes) {
		t.Fatalf("class disappeared after editing: %+v, dropped=%+v", result.Sheet.Identity, result.Dropped)
	}
	for _, dropped := range result.Dropped {
		if dropped.Type == domain.EventClass || dropped.Type == domain.EventSubclass {
			t.Fatalf("imported class dropped: %+v", dropped)
		}
	}
}
