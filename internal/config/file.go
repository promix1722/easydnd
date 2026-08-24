package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/goccy/go-yaml"
)

// EnvConfigPath names the one and only environment variable this package reads.
// Every other setting lives in the YAML file it points at: a single source of
// truth beats a config file that any stray export can quietly override.
const EnvConfigPath = "EASYDND_CONFIG"

// fileConfig mirrors Config with YAML tags. It exists as a separate type so
// that "absent from the file" is distinguishable from "resolved value": every
// field here is a zero value until the file says otherwise, and the zero value
// is what selects the built-in default.
//
// Durations are strings ("10s") rather than time.Duration so that a malformed
// value produces our own error naming the key, not a yaml type error.
type fileConfig struct {
	Env  string   `yaml:"env"`
	HTTP fileHTTP `yaml:"http"`
	Log  fileLog  `yaml:"log"`
	Data fileData `yaml:"data"`
	Auth fileAuth `yaml:"auth"`
	DB   fileDB   `yaml:"db"`
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
	SRDDir string `yaml:"srd_dir"`
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
	SessionTTL    string   `yaml:"session_ttl"`
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

// fileSource is a parsed config file plus what we learned about the file itself.
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
	// worldReadable records that the file is readable by every account on the
	// box. It holds the session signing key, so this is worth saying out loud.
	worldReadable bool
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
		"no config file: pass -config <path> or set %s (see deploy/config.example.yaml)",
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

	src := &fileSource{cfg: cfg, path: path}
	// A stat failure here is not fatal: we have already read the file, and
	// losing the permission warning is not worth refusing to start over.
	if info, err := os.Stat(path); err == nil {
		src.worldReadable = info.Mode().Perm()&0o004 != 0
	}
	return src, nil
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
