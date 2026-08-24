package httpapi_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/promix1722/easydnd/internal/adapter/repository/postgres"
	"github.com/promix1722/easydnd/internal/config"
)

// The user-visible half of the durable account store.
//
// The repository tests prove a row survives; this proves what a person actually
// experiences. Somebody registers a passkey, the server is replaced -- a deploy,
// a `supervisorctl restart`, a crash -- and their browser sends back the session
// cookie it already had. That must still resolve to their account.
//
// Before Postgres it did not: the cookie outlived the store, GET /v1/auth/me
// answered 401, and the visible symptom was a silent drop to the landing page
// with a passkey in their password manager that the server no longer knew.
func TestSessionSurvivesARestart(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is unset; skipping the durability test")
	}
	cfg := config.DBConfig{URL: url, MaxConns: 5, ConnectTimeout: 10 * time.Second}
	ctx := context.Background()

	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := postgres.Migrate(ctx, cfg, quiet, postgres.CommandUp); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// The process that serves the sign-up.
	before, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if _, err := before.Exec(ctx, `TRUNCATE users CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	ceremony := &stubCeremony{credentialID: "durable-credential"}
	r, _, cookies, _ := newTestRouterOver(t, ceremony, postgres.NewUserRepository(before), &stubFederation{})
	session := register(t, r, cookies)

	// The restart. Nothing survives but the database and the cookie already in
	// the browser -- and the signing key, which lives in the supervisor config
	// precisely so that it does.
	before.Close()

	after, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	defer after.Close()

	restarted, _, _, _ := newTestRouterOver(t, ceremony, postgres.NewUserRepository(after), &stubFederation{})

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	req.AddCookie(session)
	rec := httptest.NewRecorder()
	restarted.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/auth/me after restart = %d, want 200: %s", rec.Code, rec.Body)
	}

	// And the passkey itself still signs in -- the credential, not just the
	// account row, came back.
	begin := post(t, restarted, "/v1/auth/login/begin", `{}`)
	if begin.Code != http.StatusOK {
		t.Fatalf("login/begin after restart = %d: %s", begin.Code, begin.Body)
	}
	ceremonyCookie := cookieNamed(begin, cookies.CeremonyCookieName())
	if ceremonyCookie == nil {
		t.Fatal("login/begin set no ceremony cookie")
	}
	finish := post(t, restarted, "/v1/auth/login/finish", `{"id":"stub"}`, ceremonyCookie)
	if finish.Code != http.StatusOK {
		t.Fatalf("login/finish after restart = %d: %s -- the passkey no longer resolves", finish.Code, finish.Body)
	}
}
