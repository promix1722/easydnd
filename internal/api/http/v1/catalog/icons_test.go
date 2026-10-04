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
