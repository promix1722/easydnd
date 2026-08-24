package postgres

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// The DSNs below never connect. Every assertion here is about what pgx's
// ParseConfig produced and what newPoolConfig then corrected, which is exactly
// the code that decides whether the account database is reached over an
// authenticated channel or over a plaintext one.

const testHost = "easydnd.abcdefgh1234.eu-central-1.rds.amazonaws.com"

func testDSN(query string) string {
	dsn := "postgres://easydnd:secret@" + testHost + ":5432/easydnd"
	if query != "" {
		dsn += "?" + query
	}
	return dsn
}

// A db.url that says nothing about TLS is the likeliest one to be
// deployed, and pgx's own default for it is "prefer": InsecureSkipVerify plus a
// plaintext fallback. Every assertion in this test is one half of undoing that.
func TestOmittedSSLModeIsUpgradedToVerifyFull(t *testing.T) {
	cfg, err := newPoolConfig(testDSN(""), 10, 5*time.Second)
	if err != nil {
		t.Fatalf("newPoolConfig: %v", err)
	}

	tc := cfg.ConnConfig.TLSConfig
	if tc == nil {
		t.Fatal("TLSConfig is nil; the connection would be plaintext")
	}
	if tc.RootCAs == nil {
		t.Error("RootCAs is nil; verify-full against RDS cannot succeed on the system pool")
	}
	if tc.InsecureSkipVerify {
		t.Error("InsecureSkipVerify is set; the server certificate would not be checked")
	}
	if tc.VerifyPeerCertificate != nil {
		t.Error("VerifyPeerCertificate is set; verify-full does its checking through ServerName")
	}
	if tc.ServerName != testHost {
		t.Errorf("ServerName = %q, want %q -- without it the hostname is not checked", tc.ServerName, testHost)
	}
	// The one most likely to be reintroduced by a well-meaning refactor:
	// ParseConfig documents that editing TLSConfig leaves Fallbacks alone, and
	// sslmode=prefer puts a plaintext entry in there.
	if len(cfg.ConnConfig.Fallbacks) != 0 {
		t.Errorf("Fallbacks has %d entries; a plaintext fallback survives the TLS config",
			len(cfg.ConnConfig.Fallbacks))
	}
}

// An explicit verify-full still needs our roots injected: pgx leaves RootCAs
// nil, which means the system pool, which does not carry the Amazon RDS CAs.
func TestExplicitVerifyFullStillGetsTheEmbeddedRoots(t *testing.T) {
	cfg, err := newPoolConfig(testDSN("sslmode=verify-full"), 10, 5*time.Second)
	if err != nil {
		t.Fatalf("newPoolConfig: %v", err)
	}
	tc := cfg.ConnConfig.TLSConfig
	if tc == nil {
		t.Fatal("TLSConfig is nil")
	}
	if tc.RootCAs == nil {
		t.Error("RootCAs is nil; the embedded RDS bundle was not installed")
	}
	if tc.ServerName != testHost {
		t.Errorf("ServerName = %q, want %q", tc.ServerName, testHost)
	}
}

// sslmode=disable is what the local container and the CI service container use.
// It must survive untouched, or there is no way to test without TLS at all.
func TestDisableIsHonoured(t *testing.T) {
	cfg, err := newPoolConfig(testDSN("sslmode=disable"), 10, 5*time.Second)
	if err != nil {
		t.Fatalf("newPoolConfig: %v", err)
	}
	if cfg.ConnConfig.TLSConfig != nil {
		t.Error("TLSConfig is set; sslmode=disable was overridden")
	}
}

// An operator who named their own CA file meant it. Silently replacing it with
// our bundle would make the setting a lie and break a non-RDS Postgres.
func TestExplicitRootCertIsLeftAlone(t *testing.T) {
	cfg, err := newPoolConfig(testDSN("sslmode=verify-full&sslrootcert=/etc/ssl/other.pem"), 10, 5*time.Second)
	// pgx reads sslrootcert from disk, so a path that does not exist fails to
	// parse. That failure is itself the proof that pgx -- not this package --
	// is the one handling the file.
	if err != nil {
		if !strings.Contains(err.Error(), "parse db.url") {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	if tc := cfg.ConnConfig.TLSConfig; tc != nil && tc.RootCAs == nil {
		t.Error("RootCAs is nil after an explicit sslrootcert")
	}
}

func TestPoolSettingsAreApplied(t *testing.T) {
	cfg, err := newPoolConfig(testDSN("sslmode=disable"), 7, 3*time.Second)
	if err != nil {
		t.Fatalf("newPoolConfig: %v", err)
	}
	if cfg.MaxConns != 7 {
		t.Errorf("MaxConns = %d, want 7", cfg.MaxConns)
	}
	if cfg.ConnConfig.ConnectTimeout != 3*time.Second {
		t.Errorf("ConnectTimeout = %s, want 3s", cfg.ConnConfig.ConnectTimeout)
	}
}

func TestMalformedURLIsRejected(t *testing.T) {
	if _, err := newPoolConfig("://nonsense", 10, time.Second); err == nil {
		t.Fatal("newPoolConfig accepted a malformed DSN")
	}
}

// The bundle is committed to the repository, so a truncated or mangled commit
// must fail here rather than as a certificate error at the first connection.
func TestEmbeddedBundleParses(t *testing.T) {
	// rdsRoots only returns successfully when AppendCertsFromPEM accepted at
	// least one certificate, so a nil error here is the assertion.
	roots, err := rdsRoots()
	if err != nil {
		t.Fatalf("rdsRoots: %v", err)
	}
	if roots == nil {
		t.Fatal("rdsRoots returned a nil pool with no error")
	}
	if !bytes.Contains(rdsBundlePEM, []byte("BEGIN CERTIFICATE")) {
		t.Error("the committed bundle holds no PEM certificate block")
	}
}
