package httpapi_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	httpapi "github.com/promix1722/easydnd/internal/api/http"
	"github.com/promix1722/easydnd/internal/api/http/v1/system"
	"github.com/promix1722/easydnd/internal/config"
)

// routerServing builds the smallest router that answers, over an optional
// bundle directory.
//
// Deliberately not newTestRouterOver: nothing here signs in, and the point of
// these tests is what happens to a request no route claims. Handing them the
// whole auth graph would obscure that.
func routerServing(t *testing.T, webDir string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	r, err := httpapi.NewRouter(
		&config.Config{
			Env:  config.EnvDevelopment,
			HTTP: config.HTTPConfig{TrustedProxies: []string{"127.0.0.1", "::1"}},
			Auth: config.AuthConfig{RPOrigins: []string{testOrigin}},
		},
		slog.New(slog.NewJSONHandler(io.Discard, nil)),
		httpapi.Handlers{System: system.New(testVersion), Version: testVersion, WebDir: webDir},
	)
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	return r
}

// bundle is a directory shaped like web/dist: an index and one real asset.
func bundle(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "index.html"), "<!doctype html><title>easydnd</title>")
	if err := os.Mkdir(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	write(t, filepath.Join(dir, "assets", "index-abc.js"), "console.log(1)")
	return dir
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func notFoundEnvelope(t *testing.T, body []byte) {
	t.Helper()
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode %q: %v", body, err)
	}
	if envelope.Error.Code == "" {
		t.Errorf("body %q is not the error envelope", body)
	}
}

// TestWithoutABundleNothingChanges is the guarantee production depends on.
// -web is unset on the server, so every one of these paths has to answer
// exactly as it did before the flag existed.
func TestWithoutABundleNothingChanges(t *testing.T) {
	r := routerServing(t, "")

	for _, path := range []string{"/", "/characters", "/v1/nope", "/index.html"} {
		rec := do(t, r, http.MethodGet, path, nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
		notFoundEnvelope(t, rec.Body.Bytes())
	}
}

// TestAPIPathsNeverGetTheBundle is the one worth having.
//
// A fallback that answered /v1 with index.html would hand the client 200 and
// an HTML body where it expected the error envelope -- so every genuine API
// 404 would surface as a parse failure somewhere far from its cause, and only
// in the one configuration a developer is using to test something else.
func TestAPIPathsNeverGetTheBundle(t *testing.T) {
	r := routerServing(t, bundle(t))

	// All unrouted: a mounted path like /v1/characters/:id answers 401 and
	// never reaches NoRoute at all, which would prove nothing here.
	for _, path := range []string{"/v1", "/v1/nope", "/v1/nope/deeper"} {
		rec := do(t, r, http.MethodGet, path, nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
		notFoundEnvelope(t, rec.Body.Bytes())
	}
}

func TestABundleIsServedWithASPAFallback(t *testing.T) {
	dir := bundle(t)
	r := routerServing(t, dir)

	t.Run("a real file is served", func(t *testing.T) {
		rec := do(t, r, http.MethodGet, "/assets/index-abc.js", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if rec.Body.String() != "console.log(1)" {
			t.Errorf("body = %q", rec.Body.String())
		}
	})

	t.Run("a deep link gets the app", func(t *testing.T) {
		// A client-side route, so the app has to load and decide -- which is
		// what nginx's try_files does in production.
		rec := do(t, r, http.MethodGet, "/characters/abc/build", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if rec.Body.String() != "<!doctype html><title>easydnd</title>" {
			t.Errorf("body = %q, want index.html", rec.Body.String())
		}
	})

	t.Run("the manifest is served as a manifest", func(t *testing.T) {
		// Go's mime table has no .webmanifest entry, so without the init() in
		// static.go this is sniffed as text/plain -- and Chrome then discards
		// the manifest and names an installed app after its URL. Invisible
		// until somebody installs it, which is why it is pinned here.
		write(t, filepath.Join(dir, "manifest.webmanifest"), `{"name":"easydnd"}`)

		rec := do(t, r, http.MethodGet, "/manifest.webmanifest", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/manifest+json") {
			t.Errorf("Content-Type = %q, want application/manifest+json", got)
		}
	})

	t.Run("a real route still wins", func(t *testing.T) {
		// /v1/version is mounted, so it never reaches NoRoute at all. Cheap to
		// assert and it pins that mounting a bundle did not shadow the API.
		rec := do(t, r, http.MethodGet, "/v1/version", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})
}
