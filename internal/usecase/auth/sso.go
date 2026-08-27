package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"slices"
	"strings"
	"time"

	domain "github.com/promix1722/easydnd/internal/domain/auth"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// randomBytes is the entropy behind the state, the nonce and the PKCE
// verifier. Base64url'd, 32 bytes becomes 43 characters, which is exactly the
// minimum RFC 7636 allows for a code verifier and far past guessing for the
// other two.
const randomBytes = 32

// DefaultReturnTo is where a sign-in lands when nothing better was asked for.
const DefaultReturnTo = "/"

// errStartUnavailable is what an empty AuthCodeURL means. The port returns no
// error of its own, so this is the cause the usecase wraps to say the provider
// could not be reached.
var errStartUnavailable = errors.New("the provider could not be reached")

// flight is the in-flight SSO state, sealed into a cookie between the redirect
// out and the callback back.
//
// It is the SSO counterpart of pending, and like pending it means no
// server-side map has to survive between two requests that may not even reach
// the same process. Everything in it is chosen by us and sealed, so the
// callback can trust every field -- which is what makes it safe to carry the
// link target and the return path here rather than in the query string.
type flight struct {
	Kind     string `json:"knd"`
	Provider string `json:"prv"`
	State    string `json:"st"`
	Nonce    string `json:"nc"`
	Verifier string `json:"vf"`
	ReturnTo string `json:"rt,omitempty"`
	// LinkTo names the account this attempt attaches an identity to. Empty
	// means an ordinary sign-in. A link is a different operation from a
	// sign-in and must be decided when the flow starts -- deciding it at the
	// callback, from whoever happens to be signed in then, would let a stray
	// sign-in silently absorb somebody's Google account.
	LinkTo string `json:"lnk,omitempty"`
}

// Providers lists the external providers this deployment can offer, sorted so
// the client renders them in a stable order.
func (s *Service) Providers() []user.Provider {
	out := make([]user.Provider, 0, len(s.federations))
	for provider := range s.federations {
		out = append(out, provider)
	}
	slices.Sort(out)
	return out
}

// StartSSO begins a federated sign-in and returns where to send the browser.
//
// linkTo is nil for an ordinary sign-in, or names the account to attach the
// resulting identity to.
func (s *Service) StartSSO(
	_ context.Context,
	provider user.Provider,
	returnTo string,
	linkTo *user.ID,
) (redirect, sealed string, err error) {
	federation, err := s.federation(provider)
	if err != nil {
		return "", "", err
	}

	state, err := randomToken()
	if err != nil {
		return "", "", err
	}
	nonce, err := randomToken()
	if err != nil {
		return "", "", err
	}
	verifier, err := randomToken()
	if err != nil {
		return "", "", err
	}

	f := flight{
		Kind:     kindSSO,
		Provider: string(provider),
		State:    state,
		Nonce:    nonce,
		Verifier: verifier,
		ReturnTo: SafeReturnTo(returnTo),
	}
	if linkTo != nil {
		f.LinkTo = string(*linkTo)
	}

	sealed, err = s.sealFlight(f)
	if err != nil {
		return "", "", err
	}

	redirect = federation.AuthCodeURL(state, nonce, verifier)
	if redirect == "" {
		// The port cannot report an error, so an empty URL is how an adapter
		// says it could not reach its issuer. Sending the browser to "" would
		// reload our own page and look like the button did nothing.
		return "", "", types.WrapServerError(
			errStartUnavailable, "start %s sign-in", provider)
	}
	return redirect, sealed, nil
}

// FinishSSO completes a federated sign-in or link.
//
// sessionUserID is whoever the session cookie currently identifies, or empty.
// It is only consulted for a link: the callback itself cannot be guarded,
// because it is the request that establishes a session in the first place.
func (s *Service) FinishSSO(
	ctx context.Context,
	provider user.Provider,
	sealed, state, code string,
	sessionUserID user.ID,
) (account user.User, token, returnTo string, err error) {
	federation, err := s.federation(provider)
	if err != nil {
		return user.User{}, "", "", err
	}

	f, err := s.openFlight(sealed)
	if err != nil {
		return user.User{}, "", "", err
	}

	// Resolved before anything can fail, and returned alongside every error
	// below: a failed link should land back on the page it was started from,
	// not on the party list. It is sanitised here rather than trusted, even
	// though we sealed it -- see safeReturnTo.
	returnTo = SafeReturnTo(f.ReturnTo)

	if f.Provider != string(provider) {
		return user.User{}, "", returnTo, types.NewUnauthenticatedError(
			"this sign-in was started with a different provider")
	}
	// Constant time because a comparison that returns early leaks how much of
	// the state a guess got right.
	if subtle.ConstantTimeCompare([]byte(f.State), []byte(state)) != 1 {
		return user.User{}, "", returnTo, types.NewUnauthenticatedError(
			"sign-in did not match the request that started it")
	}
	if code == "" {
		return user.User{}, "", returnTo, types.NewValidationError("no authorization code was returned")
	}

	identity, err := federation.Exchange(ctx, code, f.Nonce, f.Verifier)
	if err != nil {
		return user.User{}, "", returnTo, err
	}
	identity.Provider = provider
	now := s.now()
	identity.LastUsedAt = now

	if f.LinkTo != "" {
		account, err = s.link(ctx, user.ID(f.LinkTo), sessionUserID, identity, now)
	} else {
		account, err = s.signIn(ctx, identity, now)
	}
	if err != nil {
		return user.User{}, "", returnTo, err
	}

	token, err = s.issue(account.ID)
	if err != nil {
		return user.User{}, "", returnTo, err
	}
	return account, token, returnTo, nil
}

// signIn resolves an identity to an account, creating one the first time.
//
// It never matches on email. An address can be released by one person and
// handed to another, so keying on one would eventually sign a stranger into
// somebody else's party -- and a passkey-only account has no email to match
// against in the first place. Attaching a provider to an existing account is
// always the deliberate act in link, never a guess made here.
func (s *Service) signIn(ctx context.Context, identity user.Identity, now time.Time) (user.User, error) {
	found, err := s.repo.ByIdentity(ctx, identity.Provider, identity.Subject)
	switch {
	case err == nil:
		if err := s.repo.TouchIdentity(ctx, found.ID, identity); err != nil {
			return user.User{}, err
		}
		s.log.Info("account signed in", "user_id", found.ID, "provider", identity.Provider)
		return s.repo.ByID(ctx, found.ID)
	case !types.IsNotFound(err):
		return user.User{}, err
	}

	id, err := newUserID()
	if err != nil {
		return user.User{}, err
	}
	identity.CreatedAt = now

	account := user.User{
		ID:          id,
		DisplayName: displayNameFor(identity),
		CreatedAt:   now,
		Identities:  []user.Identity{identity},
	}
	if err := s.repo.Create(ctx, account); err != nil {
		return user.User{}, err
	}

	s.log.Info("account registered", "user_id", account.ID, "provider", identity.Provider)
	return account, nil
}

// link attaches an identity to an account that already exists.
func (s *Service) link(
	ctx context.Context,
	target, sessionUserID user.ID,
	identity user.Identity,
	now time.Time,
) (user.User, error) {
	// Defence in depth. The target came out of a cookie we sealed and only
	// ever issue to a signed-in caller, so it is already trustworthy; this
	// costs one comparison and means a link cannot complete for anyone but
	// the person sitting in front of the account.
	if sessionUserID != target {
		return user.User{}, types.NewUnauthenticatedError(
			"sign in again before connecting an account")
	}

	// Report the conflict as a conflict rather than silently moving the
	// identity: a subject that already signs somebody in elsewhere is either
	// a mistake or an attempt to hijack that account, and neither should
	// resolve quietly.
	switch existing, err := s.repo.ByIdentity(ctx, identity.Provider, identity.Subject); {
	case err == nil && existing.ID == target:
		if err := s.repo.TouchIdentity(ctx, target, identity); err != nil {
			return user.User{}, err
		}
		return s.repo.ByID(ctx, target)
	case err == nil:
		return user.User{}, types.NewValidationError(
			"that %s account is already connected to a different easydnd account", identity.Provider)
	case !types.IsNotFound(err):
		return user.User{}, err
	}

	identity.CreatedAt = now
	if err := s.repo.AddIdentity(ctx, target, identity); err != nil {
		return user.User{}, err
	}

	s.log.Info("identity linked", "user_id", target, "provider", identity.Provider)
	return s.repo.ByID(ctx, target)
}

// Unlink detaches an external identity from an account.
func (s *Service) Unlink(
	ctx context.Context,
	id user.ID,
	provider user.Provider,
	subject string,
) (user.User, error) {
	current, err := s.repo.ByID(ctx, id)
	if err != nil {
		return user.User{}, s.asUnauthenticated(err)
	}

	// The rule that spans both kinds of proof, and the reason it lives here
	// rather than in the repository: an account with no passkey and no
	// identity can never be signed into again, and nothing in this design can
	// restore it. Refusing is the only honest answer.
	if current.SignInMethods() <= 1 {
		return user.User{}, types.NewValidationError(
			"this is the only way left to sign in to this account; add a passkey first")
	}

	if err := s.repo.RemoveIdentity(ctx, id, provider, subject); err != nil {
		return user.User{}, err
	}

	s.log.Info("identity unlinked", "user_id", id, "provider", provider)
	return s.repo.ByID(ctx, id)
}

// federation looks up a configured provider.
func (s *Service) federation(provider user.Provider) (domain.Federation, error) {
	federation, ok := s.federations[provider]
	if !ok {
		// NotFound rather than NotImplemented: from the client's side an
		// unconfigured provider and a misspelt one are the same thing, and
		// distinguishing them would report which providers a deployment
		// could have had.
		return nil, types.NewNotFoundError("unknown sign-in provider %q", provider)
	}
	return federation, nil
}

func (s *Service) sealFlight(f flight) (string, error) {
	encoded, err := json.Marshal(f)
	if err != nil {
		return "", types.WrapServerError(err, "encode sign-in envelope")
	}
	return s.signer.Seal(encoded, s.ceremonyTTL, s.now())
}

func (s *Service) openFlight(sealed string) (flight, error) {
	if sealed == "" {
		return flight{}, types.NewValidationError("no sign-in is in progress").Because("auth.noCeremony")
	}
	payload, err := s.signer.Open(sealed, s.now())
	if err != nil {
		return flight{}, err
	}
	var f flight
	if err := json.Unmarshal(payload, &f); err != nil {
		return flight{}, types.NewUnauthenticatedError("sign-in envelope is not readable")
	}
	if f.Kind != kindSSO {
		return flight{}, types.NewUnauthenticatedError("this is not a federated sign-in")
	}
	if f.State == "" || f.Nonce == "" || f.Verifier == "" {
		return flight{}, types.NewUnauthenticatedError("sign-in envelope is incomplete")
	}
	return f, nil
}

// displayNameFor picks what to call somebody who has just arrived.
//
// The provider's name is the good answer; the email's local part is a
// serviceable one; the constant is what stops an account existing with no
// label at all. The result goes through the same normalisation a typed name
// does, because a provider is just as capable of returning 200 characters of
// whitespace as a text input is.
func displayNameFor(identity user.Identity) string {
	candidates := []string{identity.DisplayName, identity.Email}
	if local, _, ok := strings.Cut(identity.Email, "@"); ok {
		candidates = []string{identity.DisplayName, local, identity.Email}
	}
	for _, candidate := range candidates {
		if name, err := normalizeDisplayName(candidate); err == nil {
			return name
		}
	}
	return "Adventurer"
}

// SafeReturnTo reduces a requested landing place to one that cannot leave this
// site.
//
// Exported because the HTTP layer needs it too: a failure that never got as
// far as opening the flight can only fall back on the raw query parameter.
//
// Anything absolute, protocol-relative or otherwise capable of naming a host
// is discarded rather than repaired: a redirect target is the classic way an
// otherwise sound sign-in becomes a phishing hop, and the only safe handling
// of an unexpected one is to ignore it.
//
// The check is structural rather than a list of forbidden characters, because
// a list is exactly what this got wrong once. A tab is not a newline and was
// not on it -- but the URL parser every browser implements strips tab, CR and
// LF from a URL *before* parsing it, so "/\t/evil.com" survives a naive
// filter, travels through Go's Location header untouched (net/http rewrites
// only CR and LF), and resolves in the browser as "//evil.com". Parsing and
// then insisting on no scheme and no host cannot be outflanked that way:
// url.Parse rejects control characters outright, and reports the host of a
// protocol-relative path as a host.
func SafeReturnTo(path string) string {
	if path == "" || !strings.HasPrefix(path, "/") {
		return DefaultReturnTo
	}
	// A backslash is checked separately because url.Parse treats it as an
	// ordinary path character while several browsers normalise it to "/",
	// which would turn "/\evil.com" into a protocol-relative URL after this
	// function had already approved it.
	if strings.Contains(path, "\\") {
		return DefaultReturnTo
	}
	// A fragment is never wanted here and would break the one thing appended
	// to this path afterwards: "?auth_error=..." landing after a "#" is part
	// of the fragment, so the client would never see the message.
	if strings.Contains(path, "#") {
		return DefaultReturnTo
	}

	parsed, err := url.Parse(path)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || parsed.Opaque != "" {
		return DefaultReturnTo
	}
	if !strings.HasPrefix(parsed.EscapedPath(), "/") {
		return DefaultReturnTo
	}
	return path
}

func randomToken() (string, error) {
	raw := make([]byte, randomBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", types.WrapServerError(err, "generate sign-in token")
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
