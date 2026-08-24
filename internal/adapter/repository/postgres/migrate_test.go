package postgres

import (
	"context"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/promix1722/easydnd/internal/adapter/repository/postgres/migrations"
	"github.com/promix1722/easydnd/internal/config"
)

// testDBConfig returns the configuration for the throwaway database, or skips.
//
// Skipping rather than failing is what lets `go test ./...` stay green on a
// machine with no Postgres -- which every contributor has before they have run
// `make db/up`, and which `make verify` relies on.
func testDBConfig(t *testing.T) config.DBConfig {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is unset; skipping the Postgres adapter tests")
	}
	return config.DBConfig{
		URL:            url,
		MaxConns:       5,
		ConnectTimeout: 10 * time.Second,
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// A misplaced //go:embed does not fail the build and does not warn: goose globs
// "*.sql" at the FS root, finds nothing, and a server starts cheerfully against
// an empty database. This test is the only thing standing between that and a
// deploy.
func TestMigrationsAreEmbedded(t *testing.T) {
	entries, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no .sql files at the root of migrations.FS; goose would report ErrNoMigrations")
	}
	for _, name := range entries {
		body, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if len(body) == 0 {
			t.Errorf("%s is empty", name)
		}
	}
}

// Applying twice must be a no-op. Supervisor restarts, deploy.sh's rollback
// path and an operator running -migrate up by hand all produce a second run.
func TestMigrateUpIsIdempotent(t *testing.T) {
	cfg := testDBConfig(t)
	ctx := context.Background()
	log := testLogger()

	if err := Migrate(ctx, cfg, log, CommandUp); err != nil {
		t.Fatalf("first up: %v", err)
	}
	if err := Migrate(ctx, cfg, log, CommandUp); err != nil {
		t.Fatalf("second up: %v", err)
	}
	if err := Migrate(ctx, cfg, log, CommandStatus); err != nil {
		t.Fatalf("status: %v", err)
	}
}

func TestParseCommand(t *testing.T) {
	for _, in := range []string{"up", "status", "down"} {
		if _, err := ParseCommand(in); err != nil {
			t.Errorf("ParseCommand(%q): %v", in, err)
		}
	}
	if _, err := ParseCommand("sideways"); err == nil {
		t.Error("ParseCommand accepted an unknown command")
	}
}
