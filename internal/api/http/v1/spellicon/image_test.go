package spellicon_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	api "github.com/promix1722/easydnd/internal/api/http/v1/spellicon"
	"github.com/promix1722/easydnd/internal/domain/imageasset"
	"github.com/promix1722/easydnd/internal/types"
)

// stubReader fakes the seed store's serve path: every call is recorded so the
// tests can prove which slug was asked for, and misses answer NotFound like
// the repository does.
type stubReader struct {
	images map[string]imageasset.Image
	err    error
	calls  []string
}

func (s *stubReader) Image(_ context.Context, slug string) (imageasset.Image, error) {
	s.calls = append(s.calls, slug)
	if s.err != nil {
		return imageasset.Image{}, s.err
	}
	img, ok := s.images[slug]
	if !ok {
		return imageasset.Image{}, types.NewNotFoundError("spell icon not found")
	}
	return img, nil
}

func imageRouter(reader api.ImageReader) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/v1/spell-icons/*slug", api.NewImages(reader).Get)
	return r
}

func request(t *testing.T, r *gin.Engine, path, etag string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestGetServesWebPWithRevisionCaching(t *testing.T) {
	reader := &stubReader{images: map[string]imageasset.Image{
		"acid-splash": {Slug: "acid-splash", Revision: "abc123", ContentType: "image/webp", Data: []byte("webp-bytes")},
	}}
	r := imageRouter(reader)

	rec := request(t, r, "/v1/spell-icons/acid-splash.webp", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "image/webp" {
		t.Errorf("Content-Type = %q, want image/webp", got)
	}
	if got := rec.Header().Get("ETag"); got != `"abc123"` {
		t.Errorf("ETag = %q, want %q", got, `"abc123"`)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache: immutable would pin a revision that regeneration changes", got)
	}
	if rec.Body.String() != "webp-bytes" {
		t.Errorf("body = %q, want the stored bytes", rec.Body.String())
	}
	if reader.calls[0] != "acid-splash" {
		t.Errorf("reader asked for %q, want the slug without .webp", reader.calls[0])
	}
}

func TestGetServesNamespacedSlug(t *testing.T) {
	reader := &stubReader{images: map[string]imageasset.Image{
		"dnd-2014/acid-splash": {Slug: "dnd-2014/acid-splash", Revision: "r2", ContentType: "image/webp", Data: []byte("ns")},
	}}
	r := imageRouter(reader)

	rec := request(t, r, "/v1/spell-icons/dnd-2014/acid-splash.webp", "")
	if rec.Code != http.StatusOK || rec.Body.String() != "ns" {
		t.Fatalf("status = %d body = %q, want the namespaced icon", rec.Code, rec.Body.String())
	}
	if reader.calls[0] != "dnd-2014/acid-splash" {
		t.Errorf("reader asked for %q, want the whole wildcard minus .webp", reader.calls[0])
	}
}

func TestGetRevalidatesByETag(t *testing.T) {
	reader := &stubReader{images: map[string]imageasset.Image{
		"acid-splash": {Slug: "acid-splash", Revision: "abc123", ContentType: "image/webp", Data: []byte("webp-bytes")},
	}}
	r := imageRouter(reader)

	rec := request(t, r, "/v1/spell-icons/acid-splash.webp", `"abc123"`)
	if rec.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304 for a matching revision", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("304 body = %q, want empty -- revalidation must not resend the artwork", rec.Body.String())
	}
	if rec.Header().Get("ETag") != `"abc123"` {
		t.Errorf("304 lost the ETag")
	}

	// The header can carry several validators; a matching one still applies.
	rec = request(t, r, "/v1/spell-icons/acid-splash.webp", `"other", W/"abc123"`)
	if rec.Code != http.StatusNotModified {
		t.Errorf("status = %d, want 304 for a weak match inside a list", rec.Code)
	}

	// A stale revision -- the client kept art that was regenerated since --
	// gets the new bytes, which is the whole point of not caching immutably.
	rec = request(t, r, "/v1/spell-icons/acid-splash.webp", `"stale"`)
	if rec.Code != http.StatusOK || rec.Body.String() != "webp-bytes" {
		t.Errorf("stale ETag got %d %q, want a fresh 200", rec.Code, rec.Body.String())
	}
}

func TestGetMissesArePlain404s(t *testing.T) {
	reader := &stubReader{images: map[string]imageasset.Image{}}
	r := imageRouter(reader)

	for _, path := range []string{
		"/v1/spell-icons/never-seeded.webp",
		"/v1/spell-icons/../secret.webp",
		"/v1/spell-icons/acid-splash",
		"/v1/spell-icons/acid-splash.png",
		"/v1/spell-icons/.webp",
		"/v1/spell-icons/",
	} {
		rec := request(t, r, path, "")
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, rec.Code)
		}
		var body struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		// Every miss is the API error envelope -- the SPA's index.html is
		// never an answer for /v1.
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Error.Code != "not_found" {
			t.Errorf("GET %s body = %q, want the not_found envelope", path, rec.Body.String())
		}
	}
	for _, slug := range reader.calls {
		if slug == "acid-splash.png" || slug == "" {
			t.Errorf("reader was asked for %q, want the .webp gate to stop it first", slug)
		}
	}
}

func TestGetRepositoryFailureIsAServerError(t *testing.T) {
	reader := &stubReader{err: errors.New("database is gone")}
	r := imageRouter(reader)

	rec := request(t, r, "/v1/spell-icons/acid-splash.webp", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500: a down repository is not a missing icon", rec.Code)
	}
}
