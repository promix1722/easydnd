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

// TestAMissingAssetIs404NotTheIndex pins the divergence that produced a
// misleading error rather than a missing file.
//
// Names under /assets/ carry a content hash, so a request for one that is gone
// is a page built against an older bundle -- a tab left open across a rebuild.
// Answering it with index.html hands a `<script type="module">` an HTML body,
// and the browser reports a MIME type error naming neither the stale page nor
// the missing chunk. nginx answers `=404` here; so does this.
func TestAMissingAssetIs404NotTheIndex(t *testing.T) {
	r := routerServing(t, bundle(t))

	rec := do(t, r, http.MethodGet, "/assets/index-gone.js", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET a vanished asset = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if body := rec.Body.String(); strings.Contains(body, "<!doctype html") {
		t.Errorf("body = %q, want anything but the index", body)
	}

	// The asset that is there is still served, and still as itself.
	rec = do(t, r, http.MethodGet, "/assets/index-abc.js", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("GET a real asset = %d, want %d", rec.Code, http.StatusOK)
	}
}

// TestTheBundleSaysHowLongItKeeps covers the half of production's caching policy
// this server has to carry: without it a browser is free to reuse sw.js on
// heuristic freshness, and the update dialog's reload finds no new worker to
// wait for and comes back to the same page.
func TestTheBundleSaysHowLongItKeeps(t *testing.T) {
	r := routerServing(t, bundle(t))

	for path, want := range map[string]string{
		"/":                    "no-cache",
		"/index.html":          "no-cache",
		"/sw.js":               "no-cache",
		"/characters":          "no-cache", // a deep link, answered by the index
		"/assets/index-abc.js": "public, max-age=31536000, immutable",
		"/workbox-abc123.js":   "public, max-age=31536000, immutable",
	} {
		rec := do(t, r, http.MethodGet, path, nil)
		if got := rec.Header().Get("Cache-Control"); got != want {
			t.Errorf("GET %s Cache-Control = %q, want %q", path, got, want)
		}
	}
}

// TestOnlyTheHomepageIsIndexable pins the SEO boundary. Every missing file is
// an SPA navigation, but only `/` is public content; the rest are authenticated
// routes or the client-side not-found page and begin life with the homepage's
// metadata before React replaces it.
func TestOnlyTheHomepageIsIndexable(t *testing.T) {
	r := routerServing(t, bundle(t))

	for path, want := range map[string]string{
		"/":                 "",
		"/index.html":       "noindex, nofollow",
		"/characters/abc":   "noindex, nofollow",
		"/not-a-real-route": "noindex, nofollow",
	} {
		rec := do(t, r, http.MethodGet, path, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d, want %d", path, rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get("X-Robots-Tag"); got != want {
			t.Errorf("GET %s X-Robots-Tag = %q, want %q", path, got, want)
		}
	}
}

func TestDiscoveryFilesAreServedAsFiles(t *testing.T) {
	dir := bundle(t)
	write(t, filepath.Join(dir, "robots.txt"), "User-agent: *\n")
	write(t, filepath.Join(dir, "sitemap.xml"), "<?xml version=\"1.0\"?><urlset></urlset>")
	write(t, filepath.Join(dir, "llms.txt"), "# easydnd.org\n")
	r := routerServing(t, dir)

	for path, want := range map[string]string{
		"/robots.txt":  "User-agent: *",
		"/sitemap.xml": "<urlset>",
		"/llms.txt":    "# easydnd.org",
	} {
		rec := do(t, r, http.MethodGet, path, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d, want %d", path, rec.Code, http.StatusOK)
		}
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("GET %s body = %q, want it to contain %q", path, rec.Body.String(), want)
		}
		if strings.Contains(rec.Body.String(), "<!doctype html") {
			t.Errorf("GET %s returned the SPA fallback", path)
		}
	}
}

// TestIndexHtmlIsServedRatherThanRedirected pins the other thing the service
// worker's install depends on: it precaches /index.html by that name, and
// net/http's helpers answer it with a 301 to ./, so the install spent a
// redirect and stored a response marked `redirected`. nginx serves the file.
func TestIndexHtmlIsServedRatherThanRedirected(t *testing.T) {
	r := routerServing(t, bundle(t))

	rec := do(t, r, http.MethodGet, "/index.html", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /index.html = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "<!doctype html") {
		t.Errorf("body = %q, want the index", rec.Body.String())
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
