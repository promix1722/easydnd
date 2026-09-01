package httpapi

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Go's mime table has no entry for .webmanifest, so http.ServeContent falls
// back to sniffing and calls it text/plain. Chrome then discards the manifest
// and names an installed app after its URL instead of what the manifest says.
//
// deploy/nginx/easydnd.conf states the same type for the same reason -- stock
// nginx mime.types does not know the extension either. Two servers, one gap,
// and it is only ever visible after somebody installs the app.
func init() {
	if err := mime.AddExtensionType(".webmanifest", "application/manifest+json"); err != nil {
		// Only returned for an extension not starting with a dot, which this
		// is. Unreachable, and ignoring it silently would be the wrong habit.
		panic(err)
	}
}

// staticSite serves a built frontend bundle from disk, with the SPA fallback
// nginx does in production (`try_files $uri $uri/ /index.html`).
//
// DEVELOPMENT ONLY. Production serves the bundle from nginx and this is
// unreachable there: it exists only when `-web` names a directory, and nothing
// on the server passes that flag. It is a flag rather than a config key for
// exactly that reason -- a YAML file on the server cannot switch it on.
//
// What it buys is a single origin for a preview build: the service worker,
// the install prompt and passkeys all need a secure context, and one process
// answering both /v1 and the bundle behind one TLS port is the shortest way to
// have one without a second server and a proxy between them. See `make
// preview` and docs/web.md.
//
// It does not try to reproduce production's caching policy, and should not: the
// whole of that lives in deploy/nginx/easydnd.conf, and a second half-copy here
// would be a lie about what a release actually serves. It does carry the one
// rule that policy has which this preview exists to exercise -- see
// cacheControl below.
func staticSite(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	index := filepath.Join(dir, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", cacheControl(r.URL.Path))
		// A deep link is a client-side route, so anything without a file behind
		// it is the app rather than a 404. Checked on disk rather than by
		// letting FileServer answer and rewriting its 404, because that would
		// mean buffering every response to find out.
		if path := filepath.Join(dir, filepath.Clean("/"+r.URL.Path)); !exists(path) {
			// Except under /assets/, which is never a route. Those names carry
			// a content hash, so a request for one that is gone comes from a
			// page built against an older bundle -- a tab left open across a
			// rebuild, or a service worker holding the previous index. Handing
			// it index.html answers a `<script type="module">` with
			// `text/html`, and the browser reports a MIME type error that says
			// nothing about the stale page that caused it. nginx already
			// answers these `=404` (deploy/nginx/easydnd.conf); this is that
			// rule, so a preview fails the way production does.
			if strings.HasPrefix(r.URL.Path, "/assets/") {
				http.NotFound(w, r)
				return
			}
			// Only the public landing page is search content. Every other
			// fallback is either an authenticated application route or the
			// client-side not-found page, and all of them contain the landing
			// metadata until React starts. Keep that shared document from turning
			// private and invented URLs into duplicate search results.
			if r.URL.Path != "/" {
				w.Header().Set("X-Robots-Tag", "noindex, nofollow")
			}
			http.ServeFile(w, r, index)
			return
		}
		// Both http.FileServer and http.ServeFile answer a path ending
		// /index.html with a 301 to ./, on the general principle that a
		// directory should have one URL. The service worker precaches exactly
		// that path, so every install spent a redirect on it and stored a
		// response marked `redirected`, which a browser may refuse to hand to a
		// navigation. nginx serves the file, so this serves the file: opened and
		// handed to ServeContent, which is the same machinery without the
		// redirect in front of it.
		if r.URL.Path == "/index.html" {
			// The worker precaches this spelling, so it cannot redirect to `/`.
			// Canonical metadata points at `/`; noindex keeps a crawler that
			// guesses the implementation URL from indexing a second copy.
			w.Header().Set("X-Robots-Tag", "noindex, nofollow")
			serveIndex(w, r, index)
			return
		}
		files.ServeHTTP(w, r)
	})
}

// serveIndex writes index.html itself, rather than through the redirect every
// helper in net/http puts in front of that name. Errors fall back to a 404: an
// index that cannot be opened is a broken bundle, and the fallback would only
// try to open the same file again.
func serveIndex(w http.ResponseWriter, r *http.Request, index string) {
	file, err := os.Open(index)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

// cacheControl is the one half of production's caching policy that a preview
// cannot do without.
//
// Everything here was served with a Last-Modified and no Cache-Control, which
// is an invitation to *heuristic* freshness: a browser is allowed to reuse such
// a response for a fraction of its age without asking. For a bundle that is a
// few minutes old that window is short, and it is long enough to break the one
// flow this server exists to test. `registration.update()` fetched a cached
// sw.js, found the same bytes, installed nothing; the update dialog's reload
// then had no new worker to wait for, so it fell through to a plain reload,
// which the old worker answered from its own precache -- the same page, and
// the dialog again. Twice, or until the cache revalidated.
//
// nginx says `no-cache` on sw.js, index.html, version.json and registerSW.js
// for exactly this reason, and `immutable` on the hashed names, which cannot
// change meaning. Two lines here, and the same two answers.
func cacheControl(path string) string {
	if strings.HasPrefix(path, "/assets/") || workboxChunk.MatchString(path) {
		return "public, max-age=31536000, immutable"
	}
	// no-cache is "revalidate before reuse", not "do not store": the response
	// is still cached, and a 304 still saves the body.
	return "no-cache"
}

// The worker's runtime chunk is content-hashed like everything under /assets/,
// but vite-plugin-pwa emits it at the bundle root. Same expression as the
// location block in deploy/nginx/easydnd.conf.
var workboxChunk = regexp.MustCompile(`^/workbox-[^/]+\.js(\.map)?$`)

func exists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// isAPIPath reports whether a request belongs to the API rather than the
// bundle.
//
// The SPA fallback must never answer one. A /v1 path that fell through to
// index.html would hand the client 200 and an HTML body where it expected the
// error envelope, turning every genuine API 404 into a parse failure somewhere
// far from the cause.
func isAPIPath(path string) bool {
	return path == "/v1" || strings.HasPrefix(path, "/v1/")
}
