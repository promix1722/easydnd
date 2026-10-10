package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// countedName is an item written with its quantity: "10 Rations (1 day)",
// "2x Dagger", "Playing Card Set x2".
var countedName = regexp.MustCompile(`^(?:([0-9]+)\s*[x×]?\s+(.+)|(.+?)\s*[x×]\s*([0-9]+))$`)

// withAttackWeapons heads a sheet's items with the weapons it attacks with.
//
// The attacks table is the better witness of what is in hand, and often the
// only one: a sheet printing a longsword's to-hit and damage listed a rapier
// among its equipment and no sword at all, and an inventory copied from the
// equipment box alone left the character without the weapon it fights with.
// First in the list is first to a hand -- setInventory seats one item to a
// slot -- so the sword is wielded and the rapier carried. A row that names no
// catalogue weapon (a breath weapon, an unarmed strike) is not an item and is
// left to the checklist.
func (a *Agent) withAttackWeapons(ctx context.Context, s *AgentSession, cat *catalog.Catalog, items []agentArgs, attacks []string) []agentArgs {
	slugOf := func(item agentArgs) rules.Slug {
		fact, _, err := a.inventoryFact(ctx, s, cat, item)
		if err != nil {
			return ""
		}
		return rules.Slug(fact.Path[strings.LastIndex(fact.Path, ".")+1:])
	}
	head, seen := []agentArgs{}, map[rules.Slug]bool{}
	for _, name := range attacks {
		slug := slugOf(agentArgs{Name: name})
		if it, ok := cat.Items.Get(slug); !ok || it.Weapon == nil || seen[slug] {
			continue
		}
		seen[slug] = true
		wielded := agentArgs{Name: name, Count: 1}
		// The equipment box lists it too: that line, with its count, moves up.
		if at := slices.IndexFunc(items, func(item agentArgs) bool { return slugOf(item) == slug }); at >= 0 {
			wielded, items = items[at], slices.Delete(slices.Clone(items), at, at+1)
		}
		wielded.Placement = "equipped"
		head = append(head, wielded)
	}
	return append(head, items...)
}

// setInventory puts a sheet's items on the character, by printed name.
func (a *Agent) setInventory(ctx context.Context, s *AgentSession, cat *catalog.Catalog, items []agentArgs) map[string]any {
	// A sheet's inventory is the whole inventory: the kit a class grants by
	// default is on it already, or was spent long ago.
	if len(s.Files) > 0 {
		for _, placement := range []string{"equipped", "backpack", "loot"} {
			s.Log = clearImportedInventory(s.Log, placement)
		}
	}
	applied, unmatched := []map[string]any{}, []map[string]any{}
	// One item to a slot. A sheet lists every weapon it has as wielded, and
	// taking that at its word seated a barbarian's net, longsword, four
	// javelins and handaxe in one hand. The first thing the sheet calls worn
	// takes its slot -- so the armor class the sheet prints is still one the
	// build can reproduce -- and everything after it for that slot, and every
	// further copy of it, is carried.
	seated, wornOf := map[catalog.Slot]int{}, map[string]int{}
	// An equipment pack is carried as what is in it, as it is when a class
	// grants one. A sheet that names the pack often lists what was in it as
	// well, with the counts as they stand now, so a line of the sheet's own
	// is never overwritten by a pack's.
	listed, unpacked := map[rules.Slug]bool{}, map[rules.Slug]int{}
	for _, item := range items {
		fact, found, err := a.inventoryFact(ctx, s, cat, item)
		packed := false
		if err == nil {
			slug := rules.Slug(fact.Path[strings.LastIndex(fact.Path, ".")+1:])
			count, _ := strconv.Atoi(string(fact.Value))
			contents := packContents(cat, slug, count)
			if packed = len(contents) > 0; !packed {
				listed[slug] = true
			}
			for _, inside := range contents {
				if listed[inside.Item] || err != nil {
					continue
				}
				unpacked[inside.Item] += inside.Count
				_, err = a.importFact(ctx, s, cat, agentArgs{Path: "equipment.backpack." + inside.Item.String(), Value: json.RawMessage(strconv.Itoa(unpacked[inside.Item])), Source: item.Source})
			}
			if packed {
				fact.Path = "equipment.backpack." + slug.String()
			}
		}
		if slug, worn := strings.CutPrefix(fact.Path, "equipment.equipped."); err == nil && worn {
			slot, capacity := catalog.SlotNone, 1
			if it, ok := cat.Items.Get(rules.Slug(slug)); ok {
				slot = it.Slot
			} else if it, ok := cat.MagicItems.Get(rules.Slug(slug)); ok {
				slot = it.Slot
			}
			if slot == catalog.SlotRing {
				capacity = 2
			}
			count, _ := strconv.Atoi(string(fact.Value))
			// The same item on a second page of the sheet is the same item:
			// it keeps the seat it was given and takes no other.
			wear, again := wornOf[slug]
			if !again {
				if slot != catalog.SlotNone {
					wear = min(count, capacity-seated[slot])
				}
				seated[slot] += wear
				wornOf[slug] = wear
			}
			if wear == 0 {
				fact.Path = "equipment.backpack." + slug
			} else if count > wear {
				fact.Value = json.RawMessage(strconv.Itoa(wear))
				_, err = a.importFact(ctx, s, cat, agentArgs{Path: "equipment.backpack." + slug, Value: json.RawMessage(strconv.Itoa(count - wear)), Source: item.Source})
			}
		}
		if err == nil && !packed {
			_, err = a.importFact(ctx, s, cat, fact)
		}
		if err != nil {
			a.service.Logger().Warn("AI wizard tool rejected", "tool", "set_inventory", "item", item.Name, "error", err)
			failure := map[string]any{}
			_ = json.Unmarshal(agentError(err), &failure)
			failure["name"] = item.Name
			unmatched = append(unmatched, failure)
			continue
		}
		a.recordProgress(ctx, s, "set_inventory", agentArgs{Ref: found.Ref, Name: found.Name, Value: fact.Value}, nil)
		applied = append(applied, map[string]any{"name": item.Name, "ref": found.Ref, "count": fact.Value, "placement": strings.Split(fact.Path, ".")[1]})
	}
	return map[string]any{"applied": applied, "unmatched": unmatched, "next": "Resend an unmatched item by one of its candidate refs, or keep it with upsert_custom_option kind item, its count and placement."}
}

// packContents is what count of an equipment pack holds, packs inside it
// opened too, and nothing for an item that is not a pack.
func packContents(cat *catalog.Catalog, slug rules.Slug, count int) []catalog.ItemStack {
	it, ok := cat.Items.Get(slug)
	if !ok || it.Gear == nil {
		return nil
	}
	var out []catalog.ItemStack
	for _, inside := range it.Gear.Contents {
		n := max(inside.Count, 1) * max(count, 1)
		if nested := packContents(cat, inside.Item, n); len(nested) > 0 {
			out = append(out, nested...)
			continue
		}
		out = append(out, catalog.ItemStack{Item: inside.Item, Count: n})
	}
	return out
}

// inventoryFact turns one printed item into the stack it is: a catalogue item,
// how many, and where it is carried.
func (a *Agent) inventoryFact(ctx context.Context, s *AgentSession, cat *catalog.Catalog, item agentArgs) (agentArgs, AgentCandidate, error) {
	placement := item.Placement
	if placement == "" {
		placement = "backpack"
	}
	// A sheet says where a thing is in its own words. What is in hand or on
	// the body is equipped; anything else carried is in the pack.
	switch strings.ToLower(placement) {
	case "wielded", "held", "worn", "wearing", "in hand", "hands", "armor", "weapon":
		placement = "equipped"
	case "carried", "pack", "bag", "inventory", "stowed":
		placement = "backpack"
	}
	if !slices.Contains([]string{"equipped", "backpack", "loot"}, placement) {
		return agentArgs{}, AgentCandidate{}, fmt.Errorf("placement is equipped, backpack or loot")
	}
	// A sheet prints "10 Rations" or "Dagger x2", and a model copies the cell
	// whole. The number is the count unless one was given apart from it.
	name, count := item.Name, item.Count
	if match := countedName.FindStringSubmatch(strings.TrimSpace(name)); match != nil && count <= 1 {
		digits := match[1] + match[4]
		name = match[2] + match[3]
		count, _ = strconv.Atoi(digits)
	}
	if count == 0 {
		count = 1
	}
	if count < 1 || count > 100000 {
		return agentArgs{}, AgentCandidate{}, fmt.Errorf("invalid inventory quantity")
	}
	text := name
	if item.Ref != "" {
		text = item.Ref
	}
	found, err := a.resolve(ctx, s, cat, []string{"item", "magic-item"}, text, nil, false)
	if err != nil {
		return agentArgs{}, AgentCandidate{}, err
	}
	ref, _ := rules.ParseRef(found.Ref)
	return agentArgs{Path: "equipment." + placement + "." + ref.Slug.String(), Value: json.RawMessage(strconv.Itoa(count)), Source: item.Source}, found, nil
}

func clearImportedInventory(log domain.Log, placement string) domain.Log {
	if placement == "" {
		placement = "backpack"
	}
	if !slices.Contains([]string{"equipped", "backpack", "loot"}, placement) {
		return log
	}
	path := domain.Path("equipment." + placement)
	for _, event := range log.Events {
		for _, change := range event.Changes {
			if change.Path == path && change.Op == domain.OpSet {
				return log
			}
		}
	}
	updated := log.Clone()
	_ = updated.Append(domain.Event{Type: domain.EventChange, Observed: true, Changes: []domain.Change{{Path: path, Op: domain.OpSet, Value: domain.SlugListValue(nil)}}})
	return updated
}
