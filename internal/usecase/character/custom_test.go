package character_test

import (
	"context"
	"testing"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

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
