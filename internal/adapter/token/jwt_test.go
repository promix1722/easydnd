package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	domain "github.com/promix1722/easydnd/internal/domain/auth"
	"github.com/promix1722/easydnd/internal/types"
)

var testSecret = []byte("0123456789abcdef0123456789abcdef")

func TestSignSessionRoundTrip(t *testing.T) {
	s := NewSigner(testSecret, time.Hour)
	now := time.Now().Truncate(time.Second)

	token, err := s.SignSession(domain.Session{
		UserID:    "abc123",
		IssuedAt:  now,
		ExpiresAt: now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("SignSession: %v", err)
	}

	got, err := s.VerifySession(token, now)
	if err != nil {
		t.Fatalf("VerifySession: %v", err)
	}
	if got.UserID != "abc123" {
		t.Errorf("UserID = %q, want %q", got.UserID, "abc123")
	}
	if !got.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Errorf("ExpiresAt = %v, want %v", got.ExpiresAt, now.Add(time.Hour))
	}
}

func TestVerifySessionRejectsExpired(t *testing.T) {
	s := NewSigner(testSecret, time.Hour)
	now := time.Now()

	token, err := s.SignSession(domain.Session{UserID: "u", IssuedAt: now, ExpiresAt: now.Add(time.Minute)})
	if err != nil {
		t.Fatalf("SignSession: %v", err)
	}

	if _, err := s.VerifySession(token, now.Add(2*time.Minute)); !types.IsUnauthenticated(err) {
		t.Fatalf("err = %v, want *types.UnauthenticatedError", err)
	}
}

func TestVerifySessionRejectsForeignKey(t *testing.T) {
	mine := NewSigner(testSecret, time.Hour)
	theirs := NewSigner([]byte("ffffffffffffffffffffffffffffffff"), time.Hour)
	now := time.Now()

	token, err := theirs.SignSession(domain.Session{UserID: "u", IssuedAt: now, ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatalf("SignSession: %v", err)
	}

	if _, err := mine.VerifySession(token, now); !types.IsUnauthenticated(err) {
		t.Fatalf("err = %v, want *types.UnauthenticatedError", err)
	}
}

// A rotated signing key must invalidate outstanding sessions -- that property
// is the only revocation lever this design has, so it is worth a test of its
// own rather than being inferred from the case above.
func TestRotatingTheSecretInvalidatesSessions(t *testing.T) {
	now := time.Now()
	before := NewSigner(testSecret, time.Hour)
	token, err := before.SignSession(domain.Session{UserID: "u", IssuedAt: now, ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatalf("SignSession: %v", err)
	}

	after := NewSigner([]byte("11111111111111111111111111111111"), time.Hour)
	if _, err := after.VerifySession(token, now); err == nil {
		t.Fatal("a session survived a key rotation")
	}
}

// The classic JWT forgery: an attacker rewrites the header to alg "none" and
// strips the signature. jwt.WithValidMethods is what stops it, and this is the
// test that proves the option is actually in place.
func TestVerifySessionRejectsAlgNone(t *testing.T) {
	s := NewSigner(testSecret, time.Hour)
	now := time.Now()

	header := b64(t, map[string]string{"alg": "none", "typ": "JWT"})
	body := b64(t, map[string]any{
		"sub": "victim",
		"knd": kindSession,
		"iat": now.Unix(),
		"exp": now.Add(time.Hour).Unix(),
	})

	if _, err := s.VerifySession(header+"."+body+".", now); !types.IsUnauthenticated(err) {
		t.Fatalf("alg:none token was accepted (err = %v)", err)
	}
}

// The other half of the same family: keep a real HMAC signature but claim an
// asymmetric algorithm in the header, hoping the parser trusts the header.
func TestVerifySessionRejectsAlgConfusion(t *testing.T) {
	s := NewSigner(testSecret, time.Hour)
	now := time.Now()

	header := b64(t, map[string]string{"alg": "RS256", "typ": "JWT"})
	body := b64(t, map[string]any{
		"sub": "victim",
		"knd": kindSession,
		"iat": now.Unix(),
		"exp": now.Add(time.Hour).Unix(),
	})
	mac := hmac.New(sha256.New, testSecret)
	mac.Write([]byte(header + "." + body))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if _, err := s.VerifySession(header+"."+body+"."+signature, now); !types.IsUnauthenticated(err) {
		t.Fatalf("RS256-header token was accepted (err = %v)", err)
	}
}

func TestVerifySessionRejectsTamperedPayload(t *testing.T) {
	s := NewSigner(testSecret, time.Hour)
	now := time.Now()

	token, err := s.SignSession(domain.Session{UserID: "alice", IssuedAt: now, ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatalf("SignSession: %v", err)
	}

	parts := strings.Split(token, ".")
	parts[1] = b64(t, map[string]any{
		"sub": "mallory",
		"knd": kindSession,
		"iat": now.Unix(),
		"exp": now.Add(time.Hour).Unix(),
	})

	if _, err := s.VerifySession(strings.Join(parts, "."), now); !types.IsUnauthenticated(err) {
		t.Fatalf("tampered token was accepted (err = %v)", err)
	}
}

func TestSealOpenRoundTrip(t *testing.T) {
	s := NewSigner(testSecret, time.Hour)
	now := time.Now()

	sealed, err := s.Seal([]byte(`{"challenge":"xyz"}`), 5*time.Minute, now)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	got, err := s.Open(sealed, now)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if string(got) != `{"challenge":"xyz"}` {
		t.Errorf("Open = %s, want the sealed payload", got)
	}
}

func TestOpenRejectsExpired(t *testing.T) {
	s := NewSigner(testSecret, time.Hour)
	now := time.Now()

	sealed, err := s.Seal([]byte("state"), time.Minute, now)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	if _, err := s.Open(sealed, now.Add(2*time.Minute)); !types.IsUnauthenticated(err) {
		t.Fatalf("err = %v, want *types.UnauthenticatedError", err)
	}
}

// Both cookies are signed with the same key, so nothing but the kind claim
// stops a half-finished ceremony from being presented as proof of sign-in.
func TestKindsAreNotInterchangeable(t *testing.T) {
	s := NewSigner(testSecret, time.Hour)
	now := time.Now()

	ceremony, err := s.Seal([]byte("state"), time.Minute, now)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if _, err := s.VerifySession(ceremony, now); err == nil {
		t.Fatal("a ceremony token was accepted as a session")
	}

	session, err := s.SignSession(domain.Session{UserID: "u", IssuedAt: now, ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatalf("SignSession: %v", err)
	}
	if _, err := s.Open(session, now); err == nil {
		t.Fatal("a session token was accepted as ceremony state")
	}
}

func TestVerifySessionRejectsEmpty(t *testing.T) {
	s := NewSigner(testSecret, time.Hour)
	if _, err := s.VerifySession("", time.Now()); !types.IsUnauthenticated(err) {
		t.Fatalf("err = %v, want *types.UnauthenticatedError", err)
	}
}

// NewSigner copies the key, so a caller reusing or zeroing its buffer cannot
// silently invalidate every session in flight.
func TestNewSignerCopiesTheSecret(t *testing.T) {
	secret := make([]byte, len(testSecret))
	copy(secret, testSecret)

	s := NewSigner(secret, time.Hour)
	now := time.Now()
	token, err := s.SignSession(domain.Session{UserID: "u", IssuedAt: now, ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatalf("SignSession: %v", err)
	}

	for i := range secret {
		secret[i] = 0
	}

	if _, err := s.VerifySession(token, now); err != nil {
		t.Fatalf("VerifySession after the caller zeroed its buffer: %v", err)
	}
}

func b64(t *testing.T, v any) string {
	t.Helper()
	encoded, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(encoded)
}

// The anonymous marker has to survive the round trip, because it is the only
// thing standing between a guest session and a database lookup that can only
// fail.
func TestSessionCarriesTheAnonymousFlag(t *testing.T) {
	s := NewSigner(testSecret, time.Hour)
	now := time.Now()

	for _, anonymous := range []bool{true, false} {
		signed, err := s.SignSession(domain.Session{
			UserID:    "u1",
			IssuedAt:  now,
			ExpiresAt: now.Add(time.Hour),
			Anonymous: anonymous,
		})
		if err != nil {
			t.Fatalf("SignSession(anonymous=%t): %v", anonymous, err)
		}
		got, err := s.VerifySession(signed, now)
		if err != nil {
			t.Fatalf("VerifySession(anonymous=%t): %v", anonymous, err)
		}
		if got.Anonymous != anonymous {
			t.Errorf("Anonymous = %t, want %t", got.Anonymous, anonymous)
		}
	}
}

// A guest token is still a session token and nothing else: the kind check that
// separates the two cookie types must not have been weakened by the new claim.
func TestAnonymousSessionIsNotACeremonyToken(t *testing.T) {
	s := NewSigner(testSecret, time.Hour)
	now := time.Now()

	signed, err := s.SignSession(domain.Session{
		UserID:    "u1",
		IssuedAt:  now,
		ExpiresAt: now.Add(time.Hour),
		Anonymous: true,
	})
	if err != nil {
		t.Fatalf("SignSession: %v", err)
	}
	if _, err := s.Open(signed, now); err == nil {
		t.Fatal("Open accepted a session token as ceremony state")
	}
}
