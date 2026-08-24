package postgres

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/promix1722/easydnd/internal/config"
)

// NewPool connects to the configured database and proves the connection works.
//
// The ping is not ceremony. Without it a wrong db.url, a closed security
// group or an RDS instance that has not finished starting surfaces at the first
// sign-in rather than at startup -- and startup is where deploy.sh's health gate
// is watching.
func NewPool(ctx context.Context, cfg config.DBConfig) (*pgxpool.Pool, error) {
	poolCfg, err := newPoolConfig(cfg.URL, cfg.MaxConns, cfg.ConnectTimeout)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// newPoolConfig parses a libpq URL and repairs pgx's TLS defaults.
//
// Three things about pgx's configTLS make this function load-bearing rather
// than a convenience:
//
//  1. An unset sslmode defaults to "prefer", which sets InsecureSkipVerify and
//     ALSO appends a plaintext fallback. A db.url that simply forgot to
//     say sslmode therefore gets an unauthenticated -- possibly unencrypted --
//     connection to the table holding every account.
//  2. sslmode=verify-full leaves RootCAs nil, meaning the system pool. Amazon's
//     RDS CAs are not in a stock ca-certificates store, so verify-full on its
//     own does not merely fail to protect anything: it fails to connect.
//  3. sslrootcert= is read from the filesystem, so an embedded PEM can only be
//     installed by reaching into the parsed config afterwards.
//
// Note that PGSSLMODE and PGSSLROOTCERT in the environment also reach
// ParseConfig and are not inspected here; the URL is the documented interface.
func newPoolConfig(dsn string, maxConns int32, connectTimeout time.Duration) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse db.url: %w", err)
	}

	cfg.MaxConns = maxConns
	cfg.ConnConfig.ConnectTimeout = connectTimeout

	switch {
	case dsnHasKey(dsn, "sslrootcert"):
		// The operator named their own CA file and pgx has already loaded it.
		// Overriding that would make the setting a lie.

	case cfg.ConnConfig.TLSConfig == nil:
		// sslmode=disable. Honoured so a local container and the CI service
		// container work; config.validate is what stops it reaching production.

	default:
		roots, err := rdsRoots()
		if err != nil {
			return nil, err
		}
		tc := cfg.ConnConfig.TLSConfig
		tc.RootCAs = roots

		if !dsnHasKey(dsn, "sslmode") {
			// Default to verify-full rather than libpq's "prefer". These three
			// assignments are exactly what configTLS builds for verify-full.
			tc.InsecureSkipVerify = false
			tc.VerifyPeerCertificate = nil
			tc.ServerName = cfg.ConnConfig.Host
		}

		// ParseConfig's own documentation warns that modifying TLSConfig does
		// not touch Fallbacks, "which can lead to an unexpected unencrypted
		// connection" -- and sslmode=prefer produces precisely such a fallback.
		// This service talks to one host and has no interest in downgrading.
		cfg.ConnConfig.Fallbacks = nil
	}

	return cfg, nil
}

// dsnHasKey reports whether the connection URL sets the given parameter.
//
// A malformed URL is reported as "not set" rather than as an error: ParseConfig
// above is the authority on whether the DSN is usable, and it produces a far
// better message than anything this function could.
func dsnHasKey(dsn, key string) bool {
	u, err := url.Parse(dsn)
	if err != nil {
		return false
	}
	return u.Query().Has(key)
}
