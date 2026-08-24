package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// getWith issues a GET carrying cookies, which the shared do() helper cannot.
func getWith(t *testing.T, r *gin.Engine, path string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Origin", testOrigin)
	req.Header.Set("X-Request-Id", "test")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// startSSO drives the redirect half and hands back the flight cookie and the
// state the provider was given -- the two halves a real callback carries.
func startSSO(
	t *testing.T,
	r *gin.Engine,
	cookies helpers.CookieOptions,
	federation *stubFederation,
	path string,
	session ...*http.Cookie,
) (*http.Cookie, string) {
	t.Helper()
	rec := getWith(t, r, path, session...)
	if rec.Code != http.StatusFound {
		t.Fatalf("%s status = %d, want 302: %s", path, rec.Code, rec.Body)
	}
	flight := cookieNamed(rec, cookies.FlightCookieName())
	if flight == nil {
		t.Fatalf("%s set no flight cookie", path)
	}
	return flight, federation.lastState
}

// THE test for this feature's one genuinely fragile attribute.
//
// The callback is a top-level GET arriving from the provider's origin, which
// is cross-site by every definition a browser uses. Lax is sent on exactly
// that; Strict -- which the ceremony cookie beside it uses, quite correctly --
// is withheld. Tightening this to match its neighbour would break every
// federated sign-in with "no sign-in is in progress" and nothing in the logs
// to say why.
func TestFlightCookieIsLaxSoItSurvivesTheCallback(t *testing.T) {
	r, _, cookies, federation := newTestRouterWithFederation(t, &stubCeremony{}, &stubFederation{})

	flight, _ := startSSO(t, r, cookies, federation, "/v1/auth/sso/google/start")

	if flight.SameSite != http.SameSiteLaxMode {
		t.Fatalf("flight cookie SameSite = %v, want Lax; Strict is not sent on the callback navigation",
			flight.SameSite)
	}
	if !flight.HttpOnly {
		t.Error("flight cookie is not HttpOnly")
	}
	if flight.Path != "/v1/auth/sso" {
		t.Errorf("flight cookie path = %q, want /v1/auth/sso", flight.Path)
	}
	if flight.Value == "" {
		t.Error("flight cookie is empty")
	}
}

// The prefixes are not decoration: a browser enforces them, and __Secure-
// requires Secure while permitting the narrow path this cookie wants.
func TestFlightCookieIsPrefixedInProduction(t *testing.T) {
	secure := helpers.CookieOptions{Secure: true}
	insecure := helpers.CookieOptions{Secure: false}

	if got := secure.FlightCookieName(); got != "__Secure-easydnd_sso" {
		t.Errorf("secure flight cookie name = %q", got)
	}
	if got := insecure.FlightCookieName(); got != "easydnd_sso" {
		t.Errorf("development flight cookie name = %q", got)
	}
}

func TestSSOStartRedirectsCarryingStateAndPKCE(t *testing.T) {
	r, _, cookies, federation := newTestRouterWithFederation(t, &stubCeremony{}, &stubFederation{})

	rec := getWith(t, r, "/v1/auth/sso/google/start")
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}

	location := rec.Header().Get("Location")
	if !strings.HasPrefix(location, "https://accounts.example.test/authorize") {
		t.Fatalf("redirected to %q, want the provider", location)
	}
	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	for _, key := range []string{"state", "nonce", "code_challenge"} {
		if parsed.Query().Get(key) == "" {
			t.Errorf("redirect carries no %s", key)
		}
	}
	if parsed.Query().Get("state") != federation.lastState {
		t.Error("the redirect's state is not the one sealed into the flight")
	}
	if cookieNamed(rec, cookies.SessionCookieName()) != nil {
		t.Error("start set a session cookie; only the callback may")
	}
}

// An unknown provider reached by a link is still a navigation, so it comes
// back as a redirect rather than as a 404 envelope rendered on screen.
func TestSSOStartRejectsAnUnknownProvider(t *testing.T) {
	r, _, cookies, _ := newTestRouterWithFederation(t, &stubCeremony{}, &stubFederation{})

	rec := getWith(t, r, "/v1/auth/sso/nope/start")
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302: %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Location"); got != "/?auth_error=unknown_provider" {
		t.Fatalf("landed on %q", got)
	}
	if strings.Contains(rec.Body.String(), "\"error\"") {
		t.Errorf("an error envelope reached a navigation: %s", rec.Body)
	}
	if cookieNamed(rec, cookies.FlightCookieName()) != nil {
		t.Error("an unknown provider set a flight cookie")
	}
}

// The regression this pins: /link is a top-level navigation, so answering an
// expired cookie with the standard JSON envelope would replace the whole
// application with a page of braces. It is guarded just as tightly -- no
// flight cookie is issued -- but reported as a redirect.
func TestSSOLinkWithNoSessionRedirectsRatherThanReturningJSON(t *testing.T) {
	r, _, cookies, _ := newTestRouterWithFederation(t, &stubCeremony{}, &stubFederation{})

	rec := getWith(t, r, "/v1/auth/sso/google/link?return_to=%2Faccount")

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302: %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Location"); got != "/account?auth_error=session_expired" {
		t.Fatalf("landed on %q, want the page it was started from", got)
	}
	if strings.Contains(rec.Body.String(), "\"error\"") {
		t.Errorf("an error envelope reached a navigation: %s", rec.Body)
	}
	// Guarded just as tightly: nothing is started for an unauthenticated
	// caller.
	if cookieNamed(rec, cookies.FlightCookieName()) != nil {
		t.Error("an unauthenticated link attempt set a flight cookie")
	}
}

// A failure must land back where the attempt started, or a failed link dumps
// somebody on the party list with no explanation and no way back.
func TestSSOFailureReturnsToThePageItStartedFrom(t *testing.T) {
	r, _, cookies, federation := newTestRouterWithFederation(t, &stubCeremony{}, &stubFederation{})

	session := register(t, r, cookies)
	flight, _ := startSSO(t, r, cookies, federation,
		"/v1/auth/sso/google/link?return_to=%2Faccount", session)

	rec := getWith(t, r, "/v1/auth/sso/google/callback?code=abc&state=forged", flight, session)

	if got := rec.Header().Get("Location"); got != "/account?auth_error=sign_in_failed" {
		t.Fatalf("landed on %q, want /account?auth_error=sign_in_failed", got)
	}
}

// The return path on a failed start comes straight from the query string, so
// it is the one place a hostile value could reach a Location header.
func TestSSOFailureWillNotRedirectOffSite(t *testing.T) {
	r, _, _, _ := newTestRouterWithFederation(t, &stubCeremony{}, &stubFederation{})

	for _, hostile := range []string{
		"https%3A%2F%2Fevil.test",
		"%2F%09%2Fevil.test",
		"%2F%2Fevil.test",
	} {
		rec := getWith(t, r, "/v1/auth/sso/google/link?return_to="+hostile)
		location := rec.Header().Get("Location")
		if strings.Contains(location, "evil.test") {
			t.Errorf("return_to=%s escaped to %q", hostile, location)
		}
	}
}

// The provider's own error code is attacker-influenced by way of a crafted
// callback URL, so it must not choose the key the client looks up.
func TestSSOCallbackNarrowsTheProvidersRefusal(t *testing.T) {
	r, _, cookies, federation := newTestRouterWithFederation(t, &stubCeremony{}, &stubFederation{})

	flight, _ := startSSO(t, r, cookies, federation, "/v1/auth/sso/google/start")
	rec := getWith(t, r, "/v1/auth/sso/google/callback?error=totally_made_up", flight)

	if got := rec.Header().Get("Location"); got != "/?auth_error=sign_in_failed" {
		t.Fatalf("landed on %q, want the generic code", got)
	}
}

func TestSSOCallbackSignsInAndRedirectsHome(t *testing.T) {
	federation := &stubFederation{identity: user.Identity{
		Subject: "google-1", Email: "rogue@example.test", DisplayName: "Rogue",
	}}
	r, _, cookies, _ := newTestRouterWithFederation(t, &stubCeremony{}, federation)

	flight, state := startSSO(t, r, cookies, federation, "/v1/auth/sso/google/start?return_to=%2Fcharacters")
	rec := getWith(t, r, "/v1/auth/sso/google/callback?code=abc&state="+url.QueryEscape(state), flight)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302: %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Location"); got != "/characters" {
		t.Fatalf("landed on %q, want /characters", got)
	}

	session := cookieNamed(rec, cookies.SessionCookieName())
	if session == nil || session.Value == "" {
		t.Fatal("callback set no session cookie")
	}
	// Spent either way: a sealed attempt must not survive to be replayed.
	cleared := cookieNamed(rec, cookies.FlightCookieName())
	if cleared == nil || cleared.MaxAge >= 0 {
		t.Fatalf("callback did not clear the flight cookie: %+v", cleared)
	}

	// And the session actually works.
	me := getWith(t, r, "/v1/auth/me", session)
	if me.Code != http.StatusOK {
		t.Fatalf("/v1/auth/me after federated sign-in = %d: %s", me.Code, me.Body)
	}
	var body struct {
		User wireUser `json:"user"`
	}
	if err := json.Unmarshal(me.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode /me: %v", err)
	}
	if body.User.DisplayName != "Rogue" {
		t.Errorf("display name = %q, want Rogue", body.User.DisplayName)
	}
	if len(body.User.Identities) != 1 || body.User.Identities[0].Provider != "google" {
		t.Errorf("identities = %+v", body.User.Identities)
	}
	if body.User.Identities[0].Subject != "google-1" {
		t.Errorf("subject = %q", body.User.Identities[0].Subject)
	}
}

// wireUser mirrors the wire shape without importing the handler package's
// types, so a rename there shows up here as a decode failure rather than a
// silent pass.
type wireUser struct {
	DisplayName string `json:"display_name"`
	Identities  []struct {
		Provider string `json:"provider"`
		Subject  string `json:"subject"`
		Email    string `json:"email"`
	} `json:"identities"`
	Credentials []struct {
		ID string `json:"id"`
	} `json:"credentials"`
}

// A callback that does not match the attempt that started it must establish
// nothing at all.
func TestSSOCallbackWithABadStateSetsNoSession(t *testing.T) {
	r, _, cookies, federation := newTestRouterWithFederation(t, &stubCeremony{}, &stubFederation{})

	flight, _ := startSSO(t, r, cookies, federation, "/v1/auth/sso/google/start")
	rec := getWith(t, r, "/v1/auth/sso/google/callback?code=abc&state=forged", flight)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want a redirect", rec.Code)
	}
	if session := cookieNamed(rec, cookies.SessionCookieName()); session != nil && session.Value != "" {
		t.Fatal("a forged state established a session")
	}
	if got := rec.Header().Get("Location"); !strings.Contains(got, "auth_error=") {
		t.Fatalf("landed on %q, want an auth_error redirect", got)
	}
}

func TestSSOCallbackWithoutAFlightCookieFails(t *testing.T) {
	r, _, cookies, _ := newTestRouterWithFederation(t, &stubCeremony{}, &stubFederation{})

	rec := getWith(t, r, "/v1/auth/sso/google/callback?code=abc&state=anything")
	if session := cookieNamed(rec, cookies.SessionCookieName()); session != nil && session.Value != "" {
		t.Fatal("a callback with no flight cookie established a session")
	}
	if !strings.Contains(rec.Header().Get("Location"), "auth_error=") {
		t.Fatalf("landed on %q, want an auth_error redirect", rec.Header().Get("Location"))
	}
}

// Clicking Cancel at the provider is not an error worth alarming anyone about,
// but it must still land the browser back in the application rather than on a
// page of JSON.
func TestSSOCallbackHandlesAProviderRefusal(t *testing.T) {
	r, _, cookies, federation := newTestRouterWithFederation(t, &stubCeremony{}, &stubFederation{})

	flight, _ := startSSO(t, r, cookies, federation, "/v1/auth/sso/google/start")
	rec := getWith(t, r, "/v1/auth/sso/google/callback?error=access_denied", flight)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	if got := rec.Header().Get("Location"); !strings.HasPrefix(got, "/?auth_error=") {
		t.Fatalf("landed on %q", got)
	}
	if strings.Contains(rec.Header().Get("Content-Type"), "json") {
		t.Error("a top-level navigation was answered with JSON")
	}
}

// An exchange failure must not leak the provider's message into the URL: a
// sentence rendered from a query parameter is a way to put chosen text on
// somebody else's page.
func TestSSOCallbackDoesNotLeakTheFailureReason(t *testing.T) {
	federation := &stubFederation{
		err: types.NewUnauthenticatedError("client_secret is wrong for client 12345.apps.googleusercontent.com"),
	}
	r, _, cookies, _ := newTestRouterWithFederation(t, &stubCeremony{}, federation)

	flight, state := startSSO(t, r, cookies, federation, "/v1/auth/sso/google/start")
	rec := getWith(t, r, "/v1/auth/sso/google/callback?code=abc&state="+url.QueryEscape(state), flight)

	location := rec.Header().Get("Location")
	if strings.Contains(location, "client_secret") || strings.Contains(location, "googleusercontent") {
		t.Fatalf("the provider's message reached the URL: %q", location)
	}
	if location != "/?auth_error=sign_in_failed" {
		t.Fatalf("landed on %q, want /?auth_error=sign_in_failed", location)
	}
}

// --- guards ---

func TestSSOPublicRoutesNeedNoSession(t *testing.T) {
	r, _, _, _ := newTestRouterWithFederation(t, &stubCeremony{}, &stubFederation{})

	for _, path := range []string{
		"/v1/auth/providers",
		"/v1/auth/sso/google/start",
		"/v1/auth/sso/google/callback?error=access_denied",
	} {
		rec := getWith(t, r, path)
		if rec.Code == http.StatusUnauthorized {
			t.Errorf("%s answered 401; it must be reachable signed out", path)
		}
	}
}

// unlink is a fetch, not a navigation, so the envelope is right for it.
func TestSSOUnlinkRequiresASession(t *testing.T) {
	r, _, _, _ := newTestRouterWithFederation(t, &stubCeremony{}, &stubFederation{})

	if rec := post(t, r, "/v1/auth/sso/google/unlink", `{"subject":"google-1"}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("POST unlink = %d, want 401", rec.Code)
	}
}

func TestProvidersListsGoogle(t *testing.T) {
	r, _, _, _ := newTestRouterWithFederation(t, &stubCeremony{}, &stubFederation{})

	rec := getWith(t, r, "/v1/auth/providers")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	var body struct {
		Providers []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"providers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Providers) != 1 || body.Providers[0].ID != "google" {
		t.Fatalf("providers = %+v", body.Providers)
	}
	if body.Providers[0].Name != "Google" {
		t.Errorf("name = %q, want Google", body.Providers[0].Name)
	}
}

// --- linking over HTTP ---

func TestLinkingGoogleToAPasskeyAccount(t *testing.T) {
	federation := &stubFederation{identity: user.Identity{Subject: "google-1", Email: "a@example.test"}}
	r, _, cookies, _ := newTestRouterWithFederation(t, &stubCeremony{}, federation)

	session := register(t, r, cookies)

	// The name the server minted at sign-up. Linking must not touch it: the
	// provider's claims name the identity, not the account it attaches to.
	var before struct {
		User wireUser `json:"user"`
	}
	if err := json.Unmarshal(getWith(t, r, "/v1/auth/me", session).Body.Bytes(), &before); err != nil {
		t.Fatalf("decode /me: %v", err)
	}

	flight, state := startSSO(t, r, cookies, federation, "/v1/auth/sso/google/link?return_to=%2Faccount", session)
	rec := getWith(t, r,
		"/v1/auth/sso/google/callback?code=abc&state="+url.QueryEscape(state), flight, session)

	if rec.Code != http.StatusFound {
		t.Fatalf("callback status = %d: %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Location"); got != "/account" {
		t.Fatalf("landed on %q, want /account", got)
	}

	me := getWith(t, r, "/v1/auth/me", session)
	var body struct {
		User wireUser `json:"user"`
	}
	if err := json.Unmarshal(me.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode /me: %v", err)
	}
	// The original account, now reachable two ways.
	if len(body.User.Credentials) != 1 || len(body.User.Identities) != 1 {
		t.Fatalf("account has %d credentials and %d identities, want 1 and 1",
			len(body.User.Credentials), len(body.User.Identities))
	}
	if body.User.DisplayName != before.User.DisplayName {
		t.Errorf("linking renamed the account from %q to %q",
			before.User.DisplayName, body.User.DisplayName)
	}
}

func TestUnlinkRefusesTheLastWayIn(t *testing.T) {
	federation := &stubFederation{identity: user.Identity{Subject: "google-1"}}
	r, _, cookies, _ := newTestRouterWithFederation(t, &stubCeremony{}, federation)

	// A Google-only account: signing in creates one with no passkey.
	flight, state := startSSO(t, r, cookies, federation, "/v1/auth/sso/google/start")
	rec := getWith(t, r, "/v1/auth/sso/google/callback?code=abc&state="+url.QueryEscape(state), flight)
	session := cookieNamed(rec, cookies.SessionCookieName())
	if session == nil {
		t.Fatal("no session established")
	}

	unlink := post(t, r, "/v1/auth/sso/google/unlink", `{"subject":"google-1"}`, session)
	if unlink.Code != http.StatusBadRequest {
		t.Fatalf("unlink status = %d, want 400: %s", unlink.Code, unlink.Body)
	}

	// And the account is still reachable.
	if me := getWith(t, r, "/v1/auth/me", session); me.Code != http.StatusOK {
		t.Fatalf("/me after a refused unlink = %d", me.Code)
	}
}

func TestUnlinkRequiresASubject(t *testing.T) {
	r, _, cookies, _ := newTestRouterWithFederation(t, &stubCeremony{}, &stubFederation{})
	session := register(t, r, cookies)

	rec := post(t, r, "/v1/auth/sso/google/unlink", `{}`, session)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body)
	}
}

// Unlink changes something, so unlike the rest of this flow it must travel
// through the CSRF guard rather than around it.
func TestUnlinkIsRefusedCrossOrigin(t *testing.T) {
	r, _, cookies, _ := newTestRouterWithFederation(t, &stubCeremony{}, &stubFederation{})
	session := register(t, r, cookies)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/sso/google/unlink",
		strings.NewReader(`{"subject":"google-1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.test")
	req.Header.Set("X-Request-Id", "test")
	req.AddCookie(session)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-origin unlink = %d, want 403: %s", rec.Code, rec.Body)
	}
}
