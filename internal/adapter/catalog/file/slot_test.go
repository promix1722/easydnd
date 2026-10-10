package file

import (
	"testing"

	"github.com/promix1722/easydnd/internal/domain/catalog"
)

// A pack row may say where an item goes; when it does not, the item's shape
// decides, and a namespaced category still counts as its bare word.
func TestSlotIsExplicitOrDerivedFromTheItem(t *testing.T) {
	t.Parallel()
	c := &conv{where: "test"}
	items := []struct {
		name string
		got  catalog.Slot
		want catalog.Slot
	}{
		{"explicit", c.item(Item{Slug: "robes", Category: "adventuring-gear", Slot: SlotBody}, nil).Slot, catalog.SlotBody},
		{"shield", c.item(Item{Slug: "shield", Category: "armor", Armor: &Armor{Category: ArmorShield}}, nil).Slot, catalog.SlotOffHand},
		{"armor", c.item(Item{Slug: "chain-mail", Category: "armor", Armor: &Armor{Category: ArmorHeavy}}, nil).Slot, catalog.SlotBody},
		{"weapon", c.item(Item{Slug: "dagger", Category: "weapon", Weapon: &Weapon{Category: WeaponSimple, Range: WeaponMelee}}, nil).Slot, catalog.SlotMainHand},
		{"focus", c.item(Item{Slug: "wand", Category: "adventuring-gear", Gear: &Gear{GearCategory: "arcane-foci"}}, nil).Slot, catalog.SlotMainHand},
		{"carried", c.item(Item{Slug: "torch", Category: "adventuring-gear", Gear: &Gear{GearCategory: "standard-gear"}}, nil).Slot, catalog.SlotNone},
		{"magic explicit", c.magicItem(MagicItem{Slug: "boots-of-speed", Category: "wondrous-items", Slot: SlotFeet}, nil).Slot, catalog.SlotFeet},
		{"hands, the old name for arms", c.magicItem(MagicItem{Slug: "gloves-of-missile-snaring", Category: "wondrous-items", Slot: "hands"}, nil).Slot, catalog.SlotArms},
		{"magic ring, namespaced", c.magicItem(MagicItem{Slug: "dnd-2014/ring-of-jumping", Category: "dnd-2014/ring"}, nil).Slot, catalog.SlotRing},
		{"magic armor", c.magicItem(MagicItem{Slug: "armor-1", Category: "armor"}, nil).Slot, catalog.SlotBody},
		{"wand", c.magicItem(MagicItem{Slug: "wand-of-fear", Category: "wand"}, nil).Slot, catalog.SlotMainHand},
		{"wondrous, no slot", c.magicItem(MagicItem{Slug: "bag-of-holding", Category: "wondrous-items"}, nil).Slot, catalog.SlotNone},
	}
	for _, tc := range items {
		if tc.got != tc.want {
			t.Errorf("%s: slot = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
	if c.Err() != nil {
		t.Fatalf("unexpected error: %v", c.Err())
	}

	c.item(Item{Slug: "hat", Category: "adventuring-gear", Slot: "elbow"}, nil)
	if c.Err() == nil {
		t.Fatal("an unknown slot loaded")
	}
}
