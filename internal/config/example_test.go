package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The committed config files are the only documentation of this schema that is
// executable, and they are what development and production actually load. Parsing
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

// The production config ships inside the release with no secret in it, so on
// its own it must be refused -- and with the secrets the environment supplies
// in production, it must load. That pair is what proves no key has drifted and
// that nothing the server needs was left out of easydnd.example.env.
func TestProdConfigNeedsItsSecretsFromTheEnvironment(t *testing.T) {
	path := repoFile(t, "config.prod.yaml")

	_, err := Load(path)
	if err == nil {
		t.Fatal("config.prod.yaml loaded with no secrets in the environment")
	}
	if !strings.Contains(err.Error(), "auth.session_secret") {
		t.Fatalf("config.prod.yaml failed for the wrong reason: %v", err)
	}

	t.Setenv("EASYDND_SESSION_SECRET", base64Secret)
	t.Setenv("EASYDND_DB_URL", "postgres://easydnd:s3cret@db.example.com:5432/easydnd?sslmode=verify-full")
	t.Setenv("EASYDND_AGENT_API_KEY", "sk-test")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("config.prod.yaml does not load with its secrets: %v", err)
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
	// The key arrives from the env file and the model from the config; the AI
	// Wizard needs both.
	if cfg.Agent.APIKey != "sk-test" || cfg.Agent.Model == "" {
		t.Errorf("agent = key %q model %q, want the env key and a configured model", cfg.Agent.APIKey, cfg.Agent.Model)
	}
}

// A committed config is public. Parsed with an empty environment, neither may
// hold any of the values that belong in an env file.
func TestCommittedConfigsCarryNoSecret(t *testing.T) {
	for _, name := range []string{"config.dev.yaml", "config.prod.yaml"} {
		src, err := readFile(repoFile(t, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		f := src.cfg
		for key, v := range map[string]string{
			"analytics.token":           f.Analytics.Token,
			"agent.api_key":             f.Agent.APIKey,
			"auth.session_secret":       f.Auth.SessionSecret,
			"db.url":                    f.DB.URL,
			"auth.google.client_secret": f.Auth.Google.ClientSecret,
		} {
			if v != "" {
				t.Errorf("%s sets %s; it belongs in the env file", name, key)
			}
		}
	}
}

// The template is the thing an operator copies, so its placeholders must be the
// ones the loader rejects by name: an unedited prod.env cannot reach production.
func TestExampleEnvPlaceholdersAreRejected(t *testing.T) {
	raw, err := os.ReadFile(repoFile(t, "easydnd.example.env"))
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	for _, placeholder := range []string{placeholderSecret, placeholderDBPassword, placeholderGoogleSecret} {
		if !strings.Contains(string(raw), placeholder) {
			t.Errorf("easydnd.example.env no longer carries %q; update the loader's rejection alongside it", placeholder)
		}
	}
}
