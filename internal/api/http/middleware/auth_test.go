package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	"github.com/promix1722/easydnd/internal/config"
	"github.com/promix1722/easydnd/internal/domain/user"
)

type stubAuth map[string]bool

func (s stubAuth) Session(_ context.Context, token string) (user.User, error) {
	if s[token] {
		return user.User{ID: user.ID(token)}, nil
	}
	return user.User{}, errors.New("dead")
}

// A development request carries the session cookies of every selector ever
// used on the hostname. The ones this server no longer accepts are cleared; a
// live sibling (another tab's identity) and another server's namespace stay.
func TestRequireSessionClearsDeadSiblingCookies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{Env: config.EnvDevelopment, HTTP: config.HTTPConfig{Port: "18080"}}
	cookies := helpers.NewCookieOptions(cfg)
	own := cookies.SessionCookieName()
	foreign := strings.Replace(own, "_dev_", "_dev_ffff", 1)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set(helpers.HeaderDevelopmentSession, strings.Repeat("a", 32))
	for name, value := range map[string]string{
		own + "_" + strings.Repeat("a", 32):     "current",
		own + "_" + strings.Repeat("b", 32):     "live-sibling",
		own + "_" + strings.Repeat("c", 32):     "dead-sibling",
		own:                                     "dead-bare",
		foreign + "_" + strings.Repeat("d", 32): "foreign",
	} {
		c.Request.AddCookie(&http.Cookie{Name: name, Value: value})
	}

	RequireSession(stubAuth{"current": true, "live-sibling": true}, cookies)(c)

	if c.IsAborted() {
		t.Fatalf("current session rejected: %d %s", rec.Code, rec.Body)
	}
	cleared := map[string]bool{}
	for _, cookie := range rec.Result().Cookies() {
		if cookie.MaxAge < 0 {
			cleared[cookie.Value+cookie.Name] = true
		}
	}
	want := map[string]bool{own + "_" + strings.Repeat("c", 32): true, own: true}
	if len(cleared) != len(want) {
		t.Fatalf("cleared %v, want exactly %v", cleared, want)
	}
	for name := range want {
		if !cleared[name] {
			t.Fatalf("dead cookie %s not cleared; cleared %v", name, cleared)
		}
	}
}
