package catalog

import (
	domain "github.com/promix1722/easydnd/internal/domain/catalog"
	"testing"
)

func TestSpellArtworkInSummaryAndDetail(t *testing.T) {
	c := converter{}
	for _, icon := range []string{"", "data:image/webp;base64,cGFjayBieXRlcw=="} {
		spell := domain.Spell{Icon: icon}
		if c.spellSummary(spell).Icon != icon || c.spell(spell).Icon != icon {
			t.Fatal("catalog lost icon")
		}
	}
}

func TestItemArtworkInDetail(t *testing.T) {
	c := converter{}
	for _, icon := range []string{"", "data:image/webp;base64,cGFjayBieXRlcw=="} {
		if c.item(domain.Item{Icon: icon}).Icon != icon || c.magicItem(domain.MagicItem{Icon: icon}).Icon != icon {
			t.Fatal("catalog lost item artwork")
		}
	}
}

func TestItemIconPaletteIsACollectionOfLabels(t *testing.T) {
	got, ok := entries(converter{cat: &domain.Catalog{ItemIcons: map[string]string{"sword": "b", "axe": "a"}}}, CollectionItemIcons)
	icons, _ := got.([]Item)
	if !ok || len(icons) != 2 || icons[0].Slug != "axe" || icons[0].Icon != "a" || icons[1].Slug != "sword" {
		t.Fatalf("item-icons = %+v", got)
	}
}
