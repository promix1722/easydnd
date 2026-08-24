package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeConfig puts a config file in a temp dir and returns its path. Tests pass
// that path to Load directly rather than through EASYDND_CONFIG, so they stay
// parallel-safe and never depend on process-wide environment.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// A 48-byte secret in base64, which is what `openssl rand -base64 48` produces.
const base64Secret = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

// productionDB is what a production config must carry alongside its secret.
// Never dialled -- Load only records it -- but validate refuses production
// without it, so every production fixture below has to state it.
const productionDB = "db:\n  url: postgres://easydnd:s3cret@db.example.com:5432/easydnd?sslmode=verify-full\n"

func loadOK(t *testing.T, body string) *Config {
	t.Helper()
	cfg, err := Load(writeConfig(t, body))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

func loadErr(t *testing.T, body, because string) error {
	t.Helper()
	cfg, err := Load(writeConfig(t, body))
	if err == nil {
		t.Fatalf("Load succeeded %s: %+v", because, cfg)
	}
	return err
}

// --- config file resolution -------------------------------------------------

// The file is mandatory in every environment. Guessing a location and falling
// through to built-in defaults is how production ends up running with an
// ephemeral signing key nobody ordered.
func TestLoadRequiresAConfigFile(t *testing.T) {
	t.Setenv(EnvConfigPath, "")

	_, err := Load("")
	if err == nil {
		t.Fatal("Load succeeded with no -config and no " + EnvConfigPath)
	}
	if !strings.Contains(err.Error(), EnvConfigPath) || !strings.Contains(err.Error(), "-config") {
		t.Errorf("error %q does not name both ways to supply a path", err)
	}
}

func TestConfigPathComesFromTheEnvironment(t *testing.T) {
	path := writeConfig(t, "env: development\n")
	t.Setenv(EnvConfigPath, path)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Source != path {
		t.Errorf("Source = %q, want %q", cfg.Source, path)
	}
}

// The flag wins so that `make run/server` can name config.dev.yaml without
// caring what happens to be exported in the shell.
func TestFlagPathBeatsTheEnvironment(t *testing.T) {
	t.Setenv(EnvConfigPath, writeConfig(t, "env: production\nauth:\n  session_secret: \""+base64Secret+"\"\n"))
	flagPath := writeConfig(t, "env: development\n")

	cfg, err := Load(flagPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Env != EnvDevelopment {
		t.Errorf("Env = %q, want the flag's file to win", cfg.Env)
	}
}

func TestMissingFileIsAnError(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "absent.yaml")); err == nil {
		t.Fatal("Load succeeded with a path that does not exist")
	}
}

// A silently ignored key is the worst failure mode a config file has:
// `rp_origin` instead of `rp_origins` would leave production on the default
// origin list with nothing anywhere saying so.
func TestUnknownKeysAreRejected(t *testing.T) {
	err := loadErr(t, "env: development\nauth:\n  rp_origin: http://localhost:5173\n",
		"with a misspelled key")
	if !strings.Contains(err.Error(), "rp_origin") {
		t.Errorf("error %q does not name the offending key", err)
	}
}

func TestMalformedYAMLIsAnError(t *testing.T) {
	loadErr(t, "env: development\n  http:\n\tport: nope\n", "with unparseable YAML")
}

// --- defaults ---------------------------------------------------------------

// A file only has to state what it changes.
func TestOmittedKeysFallBackToDefaults(t *testing.T) {
	cfg := loadOK(t, "env: development\n")

	if got := cfg.HTTP.Addr(); got != "127.0.0.1:8080" {
		t.Errorf("Addr() = %q, want the loopback default", got)
	}
	if cfg.HTTP.ShutdownTimeout.Seconds() != 5 {
		t.Errorf("ShutdownTimeout = %s, want 5s", cfg.HTTP.ShutdownTimeout)
	}
	if cfg.HTTP.MaxHeaderBytes != 1<<20 {
		t.Errorf("MaxHeaderBytes = %d, want 1MiB", cfg.HTTP.MaxHeaderBytes)
	}
	if len(cfg.HTTP.TrustedProxies) != 2 {
		t.Errorf("TrustedProxies = %v, want the two loopback entries", cfg.HTTP.TrustedProxies)
	}
	if cfg.Log.Format != FormatJSON {
		t.Errorf("Log.Format = %q, want %q", cfg.Log.Format, FormatJSON)
	}
	if cfg.Data.SRDDir != "data/srd_5.1" {
		t.Errorf("SRDDir = %q, want the repo-relative default", cfg.Data.SRDDir)
	}
}

// env itself defaults to production: the safe direction, since production is
// the configuration that refuses to start when something is missing.
func TestEnvDefaultsToProduction(t *testing.T) {
	cfg := loadOK(t, "auth:\n  session_secret: \""+base64Secret+"\"\n"+productionDB)
	if cfg.Env != EnvProduction {
		t.Errorf("Env = %q, want %q for an empty file", cfg.Env, EnvProduction)
	}
}

func TestValuesFromTheFileAreApplied(t *testing.T) {
	cfg := loadOK(t, `
env: development
http:
  host: 0.0.0.0
  port: "9090"
  read_timeout: 30s
  max_header_bytes: 4096
  trusted_proxies:
    - 10.0.0.1
log:
  level: debug
  format: text
data:
  srd_dir: /srv/srd
auth:
  rp_id: example.test
  rp_name: Example
  rp_origins:
    - https://example.test
    - https://www.example.test
  session_ttl: 1h
  ceremony_ttl: 30s
`)

	if got := cfg.HTTP.Addr(); got != "0.0.0.0:9090" {
		t.Errorf("Addr() = %q", got)
	}
	if cfg.HTTP.ReadTimeout.Seconds() != 30 {
		t.Errorf("ReadTimeout = %s", cfg.HTTP.ReadTimeout)
	}
	if cfg.HTTP.MaxHeaderBytes != 4096 {
		t.Errorf("MaxHeaderBytes = %d", cfg.HTTP.MaxHeaderBytes)
	}
	if len(cfg.HTTP.TrustedProxies) != 1 || cfg.HTTP.TrustedProxies[0] != "10.0.0.1" {
		t.Errorf("TrustedProxies = %v, want only the configured entry", cfg.HTTP.TrustedProxies)
	}
	if cfg.Log.Level.String() != "DEBUG" || cfg.Log.Format != FormatText {
		t.Errorf("Log = %v/%q", cfg.Log.Level, cfg.Log.Format)
	}
	if cfg.Data.SRDDir != "/srv/srd" {
		t.Errorf("SRDDir = %q", cfg.Data.SRDDir)
	}
	if cfg.Auth.RPID != "example.test" || cfg.Auth.RPDisplayName != "Example" {
		t.Errorf("RP = %q/%q", cfg.Auth.RPID, cfg.Auth.RPDisplayName)
	}
	if len(cfg.Auth.RPOrigins) != 2 {
		t.Errorf("RPOrigins = %v, want both entries", cfg.Auth.RPOrigins)
	}
	if cfg.Auth.SessionTTL.Hours() != 1 || cfg.Auth.CeremonyTTL.Seconds() != 30 {
		t.Errorf("TTLs = %s/%s", cfg.Auth.SessionTTL, cfg.Auth.CeremonyTTL)
	}
}

// --- malformed values -------------------------------------------------------

// The environment readers this replaced fell back to the default on a parse
// error. A config file is hand-edited, so "10seconds" should be corrected
// rather than silently ignored.
func TestMalformedDurationIsFatal(t *testing.T) {
	err := loadErr(t, "env: development\nhttp:\n  read_timeout: 10seconds\n",
		"with an unparseable duration")
	if !strings.Contains(err.Error(), "http.read_timeout") {
		t.Errorf("error %q does not name the offending key", err)
	}
}

func TestMalformedAuthDurationIsFatal(t *testing.T) {
	err := loadErr(t, "env: development\nauth:\n  session_ttl: forever\n",
		"with an unparseable auth TTL")
	if !strings.Contains(err.Error(), "auth.session_ttl") {
		t.Errorf("error %q does not name the offending key", err)
	}
}

func TestMalformedLogLevelIsFatal(t *testing.T) {
	err := loadErr(t, "env: development\nlog:\n  level: chatty\n", "with an unknown log level")
	if !strings.Contains(err.Error(), "log.level") {
		t.Errorf("error %q does not name the offending key", err)
	}
}

func TestMalformedIntIsFatal(t *testing.T) {
	loadErr(t, "env: development\nhttp:\n  max_header_bytes: plenty\n",
		"with a non-numeric max_header_bytes")
}

// --- validation -------------------------------------------------------------

func TestUnknownEnvIsRejected(t *testing.T) {
	loadErr(t, "env: staging\n", "with an env that is neither development nor production")
}

func TestUnknownLogFormatIsRejected(t *testing.T) {
	loadErr(t, "env: development\nlog:\n  format: xml\n", "with an unsupported log format")
}

func TestShutdownTimeoutMustBePositive(t *testing.T) {
	loadErr(t, "env: development\nhttp:\n  shutdown_timeout: -1s\n",
		"with a negative shutdown timeout")
}

// --- the session secret -----------------------------------------------------

// A production process with no signing key must refuse to start. Falling back
// to a generated one would mean every restart silently signed everybody out,
// and the failure would only ever be visible as user confusion.
func TestProductionRequiresASessionSecret(t *testing.T) {
	err := loadErr(t, "env: production\n", "in production with no session secret")
	if !strings.Contains(err.Error(), "auth.session_secret") {
		t.Errorf("error %q does not name the missing key", err)
	}
}

func TestProductionRejectsAShortSecret(t *testing.T) {
	loadErr(t, "env: production\nauth:\n  session_secret: too-short\n",
		"with a secret shorter than the hash it feeds")
}

// `openssl rand -base64 48` is what the deploy notes tell an operator to run,
// so its output must be accepted and decoded rather than taken literally.
func TestSecretIsDecodedFromBase64(t *testing.T) {
	cfg := loadOK(t, "env: production\nauth:\n  session_secret: \""+base64Secret+"\"\n"+productionDB)

	if len(cfg.Auth.SessionSecret) != 48 {
		t.Errorf("SessionSecret = %d bytes, want the 48 decoded from base64", len(cfg.Auth.SessionSecret))
	}
	if cfg.Auth.EphemeralSecret {
		t.Error("EphemeralSecret is true despite a configured secret")
	}
}

// A long random passphrase that is not valid base64 should work rather than
// fail with a decoding error nobody expects.
func TestNonBase64SecretIsTakenLiterally(t *testing.T) {
	const passphrase = "correct horse battery staple correct horse battery staple!"
	cfg := loadOK(t, "env: production\nauth:\n  session_secret: \""+passphrase+"\"\n"+productionDB)

	if string(cfg.Auth.SessionSecret) != passphrase {
		t.Error("a non-base64 secret was not used verbatim")
	}
}

func TestDevelopmentGeneratesAnEphemeralSecret(t *testing.T) {
	cfg := loadOK(t, "env: development\n")

	if !cfg.Auth.EphemeralSecret {
		t.Error("EphemeralSecret is false; internal/app would not warn")
	}
	if len(cfg.Auth.SessionSecret) < MinSessionSecretBytes {
		t.Errorf("generated secret is %d bytes, want at least %d",
			len(cfg.Auth.SessionSecret), MinSessionSecretBytes)
	}
}

// --- the relying party ------------------------------------------------------

// The RP id is burned into every passkey at creation and can never be changed
// without orphaning all of them, so the two environments must not share one.
func TestRelyingPartyDefaultsDifferByEnvironment(t *testing.T) {
	prod := loadOK(t, "env: production\nauth:\n  session_secret: \""+base64Secret+"\"\n"+productionDB)
	if prod.Auth.RPID != "easydnd.org" {
		t.Errorf("production RPID = %q, want easydnd.org", prod.Auth.RPID)
	}
	if !prod.Auth.SecureCookies {
		t.Error("production does not set Secure cookies")
	}

	dev := loadOK(t, "env: development\n")
	if dev.Auth.RPID != "localhost" {
		t.Errorf("development RPID = %q, want localhost", dev.Auth.RPID)
	}
	// The Vite dev server is plain HTTP; a Secure cookie there is never sent.
	if dev.Auth.SecureCookies {
		t.Error("development sets Secure cookies, which the dev server can never deliver")
	}
	if dev.Auth.RPOrigins[0] != "http://localhost:5173" {
		t.Errorf("development origin = %q, want the Vite dev server", dev.Auth.RPOrigins[0])
	}
}

// An origin carries scheme and port, unlike the RP id. A bare hostname here is
// a configuration error that would fail every ceremony at verification time.
func TestOriginsMustCarryAScheme(t *testing.T) {
	loadErr(t, "env: development\nauth:\n  rp_origins:\n    - easydnd.org\n",
		"with an origin that has no scheme")
}

func TestAuthTTLsMustBePositive(t *testing.T) {
	loadErr(t, "env: development\nauth:\n  session_ttl: -1h\n", "with a negative session TTL")
	loadErr(t, "env: development\nauth:\n  ceremony_ttl: 0s\n", "with a zero ceremony TTL")
}

// --- permissions ------------------------------------------------------------

// The config file holds the session signing key, so a world-readable one means
// every account on the host can forge a session cookie. internal/app warns.
func TestWorldReadableConfigIsFlagged(t *testing.T) {
	path := writeConfig(t, "env: development\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.WorldReadable {
		t.Error("WorldReadable is true for a 0600 file")
	}

	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	cfg, err = Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.WorldReadable {
		t.Error("WorldReadable is false for a 0644 file; internal/app would not warn")
	}
}

// --- database ---------------------------------------------------------------

// A production process with no database must refuse to start, for the same
// reason it refuses without a signing key: accounts would silently live in
// memory, every restart would destroy every registered passkey, and -- because
// sign-in is passkeys-only with no recovery -- nobody could get back in.
func TestProductionRequiresADatabase(t *testing.T) {
	err := loadErr(t,
		"env: production\nauth:\n  session_secret: \""+base64Secret+"\"\n",
		"in production with no db.url")
	if !strings.Contains(err.Error(), "db.url") {
		t.Errorf("error %q does not name the missing key", err)
	}
}

// Development must keep working with no infrastructure at all: `make
// run/server`, `go test ./...` and `make verify` all run on a machine with no
// Postgres, and the in-memory fallback is what allows that.
func TestDevelopmentRunsWithoutADatabase(t *testing.T) {
	cfg := loadOK(t, "env: development\n")
	if cfg.DB.Enabled() {
		t.Error("DB.Enabled() is true with no db.url")
	}
}

func TestDatabaseDefaults(t *testing.T) {
	cfg := loadOK(t, "env: development\n"+productionDB)

	if !cfg.DB.Enabled() {
		t.Fatal("DB.Enabled() is false with a db.url set")
	}
	if cfg.DB.MaxConns != 10 {
		t.Errorf("MaxConns = %d, want 10", cfg.DB.MaxConns)
	}
	if cfg.DB.ConnectTimeout != 5*time.Second {
		t.Errorf("ConnectTimeout = %s, want 5s", cfg.DB.ConnectTimeout)
	}
	// Migrating by default is what makes a release self-contained; an operator
	// turns it off deliberately, never by omission.
	if !cfg.DB.MigrateOnStart {
		t.Error("MigrateOnStart defaulted to false")
	}
}

func TestDatabaseValuesFromTheFileAreApplied(t *testing.T) {
	cfg := loadOK(t, `
env: development
db:
  url: postgres://u:p@localhost:5432/easydnd?sslmode=disable
  max_conns: 4
  connect_timeout: 12s
  migrate_on_start: false
`)
	if cfg.DB.MaxConns != 4 {
		t.Errorf("MaxConns = %d, want 4", cfg.DB.MaxConns)
	}
	if cfg.DB.ConnectTimeout != 12*time.Second {
		t.Errorf("ConnectTimeout = %s, want 12s", cfg.DB.ConnectTimeout)
	}
	// The whole reason fileDB.MigrateOnStart is a *bool. With a plain bool,
	// "false" is indistinguishable from "absent" and the true default would
	// win -- so this setting could never be turned off from the file at all.
	if cfg.DB.MigrateOnStart {
		t.Error("migrate_on_start: false was ignored; the default won")
	}
}

func TestDatabaseSettingsAreValidated(t *testing.T) {
	loadErr(t, "env: development\ndb:\n  url: postgres://u@h/d\n  max_conns: -1\n",
		"with a negative db.max_conns")
	loadErr(t, "env: development\ndb:\n  url: postgres://u@h/d\n  connect_timeout: -1s\n",
		"with a negative db.connect_timeout")
}

// A malformed duration is fatal rather than silently defaulted, as it is
// everywhere else in this file: a config file is hand-edited, and `12sec`
// should be corrected rather than ignored.
func TestMalformedDBDurationIsFatal(t *testing.T) {
	err := loadErr(t, "env: development\ndb:\n  url: postgres://u@h/d\n  connect_timeout: 12sec\n",
		"with a malformed db.connect_timeout")
	if !strings.Contains(err.Error(), "db.connect_timeout") {
		t.Errorf("error %q does not name the offending key", err)
	}
}

// The example config is published in this repository, so its password must
// never be able to reach production unedited -- exactly as with the secret.
func TestPlaceholderDBPasswordIsRejected(t *testing.T) {
	err := loadErr(t,
		"env: production\nauth:\n  session_secret: \""+base64Secret+"\"\n"+
			"db:\n  url: postgres://easydnd:"+placeholderDBPassword+"@h:5432/easydnd\n",
		"with the placeholder database password")
	if !strings.Contains(err.Error(), "db.url") {
		t.Errorf("error %q does not name the offending key", err)
	}
}

// --- the guest session lifetime ---------------------------------------------

// A guest token cannot be revoked and names nothing recoverable, so its
// lifetime is deliberately its own key rather than sharing the account one.
func TestGuestSessionTTLDefaultsToADay(t *testing.T) {
	cfg := loadOK(t, "env: development\n")

	if cfg.Auth.GuestSessionTTL != 24*time.Hour {
		t.Errorf("GuestSessionTTL = %s, want 24h", cfg.Auth.GuestSessionTTL)
	}
	if cfg.Auth.GuestSessionTTL >= cfg.Auth.SessionTTL {
		t.Errorf("guest TTL %s is not shorter than the account TTL %s",
			cfg.Auth.GuestSessionTTL, cfg.Auth.SessionTTL)
	}
}

func TestGuestSessionTTLIsConfigurable(t *testing.T) {
	cfg := loadOK(t, "env: development\nauth:\n  guest_session_ttl: 90m\n")

	if cfg.Auth.GuestSessionTTL != 90*time.Minute {
		t.Errorf("GuestSessionTTL = %s, want 90m", cfg.Auth.GuestSessionTTL)
	}
	// Setting one TTL must not disturb the other.
	if cfg.Auth.SessionTTL != 168*time.Hour {
		t.Errorf("SessionTTL = %s, want the untouched default", cfg.Auth.SessionTTL)
	}
}

func TestMalformedGuestSessionTTLIsFatal(t *testing.T) {
	err := loadErr(t, "env: development\nauth:\n  guest_session_ttl: forever\n",
		"with an unparseable guest TTL")
	if !strings.Contains(err.Error(), "auth.guest_session_ttl") {
		t.Errorf("error %q does not name the offending key", err)
	}
}

func TestNegativeGuestSessionTTLIsFatal(t *testing.T) {
	loadErr(t, "env: development\nauth:\n  guest_session_ttl: -1h\n",
		"with a negative guest session TTL")
}

// --- Sign in with Google ----------------------------------------------------

// Half a Google configuration would pass startup and then fail at the first
// click, which is the worst place to find out. Both or neither.
func TestGoogleCredentialsMustBeSetTogether(t *testing.T) {
	for _, half := range []string{
		"auth:\n  google:\n    client_id: an-id\n",
		"auth:\n  google:\n    client_secret: a-secret\n",
	} {
		err := loadErr(t, "env: development\n"+half, "with half a Google configuration")
		if !strings.Contains(err.Error(), "must be set together") {
			t.Errorf("error was %q, want it to name the pairing rule", err)
		}
	}
}

// Not configuring Google is a supported deployment, not a broken one: this
// application signs people in with passkeys too.
func TestGoogleIsOptional(t *testing.T) {
	cfg := loadOK(t, "env: development\n")

	if cfg.Auth.Google.Configured() {
		t.Error("Google reported as configured with no credentials")
	}
	if cfg.Auth.Google.ClientID != "" || cfg.Auth.Google.ClientSecret != "" {
		t.Errorf("credentials appeared from nowhere: %+v", cfg.Auth.Google)
	}
}

func TestGoogleRedirectDefaultsPerEnvironment(t *testing.T) {
	const google = "auth:\n  google:\n    client_id: an-id\n    client_secret: a-secret\n"

	// Development points at the Vite dev server, not at this process: Vite
	// proxies /v1 here, and the session cookie has to land on the origin the
	// browser is actually on.
	dev := loadOK(t, "env: development\n"+google)
	if want := "http://localhost:5173" + GoogleRedirectPath; dev.Auth.Google.RedirectURL != want {
		t.Errorf("development redirect = %q, want %q", dev.Auth.Google.RedirectURL, want)
	}

	prod := loadOK(t, "env: production\nauth:\n  session_secret: "+base64Secret+
		"\n  google:\n    client_id: an-id\n    client_secret: a-secret\n"+productionDB)
	if want := "https://easydnd.org" + GoogleRedirectPath; prod.Auth.Google.RedirectURL != want {
		t.Errorf("production redirect = %q, want %q", prod.Auth.Google.RedirectURL, want)
	}
	if !prod.Auth.Google.Configured() {
		t.Error("production Google config did not report as configured")
	}
}

// The redirect URI is registered with Google and must match byte for byte; a
// relative one could never match anything.
func TestGoogleRedirectMustBeAbsolute(t *testing.T) {
	loadErr(t, "env: development\nauth:\n  google:\n    client_id: an-id\n"+
		"    client_secret: a-secret\n    redirect_url: "+GoogleRedirectPath+"\n",
		"with a relative redirect url")
}

// "Configured" for this provider means nothing more than "non-empty", so a
// placeholder copied from the example would put a Google button on the sign-in
// page that fails at the token exchange on every click. Same reasoning as the
// session secret and the database password.
func TestGooglePlaceholderSecretIsRejected(t *testing.T) {
	err := loadErr(t, "env: development\nauth:\n  google:\n    client_id: an-id\n"+
		"    client_secret: "+placeholderGoogleSecret+"\n",
		"with the example placeholder client secret")
	if !strings.Contains(err.Error(), "placeholder") {
		t.Errorf("error was %q, want it to name the placeholder", err)
	}
}

// The loader rejects unknown keys, so a typo in the block is a startup error
// rather than a silently ignored setting.
func TestGoogleRejectsAnUnknownKey(t *testing.T) {
	loadErr(t, "env: development\nauth:\n  google:\n    clientid: an-id\n",
		"with a misspelt key inside auth.google")
}
