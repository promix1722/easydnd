package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	catalogfile "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/adapter/sheet/hexsheet"
	"github.com/promix1722/easydnd/internal/adapter/token"
	httpapi "github.com/promix1722/easydnd/internal/api/http"
	"github.com/promix1722/easydnd/internal/api/http/helpers"
	authapi "github.com/promix1722/easydnd/internal/api/http/v1/auth"
	catalogapi "github.com/promix1722/easydnd/internal/api/http/v1/catalog"
	characterapi "github.com/promix1722/easydnd/internal/api/http/v1/character"
	"github.com/promix1722/easydnd/internal/api/http/v1/system"
	"github.com/promix1722/easydnd/internal/config"
	domain "github.com/promix1722/easydnd/internal/domain/auth"
	"github.com/promix1722/easydnd/internal/domain/user"
	authuc "github.com/promix1722/easydnd/internal/usecase/auth"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

// newFullRouter builds the whole route table over the real compendium, an
// in-memory store and a stubbed passkey ceremony, and returns a router
// together with the session cookie of a freshly registered account.
//
// Every character route now sits behind RequireSession, so a test that does
// not sign in tests the guard rather than the feature. Registering is one
// call because auth_test.go already has the helpers.
func newFullRouter(t *testing.T) (*gin.Engine, *http.Cookie) {
	r, session, _ := newFullRouterWithCeremony(t)
	return r, session
}

// newFullRouterWithCeremony also hands back the stubbed ceremony, so a test
// that needs a second account can give it a different credential id -- the
// stub issues one fixed id, and registering the same passkey twice is
// correctly refused.
func newFullRouterWithCeremony(t *testing.T) (*gin.Engine, *http.Cookie, *stubCeremony) {
	r, session, ceremony, _ := newFullRouterWithFederation(t)
	return r, session, ceremony
}

// newFullRouterWithFederation also hands back the stubbed identity provider,
// for the tests that drive a federated sign-in. It is separate from the
// three-value helper above so that the existing callers do not have to grow a
// return value they ignore -- and so that nothing is shared through package
// state, which would break the moment a test called t.Parallel.
func newFullRouterWithFederation(t *testing.T) (*gin.Engine, *http.Cookie, *stubCeremony, *stubFederation) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Env:  config.EnvDevelopment,
		HTTP: config.HTTPConfig{TrustedProxies: []string{"127.0.0.1", "::1"}},
		Auth: config.AuthConfig{
			RPID:            "easydnd.test",
			RPDisplayName:   "easydnd",
			RPOrigins:       []string{testOrigin},
			SessionSecret:   []byte("0123456789abcdef0123456789abcdef"),
			SessionTTL:      time.Hour,
			GuestSessionTTL: 15 * time.Minute,
			CeremonyTTL:     5 * time.Minute,
			SecureCookies:   false,
		},
	}
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))

	ceremony := &stubCeremony{}
	federation := &stubFederation{identity: user.Identity{Subject: "google-1", Email: "g@example.test"}}
	authService := authuc.NewService(
		memory.NewUserRepository(),
		ceremony,
		token.NewSigner(cfg.Auth.SessionSecret, cfg.Auth.SessionTTL),
		map[user.Provider]domain.Federation{user.ProviderGoogle: federation},
		authuc.Config{
			SessionTTL:      cfg.Auth.SessionTTL,
			GuestSessionTTL: cfg.Auth.GuestSessionTTL,
			CeremonyTTL:     cfg.Auth.CeremonyTTL,
		},
		log,
	)
	cookies := helpers.CookieOptions{Secure: cfg.Auth.SecureCookies}

	source := catalogfile.NewSource(filepath.Join("..", "..", "..", "data", "srd_5.1"))
	characterService := charuc.NewService(memory.NewCharacterRepository(), source, hexsheet.NewImporter(), log)

	r, err := httpapi.NewRouter(cfg, log, httpapi.Handlers{
		System:        system.New(testVersion),
		Auth:          authapi.New(authService, cookies),
		Authenticator: authService,
		Catalog:       catalogapi.New(source, log),
		Character:     characterapi.New(characterService, log),
	})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	return r, register(t, r, cookies), ceremony, federation
}

// signInWithGoogle drives a full federated sign-in and returns the session
// cookie, the way register does for a passkey.
func signInWithGoogle(
	t *testing.T,
	r *gin.Engine,
	cookies helpers.CookieOptions,
	federation *stubFederation,
	subject string,
) *http.Cookie {
	t.Helper()
	federation.identity = user.Identity{Subject: subject, Email: subject + "@example.test"}

	begin := getWith(t, r, "/v1/auth/sso/google/start")
	flight := cookieNamed(begin, cookies.FlightCookieName())
	if flight == nil {
		t.Fatal("sso/start set no flight cookie")
	}

	finish := getWith(t, r,
		"/v1/auth/sso/google/callback?code=abc&state="+url.QueryEscape(federation.lastState), flight)
	session := cookieNamed(finish, cookies.SessionCookieName())
	if session == nil {
		t.Fatalf("sso/callback set no session cookie; landed on %q",
			finish.Header().Get("Location"))
	}
	return session
}

// The point of signing in at all: a character has an owner, and an account
// that arrived through Google must own its characters exactly as one that
// arrived through a passkey does. This is the seam where a federated sign-in
// meets domain.OwnerID, and nothing else exercises it end to end.
func TestAGoogleAccountOwnsItsCharacters(t *testing.T) {
	r, _, _, federation := newFullRouterWithFederation(t)
	cookies := helpers.CookieOptions{Secure: false}

	alice := signInWithGoogle(t, r, cookies, federation, "google-alice")

	created := post(t, r, "/v1/characters",
		`{"name":"Rogue","method":"standard_array","abilities":{"str":10,"dex":15,"con":13,"int":12,"wis":14,"cha":8}}`,
		alice)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", created.Code, created.Body)
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode create: %v", err)
	}

	// Alice sees it.
	list := getWith(t, r, "/v1/characters", alice)
	if !strings.Contains(list.Body.String(), body.ID) {
		t.Fatalf("owner's listing does not contain %q: %s", body.ID, list.Body)
	}

	// A second Google account is a different person, and must not.
	bob := signInWithGoogle(t, r, cookies, federation, "google-bob")

	bobsList := getWith(t, r, "/v1/characters", bob)
	if strings.Contains(bobsList.Body.String(), body.ID) {
		t.Fatalf("another account can see the character: %s", bobsList.Body)
	}
	// 404 rather than 403: a 403 on somebody else's id confirms it exists.
	if got := getWith(t, r, "/v1/characters/"+body.ID+"/sheet", bob); got.Code != http.StatusNotFound {
		t.Fatalf("another account reading the sheet = %d, want 404", got.Code)
	}

	// And signing in again through Google lands on the same party.
	again := signInWithGoogle(t, r, cookies, federation, "google-alice")
	if !strings.Contains(getWith(t, r, "/v1/characters", again).Body.String(), body.ID) {
		t.Fatal("signing in again through Google did not resolve the same account")
	}
}

// send issues a same-origin request carrying the session, the way the real
// client does.
//
// Three headers are not optional. SameOrigin guards the whole of /v1, so a
// state-changing method needs Origin; the same middleware requires the
// correlation id the client mints on every call; and the session cookie is
// what RequireSession reads. A test that omits any of them tests a guard
// rather than the feature -- which is worth knowing, and is what the
// unauthenticated test below does deliberately.
func send(
	t *testing.T,
	r *gin.Engine,
	session *http.Cookie,
	method, path string,
	body any,
	headers ...map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshalling the request: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Origin", testOrigin)
	req.Header.Set("X-Request-Id", "test")
	for _, extra := range headers {
		for name, value := range extra {
			req.Header.Set(name, value)
		}
	}
	if session != nil {
		req.AddCookie(session)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decoding %s: %v", rec.Body.String(), err)
	}
	return out
}

func TestCatalogManifestIndexesEveryCollection(t *testing.T) {
	r, session := newFullRouter(t)

	rec := send(t, r, session, http.MethodGet, "/v1/catalog", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}
	got := decode[catalogapi.ManifestResponse](t, rec)

	if got.Ruleset != "2014" {
		t.Errorf("ruleset = %q, want 2014", got.Ruleset)
	}
	if len(got.Collections) != len(catalogapi.Collections()) {
		t.Errorf("collections = %d, want %d", len(got.Collections), len(catalogapi.Collections()))
	}
	// Every name the manifest lists must actually be fetchable. This is the
	// contract the manifest exists to offer -- read it once and know every
	// URL under /v1/catalog without a hardcoded list.
	for _, collection := range got.Collections {
		if collection.Count == 0 {
			t.Errorf("collection %q is empty", collection.Name)
		}
		rec := send(t, r, session, http.MethodGet, "/v1/catalog/"+collection.Name, nil)
		if rec.Code != http.StatusOK {
			t.Errorf("GET /v1/catalog/%s = %d, want 200", collection.Name, rec.Code)
		}
	}
}

func TestCatalogNegotiatesLocale(t *testing.T) {
	r, session := newFullRouter(t)

	type named struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
	}
	races := func(path string, headers ...map[string]string) []named {
		t.Helper()
		return decode[[]named](t, send(t, r, session, http.MethodGet, path, nil, headers...))
	}

	english := races("/v1/catalog/races?slugs=elf")
	if len(english) != 1 || english[0].Name != "Elf" {
		t.Fatalf("english = %+v, want one Elf", english)
	}

	russian := races("/v1/catalog/races?slugs=elf",
		map[string]string{"Accept-Language": "ru-RU,ru;q=0.9,en;q=0.8"})
	if len(russian) != 1 || russian[0].Name == "Elf" {
		t.Errorf("russian = %+v, want a translated name", russian)
	}

	// The query parameter beats the header, because a language switcher is
	// something a user clicks and a page cannot rewrite Accept-Language.
	override := races("/v1/catalog/races?slugs=elf&locale=ru",
		map[string]string{"Accept-Language": "en"})
	if len(override) != 1 || override[0].Name == "Elf" {
		t.Errorf("override = %+v, want the query parameter to win", override)
	}
}

func TestUnknownCollectionIsNotFound(t *testing.T) {
	r, session := newFullRouter(t)
	rec := send(t, r, session, http.MethodGet, "/v1/catalog/dragons", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// The whole build flow through the API, in the shape a client actually sends
// it: read the prompts, answer one, read the prompts again.
func TestCharacterBuildFlow(t *testing.T) {
	r, session := newFullRouter(t)

	rec := send(t, r, session, http.MethodPost, "/v1/characters", map[string]any{
		"name": "Сахарок",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d, want 201: %s", rec.Code, rec.Body)
	}
	created := decode[characterapi.CreateResponse](t, rec)
	id := created.ID
	if created.Seq != 1 {
		t.Errorf("seq = %d, want 1", created.Seq)
	}

	// Creation carries a name and nothing else, so the scores are the first
	// thing the character is asked for -- an ordinary open choice with an
	// entry of its own, rather than eight selections bundled into the init.
	prompts := decode[characterapi.PromptsResponse](t,
		send(t, r, session, http.MethodGet, "/v1/characters/"+id+"/prompts", nil))
	if prompts.Complete {
		t.Error("a character with nothing but a name reads as complete")
	}
	if first := firstRequired(t, prompts); first.Choice.Prompt != "character/abilities" {
		t.Fatalf("first required prompt = %q, want character/abilities", first.Choice.Prompt)
	}

	rec = send(t, r, session, http.MethodPost, "/v1/characters/"+id+"/events", map[string]any{
		"expectedSeq": 1,
		"events":      []map[string]any{scoresEvent()},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("scores = %d, want 200: %s", rec.Code, rec.Body)
	}
	// The base array, unmodified: no race yet.
	if got := decode[characterapi.WriteResponse](t, rec).Sheet.Abilities.Scores["dex"]; got != 15 {
		t.Errorf("dexterity = %d, want the base 15", got)
	}

	prompts = decode[characterapi.PromptsResponse](t,
		send(t, r, session, http.MethodGet, "/v1/characters/"+id+"/prompts", nil))
	first := firstRequired(t, prompts)
	if first.Choice.Prompt != "character/race" {
		t.Fatalf("first required prompt = %q, want character/race", first.Choice.Prompt)
	}
	// The prompt says what event carries its answer, so the client never has
	// to know that a race is a race event and a fourth level is a level one.
	if first.Event.Type != "race" {
		t.Errorf("character/race posts a %q event, want race", first.Event.Type)
	}

	rec = send(t, r, session, http.MethodPost, "/v1/characters/"+id+"/events", map[string]any{
		"expectedSeq": 2,
		"events": []map[string]any{{
			"type": "race",
			"ref":  "race:half-elf",
			"choices": []map[string]any{
				{"prompt": "half-elf/ability-bonus/0", "picks": []string{"dex", "con"}},
			},
		}},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("append = %d, want 200: %s", rec.Code, rec.Body)
	}
	written := decode[characterapi.WriteResponse](t, rec)
	if written.Seq != 3 {
		t.Errorf("seq = %d, want 3", written.Seq)
	}
	// The write returns the new sheet, which is why the client needs no
	// cache invalidation: the response is the invalidation.
	if got := written.Sheet.Abilities.Scores["dex"]; got != 16 {
		t.Errorf("dexterity = %d, want 16", got)
	}
	if got := written.Sheet.Abilities.Modifiers["dex"]; got != 3 {
		t.Errorf("dexterity modifier = %d, want 3", got)
	}

	// The trait's prompt did not exist before the race was chosen.
	prompts = decode[characterapi.PromptsResponse](t,
		send(t, r, session, http.MethodGet, "/v1/characters/"+id+"/prompts", nil))
	if !hasPrompt(prompts, "skill-versatility/proficiency/0") {
		t.Error("choosing half-elf did not open Skill Versatility's prompt")
	}
}

// The events route serves the record itself, which the log screen in the web
// client reads directly -- so the shape of what it returns is a contract, not
// an implementation detail.
func TestEventsReturnsTheLog(t *testing.T) {
	r, session := newFullRouter(t)
	id := createCharacter(t, r, session)

	rec := send(t, r, session, http.MethodPost, "/v1/characters/"+id+"/events", map[string]any{
		"expectedSeq": 1,
		"events": []map[string]any{scoresEvent(), {
			"type": "race",
			"ref":  "race:half-elf",
			// A source the server must not repeat: it writes its own, from
			// the prompt the event turns out to answer.
			"source": "background",
			"choices": []map[string]any{
				{"prompt": "half-elf/ability-bonus/0", "picks": []string{"dex", "con"}},
			},
		}},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("append = %d, want 200: %s", rec.Code, rec.Body)
	}

	got := decode[characterapi.EventsResponse](t, readLog(t, r, session, id))
	if got.Seq != 3 {
		t.Errorf("seq = %d, want 3", got.Seq)
	}
	if len(got.Events) != 3 {
		t.Fatalf("events = %d, want 3", len(got.Events))
	}

	// Creation seeds the log, and what it seeds is the name -- one entry, one
	// selection, so the name is a thing a player can point at and change.
	if got.Events[0].Type != "init" || got.Events[0].Seq != 1 {
		t.Errorf("first event = %q at %d, want init at 1", got.Events[0].Type, got.Events[0].Seq)
	}
	if len(got.Events[0].Changes) != 1 || got.Events[0].Changes[0].Path != "identity.name" {
		t.Errorf("init changes = %+v, want the name alone", got.Events[0].Changes)
	}
	if got.Events[0].Source != "identity" {
		t.Errorf("init source = %q, want identity", got.Events[0].Source)
	}
	// The server stamps the time, but not on the event Create seeds: a client
	// reading the log has to render an event with no time at all.
	if got.Events[0].At != "" {
		t.Errorf("init At = %q, want empty", got.Events[0].At)
	}

	if got.Events[1].Source != "abilities" {
		t.Errorf("scores source = %q, want abilities", got.Events[1].Source)
	}

	third := got.Events[2]
	if third.Seq != 3 || third.Type != "race" || third.Ref != "race:half-elf" {
		t.Errorf("third event = %+v, want the race at seq 3", third)
	}
	if third.Source != "race" {
		t.Errorf("race source = %q, want race -- the body said background", third.Source)
	}
	if third.At == "" {
		t.Error("an appended event has no At, so the log cannot say when it happened")
	}
	if len(third.Choices) != 1 || third.Choices[0].Prompt != "half-elf/ability-bonus/0" {
		t.Errorf("choices = %+v, want the answer as it was posted", third.Choices)
	}
}

// readLog reads the log back, kept apart so the assertions above read as one list.
func readLog(t *testing.T, r *gin.Engine, session *http.Cookie, id string) *httptest.ResponseRecorder {
	t.Helper()
	rec := send(t, r, session, http.MethodGet, "/v1/characters/"+id+"/events", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("events = %d, want 200: %s", rec.Code, rec.Body)
	}
	return rec
}

// Optimistic concurrency: the whole log is one record, so a client writing
// against a sequence that has moved must be told rather than silently
// discarding whatever moved it.
func TestAppendRejectsAStaleSequence(t *testing.T) {
	r, session := newFullRouter(t)
	id := createCharacter(t, r, session)

	rec := send(t, r, session, http.MethodPost, "/v1/characters/"+id+"/events", map[string]any{
		"expectedSeq": 9,
		"events":      []map[string]any{{"type": "note", "note": "x"}},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body)
	}
	if code := errorCode(t, rec); code != "validation_error" {
		t.Errorf("code = %q, want validation_error", code)
	}
}

// A bad answer names the prompt it failed on, so a client can point at the
// control that produced it rather than showing a banner.
func TestBadAnswerIsAFieldError(t *testing.T) {
	r, session := newFullRouter(t)
	id := createCharacter(t, r, session)

	rec := send(t, r, session, http.MethodPost, "/v1/characters/"+id+"/events", map[string]any{
		"expectedSeq": 1,
		"events": []map[string]any{{
			"type": "race",
			"ref":  "race:half-elf",
			// Charisma is the half-elf's fixed +2 and is not on offer.
			"choices": []map[string]any{
				{"prompt": "half-elf/ability-bonus/0", "picks": []string{"dex", "cha"}},
			},
		}},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body)
	}
	body := decode[struct {
		Error struct {
			Code   string `json:"code"`
			Fields []struct {
				Field string `json:"field"`
				Rule  string `json:"rule"`
			} `json:"fields"`
		} `json:"error"`
	}](t, rec)
	if body.Error.Code != "field_validation_error" {
		t.Fatalf("code = %q, want field_validation_error", body.Error.Code)
	}
	if len(body.Error.Fields) == 0 {
		t.Fatal("no fields named")
	}
	if body.Error.Fields[0].Rule != "option" {
		t.Errorf("rule = %q, want option", body.Error.Fields[0].Rule)
	}
}

// Undo, and the one thing undo may never do.
func TestTruncateUndoesAndProtectsInit(t *testing.T) {
	r, session := newFullRouter(t)
	id := createCharacter(t, r, session)

	send(t, r, session, http.MethodPost, "/v1/characters/"+id+"/events", map[string]any{
		"expectedSeq": 1,
		"events":      []map[string]any{{"type": "race", "ref": "race:half-elf"}},
	})

	rec := send(t, r, session, http.MethodDelete,
		"/v1/characters/"+id+"/events?after=1&expectedSeq=2", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("truncate = %d, want 200: %s", rec.Code, rec.Body)
	}
	written := decode[characterapi.WriteResponse](t, rec)
	if written.Seq != 1 {
		t.Errorf("seq = %d, want 1", written.Seq)
	}
	if written.Sheet.Identity.Race != "" {
		t.Errorf("race = %q, want it undone", written.Sheet.Identity.Race)
	}

	rec = send(t, r, session, http.MethodDelete, "/v1/characters/"+id+"/events?after=0&expectedSeq=1", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("dropping the init event = %d, want 400", rec.Code)
	}

	// Both parameters are required: a truncation with no expected sequence
	// is a deletion with no concurrency check.
	rec = send(t, r, session, http.MethodDelete, "/v1/characters/"+id+"/events?after=1", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("truncate without expectedSeq = %d, want 400", rec.Code)
	}
}

// The route pair that makes a choice changeable, over HTTP: replace one entry
// by position, see what it cost, and see it not cost anything until asked.
func TestReplaceAndDeleteAnEntry(t *testing.T) {
	r, session := newFullRouter(t)
	id := createCharacter(t, r, session)

	// A dwarf who took the hill dwarf subrace, so that changing the race has
	// something real to orphan.
	rec := send(t, r, session, http.MethodPost, "/v1/characters/"+id+"/events", map[string]any{
		"expectedSeq": 1,
		"events": []map[string]any{
			scoresEvent(),
			{"type": "race", "ref": "race:dwarf"},
			{"type": "subrace", "ref": "subrace:hill-dwarf"},
		},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("build = %d, want 200: %s", rec.Code, rec.Body)
	}
	if seq := decode[characterapi.WriteResponse](t, rec).Seq; seq != 4 {
		t.Fatalf("seq = %d, want 4", seq)
	}

	replacement := map[string]any{
		"expectedSeq": 4,
		"event":       map[string]any{"type": "race", "ref": "race:half-elf"},
	}

	// The dry run: the full cost, and nothing written.
	rec = send(t, r, session, http.MethodPut,
		"/v1/characters/"+id+"/events/3?dryRun=true", replacement)
	if rec.Code != http.StatusOK {
		t.Fatalf("dry run = %d, want 200: %s", rec.Code, rec.Body)
	}
	preview := decode[characterapi.WriteResponse](t, rec)
	if len(preview.Dropped) != 1 {
		t.Fatalf("dropped = %+v, want the orphaned subrace", preview.Dropped)
	}
	if preview.Dropped[0].Seq != 4 || preview.Dropped[0].Reason != "not-offered" {
		t.Errorf("dropped = %+v, want entry 4 as not-offered", preview.Dropped[0])
	}
	if preview.Dropped[0].Source != "race" {
		t.Errorf("dropped source = %q, want race", preview.Dropped[0].Source)
	}
	stored := decode[characterapi.EventsResponse](t, readLog(t, r, session, id))
	if stored.Seq != 4 || stored.Events[2].Ref != "race:dwarf" {
		t.Fatalf("the dry run wrote to the log: %+v", stored)
	}

	// The commit: the same request without the flag, reporting the same cost.
	rec = send(t, r, session, http.MethodPut, "/v1/characters/"+id+"/events/3", replacement)
	if rec.Code != http.StatusOK {
		t.Fatalf("replace = %d, want 200: %s", rec.Code, rec.Body)
	}
	written := decode[characterapi.WriteResponse](t, rec)
	if len(written.Dropped) != len(preview.Dropped) {
		t.Errorf("committed %d drops, previewed %d", len(written.Dropped), len(preview.Dropped))
	}
	if written.Seq != 3 {
		t.Errorf("seq = %d, want 3: the subrace entry is gone", written.Seq)
	}
	if written.Sheet.Identity.Race != "half-elf" || written.Sheet.Identity.Subrace != "" {
		t.Errorf("identity = %+v, want a half-elf with no subrace", written.Sheet.Identity)
	}

	// And the same log, addressed the same way, with nothing to put back.
	rec = send(t, r, session, http.MethodDelete,
		"/v1/characters/"+id+"/events/3?expectedSeq=3", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete = %d, want 200: %s", rec.Code, rec.Body)
	}
	if got := decode[characterapi.WriteResponse](t, rec); got.Sheet.Identity.Race != "" {
		t.Errorf("race = %q, want the entry removed", got.Sheet.Identity.Race)
	}
}

// The guards on the route: the position, the concurrency token, and the flag.
func TestReplaceGuards(t *testing.T) {
	r, session := newFullRouter(t)
	id := createCharacter(t, r, session)

	init := map[string]any{"type": "init", "changes": []map[string]any{{
		"path": "identity.name", "op": "set",
		"value": map[string]any{"kind": "string", "string": "Рурик"},
	}}}

	for _, tt := range []struct {
		name string
		path string
		body any
		want int
	}{
		{
			name: "the opening entry, replaced by another init",
			path: "/v1/characters/" + id + "/events/1",
			body: map[string]any{"expectedSeq": 1, "event": init},
			want: http.StatusOK,
		},
		{
			name: "the opening entry, replaced by anything else",
			path: "/v1/characters/" + id + "/events/1",
			body: map[string]any{"expectedSeq": 1, "event": map[string]any{"type": "note", "note": "x"}},
			want: http.StatusBadRequest,
		},
		{
			name: "an entry the log does not have",
			path: "/v1/characters/" + id + "/events/9",
			body: map[string]any{"expectedSeq": 1, "event": init},
			want: http.StatusBadRequest,
		},
		{
			name: "a sequence that has moved",
			path: "/v1/characters/" + id + "/events/1",
			body: map[string]any{"expectedSeq": 7, "event": init},
			want: http.StatusBadRequest,
		},
		{
			// Anything but true or false, because ?dryRun=yes silently read
			// as a commit is the change the player asked to preview.
			name: "a dry-run flag that is neither",
			path: "/v1/characters/" + id + "/events/1?dryRun=yes",
			body: map[string]any{"expectedSeq": 1, "event": init},
			want: http.StatusBadRequest,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(t, r, session, http.MethodPut, tt.path, tt.body)
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d: %s", rec.Code, tt.want, rec.Body)
			}
		})
	}

	// Removing the opening entry is not a thing either: a character with no
	// opening state is not an earlier version of itself.
	rec := send(t, r, session, http.MethodDelete, "/v1/characters/"+id+"/events/1?expectedSeq=1", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("deleting the init event = %d, want 400", rec.Code)
	}
	// And the concurrency token is required, exactly as it is on a truncation.
	rec = send(t, r, session, http.MethodDelete, "/v1/characters/"+id+"/events/1", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("delete without expectedSeq = %d, want 400", rec.Code)
	}
}

func TestListAndDelete(t *testing.T) {
	r, session := newFullRouter(t)
	id := createCharacter(t, r, session)

	listed := decode[characterapi.ListResponse](t, send(t, r, session, http.MethodGet, "/v1/characters", nil))
	if len(listed.Characters) != 1 || listed.Characters[0].Name != "Сахарок" {
		t.Fatalf("listing = %+v, want one named character", listed.Characters)
	}

	if rec := send(t, r, session, http.MethodDelete, "/v1/characters/"+id, nil); rec.Code != http.StatusNoContent {
		t.Errorf("delete = %d, want 204", rec.Code)
	}
	if rec := send(t, r, session, http.MethodGet, "/v1/characters/"+id, nil); rec.Code != http.StatusNotFound {
		t.Errorf("get after delete = %d, want 404", rec.Code)
	}
}

// createCharacter creates one the way the route now takes it: a name, and
// nothing that has a prompt of its own.
func createCharacter(t *testing.T, r *gin.Engine, session *http.Cookie) string {
	t.Helper()
	rec := send(t, r, session, http.MethodPost, "/v1/characters", map[string]any{
		"name": "Сахарок",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d, want 201: %s", rec.Code, rec.Body)
	}
	return decode[characterapi.CreateResponse](t, rec).ID
}

// scoresEvent is the answer to character/abilities: six scores and the method
// they were generated with, as one entry of its own.
func scoresEvent() map[string]any {
	set := func(path string, value any) map[string]any {
		kind := "int"
		if _, ok := value.(string); ok {
			kind = "slug"
		}
		return map[string]any{
			"path": path, "op": "set",
			"value": map[string]any{"kind": kind, kind: value},
		}
	}
	return map[string]any{
		"type": "change",
		"changes": []map[string]any{
			set("abilities.method", "point-buy"),
			set("abilities.str", 10), set("abilities.dex", 15), set("abilities.con", 13),
			set("abilities.int", 10), set("abilities.wis", 12), set("abilities.cha", 12),
		},
	}
}

func firstRequired(t *testing.T, p characterapi.PromptsResponse) characterapi.Prompt {
	t.Helper()
	for _, prompt := range p.Prompts {
		if !prompt.Optional {
			return prompt
		}
	}
	t.Fatal("no required prompt")
	return characterapi.Prompt{}
}

func hasPrompt(p characterapi.PromptsResponse, id string) bool {
	for _, prompt := range p.Prompts {
		if prompt.Choice.Prompt == id {
			return true
		}
	}
	return false
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	body := decode[struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}](t, rec)
	return body.Error.Code
}

// Every character route is behind RequireSession. This is the test that
// notices an endpoint declared one line above the guarded group, which is the
// failure the router's own comment warns about.
func TestCharacterRoutesRequireASession(t *testing.T) {
	r, session := newFullRouter(t)
	id := createCharacter(t, r, session)

	for _, route := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/v1/characters"},
		{http.MethodPost, "/v1/characters"},
		{http.MethodGet, "/v1/characters/" + id},
		{http.MethodDelete, "/v1/characters/" + id},
		{http.MethodGet, "/v1/characters/" + id + "/sheet"},
		{http.MethodGet, "/v1/characters/" + id + "/prompts"},
		{http.MethodGet, "/v1/characters/" + id + "/events"},
		{http.MethodPost, "/v1/characters/" + id + "/events"},
		{http.MethodDelete, "/v1/characters/" + id + "/events?after=1&expectedSeq=1"},
		{http.MethodPut, "/v1/characters/" + id + "/events/1"},
		{http.MethodDelete, "/v1/characters/" + id + "/events/1?expectedSeq=1"},
		{http.MethodGet, "/v1/catalog"},
		{http.MethodGet, "/v1/catalog/races"},
	} {
		rec := send(t, r, nil, route.method, route.path, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without a session = %d, want 401", route.method, route.path, rec.Code)
		}
	}
}

// A character is somebody's. Another signed-in account must not be able to
// read it, write to it or delete it -- and must not be able to learn that it
// exists, which is why the answer is 404 rather than 403.
func TestAnotherAccountCannotReachTheCharacter(t *testing.T) {
	r, session, ceremony := newFullRouterWithCeremony(t)
	id := createCharacter(t, r, session)

	// A second account on the same router, with its own passkey.
	ceremony.credentialID = "second-credential"
	intruder := register(t, r, helpers.CookieOptions{Secure: false})

	for _, route := range []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/v1/characters/" + id, nil},
		{http.MethodGet, "/v1/characters/" + id + "/sheet", nil},
		{http.MethodGet, "/v1/characters/" + id + "/prompts", nil},
		{http.MethodGet, "/v1/characters/" + id + "/events", nil},
		{http.MethodDelete, "/v1/characters/" + id, nil},
		{
			http.MethodPost, "/v1/characters/" + id + "/events",
			map[string]any{"expectedSeq": 1, "events": []map[string]any{{"type": "note", "note": "mine"}}},
		},
		{http.MethodDelete, "/v1/characters/" + id + "/events?after=1&expectedSeq=1", nil},
		{
			http.MethodPut, "/v1/characters/" + id + "/events/1",
			map[string]any{"expectedSeq": 1, "event": map[string]any{"type": "init"}},
		},
		{http.MethodDelete, "/v1/characters/" + id + "/events/1?expectedSeq=1", nil},
	} {
		rec := send(t, r, intruder, route.method, route.path, route.body)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s as another account = %d, want 404", route.method, route.path, rec.Code)
		}
	}

	// The intruder's own listing is empty, and the owner's character survived.
	listed := decode[characterapi.ListResponse](t, send(t, r, intruder, http.MethodGet, "/v1/characters", nil))
	if len(listed.Characters) != 0 {
		t.Errorf("the intruder sees %d characters, want 0", len(listed.Characters))
	}
	if rec := send(t, r, session, http.MethodGet, "/v1/characters/"+id, nil); rec.Code != http.StatusOK {
		t.Errorf("the owner's character = %d, want 200", rec.Code)
	}
}

// newFullRouterAsGuest is newFullRouter signed in anonymously instead.
//
// It exists for one test, but that test is the load-bearing one: every
// character route resolves its owner from the session, and a guest is the only
// session whose owner has no row behind it.
func newFullRouterAsGuest(t *testing.T) (*gin.Engine, *http.Cookie) {
	t.Helper()
	r, _, _ := newFullRouterWithCeremony(t)
	return r, guest(t, r, helpers.CookieOptions{Secure: false})
}

// A guest owns characters like anybody else. Nothing in the character path
// touches the account store, and this is what proves it stays that way.
func TestGuestCanCreateAndListCharacters(t *testing.T) {
	r, session := newFullRouterAsGuest(t)

	created := send(t, r, session, http.MethodPost, "/v1/characters", map[string]any{"name": "Ghost"})
	if created.Code != http.StatusCreated && created.Code != http.StatusOK {
		t.Fatalf("create status = %d: %s", created.Code, created.Body)
	}

	listed := send(t, r, session, http.MethodGet, "/v1/characters", nil)
	if listed.Code != http.StatusOK {
		t.Fatalf("list status = %d: %s", listed.Code, listed.Body)
	}
	var body struct {
		Characters []struct {
			ID string `json:"id"`
		} `json:"characters"`
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", listed.Body.String(), err)
	}
	if len(body.Characters) != 1 {
		t.Fatalf("guest sees %d characters, want the 1 they just made", len(body.Characters))
	}
}

// Two guests are two owners. They share no row, so the only thing keeping them
// apart is the id in the token.
func TestGuestsDoNotSeeEachOthersCharacters(t *testing.T) {
	r, first := newFullRouterAsGuest(t)
	second := guest(t, r, helpers.CookieOptions{Secure: false})

	if created := send(t, r, first, http.MethodPost, "/v1/characters",
		map[string]any{"name": "Ghost"}); created.Code >= http.StatusBadRequest {
		t.Fatalf("create status = %d: %s", created.Code, created.Body)
	}

	listed := send(t, r, second, http.MethodGet, "/v1/characters", nil)
	if listed.Code != http.StatusOK {
		t.Fatalf("list status = %d: %s", listed.Code, listed.Body)
	}
	var body struct {
		Characters []json.RawMessage `json:"characters"`
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", listed.Body.String(), err)
	}
	if len(body.Characters) != 0 {
		t.Errorf("second guest sees %d characters belonging to the first", len(body.Characters))
	}
}
