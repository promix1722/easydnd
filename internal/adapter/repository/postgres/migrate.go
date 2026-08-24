package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"github.com/promix1722/easydnd/internal/adapter/repository/postgres/migrations"
	"github.com/promix1722/easydnd/internal/config"
)

// Command is one schema operation the binary can be asked to perform.
type Command string

// The commands `easydnd -migrate` accepts.
const (
	CommandUp     Command = "up"
	CommandStatus Command = "status"
	CommandDown   Command = "down"
)

// ParseCommand converts a -migrate flag value into a Command.
func ParseCommand(s string) (Command, error) {
	switch Command(s) {
	case CommandUp:
		return CommandUp, nil
	case CommandStatus:
		return CommandStatus, nil
	case CommandDown:
		return CommandDown, nil
	default:
		return "", fmt.Errorf("unknown migrate command %q; want up, status or down", s)
	}
}

// Migrate runs cmd against the configured database on a connection pool of its
// own, then closes it.
//
// The separate pool is the point, not an oversight. goose's advisory lock is a
// SESSION lock, and stdlib.OpenDBFromPool hands goose a pgxpool connection that
// is released back into the pool when it is done. If the unlock ever fails --
// goose retries for a minute and then gives up -- the lock stays held by a
// connection that is now serving sign-ins, and nothing releases it until that
// connection is finally destroyed. A pool this function closes cannot do that.
// The cost is one extra TLS handshake at startup.
func Migrate(ctx context.Context, cfg config.DBConfig, log *slog.Logger, cmd Command) error {
	// Two connections, not cfg.MaxConns: goose uses one for the lock and one
	// for the work, and a migration is not a throughput problem.
	poolCfg, err := newPoolConfig(cfg.URL, 2, cfg.ConnectTimeout)
	if err != nil {
		return err
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return fmt.Errorf("connect for migrations: %w", err)
	}
	defer pool.Close()

	// Documented not to close the pool, which is why the defer above is still
	// the one that matters.
	db := stdlib.OpenDBFromPool(pool)
	defer func() { _ = db.Close() }()

	provider, err := newProvider(db, log)
	if err != nil {
		return err
	}

	switch cmd {
	case CommandUp:
		return up(ctx, provider, log)
	case CommandStatus:
		return status(ctx, provider, log)
	case CommandDown:
		return down(ctx, provider, log)
	default:
		return fmt.Errorf("unknown migrate command %q", cmd)
	}
}

// newProvider builds the goose provider over an already-open *sql.DB.
func newProvider(db *sql.DB, log *slog.Logger) (*goose.Provider, error) {
	// WithSessionLocker is not optional. Its own documentation says that
	// without it "locking is disabled" -- so two processes racing `up`, which
	// deploy.sh's rollback path and supervisor's autorestart can both produce,
	// would each read the same version and each try the same CREATE TABLE.
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return nil, fmt.Errorf("build advisory locker: %w", err)
	}

	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		db,
		migrations.FS,
		goose.WithSessionLocker(locker),
		goose.WithSlog(log.With("component", "goose")),
	)
	if err != nil {
		return nil, fmt.Errorf("build migration provider: %w", err)
	}
	return provider, nil
}

func up(ctx context.Context, p *goose.Provider, log *slog.Logger) error {
	results, err := p.Up(ctx)
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	for _, r := range results {
		log.Info("migration applied",
			"schema_version", r.Source.Version,
			"path", r.Source.Path,
			"duration", r.Duration,
		)
	}
	version, err := p.GetDBVersion(ctx)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	log.Info("schema up to date", "schema_version", version, "applied", len(results))
	return nil
}

func status(ctx context.Context, p *goose.Provider, log *slog.Logger) error {
	items, err := p.Status(ctx)
	if err != nil {
		return fmt.Errorf("read migration status: %w", err)
	}
	for _, item := range items {
		log.Info("migration",
			"schema_version", item.Source.Version,
			"path", item.Source.Path,
			"state", item.State,
			"applied_at", item.AppliedAt,
		)
	}
	return nil
}

func down(ctx context.Context, p *goose.Provider, log *slog.Logger) error {
	result, err := p.Down(ctx)
	if err != nil {
		return fmt.Errorf("roll back migration: %w", err)
	}
	log.Warn("migration rolled back",
		"schema_version", result.Source.Version,
		"path", result.Source.Path,
	)
	return nil
}
