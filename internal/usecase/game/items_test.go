package game_test

import (
	"context"
	"testing"

	"github.com/promix1722/easydnd/internal/adapter/repository/repotest"
	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// The table hands things over: the DM gives out items and coins, a player
// passes on what is theirs, and nobody takes what is somebody else's.
func TestTheTableHandsOverItemsAndCoins(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.table(t, "table", "alice", map[user.ID]group.Role{"bob": group.RolePlayer, "carol": group.RolePlayer})
	bobs, carols := f.character(t, "bob"), f.character(t, "carol")
	for owner, cid := range map[user.ID]character.ID{"bob": bobs, "carol": carols} {
		if err := repotest.Append(ctx, f.characters, cid, character.Event{Type: character.EventInit}); err != nil {
			t.Fatal(err)
		}
		if err := f.svc.Share(ctx, owner, "table", cid); err != nil {
			t.Fatal(err)
		}
	}
	g, err := f.svc.Create(ctx, "alice", "table", "Game")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.AddCharacters(ctx, "alice", g.ID, []character.ID{bobs, carols}); err != nil {
		t.Fatal(err)
	}
	bob, carol := "pc_"+string(bobs), "pc_"+string(carols)
	carrying := func(owner user.ID, cid character.ID, slug rules.Slug) int {
		t.Helper()
		state, err := f.svc.Sheet(ctx, owner, cid, rules.DefaultLocale)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, stack := range state.Equipment.Backpack {
			if stack.Item == slug {
				n += stack.Count
			}
		}
		return n
	}
	gold := func(owner user.ID, cid character.ID) int {
		t.Helper()
		state, err := f.svc.Sheet(ctx, owner, cid, rules.DefaultLocale)
		if err != nil {
			t.Fatal(err)
		}
		return state.Equipment.Purse[rules.Gold]
	}

	assertDenied(t, f.svc.GrantItem(ctx, "bob", g.ID, bob, "dagger", 1), "player granting")
	assertDenied(t, f.svc.AdjustCoins(ctx, "bob", g.ID, bob, rules.Gold, 5), "player minting")
	if err := f.svc.GrantItem(ctx, "alice", g.ID, bob, "no-such-item", 1); err == nil {
		t.Fatal("granted an item no catalogue has")
	}
	if err := f.svc.GrantItem(ctx, "alice", g.ID, bob, "dagger", 3); err != nil {
		t.Fatal(err)
	}
	if got := carrying("bob", bobs, "dagger"); got != 3 {
		t.Fatalf("bob carries %d daggers, want 3", got)
	}

	if err := f.svc.AdjustCoins(ctx, "alice", g.ID, bob, rules.Gold, 10); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.AdjustCoins(ctx, "alice", g.ID, bob, rules.Gold, -4); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.AdjustCoins(ctx, "alice", g.ID, bob, rules.Gold, -7); err == nil {
		t.Fatal("took more gold than the purse holds")
	}
	if got := gold("bob", bobs); got != 6 {
		t.Fatalf("bob has %d gp, want 6", got)
	}

	assertDenied(t, f.svc.GiveItem(ctx, "carol", g.ID, bob, carol, "dagger", 1), "giving away somebody else's")
	if err := f.svc.GiveItem(ctx, "bob", g.ID, bob, carol, "dagger", 4); err == nil {
		t.Fatal("gave more than is carried")
	}
	if err := f.svc.GiveItem(ctx, "bob", g.ID, bob, bob, "dagger", 1); err == nil {
		t.Fatal("gave to itself")
	}
	if b, c := carrying("bob", bobs, "dagger"), carrying("carol", carols, "dagger"); b != 3 || c != 0 {
		t.Fatalf("a refused gift moved something: bob %d, carol %d", b, c)
	}
	if err := f.svc.GiveItem(ctx, "bob", g.ID, bob, carol, "dagger", 2); err != nil {
		t.Fatal(err)
	}
	if b, c := carrying("bob", bobs, "dagger"), carrying("carol", carols, "dagger"); b != 1 || c != 2 {
		t.Fatalf("after the gift bob has %d and carol %d, want 1 and 2", b, c)
	}
}

// A custom item is handed over by the same rule as a catalogue one: whoever
// runs the game, to a character seated at it, as one new entry in the backpack.
func TestOnlyTheTableGrantsACustomItem(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.table(t, "table", "alice", map[user.ID]group.Role{"bob": group.RolePlayer})
	seated := f.character(t, "bob")
	if err := repotest.Append(ctx, f.characters, seated, character.Event{Type: character.EventInit},
		character.Event{Type: character.EventClass, Ref: rules.NewRef(rules.RefClass, "fighter"), Level: 1}); err != nil {
		t.Fatal(err)
	}
	g, err := f.svc.Create(ctx, "alice", "table", "Game")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Share(ctx, "bob", "table", seated); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.AddCharacters(ctx, "alice", g.ID, []character.ID{seated}); err != nil {
		t.Fatal(err)
	}
	entries, err := f.svc.Participants(ctx, "alice", g.ID, rules.DefaultLocale)
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries: %+v, %v", entries, err)
	}
	entry := entries[0].Entry.ID
	grant := func(actor user.ID, entry string) error {
		return f.svc.GrantCustomItem(ctx, actor, g.ID, entry, rules.DefaultLocale, "Moon Shield", "", &character.CustomItem{Armor: &catalog.Armor{Category: catalog.Shield, BaseAC: 2}})
	}
	before, _ := f.characters.Get(ctx, seated)
	if err := grant("alice", entry); err != nil {
		t.Fatalf("the DM grants: %v", err)
	}
	after, _ := f.characters.Get(ctx, seated)
	if after.Log.Len() != before.Log.Len()+1 || after.Revision != before.Revision+1 {
		t.Errorf("granting wrote %d events over %d revisions, want one of each", after.Log.Len()-before.Log.Len(), after.Revision-before.Revision)
	}
	assertDenied(t, grant("bob", entry), "a player")
	assertNotFound(t, grant("alice", "no-such-entry"), "an entry not at the game")

	// The DM reads what was given, and can hand its owner another catalogue item beside it.
	if err := f.svc.GrantItem(ctx, "alice", g.ID, entry, "torch", 1); err != nil {
		t.Fatalf("a catalogue item beside a custom one: %v", err)
	}
	state, err := f.svc.Sheet(ctx, "alice", seated, rules.DefaultLocale)
	if err != nil || len(state.CustomOptions) != 1 || len(state.Equipment.Backpack) != 2 {
		t.Fatalf("the DM's read of what was given: %+v, %v", state.Equipment, err)
	}
	l := types.DefaultLimits
	l.CharacterCustomOptions = 1
	f.svc.SetLimits(l)
	wantLimit(t, grant("alice", entry), "characterCustomOptions")
}
