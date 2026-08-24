// Package auth declares the ports the sign-in usecase drives.
//
// The three things authentication needs -- running a WebAuthn ceremony,
// exchanging an authorization code with an external provider, and signing a
// session token -- all arrive through libraries that import net/http, which
// depguard and `make lint/layers` forbid inside this layer and the one above
// it. So they are declared here as interfaces over plain strings, bytes and
// domain types, and implemented under internal/adapter. That indirection is
// not ceremony for its own sake: it is what keeps a transport library from
// deciding the shape of the application layer.
package auth

import (
	"context"
	"time"

	"github.com/promix1722/easydnd/internal/domain/user"
)

// Session is a proven claim that a request belongs to an account. It is the
// decoded form of the cookie the browser carries; nothing on the server
// records that it was issued.
type Session struct {
	UserID    user.ID
	IssuedAt  time.Time
	ExpiresAt time.Time

	// Anonymous reports that no account backs this session: the id was minted
	// for a guest and was never stored. It travels in the token because the
	// token is the only thing that knows -- looking the id up would find
	// nothing, which is indistinguishable from a deleted account.
	Anonymous bool
}

// UserLookup resolves the account behind a discoverable assertion.
//
// The usecase supplies this closure so the ceremony adapter never reaches for
// the repository itself -- an adapter that could load accounts would be a
// second, unsupervised path into storage.
type UserLookup func(rawCredentialID, userHandle []byte) (user.User, error)

// Ceremony runs the two WebAuthn exchanges. `options` is the JSON the browser
// hands to navigator.credentials; `state` is the challenge and its bindings,
// which must survive between the two calls and must not be chosen by the
// client. Both are opaque here on purpose.
type Ceremony interface {
	// BeginRegistration builds creation options for a brand-new account. u is
	// not yet stored: nothing is persisted until the ceremony finishes.
	BeginRegistration(u user.User) (options, state []byte, err error)

	// FinishRegistration verifies the attestation response and returns the
	// credential to store.
	FinishRegistration(u user.User, state, responseBody []byte) (user.Credential, error)

	// BeginLogin builds request options for a usernameless sign-in. It takes
	// no account, which is the point: the browser picks the passkey.
	BeginLogin() (options, state []byte, err error)

	// FinishLogin verifies the assertion, resolving the account through lookup,
	// and returns the account id and the credential with its updated counter.
	FinishLogin(state, responseBody []byte, lookup UserLookup) (user.ID, user.Credential, error)
}

// Federation runs the OAuth 2.0 authorization-code exchange with one external
// identity provider.
//
// It is one interface per provider rather than one with a provider argument,
// so that a provider's endpoints, client credentials and quirks are settled
// once when its adapter is constructed instead of being re-decided on every
// call. The usecase holds a map of them.
type Federation interface {
	// AuthCodeURL builds the URL to send the browser to. The three arguments
	// are the caller's, not the adapter's: state and nonce bind the eventual
	// callback to this attempt, and an adapter that minted them itself would
	// have nowhere to keep them.
	//
	// The third is the PKCE *verifier*, not the challenge derived from it --
	// the same value Exchange is later handed. Deriving the challenge is the
	// adapter's job, because the derivation belongs with the library that
	// checks it: passing a pre-hashed challenge here once meant it was hashed
	// a second time on the way out, which no test with a stubbed provider
	// could notice and which broke every real sign-in at the exchange. One
	// value crossing this boundary is what makes the two halves impossible to
	// mismatch. It must never appear in the URL.
	AuthCodeURL(state, nonce, verifier string) string

	// Exchange trades an authorization code for a verified identity: it posts
	// the code and the PKCE verifier to the provider, validates the returned
	// ID token, and checks that the token's nonce is the one we sent.
	//
	// It takes a context because, unlike every other port in this package, it
	// performs network I/O against a third party -- and a sign-in must not
	// outlive the request that started it.
	Exchange(ctx context.Context, code, nonce, verifier string) (user.Identity, error)
}

// Signer mints and checks the two opaque strings this design puts in cookies.
//
// Sessions are stateless: a token is valid because it verifies and has not
// expired, not because a row somewhere says so. There is deliberately no
// Revoke method -- the only way to invalidate outstanding sessions is to
// rotate the signing key, which invalidates all of them.
type Signer interface {
	// SignSession renders s as a token.
	SignSession(s Session) (string, error)

	// VerifySession checks a token against now and returns what it claims.
	// A token that is malformed, expired or signed by another key yields a
	// *types.UnauthenticatedError.
	VerifySession(token string, now time.Time) (Session, error)

	// Seal wraps short-lived ceremony state so it can ride in a cookie
	// without a server-side store, tamper-evident and self-expiring.
	Seal(payload []byte, ttl time.Duration, now time.Time) (string, error)

	// Open unwraps a value produced by Seal.
	Open(token string, now time.Time) ([]byte, error)
}
