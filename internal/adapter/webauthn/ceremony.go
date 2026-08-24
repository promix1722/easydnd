// Package webauthn runs the two WebAuthn exchanges.
//
// It is an outbound adapter implementing the auth.Ceremony port, and it is the
// only package in the tree that imports github.com/go-webauthn/webauthn. That
// containment is not stylistic: the library's types reach for net/http, which
// depguard forbids in the domain and application layers.
//
// The package is named webauthn and imports a package of the same name, so the
// import is aliased to `lib`.
package webauthn

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	lib "github.com/go-webauthn/webauthn/webauthn"

	domain "github.com/promix1722/easydnd/internal/domain/auth"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// requireResidentKey is addressable because the protocol struct takes a
// *bool: unset and false mean different things on the wire.
var requireResidentKey = true

// Ceremony implements auth.Ceremony.
type Ceremony struct {
	rp  *lib.WebAuthn
	now func() time.Time
}

// Config is what the adapter needs to identify the relying party. It mirrors
// config.AuthConfig without importing it, so this package stays a leaf.
type Config struct {
	RPID          string
	RPDisplayName string
	RPOrigins     []string
	// CeremonyTimeout bounds how long a begin/finish pair may take. It is
	// enforced server-side as well as advertised to the browser, so a sealed
	// challenge cannot outlive it even if the client ignores the hint.
	CeremonyTimeout time.Duration
}

// New builds a Ceremony.
//
// The authenticator selection below is what makes sign-in usernameless:
// requiring a resident (discoverable) key means the credential carries the
// user handle on the authenticator, so the browser can offer the right passkey
// with nothing typed. User verification is preferred rather than required so
// that a hardware key without a PIN or biometric still works; the passkey
// itself is the authentication factor.
//
// Attestation is "none" deliberately. We do not care which make and model of
// authenticator a player used, and asking for attestation would collect a
// device identifier we have no use for.
func New(cfg Config) (*Ceremony, error) {
	timeout := lib.TimeoutConfig{
		Enforce:    true,
		Timeout:    cfg.CeremonyTimeout,
		TimeoutUVD: cfg.CeremonyTimeout,
	}

	rp, err := lib.New(&lib.Config{
		RPID:                  cfg.RPID,
		RPDisplayName:         cfg.RPDisplayName,
		RPOrigins:             cfg.RPOrigins,
		AttestationPreference: protocol.PreferNoAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey: protocol.ResidentKeyRequirementRequired,
			// The superseded spelling of the line above, sent alongside it
			// because authenticators and browsers predating residentKey still
			// read this one and would otherwise make a non-discoverable
			// credential -- which sign-in, having no username to offer, could
			// never find again.
			RequireResidentKey: &requireResidentKey,
			UserVerification:   protocol.VerificationPreferred,
		},
		Timeouts: lib.TimeoutsConfig{Login: timeout, Registration: timeout},
	})
	if err != nil {
		return nil, fmt.Errorf("configure webauthn relying party: %w", err)
	}
	return &Ceremony{rp: rp, now: time.Now}, nil
}

// BeginRegistration builds creation options for a new account.
func (c *Ceremony) BeginRegistration(u user.User) (options, state []byte, err error) {
	// Exclude what the account already has, so a second registration on the
	// same authenticator is refused by the browser with a clear message
	// instead of quietly making a duplicate passkey.
	var opts []lib.RegistrationOption
	if len(u.Credentials) > 0 {
		existing := wrap(u).WebAuthnCredentials()
		exclusions := make([]protocol.CredentialDescriptor, 0, len(existing))
		for i := range existing {
			exclusions = append(exclusions, existing[i].Descriptor())
		}
		opts = append(opts, lib.WithExclusions(exclusions))
	}

	creation, session, err := c.rp.BeginRegistration(wrap(u), opts...)
	if err != nil {
		return nil, nil, types.WrapServerError(err, "begin registration")
	}
	return marshalPair(creation, session)
}

// FinishRegistration verifies an attestation response and returns the
// credential to store.
func (c *Ceremony) FinishRegistration(u user.User, state, responseBody []byte) (user.Credential, error) {
	session, err := unmarshalState(state)
	if err != nil {
		return user.Credential{}, err
	}

	parsed, err := protocol.ParseCredentialCreationResponseBody(bytes.NewReader(responseBody))
	if err != nil {
		return user.Credential{}, types.NewValidationError("registration response is not usable: %s", protocolReason(err))
	}

	credential, err := c.rp.CreateCredential(wrap(u), session, parsed)
	if err != nil {
		// A failed ceremony is the client's problem, not ours: a mismatched
		// challenge, a wrong origin, an unverified user. None of them are a
		// 500, and none of them should say which check failed.
		return user.Credential{}, types.NewValidationError("registration could not be verified")
	}
	return c.toDomain(credential), nil
}

// BeginLogin builds request options for a usernameless sign-in.
func (c *Ceremony) BeginLogin() (options, state []byte, err error) {
	// Discoverable: no user, no allowed-credentials list. The list is what
	// would leak which accounts exist, and omitting it is what lets the
	// browser pick the passkey.
	assertion, session, err := c.rp.BeginDiscoverableLogin()
	if err != nil {
		return nil, nil, types.WrapServerError(err, "begin login")
	}
	return marshalPair(assertion, session)
}

// FinishLogin verifies an assertion and reports whose it was.
func (c *Ceremony) FinishLogin(state, responseBody []byte, lookup domain.UserLookup) (user.ID, user.Credential, error) {
	session, err := unmarshalState(state)
	if err != nil {
		return "", user.Credential{}, err
	}

	parsed, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(responseBody))
	if err != nil {
		return "", user.Credential{}, types.NewValidationError("sign-in response is not usable: %s", protocolReason(err))
	}

	// resolved captures who the handler found. The library hands us back only
	// the credential, and we need the account id that went with it.
	var resolved user.User
	handler := func(rawID, userHandle []byte) (lib.User, error) {
		found, lookupErr := lookup(rawID, userHandle)
		if lookupErr != nil {
			return nil, lookupErr
		}
		resolved = found
		return wrap(found), nil
	}

	credential, err := c.rp.ValidateDiscoverableLogin(handler, session, parsed)
	if err != nil {
		return "", user.Credential{}, types.NewUnauthenticatedError("sign-in could not be verified")
	}
	return resolved.ID, c.toDomain(credential), nil
}

// toDomain narrows the library's credential record to what we keep.
//
// CloneWarning is deliberately not fatal. Platform authenticators overwhelmingly
// report a sign count of zero forever, so a stalled counter is the norm and the
// warning is advisory; the caller logs it rather than locking an account out on
// the strength of a counter that most devices never move.
func (c *Ceremony) toDomain(cred *lib.Credential) user.Credential {
	transports := make([]string, 0, len(cred.Transport))
	for _, t := range cred.Transport {
		transports = append(transports, string(t))
	}
	now := c.now()
	return user.Credential{
		ID:              cred.ID,
		PublicKey:       cred.PublicKey,
		AttestationType: cred.AttestationType,
		Transports:      transports,
		AAGUID:          cred.Authenticator.AAGUID,
		SignCount:       cred.Authenticator.SignCount,
		BackupEligible:  cred.Flags.BackupEligible,
		BackupState:     cred.Flags.BackupState,
		CreatedAt:       now,
		LastUsedAt:      now,
	}
}

// wrap adapts a domain account to the interface the library expects.
func wrap(u user.User) lib.User { return webauthnUser{u} }

// webauthnUser exists only so that user.User itself needs no WebAuthn-shaped
// methods. Keeping the adaptation here is what lets the domain package stay
// ignorant of the protocol.
type webauthnUser struct{ u user.User }

// WebAuthnID returns the user handle. It is the account id verbatim, which is
// why user.ID must be opaque: the handle is written to the authenticator and
// we can never take it back.
func (w webauthnUser) WebAuthnID() []byte { return []byte(w.u.ID) }

// WebAuthnName is what the passkey is listed as in the OS credential manager.
// There is no username in this system, so the display name serves for both.
func (w webauthnUser) WebAuthnName() string { return w.u.DisplayName }

// WebAuthnDisplayName is the human-facing name in the passkey prompt.
func (w webauthnUser) WebAuthnDisplayName() string { return w.u.DisplayName }

// WebAuthnCredentials lists the account's registered passkeys.
func (w webauthnUser) WebAuthnCredentials() []lib.Credential {
	out := make([]lib.Credential, 0, len(w.u.Credentials))
	for _, c := range w.u.Credentials {
		transports := make([]protocol.AuthenticatorTransport, 0, len(c.Transports))
		for _, t := range c.Transports {
			transports = append(transports, protocol.AuthenticatorTransport(t))
		}
		out = append(out, lib.Credential{
			ID:              c.ID,
			PublicKey:       c.PublicKey,
			AttestationType: c.AttestationType,
			Transport:       transports,
			Flags: lib.CredentialFlags{
				BackupEligible: c.BackupEligible,
				BackupState:    c.BackupState,
			},
			Authenticator: lib.Authenticator{
				AAGUID:    c.AAGUID,
				SignCount: c.SignCount,
			},
		})
	}
	return out
}

// marshalPair renders the browser options and the ceremony state as the two
// opaque byte slices the port trades in.
func marshalPair(options any, session *lib.SessionData) ([]byte, []byte, error) {
	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return nil, nil, types.WrapServerError(err, "encode ceremony options")
	}
	stateJSON, err := json.Marshal(session)
	if err != nil {
		return nil, nil, types.WrapServerError(err, "encode ceremony state")
	}
	return optionsJSON, stateJSON, nil
}

// unmarshalState decodes ceremony state that came back from a sealed cookie.
// It is signed, so corruption here means a bug or a forgery, never ordinary
// client input.
func unmarshalState(state []byte) (lib.SessionData, error) {
	var session lib.SessionData
	if err := json.Unmarshal(state, &session); err != nil {
		return lib.SessionData{}, types.NewUnauthenticatedError("ceremony state is not readable")
	}
	return session, nil
}

// protocolReason extracts the library's short reason for a parse failure. The
// full protocol.Error carries developer detail we do not want on the wire.
func protocolReason(err error) string {
	var protoErr *protocol.Error
	if !errors.As(err, &protoErr) || protoErr.Details == "" {
		return "malformed response"
	}
	return protoErr.Details
}

// Compile-time proof that this adapter satisfies the port.
var _ domain.Ceremony = (*Ceremony)(nil)
