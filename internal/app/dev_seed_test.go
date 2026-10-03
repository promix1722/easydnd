package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	authapi "github.com/promix1722/easydnd/internal/api/http/v1/auth"
	characterapi "github.com/promix1722/easydnd/internal/api/http/v1/character"
	gameapi "github.com/promix1722/easydnd/internal/api/http/v1/game"
	"github.com/promix1722/easydnd/internal/config"
)

func developmentApp(t *testing.T, env string) *App {
	return developmentAppAtPort(t, env, "8080")
}

func developmentAppAtPort(t *testing.T, env, port string) *App {
	t.Helper()
	cfg, err := config.Load(filepath.Join("..", "..", "config.dev.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Env = env
	cfg.HTTP.Port = port
	cfg.Data.SRDDir = filepath.Join("..", "..", "data", "srd_5.1")
	a, err := New(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	return a
}

func devRequest(t *testing.T, a *App, method, path string, body any, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(encoded))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://localhost:5173")
	r.Header.Set("X-Request-Id", "seed-test")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	a.srv.Handler.ServeHTTP(rec, r)
	return rec
}

func devDecode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP %d: %s", rec.Code, rec.Body.String())
	}
	var result T
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func devSignIn(t *testing.T, a *App, account string) (*http.Cookie, []string) {
	t.Helper()
	rec := devRequest(t, a, http.MethodPost, "/v1/dev/login", map[string]string{"account": account}, nil)
	result := devDecode[struct {
		Games []string `json:"game_ids"`
	}](t, rec)
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("development sign-in must establish a normal private session")
	}
	return cookies[0], result.Games
}

func TestDevelopmentSeedsAndRoleSwitching(t *testing.T) {
	a := developmentApp(t, config.EnvDevelopment)
	master, games := devSignIn(t, a, "master")
	if len(games) != 2 {
		t.Fatalf("games = %v", games)
	}
	player1, _ := devSignIn(t, a, "player1")
	player2, _ := devSignIn(t, a, "player2")
	for name, cookie := range map[string]*http.Cookie{"master": master, "player1": player1, "player2": player2} {
		me := devDecode[authapi.SessionResponse](t, devRequest(t, a, "GET", "/v1/auth/me", nil, cookie))
		if me.User.DisplayName != name || me.User.Anonymous {
			t.Fatal("wrong seeded identity")
		}
		list := devDecode[characterapi.ListResponse](t, devRequest(t, a, "GET", "/v1/characters", nil, cookie))
		if len(list.Characters) != 1 || list.Characters[0].Level != 1 {
			t.Fatalf("%s characters = %+v", name, list.Characters)
		}
	}
	training := devDecode[gameapi.Game](t, devRequest(t, a, "GET", "/v1/games/"+games[0], nil, master))
	if training.Role != "owner" || len(training.Entries) != 4 || len(training.Characters) != 2 {
		t.Fatalf("training = %+v", training)
	}
	if *training.Entries[3].HP != 10 || training.Entries[3].Stats.MaxHP != 10 {
		t.Fatal("seeded stub needs 10/10 HP")
	}
	view := devDecode[gameapi.Game](t, devRequest(t, a, "GET", "/v1/games/"+games[0], nil, player1))
	if !view.Entries[0].CanEdit || view.Entries[1].CanEdit || view.Entries[2].Stats != nil || view.Entries[3].Stats != nil {
		t.Fatal("seeded player ownership or monster privacy is wrong")
	}
	locked := devDecode[gameapi.Game](t, devRequest(t, a, "GET", "/v1/games/"+games[1], nil, player2))
	if locked.Entries[1].CanEdit || !*locked.Entries[1].Locked {
		t.Fatal("player2 should be locked in the second game")
	}
	entryPath := "/v1/games/" + games[1] + "/entries/" + locked.Entries[1].ID
	if rec := devRequest(t, a, "PATCH", entryPath, map[string]any{"hp": 5}, player2); rec.Code != http.StatusForbidden {
		t.Fatal("locked player edit succeeded")
	}
	devDecode[gameapi.Game](t, devRequest(t, a, "PATCH", entryPath, map[string]any{"locked": false}, master))
	devDecode[gameapi.Game](t, devRequest(t, a, "PATCH", entryPath, map[string]any{"hp": 5, "tags": []string{"Testing"}}, player2))
	player2Again, gamesAgain := devSignIn(t, a, "player2")
	if gamesAgain[0] != games[0] {
		t.Fatal("logging in must not rebuild seeds")
	}
	updated := devDecode[gameapi.Game](t, devRequest(t, a, "GET", "/v1/games/"+games[1], nil, player2Again))
	if *updated.Entries[1].HP != 5 || len(*updated.Entries[1].Tags) != 1 {
		t.Fatal("switching accounts lost game edits")
	}
	for _, unknown := range []string{"alice", "dev:master", "master "} {
		rec := devRequest(t, a, "POST", "/v1/dev/login", map[string]any{"account": unknown}, nil)
		if rec.Code != http.StatusBadRequest || len(rec.Result().Cookies()) != 0 {
			t.Fatal("development login accepted an unknown account")
		}
	}
}

func TestDevelopmentLoginAbsentInProduction(t *testing.T) {
	a := developmentApp(t, config.EnvProduction)
	rec := devRequest(t, a, "POST", "/v1/dev/login", map[string]any{"account": "master"}, nil)
	if rec.Code != http.StatusNotFound || len(rec.Result().Cookies()) != 0 {
		t.Fatal("production exposed development login")
	}
}

func TestDevelopmentLoginKeepsSameOriginGuard(t *testing.T) {
	a := developmentApp(t, config.EnvDevelopment)
	for _, origin := range []string{"https://elsewhere.example", "http://localhost:5173"} {
		r := httptest.NewRequest("POST", "/v1/dev/login", bytes.NewBufferString(`{"account":"master"}`))
		r.Header.Set("Origin", origin)
		if origin != "http://localhost:5173" {
			r.Header.Set("X-Request-Id", "seed-test")
		}
		rec := httptest.NewRecorder()
		a.srv.Handler.ServeHTTP(rec, r)
		if rec.Code != http.StatusForbidden || len(rec.Result().Cookies()) != 0 {
			t.Fatal("development login bypassed the mutation guard")
		}
	}
}

// One browser cookie jar, two ports and three tabs. This catches both the
// cross-port overwrite and the stale UI silently acting as another player.
func TestDevelopmentBrowserSessionIsolation(t *testing.T) {
	first := developmentAppAtPort(t, config.EnvDevelopment, "18082")
	second := developmentAppAtPort(t, config.EnvDevelopment, "18083")
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	send := func(app *App, port, scope, method, path, body string) *httptest.ResponseRecorder {
		address, err := url.Parse("http://localhost:" + port + path)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(method, address.String(), strings.NewReader(body))
		req.Header.Set("Origin", "http://localhost:5173")
		req.Header.Set("X-Request-Id", "isolation-test")
		req.Header.Set("X-EasyDnD-Dev-Session", scope)
		req.Header.Set("Content-Type", "application/json")
		for _, cookie := range jar.Cookies(address) {
			req.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		app.srv.Handler.ServeHTTP(rec, req)
		jar.SetCookies(address, rec.Result().Cookies())
		return rec
	}
	master, player1, player2 := strings.Repeat("a", 32), strings.Repeat("b", 32), strings.Repeat("c", 32)
	login := func(app *App, port, scope, account string) {
		t.Helper()
		rec := send(app, port, scope, "POST", "/v1/dev/login", `{"account":"`+account+`"}`)
		if rec.Code != 200 {
			t.Fatalf("login: %d %s", rec.Code, rec.Body)
		}
	}
	assertUser := func(app *App, port, scope, name string) {
		t.Helper()
		me := devDecode[authapi.SessionResponse](t, send(app, port, scope, "GET", "/v1/auth/me", ""))
		if me.User.DisplayName != name {
			t.Fatalf("got %s, want %s", me.User.DisplayName, name)
		}
	}
	login(first, "18082", master, "master")
	login(first, "18082", player1, "player1")
	login(first, "18082", player2, "player2")
	// Even identical tab selectors cannot collide across API servers.
	login(second, "18083", master, "player2")
	for scope, name := range map[string]string{master: "master", player1: "player1", player2: "player2"} {
		assertUser(first, "18082", scope, name)
	}
	assertUser(second, "18083", master, "player2")
	if rec := send(first, "18082", master, "POST", "/v1/auth/logout", "{}"); rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	if rec := send(first, "18082", master, "GET", "/v1/auth/me", ""); rec.Code != 401 {
		t.Fatal("signed-out tab still authenticated")
	}
	assertUser(first, "18082", player1, "player1")
	assertUser(first, "18082", player2, "player2")
	assertUser(second, "18083", master, "player2")
	// Plain sessions (including built previews) also need cross-port isolation.
	login(first, "18082", "", "master")
	login(second, "18083", "", "player1")
	assertUser(first, "18082", "", "master")
	assertUser(second, "18083", "", "player1")
	// An unknown selector must not fall back to another tab's normal session.
	if rec := send(first, "18082", strings.Repeat("d", 32), "GET", "/v1/auth/me", ""); rec.Code != 401 {
		t.Fatal("unknown tab inherited normal session")
	}
	assertUser(first, "18082", "", "master")
}
