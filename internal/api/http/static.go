package httpapi

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
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
// would be a lie about what a release actually serves.
func staticSite(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	index := filepath.Join(dir, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A deep link is a client-side route, so anything without a file behind
		// it is the app rather than a 404. Checked on disk rather than by
		// letting FileServer answer and rewriting its 404, because that would
		// mean buffering every response to find out.
		if path := filepath.Join(dir, filepath.Clean("/"+r.URL.Path)); !exists(path) {
			http.ServeFile(w, r, index)
			return
		}
		files.ServeHTTP(w, r)
	})
}

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
