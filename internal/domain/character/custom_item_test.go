package character

import (
	"slices"
	"testing"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// projectCustom projects the rogue with one custom item and whatever the
// player has since done to it, against the overlay a real request builds.
func projectCustom(t *testing.T, option CustomOption, changes ...Change) State {
	t.Helper()
	log := RogueLog(t)
	if err := log.Append(Event{Type: EventNote, Custom: &option}); err != nil {
		t.Fatal(err)
	}
	if len(changes) > 0 {
		if err := log.Append(Event{Type: EventChange, Changes: changes}); err != nil {
			t.Fatal(err)
		}
	}
	state, err := Project(log, WithCustomCatalog(log, LoadCatalog(t)))
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	return state
}

func holds(stacks []ItemStack, slug rules.Slug) bool {
	return slices.ContainsFunc(stacks, func(s ItemStack) bool { return s.Item == slug })
}

// A custom item is equipped by the changes the sheet writes for any item, and
// then it is armor and a weapon like any other.
func TestAnEquippedCustomItemCountsLikeACatalogueOne(t *testing.T) {
	t.Parallel()
	plate := CustomOption{ID: "plate", Kind: "item", Name: "Glass Plate", Selected: true, Item: &CustomItem{Armor: &catalog.Armor{Category: catalog.HeavyArmor, BaseAC: 17}}}
	carried := projectCustom(t, plate)
	if !holds(carried.Equipment.Backpack, "custom-plate") || carried.Status.ArmorClass == 17 {
		t.Fatalf("a new custom item is not simply carried: %+v, AC %d", carried.Equipment, carried.Status.ArmorClass)
	}
	worn := projectCustom(t, plate,
		Change{Path: "equipment.equipped", Op: OpSet, Value: SlugListValue([]rules.Slug{"custom-plate"})},
		Change{Path: "equipment.equipped.custom-plate", Op: OpSet, Value: IntValue(1)},
		Change{Path: "equipment.backpack.custom-plate", Op: OpSet, Value: IntValue(0)},
	)
	if worn.Status.ArmorClass != 17 || holds(worn.Equipment.Backpack, "custom-plate") {
		t.Errorf("worn custom plate: AC %d, backpack %+v; want 17 and not carried", worn.Status.ArmorClass, worn.Equipment.Backpack)
	}

	damage := rules.Damage{Dice: rules.Dice{Terms: []rules.DiceTerm{{Count: 1, Faces: 10}}}, Type: "piercing"}
	spear := CustomOption{ID: "spear", Kind: "item", Name: "Bone Spear", Selected: true, Item: &CustomItem{Weapon: &catalog.Weapon{Category: catalog.SimpleWeapon, Range: catalog.MeleeWeapon, Damage: &damage}}}
	armed := projectCustom(t, spear, Change{Path: "equipment.equipped.custom-spear", Op: OpSet, Value: IntValue(1)}, Change{Path: "equipment.backpack.custom-spear", Op: OpSet, Value: IntValue(0)})
	at := slices.IndexFunc(armed.Actions, func(a Action) bool { return a.Origin.String() == "item:custom-spear" })
	if at < 0 {
		t.Fatalf("an equipped custom weapon gives no attack: %+v", armed.Actions)
	}
	// A rogue is proficient with simple weapons: Strength plus proficiency.
	if want := armed.Abilities.Modifier(rules.Strength) + armed.Status.ProficiencyBonus; *armed.Actions[at].ToHit != want {
		t.Errorf("to hit = %d, want %d", *armed.Actions[at].ToHit, want)
	}
}

// The definition says where the item starts, not where it is: it used to be
// put back on every projection, so a dropped custom item could not be dropped.
func TestADroppedCustomItemStaysDropped(t *testing.T) {
	t.Parallel()
	ring := CustomOption{ID: "ring", Kind: "item", Name: "Ring", Selected: true, Count: 2}
	if s := projectCustom(t, ring, Change{Path: "equipment.backpack.custom-ring", Op: OpSet, Value: IntValue(0)}); holds(s.Equipment.Backpack, "custom-ring") {
		t.Errorf("dropped custom item is back: %+v", s.Equipment.Backpack)
	}
}

// An item nobody has touched keeps the placement it was imported with, even
// past a later whole-list write -- the AI Wizard clears a list that way.
func TestAnUntouchedCustomItemKeepsItsPlacement(t *testing.T) {
	t.Parallel()
	gem := CustomOption{ID: "gem", Kind: "item", Name: "Gem", Selected: true, Placement: "loot"}
	s := projectCustom(t, gem, Change{Path: "equipment.loot", Op: OpSet, Value: SlugListValue(nil)})
	if !holds(s.Equipment.Loot, "custom-gem") {
		t.Errorf("loot = %+v, want the custom gem", s.Equipment.Loot)
	}
}
