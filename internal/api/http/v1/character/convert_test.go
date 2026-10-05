package character_test

import (
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
