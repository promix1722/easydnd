package repotest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/promix1722/easydnd/internal/domain/character"
	domain "github.com/promix1722/easydnd/internal/domain/game"
	"github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// NewSharedRepository builds an empty pool for one subtest, together with
// the group and account stores it refers to: every pool row names a group,
// which in Postgres is a foreign key, so the suite seeds the groups it uses.
type NewSharedRepository func(t *testing.T) (domain.SharedRepository, group.Repository, user.Repository)

// NewGameRepository builds an empty game store for one subtest, with the
// group and account stores for the same reason.
type NewGameRepository func(t *testing.T) (domain.Repository, group.Repository, user.Repository)

// at stamps a whole second, so comparisons do not depend on monotonic clock
// readings surviving a round trip through the store.
func at(sec int64) time.Time { return time.Unix(sec, 0).UTC() }

func shared(g group.ID, c character.ID, sec int64) domain.Shared {
	return domain.Shared{Group: g, Character: c, Owner: "acct-a", SharedAt: at(sec)}
}

// seedTables creates the account acct-a and the groups named, so that a
// foreign key has something to point at.
func seedTables(t *testing.T, groups group.Repository, users user.Repository, ids ...group.ID) {
	t.Helper()
	ctx := context.Background()
	if err := users.Create(ctx, Account("acct-a")); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	for _, id := range ids {
		if err := groups.Create(ctx, groupAt(string(id), "Table", "acct-a", 1), "acct-a"); err != nil {
			t.Fatalf("seed group %q: %v", id, err)
		}
	}
}

// RunSharedRepository runs the pool contract against one implementation.
func RunSharedRepository(t *testing.T, newRepos NewSharedRepository) {
	t.Helper()

	setup := func(t *testing.T) domain.SharedRepository {
		t.Helper()
		repo, groups, users := newRepos(t)
		seedTables(t, groups, users, "grp_a", "grp_b", "grp_c")
		return repo
	}

	tests := []struct {
		name string
		run  func(t *testing.T, repo domain.SharedRepository)
	}{
		{
			name: "a pool lists in the order characters were shared",
			run: func(t *testing.T, repo domain.SharedRepository) {
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
					if got[i].Character != want[i] || got[i].Owner != "acct-a" || !got[i].SharedAt.Equal(at(int64(i))) {
						t.Errorf("List()[%d] = %+v, want %q by acct-a at %v", i, got[i], want[i], at(int64(i)))
					}
				}
				empty, err := repo.List(ctx, "grp_b")
				if err != nil {
					t.Fatalf("List() error = %v", err)
				}
				if empty == nil || len(empty) != 0 {
					t.Errorf("List() of an empty pool = %v, want an empty, non-nil slice", empty)
				}
			},
		},
		{
			name: "the same character cannot be shared twice with one group",
			run: func(t *testing.T, repo domain.SharedRepository) {
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
				// A share missing any of its three names is refused.
				for name, s := range map[string]domain.Shared{
					"group":     {Character: "chr_9", Owner: "acct-a"},
					"character": {Group: "grp_a", Owner: "acct-a"},
					"owner":     {Group: "grp_a", Character: "chr_9"},
				} {
					if err := repo.Share(ctx, s); !errors.As(err, &invalid) {
						t.Errorf("Share() without a %s error = %v, want *types.ValidationError", name, err)
					}
				}
			},
		},
		{
			name: "unshare takes one character off one table",
			run: func(t *testing.T, repo domain.SharedRepository) {
				ctx := context.Background()
				for _, c := range []character.ID{"chr_1", "chr_2"} {
					if err := repo.Share(ctx, shared("grp_a", c, 1)); err != nil {
						t.Fatalf("Share() error = %v", err)
					}
				}
				if err := repo.Unshare(ctx, "grp_a", "chr_1"); err != nil {
					t.Fatalf("Unshare() error = %v", err)
				}
				if err := repo.Unshare(ctx, "grp_a", "chr_1"); !types.IsNotFound(err) {
					t.Errorf("second Unshare() error = %v, want a NotFoundError", err)
				}
				ok, err := repo.IsShared(ctx, "grp_a", "chr_2")
				if err != nil || !ok {
					t.Errorf("IsShared(chr_2) = %v, %v; want true", ok, err)
				}
				ok, err = repo.IsShared(ctx, "grp_a", "chr_1")
				if err != nil || ok {
					t.Errorf("IsShared(chr_1) = %v, %v; want false", ok, err)
				}
			},
		},
		{
			name: "groups sharing finds every table a character is on",
			run: func(t *testing.T, repo domain.SharedRepository) {
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
				// Sorted, so the answer does not depend on storage order.
				if len(got) != 2 || got[0] != "grp_a" || got[1] != "grp_b" {
					t.Errorf("GroupsSharing() = %v, want [grp_a grp_b]", got)
				}
			},
		},
		{
			name: "deleting a character takes it off every table",
			run: func(t *testing.T, repo domain.SharedRepository) {
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
				// And a character on no table is not an error.
				if err := repo.UnshareEverywhere(ctx, "chr_nobody"); err != nil {
					t.Errorf("UnshareEverywhere() of an unshared character error = %v", err)
				}
			},
		},
		{
			name: "clear group empties one pool",
			run: func(t *testing.T, repo domain.SharedRepository) {
				ctx := context.Background()
				for _, g := range []group.ID{"grp_a", "grp_b"} {
					if err := repo.Share(ctx, shared(g, "chr_1", 1)); err != nil {
						t.Fatalf("Share() error = %v", err)
					}
				}
				if err := repo.ClearGroup(ctx, "grp_a"); err != nil {
					t.Fatalf("ClearGroup() error = %v", err)
				}
				if err := repo.ClearGroup(ctx, "grp_a"); err != nil {
					t.Errorf("ClearGroup() of an empty pool error = %v", err)
				}
				got, err := repo.GroupsSharing(ctx, "chr_1")
				if err != nil {
					t.Fatalf("GroupsSharing() error = %v", err)
				}
				if len(got) != 1 || got[0] != "grp_b" {
					t.Errorf("GroupsSharing() = %v, want [grp_b]", got)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t, setup(t))
		})
	}
}

// RunGameRepository runs the game and roster contract against one
// implementation.
func RunGameRepository(t *testing.T, newRepos NewGameRepository) {
	t.Helper()

	setup := func(t *testing.T) domain.Repository {
		t.Helper()
		repo, groups, users := newRepos(t)
		seedTables(t, groups, users, "grp_a", "grp_b")
		return repo
	}

	tests := []struct {
		name string
		run  func(t *testing.T, repo domain.Repository)
	}{
		{
			name: "create loads back and refuses a duplicate or an empty field",
			run: func(t *testing.T, repo domain.Repository) {
				ctx := context.Background()
				newGame(t, repo, "gam_1", "grp_a", 1)
				got, err := repo.ByID(ctx, "gam_1")
				if err != nil {
					t.Fatalf("ByID() error = %v", err)
				}
				if got.Group != "grp_a" || got.Name != "Thursday" || got.CreatedBy != "acct-a" || !got.CreatedAt.Equal(at(1)) {
					t.Errorf("ByID() = %+v", got)
				}
				var invalid *types.ValidationError
				err = repo.Create(ctx, domain.Game{ID: "gam_1", Group: "grp_a", Name: "Again", CreatedBy: "acct-a", CreatedAt: at(2)})
				if !errors.As(err, &invalid) {
					t.Errorf("Create() of a duplicate error = %v, want *types.ValidationError", err)
				}
				for name, g := range map[string]domain.Game{
					"id":    {Group: "grp_a", Name: "x"},
					"group": {ID: "gam_2", Name: "x"},
					"name":  {ID: "gam_2", Group: "grp_a"},
				} {
					if err := repo.Create(ctx, g); !errors.As(err, &invalid) {
						t.Errorf("Create() without a %s error = %v, want *types.ValidationError", name, err)
					}
				}
				if _, err := repo.ByID(ctx, "gam_missing"); !types.IsNotFound(err) {
					t.Errorf("ByID() error = %v, want a NotFoundError", err)
				}
			},
		},
		{
			name: "rename changes the name and nothing else",
			run: func(t *testing.T, repo domain.Repository) {
				ctx := context.Background()
				newGame(t, repo, "gam_1", "grp_a", 1)
				if err := repo.Rename(ctx, "gam_1", "Friday"); err != nil {
					t.Fatalf("Rename() error = %v", err)
				}
				got, err := repo.ByID(ctx, "gam_1")
				if err != nil {
					t.Fatalf("ByID() error = %v", err)
				}
				if got.Name != "Friday" || !got.CreatedAt.Equal(at(1)) {
					t.Errorf("ByID() after Rename = %+v", got)
				}
				var invalid *types.ValidationError
				if err := repo.Rename(ctx, "gam_1", ""); !errors.As(err, &invalid) {
					t.Errorf("Rename() to nothing error = %v, want *types.ValidationError", err)
				}
				if err := repo.Rename(ctx, "gam_missing", "x"); !types.IsNotFound(err) {
					t.Errorf("Rename() error = %v, want a NotFoundError", err)
				}
			},
		},
		{
			name: "a roster keeps the order characters were added",
			run: func(t *testing.T, repo domain.Repository) {
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
					if roster[i].Character != want[i] || roster[i].Kind != "player" || roster[i].ID != "pc_"+string(want[i]) {
						t.Errorf("Characters()[%d] = %+v, want player %q", i, roster[i], want[i])
					}
				}
				var invalid *types.ValidationError
				if err := repo.AddCharacters(ctx, "gam_1", []character.ID{""}, at(4)); !errors.As(err, &invalid) {
					t.Errorf("AddCharacters() of a nameless seat error = %v, want *types.ValidationError", err)
				}
				if err := repo.AddCharacters(ctx, "gam_missing", []character.ID{"chr_1"}, at(4)); !types.IsNotFound(err) {
					t.Errorf("AddCharacters() error = %v, want a NotFoundError", err)
				}
				if _, err := repo.Characters(ctx, "gam_missing"); !types.IsNotFound(err) {
					t.Errorf("Characters() error = %v, want a NotFoundError", err)
				}
			},
		},
		{
			name: "adding somebody already seated changes nothing",
			run: func(t *testing.T, repo domain.Repository) {
				ctx := context.Background()
				newGame(t, repo, "gam_1", "grp_a", 1)
				if err := repo.AddCharacters(ctx, "gam_1", []character.ID{"chr_1"}, at(2)); err != nil {
					t.Fatalf("AddCharacters() error = %v", err)
				}
				// Adding everybody when most are already seated is the
				// common case: it must not duplicate a seat or re-stamp the
				// one already taken.
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
			},
		},
		{
			name: "remove character clears one seat",
			run: func(t *testing.T, repo domain.Repository) {
				ctx := context.Background()
				newGame(t, repo, "gam_1", "grp_a", 1)
				if err := repo.AddCharacters(ctx, "gam_1", []character.ID{"chr_1", "chr_2"}, at(2)); err != nil {
					t.Fatalf("AddCharacters() error = %v", err)
				}
				if err := repo.RemoveCharacter(ctx, "gam_1", "chr_1"); err != nil {
					t.Fatalf("RemoveCharacter() error = %v", err)
				}
				if err := repo.RemoveCharacter(ctx, "gam_1", "chr_1"); !types.IsNotFound(err) {
					t.Errorf("second RemoveCharacter() error = %v, want a NotFoundError", err)
				}
				if err := repo.RemoveCharacter(ctx, "gam_missing", "chr_1"); !types.IsNotFound(err) {
					t.Errorf("RemoveCharacter() error = %v, want a NotFoundError", err)
				}
				roster, err := repo.Characters(ctx, "gam_1")
				if err != nil {
					t.Fatalf("Characters() error = %v", err)
				}
				if len(roster) != 1 || roster[0].Character != "chr_2" {
					t.Errorf("Characters() = %+v, want just chr_2", roster)
				}
			},
		},
		{
			name: "unsharing clears the seat in every game at that table",
			run: func(t *testing.T, repo domain.Repository) {
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
			},
		},
		{
			name: "games come back newest first",
			run: func(t *testing.T, repo domain.Repository) {
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
			},
		},
		{
			name: "deleting a game takes its roster with it",
			run: func(t *testing.T, repo domain.Repository) {
				ctx := context.Background()
				newGame(t, repo, "gam_1", "grp_a", 1)
				if err := repo.AddCharacters(ctx, "gam_1", []character.ID{"chr_1"}, at(2)); err != nil {
					t.Fatalf("AddCharacters() error = %v", err)
				}
				if err := repo.Delete(ctx, "gam_1"); err != nil {
					t.Fatalf("Delete() error = %v", err)
				}
				if err := repo.Delete(ctx, "gam_1"); !types.IsNotFound(err) {
					t.Errorf("second Delete() error = %v, want a NotFoundError", err)
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
			},
		},
		{
			name: "deleting a group's games leaves the other tables alone",
			run: func(t *testing.T, repo domain.Repository) {
				ctx := context.Background()
				newGame(t, repo, "gam_1", "grp_a", 1)
				newGame(t, repo, "gam_2", "grp_a", 2)
				newGame(t, repo, "gam_3", "grp_b", 3)
				if err := repo.DeleteForGroup(ctx, "grp_a"); err != nil {
					t.Fatalf("DeleteForGroup() error = %v", err)
				}
				if err := repo.DeleteForGroup(ctx, "grp_a"); err != nil {
					t.Errorf("DeleteForGroup() of an empty table error = %v", err)
				}
				if got, err := repo.ListFor(ctx, "grp_a"); err != nil || len(got) != 0 {
					t.Errorf("ListFor(grp_a) = %v, %v; want nothing", got, err)
				}
				if got, err := repo.ListFor(ctx, "grp_b"); err != nil || len(got) != 1 {
					t.Errorf("ListFor(grp_b) = %v, %v; want one game", got, err)
				}
			},
		},
		{
			name: "tracker mutation is atomic and deeply isolated",
			run: func(t *testing.T, store domain.Repository) {
				ctx := context.Background()
				newGame(t, store, "g", "grp_a", 1)
				initiative := 12
				entry := domain.Entry{ID: "monster", Kind: "monster", HP: 4, Initiative: &initiative, Tags: []string{"secret"}, Used: map[string]int{"breath": 1}, Monster: &domain.Stats{
					Name: "Beast", Speeds: []character.Speed{{Kind: character.Walking, Distance: 30}},
					Abilities: character.Abilities{Scores: map[rules.Ability]int{rules.Strength: 14}, ModifierRule: rules.Expression{Op: "add", Args: []rules.Expression{{Op: "constant", Value: 2}}}},
				}}
				if err := store.MutateEntries(ctx, "g", func(entries []domain.Entry) ([]domain.Entry, error) { return append(entries, entry), nil }); err != nil {
					t.Fatal(err)
				}
				// Input references cannot write through into storage.
				entry.Tags[0], *entry.Initiative, entry.Monster.Abilities.Scores[rules.Strength] = "changed", 99, 1
				first, err := store.Characters(ctx, "g")
				if err != nil {
					t.Fatal(err)
				}
				if first[0].Tags[0] != "secret" || *first[0].Initiative != 12 || first[0].Monster.Abilities.Score(rules.Strength) != 14 || first[0].Used["breath"] != 1 {
					t.Fatal("input aliases storage")
				}
				first[0].Monster.Speeds[0].Distance = 99
				first[0].Monster.Abilities.ModifierRule.Args[0].Value = 99
				err = store.MutateEntries(ctx, "g", func(entries []domain.Entry) ([]domain.Entry, error) {
					entries[0].Tags[0] = "rejected"
					entries[0].HP = 99
					return entries, types.NewValidationError("reject")
				})
				if err == nil {
					t.Fatal("expected refusal")
				}
				after, err := store.Characters(ctx, "g")
				if err != nil {
					t.Fatal(err)
				}
				if after[0].HP != 4 || after[0].Tags[0] != "secret" || after[0].Monster.Speeds[0].Distance != 30 || after[0].Monster.Abilities.ModifierRule.Args[0].Value != 2 {
					t.Fatal("read or rejected mutation aliases storage")
				}
				if err := store.MutateEntries(ctx, "gam_missing", func(entries []domain.Entry) ([]domain.Entry, error) { return entries, nil }); !types.IsNotFound(err) {
					t.Errorf("MutateEntries() error = %v, want a NotFoundError", err)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t, setup(t))
		})
	}
}

func newGame(t *testing.T, repo domain.Repository, id domain.ID, g group.ID, sec int64) {
	t.Helper()
	err := repo.Create(context.Background(), domain.Game{
		ID: id, Group: g, Name: "Thursday", CreatedBy: "acct-a", CreatedAt: at(sec),
	})
	if err != nil {
		t.Fatalf("Create(%q) error = %v", id, err)
	}
}
