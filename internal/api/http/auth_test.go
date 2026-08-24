package httpapi_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/adapter/token"
	httpapi "github.com/promix1722/easydnd/internal/api/http"
	"github.com/promix1722/easydnd/internal/api/http/helpers"
	"github.com/promix1722/easydnd/internal/api/http/middleware"
	authapi "github.com/promix1722/easydnd/internal/api/http/v1/auth"
	"github.com/promix1722/easydnd/internal/api/http/v1/system"
	"github.com/promix1722/easydnd/internal/config"
	domain "github.com/promix1722/easydnd/internal/domain/auth"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
	authuc "github.com/promix1722/easydnd/internal/usecase/auth"
)

const testOrigin = "https://easydnd.test"

// stubCeremony replaces the WebAuthn adapter so the HTTP layer can be
// exercised without an authenticator. It always succeeds, which is the point:
// these tests are about cookies, guards and status codes, not about
// cryptography, which internal/adapter/token and the usecase cover.
type stubCeremony struct {
	credentialID string
}

func (s *stubCeremony) BeginRegistration(user.User) ([]byte, []byte, error) {
	return []byte(`{"publicKey":{"challenge":"stub"}}`), []byte(`{"state":true}`), nil
}

func (s *stubCeremony) FinishRegistration(_ user.User, _, _ []byte) (user.Credential, error) {
	return user.Credential{ID: []byte(s.id()), PublicKey: []byte("pk")}, nil
}

func (s *stubCeremony) BeginLogin() ([]byte, []byte, error) {
	return []byte(`{"publicKey":{"challenge":"stub"}}`), []byte(`{"state":true}`), nil
}

func (s *stubCeremony) FinishLogin(_, _ []byte, lookup domain.UserLookup) (user.ID, user.Credential, error) {
	found, err := lookup([]byte(s.id()), nil)
	if err != nil {
		return "", user.Credential{}, err
	}
	return found.ID, user.Credential{ID: []byte(s.id()), SignCount: 1}, nil
}

func (s *stubCeremony) id() string {
	if s.credentialID == "" {
		return "stub-credential"
	}
	return s.credentialID
}

// stubFederation stands in for a real identity provider. It never reaches the
// network, so the state, nonce and PKCE plumbing can be exercised without one.
type stubFederation struct {
	// lastState, lastNonce and lastVerifier record what the usecase handed
	// out, so a test can prove the redirect carries them.
	lastState    string
	lastNonce    string
	lastVerifier string

	// identity is what Exchange returns; err, if set, is returned instead.
	identity user.Identity
	err      error

	// unreachable makes AuthCodeURL return "", which is how the port says it
	// could not reach its issuer.
	unreachable bool
}

func (f *stubFederation) AuthCodeURL(state, nonce, verifier string) string {
	f.lastState, f.lastNonce, f.lastVerifier = state, nonce, verifier
	if f.unreachable {
		return ""
	}
	return "https://accounts.example.test/authorize?state=" + state +
		"&nonce=" + nonce + "&code_challenge=" + challengeFrom(verifier)
}

// challengeFrom mirrors what a real adapter does with the verifier, so the
// stubbed redirect looks like the real one and no test can come to depend on
// the verifier appearing in a URL.
func challengeFrom(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (f *stubFederation) Exchange(_ context.Context, _, nonce, _ string) (user.Identity, error) {
	if f.err != nil {
		return user.Identity{}, f.err
	}
	if nonce != f.lastNonce {
		return user.Identity{}, types.NewUnauthenticatedError("nonce mismatch")
	}
	identity := f.identity
	if identity.Subject == "" {
		identity.Subject = "google-subject"
	}
	return identity, nil
}

func newTestRouterWith(t *testing.T, ceremony *stubCeremony) (*gin.Engine, *authuc.Service, helpers.CookieOptions) {
	t.Helper()
	r, svc, cookies, _ := newTestRouterOver(t, ceremony, memory.NewUserRepository(), &stubFederation{})
	return r, svc, cookies
}

// newTestRouterWithFederation builds over the memory store but hands back the
// stubbed identity provider, for the tests that drive a federated sign-in.
func newTestRouterWithFederation(
	t *testing.T,
	ceremony *stubCeremony,
	federation *stubFederation,
) (*gin.Engine, *authuc.Service, helpers.CookieOptions, *stubFederation) {
	return newTestRouterOver(t, ceremony, memory.NewUserRepository(), federation)
}

// newTestRouterOver builds the same router over a caller-supplied account
// store, so the durability test below can point the real HTTP stack at
// Postgres. Everything else -- the signing key, the cookie options, the stub
// ceremony -- is identical, which is what makes the two comparable.
func newTestRouterOver(
	t *testing.T,
	ceremony *stubCeremony,
	repo user.Repository,
	federation *stubFederation,
) (*gin.Engine, *authuc.Service, helpers.CookieOptions, *stubFederation) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Env:  config.EnvDevelopment,
		HTTP: config.HTTPConfig{TrustedProxies: []string{"127.0.0.1", "::1"}},
		Auth: config.AuthConfig{
			RPID:          "easydnd.test",
			RPDisplayName: "easydnd",
			RPOrigins:     []string{testOrigin},
			SessionSecret: []byte("0123456789abcdef0123456789abcdef"),
			SessionTTL:    time.Hour,
			// Distinct from SessionTTL so the guest cookie's Max-Age proves
			// which lifetime the handler applied.
			GuestSessionTTL: 15 * time.Minute,
			CeremonyTTL:     5 * time.Minute,
			SecureCookies:   false,
		},
	}
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))

	svc := authuc.NewService(
		repo,
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

	r, err := httpapi.NewRouter(cfg, log, httpapi.Handlers{
		System:        system.New(testVersion),
		Auth:          authapi.New(svc, cookies),
		Authenticator: svc,
	})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	return r, svc, cookies, federation
}

// post issues a same-origin POST carrying the headers the real client sends.
func post(t *testing.T, r *gin.Engine, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", testOrigin)
	req.Header.Set("X-Request-Id", "test")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func cookieNamed(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range (&http.Response{Header: rec.Header()}).Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// register drives a full sign-up and returns the session cookie.
func register(t *testing.T, r *gin.Engine, cookies helpers.CookieOptions) *http.Cookie {
	t.Helper()

	// No body, exactly like login/begin. That is the endpoint's contract now,
	// not an omission this helper is getting away with.
	begin := post(t, r, "/v1/auth/register/begin", "")
	if begin.Code != http.StatusOK {
		t.Fatalf("register/begin status = %d: %s", begin.Code, begin.Body)
	}
	ceremony := cookieNamed(begin, cookies.CeremonyCookieName())
	if ceremony == nil {
		t.Fatal("register/begin set no ceremony cookie")
	}

	finish := post(t, r, "/v1/auth/register/finish", `{"id":"stub"}`, ceremony)
	if finish.Code != http.StatusOK {
		t.Fatalf("register/finish status = %d: %s", finish.Code, finish.Body)
	}
	session := cookieNamed(finish, cookies.SessionCookieName())
	if session == nil {
		t.Fatal("register/finish set no session cookie")
	}
	return session
}

// me issues the bootstrap call the SPA makes on every load, carrying session.
func me(t *testing.T, r *gin.Engine, session *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	req.AddCookie(session)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// guest drives an anonymous sign-in and returns the session cookie, the way
// register does for an account.
func guest(t *testing.T, r *gin.Engine, cookies helpers.CookieOptions) *http.Cookie {
	t.Helper()

	rec := post(t, r, "/v1/auth/anonymous", `{}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("auth/anonymous status = %d: %s", rec.Code, rec.Body)
	}
	session := cookieNamed(rec, cookies.SessionCookieName())
	if session == nil {
		t.Fatal("auth/anonymous set no session cookie")
	}
	return session
}

func TestMeIsUnauthenticatedWithoutACookie(t *testing.T) {
	r := newTestRouter(t)
	rec := do(t, r, http.MethodGet, "/v1/auth/me", nil)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	var body struct {
		Error struct {
			Code      string `json:"code"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	if body.Error.Code != "unauthenticated" {
		t.Errorf("error.code = %q, want %q", body.Error.Code, "unauthenticated")
	}
	if body.Error.RequestID == "" {
		t.Error("error.request_id is empty")
	}
}

func TestRegisterThenMe(t *testing.T) {
	r, _, cookies := newTestRouterWith(t, &stubCeremony{})
	session := register(t, r, cookies)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	req.AddCookie(session)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body)
	}
	var body struct {
		User struct {
			DisplayName string `json:"display_name"`
			Credentials []struct {
				ID string `json:"id"`
			} `json:"credentials"`
		} `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	// Nobody sent a name, so the server owed us one -- this is the handler-level
	// proof that the fixed label reaches the client rather than an empty string.
	if body.User.DisplayName != authuc.PasskeyDisplayName {
		t.Errorf("display_name = %q, want %q", body.User.DisplayName, authuc.PasskeyDisplayName)
	}
	if len(body.User.Credentials) != 1 {
		t.Errorf("credentials = %d, want 1", len(body.User.Credentials))
	}
}

// The session cookie's attributes are the whole of its security. A regression
// here is silent -- everything still works, it is just no longer protected.
func TestSessionCookieAttributes(t *testing.T) {
	r, _, cookies := newTestRouterWith(t, &stubCeremony{})
	session := register(t, r, cookies)

	if !session.HttpOnly {
		t.Error("session cookie is not HttpOnly; script could read it")
	}
	if session.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", session.SameSite)
	}
	if session.Path != "/" {
		t.Errorf("Path = %q, want /", session.Path)
	}
	if session.Domain != "" {
		t.Errorf("Domain = %q, want empty so the cookie stays host-only", session.Domain)
	}
	if session.MaxAge <= 0 {
		t.Errorf("MaxAge = %d, want the session TTL", session.MaxAge)
	}
}

// In production the prefixes are browser-enforced, so the names must switch
// with the environment rather than being hard-coded.
func TestSecureCookieNamesUsePrefixes(t *testing.T) {
	secure := helpers.CookieOptions{Secure: true}
	if got := secure.SessionCookieName(); got != "__Host-easydnd_session" {
		t.Errorf("session name = %q", got)
	}
	if got := secure.CeremonyCookieName(); got != "__Secure-easydnd_ceremony" {
		t.Errorf("ceremony name = %q", got)
	}

	insecure := helpers.CookieOptions{Secure: false}
	if strings.HasPrefix(insecure.SessionCookieName(), "__") {
		t.Error("development uses a prefixed name, which implies Secure and would never be set over plain HTTP")
	}
}

func TestCeremonyCookieIsScopedAndStrict(t *testing.T) {
	r, _, cookies := newTestRouterWith(t, &stubCeremony{})

	begin := post(t, r, "/v1/auth/login/begin", "")
	ceremony := cookieNamed(begin, cookies.CeremonyCookieName())
	if ceremony == nil {
		t.Fatal("login/begin set no ceremony cookie")
	}
	if ceremony.Path != "/v1/auth" {
		t.Errorf("Path = %q, want /v1/auth", ceremony.Path)
	}
	if ceremony.SameSite != http.SameSiteStrictMode {
		t.Errorf("SameSite = %v, want Strict", ceremony.SameSite)
	}
	if !ceremony.HttpOnly {
		t.Error("ceremony cookie is not HttpOnly")
	}
}

// register/begin takes no body at all, exactly like login/begin.
//
// Worth its own test because the failure is easy to reintroduce and easy to
// miss: binding a request struct here would 400 on the empty body the client
// now sends, and it would do so only on sign-up -- the half of the flow that
// only runs for people who do not have an account yet, and who therefore
// cannot tell us it is broken.
func TestRegisterBeginNeedsNoBody(t *testing.T) {
	r, _, cookies := newTestRouterWith(t, &stubCeremony{})

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/register/begin", nil)
	req.Header.Set("Origin", testOrigin)
	req.Header.Set("X-Request-Id", "test")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body)
	}
	if cookieNamed(rec, cookies.CeremonyCookieName()) == nil {
		t.Error("register/begin set no ceremony cookie")
	}
}

// A resolved ceremony must be spent, or the same sealed challenge could be
// presented twice.
func TestFinishClearsTheCeremonyCookie(t *testing.T) {
	r, _, cookies := newTestRouterWith(t, &stubCeremony{})

	begin := post(t, r, "/v1/auth/register/begin", "")
	ceremony := cookieNamed(begin, cookies.CeremonyCookieName())
	finish := post(t, r, "/v1/auth/register/finish", `{"id":"stub"}`, ceremony)

	cleared := cookieNamed(finish, cookies.CeremonyCookieName())
	if cleared == nil {
		t.Fatal("register/finish did not clear the ceremony cookie")
	}
	if cleared.MaxAge >= 0 || cleared.Value != "" {
		t.Errorf("ceremony cookie was not expired: value=%q maxage=%d", cleared.Value, cleared.MaxAge)
	}
}

func TestLoginRoundTrip(t *testing.T) {
	r, _, cookies := newTestRouterWith(t, &stubCeremony{})
	register(t, r, cookies)

	begin := post(t, r, "/v1/auth/login/begin", "")
	if begin.Code != http.StatusOK {
		t.Fatalf("login/begin status = %d: %s", begin.Code, begin.Body)
	}
	ceremony := cookieNamed(begin, cookies.CeremonyCookieName())

	finish := post(t, r, "/v1/auth/login/finish", `{"id":"stub"}`, ceremony)
	if finish.Code != http.StatusOK {
		t.Fatalf("login/finish status = %d: %s", finish.Code, finish.Body)
	}
	if cookieNamed(finish, cookies.SessionCookieName()) == nil {
		t.Fatal("login/finish set no session cookie")
	}
}

func TestFinishWithoutACeremonyCookieFails(t *testing.T) {
	r, _, _ := newTestRouterWith(t, &stubCeremony{})

	for _, path := range []string{"/v1/auth/register/finish", "/v1/auth/login/finish"} {
		rec := post(t, r, path, `{"id":"stub"}`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s status = %d, want %d", path, rec.Code, http.StatusBadRequest)
		}
	}
}

// Logout must answer with a body: web/src/lib/api/client.ts treats an empty
// successful response as a transport fault.
func TestLogoutClearsTheCookieAndReturnsABody(t *testing.T) {
	r, _, cookies := newTestRouterWith(t, &stubCeremony{})
	session := register(t, r, cookies)

	rec := post(t, r, "/v1/auth/logout", "", session)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.Len() == 0 {
		t.Fatal("logout returned an empty body")
	}

	cleared := cookieNamed(rec, cookies.SessionCookieName())
	if cleared == nil || cleared.MaxAge >= 0 || cleared.Value != "" {
		t.Fatal("logout did not expire the session cookie")
	}
}

// Signing out has to work when the session is already unusable -- that is
// exactly when the browser most needs the cookie cleared.
func TestLogoutWorksWithoutASession(t *testing.T) {
	r, _, _ := newTestRouterWith(t, &stubCeremony{})
	if rec := post(t, r, "/v1/auth/logout", ""); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestCrossSiteOriginIsRejected(t *testing.T) {
	r, _, _ := newTestRouterWith(t, &stubCeremony{})

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login/begin", nil)
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("X-Request-Id", "test")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// An HTML form -- the only cross-site POST that carries cookies without a
// preflight -- cannot set a custom header, so requiring one is a CSRF defence
// that costs the real client nothing: it already sends this header.
func TestPostWithoutTheRequestIDHeaderIsRejected(t *testing.T) {
	r, _, _ := newTestRouterWith(t, &stubCeremony{})

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login/begin", nil)
	req.Header.Set("Origin", testOrigin)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestCrossSiteFetchMetadataIsRejected(t *testing.T) {
	r, _, _ := newTestRouterWith(t, &stubCeremony{})

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login/begin", nil)
	req.Header.Set("X-Request-Id", "test")
	req.Header.Set(middleware.HeaderSecFetchSite, "cross-site")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// Safe methods must not be caught by the guard: a GET changes nothing, and a
// blocked /v1/version would break the deploy gate.
func TestGuardDoesNotBlockSafeMethods(t *testing.T) {
	r := newTestRouter(t)
	if rec := do(t, r, http.MethodGet, "/v1/version", nil); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAuthResponsesAreNotCacheable(t *testing.T) {
	r := newTestRouter(t)
	rec := do(t, r, http.MethodGet, "/v1/auth/me", nil)

	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
}

// A cookie that no longer works must be cleared, or the browser keeps sending
// a dead token on every request. This is the ordinary path after a restart,
// since the account store lives in the process.
func TestStaleSessionCookieIsCleared(t *testing.T) {
	r, _, cookies := newTestRouterWith(t, &stubCeremony{})
	session := register(t, r, cookies)

	// A second router: same signing key, brand-new (empty) account store.
	restarted, _, _ := newTestRouterWith(t, &stubCeremony{})

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	req.AddCookie(session)
	rec := httptest.NewRecorder()
	restarted.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	cleared := cookieNamed(rec, cookies.SessionCookieName())
	if cleared == nil || cleared.MaxAge >= 0 {
		t.Fatal("the dead session cookie was not cleared")
	}
}

// An unbounded read on an unauthenticated endpoint is an invitation; gin
// imposes no limit of its own.
func TestOversizedCeremonyBodyIsRejected(t *testing.T) {
	r, _, cookies := newTestRouterWith(t, &stubCeremony{})

	begin := post(t, r, "/v1/auth/login/begin", "")
	ceremony := cookieNamed(begin, cookies.CeremonyCookieName())

	huge := `{"id":"` + strings.Repeat("A", 128<<10) + `"}`
	rec := post(t, r, "/v1/auth/login/finish", huge, ceremony)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// The anonymous endpoint is the one way in that establishes a session without
// a ceremony, so what it sets is worth pinning as tightly as registration's.
func TestAnonymousEstablishesASession(t *testing.T) {
	r, _, cookies := newTestRouterWith(t, &stubCeremony{})

	rec := post(t, r, "/v1/auth/anonymous", `{}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}

	var body struct {
		User struct {
			ID          string          `json:"id"`
			DisplayName string          `json:"display_name"`
			Anonymous   bool            `json:"anonymous"`
			Credentials json.RawMessage `json:"credentials"`
		} `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	if !body.User.Anonymous {
		t.Error("response does not mark the user anonymous; the SPA cannot tell")
	}
	if body.User.ID == "" {
		t.Error("no user id issued")
	}
	// Not null: the client types this as an array and iterates it.
	if string(body.User.Credentials) != "[]" {
		t.Errorf("credentials = %s, want []", body.User.Credentials)
	}

	session := cookieNamed(rec, cookies.SessionCookieName())
	if session == nil {
		t.Fatal("no session cookie set")
	}
	if !session.HttpOnly {
		t.Error("session cookie is not HttpOnly; script could read it")
	}
	if session.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", session.SameSite)
	}
	if session.Path != "/" {
		t.Errorf("Path = %q, want /", session.Path)
	}
	// The guest TTL, not the account one -- the harness sets them apart so
	// this assertion can tell which was applied.
	if want := int((15 * time.Minute).Seconds()); session.MaxAge != want {
		t.Errorf("MaxAge = %d, want the guest TTL of %d", session.MaxAge, want)
	}
}

// A guest session has to work everywhere an account's does, which for the auth
// tree means /me -- the call the SPA makes on every load.
func TestAnonymousThenMe(t *testing.T) {
	r, _, cookies := newTestRouterWith(t, &stubCeremony{})
	session := guest(t, r, cookies)

	rec := me(t, r, session)
	if rec.Code != http.StatusOK {
		t.Fatalf("me status = %d: %s", rec.Code, rec.Body)
	}
	var body struct {
		User struct {
			DisplayName string `json:"display_name"`
			Anonymous   bool   `json:"anonymous"`
		} `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	if !body.User.Anonymous {
		t.Error("me does not report the session as anonymous")
	}
	if body.User.DisplayName == "" {
		t.Error("no display name; the signed-in header would render blank")
	}
}

// Registration still works after a guest session exists, and the account it
// creates is not anonymous -- the flag must not leak across the two paths.
func TestRegisteringIsNotAnonymous(t *testing.T) {
	r, _, cookies := newTestRouterWith(t, &stubCeremony{})
	session := register(t, r, cookies)

	rec := me(t, r, session)
	if rec.Code != http.StatusOK {
		t.Fatalf("me status = %d: %s", rec.Code, rec.Body)
	}
	var body struct {
		User struct {
			Anonymous bool `json:"anonymous"`
		} `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	if body.User.Anonymous {
		t.Error("a registered account is reported as anonymous")
	}
}
