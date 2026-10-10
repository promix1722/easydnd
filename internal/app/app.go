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

	agentmodel "github.com/promix1722/easydnd/internal/adapter/agent/openai"
	catalogfile "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	oidcadapter "github.com/promix1722/easydnd/internal/adapter/oidc"
	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/adapter/repository/postgres"
	"github.com/promix1722/easydnd/internal/adapter/token"
	webauthnadapter "github.com/promix1722/easydnd/internal/adapter/webauthn"
	httpapi "github.com/promix1722/easydnd/internal/api/http"
	"github.com/promix1722/easydnd/internal/api/http/helpers"
	adminapi "github.com/promix1722/easydnd/internal/api/http/v1/admin"
	appearanceapi "github.com/promix1722/easydnd/internal/api/http/v1/appearance"
	authapi "github.com/promix1722/easydnd/internal/api/http/v1/auth"
	catalogapi "github.com/promix1722/easydnd/internal/api/http/v1/catalog"
	characterapi "github.com/promix1722/easydnd/internal/api/http/v1/character"
	"github.com/promix1722/easydnd/internal/api/http/v1/development"
	folderapi "github.com/promix1722/easydnd/internal/api/http/v1/folder"
	gameapi "github.com/promix1722/easydnd/internal/api/http/v1/game"
	groupapi "github.com/promix1722/easydnd/internal/api/http/v1/group"
	packapi "github.com/promix1722/easydnd/internal/api/http/v1/pack"
	profileapi "github.com/promix1722/easydnd/internal/api/http/v1/profile"
	"github.com/promix1722/easydnd/internal/api/http/v1/system"
	"github.com/promix1722/easydnd/internal/buildinfo"
	"github.com/promix1722/easydnd/internal/config"
	authdomain "github.com/promix1722/easydnd/internal/domain/auth"
	"github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/game"
	"github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/domain/user"
	adminuc "github.com/promix1722/easydnd/internal/usecase/admin"
	agentuc "github.com/promix1722/easydnd/internal/usecase/agent"
	appearanceuc "github.com/promix1722/easydnd/internal/usecase/appearance"
	authuc "github.com/promix1722/easydnd/internal/usecase/auth"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
	gameuc "github.com/promix1722/easydnd/internal/usecase/game"
	groupuc "github.com/promix1722/easydnd/internal/usecase/group"
	packuc "github.com/promix1722/easydnd/internal/usecase/pack"
	profileuc "github.com/promix1722/easydnd/internal/usecase/profile"
)

// App owns the wired object graph and the HTTP server lifecycle.
type App struct {
	agent *agentuc.Agent
	cfg   *config.Config
	log   *slog.Logger
	srv   *http.Server
	// pool is nil when no db.url was configured, which only development
	// permits. Close releases it.
	pool *pgxpool.Pool
}

// Options carries what the command line decides rather than the config file.
//
// One field so far, and it is a struct rather than a fourth parameter because
// the alternative is renaming every call site the next time something is added
// -- and because `New(ctx, cfg, log, "")` at the only call site says nothing
// about what the empty string means.
type Options struct {
	// WebDir serves a built frontend bundle alongside the API. Development
	// only; see internal/api/http/static.go.
	WebDir string
}

// New builds the application graph.
//
// ctx bounds the work that can block: connecting to the database and applying
// migrations. A Ctrl-C during a slow connection should abort rather than hang.
func New(ctx context.Context, cfg *config.Config, log *slog.Logger, opts Options) (*App, error) {
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

	// Outbound adapters. The assignments in newRepositories are what
	// type-check the adapters against the domain's ports: every store has an
	// in-memory and a Postgres implementation, and which one runs is decided
	// there, by db.url, in one place.
	// ponytail: a guest's characters are rows nothing deletes once the guest
	// session expires; see docs/known-caveats.md. Add a sweep beside the
	// wizard's, keyed on the guest id prefix and the guest session TTL, when
	// the table's size says so.
	repos, err := newRepositories(ctx, cfg, log)
	if err != nil {
		return nil, err
	}
	userRepo, groupRepo, pool := repos.users, repos.groups, repos.pool
	characterRepo, folderRepo := repos.characters, repos.folders
	sharedRepo, gameRepo := repos.shared, repos.games

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
	packPaths := append([]string{cfg.Data.SRDDir}, cfg.Data.PackFiles...)
	var roots []catalogfile.Dependency
	for id, version := range cfg.Data.DefaultPacks {
		roots = append(roots, catalogfile.Dependency{ID: id, Version: version})
	}
	folders := make([]catalogfile.PackFolder, 0, len(cfg.Data.AutoloadPacks))
	for _, folder := range cfg.Data.AutoloadPacks {
		folders = append(folders, catalogfile.PackFolder{Path: folder.Path, ID: folder.ID})
	}
	// Folders rather than paths: a folder is installed and compiled but never
	// becomes a default root, which is half of what makes a pack private.
	for _, path := range cfg.Data.PrivatePackFiles {
		folders = append(folders, catalogfile.PackFolder{Path: path, Restricted: true})
	}
	catalogSource, err := catalogfile.NewRegistry(packPaths, roots, cfg.Data.PackArchive, folders...)
	if err != nil {
		return fail(fmt.Errorf("load rule packs: %w", err))
	}
	if _, err := catalogSource.Load(ctx, rules.DefaultLocale); err != nil {
		return fail(fmt.Errorf("load SRD data from %s: %w", cfg.Data.SRDDir, err))
	}

	packSource := catalogfile.NewAuthoring(catalogSource, repos.packs)
	packService := packuc.NewService(repos.packs, packSource, groupRepo, userRepo)
	packService.SetSuperadmins(cfg.Auth.Superadmins)

	// Application layer. The game service is built first because the two
	// services either side of it have to tell it when the things it refers to
	// go away -- a deleted character comes off every table, a deleted group
	// takes its games with it -- and it refers to their stores rather than to
	// them, so nothing points back and the graph stays acyclic.
	//
	// One character store, shared with the character service below rather than
	// a second instance. The game service reads shared characters out of the
	// same map that one writes them into; give it its own and every shared
	// character is a 404 with nothing in the failure pointing at why. It is
	// the same hazard as the one account store, and it fails the same silent
	// way.
	gameService := gameuc.NewService(
		gameRepo, sharedRepo, groupRepo, characterRepo, packSource,
		log.With("usecase", "game"))
	gameService.SetSuperadmin(packService.Superadmin)
	characterService := charuc.NewService(
		characterRepo, folderRepo, packSource, gameService,
		log.With("usecase", "character"))
	characterService.SetPackAccess(packService)
	characterService.SetCopyLinks(signer)
	authService := authuc.NewService(userRepo, ceremony, signer, federations, authuc.Config{
		SessionTTL:      cfg.Auth.SessionTTL,
		GuestSessionTTL: cfg.Auth.GuestSessionTTL,
		CeremonyTTL:     cfg.Auth.CeremonyTTL,
	}, log.With("usecase", "auth"))
	// The same signer mints invite links. It is the one thing that knows the
	// signing key, and an invite is separated from a session cookie by the
	// token kind rather than by a second key -- see internal/adapter/token.
	groupService := groupuc.NewService(
		groupRepo, userRepo, signer, gameService, log.With("usecase", "group"))

	var devHandler *development.Handler
	if cfg.Env == config.EnvDevelopment {
		// Demo selections are authored against the base dataset. A different
		// default pack must not change those choices or prevent startup.
		basePack, err := catalogfile.LoadPack(cfg.Data.SRDDir)
		if err != nil {
			return fail(fmt.Errorf("load development seed rules: %w", err))
		}
		seedRules, err := catalogSource.Resolve([]catalogfile.Dependency{{ID: basePack.Manifest.ID, Version: basePack.Manifest.Version}})
		if err != nil {
			return fail(fmt.Errorf("resolve development seed rules: %w", err))
		}
		seed, err := seedDevelopment(ctx, userRepo, groupRepo, characterService, gameService, signer, cfg.Auth.SessionTTL, seedRules)
		if err != nil {
			return fail(fmt.Errorf("seed development game: %w", err))
		}
		devHandler = development.New(seed, helpers.NewCookieOptions(cfg), cfg.Auth.SessionTTL)
		log.Info("development party seeded", "accounts", []string{"master", "player1", "player2"}, "group_id", devGroupID, "game_ids", seed.games)
	}
	var model agentuc.AgentModel
	if cfg.Agent.APIKey != "" {
		model = agentmodel.New(cfg.Agent.APIKey, cfg.Agent.Model, cfg.Agent.ReasoningEffort)
	}
	// The wizard's chats are in the database when there is one, so that a
	// restart keeps them and a second process can answer for them. Without
	// one they are in memory, like everything else in such a process.
	var agentStore agentuc.Store
	if pool != nil {
		agentStore = postgres.NewAgentStore(pool)
	}
	agent := agentuc.NewAgent(characterService, model, agentuc.AgentConfig{Workers: cfg.Agent.Workers, MaxTurns: cfg.Agent.MaxTurns, MaxSessions: cfg.Agent.MaxSessions, Timeout: cfg.Agent.RequestTimeout, Store: agentStore})

	// Inbound adapters. The character routes are declared behind
	// RequireSession, and the handler reads the owner from the account that
	// middleware resolved -- which is the honest source the comment that
	// stood here was waiting for.
	router, err := httpapi.NewRouter(cfg, log, httpapi.Handlers{
		Development:   devHandler,
		System:        system.New(buildinfo.Version),
		Version:       buildinfo.Version,
		WebDir:        opts.WebDir,
		Admin:         adminapi.New(adminuc.NewService(userRepo, characterRepo)),
		Auth:          authapi.New(authService, helpers.NewCookieOptions(cfg)).WithSuperadmins(cfg.Auth.Superadmins),
		Appearance:    appearanceapi.New(appearanceuc.NewService(userRepo)),
		Profile:       profileapi.New(profileuc.NewService(userRepo)),
		Authenticator: authService,
		Pack:          packapi.New(packService, packSource),
		Catalog:       catalogapi.New(packSource, log.With("handler", "catalog")),
		Character:     characterapi.New(characterService, log.With("handler", "character")).WithAgent(agent),
		Folder:        folderapi.New(characterService, log.With("handler", "folder")),
		Game:          gameapi.New(gameService, log.With("handler", "game")),
		Group:         groupapi.New(groupService, log.With("handler", "group")),
	})
	if err != nil {
		agent.Close()
		return fail(fmt.Errorf("build router: %w", err))
	}

	return &App{
		agent: agent,
		cfg:   cfg,
		log:   log,
		srv:   httpapi.NewServer(cfg.HTTP, router),
		pool:  pool,
	}, nil
}

// repositories is every outbound store the graph is built from, and the pool
// they share when they are the durable ones.
type repositories struct {
	users      user.Repository
	groups     group.Repository
	characters character.Repository
	folders    character.FolderRepository
	shared     game.SharedRepository
	games      game.Repository
	packs      pack.Repository
	pool       *pgxpool.Pool
}

// newRepositories picks the stores and, when they are the durable ones,
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
func newRepositories(
	ctx context.Context, cfg *config.Config, log *slog.Logger,
) (repositories, error) {
	if !cfg.DB.Enabled() {
		// config.validate refuses this in production, so it can only be a
		// developer with no Postgres running.
		log.Warn("db.url is unset; accounts, groups, characters, folders and games live in this process only -- every restart destroys all of them, every registered passkey included",
			"config", cfg.Source)
		// One user store, shared. The in-memory group store reads display
		// names out of it, exactly as the Postgres one reads them with a
		// join -- give it a second instance and every roster comes back
		// nameless.
		users := memory.NewUserRepository()
		return repositories{
			users:      users,
			groups:     memory.NewGroupRepository(users),
			characters: memory.NewCharacterRepository(),
			folders:    memory.NewFolderRepository(),
			shared:     memory.NewSharedRepository(),
			games:      memory.NewGameRepository(),
			packs:      memory.NewPackRepository(),
		}, nil
	}

	if cfg.DB.MigrateOnStart {
		if err := postgres.Migrate(ctx, cfg.DB, log, postgres.CommandUp); err != nil {
			return repositories{}, fmt.Errorf("migrate database: %w", err)
		}
	}

	pool, err := postgres.NewPool(ctx, cfg.DB)
	if err != nil {
		return repositories{}, fmt.Errorf("connect to database: %w", err)
	}
	return repositories{
		users:      postgres.NewUserRepository(pool),
		groups:     postgres.NewGroupRepository(pool),
		characters: postgres.NewCharacterRepository(pool),
		folders:    postgres.NewFolderRepository(pool),
		shared:     postgres.NewSharedRepository(pool),
		games:      postgres.NewGameRepository(pool),
		packs:      postgres.NewPackRepository(pool),
		pool:       pool,
	}, nil
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
	if a.agent != nil {
		a.agent.Close()
	}
	if a.pool != nil {
		a.pool.Close()
	}
}
