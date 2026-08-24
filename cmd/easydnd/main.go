// Command easydnd serves the D&D character and battle tracker HTTP API.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/promix1722/easydnd/internal/app"
	"github.com/promix1722/easydnd/internal/buildinfo"
	"github.com/promix1722/easydnd/internal/config"
	"github.com/promix1722/easydnd/internal/logging"
)

func main() {
	// run exists so that deferred calls still fire: os.Exit must only ever be
	// reached from main.
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	showVersion := flag.Bool("version", false, "print the build version and exit")
	// Overrides EASYDND_CONFIG. Mostly for development -- `make run/server`
	// says -config config.dev.yaml rather than exporting a variable.
	configPath := flag.String("config", "", "path to the YAML config file (overrides $"+config.EnvConfigPath+")")
	migrateCmd := flag.String("migrate", "",
		"run a schema migration and exit: `up`, `status` or `down`")
	migrateForce := flag.Bool("migrate-force", false,
		"permit a destructive -migrate in production")
	flag.Parse()

	// Deliberately before config.Load: CI asserts `./easydnd -version` equals
	// the commit SHA right after building, where no config file exists.
	if *showVersion {
		// CI asserts this equals the commit SHA immediately after building.
		// Without that check a wrong -X package path is silent: the linker
		// does not complain about a symbol it cannot find, so the binary would
		// ship reporting "dev", fail the deploy health gate, and roll back.
		fmt.Println(buildinfo.Version)
		return nil
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log, err := logging.New(cfg.Log.Level, cfg.Log.Format, os.Stdout)
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	log = log.With(
		slog.String("service", "easydnd"),
		slog.String("version", buildinfo.Version),
	)

	// supervisorctl restart sends SIGTERM; catching it is what makes the
	// in-flight request drain in App.Run actually happen.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *migrateCmd != "" {
		return migrate(ctx, cfg, log, *migrateCmd, *migrateForce)
	}

	a, err := app.New(ctx, cfg, log)
	if err != nil {
		return fmt.Errorf("build app: %w", err)
	}
	defer a.Close()

	return a.Run(ctx)
}

// migrate runs one schema command and returns without starting the server.
//
// The normal path needs none of this: a release migrates itself at startup.
// This exists for the operator who set DB_MIGRATE_ON_START=false to stage a
// risky change, and for reading the current state of a database without
// guessing from the logs.
func migrate(ctx context.Context, cfg *config.Config, log *slog.Logger, name string, force bool) error {
	if !cfg.DB.Enabled() {
		return fmt.Errorf("-migrate needs db.url in %s", cfg.Source)
	}

	cmd, err := app.ParseMigrateCommand(name)
	if err != nil {
		return err
	}

	// A down migration on the account store drops passkeys, and a passkey
	// cannot be reissued: there is no password and no email to recover
	// through. Making this deliberate costs one flag and is worth it.
	if cmd == app.MigrateDown && cfg.Env == config.EnvProduction && !force {
		return fmt.Errorf(
			"-migrate down in production needs -migrate-force; it deletes passkeys that cannot be reissued")
	}

	return app.Migrate(ctx, cfg, log, cmd)
}
