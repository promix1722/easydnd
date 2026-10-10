package helpers

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/config"
)

func TestProductionIgnoresDevelopmentCookieSelector(t *testing.T) {
	cfg := &config.Config{Env: config.EnvProduction, HTTP: config.HTTPConfig{Port: "8080"}, Auth: config.AuthConfig{SecureCookies: true}}
	options := NewCookieOptions(cfg)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set(HeaderDevelopmentSession, strings.Repeat("a", 32))
	options.SetSession(c, "signed-token", time.Hour)
	cookie := rec.Result().Cookies()[0]
	if cookie.Name != "__Host-easydnd_session" || !cookie.Secure || !cookie.HttpOnly {
		t.Fatalf("production cookie changed: %+v", cookie)
	}
	c.Request.AddCookie(cookie)
	if options.Session(c) != "signed-token" {
		t.Fatal("production accepted development selector")
	}
}

func TestDevelopmentSeparatesAllAuthCookieNames(t *testing.T) {
	cfg := &config.Config{Env: config.EnvDevelopment, HTTP: config.HTTPConfig{Port: "18082"}}
	first := NewCookieOptions(cfg)
	cfg.HTTP.Port = "18083"
	second := NewCookieOptions(cfg)
	if first.SessionCookieName() == second.SessionCookieName() ||
		first.CeremonyCookieName() == second.CeremonyCookieName() ||
		first.FlightCookieName() == second.FlightCookieName() {
		t.Fatal("development servers share auth cookies")
	}
}
