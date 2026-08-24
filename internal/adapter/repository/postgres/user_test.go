package postgres_test

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/promix1722/easydnd/internal/adapter/repository/postgres"
	"github.com/promix1722/easydnd/internal/adapter/repository/repotest"
	"github.com/promix1722/easydnd/internal/config"
	domain "github.com/promix1722/easydnd/internal/domain/user"
)

var migrateOnce sync.Once

// TestUserRepository runs the shared port contract against a real database.
//
// It skips rather than fails when TEST_DATABASE_URL is unset, so `go test ./...`
// and `make verify` stay green on a machine with no Postgres. CI sets the
// variable against a service container, which is what stops the package that
// now owns account durability from shipping untested.
func TestUserRepository(t *testing.T) {
	cfg := testConfig(t)

	repotest.RunUserRepository(t, func(t *testing.T) domain.Repository {
		pool := testPool(t, cfg)
		// TRUNCATE rather than drop-and-remigrate: the latter turns a
		// sub-second suite into a twenty-second one for no extra isolation.
		// CASCADE because user_credentials references users.
		if _, err := pool.Exec(context.Background(), `TRUNCATE users CASCADE`); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		return postgres.NewUserRepository(pool)
	})
}

func testConfig(t *testing.T) config.DBConfig {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is unset; skipping the Postgres adapter tests")
	}
	return config.DBConfig{URL: url, MaxConns: 5, ConnectTimeout: 10 * time.Second}
}

func testPool(t *testing.T, cfg config.DBConfig) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	var migrateErr error
	migrateOnce.Do(func() {
		migrateErr = postgres.Migrate(ctx, cfg, testLogger(), postgres.CommandUp)
	})
	if migrateErr != nil {
		t.Fatalf("migrate: %v", migrateErr)
	}

	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// The whole reason this adapter exists.
//
// A restart discards the process, its pool and everything it held. The account
// and its passkey must not go with them -- and because sign-in is passkeys-only
// with no password, no email and no recovery, an account that does not survive
// is an account nobody can ever get back into.
//
// Two independent pools stand in for two processes: the first writes, is closed
// exactly as a shutdown would close it, and the second reads with no shared
// state.
func TestAccountsSurviveTheProcess(t *testing.T) {
	cfg := testConfig(t)
	ctx := context.Background()

	pool := testPool(t, cfg)
	if _, err := pool.Exec(ctx, `TRUNCATE users CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	registered := repotest.Account("survivor", "passkey-1")
	if err := postgres.NewUserRepository(pool).Create(ctx, registered); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// The old process ends here.
	pool.Close()

	// A new process, sharing nothing but the database.
	restarted, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	defer restarted.Close()

	// The lookup a returning visitor's browser actually makes: the assertion
	// names the credential, not the account.
	found, err := postgres.NewUserRepository(restarted).ByCredentialID(ctx, []byte("passkey-1"))
	if err != nil {
		t.Fatalf("ByCredentialID after restart: %v", err)
	}
	if found.ID != registered.ID {
		t.Errorf("ID = %q, want %q", found.ID, registered.ID)
	}
	if len(found.Credentials) != 1 {
		t.Fatalf("Credentials = %d, want 1", len(found.Credentials))
	}
	if string(found.Credentials[0].PublicKey) != "key-passkey-1" {
		t.Errorf("PublicKey = %q -- the passkey no longer verifies", found.Credentials[0].PublicKey)
	}
}
