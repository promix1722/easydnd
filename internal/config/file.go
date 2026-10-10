package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
)

// EnvConfigPath names the config file. The file is committed and carries no
// secret; what it cannot carry arrives through the variables in applyEnv.
const EnvConfigPath = "EASYDND_CONFIG"

// applyEnv lays the environment over the parsed file. Two kinds of value come
// this way and no others: secrets, which a committed config must not hold, and
// what differs per machine or per worktree -- a development slot's port and
// origins. The process reads only its environment; the env *file* is loaded by
// whatever starts it, make in development and supervisor in production.
//
// A fixed list rather than a naming rule, so that a stray export cannot reach
// a key nobody meant to open, and an unset or empty variable leaves the file's
// value alone.
func applyEnv(f *fileConfig) {
	for name, dst := range map[string]*string{
		"EASYDND_POSTHOG_TOKEN":        &f.Analytics.Token,
		"EASYDND_AGENT_API_KEY":        &f.Agent.APIKey,
		"EASYDND_SESSION_SECRET":       &f.Auth.SessionSecret,
		"EASYDND_DB_URL":               &f.DB.URL,
		"EASYDND_GOOGLE_CLIENT_ID":     &f.Auth.Google.ClientID,
		"EASYDND_GOOGLE_CLIENT_SECRET": &f.Auth.Google.ClientSecret,
		"EASYDND_HTTP_PORT":            &f.HTTP.Port,
		"EASYDND_RP_ID":                &f.Auth.RPID,
	} {
		if v := os.Getenv(name); v != "" {
			*dst = v
		}
	}
	if v := os.Getenv("EASYDND_RP_ORIGINS"); v != "" {
		f.Auth.RPOrigins = strings.Split(v, ",")
	}
	// A path on this machine, not a secret -- but it is true of one host only,
	// and a missing directory is a startup error, so it cannot be committed.
	if v := os.Getenv("EASYDND_PRIVATE_PACK_FILES"); v != "" {
		f.Data.PrivatePackFiles = strings.Split(v, ",")
	}
}

// fileConfig mirrors Config with YAML tags. It exists as a separate type so
// that "absent from the file" is distinguishable from "resolved value": every
// field here is a zero value until the file says otherwise, and the zero value
// is what selects the built-in default.
//
// Durations are strings ("10s") rather than time.Duration so that a malformed
// value produces our own error naming the key, not a yaml type error.
type fileConfig struct {
	Analytics AnalyticsConfig `yaml:"analytics"`
	Agent     fileAgent       `yaml:"agent"`
	Env       string          `yaml:"env"`
	HTTP      fileHTTP        `yaml:"http"`
	Log       fileLog         `yaml:"log"`
	Data      fileData        `yaml:"data"`
	Auth      fileAuth        `yaml:"auth"`
	DB        fileDB          `yaml:"db"`
}

type fileAgent struct {
	APIKey string `yaml:"api_key"`
	Model  string `yaml:"model"`
	// ReasoningEffort is passed to the provider as written. Empty is "low";
	// "default" sends nothing, for a model that takes no such setting.
	ReasoningEffort string `yaml:"reasoning_effort"`
	Workers         int    `yaml:"workers"`
	MaxTurns        int    `yaml:"max_turns"`
	MaxSessions     int    `yaml:"max_sessions"`
	RequestTimeout  string `yaml:"request_timeout"`
}

type fileHTTP struct {
	Host              string   `yaml:"host"`
	Port              string   `yaml:"port"`
	ReadTimeout       string   `yaml:"read_timeout"`
	ReadHeaderTimeout string   `yaml:"read_header_timeout"`
	WriteTimeout      string   `yaml:"write_timeout"`
	IdleTimeout       string   `yaml:"idle_timeout"`
	ShutdownTimeout   string   `yaml:"shutdown_timeout"`
	MaxHeaderBytes    int      `yaml:"max_header_bytes"`
	TrustedProxies    []string `yaml:"trusted_proxies"`
}

type fileLog struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type fileData struct {
	SRDDir    string   `yaml:"srd_dir"`
	PackFiles []string `yaml:"pack_files"`
	// PrivatePackFiles are installed like pack_files but never join the
	// default lock and are listed only for the accounts allowed to read them.
	PrivatePackFiles []string          `yaml:"private_pack_files"`
	AutoloadPacks    []PackFolder      `yaml:"autoload_packs"`
	DefaultPacks     map[string]string `yaml:"default_packs"`
	PackArchive      string            `yaml:"pack_archive"`
}

type fileDB struct {
	URL      string `yaml:"url"`
	MaxConns int    `yaml:"max_conns"`
	// String, like every other duration here, so a malformed value produces our
	// own error naming the key rather than a yaml type error.
	ConnectTimeout string `yaml:"connect_timeout"`
	// A POINTER, unlike every other field in this file. The zero-value-means-
	// absent convention cannot work for a bool: `migrate_on_start: false` and
	// an omitted key are both `false`, so a plain bool would make it impossible
	// to turn migration off -- the default would win every time. nil is absent.
	MigrateOnStart *bool `yaml:"migrate_on_start"`
}

type fileAuth struct {
	RPID          string   `yaml:"rp_id"`
	RPName        string   `yaml:"rp_name"`
	RPOrigins     []string `yaml:"rp_origins"`
	SessionSecret string   `yaml:"session_secret"`
	// Superadmins names accounts by verified Google email or by account id.
	Superadmins []string `yaml:"superadmins"`
	SessionTTL  string   `yaml:"session_ttl"`
	// GuestSessionTTL is the anonymous-session lifetime. It is a separate key
	// from session_ttl because a guest token names nothing recoverable and
	// cannot be revoked, so it wants a shorter life than an account's.
	GuestSessionTTL string `yaml:"guest_session_ttl"`
	CeremonyTTL     string `yaml:"ceremony_ttl"`
	// Google is optional in a way the fields above are not: omitting the whole
	// block means Google sign-in is not offered, which is a supported
	// deployment rather than a broken one.
	Google fileGoogle `yaml:"google"`
}

type fileGoogle struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	// RedirectURL must match a URI registered with Google byte for byte. It is
	// defaulted per environment, so it is only worth setting when this
	// deployment answers on some other hostname.
	RedirectURL string `yaml:"redirect_url"`
}

type fileSource struct {
	cfg fileConfig
	// path is echoed in logs so that "which config is this process running?"
	// is answerable from the log stream alone.
	path string
}

// resolvePath picks the config path: an explicit flag value wins over
// EASYDND_CONFIG. There is no default location -- guessing at /etc and silently
// running on built-in defaults is how a production process ends up with an
// ephemeral signing key nobody ordered.
func resolvePath(flagPath string) (string, error) {
	if flagPath != "" {
		return flagPath, nil
	}
	if v := os.Getenv(EnvConfigPath); v != "" {
		return v, nil
	}
	return "", fmt.Errorf(
		"no config file: pass -config <path> or set %s (see config.prod.yaml)",
		EnvConfigPath)
}

// readFile loads and strictly parses the config file.
//
// Unknown keys are an error on purpose. A silently ignored key is the worst
// possible failure mode for a config file: `auth.rp_origin` instead of
// `auth.rp_origins` would leave production running the default origin list and
// nothing anywhere would say so.
func readFile(path string) (*fileSource, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg fileConfig
	if err := yaml.UnmarshalWithOptions(raw, &cfg, yaml.DisallowUnknownField()); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	applyEnv(&cfg)

	return &fileSource{cfg: cfg, path: path}, nil
}

// parser accumulates the first conversion error so that the mapping code below
// reads as straight-line assignments instead of a ladder of error checks.
type parser struct{ err error }

func (p *parser) fail(err error) {
	if p.err == nil {
		p.err = err
	}
}

// str returns the configured value, or def when the key was absent.
func (p *parser) str(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func (p *parser) intVal(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

// boolVal returns the configured value, or def when the key was absent.
// See fileDB.MigrateOnStart for why the argument is a pointer.
func (p *parser) boolVal(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}

func (p *parser) slice(v, def []string) []string {
	out := make([]string, 0, len(v))
	for _, s := range v {
		if s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return def
	}
	return out
}

// duration parses a Go duration string. Unlike the environment readers this
// replaces, a malformed value is fatal rather than silently defaulted: a config
// file is hand-edited, and `10seconds` should be corrected, not ignored.
func (p *parser) duration(key, v string, def time.Duration) time.Duration {
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		p.fail(fmt.Errorf("%s: invalid duration %q (want e.g. \"10s\", \"5m\", \"168h\")", key, v))
		return def
	}
	return d
}

func (p *parser) level(key, v string, def slog.Level) slog.Level {
	if v == "" {
		return def
	}
	// slog.Level.UnmarshalText accepts debug/info/warn/error case-insensitively,
	// plus offsets such as "warn+2".
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(v)); err != nil {
		p.fail(fmt.Errorf("%s: invalid level %q (want debug, info, warn or error)", key, v))
		return def
	}
	return lvl
}
