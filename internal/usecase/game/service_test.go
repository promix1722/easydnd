package game_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	catalogfile "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
	gameuc "github.com/promix1722/easydnd/internal/usecase/game"
)

type fixture struct {
	svc        *gameuc.Service
	groups     group.Repository
	characters character.Repository
}

// newFixture seeds three accounts and wires the service over empty stores.
//
// It uses the real SRD compendium rather than a stub, because summarising a
// character folds its log against the catalogue and a fake one would make the
// listing tests prove nothing about the fold they exercise.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	users := memory.NewUserRepository()
	for _, id := range []string{"alice", "bob", "carol"} {
		err := users.Create(context.Background(), user.User{
			ID: user.ID(id), DisplayName: id,
		})
		if err != nil {
			t.Fatalf("seed account %q: %v", id, err)
		}
	}
	groups := memory.NewGroupRepository(users)
	characters := memory.NewCharacterRepository()
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))

	return &fixture{
		svc: gameuc.NewService(
			memory.NewGameRepository(),
			memory.NewSharedRepository(),
			groups,
			characters,
			catalogfile.NewSource(filepath.Join("..", "..", "..", "data", "srd_5.1")),
			log,
		),
		groups:     groups,
		characters: characters,
	}
}

// table creates a group owned by owner and seats the rest at the given ranks.
func (f *fixture) table(t *testing.T, id group.ID, owner user.ID, seats map[user.ID]group.Role) {
	t.Helper()
	ctx := context.Background()
	g := group.Group{ID: id, Name: "The Table", CreatedBy: owner, CreatedAt: at(1)}
	if err := f.groups.Create(ctx, g, owner); err != nil {
		t.Fatalf("create group: %v", err)
	}
	for u, role := range seats {
		if err := f.groups.AddMember(ctx, id, u, role, at(2)); err != nil {
			t.Fatalf("seat %q: %v", u, err)
		}
	}
}

// character makes one belonging to owner and returns its id.
func (f *fixture) character(t *testing.T, owner user.ID) character.ID {
	t.Helper()
	c, err := f.characters.Create(context.Background(), character.OwnerID(owner), "")
	if err != nil {
		t.Fatalf("create character: %v", err)
	}
	return c.ID
}

// at stamps a whole second, so a comparison does not depend on a monotonic
// reading surviving a round trip through a store.
func at(sec int64) time.Time { return time.Unix(sec, 0).UTC() }

func assertNotFound(t *testing.T, err error, what string) {
	t.Helper()
	var nf *types.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("%s: error = %v, want *types.NotFoundError", what, err)
	}
}

func assertDenied(t *testing.T, err error, what string) {
	t.Helper()
	var denied *types.AccessDeniedError
	if !errors.As(err, &denied) {
		t.Fatalf("%s: error = %v, want *types.AccessDeniedError", what, err)
	}
}

func TestAPlayerSharesTheirOwnCharacterAndNobodyElses(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.table(t, "grp_a", "alice", map[user.ID]group.Role{"bob": group.RolePlayer})
	mine := f.character(t, "bob")
	hers := f.character(t, "alice")

	if err := f.svc.Share(ctx, "bob", "grp_a", mine); err != nil {
		t.Fatalf("a player sharing their own character: %v", err)
	}
	// Somebody else's is a 404, not a 403: a member must not be able to probe
	// which character ids are real by trying to share them.
	assertNotFound(t, f.svc.Share(ctx, "bob", "grp_a", hers), "sharing another account's character")
}

func TestAStrangerCannotReachTheTableAtAll(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.table(t, "grp_a", "alice", nil)
	mine := f.character(t, "carol")

	assertNotFound(t, f.svc.Share(ctx, "carol", "grp_a", mine), "a stranger sharing")
	_, err := f.svc.SharedCharacters(ctx, "carol", "grp_a", rules.DefaultLocale)
	assertNotFound(t, err, "a stranger listing the table")
	_, err = f.svc.Create(ctx, "carol", "grp_a", "Thursday")
	assertNotFound(t, err, "a stranger opening a game")
	// And a stranger's own list is empty rather than an error: they are at no
	// table, which is a fact about them and not a refusal.
	strangersGames, err := f.svc.Mine(ctx, "carol")
	if err != nil {
		t.Fatalf("Mine() error = %v", err)
	}
	if len(strangersGames) != 0 {
		t.Errorf("a stranger has %d games, want 0", len(strangersGames))
	}
}

func TestEveryMemberCanReadASharedSheetAndNoneCanEditIt(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.table(t, "grp_a", "alice", map[user.ID]group.Role{"bob": group.RolePlayer})
	bobs := f.character(t, "bob")
	if err := f.svc.Share(ctx, "bob", "grp_a", bobs); err != nil {
		t.Fatalf("Share() error = %v", err)
	}

	// The owner, and the person at the other end of the table, see the same
	// sheet. That is the whole point of sharing.
	for _, who := range []user.ID{"bob", "alice"} {
		if _, err := f.svc.Sheet(ctx, who, bobs, rules.DefaultLocale); err != nil {
			t.Errorf("Sheet() for %q error = %v", who, err)
		}
	}
	// A stranger sees nothing, and is told nothing.
	_, err := f.svc.Sheet(ctx, "carol", bobs, rules.DefaultLocale)
	assertNotFound(t, err, "a stranger reading a shared sheet")

	// There is no write path here at all. That is the invariant this feature
	// rests on, and it is proved by the character service being untouched --
	// this package exposes nothing that appends to a log.
}

func TestAnUnsharedCharacterIsInvisibleToTheTable(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.table(t, "grp_a", "alice", map[user.ID]group.Role{"bob": group.RolePlayer})
	bobs := f.character(t, "bob")

	// Being at the same table is not enough; sharing is what grants the read.
	_, err := f.svc.Sheet(ctx, "alice", bobs, rules.DefaultLocale)
	assertNotFound(t, err, "reading a character that was never shared")
}

func TestOnlyADMOpensAGame(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.table(t, "grp_a", "alice", map[user.ID]group.Role{
		"bob": group.RoleDM, "carol": group.RolePlayer,
	})

	for _, who := range []user.ID{"alice", "bob"} {
		if _, err := f.svc.Create(ctx, who, "grp_a", "Thursday"); err != nil {
			t.Errorf("Create() as %q error = %v", who, err)
		}
	}
	_, err := f.svc.Create(ctx, "carol", "grp_a", "Thursday")
	// A player is at the table and knows it exists, so this is a 403.
	assertDenied(t, err, "a player opening a game")
}

func TestAGameTakesOnlyCharactersSharedToItsGroup(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.table(t, "grp_a", "alice", map[user.ID]group.Role{"bob": group.RolePlayer})
	shared := f.character(t, "bob")
	private := f.character(t, "bob")
	if err := f.svc.Share(ctx, "bob", "grp_a", shared); err != nil {
		t.Fatalf("Share() error = %v", err)
	}
	g, err := f.svc.Create(ctx, "alice", "grp_a", "Thursday")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := f.svc.AddCharacters(ctx, "alice", g.ID, []character.ID{shared}); err != nil {
		t.Fatalf("AddCharacters() error = %v", err)
	}
	// Without this rule the sharing step would be decoration: a DM could seat
	// any character id they could guess.
	err = f.svc.AddCharacters(ctx, "alice", g.ID, []character.ID{private})
	var invalid *types.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("seating an unshared character: error = %v, want *types.ValidationError", err)
	}
}

func TestSeatingYourOwnCharacterPutsItOnTheTable(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.table(t, "grp_a", "alice", nil)
	mine := f.character(t, "alice")
	g, err := f.svc.Create(ctx, "alice", "grp_a", "Thursday")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Never shared. Seating it is one action, not two -- and it lands on the
	// table as a side effect, because everybody at the game has to be able to
	// read what is in front of them.
	if err := f.svc.AddCharacters(ctx, "alice", g.ID, []character.ID{mine}); err != nil {
		t.Fatalf("AddCharacters() error = %v", err)
	}

	table, err := f.svc.SharedCharacters(ctx, "alice", "grp_a", rules.DefaultLocale)
	if err != nil {
		t.Fatalf("SharedCharacters() error = %v", err)
	}
	if len(table) != 1 || table[0].ID != mine {
		t.Errorf("seating did not put the character on the table: %v", table)
	}
}

func TestSeatingSomebodyElsesUnsharedCharacterIsRefused(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.table(t, "grp_a", "alice", map[user.ID]group.Role{"bob": group.RolePlayer})
	theirs := f.character(t, "bob")
	g, err := f.svc.Create(ctx, "alice", "grp_a", "Thursday")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// A DM runs the table; they do not get to publish another player's
	// character on their behalf.
	err = f.svc.AddCharacters(ctx, "alice", g.ID, []character.ID{theirs})
	var invalid *types.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("seating another account's unshared character: error = %v, want *types.ValidationError", err)
	}
}

func TestSeatingNobodyIsRefused(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.table(t, "grp_a", "alice", nil)
	g, err := f.svc.Create(ctx, "alice", "grp_a", "Thursday")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	err = f.svc.AddCharacters(ctx, "alice", g.ID, nil)
	var invalid *types.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("seating an empty list: error = %v, want *types.ValidationError", err)
	}
}

func TestUnsharingTakesTheCharacterOutOfEveryGame(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.table(t, "grp_a", "alice", map[user.ID]group.Role{"bob": group.RolePlayer})
	c := f.character(t, "bob")
	if err := f.svc.Share(ctx, "bob", "grp_a", c); err != nil {
		t.Fatalf("Share() error = %v", err)
	}
	g, err := f.svc.Create(ctx, "alice", "grp_a", "Thursday")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := f.svc.AddCharacters(ctx, "alice", g.ID, []character.ID{c}); err != nil {
		t.Fatalf("AddCharacters() error = %v", err)
	}

	if err := f.svc.Unshare(ctx, "bob", "grp_a", c); err != nil {
		t.Fatalf("Unshare() error = %v", err)
	}

	// A character seated at a game but not on the table is the state the pool
	// exists to prevent.
	_, _, roster, err := f.svc.Get(ctx, "alice", g.ID, rules.DefaultLocale)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if len(roster) != 0 {
		t.Errorf("game still seats %d characters after unsharing, want 0", len(roster))
	}
	// And the read it granted is revoked with it.
	_, err = f.svc.Sheet(ctx, "alice", c, rules.DefaultLocale)
	assertNotFound(t, err, "reading a character that was unshared")
}

func TestADMMayClearSomebodyElsesCharacterOffTheTable(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.table(t, "grp_a", "alice", map[user.ID]group.Role{
		"bob": group.RolePlayer, "carol": group.RolePlayer,
	})
	c := f.character(t, "bob")
	if err := f.svc.Share(ctx, "bob", "grp_a", c); err != nil {
		t.Fatalf("Share() error = %v", err)
	}

	// A fellow player may not: it is not theirs and they do not run the table.
	assertDenied(t, f.svc.Unshare(ctx, "carol", "grp_a", c), "a player unsharing another's character")
	// The owner of the group may, because a guest's character could otherwise
	// sit there forever after their session expired.
	if err := f.svc.Unshare(ctx, "alice", "grp_a", c); err != nil {
		t.Errorf("a DM clearing the table: %v", err)
	}
}

func TestARosterSurvivesACharacterVanishing(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.table(t, "grp_a", "alice", nil)
	gone := f.character(t, "alice")
	stays := f.character(t, "alice")
	for _, c := range []character.ID{gone, stays} {
		if err := f.svc.Share(ctx, "alice", "grp_a", c); err != nil {
			t.Fatalf("Share() error = %v", err)
		}
	}

	// Deleted straight out of the character store, as a restart effectively
	// does to every id at once. One dead row must not break the screen.
	if err := f.characters.Delete(ctx, gone); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	got, err := f.svc.SharedCharacters(ctx, "alice", "grp_a", rules.DefaultLocale)
	if err != nil {
		t.Fatalf("SharedCharacters() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != stays {
		t.Errorf("SharedCharacters() = %v, want just the surviving character", got)
	}
}

func TestOnlyADMChangesARosterOrDeletesTheGame(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.table(t, "grp_a", "alice", map[user.ID]group.Role{"bob": group.RolePlayer})
	c := f.character(t, "bob")
	if err := f.svc.Share(ctx, "bob", "grp_a", c); err != nil {
		t.Fatalf("Share() error = %v", err)
	}
	g, err := f.svc.Create(ctx, "alice", "grp_a", "Thursday")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// A player may look at the game -- it is their table -- and change nothing.
	if _, _, _, err := f.svc.Get(ctx, "bob", g.ID, rules.DefaultLocale); err != nil {
		t.Errorf("a player reading a game: %v", err)
	}
	assertDenied(t, f.svc.AddCharacters(ctx, "bob", g.ID, []character.ID{c}), "a player seating a character")
	_, err = f.svc.Rename(ctx, "bob", g.ID, "Friday")
	assertDenied(t, err, "a player renaming a game")
	assertDenied(t, f.svc.Delete(ctx, "bob", g.ID), "a player deleting a game")
}

func TestDeletingAGameLeavesTheCharactersOnTheTable(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.table(t, "grp_a", "alice", nil)
	c := f.character(t, "alice")
	if err := f.svc.Share(ctx, "alice", "grp_a", c); err != nil {
		t.Fatalf("Share() error = %v", err)
	}
	g, err := f.svc.Create(ctx, "alice", "grp_a", "Thursday")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := f.svc.AddCharacters(ctx, "alice", g.ID, []character.ID{c}); err != nil {
		t.Fatalf("AddCharacters() error = %v", err)
	}

	if err := f.svc.Delete(ctx, "alice", g.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	// The characters were never the game's. Unlike a folder, it takes nothing
	// with it.
	got, err := f.svc.SharedCharacters(ctx, "alice", "grp_a", rules.DefaultLocale)
	if err != nil {
		t.Fatalf("SharedCharacters() error = %v", err)
	}
	if len(got) != 1 {
		t.Errorf("deleting a game left %d characters on the table, want 1", len(got))
	}
}

func TestAGameIsInvisibleFromOutsideItsGroup(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.table(t, "grp_a", "alice", nil)
	g, err := f.svc.Create(ctx, "alice", "grp_a", "Thursday")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Not a 403: a stranger must learn neither that the game exists nor that
	// the group behind it does.
	_, _, _, err = f.svc.Get(ctx, "carol", g.ID, rules.DefaultLocale)
	assertNotFound(t, err, "a stranger reading a game")
}

func TestYourGamesSpanEveryTableYouSitAt(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.table(t, "grp_a", "alice", map[user.ID]group.Role{"bob": group.RolePlayer})
	f.table(t, "grp_b", "bob", map[user.ID]group.Role{"alice": group.RolePlayer})

	if _, err := f.svc.Create(ctx, "alice", "grp_a", "Thursday"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := f.svc.Create(ctx, "bob", "grp_b", "Saturday"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Alice owns one table and plays at the other. Both games are hers to see,
	// and each row says which table it belongs to.
	mine, err := f.svc.Mine(ctx, "alice")
	if err != nil {
		t.Fatalf("Mine() error = %v", err)
	}
	if len(mine) != 2 {
		t.Fatalf("Mine() returned %d games, want 2", len(mine))
	}
	names := map[string]string{}
	for _, at := range mine {
		names[at.Game.Name] = at.GroupName
	}
	if names["Thursday"] != "The Table" || names["Saturday"] != "The Table" {
		t.Errorf("Mine() lost the table names: %v", names)
	}
}

// A refused name has to say so on the field it was typed into.
//
// Without a Message the client renders a blank inline error and falls back to
// an alert carrying the raw envelope -- error code, request id and all -- which
// is what a person sees instead of "a game needs a name".
func TestARefusedNameSaysSoOnTheField(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.table(t, "grp_a", "alice", nil)

	_, err := f.svc.Create(ctx, "alice", "grp_a", "   ")
	var fields *types.FieldValidationError
	if !errors.As(err, &fields) {
		t.Fatalf("Create(blank name) error = %v, want *types.FieldValidationError", err)
	}
	if len(fields.Fields) != 1 {
		t.Fatalf("error carries %d fields, want 1", len(fields.Fields))
	}
	got := fields.Fields[0]
	if got.Field != "name" {
		t.Errorf("field = %q, want %q", got.Field, "name")
	}
	if got.Message == "" {
		t.Error("the field error carries no message, so the input shows a blank error")
	}
}
