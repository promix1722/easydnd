package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Defaults mirror the Makefile's. They are duplicated rather than shared
// because the Makefile is the one place a human edits them and it passes every
// value on the command line; these exist so that running the command by hand
// does something sensible.
const (
	defaultCount      = 10
	defaultWebBase    = 8080
	defaultAPIBase    = 18080
	defaultPGBase     = 5440
	defaultPublicBase = 8880
)

// claimsDirName lives under the machine's runtime directory so that claims are
// visible to every worktree but never outlive a reboot -- a slot whose machine
// has restarted is free by definition.
const claimsDirName = "easydnd-devslots"

func parseLayout(args []string) (layout, error) {
	fs := flag.NewFlagSet("devslot", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	count := fs.Int("count", defaultCount, "how many slots exist")
	web := fs.Int("web", defaultWebBase, "first web (Vite) port")
	api := fs.Int("api", defaultAPIBase, "first API port")
	pg := fs.Int("pg", defaultPGBase, "first Postgres port")
	publicHost := fs.String("public-host", "", "host a browser reaches this machine on, if a proxy is in front")
	publicBase := fs.Int("public-base", defaultPublicBase, "first port on that host")

	if err := fs.Parse(args); err != nil {
		return layout{}, fmt.Errorf("parse flags: %w", err)
	}
	if *count < 1 {
		return layout{}, fmt.Errorf("-count must be at least 1, got %d", *count)
	}

	return layout{
		count:      *count,
		webBase:    *web,
		apiBase:    *api,
		pgBase:     *pg,
		publicHost: *publicHost,
		publicBase: *publicBase,
		probe:      portFree,
		claims:     claimsDir(),
	}, nil
}

func claimsDir() string {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, claimsDirName)
}
