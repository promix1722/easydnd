// Package config loads all runtime configuration from a single YAML file whose
// path comes from EASYDND_CONFIG (or -config). It knows nothing about HTTP
// frameworks, databases, or the domain.
//
// One file, one source of truth. Settings used to be spread across the
// supervisor program definition's environment= line, which mixed the session
// signing key in with the port number and forced the whole supervisor config to
// be chmod 600. Now that line names a path and nothing else.
package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"
)

// MinSessionSecretBytes is the shortest signing key we will start with. HS256
// keys shorter than the hash they feed buy nothing and invite a brute force.
const MinSessionSecretBytes = 32

// Environment names.
const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
)

// Log output formats.
const (
	FormatJSON = "json"
	FormatText = "text"
)

// GoogleRedirectPath is the callback path Google sends the browser back to.
// It is a constant rather than a setting because it must agree with the route
// declared in internal/api/http/router.go; only the origin in front of it
// differs between development and production.
const GoogleRedirectPath = "/v1/auth/sso/google/callback"

// Config is the fully resolved runtime configuration.
type Config struct {
	Env  string
	HTTP HTTPConfig
	Auth AuthConfig
	Log  LogConfig
	Data DataConfig
	DB   DBConfig

	// Source is the config file this was loaded from, logged at startup so the
	// log stream answers "which config is this process running?".
	Source string
	// WorldReadable reports that Source is readable by every account on the
	// host. It holds the session signing key, so the app warns about it.
	WorldReadable bool
}

// DBConfig points at the Postgres instance holding accounts and passkeys.
type DBConfig struct {
	// URL is a libpq connection URL:
	//   postgres://user:pass@host:5432/easydnd?sslmode=verify-full
	//
	// It carries the database password, which is the second secret in this
	// file after the session signing key -- and the reason the live copy is
	// 640 root:easydnd rather than world-readable.
	//
	// Empty means "no database", which is a development-only state: validate
	// refuses to start production without it. An unset sslmode is defaulted to
	// verify-full by the adapter rather than to libpq's "prefer", which would
	// permit an unauthenticated and possibly unencrypted connection.
	URL string

	MaxConns       int32
	ConnectTimeout time.Duration

	// MigrateOnStart applies pending migrations before the listener binds.
	// Turning it off is for the operator who would rather run `easydnd
	// -migrate up` by hand before letting a release serve.
	MigrateOnStart bool
}

// Enabled reports whether a durable account store is configured.
func (d DBConfig) Enabled() bool { return d.URL != "" }

// DataConfig locates the static game data.
type DataConfig struct {
	// SRDDir holds the SRD 5.1 compendium, read at startup. It is a
	// directory rather than an embedded blob so the data can be corrected
	// without rebuilding the binary -- which also means deploy.sh must ship
	// it alongside the binary.
	SRDDir string
}

// AuthConfig configures passkey sign-in.
type AuthConfig struct {
	// RPID is the WebAuthn Relying Party ID -- effectively the registrable
	// domain a passkey is bound to. Changing it orphans every passkey ever
	// registered, so it is a permanent decision, not a tunable.
	RPID string
	// RPDisplayName is what the operating system's passkey prompt calls us.
	RPDisplayName string
	// RPOrigins lists the exact origins allowed to complete a ceremony.
	// Unlike RPID these carry scheme and port, and dev differs from prod.
	RPOrigins []string

	// SessionSecret signs the session and ceremony tokens. Rotating it is the
	// only way to invalidate every outstanding session at once, because
	// nothing server-side records that a session exists.
	SessionSecret []byte
	SessionTTL    time.Duration
	// GuestSessionTTL bounds an anonymous session. Shorter than SessionTTL on
	// purpose: nothing server-side records that a guest token was issued and
	// nothing it names can be recovered, so the only thing limiting the damage
	// of a leaked one is how soon it expires.
	GuestSessionTTL time.Duration
	CeremonyTTL     time.Duration

	// SecureCookies marks cookies Secure and enables the __Host-/__Secure-
	// name prefixes. Derived from Env rather than configured: the Vite dev
	// server is plain HTTP, and a Secure cookie there is simply never sent.
	SecureCookies bool

	// EphemeralSecret records that no secret was configured and one was
	// generated for this process. Development only; Load refuses to start
	// production this way.
	EphemeralSecret bool

	// Google configures Sign in with Google. Zero value means not configured,
	// which is not an error: the provider is simply not offered, and
	// `make run/server` with no environment at all keeps working.
	Google GoogleConfig
}

// GoogleConfig configures the Google identity provider.
type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	// RedirectURL must match a URI registered in the Google Cloud console
	// byte for byte, or the exchange fails with redirect_uri_mismatch. In
	// development it points at the Vite dev server rather than at this
	// process, because Vite proxies /v1 here and the session cookie has to
	// land on the origin the browser is actually on.
	RedirectURL string
}

// Configured reports whether Google sign-in should be offered.
func (g GoogleConfig) Configured() bool {
	return g.ClientID != "" && g.ClientSecret != ""
}

// HTTPConfig configures the API server.
type HTTPConfig struct {
	Host              string
	Port              string
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	MaxHeaderBytes    int
	TrustedProxies    []string
}

// Addr renders the listen address.
func (h HTTPConfig) Addr() string { return net.JoinHostPort(h.Host, h.Port) }

// LogConfig configures the structured logger.
type LogConfig struct {
	Level  slog.Level
	Format string
}

// Load reads the config file at path -- or, when path is empty, at
// EASYDND_CONFIG -- applies defaults for everything it does not set, and
// validates the result.
//
// Defaults are chosen so that a config file only has to state what it changes,
// and so that production behaviour matches the supervisor plus reverse-proxy
// deployment on easydnd.org. The file itself is mandatory in every environment:
// see resolvePath for why there is no default location.
func Load(path string) (*Config, error) {
	resolved, err := resolvePath(path)
	if err != nil {
		return nil, err
	}
	src, err := readFile(resolved)
	if err != nil {
		return nil, err
	}

	var p parser
	f := src.cfg

	// Read first: several auth defaults differ between development and
	// production, and picking them needs the environment already resolved.
	env := p.str(f.Env, EnvProduction)
	production := env == EnvProduction

	auth, err := loadAuth(&p, f.Auth, production)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Env:           env,
		Auth:          auth,
		Source:        src.path,
		WorldReadable: src.worldReadable,
		HTTP: HTTPConfig{
			// Loopback on purpose: the reverse proxy terminates TLS and
			// forwards here. Binding 0.0.0.0 would expose the API directly.
			Host: p.str(f.HTTP.Host, "127.0.0.1"),
			// Must match the port deploy/deploy.sh health-checks and the one
			// the nginx site proxies to.
			Port:              p.str(f.HTTP.Port, "8080"),
			ReadTimeout:       p.duration("http.read_timeout", f.HTTP.ReadTimeout, 10*time.Second),
			ReadHeaderTimeout: p.duration("http.read_header_timeout", f.HTTP.ReadHeaderTimeout, 5*time.Second),
			WriteTimeout:      p.duration("http.write_timeout", f.HTTP.WriteTimeout, 15*time.Second),
			IdleTimeout:       p.duration("http.idle_timeout", f.HTTP.IdleTimeout, 60*time.Second),
			// Must stay below supervisor's stopwaitsecs (default 10s), or the
			// drain is cut short by SIGKILL and graceful shutdown never
			// actually completes. Raise stopwaitsecs first if you raise this.
			ShutdownTimeout: p.duration("http.shutdown_timeout", f.HTTP.ShutdownTimeout, 5*time.Second),
			MaxHeaderBytes:  p.intVal(f.HTTP.MaxHeaderBytes, 1<<20),
			// gin trusts 0.0.0.0/0 by default, which lets any client forge
			// X-Forwarded-For and poison ClientIP. Narrow it to our own proxy.
			TrustedProxies: p.slice(f.HTTP.TrustedProxies, []string{"127.0.0.1", "::1"}),
		},
		Log: LogConfig{
			Level:  p.level("log.level", f.Log.Level, slog.LevelInfo),
			Format: p.str(f.Log.Format, FormatJSON),
		},
		Data: DataConfig{
			// Relative by default so `make run/server` works from the repo
			// root; the deploy sets it to the release directory.
			SRDDir: p.str(f.Data.SRDDir, "data/srd_5.1"),
		},
		DB: DBConfig{
			URL:      p.str(f.DB.URL, ""),
			MaxConns: int32(p.intVal(f.DB.MaxConns, 10)),
			// Deliberately short. deploy.sh's health gate allows 15 seconds
			// for the whole of connect plus migrate plus listen, so a database
			// that is not answering must give up well inside that budget.
			ConnectTimeout: p.duration("db.connect_timeout", f.DB.ConnectTimeout, 5*time.Second),
			// Default true, so a release is self-contained. bool cannot use
			// the zero-value-means-absent trick the other readers rely on --
			// `false` and "not set" are the same value -- so the file field is
			// a *bool and nil is what means absent.
			MigrateOnStart: p.boolVal(f.DB.MigrateOnStart, true),
		},
	}

	// Conversion errors before semantic ones: "invalid duration" is more useful
	// than the "must be positive" it would otherwise cascade into.
	if p.err != nil {
		return nil, p.err
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	switch c.Env {
	case EnvDevelopment, EnvProduction:
	default:
		return fmt.Errorf("env must be %q or %q, got %q", EnvDevelopment, EnvProduction, c.Env)
	}

	switch strings.ToLower(c.Log.Format) {
	case FormatJSON, FormatText:
	default:
		return fmt.Errorf("log.format must be %q or %q, got %q", FormatJSON, FormatText, c.Log.Format)
	}

	if c.HTTP.Host == "" {
		return fmt.Errorf("http.host must not be empty")
	}
	if c.HTTP.Port == "" {
		return fmt.Errorf("http.port must not be empty")
	}
	if c.HTTP.ShutdownTimeout <= 0 {
		return fmt.Errorf("http.shutdown_timeout must be positive, got %s", c.HTTP.ShutdownTimeout)
	}
	if c.Data.SRDDir == "" {
		return fmt.Errorf("data.srd_dir must not be empty")
	}

	// The same shape as the auth.session_secret rule, and for the same reason:
	// a production process that quietly forgot where accounts live is the exact
	// failure the durable store exists to end.
	if c.Env == EnvProduction && !c.DB.Enabled() {
		return fmt.Errorf(
			"db.url is required in production; without it accounts live in memory " +
				"and every restart destroys every registered passkey")
	}
	// A known-value password is worse than no password: deploy/config.example.yaml
	// is published in this repository, so an operator who installed it and
	// forgot to edit would be running production on credentials anyone can read.
	// Rejected by name, exactly as the session secret is.
	if c.DB.Enabled() && strings.Contains(c.DB.URL, placeholderDBPassword) {
		return fmt.Errorf(
			"db.url still contains the placeholder password from deploy/config.example.yaml")
	}
	if c.DB.Enabled() {
		if c.DB.MaxConns < 1 {
			return fmt.Errorf("db.max_conns must be at least 1, got %d", c.DB.MaxConns)
		}
		if c.DB.ConnectTimeout <= 0 {
			return fmt.Errorf("db.connect_timeout must be positive, got %s", c.DB.ConnectTimeout)
		}
	}
	return nil
}

// loadAuth resolves the passkey configuration.
//
// The RP id and origins default to easydnd.org in production and to the Vite
// dev server in development, so that config.dev.yaml can stay almost empty
// while production still fails loudly on a missing secret.
func loadAuth(p *parser, f fileAuth, production bool) (AuthConfig, error) {
	defaultRPID, defaultOrigins := "localhost", []string{"http://localhost:5173"}
	if production {
		defaultRPID, defaultOrigins = "easydnd.org", []string{"https://easydnd.org"}
	}

	cfg := AuthConfig{
		RPID:          p.str(f.RPID, defaultRPID),
		RPDisplayName: p.str(f.RPName, "easydnd"),
		RPOrigins:     p.slice(f.RPOrigins, defaultOrigins),
		SessionTTL:    p.duration("auth.session_ttl", f.SessionTTL, 168*time.Hour),
		// A day: long enough to finish building a character over an evening,
		// short enough that an abandoned guest token stops working soon.
		GuestSessionTTL: p.duration("auth.guest_session_ttl", f.GuestSessionTTL, 24*time.Hour),
		CeremonyTTL:     p.duration("auth.ceremony_ttl", f.CeremonyTTL, 5*time.Minute),
		SecureCookies:   production,
	}

	secret, err := sessionSecret(f.SessionSecret, production)
	if err != nil {
		return AuthConfig{}, err
	}
	cfg.SessionSecret = secret
	cfg.EphemeralSecret = f.SessionSecret == ""

	google, err := loadGoogle(p, f.Google, production)
	if err != nil {
		return AuthConfig{}, err
	}
	cfg.Google = google

	if cfg.RPID == "" {
		return AuthConfig{}, fmt.Errorf("auth.rp_id must not be empty")
	}
	for _, origin := range cfg.RPOrigins {
		if !strings.Contains(origin, "://") {
			return AuthConfig{}, fmt.Errorf(
				"auth.rp_origins entries must include a scheme, got %q", origin)
		}
	}
	// A conversion error would have left these at their defaults, which are
	// positive -- so report it now rather than passing a bogus TTL as valid.
	if p.err != nil {
		return AuthConfig{}, p.err
	}
	if cfg.SessionTTL <= 0 {
		return AuthConfig{}, fmt.Errorf("auth.session_ttl must be positive, got %s", cfg.SessionTTL)
	}
	if cfg.GuestSessionTTL <= 0 {
		return AuthConfig{}, fmt.Errorf("auth.guest_session_ttl must be positive, got %s", cfg.GuestSessionTTL)
	}
	if cfg.CeremonyTTL <= 0 {
		return AuthConfig{}, fmt.Errorf("auth.ceremony_ttl must be positive, got %s", cfg.CeremonyTTL)
	}
	return cfg, nil
}

// placeholderSecret is what deploy/config.example.yaml ships. It is 38 bytes of
// non-base64 text, so it clears the length floor on its own -- meaning an
// operator who installed the example and forgot to edit it would boot
// production with a signing key published in this repository. Reject it by
// name; a known-value key is worse than no key at all.
const placeholderSecret = "REPLACE-ME-WITH-openssl-rand-base64-48"

// placeholderDBPassword is what deploy/config.example.yaml ships in db.url.
// Same reasoning as placeholderSecret: a credential published in this
// repository must never be able to reach production unedited.
const placeholderDBPassword = "REPLACE-ME-WITH-THE-RDS-PASSWORD"

// sessionSecret decodes auth.session_secret, or in development invents one.

// placeholderGoogleSecret is what deploy/config.example.yaml would ship if the
// Google block were ever filled in. Same reasoning as the two above: a
// credential published in this repository must not be able to reach production
// unedited. It matters more here than it looks, because "configured" for this
// provider means nothing more than "non-empty" -- a placeholder would put a
// Google button on the sign-in page that fails at the token exchange on every
// click.
const placeholderGoogleSecret = "REPLACE-ME-WITH-THE-GOOGLE-CLIENT-SECRET"

// loadGoogle resolves the Google provider configuration.
//
// Absent means "not offered", not "misconfigured": this application signs
// people in with passkeys too, and a deployment that wants only those should
// not have to supply credentials it will never use. What is an error is
// supplying half of it -- a client id without its secret would pass startup and
// then fail at the first click, which is the worst place to find out.
func loadGoogle(p *parser, f fileGoogle, production bool) (GoogleConfig, error) {
	defaultRedirect := "http://localhost:5173" + GoogleRedirectPath
	if production {
		defaultRedirect = "https://easydnd.org" + GoogleRedirectPath
	}

	cfg := GoogleConfig{
		ClientID:     f.ClientID,
		ClientSecret: f.ClientSecret,
		RedirectURL:  p.str(f.RedirectURL, defaultRedirect),
	}

	if (cfg.ClientID == "") != (cfg.ClientSecret == "") {
		return GoogleConfig{}, fmt.Errorf(
			"auth.google.client_id and auth.google.client_secret must be set together; " +
				"set both to offer Google sign-in, or omit the auth.google block to leave it off")
	}
	if !cfg.Configured() {
		return GoogleConfig{}, nil
	}
	if cfg.ClientSecret == placeholderGoogleSecret {
		return GoogleConfig{}, fmt.Errorf(
			"auth.google.client_secret is still the example placeholder; " +
				"paste the real one from the Google Cloud console, or omit the auth.google block")
	}
	if !strings.Contains(cfg.RedirectURL, "://") {
		return GoogleConfig{}, fmt.Errorf(
			"auth.google.redirect_url must be an absolute URL, got %q", cfg.RedirectURL)
	}
	return cfg, nil
}

// sessionSecret decodes auth.session_secret, or in development invents one.
//
// The value is read as standard base64 first, since that is what the documented
// `openssl rand -base64 48` produces, and taken literally if it is not valid
// base64 -- a long random passphrase should work rather than fail with a
// decoding error nobody expects.
func sessionSecret(raw string, production bool) ([]byte, error) {
	if raw == "" {
		if production {
			return nil, fmt.Errorf(
				"auth.session_secret is required in production; generate one with `openssl rand -base64 48`")
		}
		// Development convenience. Every restart invalidates every session,
		// which is honest: the account store is in-memory and does not
		// survive a restart either.
		secret := make([]byte, MinSessionSecretBytes)
		if _, err := rand.Read(secret); err != nil {
			return nil, fmt.Errorf("generate development session secret: %w", err)
		}
		return secret, nil
	}

	if strings.Contains(raw, placeholderSecret) {
		return nil, fmt.Errorf(
			"auth.session_secret is still the placeholder from deploy/config.example.yaml; generate a real one with `openssl rand -base64 48`")
	}

	secret, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		secret = []byte(raw)
	}
	if len(secret) < MinSessionSecretBytes {
		return nil, fmt.Errorf(
			"auth.session_secret must decode to at least %d bytes, got %d",
			MinSessionSecretBytes, len(secret))
	}
	return secret, nil
}
