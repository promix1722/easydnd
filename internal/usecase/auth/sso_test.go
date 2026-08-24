package auth

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	domain "github.com/promix1722/easydnd/internal/domain/auth"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// fakeFederation stands in for the OIDC adapter, for the same reason
// fakeCeremony stands in for the WebAuthn one: the real thing needs a live
// identity provider, and the port is what makes it replaceable.
type fakeFederation struct {
	lastState    string
	lastNonce    string
	lastVerifier string

	identity    user.Identity
	exchangeErr error
	unreachable bool

	// gotNonce and gotVerifier record what Exchange was handed, so a test can
	// prove the values from the sealed flight are the ones that travel.
	gotNonce    string
	gotVerifier string
}

func (f *fakeFederation) AuthCodeURL(state, nonce, verifier string) string {
	f.lastState, f.lastNonce, f.lastVerifier = state, nonce, verifier
	if f.unreachable {
		return ""
	}
	return "https://idp.test/authorize?state=" + state
}

func (f *fakeFederation) Exchange(_ context.Context, _, nonce, verifier string) (user.Identity, error) {
	f.gotNonce, f.gotVerifier = nonce, verifier
	if f.exchangeErr != nil {
		return user.Identity{}, f.exchangeErr
	}
	identity := f.identity
	if identity.Subject == "" {
		identity.Subject = "sub-1"
	}
	return identity, nil
}

func newSSOService(t *testing.T, federation *fakeFederation) (*Service, *memory.UserRepository) {
	t.Helper()
	repo := memory.NewUserRepository()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(repo, &fakeCeremony{state: []byte("state")}, newFakeSigner(),
		map[user.Provider]domain.Federation{user.ProviderGoogle: federation},
		Config{SessionTTL: time.Hour, CeremonyTTL: 5 * time.Minute}, log)
	return svc, repo
}

// start runs the redirect half and returns the sealed flight and the state the
// provider was given -- which is what a real callback would carry back.
func start(t *testing.T, svc *Service, f *fakeFederation, returnTo string, linkTo *user.ID) (flight, state string) {
	t.Helper()
	_, sealed, err := svc.StartSSO(context.Background(), user.ProviderGoogle, returnTo, linkTo)
	if err != nil {
		t.Fatalf("StartSSO: %v", err)
	}
	return sealed, f.lastState
}

func TestStartSSOSendsPKCEAndBindsTheAttempt(t *testing.T) {
	federation := &fakeFederation{}
	svc, _ := newSSOService(t, federation)

	redirect, sealed, err := svc.StartSSO(context.Background(), user.ProviderGoogle, "/characters", nil)
	if err != nil {
		t.Fatalf("StartSSO: %v", err)
	}
	if redirect == "" || sealed == "" {
		t.Fatal("StartSSO returned an empty redirect or flight")
	}
	if federation.lastState == "" || federation.lastNonce == "" || federation.lastVerifier == "" {
		t.Fatalf("state, nonce and verifier must all be minted: %+v", federation)
	}
	if federation.lastState == federation.lastNonce {
		t.Fatal("state and nonce are the same value; they bind different things")
	}

	// The same verifier must reach both halves. The port deliberately carries
	// the verifier rather than a challenge derived from it, so that deriving
	// it happens exactly once, in the adapter, beside the library that checks
	// it -- see internal/adapter/oidc for the derivation itself.
	if _, _, _, err := svc.FinishSSO(context.Background(), user.ProviderGoogle,
		sealed, federation.lastState, "code", ""); err != nil {
		t.Fatalf("FinishSSO: %v", err)
	}
	if federation.gotVerifier != federation.lastVerifier {
		t.Fatalf("Exchange got verifier %q, want the one the redirect was built from %q",
			federation.gotVerifier, federation.lastVerifier)
	}
	if federation.gotNonce != federation.lastNonce {
		t.Fatalf("Exchange got nonce %q, want the one sent %q", federation.gotNonce, federation.lastNonce)
	}

	// RFC 7636 puts the verifier between 43 and 128 characters; ours is 32
	// random bytes base64url'd, which is exactly 43.
	if n := len(federation.lastVerifier); n < 43 || n > 128 {
		t.Errorf("verifier is %d characters, outside RFC 7636's 43..128", n)
	}
}

func TestStartSSORejectsAnUnknownProvider(t *testing.T) {
	svc, _ := newSSOService(t, &fakeFederation{})

	if _, _, err := svc.StartSSO(context.Background(), "nope", "/", nil); !types.IsNotFound(err) {
		t.Fatalf("unknown provider: got %v, want NotFound", err)
	}
}

// An adapter that cannot reach its issuer returns an empty URL. Redirecting to
// "" would reload our own page and look like the button did nothing.
func TestStartSSOFailsWhenTheProviderIsUnreachable(t *testing.T) {
	svc, _ := newSSOService(t, &fakeFederation{unreachable: true})

	if _, _, err := svc.StartSSO(context.Background(), user.ProviderGoogle, "/", nil); err == nil {
		t.Fatal("StartSSO succeeded with an unreachable provider")
	}
}

func TestFirstSSOSignInCreatesAnAccount(t *testing.T) {
	federation := &fakeFederation{identity: user.Identity{
		Subject: "google-1", Email: "rogue@example.test", EmailVerified: true, DisplayName: "Rogue",
	}}
	svc, repo := newSSOService(t, federation)

	flight, state := start(t, svc, federation, "/characters", nil)
	account, token, returnTo, err := svc.FinishSSO(
		context.Background(), user.ProviderGoogle, flight, state, "code", "")
	if err != nil {
		t.Fatalf("FinishSSO: %v", err)
	}
	if token == "" {
		t.Fatal("no session token issued")
	}
	if returnTo != "/characters" {
		t.Fatalf("returnTo %q, want /characters", returnTo)
	}
	if account.DisplayName != "Rogue" {
		t.Fatalf("display name %q, want Rogue", account.DisplayName)
	}
	if len(account.Identities) != 1 || account.Identities[0].Subject != "google-1" {
		t.Fatalf("account identities: %+v", account.Identities)
	}

	stored, err := repo.ByIdentity(context.Background(), user.ProviderGoogle, "google-1")
	if err != nil || stored.ID != account.ID {
		t.Fatalf("account not stored under its identity: %q (%v)", stored.ID, err)
	}
}

// The whole point of keying on the subject: coming back must land on the same
// account, not mint a second one.
func TestReturningSSOSignInResolvesTheSameAccount(t *testing.T) {
	federation := &fakeFederation{identity: user.Identity{Subject: "google-1", DisplayName: "Rogue"}}
	svc, _ := newSSOService(t, federation)

	flight, state := start(t, svc, federation, "/", nil)
	first, _, _, err := svc.FinishSSO(context.Background(), user.ProviderGoogle, flight, state, "code", "")
	if err != nil {
		t.Fatalf("first sign-in: %v", err)
	}

	// Same subject, a different name upstream: the account keeps its own.
	federation.identity.DisplayName = "Renamed Upstream"
	flight, state = start(t, svc, federation, "/", nil)
	second, _, _, err := svc.FinishSSO(context.Background(), user.ProviderGoogle, flight, state, "code", "")
	if err != nil {
		t.Fatalf("second sign-in: %v", err)
	}

	if second.ID != first.ID {
		t.Fatalf("second sign-in created account %q, want %q", second.ID, first.ID)
	}
	if second.DisplayName != "Rogue" {
		t.Fatalf("display name was overwritten to %q", second.DisplayName)
	}
}

// A different person at the same provider is a different account. This is the
// test that would fail if anything ever started matching on email.
func TestSSOSignInDoesNotMatchOnEmail(t *testing.T) {
	federation := &fakeFederation{identity: user.Identity{Subject: "google-1", Email: "shared@example.test"}}
	svc, _ := newSSOService(t, federation)

	flight, state := start(t, svc, federation, "/", nil)
	first, _, _, err := svc.FinishSSO(context.Background(), user.ProviderGoogle, flight, state, "code", "")
	if err != nil {
		t.Fatalf("first sign-in: %v", err)
	}

	// Same address, different subject -- an address released and reassigned.
	federation.identity = user.Identity{Subject: "google-2", Email: "shared@example.test"}
	flight, state = start(t, svc, federation, "/", nil)
	second, _, _, err := svc.FinishSSO(context.Background(), user.ProviderGoogle, flight, state, "code", "")
	if err != nil {
		t.Fatalf("second sign-in: %v", err)
	}

	if second.ID == first.ID {
		t.Fatal("a reassigned email address signed somebody into another person's account")
	}
}

func TestFinishSSORejectsAMismatchedState(t *testing.T) {
	federation := &fakeFederation{}
	svc, _ := newSSOService(t, federation)

	flight, _ := start(t, svc, federation, "/", nil)
	_, _, _, err := svc.FinishSSO(context.Background(), user.ProviderGoogle, flight, "not-the-state", "code", "")
	if !types.IsUnauthenticated(err) {
		t.Fatalf("mismatched state: got %v, want Unauthenticated", err)
	}
}

func TestFinishSSORequiresAFlight(t *testing.T) {
	svc, _ := newSSOService(t, &fakeFederation{})

	if _, _, _, err := svc.FinishSSO(context.Background(), user.ProviderGoogle, "", "state", "code", ""); err == nil {
		t.Fatal("FinishSSO succeeded with no flight cookie")
	}
}

func TestFinishSSORequiresACode(t *testing.T) {
	federation := &fakeFederation{}
	svc, _ := newSSOService(t, federation)

	flight, state := start(t, svc, federation, "/", nil)
	if _, _, _, err := svc.FinishSSO(context.Background(), user.ProviderGoogle, flight, state, "", ""); err == nil {
		t.Fatal("FinishSSO succeeded with no authorization code")
	}
}

// The flight names its provider, so a flight started for one cannot be
// completed against another.
func TestFinishSSORejectsAFlightFromAnotherProvider(t *testing.T) {
	federation := &fakeFederation{}
	other := &fakeFederation{}
	svc, _ := newSSOService(t, federation)
	svc.federations["other"] = other

	flight, state := start(t, svc, federation, "/", nil)
	_, _, _, err := svc.FinishSSO(context.Background(), "other", flight, state, "code", "")
	if !types.IsUnauthenticated(err) {
		t.Fatalf("cross-provider flight: got %v, want Unauthenticated", err)
	}
}

// Both flows seal a cookie with the same Signer, so each must refuse the
// other's envelope rather than depending on how JSON happens to decode.
func TestTheTwoSealedEnvelopesRefuseEachOther(t *testing.T) {
	federation := &fakeFederation{}
	svc, _ := newSSOService(t, federation)
	ctx := context.Background()

	// A WebAuthn ceremony envelope fed to the SSO callback.
	_, ceremony, err := svc.BeginLogin(ctx)
	if err != nil {
		t.Fatalf("BeginLogin: %v", err)
	}
	if _, _, _, err := svc.FinishSSO(ctx, user.ProviderGoogle, ceremony, "state", "code", ""); err == nil {
		t.Fatal("a passkey ceremony envelope was accepted as a federated sign-in")
	}

	// And an SSO flight fed to the passkey finish endpoints.
	flight, _ := start(t, svc, federation, "/", nil)
	if _, _, err := svc.FinishLogin(ctx, flight, []byte(`{}`)); err == nil {
		t.Fatal("an SSO flight was accepted as a passkey ceremony")
	}
	if _, _, err := svc.FinishRegistration(ctx, flight, []byte(`{}`)); err == nil {
		t.Fatal("an SSO flight was accepted as a passkey registration")
	}
}

func TestFinishSSOPropagatesAnExchangeFailure(t *testing.T) {
	federation := &fakeFederation{exchangeErr: types.NewUnauthenticatedError("bad token")}
	svc, repo := newSSOService(t, federation)

	flight, state := start(t, svc, federation, "/", nil)
	if _, _, _, err := svc.FinishSSO(context.Background(), user.ProviderGoogle, flight, state, "code", ""); err == nil {
		t.Fatal("FinishSSO succeeded despite a failed exchange")
	}
	if _, err := repo.ByIdentity(context.Background(), user.ProviderGoogle, "sub-1"); !types.IsNotFound(err) {
		t.Fatalf("a failed exchange left an account behind: %v", err)
	}
}

// --- linking ---

func TestLinkAttachesAnIdentityToTheSignedInAccount(t *testing.T) {
	federation := &fakeFederation{identity: user.Identity{Subject: "google-1", Email: "a@example.test"}}
	svc, repo := newSSOService(t, federation)
	ctx := context.Background()

	existing := user.User{ID: "acct-1", DisplayName: "Passkey Person",
		Credentials: []user.Credential{{ID: []byte("c1")}}}
	if err := repo.Create(ctx, existing); err != nil {
		t.Fatalf("Create: %v", err)
	}

	flight, state := start(t, svc, federation, "/account", &existing.ID)
	account, _, returnTo, err := svc.FinishSSO(ctx, user.ProviderGoogle, flight, state, "code", existing.ID)
	if err != nil {
		t.Fatalf("FinishSSO: %v", err)
	}
	if account.ID != existing.ID {
		t.Fatalf("linked into account %q, want %q", account.ID, existing.ID)
	}
	if account.SignInMethods() != 2 {
		t.Fatalf("account has %d sign-in methods, want 2", account.SignInMethods())
	}
	if returnTo != "/account" {
		t.Fatalf("returnTo %q, want /account", returnTo)
	}
	if account.DisplayName != "Passkey Person" {
		t.Fatalf("linking renamed the account to %q", account.DisplayName)
	}
}

// A link must not complete for anyone but the person sitting in front of the
// account, even though the target came out of a cookie we sealed ourselves.
func TestLinkRequiresTheMatchingSession(t *testing.T) {
	federation := &fakeFederation{identity: user.Identity{Subject: "google-1"}}
	svc, repo := newSSOService(t, federation)
	ctx := context.Background()

	target := user.ID("acct-1")
	if err := repo.Create(ctx, user.User{ID: target, Credentials: []user.Credential{{ID: []byte("c1")}}}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	flight, state := start(t, svc, federation, "/", &target)
	for _, session := range []user.ID{"", "somebody-else"} {
		_, _, _, err := svc.FinishSSO(ctx, user.ProviderGoogle, flight, state, "code", session)
		if !types.IsUnauthenticated(err) {
			t.Fatalf("session %q: got %v, want Unauthenticated", session, err)
		}
	}
}

// Moving a subject between accounts must be an explicit unlink and relink,
// never a side effect of clicking Connect on the wrong account.
func TestLinkRefusesAnIdentityHeldByAnotherAccount(t *testing.T) {
	federation := &fakeFederation{identity: user.Identity{Subject: "google-1"}}
	svc, repo := newSSOService(t, federation)
	ctx := context.Background()

	// First account signs in with Google and takes the subject.
	flight, state := start(t, svc, federation, "/", nil)
	owner, _, _, err := svc.FinishSSO(ctx, user.ProviderGoogle, flight, state, "code", "")
	if err != nil {
		t.Fatalf("first sign-in: %v", err)
	}

	// A second, passkey-only account tries to connect the same Google account.
	other := user.ID("acct-2")
	if err := repo.Create(ctx, user.User{ID: other, Credentials: []user.Credential{{ID: []byte("c2")}}}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	flight, state = start(t, svc, federation, "/", &other)
	if _, _, _, err := svc.FinishSSO(ctx, user.ProviderGoogle, flight, state, "code", other); err == nil {
		t.Fatal("linking an identity held elsewhere succeeded")
	}

	// And the original owner keeps it.
	stored, err := repo.ByIdentity(ctx, user.ProviderGoogle, "google-1")
	if err != nil || stored.ID != owner.ID {
		t.Fatalf("identity moved to %q (%v), want %q", stored.ID, err, owner.ID)
	}
}

// Re-linking what an account already holds is a no-op, not an error: clicking
// Connect twice should not read as a failure.
func TestLinkIsIdempotentForTheSameAccount(t *testing.T) {
	federation := &fakeFederation{identity: user.Identity{Subject: "google-1"}}
	svc, _ := newSSOService(t, federation)
	ctx := context.Background()

	flight, state := start(t, svc, federation, "/", nil)
	account, _, _, err := svc.FinishSSO(ctx, user.ProviderGoogle, flight, state, "code", "")
	if err != nil {
		t.Fatalf("sign-in: %v", err)
	}

	flight, state = start(t, svc, federation, "/", &account.ID)
	again, _, _, err := svc.FinishSSO(ctx, user.ProviderGoogle, flight, state, "code", account.ID)
	if err != nil {
		t.Fatalf("re-linking the same identity: %v", err)
	}
	if len(again.Identities) != 1 {
		t.Fatalf("account holds %d identities, want 1", len(again.Identities))
	}
}

// --- unlinking ---

func TestUnlinkRefusesToRemoveTheLastWayIn(t *testing.T) {
	federation := &fakeFederation{identity: user.Identity{Subject: "google-1"}}
	svc, _ := newSSOService(t, federation)
	ctx := context.Background()

	flight, state := start(t, svc, federation, "/", nil)
	account, _, _, err := svc.FinishSSO(ctx, user.ProviderGoogle, flight, state, "code", "")
	if err != nil {
		t.Fatalf("sign-in: %v", err)
	}

	// One identity, no passkey: removing it would strand the account forever.
	if _, err := svc.Unlink(ctx, account.ID, user.ProviderGoogle, "google-1"); err == nil {
		t.Fatal("unlinking the only sign-in method succeeded")
	}
	still, err := svc.repo.ByIdentity(ctx, user.ProviderGoogle, "google-1")
	if err != nil || still.ID != account.ID {
		t.Fatalf("the identity was removed anyway: %q (%v)", still.ID, err)
	}
}

func TestUnlinkSucceedsWhenAnotherMethodRemains(t *testing.T) {
	federation := &fakeFederation{identity: user.Identity{Subject: "google-1"}}
	svc, repo := newSSOService(t, federation)
	ctx := context.Background()

	target := user.ID("acct-1")
	if err := repo.Create(ctx, user.User{ID: target, Credentials: []user.Credential{{ID: []byte("c1")}}}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	flight, state := start(t, svc, federation, "/", &target)
	if _, _, _, err := svc.FinishSSO(ctx, user.ProviderGoogle, flight, state, "code", target); err != nil {
		t.Fatalf("link: %v", err)
	}

	updated, err := svc.Unlink(ctx, target, user.ProviderGoogle, "google-1")
	if err != nil {
		t.Fatalf("Unlink: %v", err)
	}
	if len(updated.Identities) != 0 || updated.SignInMethods() != 1 {
		t.Fatalf("after unlink: %d identities, %d methods", len(updated.Identities), updated.SignInMethods())
	}
}

// --- return paths ---

// A redirect target is the classic way a sound sign-in becomes a phishing hop.
func TestReturnToCannotLeaveTheSite(t *testing.T) {
	hostile := []string{
		"https://evil.test/steal",
		"//evil.test/steal",
		"/\\evil.test",
		"http://evil.test",
		"javascript:alert(1)",
		"characters",
		"/ok\r\nSet-Cookie: x=1",

		// Every browser's URL parser strips tab, CR and LF *before* parsing,
		// so each of these resolves as "//evil.test" -- while net/http copies
		// a tab into the Location header untouched. A character blocklist
		// missed the tab exactly once; these pin the whole class.
		"/\t/evil.test",
		"/\t\t//evil.test",
		"/\n/evil.test",
		"/\r/evil.test",
		"/\v/evil.test",
		"/\f/evil.test",
		"/\x00/evil.test",
		"/\\\\evil.test",
		"/path\\..\\evil",
	}
	for _, path := range hostile {
		if got := SafeReturnTo(path); got != DefaultReturnTo {
			t.Errorf("SafeReturnTo(%q) = %q, want %q", path, got, DefaultReturnTo)
		}
	}

	for _, path := range []string{"/", "/characters", "/characters/chr_000001/build?step=2"} {
		if got := SafeReturnTo(path); got != path {
			t.Errorf("SafeReturnTo(%q) = %q, want it unchanged", path, got)
		}
	}
}

// The sealed flight is the only source of the return path, so a hostile one
// cannot reach the browser even if it is somehow sealed.
func TestFinishSSOSanitisesTheSealedReturnPath(t *testing.T) {
	federation := &fakeFederation{}
	svc, _ := newSSOService(t, federation)

	flight, state := start(t, svc, federation, "https://evil.test", nil)
	_, _, returnTo, err := svc.FinishSSO(context.Background(), user.ProviderGoogle, flight, state, "code", "")
	if err != nil {
		t.Fatalf("FinishSSO: %v", err)
	}
	if returnTo != DefaultReturnTo {
		t.Fatalf("returnTo %q, want %q", returnTo, DefaultReturnTo)
	}
}

// --- display names ---

func TestDisplayNameFallsBackThroughTheProvidersClaims(t *testing.T) {
	cases := []struct {
		identity user.Identity
		want     string
	}{
		{user.Identity{DisplayName: "Rogue", Email: "r@example.test"}, "Rogue"},
		{user.Identity{Email: "rogue@example.test"}, "rogue"},
		{user.Identity{DisplayName: "   ", Email: "rogue@example.test"}, "rogue"},
		{user.Identity{}, "Adventurer"},
		{user.Identity{DisplayName: strings.Repeat("x", 200)}, "Adventurer"},
	}
	for _, c := range cases {
		if got := displayNameFor(c.identity); got != c.want {
			t.Errorf("displayNameFor(%+v) = %q, want %q", c.identity, got, c.want)
		}
	}
}

func TestProvidersListsWhatIsConfigured(t *testing.T) {
	svc, _ := newSSOService(t, &fakeFederation{})
	if got := svc.Providers(); len(got) != 1 || got[0] != user.ProviderGoogle {
		t.Fatalf("Providers() = %v, want [google]", got)
	}

	bare := NewService(memory.NewUserRepository(), &fakeCeremony{}, newFakeSigner(), nil,
		Config{SessionTTL: time.Hour, CeremonyTTL: time.Minute},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	if got := bare.Providers(); len(got) != 0 {
		t.Fatalf("Providers() with none configured = %v, want empty", got)
	}
}
