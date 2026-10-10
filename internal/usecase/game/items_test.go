package game_test

import (
	"context"
	"testing"

	"github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/domain/user"
)

// The table hands things over: the DM gives out items and coins, a player
// passes on what is theirs, and nobody takes what is somebody else's.
func TestTheTableHandsOverItemsAndCoins(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.table(t, "table", "alice", map[user.ID]group.Role{"bob": group.RolePlayer, "carol": group.RolePlayer})
	bobs, carols := f.character(t, "bob"), f.character(t, "carol")
	for owner, cid := range map[user.ID]character.ID{"bob": bobs, "carol": carols} {
		if err := f.characters.Append(ctx, cid, 0, character.Event{Type: character.EventInit}); err != nil {
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
