// Package token signs and verifies the opaque strings this application puts in
// cookies.
//
// It is an outbound adapter: it implements the auth.Signer port and is the
// only package that knows the tokens are JWTs. Nothing above this layer may
// assume that -- the session cookie could become a sealed box tomorrow without
// a line changing outside this directory.
package token

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	domain "github.com/promix1722/easydnd/internal/domain/auth"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// Token kinds, carried in a private claim.
//
// Both cookies are signed with the same key, so without this a ceremony token
// would verify perfectly well as a session token. The check is what stops a
// half-finished registration from being replayed as proof of sign-in.
const (
	kindSession  = "session"
	kindCeremony = "ceremony"
)

// signingMethod is pinned rather than read from the token header. Trusting the
// header's alg is the classic JWT forgery: a token claiming "none", or claiming
// HMAC against a public key, verifies against a parser that believes it.
var signingMethod = jwt.SigningMethodHS256

// claims is the wire form of both token kinds.
type claims struct {
	jwt.RegisteredClaims
	Kind string `json:"knd"`
	// State carries sealed ceremony bytes. Empty on a session token.
	State string `json:"st,omitempty"`
	// Anon marks a session that names a guest rather than a stored account.
	// Absent on every other token, which is what makes an account session
	// unable to claim to be a guest one and vice versa.
	Anon bool `json:"anon,omitempty"`
}

// Signer implements auth.Signer over HMAC-SHA256.
type Signer struct {
	secret     []byte
	sessionTTL time.Duration
}

// NewSigner returns a Signer over the given key. The key must be at least
// config.MinSessionSecretBytes long; config enforces that before we get here.
func NewSigner(secret []byte, sessionTTL time.Duration) *Signer {
	// Copy: the caller's slice is configuration and could be reused or
	// zeroed, and a signing key that changes underneath us fails in a way
	// that looks like every session expiring at once.
	key := make([]byte, len(secret))
	copy(key, secret)
	return &Signer{secret: key, sessionTTL: sessionTTL}
}

// SessionTTL reports how long a freshly issued session lasts. The usecase asks
// rather than being told, so the lifetime lives in one place.
func (s *Signer) SessionTTL() time.Duration { return s.sessionTTL }

// SignSession renders a session as a token.
func (s *Signer) SignSession(session domain.Session) (string, error) {
	if session.UserID == "" {
		return "", types.NewServerError("sign session: empty user id")
	}
	return s.sign(claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   string(session.UserID),
			IssuedAt:  jwt.NewNumericDate(session.IssuedAt),
			ExpiresAt: jwt.NewNumericDate(session.ExpiresAt),
		},
		Kind: kindSession,
		Anon: session.Anonymous,
	})
}

// VerifySession checks a token and returns what it claims.
func (s *Signer) VerifySession(token string, now time.Time) (domain.Session, error) {
	parsed, err := s.parse(token, kindSession, now)
	if err != nil {
		return domain.Session{}, err
	}
	if parsed.Subject == "" || parsed.ExpiresAt == nil || parsed.IssuedAt == nil {
		return domain.Session{}, types.NewUnauthenticatedError("session token is incomplete")
	}
	return domain.Session{
		UserID:    user.ID(parsed.Subject),
		IssuedAt:  parsed.IssuedAt.Time,
		ExpiresAt: parsed.ExpiresAt.Time,
		Anonymous: parsed.Anon,
	}, nil
}

// Seal wraps ceremony state in a short-lived token.
func (s *Signer) Seal(payload []byte, ttl time.Duration, now time.Time) (string, error) {
	return s.sign(claims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		Kind:  kindCeremony,
		State: base64.RawURLEncoding.EncodeToString(payload),
	})
}

// Open unwraps a value produced by Seal.
func (s *Signer) Open(token string, now time.Time) ([]byte, error) {
	parsed, err := s.parse(token, kindCeremony, now)
	if err != nil {
		return nil, err
	}
	payload, err := base64.RawURLEncoding.DecodeString(parsed.State)
	if err != nil {
		return nil, types.NewUnauthenticatedError("ceremony state is not decodable")
	}
	return payload, nil
}

func (s *Signer) sign(c claims) (string, error) {
	signed, err := jwt.NewWithClaims(signingMethod, c).SignedString(s.secret)
	if err != nil {
		return "", types.WrapServerError(err, "sign %s token", c.Kind)
	}
	return signed, nil
}

// parse verifies signature, expiry and kind.
//
// Every failure below collapses to the same *types.UnauthenticatedError, and
// the message never says which check failed: distinguishing "expired" from
// "bad signature" for the client tells an attacker which half of a forgery
// attempt was right.
func (s *Signer) parse(token, want string, now time.Time) (claims, error) {
	if token == "" {
		return claims{}, types.NewUnauthenticatedError("no %s token", want)
	}

	var c claims
	_, err := jwt.ParseWithClaims(token, &c,
		func(*jwt.Token) (any, error) { return s.secret, nil },
		jwt.WithValidMethods([]string{signingMethod.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(func() time.Time { return now }),
	)
	if err != nil {
		// Malformed, expired, wrong key -- all the same to the caller.
		if errors.Is(err, jwt.ErrTokenExpired) {
			return claims{}, types.NewUnauthenticatedError("%s token has expired", want)
		}
		return claims{}, types.NewUnauthenticatedError("%s token is not valid", want)
	}

	// Constant time only for tidiness; the kind is not a secret. What matters
	// is that the comparison happens at all.
	if subtle.ConstantTimeCompare([]byte(c.Kind), []byte(want)) != 1 {
		return claims{}, types.NewUnauthenticatedError("token is not a %s token", want)
	}
	return c, nil
}

// Compile-time proof that this adapter satisfies the port.
var _ domain.Signer = (*Signer)(nil)
