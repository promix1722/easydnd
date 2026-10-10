package game_test

import (
	"context"
	"reflect"
	"sync"
	"testing"

	"github.com/promix1722/easydnd/internal/adapter/repository/repotest"
	"github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/domain/user"
	gameuc "github.com/promix1722/easydnd/internal/usecase/game"
)

func ptr[T any](v T) *T { return &v }

func TestTrackerPermissionsAndIndependentValues(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.table(t, "table", "alice", map[user.ID]group.Role{"bob": group.RolePlayer, "carol": group.RolePlayer})
	cid := f.character(t, "bob")
	if err := f.svc.Share(ctx, "bob", "table", cid); err != nil {
		t.Fatal(err)
	}
	g, err := f.svc.Create(ctx, "alice", "table", "Game")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.AddCharacters(ctx, "alice", g.ID, []character.ID{cid}); err != nil {
		t.Fatal(err)
	}
	original, _ := f.characters.Get(ctx, cid)
	entries, err := f.svc.Participants(ctx, "bob", g.ID, rules.DefaultLocale)
	if err != nil || len(entries) != 1 || !entries[0].CanEdit {
		t.Fatalf("entries: %+v, %v", entries, err)
	}
	eid := entries[0].Entry.ID
	tags := []string{" prone ", "prone", "concentrating"}
	patch := gameuc.EntryPatch{HP: ptr(7), TempHP: ptr(4), InitiativeSet: true, Initiative: ptr(-1), Tags: &tags}
	if err := f.svc.PatchEntry(ctx, "bob", g.ID, eid, patch); err != nil {
		t.Fatal(err)
	}
	assertDenied(t, f.svc.PatchEntry(ctx, "carol", g.ID, eid, patch), "other player")
	assertDenied(t, f.svc.PatchEntry(ctx, "bob", g.ID, eid, gameuc.EntryPatch{Locked: ptr(true)}), "player locking")
	assertDenied(t, f.svc.OrderEntries(ctx, "bob", g.ID, "", 0, true), "player sorting")
	if err := f.svc.PatchEntry(ctx, "alice", g.ID, eid, gameuc.EntryPatch{Locked: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	assertDenied(t, f.svc.PatchEntry(ctx, "bob", g.ID, eid, patch), "locked owner")
	if err := f.svc.PatchEntry(ctx, "alice", g.ID, eid, gameuc.EntryPatch{HP: ptr(8)}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.PatchEntry(ctx, "alice", g.ID, eid, gameuc.EntryPatch{Locked: ptr(false)}); err != nil {
		t.Fatal(err)
	}
	// Concurrent updates touch independent fields and must both survive.
	var wg sync.WaitGroup
	for _, p := range []gameuc.EntryPatch{{HP: ptr(9)}, {TempHP: ptr(5)}} {
		wg.Add(1)
		go func(p gameuc.EntryPatch) {
			defer wg.Done()
			if err := f.svc.PatchEntry(ctx, "bob", g.ID, eid, p); err != nil {
				t.Error(err)
			}
		}(p)
	}
	wg.Wait()
	entries, _ = f.svc.Participants(ctx, "bob", g.ID, rules.DefaultLocale)
	e := entries[0].Entry
	if e.HP != 9 || e.TempHP != 5 || *e.Initiative != -1 || !reflect.DeepEqual(e.Tags, []string{"prone", "concentrating"}) {
		t.Fatalf("values: %+v", e)
	}
	after, _ := f.characters.Get(ctx, cid)
	if !reflect.DeepEqual(original.Log, after.Log) {
		t.Fatal("tracker modified original log")
	}
	// The same character gets independent game values in another game.
	second, _ := f.svc.Create(ctx, "alice", "table", "Other")
	if err := f.svc.AddCharacters(ctx, "alice", second.ID, []character.ID{cid}); err != nil {
		t.Fatal(err)
	}
	other, _ := f.svc.Participants(ctx, "bob", second.ID, rules.DefaultLocale)
	if other[0].Entry.HP == 9 || other[0].Entry.TempHP != 0 || other[0].Entry.Initiative != nil {
		t.Fatal("values leaked between games")
	}
	// Base stats still follow the original sheet, while tracker HP stays put.
	if err := repotest.Append(ctx, f.characters, cid, character.Event{Type: character.EventInit}, character.Event{Type: character.EventChange,
		Changes: []character.Change{{Path: "status.armorClass", Op: character.OpSet, Value: character.IntValue(18)}}}); err != nil {
		t.Fatal(err)
	}
	entries, err = f.svc.Participants(ctx, "bob", g.ID, rules.DefaultLocale)
	if err != nil || entries[0].Stats.ArmorClass != 18 || entries[0].Entry.HP != 9 {
		t.Fatalf("live stats: %+v, %v", entries, err)
	}
}

func TestPrivateMonstersAndStableOrdering(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.table(t, "table", "alice", map[user.ID]group.Role{"bob": group.RolePlayer})
	source := f.character(t, "alice")
	if err := repotest.Append(ctx, f.characters, source, character.Event{Type: character.EventInit},
		character.Event{Type: character.EventClass, Ref: rules.NewRef(rules.RefClass, "rogue"), Level: 1}); err != nil {
		t.Fatal(err)
	}
	g, _ := f.svc.Create(ctx, "alice", "table", "Game")
	for range 2 {
		if err := f.svc.AddMonster(ctx, "alice", g.ID, source, rules.DefaultLocale); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.svc.AddMonster(ctx, "alice", g.ID, "", rules.DefaultLocale); err != nil {
		t.Fatal(err)
	}
	assertDenied(t, f.svc.AddMonster(ctx, "bob", g.ID, "", rules.DefaultLocale), "player monster creation")
	table, _ := f.svc.SharedCharacters(ctx, "bob", "table", rules.DefaultLocale)
	if len(table) != 0 {
		t.Fatal("monster source was shared")
	}
	monsters, _ := f.svc.Participants(ctx, "alice", g.ID, rules.DefaultLocale)
	if len(monsters) != 3 || monsters[0].Entry.ID == monsters[1].Entry.ID {
		t.Fatal("duplicate copies missing")
	}
	if monsters[0].Stats.Class != "rogue" || monsters[1].Stats.Class != "rogue" || monsters[2].Stats.Class != "" {
		t.Fatal("copies must preserve the starting class while stubs have no class")
	}
	if monsters[2].Entry.HP != 10 || monsters[2].Stats.MaxHP != 10 {
		t.Fatal("stub should start at 10/10 HP")
	}
	stats := *monsters[0].Stats
	stats.Name, stats.ArmorClass = "Secret beast", 19
	tags := []string{"secret"}
	if err := f.svc.PatchEntry(ctx, "alice", g.ID, monsters[0].Entry.ID, gameuc.EntryPatch{Stats: &gameuc.StatsPatch{Name: &stats.Name, ArmorClass: &stats.ArmorClass}, Tags: &tags, InitiativeSet: true, Initiative: ptr(15)}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.PatchEntry(ctx, "alice", g.ID, monsters[1].Entry.ID, gameuc.EntryPatch{InitiativeSet: true, Initiative: ptr(15)}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.OrderEntries(ctx, "alice", g.ID, monsters[1].Entry.ID, -1, false); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.OrderEntries(ctx, "alice", g.ID, "", 0, true); err != nil {
		t.Fatal(err)
	}
	ordered, _ := f.svc.Participants(ctx, "alice", g.ID, rules.DefaultLocale)
	if ordered[0].Entry.ID != monsters[1].Entry.ID || ordered[1].Entry.ID != monsters[0].Entry.ID || ordered[2].Entry.Initiative != nil {
		t.Fatal("sort did not preserve ties and put unset last")
	}
	public, _ := f.svc.Participants(ctx, "bob", g.ID, rules.DefaultLocale)
	if public[0].Stats.Class != "rogue" || public[1].Stats.Class != "rogue" {
		t.Fatal("public portraits lost their class")
	}
	for _, p := range public {
		if p.CanEdit || p.Entry.Monster != nil || p.Entry.Character != "" || p.Entry.HP != 0 || p.Entry.Initiative != nil || len(p.Entry.Tags) != 0 || p.Stats.ArmorClass != 0 {
			t.Fatal("private monster data leaked")
		}
	}
	if err := f.characters.Delete(ctx, source); err != nil {
		t.Fatal(err)
	}
	remaining, _ := f.svc.Participants(ctx, "alice", g.ID, rules.DefaultLocale)
	if len(remaining) != 3 {
		t.Fatal("source deletion removed copied monsters")
	}
	if err := f.svc.DeleteEntry(ctx, "alice", g.ID, monsters[0].Entry.ID); err != nil {
		t.Fatal(err)
	}
	remaining, _ = f.svc.Participants(ctx, "alice", g.ID, rules.DefaultLocale)
	if len(remaining) != 2 {
		t.Fatal("monster not deleted")
	}
}

func TestDraggingMovesOnlyOneEntryAndPreservesTheRoster(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.table(t, "table", "alice", map[user.ID]group.Role{"bob": group.RolePlayer})
	g, _ := f.svc.Create(ctx, "alice", "table", "Game")
	for range 3 {
		if err := f.svc.AddMonster(ctx, "alice", g.ID, "", rules.DefaultLocale); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := f.svc.Participants(ctx, "alice", g.ID, rules.DefaultLocale)
	a, b, c := entries[0].Entry.ID, entries[1].Entry.ID, entries[2].Entry.ID
	check := func(want []string) {
		t.Helper()
		got, err := f.svc.Participants(ctx, "alice", g.ID, rules.DefaultLocale)
		if err != nil {
			t.Fatal(err)
		}
		ids := make([]string, len(got))
		for i, e := range got {
			ids[i] = e.Entry.ID
		}
		if !reflect.DeepEqual(ids, want) {
			t.Fatalf("order = %v, want %v", ids, want)
		}
	}
	assertDenied(t, f.svc.MoveEntryBefore(ctx, "bob", g.ID, c, a), "player drag")
	if err := f.svc.MoveEntryBefore(ctx, "alice", g.ID, c, a); err != nil {
		t.Fatal(err)
	}
	check([]string{c, a, b})
	if err := f.svc.MoveEntryBefore(ctx, "alice", g.ID, c, ""); err != nil {
		t.Fatal(err)
	}
	check([]string{a, b, c})
	if err := f.svc.MoveEntryBefore(ctx, "alice", g.ID, a, c); err != nil {
		t.Fatal(err)
	}
	check([]string{b, a, c})
	if err := f.svc.MoveEntryBefore(ctx, "alice", g.ID, a, a); err != nil {
		t.Fatal(err)
	}
	check([]string{b, a, c})
	assertNotFound(t, f.svc.MoveEntryBefore(ctx, "alice", g.ID, a, "missing"), "missing target")
	check([]string{b, a, c})
	assertNotFound(t, f.svc.MoveEntryBefore(ctx, "alice", g.ID, "missing", b), "missing source")
	check([]string{b, a, c})
	// An entry added after the client's last view stays in the list.
	if err := f.svc.AddMonster(ctx, "alice", g.ID, "", rules.DefaultLocale); err != nil {
		t.Fatal(err)
	}
	latest, _ := f.svc.Participants(ctx, "alice", g.ID, rules.DefaultLocale)
	d := latest[3].Entry.ID
	if err := f.svc.MoveEntryBefore(ctx, "alice", g.ID, c, b); err != nil {
		t.Fatal(err)
	}
	check([]string{c, b, a, d})
}

// A short rest returns what the catalogue says a short rest refills -- a
// fighter's Second Wind -- and leaves the rest spent: Hit Dice need a long one.
func TestAShortRestReturnsOnlyShortRestPools(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.table(t, "table", "alice", map[user.ID]group.Role{"bob": group.RolePlayer})
	cid := f.character(t, "bob")
	if err := repotest.Append(ctx, f.characters, cid, character.Event{Type: character.EventInit},
		character.Event{Type: character.EventClass, Ref: rules.NewRef(rules.RefClass, "fighter"), Level: 5}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Share(ctx, "bob", "table", cid); err != nil {
		t.Fatal(err)
	}
	g, _ := f.svc.Create(ctx, "alice", "table", "Game")
	if err := f.svc.AddCharacters(ctx, "alice", g.ID, []character.ID{cid}); err != nil {
		t.Fatal(err)
	}
	// Another player at the table sees the card and none of what it has left to spend.
	if err := f.groups.AddMember(ctx, "table", "carol", group.RolePlayer, at(3)); err != nil {
		t.Fatal(err)
	}
	for who, want := range map[user.ID]bool{"alice": true, "bob": true, "carol": false} {
		seen, err := f.svc.Participants(ctx, who, g.ID, rules.DefaultLocale)
		if err != nil || len(seen) != 1 {
			t.Fatalf("%s: entries %+v, %v", who, seen, err)
		}
		if got := len(seen[0].Pools) > 0; got != want {
			t.Fatalf("%s sees pools = %v, want %v", who, got, want)
		}
	}
	used := func() map[string]int {
		t.Helper()
		entries, err := f.svc.Participants(ctx, "alice", g.ID, rules.DefaultLocale)
		if err != nil || len(entries) != 1 {
			t.Fatalf("entries: %+v, %v", entries, err)
		}
		out := map[string]int{}
		for _, p := range entries[0].Pools {
			out[string(p.ID)] = p.Used
		}
		return out
	}
	entries, _ := f.svc.Participants(ctx, "alice", g.ID, rules.DefaultLocale)
	spent := map[string]int{"second-wind": 1, "hit-dice/fighter": 1}
	if err := f.svc.PatchEntry(ctx, "alice", g.ID, entries[0].Entry.ID, gameuc.EntryPatch{Used: spent}); err != nil {
		t.Fatal(err)
	}
	assertDenied(t, f.svc.Rest(ctx, "bob", g.ID, true), "player calling a short rest")
	if err := f.svc.Rest(ctx, "alice", g.ID, true); err != nil {
		t.Fatal(err)
	}
	if got := used(); got["second-wind"] != 0 || got["hit-dice/fighter"] != 1 {
		t.Fatalf("after short rest: %+v", got)
	}

	// A plain-number scaling value -- a fifth-level fighter's one Extra Attack
	// -- is spent like a pool, to its number and no further. It has no
	// recovery of its own, so only a long rest gives it back.
	spend := func(n int) error {
		return f.svc.PatchEntry(ctx, "alice", g.ID, entries[0].Entry.ID, gameuc.EntryPatch{Used: map[string]int{"scaling/extra-attacks": n}})
	}
	if err := spend(2); err == nil {
		t.Fatal("spent more Extra Attacks than the character has")
	}
	if err := spend(1); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Rest(ctx, "alice", g.ID, true); err != nil {
		t.Fatal(err)
	}
	if got := used(); got["scaling/extra-attacks"] != 1 {
		t.Fatalf("a short rest returned a scaling value: %+v", got)
	}
	if err := f.svc.Rest(ctx, "alice", g.ID, false); err != nil {
		t.Fatal(err)
	}
	if got := used(); got["scaling/extra-attacks"] != 0 || got["hit-dice/fighter"] != 0 {
		t.Fatalf("after long rest: %+v", got)
	}
}

func TestSpentUsesBelongToTheGameAndALongRestReturnsThem(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.table(t, "table", "alice", map[user.ID]group.Role{"bob": group.RolePlayer, "carol": group.RolePlayer})
	cid := f.character(t, "bob")
	if err := repotest.Append(ctx, f.characters, cid, character.Event{Type: character.EventInit},
		character.Event{Type: character.EventClass, Ref: rules.NewRef(rules.RefClass, "paladin"), Level: 3}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Share(ctx, "bob", "table", cid); err != nil {
		t.Fatal(err)
	}
	g, _ := f.svc.Create(ctx, "alice", "table", "Game")
	if err := f.svc.AddCharacters(ctx, "alice", g.ID, []character.ID{cid}); err != nil {
		t.Fatal(err)
	}
	original, _ := f.characters.Get(ctx, cid)
	pools := func() map[string][2]int {
		t.Helper()
		entries, err := f.svc.Participants(ctx, "bob", g.ID, rules.DefaultLocale)
		if err != nil || len(entries) != 1 {
			t.Fatalf("entries: %+v, %v", entries, err)
		}
		out := map[string][2]int{}
		for _, p := range entries[0].Pools {
			out[string(p.ID)] = [2]int{p.Used, p.Max}
		}
		return out
	}
	// Spell slots first, hit dice last, and the pool this change declared.
	entries, _ := f.svc.Participants(ctx, "bob", g.ID, rules.DefaultLocale)
	listed := entries[0].Pools
	if listed[0].ID != "spell-slots/1" || listed[len(listed)-1].ID != "hit-dice/paladin" || pools()["lay-on-hands"] != [2]int{0, 15} {
		t.Fatalf("pools: %+v", listed)
	}
	eid := entries[0].Entry.ID
	spend := func(actor user.ID, used map[string]int) error {
		return f.svc.PatchEntry(ctx, actor, g.ID, eid, gameuc.EntryPatch{Used: used})
	}
	if err := spend("bob", map[string]int{"spell-slots/1": 2}); err != nil {
		t.Fatal(err)
	}
	if err := spend("alice", map[string]int{"hit-dice/paladin": 1}); err != nil {
		t.Fatal(err)
	}
	assertDenied(t, spend("carol", map[string]int{"spell-slots/1": 0}), "other player")
	for name, used := range map[string]map[string]int{"over capacity": {"spell-slots/1": 4}, "negative": {"lay-on-hands": -1}, "unknown pool": {"ki-points": 1}} {
		if err := spend("bob", used); err == nil {
			t.Fatalf("%s was accepted", name)
		}
	}
	if got := pools(); got["spell-slots/1"] != [2]int{2, 3} || got["hit-dice/paladin"] != [2]int{1, 3} {
		t.Fatalf("spent: %+v", got)
	}
	assertDenied(t, f.svc.Rest(ctx, "bob", g.ID, false), "player calling a rest")
	if err := f.svc.Rest(ctx, "alice", g.ID, false); err != nil {
		t.Fatal(err)
	}
	if got := pools(); got["spell-slots/1"] != [2]int{0, 3} || got["hit-dice/paladin"] != [2]int{0, 3} {
		t.Fatalf("after rest: %+v", got)
	}
	after, _ := f.characters.Get(ctx, cid)
	if !reflect.DeepEqual(original.Log, after.Log) {
		t.Fatal("tracker modified original log")
	}
}
