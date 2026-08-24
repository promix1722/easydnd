// Package app is the composition root.
//
// It is the only package permitted to import across every layer at once: it
// constructs the outbound adapters, injects them into the application
// services, injects those into the inbound adapter, and owns the server
// lifecycle. It is not itself a layer -- it is the wiring.
//
// Dependency injection here is plain constructor calls. At this size a DI
// framework would add indirection and build-time magic without removing a
// single line of the graph below.
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/jackc/pgx/v5/pgxpool"

	catalogfile "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	oidcadapter "github.com/promix1722/easydnd/internal/adapter/oidc"
	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/adapter/repository/postgres"
	"github.com/promix1722/easydnd/internal/adapter/sheet/hexsheet"
	"github.com/promix1722/easydnd/internal/adapter/token"
	webauthnadapter "github.com/promix1722/easydnd/internal/adapter/webauthn"
	httpapi "github.com/promix1722/easydnd/internal/api/http"
	"github.com/promix1722/easydnd/internal/api/http/helpers"
	authapi "github.com/promix1722/easydnd/internal/api/http/v1/auth"
	catalogapi "github.com/promix1722/easydnd/internal/api/http/v1/catalog"
	characterapi "github.com/promix1722/easydnd/internal/api/http/v1/character"
	"github.com/promix1722/easydnd/internal/api/http/v1/system"
	"github.com/promix1722/easydnd/internal/buildinfo"
	"github.com/promix1722/easydnd/internal/config"
	authdomain "github.com/promix1722/easydnd/internal/domain/auth"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/domain/user"
	authuc "github.com/promix1722/easydnd/internal/usecase/auth"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

// App owns the wired object graph and the HTTP server lifecycle.
type App struct {
	cfg *config.Config
	log *slog.Logger
	srv *http.Server
	// pool is nil when no db.url was configured, which only development
	// permits. Close releases it.
	pool *pgxpool.Pool
}

// New builds the application graph.
//
// ctx bounds the work that can block: connecting to the database and applying
// migrations. A Ctrl-C during a slow connection should abort rather than hang.
func New(ctx context.Context, cfg *config.Config, log *slog.Logger) (*App, error) {
	// Must happen before any engine is constructed. In debug mode gin prints
	// its route table and a warning banner straight to stdout, which corrupts
	// a JSON log stream that something downstream is parsing.
	if cfg.Env == config.EnvProduction {
		gin.SetMode(gin.ReleaseMode)
		gin.DefaultWriter = io.Discard
		gin.DefaultErrorWriter = io.Discard
	} else {
		gin.SetMode(gin.DebugMode)
	}

	log.Info("configuration loaded", slog.String("config", cfg.Source))

	if cfg.Auth.EphemeralSecret {
		log.Warn("auth.session_secret is unset; signing sessions with a key generated for this process only -- every restart signs everyone out")
	}
	// The config file holds the session signing key. World-readable means every
	// account on the box can forge a session cookie.
	if cfg.WorldReadable {
		log.Warn("config file is world-readable and holds the session signing key; chmod 640 it",
			slog.String("config", cfg.Source))
	}

	// Outbound adapters. The assignments below are what type-check the adapters
	// against the domain's ports, which is how the account store could move to
	// Postgres without a line changing above this layer.
	characterRepo := memory.NewCharacterRepository()

	// Accounts are durable; characters are not yet. The character store still
	// lives in the process because no character route exists to reach it, and
	// a schema written against an unfinished feature is a migration nobody can
	// revise later.
	userRepo, pool, err := newUserRepository(ctx, cfg, log)
	if err != nil {
		return nil, err
	}

	// Every failure from here on has to hand the pool back, or a failed start
	// leaves connections open against RDS until the process is reaped.
	fail := func(err error) (*App, error) {
		if pool != nil {
			pool.Close()
		}
		return nil, err
	}

	ceremony, err := webauthnadapter.New(webauthnadapter.Config{
		RPID:            cfg.Auth.RPID,
		RPDisplayName:   cfg.Auth.RPDisplayName,
		RPOrigins:       cfg.Auth.RPOrigins,
		CeremonyTimeout: cfg.Auth.CeremonyTTL,
	})
	if err != nil {
		return fail(fmt.Errorf("build webauthn relying party: %w", err))
	}

	signer := token.NewSigner(cfg.Auth.SessionSecret, cfg.Auth.SessionTTL)

	// External identity providers. An unconfigured one is left out of the map
	// rather than added as a stub, so /v1/auth/providers reports exactly what
	// can actually be used and the client draws only buttons that work.
	federations := map[user.Provider]authdomain.Federation{}
	if cfg.Auth.Google.Configured() {
		google, err := oidcadapter.NewGoogle(oidcadapter.Config{
			ClientID:     cfg.Auth.Google.ClientID,
			ClientSecret: cfg.Auth.Google.ClientSecret,
			RedirectURL:  cfg.Auth.Google.RedirectURL,
		})
		if err != nil {
			return nil, fmt.Errorf("build google identity provider: %w", err)
		}
		federations[user.ProviderGoogle] = google
		log.Info("google sign-in enabled", "redirect_url", cfg.Auth.Google.RedirectURL)
	}

	// The compendium is read from disk, so a bad path must fail here rather
	// than at the first request. Loading the default locale eagerly is what
	// turns a missing or malformed data directory into a startup error --
	// which deploy.sh's health gate then catches and rolls back.
	catalogSource := catalogfile.NewSource(cfg.Data.SRDDir)
	if _, err := catalogSource.Load(ctx, rules.DefaultLocale); err != nil {
		return fail(fmt.Errorf("load SRD data from %s: %w", cfg.Data.SRDDir, err))
	}

	// Application layer.
	characterService := charuc.NewService(
		characterRepo, catalogSource, hexsheet.NewImporter(),
		log.With("usecase", "character"))
	authService := authuc.NewService(userRepo, ceremony, signer, federations, authuc.Config{
		SessionTTL:      cfg.Auth.SessionTTL,
		GuestSessionTTL: cfg.Auth.GuestSessionTTL,
		CeremonyTTL:     cfg.Auth.CeremonyTTL,
	}, log.With("usecase", "auth"))

	// Inbound adapters. The character routes are declared behind
	// RequireSession, and the handler reads the owner from the account that
	// middleware resolved -- which is the honest source the comment that
	// stood here was waiting for.
	router, err := httpapi.NewRouter(cfg, log, httpapi.Handlers{
		System:        system.New(buildinfo.Version),
		Auth:          authapi.New(authService, helpers.CookieOptions{Secure: cfg.Auth.SecureCookies}),
		Authenticator: authService,
		Catalog:       catalogapi.New(catalogSource, log.With("handler", "catalog")),
		Character:     characterapi.New(characterService, log.With("handler", "character")),
	})
	if err != nil {
		return fail(fmt.Errorf("build router: %w", err))
	}

	return &App{
		cfg:  cfg,
		log:  log,
		srv:  httpapi.NewServer(cfg.HTTP, router),
		pool: pool,
	}, nil
}

// newUserRepository picks the account store and, when it is the durable one,
// brings the schema up to date before anything can read it.
//
// Migrating here -- before the pool the request path will use, before the
// router, before the listener binds -- is what makes a schema the code does not
// match a startup failure. deploy.sh health-gates the new release for fifteen
// seconds and rolls back when it never answers, so a bad migration undoes
// itself with no operator involved.
//
// Note the consequence, which is written up in docs/backend.md: the rollback
// runs against the schema that was just applied, so the PREVIOUS binary has to
// work on it. Migrations must be expand-only.
func newUserRepository(ctx context.Context, cfg *config.Config, log *slog.Logger) (user.Repository, *pgxpool.Pool, error) {
	if !cfg.DB.Enabled() {
		// config.validate refuses this in production, so it can only be a
		// developer with no Postgres running.
		log.Warn("db.url is unset; accounts live in this process only -- every restart destroys every account and every registered passkey",
			"config", cfg.Source)
		return memory.NewUserRepository(), nil, nil
	}

	if cfg.DB.MigrateOnStart {
		if err := postgres.Migrate(ctx, cfg.DB, log, postgres.CommandUp); err != nil {
			return nil, nil, fmt.Errorf("migrate database: %w", err)
		}
	}

	pool, err := postgres.NewPool(ctx, cfg.DB)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to database: %w", err)
	}
	return postgres.NewUserRepository(pool), pool, nil
}

// Migrate runs one schema command and returns, without building the graph.
//
// It lives here so that cmd/easydnd keeps importing only internal/app,
// internal/buildinfo, internal/config and internal/logging -- the entrypoint
// has never known which database this project uses, and adding a flag is not a
// reason to start.
func Migrate(ctx context.Context, cfg *config.Config, log *slog.Logger, cmd postgres.Command) error {
	return postgres.Migrate(ctx, cfg.DB, log, cmd)
}

// ParseMigrateCommand converts a -migrate flag value into a command.
func ParseMigrateCommand(s string) (postgres.Command, error) {
	return postgres.ParseCommand(s)
}

// MigrateDown is re-exported so that cmd/easydnd can guard it without
// importing the repository package.
const MigrateDown = postgres.CommandDown

// Run serves until ctx is cancelled, then drains in-flight requests within the
// configured shutdown timeout.
func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		a.log.Info("http server listening",
			"addr", a.cfg.HTTP.Addr(),
			"env", a.cfg.Env,
		)
		if err := a.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("listen and serve: %w", err)
		}
		return nil
	case <-ctx.Done():
		a.log.Info("shutdown signal received", "timeout", a.cfg.HTTP.ShutdownTimeout)
	}

	// context.WithoutCancel because ctx is already done: the drain needs a
	// fresh deadline of its own rather than inheriting an expired one.
	shutdownCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx), a.cfg.HTTP.ShutdownTimeout)
	defer cancel()

	if err := a.srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	a.log.Info("shutdown complete")
	return nil
}

// Close releases what outlives the HTTP server.
//
// It must run after Run returns, not alongside it: pgxpool.Close blocks until
// every connection is handed back, and the requests still draining in
// Shutdown are holding some of them.
func (a *App) Close() {
	if a.pool != nil {
		a.pool.Close()
	}
}
