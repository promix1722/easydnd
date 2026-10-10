package config

import (
	"fmt"
	"strings"
	"testing"
)

func TestAnalyticsConfig(t *testing.T) {
	t.Setenv("EASYDND_POSTHOG_TOKEN", "")
	for _, tc := range []struct {
		name, token, host string
		invalid           bool
	}{
		{name: "disabled"},
		{name: "configured", token: "phc_test", host: "https://eu.i.posthog.com"},
		{name: "missing host", token: "phc_test", invalid: true},
		{name: "insecure host", token: "phc_test", host: "http://eu.i.posthog.com", invalid: true},
		{name: "missing hostname", token: "phc_test", host: "https:///ingest", invalid: true},
		{name: "credentials", token: "phc_test", host: "https://user:secret@example.com", invalid: true},
		{name: "query", token: "phc_test", host: "https://example.com?secret=x", invalid: true},
		{name: "personal key", token: "phx_secret", host: "https://eu.i.posthog.com", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := fmt.Sprintf("env: development\nanalytics:\n  token: %q\n  host: %q\n", tc.token, tc.host)
			cfg, err := Load(writeConfig(t, body))
			if tc.invalid {
				if err == nil || !strings.Contains(err.Error(), "analytics.") {
					t.Fatalf("want analytics validation error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Analytics.Token != tc.token || cfg.Analytics.Host != tc.host {
				t.Fatalf("unexpected config: %+v", cfg.Analytics)
			}
		})
	}
}

func TestAnalyticsTokenFromEnvironment(t *testing.T) {
	for _, name := range []string{"config.dev.yaml", "config.prod.yaml"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("EASYDND_POSTHOG_TOKEN", "")
			t.Setenv("EASYDND_SESSION_SECRET", base64Secret)
			t.Setenv("EASYDND_DB_URL", "postgres://easydnd:test@db.example.com:5432/easydnd?sslmode=verify-full")
			cfg, err := Load(repoFile(t, name))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Analytics.Token != "" {
				t.Fatal("analytics must be disabled without an env token")
			}
			t.Setenv("EASYDND_POSTHOG_TOKEN", "phc_environment")
			cfg, err = Load(repoFile(t, name))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Analytics.Token != "phc_environment" || cfg.Analytics.Host != "https://eu.i.posthog.com" {
				t.Fatalf("unexpected analytics config: %+v", cfg.Analytics)
			}
		})
	}
}
