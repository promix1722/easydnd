package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	characterapi "github.com/promix1722/easydnd/internal/api/http/v1/character"
	gameapi "github.com/promix1722/easydnd/internal/api/http/v1/game"
)

// makeCharacter creates one and returns its id.
func makeCharacter(t *testing.T, r *gin.Engine, session *http.Cookie, name string) string {
	t.Helper()
	rec := send(t, r, session, http.MethodPost, "/v1/characters", newCharacter(name, ""))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/characters = %d, want 201 (%s)", rec.Code, rec.Body.String())
	}
	return decode[characterapi.CreateResponse](t, rec).ID
}

// shareCharacter puts one on a group's table.
func shareCharacter(
	t *testing.T, r *gin.Engine, session *http.Cookie, group, character string,
) gameapi.TableResponse {
	t.Helper()
	rec := send(t, r, session, http.MethodPost, "/v1/groups/"+group+"/characters",
		map[string]any{"character_id": character})
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST table = %d, want 201 (%s)", rec.Code, rec.Body.String())
	}
	return decode[gameapi.TableResponse](t, rec)
}

// seatSecondAccount registers a second account and joins it to the group as a
// player, returning its session.
func seatSecondAccount(
	t *testing.T, r *gin.Engine, owner *http.Cookie, ceremony *stubCeremony, group string,
) *http.Cookie {
	t.Helper()
	ceremony.credentialID = "second-credential"
	joiner := register(t, r, helpers.CookieOptions{Secure: false})
	token := inviteToken(t, r, owner, group, "player")
	rec := send(t, r, joiner, http.MethodPost, "/v1/invites/accept", map[string]any{"token": token})
	if rec.Code != http.StatusOK {
		t.Fatalf("accept = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	return joiner
}

// The whole feature end to end: a player puts a character on the table, the DM
// opens a game and seats everybody, and the table reads a sheet it does not own.
func TestAGameFromTheTableToItsRoster(t *testing.T) {
	r, owner, ceremony := newFullRouterWithCeremony(t)
	group := createGroup(t, r, owner, "Wednesday Night")
	player := seatSecondAccount(t, r, owner, ceremony, group.ID)

	mine := makeCharacter(t, r, owner, "Ada")
	theirs := makeCharacter(t, r, player, "Bram")

	// A player may share: it is the whole of what a player does at a table.
	table := shareCharacter(t, r, player, group.ID, theirs)
	if len(table.Characters) != 1 {
		t.Fatalf("the table holds %d characters, want 1", len(table.Characters))
	}
	shareCharacter(t, r, owner, group.ID, mine)

	// Both members see the whole table, not just their own row.
	for _, who := range []*http.Cookie{owner, player} {
		rec := send(t, r, who, http.MethodGet, "/v1/groups/"+group.ID+"/characters", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET table = %d, want 200 (%s)", rec.Code, rec.Body.String())
		}
		if got := len(decode[gameapi.TableResponse](t, rec).Characters); got != 2 {
			t.Errorf("a member sees %d characters on the table, want 2", got)
		}
	}

	// Only the DM opens a game.
	rec := send(t, r, player, http.MethodPost, "/v1/games",
		map[string]any{"group_id": group.ID, "name": "Thursday"})
	if rec.Code != http.StatusForbidden {
		t.Errorf("a player opening a game = %d, want 403", rec.Code)
	}
	rec = send(t, r, owner, http.MethodPost, "/v1/games",
		map[string]any{"group_id": group.ID, "name": "Thursday"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST games = %d, want 201 (%s)", rec.Code, rec.Body.String())
	}
	game := decode[gameapi.Game](t, rec)
	if game.Role != "owner" {
		t.Errorf("the creator's role on the game = %q, want %q", game.Role, "owner")
	}

	// Seat both, named explicitly. "Everyone on the table" is the client
	// sending the list it already has on screen, so there is one request shape
	// whether it is one character or nine.
	rec = send(t, r, owner, http.MethodPost, "/v1/games/"+game.ID+"/characters",
		map[string]any{"character_ids": []string{mine, theirs}})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST roster = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if got := len(decode[gameapi.Game](t, rec).Characters); got != 2 {
		t.Errorf("the roster seats %d characters, want 2", got)
	}

	// A player may read the game they are in and change nothing about it.
	rec = send(t, r, player, http.MethodGet, "/v1/games/"+game.ID, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("a player reading a game = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	rec = send(t, r, player, http.MethodPatch, "/v1/games/"+game.ID,
		map[string]any{"name": "Friday"})
	if rec.Code != http.StatusForbidden {
		t.Errorf("a player renaming a game = %d, want 403", rec.Code)
	}
}

// The regression test for the whole design: sharing grants a read and nothing
// else, and character.Service.owned was not loosened to achieve it.
func TestTheTableCanReadTheSheetAndNotTouchIt(t *testing.T) {
	r, owner, ceremony := newFullRouterWithCeremony(t)
	group := createGroup(t, r, owner, "Wednesday Night")
	player := seatSecondAccount(t, r, owner, ceremony, group.ID)

	theirs := makeCharacter(t, r, player, "Bram")
	shareCharacter(t, r, player, group.ID, theirs)

	// The DM reads the shared sheet.
	rec := send(t, r, owner, http.MethodGet, "/v1/shared/"+theirs+"/sheet", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("reading a shared sheet = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if decode[characterapi.Sheet](t, rec).Identity.Name != "Bram" {
		t.Error("the shared sheet is not the character that was shared")
	}

	// And can do nothing else with it. Every one of these is a 404 rather than
	// a 403, because /v1/characters is the tree of things that are yours and a
	// character that is not yours is indistinguishable from one that does not
	// exist.
	for _, probe := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/v1/characters/" + theirs, nil},
		{http.MethodGet, "/v1/characters/" + theirs + "/sheet", nil},
		{http.MethodGet, "/v1/characters/" + theirs + "/events", nil},
		{http.MethodPost, "/v1/characters/" + theirs + "/events",
			map[string]any{"expectedSeq": 1, "events": []map[string]any{
				{"type": "note", "note": "trespassing"},
			}}},
		{http.MethodPut, "/v1/characters/" + theirs + "/folder", map[string]any{"folder": ""}},
		{http.MethodDelete, "/v1/characters/" + theirs, nil},
	} {
		rec := send(t, r, owner, probe.method, probe.path, probe.body)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s = %d, want 404", probe.method, probe.path, rec.Code)
		}
	}
}

func TestAnUnsharedCharacterIsInvisibleToTheTable(t *testing.T) {
	r, owner, ceremony := newFullRouterWithCeremony(t)
	group := createGroup(t, r, owner, "Wednesday Night")
	player := seatSecondAccount(t, r, owner, ceremony, group.ID)

	// Made, and never put on the table. Being at the same table is not what
	// grants the read -- sharing is.
	theirs := makeCharacter(t, r, player, "Bram")

	rec := send(t, r, owner, http.MethodGet, "/v1/shared/"+theirs+"/sheet", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("reading an unshared character = %d, want 404", rec.Code)
	}
}

func TestASharedCharacterIsInvisibleOutsideTheGroup(t *testing.T) {
	r, owner, ceremony := newFullRouterWithCeremony(t)
	group := createGroup(t, r, owner, "Wednesday Night")
	mine := makeCharacter(t, r, owner, "Ada")
	shareCharacter(t, r, owner, group.ID, mine)

	// A third account, in no group at all.
	ceremony.credentialID = "stranger-credential"
	stranger := register(t, r, helpers.CookieOptions{Secure: false})

	rec := send(t, r, stranger, http.MethodGet, "/v1/shared/"+mine+"/sheet", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("a stranger reading a shared sheet = %d, want 404", rec.Code)
	}
	rec = send(t, r, stranger, http.MethodGet, "/v1/groups/"+group.ID+"/characters", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("a stranger reading the table = %d, want 404", rec.Code)
	}
}

func TestUnsharingClearsTheSeatInEveryGame(t *testing.T) {
	r, owner, ceremony := newFullRouterWithCeremony(t)
	group := createGroup(t, r, owner, "Wednesday Night")
	player := seatSecondAccount(t, r, owner, ceremony, group.ID)

	// The player's character, not the DM's: the read being revoked is the one
	// sharing granted, and an owner may always read their own.
	mine := makeCharacter(t, r, player, "Bram")
	shareCharacter(t, r, player, group.ID, mine)

	rec := send(t, r, owner, http.MethodPost, "/v1/games",
		map[string]any{"group_id": group.ID, "name": "Thursday"})
	game := decode[gameapi.Game](t, rec)
	send(t, r, owner, http.MethodPost, "/v1/games/"+game.ID+"/characters",
		map[string]any{"character_ids": []string{mine}})

	rec = send(t, r, owner, http.MethodDelete,
		"/v1/groups/"+group.ID+"/characters?character="+mine, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("unshare = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

	// A character seated at a game but not on the table is the state the table
	// exists to prevent.
	rec = send(t, r, owner, http.MethodGet, "/v1/games/"+game.ID, nil)
	if got := len(decode[gameapi.Game](t, rec).Characters); got != 0 {
		t.Errorf("the roster still seats %d characters, want 0", got)
	}
	// And the read it granted goes with it.
	rec = send(t, r, owner, http.MethodGet, "/v1/shared/"+mine+"/sheet", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("reading an unshared sheet = %d, want 404", rec.Code)
	}
}

func TestSeatingYourOwnCharacterPutsItOnTheTable(t *testing.T) {
	r, owner, _ := newFullRouterWithCeremony(t)
	group := createGroup(t, r, owner, "Wednesday Night")
	private := makeCharacter(t, r, owner, "Ada")

	rec := send(t, r, owner, http.MethodPost, "/v1/games",
		map[string]any{"group_id": group.ID, "name": "Thursday"})
	game := decode[gameapi.Game](t, rec)

	// Your own goes on the table by being seated -- one action, not two.
	rec = send(t, r, owner, http.MethodPost, "/v1/games/"+game.ID+"/characters",
		map[string]any{"character_ids": []string{private}})
	if rec.Code != http.StatusOK {
		t.Fatalf("seating your own character = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	rec = send(t, r, owner, http.MethodGet, "/v1/groups/"+group.ID+"/characters", nil)
	if got := len(decode[gameapi.TableResponse](t, rec).Characters); got != 1 {
		t.Errorf("the table holds %d characters after seating one, want 1", got)
	}
}

func TestGameRoutesRequireASession(t *testing.T) {
	r, _, _ := newFullRouterWithCeremony(t)
	for _, probe := range []struct{ method, path string }{
		{http.MethodGet, "/v1/groups/grp_x/characters"},
		{http.MethodPost, "/v1/groups/grp_x/characters"},
		{http.MethodGet, "/v1/games"},
		{http.MethodGet, "/v1/games/gam_x"},
		{http.MethodPost, "/v1/games/gam_x/characters"},
		{http.MethodGet, "/v1/shared/chr_x/sheet"},
	} {
		rec := send(t, r, nil, probe.method, probe.path, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without a session = %d, want 401", probe.method, probe.path, rec.Code)
		}
	}
}

// Deleting a character takes it off every table it was on, rather than leaving
// a row that names nothing.
func TestDeletingACharacterTakesItOffTheTable(t *testing.T) {
	r, owner, ceremony := newFullRouterWithCeremony(t)
	group := createGroup(t, r, owner, "Wednesday Night")
	player := seatSecondAccount(t, r, owner, ceremony, group.ID)

	doomed := makeCharacter(t, r, player, "Bram")
	stays := makeCharacter(t, r, player, "Cass")
	shareCharacter(t, r, player, group.ID, doomed)
	shareCharacter(t, r, player, group.ID, stays)

	rec := send(t, r, player, http.MethodDelete, "/v1/characters/"+doomed, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE character = %d, want 204 (%s)", rec.Code, rec.Body.String())
	}

	rec = send(t, r, owner, http.MethodGet, "/v1/groups/"+group.ID+"/characters", nil)
	table := decode[gameapi.TableResponse](t, rec)
	if len(table.Characters) != 1 || table.Characters[0].ID != stays {
		t.Errorf("the table holds %d characters after a delete, want just the survivor",
			len(table.Characters))
	}
	// Your character is always yours to delete: being on somebody's table does
	// not make it theirs, and nothing consulted the group before agreeing.
}

// Deleting a group takes its games and its table with it.
func TestDeletingAGroupTakesItsGamesWithIt(t *testing.T) {
	r, owner, _ := newFullRouterWithCeremony(t)
	group := createGroup(t, r, owner, "Wednesday Night")
	mine := makeCharacter(t, r, owner, "Ada")
	shareCharacter(t, r, owner, group.ID, mine)

	rec := send(t, r, owner, http.MethodPost, "/v1/games",
		map[string]any{"group_id": group.ID, "name": "Thursday"})
	game := decode[gameapi.Game](t, rec)

	if rec := send(t, r, owner, http.MethodDelete, "/v1/groups/"+group.ID, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE group = %d, want 204 (%s)", rec.Code, rec.Body.String())
	}

	// The game is gone rather than merely unreachable.
	rec = send(t, r, owner, http.MethodGet, "/v1/games/"+game.ID, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("reading a game whose group was deleted = %d, want 404", rec.Code)
	}
	// And the character itself survives: it was never the group's.
	if rec := send(t, r, owner, http.MethodGet, "/v1/characters/"+mine+"/sheet", nil); rec.Code != http.StatusOK {
		t.Errorf("the owner's own character after deleting the group = %d, want 200", rec.Code)
	}
}

// A game is a section of its own: it comes back from /v1/games with the table
// it sits at named, without the caller having to say which table first.
func TestYourGamesComeBackWithTheTableTheySitAt(t *testing.T) {
	r, owner, _ := newFullRouterWithCeremony(t)
	group := createGroup(t, r, owner, "Wednesday Night")
	rec := send(t, r, owner, http.MethodPost, "/v1/games",
		map[string]any{"group_id": group.ID, "name": "Thursday night"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/games = %d, want 201 (%s)", rec.Code, rec.Body.String())
	}

	rec = send(t, r, owner, http.MethodGet, "/v1/games", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/games = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	games := decode[gameapi.ListResponse](t, rec).Games
	if len(games) != 1 {
		t.Fatalf("GET /v1/games returned %d games, want 1", len(games))
	}
	if games[0].Name != "Thursday night" {
		t.Errorf("game name = %q, want %q", games[0].Name, "Thursday night")
	}
	// The row has to say which table, or a player at three of them cannot
	// tell their Thursdays apart.
	if games[0].GroupName != "Wednesday Night" {
		t.Errorf("group_name = %q, want %q", games[0].GroupName, "Wednesday Night")
	}
}

// Somebody else's game is not in your list, and naming it directly is a 404.
func TestAnotherTablesGamesAreNotYours(t *testing.T) {
	r, owner, ceremony := newFullRouterWithCeremony(t)
	group := createGroup(t, r, owner, "Wednesday Night")
	rec := send(t, r, owner, http.MethodPost, "/v1/games",
		map[string]any{"group_id": group.ID, "name": "Thursday night"})
	game := decode[gameapi.Game](t, rec)

	ceremony.credentialID = "stranger-credential"
	stranger := register(t, r, helpers.CookieOptions{Secure: false})

	rec = send(t, r, stranger, http.MethodGet, "/v1/games", nil)
	if got := len(decode[gameapi.ListResponse](t, rec).Games); got != 0 {
		t.Errorf("a stranger's game list has %d rows, want 0", got)
	}
	rec = send(t, r, stranger, http.MethodGet, "/v1/games/"+game.ID, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("a stranger naming a game = %d, want 404", rec.Code)
	}
}
