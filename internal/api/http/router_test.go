package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/middleware"
)

const testVersion = "0123456789abcdef0123456789abcdef01234567"

func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	r, _, _ := newTestRouterWith(t, &stubCeremony{})
	return r
}

func do(t *testing.T, r *gin.Engine, method, path string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// TestVersionServesInjectedValue pins the deploy contract: deploy/deploy.sh
// greps this response for the release SHA, so the endpoint must echo whatever
// build version it was given, under the key "version".
func TestVersionServesInjectedValue(t *testing.T) {
	rec := do(t, newTestRouter(t), http.MethodGet, "/v1/version", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	if body.Version != testVersion {
		t.Errorf("version = %q, want %q", body.Version, testVersion)
	}

	// deploy.sh matches this literal rather than parsing JSON, so the exact
	// bytes encoding/json produces are the contract -- the field name, the
	// quoting, and no space after the colon. The match is anchored on the
	// field because the release identifier is a tag now: a bare grep for
	// "v1.0.4" would also be satisfied by "v1.0.40".
	want := `"version":"` + testVersion + `"`
	if !strings.Contains(rec.Body.String(), want) {
		t.Errorf("response body %q does not contain %s", rec.Body.String(), want)
	}
}

// TestVersionIsNotCacheable pins the other half of the deploy contract. This
// endpoint answers "which release is live", and both of its readers -- the
// deploy gate and a browser deciding whether to reload -- are misled by an
// answer that was allowed to be held somewhere.
func TestVersionIsNotCacheable(t *testing.T) {
	rec := do(t, newTestRouter(t), http.MethodGet, "/v1/version", nil)

	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-store")
	}
}

// TestEveryResponseCarriesTheAppVersion pins the header a browser uses to
// notice it is running code from a release that is no longer deployed.
//
// The error cases are the point rather than padding: a client running against
// a newer API is exactly the client whose requests start failing, so a header
// present only on success would go missing at the moment it is needed. The 404
// also proves the middleware is global -- NoRoute is outside every group.
func TestEveryResponseCarriesTheAppVersion(t *testing.T) {
	r := newTestRouter(t)

	cases := []struct {
		name   string
		method string
		path   string
		status int
	}{
		{"ok", http.MethodGet, "/v1/health", http.StatusOK},
		{"unauthenticated", http.MethodGet, "/v1/auth/me", http.StatusUnauthorized},
		{"no route", http.MethodGet, "/v1/nothing-here", http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, r, tc.method, tc.path, nil)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
			if got := rec.Header().Get(middleware.HeaderAppVersion); got != testVersion {
				t.Errorf("%s = %q, want %q", middleware.HeaderAppVersion, got, testVersion)
			}
		})
	}
}

func TestHealth(t *testing.T) {
	rec := do(t, newTestRouter(t), http.MethodGet, "/v1/health", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("status = %q, want %q", body.Status, "ok")
	}
}

// TestUnknownRouteReturnsErrorEnvelope covers NoRoute going through
// helpers.FormatError rather than gin's plain-text 404.
func TestUnknownRouteReturnsErrorEnvelope(t *testing.T) {
	rec := do(t, newTestRouter(t), http.MethodGet, "/v1/nope", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
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
	if body.Error.Code != "not_found" {
		t.Errorf("error.code = %q, want %q", body.Error.Code, "not_found")
	}
	if body.Error.RequestID == "" {
		t.Errorf("error.request_id is empty, want the correlation id")
	}
}

// TestRootIsNotRouted documents that / was deliberately freed up.
func TestRootIsNotRouted(t *testing.T) {
	rec := do(t, newTestRouter(t), http.MethodGet, "/", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRequestIDIsMintedAndEchoed(t *testing.T) {
	r := newTestRouter(t)

	rec := do(t, r, http.MethodGet, "/v1/health", nil)
	if got := rec.Header().Get(middleware.HeaderRequestID); got == "" {
		t.Errorf("%s header is empty, want a minted id", middleware.HeaderRequestID)
	}

	const supplied = "trace-1"
	rec = do(t, r, http.MethodGet, "/v1/health", map[string]string{
		middleware.HeaderRequestID: supplied,
	})
	if got := rec.Header().Get(middleware.HeaderRequestID); got != supplied {
		t.Errorf("%s = %q, want the supplied %q", middleware.HeaderRequestID, got, supplied)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	rec := do(t, newTestRouter(t), http.MethodPost, "/v1/health", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
