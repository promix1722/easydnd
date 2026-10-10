package character_test

import (
	"context"
	"errors"
	"testing"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

func TestCustomOptionsRoundTripRevisionAndIsolation(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

// A note is the player's own text: it is added, rewritten and deleted, and it
// is the only custom entry that can be deleted at all.
func TestCustomNotesAreAddedEditedAndDeleted(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc := newService(t)
	c := mustCreateScored(t, svc)
	at := func(revision int) context.Context { return charuc.WithRevision(ctx, revision) }

	note := domain.CustomOption{Kind: "note", Name: "Backstory", Description: "Born at sea.\nRaised by gulls.", Selected: true}
	added, err := svc.UpsertCustomOption(at(c.Revision), testOwner, c.ID, rules.DefaultLocale, note)
	if err != nil {
		t.Fatal(err)
	}
	if len(added.Sheet.CustomOptions) != 1 || added.Sheet.CustomOptions[0].ID == "" {
		t.Fatalf("note not on the sheet with an id: %+v", added.Sheet.CustomOptions)
	}
	note.ID = added.Sheet.CustomOptions[0].ID
	note.Description = "Born inland."
	edited, err := svc.UpsertCustomOption(at(added.Revision), testOwner, c.ID, rules.DefaultLocale, note)
	if err != nil {
		t.Fatal(err)
	}
	if got := edited.Sheet.CustomOptions; len(got) != 1 || got[0].Description != "Born inland." || got[0].Name != "Backstory" {
		t.Fatalf("edit = %+v, want the one note rewritten", got)
	}
	background := domain.CustomOption{ID: "criminal", Kind: "background", Name: "Criminal", Selected: true}
	built, err := svc.UpsertCustomOption(at(edited.Revision), testOwner, c.ID, rules.DefaultLocale, background)
	if err != nil {
		t.Fatal(err)
	}

	remove := func(revision int, owner domain.OwnerID, option string) (charuc.Revision, error) {
		return svc.RemoveCustomOption(at(revision), owner, c.ID, rules.DefaultLocale, option)
	}
	if _, err = remove(edited.Revision, testOwner, note.ID); err == nil {
		t.Error("stale delete accepted")
	}
	if _, err = remove(built.Revision, "intruder", note.ID); err == nil {
		t.Error("foreign delete accepted")
	}
	if _, err = remove(built.Revision, testOwner, "criminal"); err == nil {
		t.Error("a background the character is built on was deleted")
	}
	if _, err = remove(built.Revision, testOwner, "no-such-note"); !types.IsNotFound(err) {
		t.Errorf("unknown note = %v, want not found", err)
	}
	removed, err := remove(built.Revision, testOwner, note.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := removed.Sheet.CustomOptions; len(got) != 1 || got[0].ID != "criminal" || removed.Sheet.Identity.Background != "custom-criminal" {
		t.Fatalf("after delete = %+v, want only the background left and still chosen", got)
	}
	// The stored character agrees with the answer, at the revision it gave.
	stored, sheet, err := svc.View(ctx, testOwner, c.ID, rules.DefaultLocale)
	if err != nil || stored.Revision != removed.Revision || len(sheet.CustomOptions) != 1 {
		t.Fatalf("stored revision %d with %d entries (err %v), want %d with 1", stored.Revision, len(sheet.CustomOptions), err, removed.Revision)
	}
}

// A custom item's mechanics are checked against the character's own rules,
// wherever the write comes from.
func TestACustomItemIsHeldToTheRules(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc := newService(t)
	c := mustCreateScored(t, svc)
	d := func(count, faces int, kind rules.Slug) *rules.Damage {
		return &rules.Damage{Dice: rules.Dice{Terms: []rules.DiceTerm{{Count: count, Faces: faces}}}, Type: kind}
	}
	sword := catalog.Weapon{Category: catalog.MartialWeapon, Range: catalog.MeleeWeapon, Damage: d(1, 8, "slashing"), Properties: []rules.Slug{"finesse"}}
	refused := map[string]domain.CustomOption{
		"mechanics on a note": {Kind: "note", Name: "n", Item: &domain.CustomItem{}},
		"weapon and armor":    {Kind: "item", Name: "n", Item: &domain.CustomItem{Weapon: &sword, Armor: &catalog.Armor{Category: catalog.LightArmor}}},
		"unknown icon":        {Kind: "item", Name: "n", Item: &domain.CustomItem{Icon: "no-such-picture"}},
		"absurd dice":         {Kind: "item", Name: "n", Item: &domain.CustomItem{Weapon: &catalog.Weapon{Category: catalog.SimpleWeapon, Range: catalog.MeleeWeapon, Damage: d(1000, 6, "slashing")}}},
		"unknown damage type": {Kind: "item", Name: "n", Item: &domain.CustomItem{Weapon: &catalog.Weapon{Category: catalog.SimpleWeapon, Range: catalog.MeleeWeapon, Damage: d(1, 6, "sarcasm")}}},
		"armor of no kind":    {Kind: "item", Name: "n", Item: &domain.CustomItem{Armor: &catalog.Armor{BaseAC: 12}}},
		"coins of no kind":    {Kind: "item", Name: "n", Item: &domain.CustomItem{Cost: rules.Coins{Amount: 5}}},
	}
	for name, option := range refused {
		var field *types.FieldValidationError
		if _, err := svc.UpsertCustomOption(ctx, testOwner, c.ID, rules.DefaultLocale, option); !errors.As(err, &field) {
			t.Errorf("%s: error = %v, want a field error", name, err)
		}
	}

	made, err := svc.UpsertCustomOption(ctx, testOwner, c.ID, rules.DefaultLocale, domain.CustomOption{ID: "blade", Kind: "item", Name: "Blade", Selected: true, Item: &domain.CustomItem{Weapon: &sword}})
	if err != nil {
		t.Fatal(err)
	}
	scoped, err := svc.CharacterCatalog(ctx, testOwner, c.ID, rules.DefaultLocale)
	if err != nil {
		t.Fatal(err)
	}
	if it, _ := scoped.Items.Get("custom-blade"); it.Slot != catalog.SlotMainHand || it.Weapon == nil {
		t.Errorf("custom blade in the catalogue = %+v, want a held weapon", it)
	}
	// A rename that says nothing about mechanics keeps them.
	renamed, err := svc.UpsertCustomOption(ctx, testOwner, c.ID, rules.DefaultLocale, domain.CustomOption{ID: "blade", Kind: "item", Name: "Old Blade", Selected: true})
	if err != nil || len(made.Sheet.CustomOptions) != 1 || renamed.Sheet.CustomOptions[0].Item == nil {
		t.Errorf("rename lost the mechanics: %+v, %v", renamed.Sheet.CustomOptions, err)
	}
}
