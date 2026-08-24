package oidc

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// issuerServer stands up just enough of an OIDC issuer for discovery, which is
// all AuthCodeURL needs and all the concurrency below is about.
func issuerServer(t *testing.T, fail *atomic.Bool, hits *atomic.Int32) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		if fail != nil && fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// go-oidc checks that the document's issuer matches the one asked
		// for, so it has to echo this server's own address.
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                server.URL,
			"authorization_endpoint":                server.URL + "/auth",
			"token_endpoint":                        server.URL + "/token",
			"jwks_uri":                              server.URL + "/keys",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	return server
}

func newTestGoogle(t *testing.T, issuer string) *Google {
	t.Helper()
	g, err := NewGoogle(Config{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURL:  "https://easydnd.test/v1/auth/sso/google/callback",
		Issuer:       issuer,
	})
	if err != nil {
		t.Fatalf("NewGoogle: %v", err)
	}
	return g
}

func TestNewGoogleRequiresItsCredentials(t *testing.T) {
	full := Config{ClientID: "id", ClientSecret: "secret", RedirectURL: "https://x.test/cb"}

	for name, mutate := range map[string]func(*Config){
		"no client id":     func(c *Config) { c.ClientID = "" },
		"no client secret": func(c *Config) { c.ClientSecret = "" },
		"no redirect url":  func(c *Config) { c.RedirectURL = "" },
	} {
		cfg := full
		mutate(&cfg)
		if _, err := NewGoogle(cfg); err == nil {
			t.Errorf("%s: NewGoogle succeeded", name)
		}
	}
}

// Discovery must not happen in the constructor: doing it at startup would make
// the process refuse to boot whenever the issuer was unreachable, which
// deploy.sh's health gate would read as a bad release and roll back.
func TestNewGoogleDoesNoNetworkIO(t *testing.T) {
	var hits atomic.Int32
	server := issuerServer(t, nil, &hits)

	newTestGoogle(t, server.URL)

	if got := hits.Load(); got != 0 {
		t.Fatalf("the constructor made %d discovery requests, want 0", got)
	}
}

// The regression this pins: discovery used to run under a sync.Once that was
// re-zeroed on failure, which races on the Once's own mutex and can publish a
// nil provider out from under a caller already holding a good one. Run with
// -race, which is what `make test/unit` does.
func TestConcurrentSignInsDiscoverOnce(t *testing.T) {
	var hits atomic.Int32
	server := issuerServer(t, nil, &hits)
	g := newTestGoogle(t, server.URL)

	const callers = 32
	var wg sync.WaitGroup
	urls := make([]string, callers)

	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			urls[i] = g.AuthCodeURL("state", "nonce", "challenge")
		}()
	}
	wg.Wait()

	for i, got := range urls {
		if !strings.HasPrefix(got, server.URL+"/auth") {
			t.Fatalf("caller %d got %q, want the issuer's authorization endpoint", i, got)
		}
	}
	// One discovery for the lot: the lock is held across the call precisely so
	// that a burst of first sign-ins does not become a burst of discoveries.
	if got := hits.Load(); got != 1 {
		t.Errorf("performed %d discoveries for %d callers, want 1", got, callers)
	}
}

// A failure must not be cached. An issuer unreachable for one request must not
// be unreachable for the life of the process -- and retrying must not be a
// data race either.
func TestDiscoveryRetriesAfterAFailure(t *testing.T) {
	var failing atomic.Bool
	var hits atomic.Int32
	failing.Store(true)
	server := issuerServer(t, &failing, &hits)
	g := newTestGoogle(t, server.URL)

	// An empty URL is how the port reports that it could not reach its issuer.
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := g.AuthCodeURL("state", "nonce", "challenge"); got != "" {
				t.Errorf("AuthCodeURL = %q while the issuer was down, want \"\"", got)
			}
		}()
	}
	wg.Wait()

	if _, err := g.Exchange(context.Background(), "code", "nonce", "verifier"); err == nil {
		t.Error("Exchange succeeded while the issuer was down")
	}

	failing.Store(false)
	if got := g.AuthCodeURL("state", "nonce", "challenge"); !strings.HasPrefix(got, server.URL+"/auth") {
		t.Fatalf("AuthCodeURL = %q after recovery, want the authorization endpoint", got)
	}
}

// PKCE, the nonce and the scopes are what make the redirect safe to hand a
// browser, so they are pinned rather than left to the library's defaults.
func TestAuthCodeURLCarriesPKCEAndNonce(t *testing.T) {
	var hits atomic.Int32
	server := issuerServer(t, nil, &hits)
	g := newTestGoogle(t, server.URL)

	const verifier = "0123456789abcdefghijklmnopqrstuvwxyz-_ABCDE"
	got := g.AuthCodeURL("the-state", "the-nonce", verifier)

	// The bug this pins: the adapter derives the challenge from the verifier
	// exactly once. Handing oauth2 a pre-hashed challenge made it hash the
	// hash, so Google compared S256(verifier) against S256(S256(verifier)) and
	// every real sign-in died at the token exchange -- invisibly to any test
	// with a stubbed provider.
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	for _, want := range []string{
		"state=the-state",
		"nonce=the-nonce",
		"code_challenge=" + challenge,
		"code_challenge_method=S256",
		"client_id=test-client",
		"scope=openid+email+profile",
		// No refresh token: there is nothing to call on the person's behalf
		// once they are signed in, so holding one would be all cost.
		"access_type=online",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("redirect is missing %q: %s", want, got)
		}
	}
	if strings.Contains(got, "test-secret") {
		t.Error("the client secret reached the browser's URL")
	}
	// The verifier is the half that must stay on the server until the
	// exchange; sending it in the redirect would make PKCE decorative.
	if strings.Contains(got, verifier) {
		t.Errorf("the PKCE verifier was sent to the browser: %s", got)
	}
}
