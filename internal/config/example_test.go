package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The committed config files are the only documentation of this schema that is
// executable, and one of them is what `make run/server` actually loads. Parsing
// them here means a key renamed in the loader cannot ship with the examples
// still naming the old one -- strict unknown-key rejection turns any drift into
// a test failure.

// repoFile resolves a path relative to the repository root.
func repoFile(t *testing.T, rel string) string {
	t.Helper()
	return filepath.Join("..", "..", rel)
}

func TestDevConfigLoads(t *testing.T) {
	cfg, err := Load(repoFile(t, "config.dev.yaml"))
	if err != nil {
		t.Fatalf("config.dev.yaml does not load: %v", err)
	}
	if cfg.Env != EnvDevelopment {
		t.Errorf("Env = %q, want %q", cfg.Env, EnvDevelopment)
	}
	// No secret is committed, so development must be inventing one.
	if !cfg.Auth.EphemeralSecret {
		t.Error("config.dev.yaml appears to carry a session secret; it must not")
	}
	if cfg.Auth.RPID != "localhost" {
		t.Errorf("RPID = %q, want localhost so dev passkeys cannot work in production", cfg.Auth.RPID)
	}
}

// The example is a template with two placeholder credentials -- the session
// signing key and the database password -- so it must parse and must NOT be
// usable as-is: an operator who forgets to edit it should be told, not left
// running production on values published in this repository.
func TestExampleConfigParsesAndRejectsThePlaceholderSecret(t *testing.T) {
	path := repoFile(t, "deploy/config.example.yaml")

	_, err := Load(path)
	if err == nil {
		t.Fatal("deploy/config.example.yaml loaded with its placeholder secret intact")
	}
	if !strings.Contains(err.Error(), "auth.session_secret") {
		t.Fatalf("deploy/config.example.yaml failed for the wrong reason: %v", err)
	}

	// With a real secret substituted, the rest of the file must be valid --
	// that is what proves no other key has drifted.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	fixed := strings.Replace(string(raw),
		"REPLACE-ME-WITH-openssl-rand-base64-48", base64Secret, 1)
	if fixed == string(raw) {
		t.Fatal("placeholder secret not found; update this test alongside the example")
	}

	// The database password is the second published credential, rejected by
	// name for the same reason. Substitute it too, or the check below would be
	// asserting on that error instead of on key drift.
	withPassword := strings.Replace(fixed, placeholderDBPassword, "s3cret", 1)
	if withPassword == fixed {
		t.Fatal("placeholder db password not found; update this test alongside the example")
	}

	cfg, err := Load(writeConfig(t, withPassword))
	if err != nil {
		t.Fatalf("deploy/config.example.yaml does not load with a real secret: %v", err)
	}
	if cfg.Env != EnvProduction {
		t.Errorf("Env = %q, want %q", cfg.Env, EnvProduction)
	}
	if cfg.Auth.RPID != "easydnd.org" {
		t.Errorf("RPID = %q, want easydnd.org", cfg.Auth.RPID)
	}
	if !filepath.IsAbs(cfg.Data.SRDDir) {
		t.Errorf("srd_dir = %q, want an absolute path", cfg.Data.SRDDir)
	}
	if !cfg.DB.Enabled() {
		t.Error("the example configures no database; production refuses to start that way")
	}
	// The example is the thing an operator copies, so it is the one place the
	// TLS mode has to be right by default.
	if !strings.Contains(cfg.DB.URL, "sslmode=verify-full") {
		t.Errorf("db.url = %q, want sslmode=verify-full", cfg.DB.URL)
	}
}

// An operator who edits the session secret but leaves the database password is
// the likelier half-finished install, so it gets its own test.
func TestExampleConfigRejectsThePlaceholderDBPassword(t *testing.T) {
	raw, err := os.ReadFile(repoFile(t, "deploy/config.example.yaml"))
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	fixed := strings.Replace(string(raw),
		"REPLACE-ME-WITH-openssl-rand-base64-48", base64Secret, 1)

	err = loadErr(t, fixed, "with the placeholder database password intact")
	if !strings.Contains(err.Error(), "db.url") {
		t.Fatalf("failed for the wrong reason: %v", err)
	}
}
