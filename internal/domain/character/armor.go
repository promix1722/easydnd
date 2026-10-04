package character

import (
	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// unarmoredBaseAC is the armor class of a character wearing nothing, before
// their Dexterity modifier.
const unarmoredBaseAC = 10

// armorClass computes AC from what the character has equipped.
//
// Body armor sets the base and decides how much Dexterity applies: light
// armor takes the full modifier, medium caps it (usually at +2), heavy takes
// none. A shield adds its BaseAC on top -- the SRD data files a shield under
// the armor category with BaseAC 2 meaning "bonus", not "base", which is why
// it is summed rather than compared.
//
// Only equipped items count. Nothing in the catalogue says what a character
// is wearing, so equipping is an explicit change event; deriving it from the
// backpack would put a rogue's spare shield on their arm alongside a
// two-handed weapon and produce a wrong number with no rule to appeal to.
//
// Pack effects can adjust this base result for features such as Unarmored
// Defense, using equipped:armor and equipped:shield to express eligibility.
func armorClass(equipped []ItemStack, cat *catalog.Catalog, dexModifier int) int {
	base := unarmoredBaseAC
	if cat.Mechanics.Core.BaseArmorClass != 0 {
		base = cat.Mechanics.Core.BaseArmorClass
	}
	dex := dexModifier
	wearing := false
	shields := 0

	for _, stack := range equipped {
		item, ok := cat.Items.Get(stack.Item)
		if !ok || item.Armor == nil {
			continue
		}
		if item.Armor.Category == catalog.Shield {
			shields += item.Armor.BaseAC
			continue
		}
		// Two suits of body armor cannot be worn at once; the last one
		// equipped wins, which is the only reading that makes changing armor
		// a single append.
		wearing = true
		base = item.Armor.BaseAC
		switch {
		case !item.Armor.AddsDexBonus:
			dex = 0
		case item.Armor.MaxDexBonus != nil && dexModifier > *item.Armor.MaxDexBonus:
			dex = *item.Armor.MaxDexBonus
		default:
			dex = dexModifier
		}
	}
	if !wearing {
		dex = dexModifier
	}
	return base + dex + shields
}

// equippedSlugs is the item slugs a character has equipped, for the callers
// that need the list rather than the numbers.
func equippedSlugs(equipped []ItemStack) []rules.Slug {
	out := make([]rules.Slug, 0, len(equipped))
	for _, stack := range equipped {
		if !stack.Item.IsZero() {
			out = append(out, stack.Item)
		}
	}
	return out
}
