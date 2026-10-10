package character

import (
	"maps"
	"slices"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// deriveActions rebuilds State.Actions: what the character can do on a turn.
//
// Nothing here names a class or a feature. An action is on the list because
// the pack put it there, in one of three ways: an equipped item is a weapon,
// an entry the character holds carries an action tag, or the pack declares a
// standalone action the character is eligible for. Spells are not actions
// here; they have their own list.
//
// It runs after actionOffers, which has already decided eligibility for the
// standalone actions that have an owner.
func deriveActions(s *State, cat *catalog.Catalog) {
	s.Actions = nil
	for _, stack := range s.Equipment.Equipped {
		if it, ok := cat.Items.Get(stack.Item); ok {
			origin := rules.NewRef(rules.RefItem, it.Slug)
			if it.Weapon != nil {
				s.Actions = append(s.Actions, weaponAttack(s, cat, origin, it))
			}
			s.tagged(origin, it.Entry, ActionFromEquipment)
		}
		if it, ok := cat.MagicItems.Get(stack.Item); ok {
			s.tagged(rules.NewRef(rules.RefMagicItem, it.Slug), it.Entry, ActionFromEquipment)
		}
	}
	for _, slug := range s.Features {
		if e, ok := cat.Features.Get(slug); ok {
			s.tagged(rules.NewRef(rules.RefFeature, slug), e.Entry, ActionFromFeature)
		}
	}
	for _, slug := range s.Traits {
		if e, ok := cat.Traits.Get(slug); ok {
			s.tagged(rules.NewRef(rules.RefTrait, slug), e.Entry, ActionFromFeature)
		}
	}
	for _, slug := range s.Feats {
		if e, ok := cat.Feats.Get(slug); ok {
			s.tagged(rules.NewRef(rules.RefFeat, slug), e.Entry, ActionFromFeature)
		}
	}
	for _, def := range cat.Mechanics.Actions {
		if def.Owner.IsZero() {
			s.Actions = append(s.Actions, Action{Source: Derived, Origin: rules.NewRef(rules.RefAction, def.Slug), Kind: actionKind(def.Kind), Category: BasicAction, Name: def.Name})
		}
	}
	for _, offer := range s.PackActions {
		a := Action{Source: Derived, Origin: rules.NewRef(rules.RefAction, offer.ID), Kind: offer.Kind, Category: ActionFromFeature, Name: offer.Name}
		// An action that spends several pools shows the first; the list has
		// room for one counter, and no pack spends two yet.
		for _, id := range slices.Sorted(maps.Keys(offer.Costs)) {
			a.Uses = id
			break
		}
		s.Actions = append(s.Actions, a)
	}
}

// tagged appends the action an entry's tag describes, if it has one.
func (s *State) tagged(origin rules.Ref, e catalog.Entry, category ActionCategory) {
	if e.Action == nil {
		return
	}
	s.Actions = append(s.Actions, Action{Source: Derived, Origin: origin, Kind: actionKind(e.Action.Kind), Category: category, Name: e.Name, Uses: e.Action.Uses})
}

// weaponAttack is the attack an equipped weapon makes.
//
// ponytail: the modifier and proficiency only. Fighting styles, magic bonuses,
// Martial Arts and the off-hand rule are not applied, because nothing in the
// catalogue states them as data yet; add them when items and features carry
// structured bonuses.
func weaponAttack(s *State, cat *catalog.Catalog, origin rules.Ref, it catalog.Item) Action {
	w := it.Weapon
	mod := s.Abilities.Modifier(rules.Strength)
	dex := s.Abilities.Modifier(rules.Dexterity)
	finesse := slices.ContainsFunc(w.Properties, func(p rules.Slug) bool { return localSlug(p) == "finesse" })
	if w.Range == catalog.RangedWeapon || (finesse && dex > mod) {
		mod = dex
	}
	toHit := mod
	if weaponProficient(s, cat, it.Slug) {
		toHit += s.Status.ProficiencyBonus
	}
	a := Action{Source: Derived, Origin: origin, Kind: MainAction, Category: ActionFromEquipment, Name: it.Name, ToHit: &toHit, Range: max(w.NormalRange, w.ThrowNormal)}
	if w.Damage != nil {
		damage := *w.Damage
		damage.Dice.Bonus += mod
		a.Damage = &damage
	}
	return a
}

// weaponProficient reports whether one of the character's proficiencies covers
// the weapon, by naming it or by naming a category it belongs to. It follows
// the references rather than comparing slugs, so it holds for a pack whose
// slugs are namespaced.
func weaponProficient(s *State, cat *catalog.Catalog, weapon rules.Slug) bool {
	for _, slug := range s.Proficiencies {
		def, ok := cat.Proficiencies.Get(slug)
		if !ok {
			continue
		}
		switch def.Reference.Kind {
		case rules.RefItem:
			if def.Reference.Slug == weapon {
				return true
			}
		case rules.RefEquipmentCategory:
			if category, ok := cat.EquipmentCategories.Get(def.Reference.Slug); ok && slices.Contains(category.Items, weapon) {
				return true
			}
		}
	}
	return false
}

// localSlug drops a pack namespace: "dnd-2014/finesse" is "finesse".
func localSlug(s rules.Slug) string {
	return s.String()[strings.LastIndex(s.String(), "/")+1:]
}

func actionKind(wire string) ActionKind {
	for kind, name := range actionKindNames {
		if name == wire {
			return kind
		}
	}
	return MainAction
}
