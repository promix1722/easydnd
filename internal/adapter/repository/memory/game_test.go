package memory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/domain/character"
	domain "github.com/promix1722/easydnd/internal/domain/game"
	"github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/types"
)

// The assignments are the conformance proof: a drift from either port fails to
// compile here rather than at wiring time.
var (
	_ domain.SharedRepository = (*memory.SharedRepository)(nil)
	_ domain.Repository       = (*memory.GameRepository)(nil)
)

// at stamps a whole second, so comparisons do not depend on monotonic clock
// readings surviving a round trip through the store.
func at(sec int64) time.Time { return time.Unix(sec, 0).UTC() }

func shared(g group.ID, c character.ID, sec int64) domain.Shared {
	return domain.Shared{Group: g, Character: c, Owner: "acct-a", SharedAt: at(sec)}
}

func TestAPoolListsInTheOrderCharactersWereShared(t *testing.T) {
	repo := memory.NewSharedRepository()
	ctx := context.Background()

	for i, c := range []character.ID{"chr_3", "chr_1", "chr_2"} {
		if err := repo.Share(ctx, shared("grp_a", c, int64(i))); err != nil {
			t.Fatalf("Share(%q) error = %v", c, err)
		}
	}

	got, err := repo.List(ctx, "grp_a")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	want := []character.ID{"chr_3", "chr_1", "chr_2"}
	if len(got) != len(want) {
		t.Fatalf("List() returned %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Character != want[i] {
			t.Errorf("List()[%d] = %q, want %q", i, got[i].Character, want[i])
		}
	}
}

func TestTheSameCharacterCannotBeSharedTwiceWithOneGroup(t *testing.T) {
	repo := memory.NewSharedRepository()
	ctx := context.Background()

	if err := repo.Share(ctx, shared("grp_a", "chr_1", 1)); err != nil {
		t.Fatalf("Share() error = %v", err)
	}
	err := repo.Share(ctx, shared("grp_a", "chr_1", 2))
	var invalid *types.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("second Share() error = %v, want *types.ValidationError", err)
	}

	// A second group is a different table, and must be unaffected.
	if err := repo.Share(ctx, shared("grp_b", "chr_1", 3)); err != nil {
		t.Fatalf("Share() into a second group error = %v", err)
	}
}

func TestGroupsSharingFindsEveryTableACharacterIsOn(t *testing.T) {
	repo := memory.NewSharedRepository()
	ctx := context.Background()

	for _, g := range []group.ID{"grp_b", "grp_a"} {
		if err := repo.Share(ctx, shared(g, "chr_1", 1)); err != nil {
			t.Fatalf("Share() error = %v", err)
		}
	}
	if err := repo.Share(ctx, shared("grp_c", "chr_2", 1)); err != nil {
		t.Fatalf("Share() error = %v", err)
	}

	got, err := repo.GroupsSharing(ctx, "chr_1")
	if err != nil {
		t.Fatalf("GroupsSharing() error = %v", err)
	}
	// Sorted, so the answer does not depend on map iteration order.
	if len(got) != 2 || got[0] != "grp_a" || got[1] != "grp_b" {
		t.Errorf("GroupsSharing() = %v, want [grp_a grp_b]", got)
	}
}

func TestDeletingACharacterTakesItOffEveryTable(t *testing.T) {
	repo := memory.NewSharedRepository()
	ctx := context.Background()

	for _, g := range []group.ID{"grp_a", "grp_b"} {
		if err := repo.Share(ctx, shared(g, "chr_1", 1)); err != nil {
			t.Fatalf("Share() error = %v", err)
		}
	}
	if err := repo.Share(ctx, shared("grp_a", "chr_2", 2)); err != nil {
		t.Fatalf("Share() error = %v", err)
	}

	if err := repo.UnshareEverywhere(ctx, "chr_1"); err != nil {
		t.Fatalf("UnshareEverywhere() error = %v", err)
	}

	for _, g := range []group.ID{"grp_a", "grp_b"} {
		ok, err := repo.IsShared(ctx, g, "chr_1")
		if err != nil {
			t.Fatalf("IsShared() error = %v", err)
		}
		if ok {
			t.Errorf("chr_1 is still shared with %q", g)
		}
	}
	// The neighbour it was sharing a pool with must survive.
	ok, err := repo.IsShared(ctx, "grp_a", "chr_2")
	if err != nil {
		t.Fatalf("IsShared() error = %v", err)
	}
	if !ok {
		t.Error("UnshareEverywhere() removed a character it was not asked about")
	}
}

func TestARosterKeepsTheOrderCharactersWereAdded(t *testing.T) {
	repo := memory.NewGameRepository()
	ctx := context.Background()
	newGame(t, repo, "gam_1", "grp_a", 1)

	if err := repo.AddCharacters(ctx, "gam_1", []character.ID{"chr_2", "chr_1"}, at(2)); err != nil {
		t.Fatalf("AddCharacters() error = %v", err)
	}
	if err := repo.AddCharacters(ctx, "gam_1", []character.ID{"chr_3"}, at(3)); err != nil {
		t.Fatalf("AddCharacters() error = %v", err)
	}

	roster, err := repo.Characters(ctx, "gam_1")
	if err != nil {
		t.Fatalf("Characters() error = %v", err)
	}
	want := []character.ID{"chr_2", "chr_1", "chr_3"}
	if len(roster) != len(want) {
		t.Fatalf("Characters() returned %d entries, want %d", len(roster), len(want))
	}
	for i := range want {
		if roster[i].Character != want[i] {
			t.Errorf("Characters()[%d] = %q, want %q", i, roster[i].Character, want[i])
		}
	}
}

func TestAddingSomebodyAlreadySeatedChangesNothing(t *testing.T) {
	repo := memory.NewGameRepository()
	ctx := context.Background()
	newGame(t, repo, "gam_1", "grp_a", 1)

	if err := repo.AddCharacters(ctx, "gam_1", []character.ID{"chr_1"}, at(2)); err != nil {
		t.Fatalf("AddCharacters() error = %v", err)
	}
	// Adding everybody when most are already seated is the common case: it
	// must not duplicate a seat or re-stamp the one already taken.
	if err := repo.AddCharacters(ctx, "gam_1", []character.ID{"chr_1", "chr_2"}, at(9)); err != nil {
		t.Fatalf("AddCharacters() error = %v", err)
	}

	roster, err := repo.Characters(ctx, "gam_1")
	if err != nil {
		t.Fatalf("Characters() error = %v", err)
	}
	if len(roster) != 2 {
		t.Fatalf("Characters() returned %d entries, want 2", len(roster))
	}
	if !roster[0].AddedAt.Equal(at(2)) {
		t.Errorf("re-adding chr_1 re-stamped it to %v, want %v", roster[0].AddedAt, at(2))
	}
}

func TestUnsharingClearsTheSeatInEveryGameAtThatTable(t *testing.T) {
	repo := memory.NewGameRepository()
	ctx := context.Background()
	newGame(t, repo, "gam_1", "grp_a", 1)
	newGame(t, repo, "gam_2", "grp_a", 2)
	newGame(t, repo, "gam_3", "grp_b", 3)

	for _, id := range []domain.ID{"gam_1", "gam_2", "gam_3"} {
		if err := repo.AddCharacters(ctx, id, []character.ID{"chr_1"}, at(4)); err != nil {
			t.Fatalf("AddCharacters(%q) error = %v", id, err)
		}
	}

	if err := repo.RemoveFromGroupGames(ctx, "grp_a", "chr_1"); err != nil {
		t.Fatalf("RemoveFromGroupGames() error = %v", err)
	}

	for _, id := range []domain.ID{"gam_1", "gam_2"} {
		roster, err := repo.Characters(ctx, id)
		if err != nil {
			t.Fatalf("Characters(%q) error = %v", id, err)
		}
		if len(roster) != 0 {
			t.Errorf("game %q still seats %d characters, want 0", id, len(roster))
		}
	}
	// The other table is a different group and must be untouched.
	roster, err := repo.Characters(ctx, "gam_3")
	if err != nil {
		t.Fatalf("Characters() error = %v", err)
	}
	if len(roster) != 1 {
		t.Errorf("a game at another table lost its roster: %d entries, want 1", len(roster))
	}
}

func TestGamesComeBackNewestFirst(t *testing.T) {
	repo := memory.NewGameRepository()
	ctx := context.Background()
	newGame(t, repo, "gam_1", "grp_a", 1)
	newGame(t, repo, "gam_2", "grp_a", 3)
	newGame(t, repo, "gam_3", "grp_b", 2)

	got, err := repo.ListFor(ctx, "grp_a")
	if err != nil {
		t.Fatalf("ListFor() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListFor() returned %d games, want 2", len(got))
	}
	if got[0].ID != "gam_2" || got[1].ID != "gam_1" {
		t.Errorf("ListFor() = [%s %s], want [gam_2 gam_1]", got[0].ID, got[1].ID)
	}
}

func TestDeletingAGameTakesItsRosterWithIt(t *testing.T) {
	repo := memory.NewGameRepository()
	ctx := context.Background()
	newGame(t, repo, "gam_1", "grp_a", 1)
	if err := repo.AddCharacters(ctx, "gam_1", []character.ID{"chr_1"}, at(2)); err != nil {
		t.Fatalf("AddCharacters() error = %v", err)
	}

	if err := repo.Delete(ctx, "gam_1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	// Re-creating the id must not inherit the dead game's seats.
	newGame(t, repo, "gam_1", "grp_a", 5)
	roster, err := repo.Characters(ctx, "gam_1")
	if err != nil {
		t.Fatalf("Characters() error = %v", err)
	}
	if len(roster) != 0 {
		t.Errorf("a recreated game inherited %d seats, want 0", len(roster))
	}
}

func newGame(t *testing.T, repo *memory.GameRepository, id domain.ID, g group.ID, sec int64) {
	t.Helper()
	err := repo.Create(context.Background(), domain.Game{
		ID: id, Group: g, Name: "Thursday", CreatedBy: "acct-a", CreatedAt: at(sec),
	})
	if err != nil {
		t.Fatalf("Create(%q) error = %v", id, err)
	}
}
