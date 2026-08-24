// Package user holds the account aggregate.
//
// An account here is a display name and the ways its owner can prove they are
// its owner: a set of WebAuthn credentials, and a set of identities asserted
// by external providers. There is still no password. The two kinds of proof
// are separate slices rather than one, because they share nothing -- a
// credential is a public key we verify a signature against, and an identity is
// a subject some other party vouched for.
//
// This is the innermost layer. It imports the standard library and nothing
// else: no gin, no net/http, no database/sql, and no JSON or database struct
// tags. In particular it does not import the WebAuthn library, whose types
// drag in net/http; the ceremony lives behind a port in internal/domain/auth
// and is implemented by an adapter.
package user

import (
	"context"
	"time"
)

// ID identifies an account. It is opaque to everything above this layer and
// doubles as the WebAuthn user handle, which is why it must contain no
// personal information: the handle is stored on the authenticator, outside
// our control and beyond our reach to delete.
type ID string

// AnonymousIDPrefix marks an id that names a guest rather than an account.
//
// This prefix is not what makes a session anonymous -- the session token says
// so, and that is the authority. What the prefix buys is a guarantee that the
// two id spaces cannot overlap: account ids are base64url text, and ':' is not
// in that alphabet, so no account can ever be issued an id that looks like a
// guest's.
//
// A guest used to be written nowhere at all. Groups ended that: somebody who
// joins another person's table has to be nameable in a roster the rest of the
// table reads, and there is exactly one place in this schema that says what a
// person is called. EnsureGuest materialises a row at that moment and at no
// other -- a guest who never joins a group is still stored nowhere.
const AnonymousIDPrefix = "anon:"

// User is an account -- or, when Anonymous, a guest that only looks like one.
type User struct {
	ID          ID
	DisplayName string
	CreatedAt   time.Time
	// Credentials holds every passkey registered to this account. It is a
	// slice rather than a single value because the only defence against a
	// lost device is a second registered authenticator.
	Credentials []Credential
	// Identities holds every external account linked to this one. Same
	// reasoning as Credentials, and the two are additive: an account with a
	// passkey and a Google identity can be reached either way, which is the
	// closest thing to account recovery this design has.
	Identities []Identity

	// Anonymous reports that no stored account backs this value: it was
	// synthesised from a guest session token and nothing in any repository
	// corresponds to it.
	//
	// Repositories never set this. It exists because everything above the
	// usecase -- the middleware context, the handlers, the wire DTO -- passes
	// a whole User around, and those layers have to be able to tell a guest
	// from an account without asking storage, which is the one thing a guest
	// cannot be found in. A guest holds neither credentials nor identities,
	// so it has no way in to lose and nothing to link.
	Anonymous bool
}

// SignInMethods counts the ways this account can be reached.
//
// Unlinking the last one would strand the account forever -- nobody could sign
// in and nothing could restore it -- so the usecase refuses to go below one.
func (u User) SignInMethods() int { return len(u.Credentials) + len(u.Identities) }

// Credential is one registered passkey.
//
// The field set is what a relying party must keep to verify a future
// assertion: the credential id to look the account up by, the public key to
// check the signature with, and the counter to notice a cloned authenticator.
type Credential struct {
	// ID is the raw credential id as the authenticator reported it, not a
	// base64 rendering of one. Encoding is the adapters' business.
	ID        []byte
	PublicKey []byte // COSE-encoded

	AttestationType string
	Transports      []string
	AAGUID          []byte

	// SignCount is the authenticator's own counter. Many platform
	// authenticators report zero forever, so a stalled counter is normal and
	// only a decrease is evidence of anything.
	SignCount uint32

	// BackupEligible and BackupState record whether the credential syncs to a
	// password manager or cloud keychain. A synced passkey survives losing the
	// device it was made on; a device-bound one does not.
	BackupEligible bool
	BackupState    bool

	CreatedAt  time.Time
	LastUsedAt time.Time
}

// Provider names an external identity provider.
//
// It is a string rather than an enum so that storage, wire formats and logs
// all render it the same way, and so adding one is a constant rather than a
// migration.
type Provider string

// ProviderGoogle is the only provider implemented today.
const ProviderGoogle Provider = "google"

// Identity is a proven claim from an external identity provider: this person
// is, as far as that provider is concerned, subject S.
type Identity struct {
	Provider Provider

	// Subject is the provider's own stable id for the person -- the OIDC
	// `sub`. It is the only field ever used as a key, and deliberately so:
	// an email address can be released and reassigned to somebody else, so
	// keying on one would eventually hand an account to a stranger.
	Subject string

	// Email and EmailVerified record what the provider asserted, for display
	// and for support questions. Nothing resolves an account by either.
	Email         string
	EmailVerified bool

	// DisplayName is the provider's name for the person at link time. The
	// account keeps its own DisplayName; this is only ever informational, so
	// that a later rename upstream cannot silently rewrite what we show.
	DisplayName string

	CreatedAt  time.Time
	LastUsedAt time.Time
}

// Repository is the persistence port for accounts. Implementations live under
// internal/adapter/repository; internal/app picks the concrete one, and that
// assignment is what proves conformance at compile time.
type Repository interface {
	// Create stores u together with its initial credentials. Implementations
	// report a *types.ValidationError if the id or any credential id is
	// already taken.
	Create(ctx context.Context, u User) error

	// EnsureGuest stores the bare minimum row a guest needs in order to be
	// named somewhere another person can read -- today, a group roster.
	//
	// It is idempotent, and that is the whole difference from Create: a guest
	// may join several groups, and every one of those joins reaches this
	// method, so "already there" is the expected case and not an error. It
	// writes no credentials and no identities, because a guest has neither and
	// never will; the row it leaves behind is an account nobody can ever sign
	// in to.
	//
	// It is only ever called with a guest id -- see AnonymousIDPrefix.
	// Implementations must not use it to conjure an account.
	EnsureGuest(ctx context.Context, u User) error

	// ByID returns the account with the given id, or a *types.NotFoundError.
	ByID(ctx context.Context, id ID) (User, error)

	// ByCredentialID returns the account owning the given raw credential id,
	// or a *types.NotFoundError. This is the lookup a usernameless sign-in
	// depends on: the assertion names the credential, not the account.
	ByCredentialID(ctx context.Context, credentialID []byte) (User, error)

	// TouchCredential records a successful assertion -- the new sign count and
	// the time it was used.
	TouchCredential(ctx context.Context, id ID, c Credential) error

	// ByIdentity returns the account holding the given external identity, or
	// a *types.NotFoundError. This is the lookup a federated sign-in depends
	// on: the provider names a subject, and only we know whose account it is.
	ByIdentity(ctx context.Context, provider Provider, subject string) (User, error)

	// AddIdentity links an external identity to an existing account.
	// Implementations report a *types.ValidationError if that identity is
	// already linked to any account, including this one -- moving a subject
	// between accounts must be an explicit unlink and relink, never a
	// side effect of signing in.
	AddIdentity(ctx context.Context, id ID, i Identity) error

	// TouchIdentity records a successful federated sign-in. Like
	// TouchCredential it updates only the fields a sign-in can legitimately
	// move, so a replayed exchange cannot rewrite the link itself.
	TouchIdentity(ctx context.Context, id ID, i Identity) error

	// RemoveIdentity unlinks an external identity. It reports a
	// *types.NotFoundError if the account does not hold it. It does not
	// enforce that an account keeps a way in -- that is the usecase's rule,
	// because it spans both kinds of proof.
	RemoveIdentity(ctx context.Context, id ID, provider Provider, subject string) error
}
