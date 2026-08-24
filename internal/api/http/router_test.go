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

	// deploy.sh does a raw substring grep rather than a JSON parse.
	if !strings.Contains(rec.Body.String(), testVersion) {
		t.Errorf("response body %q does not contain the raw version", rec.Body.String())
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
