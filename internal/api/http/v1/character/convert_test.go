package character_test

import (
	"context"
	"path/filepath"

	catalogfile "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	api "github.com/promix1722/easydnd/internal/api/http/v1/character"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"testing"
)

func TestImportedCoinsUseReadableDenominations(t *testing.T) {
	sheet := api.SheetOf(domain.State{Equipment: domain.Equipment{Purse: rules.Purse{rules.Copper: 6, rules.Silver: 6, rules.Gold: 2}}})
	if len(sheet.Equipment.Purse) != 3 || sheet.Equipment.Purse["cp"] != 6 || sheet.Equipment.Purse["sp"] != 6 || sheet.Equipment.Purse["gp"] != 2 {
		t.Fatalf("invalid coin keys: %#v", sheet.Equipment.Purse)
	}
}

func TestSheetIncludesPortrait(t *testing.T) {
	portrait := "data:image/webp;base64,cG9ydHJhaXQ="
	sheet := api.SheetOf(domain.State{Identity: domain.Identity{Image: portrait}})
	if sheet.Identity.Image != portrait {
		t.Fatal("sheet response lost portrait")
	}
}

// A sheet is drawn from one response: every slug it carries arrives with what
// it means. The client used to download whole collections -- every spell in
// the rules among them -- to do this lookup itself.
func TestResolvedSheetNamesWhatItCarries(t *testing.T) {
	cat, err := catalogfile.NewSource(filepath.Join("..", "..", "..", "..", "..", "data", "srd_5.1")).Load(context.Background(), rules.LocaleEN)
	if err != nil {
		t.Fatal(err)
	}
	sheet := api.ResolvedSheetOf(domain.State{
		CatalogNames: map[string]string{"races:elf": "Named by the projection"},
		Identity: domain.Identity{Race: "elf", Background: "acolyte",
			Classes: []domain.ClassLevel{{Class: "wizard", Subclass: "evocation", Level: 2}}},
		Base:          domain.Base{Languages: []rules.Slug{"common"}},
		Traits:        []rules.Slug{"darkvision"},
		Proficiencies: []rules.Slug{"daggers", "not-in-the-catalogue"},
		Equipment:     domain.Equipment{Backpack: []domain.ItemStack{{Item: "dagger", Count: 2}, {Item: "dagger", Count: 1}}},
		Spells: domain.Spellbook{Cantrips: []rules.Slug{"fire-bolt"},
			Sources: []domain.SpellSource{{Source: rules.Ref{Kind: rules.RefClass, Slug: "wizard"}, Spellbook: []rules.Slug{"magic-missile"}}}},
	}, cat)

	for key, want := range map[string]string{
		"races:elf": "Named by the projection", "backgrounds:acolyte": "Acolyte", "classes:wizard": "Wizard",
		"subclasses:evocation": "Evocation", "languages:common": "Common", "traits:darkvision": "Darkvision",
	} {
		if got := sheet.CatalogNames[key]; got != want {
			t.Errorf("catalogNames[%q] = %q, want %q", key, got, want)
		}
	}
	got := sheet.Catalog
	if got == nil || len(got.Skills) != 18 || len(got.Proficiencies) != 1 || len(got.Equipment) != 1 || len(got.Spells) != 2 {
		t.Fatalf("resolved entries: %+v", got)
	}
	if got.Spells[0].Name != "Fire Bolt" || got.Spells[0].Level != 0 {
		t.Errorf("a resolved spell: %+v", got.Spells[0])
	}
}
