package main

import "github.com/promix1722/easydnd/internal/adapter/catalog/file"

// The slots a starting kit is asked by. Three are the catalogue's worn slots
// and are equipped on answer; the rest only name the card.
const (
	slotBody       = file.SlotBody
	slotMainHand   = file.SlotMainHand
	slotOffHand    = file.SlotOffHand
	slotBackup     = "backup"
	slotPack       = "pack"
	slotFocus      = "focus"
	slotInstrument = "instrument"
)

// A kitSlot is one question of a class's kit: either a short list of options
// or a whole equipment category.
type kitSlot struct {
	slot     string
	category string
	options  []file.Option
}

func pick(slot string, options ...file.Option) kitSlot { return kitSlot{slot: slot, options: options} }
func anyOf(slot, category string) kitSlot              { return kitSlot{slot: slot, category: category} }

func item(slug string, count int) file.Option {
	return file.Option{Kind: file.OptionRef, Ref: file.Ref("item:" + slug), Count: count}
}
func one(slug string) file.Option { return item(slug, 1) }
func bundle(key string, items ...file.Option) file.Option {
	return file.Option{Kind: file.OptionBundle, Key: key, Items: items}
}

// anyOne is "any one of this category" beside named items. It is written as
// a nested choice here and expanded by kit into the category's members, so
// the card is one flat list: a player picks a longsword, not "Martial
// Weapons" and then a longsword.
func anyOne(category string) file.Option {
	return file.Option{Kind: file.OptionNested, Choice: &file.Choice{
		Choose: 1, Kind: choiceKindFor["equipment"],
		From: file.OptionSet{Kind: file.OptionSetEquipmentCategory, Category: category},
	}}
}

var crossbow = bundle("", one("crossbow-light"), item("crossbow-bolt", 20))

// firstRef is the item an option is, or starts with: a bundle is its first
// item, the one that gives it its name.
func firstRef(option file.Option) file.Ref {
	if option.Ref != "" {
		return option.Ref
	}
	if len(option.Items) > 0 {
		return firstRef(option.Items[0])
	}
	return ""
}

// classKits is each SRD class's starting kit, asked slot by slot rather than
// in the book's "(a) … or (b) …" pairs. The SRD text is the source
// (docs/dnd.md says where this departs from it); the fixed items still come
// from upstream `startingEquipment`.
var classKits = map[string][]kitSlot{
	"barbarian": {
		pick(slotMainHand, one("greataxe"), anyOne("martial-melee-weapons")),
		pick(slotOffHand, one("handaxe"), anyOne("simple-weapons")),
	},
	"bard": {
		pick(slotMainHand, one("rapier"), one("longsword"), anyOne("simple-weapons")),
		pick(slotPack, one("diplomats-pack"), one("entertainers-pack")),
		pick(slotInstrument, one("lute"), anyOne("musical-instruments")),
	},
	"cleric": {
		pick(slotBody, one("scale-mail"), one("leather-armor"), one("chain-mail")),
		pick(slotMainHand, one("mace"), one("warhammer")),
		pick(slotBackup, crossbow, anyOne("simple-weapons")),
		pick(slotPack, one("priests-pack"), one("explorers-pack")),
		anyOf(slotFocus, "holy-symbols"),
	},
	"druid": {
		pick(slotMainHand, one("scimitar"), anyOne("simple-melee-weapons")),
		pick(slotOffHand, one("shield"), anyOne("simple-weapons")),
		anyOf(slotFocus, "druidic-foci"),
	},
	"fighter": {
		pick(slotBody, one("chain-mail"), one("leather-armor")),
		anyOf(slotMainHand, "martial-weapons"),
		pick(slotOffHand, one("shield"), anyOne("martial-weapons")),
		pick(slotBackup, crossbow, bundle("", one("longbow"), item("arrow", 20)), item("handaxe", 2)),
		pick(slotPack, one("dungeoneers-pack"), one("explorers-pack")),
	},
	"monk": {
		pick(slotMainHand, one("shortsword"), anyOne("simple-weapons")),
		pick(slotPack, one("dungeoneers-pack"), one("explorers-pack")),
	},
	"paladin": {
		anyOf(slotMainHand, "martial-weapons"),
		pick(slotOffHand, one("shield"), anyOne("martial-weapons")),
		pick(slotBackup, item("javelin", 5), anyOne("simple-weapons")),
		pick(slotPack, one("priests-pack"), one("explorers-pack")),
		anyOf(slotFocus, "holy-symbols"),
	},
	"ranger": {
		pick(slotBody, one("scale-mail"), one("leather-armor")),
		pick(slotMainHand, one("shortsword"), anyOne("simple-melee-weapons")),
		pick(slotOffHand, one("shortsword"), anyOne("simple-melee-weapons")),
		pick(slotPack, one("dungeoneers-pack"), one("explorers-pack")),
	},
	"rogue": {
		pick(slotMainHand, one("rapier"), one("shortsword")),
		// SRD 5.1 includes a quiver with these arrows; the key predates it.
		pick(slotBackup, bundle("shortbow+arrow", one("shortbow"), item("arrow", 20), one("quiver")), one("shortsword")),
		pick(slotPack, one("burglars-pack"), one("dungeoneers-pack"), one("explorers-pack")),
	},
	"sorcerer": {
		pick(slotMainHand, crossbow, anyOne("simple-weapons")),
		pick(slotFocus, one("component-pouch"), anyOne("arcane-foci")),
		pick(slotPack, one("dungeoneers-pack"), one("explorers-pack")),
	},
	"warlock": {
		pick(slotMainHand, crossbow, anyOne("simple-weapons")),
		anyOf(slotOffHand, "simple-weapons"),
		pick(slotFocus, one("component-pouch"), anyOne("arcane-foci")),
		pick(slotPack, one("scholars-pack"), one("dungeoneers-pack")),
	},
	"wizard": {
		pick(slotMainHand, one("quarterstaff"), one("dagger")),
		pick(slotFocus, one("component-pouch"), anyOne("arcane-foci")),
		pick(slotPack, one("scholars-pack"), one("explorers-pack")),
	},
}

// kit is a class's starting-equipment choices, one per slot, with prompt ids
// named by the slot: "fighter/starting-equipment/body". A category beside
// named items is spelled out as its members, minus any item the slot already
// names on its own -- five javelins beside "any simple melee weapon" is one
// javelin option, the five -- so a card is one list and never a sub-choice.
func (g *generator) kit(class string, categories map[string][]string) []file.Choice {
	slots, ok := classKits[class]
	if !ok {
		g.warnf("no starting kit for class %q", class)
		return nil
	}
	out := make([]file.Choice, 0, len(slots))
	for _, s := range slots {
		prompt := class + "/starting-equipment/" + s.slot
		c := file.Choice{Prompt: prompt, Choose: 1, Kind: choiceKindFor["equipment"], Slot: s.slot}
		if s.category != "" {
			c.From = file.OptionSet{Kind: file.OptionSetEquipmentCategory, Category: s.category}
		} else {
			c.From = file.OptionSet{Kind: file.OptionSetExplicit, Options: make([]file.Option, 0, len(s.options))}
			named := map[file.Ref]bool{}
			for _, option := range s.options {
				if ref := firstRef(option); ref != "" {
					named[ref] = true
				}
			}
			for _, option := range s.options {
				if option.Choice == nil {
					c.From.Options = append(c.From.Options, option)
					continue
				}
				members, ok := categories[option.Choice.From.Category]
				if !ok {
					g.warnf("%s: unknown equipment category %q", prompt, option.Choice.From.Category)
				}
				for _, slug := range members {
					if !named[file.Ref("item:"+slug)] {
						c.From.Options = append(c.From.Options, one(slug))
					}
				}
			}
		}
		out = append(out, c)
	}
	return out
}
