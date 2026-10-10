package system

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAnalyticsConfigExposesOnlyPublicFields(t *testing.T) {
	r := gin.New()
	h := New("release", AnalyticsConfigResponse{Environment: "development", Token: "phc_test", Host: "https://eu.i.posthog.com"})
	r.GET("/analytics-config", h.AnalyticsConfig)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/analytics-config", nil))
	want := `{"environment":"development","token":"phc_test","host":"https://eu.i.posthog.com"}`
	if rec.Code != 200 || rec.Body.String() != want {
		t.Fatalf("unexpected response: %d %s", rec.Code, rec.Body.String())
	}
}
