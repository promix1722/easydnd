package character

import (
	"slices"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// wornSlots is where auto-equip looks, in the order it fills them, and how
// many each takes. The hands come first because the off hand depends on what
// the main hand holds.
var wornSlots = []struct {
	slot  catalog.Slot
	count int
}{
	{catalog.SlotMainHand, 1}, {catalog.SlotOffHand, 1}, {catalog.SlotBody, 1},
	{catalog.SlotHead, 1}, {catalog.SlotNeck, 1}, {catalog.SlotBack, 1},
	{catalog.SlotArms, 1}, {catalog.SlotWaist, 1}, {catalog.SlotFeet, 1}, {catalog.SlotRing, 2},
}

// AutoEquip is what a character who has put nothing on would wear: one
// suitable thing from the backpack for each slot, as the changes that wear it.
//
// It exists because nothing else equips. A build and an import both used to:
// a kit choice asked "for the main hand" was worn, and a sheet's weapons were
// imported as wielded -- which seated a barbarian's net, longsword, four
// javelins and handaxe in one hand, seven deep. Building and importing now
// only fill the backpack, and this runs once at the end.
//
// "Suitable" is deliberately modest:
//   - a slot takes the first thing in the backpack made for it, except the
//     body, which takes the armor with the best base AC, and the main hand,
//     which takes the weapon with the biggest damage die;
//   - the off hand takes only something made for the off hand -- a shield --
//     and nothing at all beside a two-handed weapon. A second weapon there is
//     a fighting style, and a guess at one is an attack nobody chose.
//
// ponytail: proficiency is not consulted -- a wizard carrying chain mail gets
// it put on. Filter by State.Proficiencies when that matters; the player can
// take anything off from the sheet meanwhile.
//
// It returns nothing when something is already equipped: this is a start, not
// a correction, and running it twice must not undo what a player arranged.
func AutoEquip(state State, cat *catalog.Catalog) []Change {
	if len(state.Equipment.Equipped) > 0 {
		return nil
	}
	slotOf := func(slug rules.Slug) (catalog.Slot, catalog.Item, bool) {
		if item, ok := cat.Items.Get(slug); ok {
			return item.Slot, item, true
		}
		if item, ok := cat.MagicItems.Get(slug); ok {
			return item.Slot, catalog.Item{}, true
		}
		return catalog.SlotNone, catalog.Item{}, false
	}
	left := map[rules.Slug]int{}
	for _, stack := range state.Equipment.Backpack {
		left[stack.Item] += stack.Count
	}
	var worn []rules.Slug
	twoHanded := false
	for _, want := range wornSlots {
		if want.slot == catalog.SlotOffHand && twoHanded {
			continue
		}
		for n := 0; n < want.count; n++ {
			var pick rules.Slug
			best := -1
			for _, stack := range state.Equipment.Backpack {
				slot, item, ok := slotOf(stack.Item)
				if !ok || slot != want.slot || left[stack.Item] == 0 {
					continue
				}
				score := 0
				switch {
				case want.slot == catalog.SlotBody && item.Armor != nil:
					score = item.Armor.BaseAC
				case want.slot == catalog.SlotMainHand && item.Weapon != nil && item.Weapon.Damage != nil:
					// The most the die can roll, so a rapier beats the two
					// daggers a kit happens to list before it.
					for _, term := range item.Weapon.Damage.Dice.Terms {
						score += term.Count * term.Faces
					}
				}
				if score > best {
					pick, best = stack.Item, score
				}
			}
			if pick.IsZero() {
				break
			}
			left[pick]--
			worn = append(worn, pick)
			if _, item, _ := slotOf(pick); want.slot == catalog.SlotMainHand && item.Weapon != nil && slices.Contains(item.Weapon.Properties, "two-handed") {
				twoHanded = true
			}
		}
	}
	if len(worn) == 0 {
		return nil
	}
	// The equipped list whole and per slug, and the backpack per slug: the
	// same three writes the sheet's own Equip makes, for the reason given on
	// the web client's `equippedChanges` -- the projection reads them at
	// different moments.
	changes := []Change{{Path: "equipment.equipped", Op: OpSet, Value: SlugListValue(worn)}}
	counted := map[rules.Slug]int{}
	for _, slug := range worn {
		counted[slug]++
	}
	for _, slug := range worn {
		if counted[slug] == 0 {
			continue
		}
		changes = append(changes,
			Change{Path: Path("equipment.equipped." + string(slug)), Op: OpSet, Value: IntValue(counted[slug])},
			Change{Path: Path("equipment.backpack." + string(slug)), Op: OpSet, Value: IntValue(left[slug])})
		counted[slug] = 0
	}
	return changes
}
