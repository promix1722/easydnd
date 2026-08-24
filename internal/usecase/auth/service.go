// Package auth implements the sign-in usecases.
//
// It depends on the user domain, the auth ports and internal/types, and on
// nothing else. In particular it never sees a *gin.Context, a *http.Request or
// the WebAuthn library: handlers pass a plain context.Context and plain bytes
// across this boundary, and the ceremony arrives through a port.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	domain "github.com/promix1722/easydnd/internal/domain/auth"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// Display name bounds. The lower bound stops an empty passkey label; the upper
// one is well under the 64-byte user handle limit and keeps the OS prompt
// readable.
const (
	MinDisplayName = 1
	MaxDisplayName = 64
)

// userIDBytes is the entropy behind an account id. Sixteen random bytes is far
// past collision risk and, once base64'd, comfortably inside the 64-byte
// WebAuthn user handle limit.
const userIDBytes = 16

// GuestDisplayName is what a session with no account behind it is called on
// screen. It is not a name anyone chose, and it is not stored anywhere; it
// exists so the header has something to render.
const GuestDisplayName = "Guest"

// Sealed-envelope kinds. Both the WebAuthn ceremony and the SSO flight ride in
// a cookie sealed by the same Signer, so each says which it is and each
// refuses the other. Without that, a value minted by one flow would be fed to
// the other's finish endpoint and rejected -- or not -- for reasons that
// depend on how JSON happens to decode, which is not a security argument.
const (
	kindWebAuthn = "webauthn"
	kindSSO      = "sso"
)

// pending is the ceremony state sealed into the client's cookie between the
// begin and finish calls.
//
// For registration it carries the not-yet-stored account, which is what lets
// an abandoned sign-up leave nothing behind: no record exists until an
// attestation actually verifies.
type pending struct {
	Kind  string      `json:"knd"`
	State []byte      `json:"state"`
	User  *storedUser `json:"user,omitempty"`
}

// storedUser is the transient account carried through registration. It is a
// separate type because domain entities carry no struct tags by design.
type storedUser struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

// Service holds the sign-in usecases. Every dependency arrives through the
// constructor; there are no package-level singletons.
type Service struct {
	repo     user.Repository
	ceremony domain.Ceremony
	signer   domain.Signer
	// federations holds one entry per configured external provider. It is
	// empty when none is configured, which is a supported deployment rather
	// than a broken one: passkeys alone are a complete sign-in story.
	federations     map[user.Provider]domain.Federation
	log             *slog.Logger
	sessionTTL      time.Duration
	guestSessionTTL time.Duration
	ceremonyTTL     time.Duration
	now             func() time.Time
}

// Config carries the lifetimes the service applies.
type Config struct {
	SessionTTL time.Duration
	// GuestSessionTTL is deliberately separate from, and shorter than,
	// SessionTTL. A guest token cannot be revoked and names nothing anyone
	// can recover, so it should not be good for a week.
	GuestSessionTTL time.Duration
	CeremonyTTL     time.Duration
}

// NewService wires a Service over the given ports.
//
// federations may be nil or empty; every SSO entry point then reports the
// provider as unknown, and Providers returns nothing for the client to offer.
func NewService(
	repo user.Repository,
	ceremony domain.Ceremony,
	signer domain.Signer,
	federations map[user.Provider]domain.Federation,
	cfg Config,
	log *slog.Logger,
) *Service {
	return &Service{
		repo:            repo,
		ceremony:        ceremony,
		signer:          signer,
		federations:     federations,
		log:             log,
		sessionTTL:      cfg.SessionTTL,
		guestSessionTTL: cfg.GuestSessionTTL,
		ceremonyTTL:     cfg.CeremonyTTL,
		now:             time.Now,
	}
}

// CeremonyTTL reports how long a begin/finish pair stays valid, so the handler
// can set a matching cookie lifetime without owning the number.
func (s *Service) CeremonyTTL() time.Duration { return s.ceremonyTTL }

// SessionTTL reports how long an issued session lasts.
func (s *Service) SessionTTL() time.Duration { return s.sessionTTL }

// GuestSessionTTL reports how long an anonymous session lasts, so the handler
// can give the cookie the same lifetime as the token inside it.
func (s *Service) GuestSessionTTL() time.Duration { return s.guestSessionTTL }

// BeginRegistration starts a sign-up.
//
// It asks for nothing, which is the whole point: sign-in is discoverable and
// names no account, so a sign-up that demanded a piece of text would be the
// only place in this API where a visitor had to know which of the two they
// were doing. The account id and the label the operating system's passkey
// prompt shows are both minted here -- see newDisplayName.
//
// Nothing is written: the freshly minted account rides inside the sealed
// ceremony token and is stored only if FinishRegistration verifies. An
// abandoned sign-up therefore leaves no orphan record and no reserved name.
func (s *Service) BeginRegistration(_ context.Context) (options []byte, ceremony string, err error) {
	name, err := newDisplayName()
	if err != nil {
		return nil, "", err
	}

	id, err := newUserID()
	if err != nil {
		return nil, "", err
	}

	candidate := user.User{ID: id, DisplayName: name, CreatedAt: s.now()}

	options, state, err := s.ceremony.BeginRegistration(candidate)
	if err != nil {
		return nil, "", err
	}

	sealed, err := s.seal(pending{
		State: state,
		User: &storedUser{
			ID:          string(candidate.ID),
			DisplayName: candidate.DisplayName,
			CreatedAt:   candidate.CreatedAt,
		},
	})
	if err != nil {
		return nil, "", err
	}
	return options, sealed, nil
}

// FinishRegistration completes a sign-up and issues a session.
func (s *Service) FinishRegistration(ctx context.Context, ceremony string, responseBody []byte) (user.User, string, error) {
	p, err := s.open(ceremony)
	if err != nil {
		return user.User{}, "", err
	}
	if p.User == nil {
		return user.User{}, "", types.NewValidationError("this is not a registration ceremony")
	}

	candidate := user.User{
		ID:          user.ID(p.User.ID),
		DisplayName: p.User.DisplayName,
		CreatedAt:   p.User.CreatedAt,
	}

	credential, err := s.ceremony.FinishRegistration(candidate, p.State, responseBody)
	if err != nil {
		return user.User{}, "", err
	}

	candidate.Credentials = []user.Credential{credential}
	if err := s.repo.Create(ctx, candidate); err != nil {
		return user.User{}, "", err
	}

	token, err := s.issue(candidate.ID)
	if err != nil {
		return user.User{}, "", err
	}

	s.log.Info("account registered", "user_id", candidate.ID)
	return candidate, token, nil
}

// BeginLogin starts a usernameless sign-in. It takes no input at all: the
// browser picks which passkey to offer.
func (s *Service) BeginLogin(_ context.Context) (options []byte, ceremony string, err error) {
	options, state, err := s.ceremony.BeginLogin()
	if err != nil {
		return nil, "", err
	}
	sealed, err := s.seal(pending{State: state})
	if err != nil {
		return nil, "", err
	}
	return options, sealed, nil
}

// FinishLogin completes a sign-in and issues a session.
func (s *Service) FinishLogin(ctx context.Context, ceremony string, responseBody []byte) (user.User, string, error) {
	p, err := s.open(ceremony)
	if err != nil {
		return user.User{}, "", err
	}
	if p.User != nil {
		return user.User{}, "", types.NewValidationError("this is not a sign-in ceremony")
	}

	// The lookup runs inside the ceremony, which is why it is a closure: the
	// adapter must never hold the repository itself.
	lookup := func(rawCredentialID, userHandle []byte) (user.User, error) {
		found, lookupErr := s.repo.ByCredentialID(ctx, rawCredentialID)
		if lookupErr != nil {
			return user.User{}, lookupErr
		}
		// The handle is the account id, and the spec requires it to match the
		// credential's owner. Without this check an authenticator could name
		// one account while presenting another's credential.
		if len(userHandle) > 0 && string(userHandle) != string(found.ID) {
			return user.User{}, types.NewUnauthenticatedError("credential does not belong to the asserted account")
		}
		return found, nil
	}

	id, credential, err := s.ceremony.FinishLogin(p.State, responseBody, lookup)
	if err != nil {
		return user.User{}, "", err
	}

	current, err := s.repo.ByID(ctx, id)
	if err != nil {
		return user.User{}, "", s.asUnauthenticated(err)
	}
	s.warnOnStalledCounter(current, credential)

	credential.LastUsedAt = s.now()
	if err := s.repo.TouchCredential(ctx, id, credential); err != nil {
		return user.User{}, "", err
	}

	token, err := s.issue(id)
	if err != nil {
		return user.User{}, "", err
	}

	s.log.Info("account signed in", "user_id", id)
	return current, token, nil
}

// Session resolves a session token to the account behind it.
//
// A token that verifies but names an account that is gone yields an
// unauthenticated error, not a 500. A session lasts a week and nothing
// server-side records that it exists, so a cookie can always outlive the
// account it names -- a deleted account, or a cookie minted against a different
// store. The browser must be told to sign in rather than shown an error page.
func (s *Service) Session(ctx context.Context, token string) (user.User, error) {
	session, err := s.signer.VerifySession(token, s.now())
	if err != nil {
		return user.User{}, err
	}
	// A guest has no row to find, so looking one up would report "that account
	// is gone" for a session that is working exactly as intended. The token is
	// the whole record; rebuild the identity from it and stop here.
	if session.Anonymous {
		return guestUser(session), nil
	}
	found, err := s.repo.ByID(ctx, session.UserID)
	if err != nil {
		return user.User{}, s.asUnauthenticated(err)
	}
	return found, nil
}

// SignInAnonymously issues a session that names no account.
//
// Nothing is written and nothing is reserved: the id is minted here, sealed
// into the token, and exists only for as long as that token does. That is the
// entire feature -- somebody who wants to try the app without committing to an
// unrecoverable passkey gets an identity that owns characters for a day and
// then stops existing.
//
// Because there is no account, there is nothing to come back to: a guest who
// clears their cookie, or waits out the TTL, has lost whatever they built.
// Every surface that shows a guest session is obliged to say so.
func (s *Service) SignInAnonymously(_ context.Context) (user.User, string, error) {
	id, err := newAnonymousID()
	if err != nil {
		return user.User{}, "", err
	}

	now := s.now()
	guest := user.User{
		ID:          id,
		DisplayName: GuestDisplayName,
		CreatedAt:   now,
		Anonymous:   true,
	}

	token, err := s.signer.SignSession(domain.Session{
		UserID:    id,
		IssuedAt:  now,
		ExpiresAt: now.Add(s.guestSessionTTL),
		Anonymous: true,
	})
	if err != nil {
		return user.User{}, "", err
	}

	s.log.Info("anonymous session issued", "user_id", id)
	return guest, token, nil
}

// guestUser rebuilds the identity a guest session names.
//
// CreatedAt is the token's issue time rather than a stored one, because there
// is no stored one: for a guest the session and the "account" are the same
// thing and began at the same moment.
func guestUser(session domain.Session) user.User {
	return user.User{
		ID:          session.UserID,
		DisplayName: GuestDisplayName,
		CreatedAt:   session.IssuedAt,
		Anonymous:   true,
	}
}

// issue mints a session token for id.
func (s *Service) issue(id user.ID) (string, error) {
	now := s.now()
	return s.signer.SignSession(domain.Session{
		UserID:    id,
		IssuedAt:  now,
		ExpiresAt: now.Add(s.sessionTTL),
	})
}

func (s *Service) seal(p pending) (string, error) {
	p.Kind = kindWebAuthn
	encoded, err := json.Marshal(p)
	if err != nil {
		return "", types.WrapServerError(err, "encode ceremony envelope")
	}
	return s.signer.Seal(encoded, s.ceremonyTTL, s.now())
}

func (s *Service) open(token string) (pending, error) {
	payload, err := s.signer.Open(token, s.now())
	if err != nil {
		return pending{}, err
	}
	var p pending
	if err := json.Unmarshal(payload, &p); err != nil {
		return pending{}, types.NewUnauthenticatedError("ceremony envelope is not readable")
	}
	if p.Kind != kindWebAuthn {
		return pending{}, types.NewUnauthenticatedError("this is not a passkey ceremony")
	}
	if len(p.State) == 0 {
		return pending{}, types.NewUnauthenticatedError("ceremony envelope is empty")
	}
	return p, nil
}

// asUnauthenticated rewrites "that account is gone" as "you are not signed
// in". Anything else passes through untouched.
func (s *Service) asUnauthenticated(err error) error {
	if types.IsNotFound(err) {
		return types.NewUnauthenticatedError("session no longer identifies an account")
	}
	return err
}

// warnOnStalledCounter logs a possible authenticator clone without acting on
// it.
//
// A decreasing sign count is the specification's clone signal, but most
// platform authenticators report zero forever, so the check fires only when
// both values are non-zero. Even then it stays a log line: locking someone out
// on the strength of a counter that most devices never move would break far
// more sign-ins than it would stop.
func (s *Service) warnOnStalledCounter(current user.User, presented user.Credential) {
	index := slices.IndexFunc(current.Credentials, func(stored user.Credential) bool {
		return slices.Equal(stored.ID, presented.ID)
	})
	if index < 0 {
		return
	}
	stored := current.Credentials[index]
	if stored.SignCount != 0 && presented.SignCount != 0 && presented.SignCount <= stored.SignCount {
		s.log.Warn("authenticator sign count did not advance",
			"user_id", current.ID,
			"stored", stored.SignCount,
			"presented", presented.SignCount,
		)
	}
}

// normalizeDisplayName trims and bounds a display name this service did not
// mint. Nothing a visitor types reaches it any more -- there is no such text
// left in the auth surface -- but a provider's claims still arrive as whatever
// that provider felt like sending, and newDisplayName runs its own output
// through it as a cheap guard on the column's CHECK constraint.
func normalizeDisplayName(name string) (string, error) {
	name = strings.TrimSpace(name)
	switch {
	case utf8.RuneCountInString(name) < MinDisplayName:
		return "", types.NewFieldValidationError("display name is required",
			types.FieldError{Field: "display_name", Rule: "required", Message: "tell us what to call you"})
	case utf8.RuneCountInString(name) > MaxDisplayName:
		return "", types.NewFieldValidationError("display name is too long",
			types.FieldError{Field: "display_name", Rule: "max", Message: "at most 64 characters"})
	case !utf8.ValidString(name):
		return "", types.NewFieldValidationError("display name is not valid text",
			types.FieldError{Field: "display_name", Rule: "encoding"})
	}
	return name, nil
}

// newUserID mints an opaque account id, which doubles as the WebAuthn user
// handle. It carries no personal information on purpose: the handle is written
// to the authenticator, where we can never reach it again.
func newUserID() (user.ID, error) {
	return randomID("", "generate account id")
}

// newAnonymousID mints a guest id, prefixed so it can never be mistaken for --
// or collide with -- an account id. See user.AnonymousIDPrefix.
func newAnonymousID() (user.ID, error) {
	return randomID(user.AnonymousIDPrefix, "generate guest id")
}

// randomID draws userIDBytes of entropy and renders it URL-safe behind prefix.
func randomID(prefix, what string) (user.ID, error) {
	raw := make([]byte, userIDBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", types.WrapServerError(err, "%s", what)
	}
	return user.ID(prefix + base64.RawURLEncoding.EncodeToString(raw)), nil
}
