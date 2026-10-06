package catalog

import (
	"testing"

	domain "github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// The unit is an enumeration; converting it as a byte sent "\x04" for gold.
func TestCostUnitIsItsAbbreviation(t *testing.T) {
	if got := costOf(rules.Coins{Amount: 25, Unit: rules.Gold}).Unit; got != "gp" {
		t.Fatalf("unit = %q, want gp", got)
	}
}

// The slot travels as its wire name, and an item that is only carried sends none.
func TestSlotIsItsWireName(t *testing.T) {
	c := converter{}
	if got := c.item(domain.Item{Slot: domain.SlotOffHand}).Slot; got != "off-hand" {
		t.Fatalf("slot = %q, want off-hand", got)
	}
	if got := c.magicItem(domain.MagicItem{Slot: domain.SlotFeet}).Slot; got != "feet" {
		t.Fatalf("slot = %q, want feet", got)
	}
	if got := c.item(domain.Item{}).Slot; got != "" {
		t.Fatalf("carried item slot = %q, want empty", got)
	}
}
