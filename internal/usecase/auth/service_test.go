package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	domain "github.com/promix1722/easydnd/internal/domain/auth"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// fakeCeremony stands in for the WebAuthn adapter. The real one cannot be
// driven from a unit test -- it needs an authenticator -- so the service is
// tested against the port instead, which is the reason the port exists.
type fakeCeremony struct {
	state      []byte
	credential user.Credential
	loginID    user.ID
	loginErr   error
	finishErr  error
	// lookupWith records the arguments FinishLogin passed to the closure, so
	// the test can prove the repository is reached through it.
	lookupWith [2][]byte
	lookupErr  error
}

func (f *fakeCeremony) BeginRegistration(user.User) ([]byte, []byte, error) {
	return []byte(`{"publicKey":{}}`), f.state, nil
}

func (f *fakeCeremony) FinishRegistration(_ user.User, _, _ []byte) (user.Credential, error) {
	if f.finishErr != nil {
		return user.Credential{}, f.finishErr
	}
	return f.credential, nil
}

func (f *fakeCeremony) BeginLogin() ([]byte, []byte, error) {
	return []byte(`{"publicKey":{}}`), f.state, nil
}

func (f *fakeCeremony) FinishLogin(_, _ []byte, lookup domain.UserLookup) (user.ID, user.Credential, error) {
	if f.loginErr != nil {
		return "", user.Credential{}, f.loginErr
	}
	if _, err := lookup(f.lookupWith[0], f.lookupWith[1]); err != nil {
		f.lookupErr = err
		return "", user.Credential{}, err
	}
	return f.loginID, f.credential, nil
}

// fakeSigner seals by passing bytes through untouched, so a test can inspect
// and forge ceremony envelopes without reimplementing JWT.
type fakeSigner struct {
	sessions map[string]domain.Session
	expired  bool
}

func newFakeSigner() *fakeSigner {
	return &fakeSigner{sessions: map[string]domain.Session{}}
}

func (f *fakeSigner) SignSession(s domain.Session) (string, error) {
	token := "session-for-" + string(s.UserID)
	f.sessions[token] = s
	return token, nil
}

func (f *fakeSigner) VerifySession(token string, _ time.Time) (domain.Session, error) {
	s, ok := f.sessions[token]
	if !ok {
		return domain.Session{}, types.NewUnauthenticatedError("unknown token")
	}
	return s, nil
}

func (f *fakeSigner) Seal(payload []byte, _ time.Duration, _ time.Time) (string, error) {
	return string(payload), nil
}

func (f *fakeSigner) Open(token string, _ time.Time) ([]byte, error) {
	if f.expired {
		return nil, types.NewUnauthenticatedError("ceremony token has expired")
	}
	return []byte(token), nil
}

func newService(t *testing.T, ceremony *fakeCeremony, signer *fakeSigner) (*Service, *memory.UserRepository) {
	t.Helper()
	repo := memory.NewUserRepository()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewService(repo, ceremony, signer, nil, Config{
		SessionTTL: time.Hour,
		// Distinct from SessionTTL so a test can tell which one was applied.
		GuestSessionTTL: 15 * time.Minute,
		CeremonyTTL:     5 * time.Minute,
	}, log), repo
}

func credential(id string) user.Credential {
	return user.Credential{ID: []byte(id), PublicKey: []byte("pk-" + id)}
}

// Nobody names an account any more, so the thing worth pinning is that the
// server does: an account reaching the store with an empty display name would
// violate users_display_name_len, and an OS passkey prompt with a blank label
// is unusable long before that.
//
// The name is asserted exactly, because it is a fixed one now. A passkey
// labelled anything but the site it opens is the regression this catches.
func TestBeginRegistrationNamesTheAccount(t *testing.T) {
	ceremony := &fakeCeremony{state: []byte("state"), credential: credential("c1")}
	svc, repo := newService(t, ceremony, newFakeSigner())
	ctx := context.Background()

	_, sealed, err := svc.BeginRegistration(ctx)
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}
	account, _, err := svc.FinishRegistration(ctx, sealed, []byte("{}"))
	if err != nil {
		t.Fatalf("FinishRegistration: %v", err)
	}
	if account.DisplayName != PasskeyDisplayName {
		t.Errorf("DisplayName = %q, want %q", account.DisplayName, PasskeyDisplayName)
	}
	if _, err := repo.ByID(ctx, account.ID); err != nil {
		t.Fatalf("the account was not stored: %v", err)
	}
}

// TestPasskeyDisplayNameIsStorable is the only test guarding
// users_display_name_len, the CHECK on users.display_name bounding it to 1..64
// characters: the Postgres adapter tests need TEST_DATABASE_URL and are skipped
// by `make verify`, so a constant edited to something the column refuses would
// otherwise reach a real database first -- at the far end of a ceremony that
// has already prompted the authenticator.
func TestPasskeyDisplayNameIsStorable(t *testing.T) {
	got, err := normalizeDisplayName(PasskeyDisplayName)
	if err != nil {
		t.Fatalf("normalizeDisplayName(%q): %v", PasskeyDisplayName, err)
	}
	// Unchanged, not merely accepted: a name carrying stray whitespace would
	// still normalize, and the stored string would still not be the constant.
	if got != PasskeyDisplayName {
		t.Fatalf("normalizeDisplayName(%q) = %q, want it unchanged", PasskeyDisplayName, got)
	}
	if n := utf8.RuneCountInString(got); n < MinDisplayName || n > MaxDisplayName {
		t.Fatalf("%q is %d runes, outside %d..%d", got, n, MinDisplayName, MaxDisplayName)
	}
}

// An abandoned sign-up must leave nothing behind: begin writes no record, so
// the endpoint cannot be used to fill the store.
func TestBeginRegistrationStoresNothing(t *testing.T) {
	ceremony := &fakeCeremony{state: []byte("state"), credential: credential("c1")}
	svc, repo := newService(t, ceremony, newFakeSigner())
	ctx := context.Background()

	_, sealed, err := svc.BeginRegistration(ctx)
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}

	var envelope struct {
		User *struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.Unmarshal([]byte(sealed), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.User == nil {
		t.Fatal("the ceremony envelope carries no account")
	}
	if _, err := repo.ByID(ctx, user.ID(envelope.User.ID)); !types.IsNotFound(err) {
		t.Fatalf("err = %v, want *types.NotFoundError -- begin wrote a record", err)
	}
}

// A failed attestation must not create the account either.
func TestFinishRegistrationStoresNothingOnFailure(t *testing.T) {
	ceremony := &fakeCeremony{
		state:     []byte("state"),
		finishErr: types.NewValidationError("bad attestation"),
	}
	svc, repo := newService(t, ceremony, newFakeSigner())
	ctx := context.Background()

	_, sealed, err := svc.BeginRegistration(ctx)
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}
	if _, _, err := svc.FinishRegistration(ctx, sealed, []byte("{}")); err == nil {
		t.Fatal("FinishRegistration succeeded on a bad attestation")
	}

	if _, err := repo.ByCredentialID(ctx, []byte("c1")); !types.IsNotFound(err) {
		t.Fatalf("err = %v, want *types.NotFoundError", err)
	}
}

// A registration envelope must not be usable to finish a sign-in, and vice
// versa: they are sealed with the same key and only the payload distinguishes
// them.
func TestCeremonyEnvelopesAreNotInterchangeable(t *testing.T) {
	ceremony := &fakeCeremony{state: []byte("state"), credential: credential("c1")}
	svc, _ := newService(t, ceremony, newFakeSigner())
	ctx := context.Background()

	_, registration, err := svc.BeginRegistration(ctx)
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}
	if _, _, err := svc.FinishLogin(ctx, registration, []byte("{}")); err == nil {
		t.Fatal("a registration envelope completed a sign-in")
	}

	_, login, err := svc.BeginLogin(ctx)
	if err != nil {
		t.Fatalf("BeginLogin: %v", err)
	}
	if _, _, err := svc.FinishRegistration(ctx, login, []byte("{}")); err == nil {
		t.Fatal("a sign-in envelope completed a registration")
	}
}

func TestLoginRoundTrip(t *testing.T) {
	ceremony := &fakeCeremony{state: []byte("state"), credential: credential("c1")}
	svc, _ := newService(t, ceremony, newFakeSigner())
	ctx := context.Background()

	_, sealed, err := svc.BeginRegistration(ctx)
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}
	account, _, err := svc.FinishRegistration(ctx, sealed, []byte("{}"))
	if err != nil {
		t.Fatalf("FinishRegistration: %v", err)
	}

	ceremony.loginID = account.ID
	ceremony.lookupWith = [2][]byte{[]byte("c1"), []byte(account.ID)}
	ceremony.credential = user.Credential{ID: []byte("c1"), SignCount: 7}

	_, loginSealed, err := svc.BeginLogin(ctx)
	if err != nil {
		t.Fatalf("BeginLogin: %v", err)
	}
	got, token, err := svc.FinishLogin(ctx, loginSealed, []byte("{}"))
	if err != nil {
		t.Fatalf("FinishLogin: %v", err)
	}
	if got.ID != account.ID {
		t.Errorf("ID = %q, want %q", got.ID, account.ID)
	}
	if token == "" {
		t.Error("FinishLogin issued no session token")
	}

	// The counter must have been written back, or clone detection would have
	// nothing to compare against next time.
	after, err := svc.Session(ctx, token)
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	if after.Credentials[0].SignCount != 7 {
		t.Errorf("SignCount = %d, want 7", after.Credentials[0].SignCount)
	}
	if after.Credentials[0].LastUsedAt.IsZero() {
		t.Error("LastUsedAt was not recorded")
	}
}

// The user handle the authenticator asserts must belong to the credential it
// presented. Without this an authenticator could name one account while
// presenting another's credential.
func TestLoginRejectsMismatchedUserHandle(t *testing.T) {
	ceremony := &fakeCeremony{state: []byte("state"), credential: credential("c1")}
	svc, _ := newService(t, ceremony, newFakeSigner())
	ctx := context.Background()

	_, sealed, err := svc.BeginRegistration(ctx)
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}
	if _, _, err := svc.FinishRegistration(ctx, sealed, []byte("{}")); err != nil {
		t.Fatalf("FinishRegistration: %v", err)
	}

	ceremony.lookupWith = [2][]byte{[]byte("c1"), []byte("somebody-else")}

	_, loginSealed, err := svc.BeginLogin(ctx)
	if err != nil {
		t.Fatalf("BeginLogin: %v", err)
	}
	if _, _, err := svc.FinishLogin(ctx, loginSealed, []byte("{}")); err == nil {
		t.Fatal("a mismatched user handle was accepted")
	}
	if !types.IsUnauthenticated(ceremony.lookupErr) {
		t.Fatalf("lookup error = %v, want *types.UnauthenticatedError", ceremony.lookupErr)
	}
}

func TestLoginRejectsUnknownCredential(t *testing.T) {
	ceremony := &fakeCeremony{state: []byte("state")}
	svc, _ := newService(t, ceremony, newFakeSigner())
	ctx := context.Background()

	ceremony.lookupWith = [2][]byte{[]byte("never-registered"), nil}

	_, sealed, err := svc.BeginLogin(ctx)
	if err != nil {
		t.Fatalf("BeginLogin: %v", err)
	}
	if _, _, err := svc.FinishLogin(ctx, sealed, []byte("{}")); err == nil {
		t.Fatal("an unregistered credential signed in")
	}
}

func TestFinishRejectsExpiredCeremony(t *testing.T) {
	signer := newFakeSigner()
	svc, _ := newService(t, &fakeCeremony{state: []byte("state")}, signer)
	ctx := context.Background()

	_, sealed, err := svc.BeginLogin(ctx)
	if err != nil {
		t.Fatalf("BeginLogin: %v", err)
	}

	signer.expired = true
	if _, _, err := svc.FinishLogin(ctx, sealed, []byte("{}")); !types.IsUnauthenticated(err) {
		t.Fatalf("err = %v, want *types.UnauthenticatedError", err)
	}
}

// A session cookie can outlive the account it names -- the account was deleted,
// or the cookie was minted against a different store -- because a week-long
// session is signed rather than recorded. That must read as "signed out", not
// as a server error, or a returning visitor gets an error page instead of a
// sign-in button.
func TestSessionForVanishedAccountIsUnauthenticated(t *testing.T) {
	ceremony := &fakeCeremony{state: []byte("state"), credential: credential("c1")}
	signer := newFakeSigner()
	svc, _ := newService(t, ceremony, signer)
	ctx := context.Background()

	_, sealed, err := svc.BeginRegistration(ctx)
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}
	_, token, err := svc.FinishRegistration(ctx, sealed, []byte("{}"))
	if err != nil {
		t.Fatalf("FinishRegistration: %v", err)
	}

	// Restart: a brand-new store, the same signing key, the same cookie.
	restarted, _ := newService(t, ceremony, signer)

	_, err = restarted.Session(ctx, token)
	if !types.IsUnauthenticated(err) {
		t.Fatalf("err = %v, want *types.UnauthenticatedError", err)
	}
	var serverErr *types.ServerError
	if errors.As(err, &serverErr) {
		t.Fatal("a vanished account surfaced as a server error")
	}
}

func TestSessionRejectsUnknownToken(t *testing.T) {
	svc, _ := newService(t, &fakeCeremony{state: []byte("state")}, newFakeSigner())
	if _, err := svc.Session(context.Background(), "forged"); !types.IsUnauthenticated(err) {
		t.Fatalf("err = %v, want *types.UnauthenticatedError", err)
	}
}

// A guest session is the one identity that exists without a row, so the two
// things worth pinning are that issuing one writes nothing and that resolving
// one reads nothing.
func TestSignInAnonymouslyStoresNothing(t *testing.T) {
	svc, repo := newService(t, &fakeCeremony{state: []byte("state")}, newFakeSigner())

	guest, token, err := svc.SignInAnonymously(context.Background())
	if err != nil {
		t.Fatalf("SignInAnonymously: %v", err)
	}
	if token == "" {
		t.Fatal("no session token issued")
	}
	if !guest.Anonymous {
		t.Error("guest is not marked anonymous")
	}
	if guest.DisplayName != GuestDisplayName {
		t.Errorf("display name = %q, want %q", guest.DisplayName, GuestDisplayName)
	}
	if len(guest.Credentials) != 0 {
		t.Errorf("guest carries %d credentials, want none", len(guest.Credentials))
	}
	if !strings.HasPrefix(string(guest.ID), user.AnonymousIDPrefix) {
		t.Errorf("id = %q, want the %q prefix", guest.ID, user.AnonymousIDPrefix)
	}

	// The whole promise of the feature: nothing was persisted.
	if _, err := repo.ByID(context.Background(), guest.ID); !types.IsNotFound(err) {
		t.Errorf("ByID = %v, want not found -- a guest must leave no record", err)
	}
}

func TestSignInAnonymouslyIssuesDistinctIDs(t *testing.T) {
	svc, _ := newService(t, &fakeCeremony{state: []byte("state")}, newFakeSigner())

	first, _, err := svc.SignInAnonymously(context.Background())
	if err != nil {
		t.Fatalf("SignInAnonymously: %v", err)
	}
	second, _, err := svc.SignInAnonymously(context.Background())
	if err != nil {
		t.Fatalf("SignInAnonymously: %v", err)
	}
	if first.ID == second.ID {
		t.Errorf("two guests share id %q; they would see each other's characters", first.ID)
	}
}

// The guest lifetime is separate from the account one, and the token has to
// carry the shorter of the two or the distinction means nothing.
func TestAnonymousSessionUsesTheGuestTTL(t *testing.T) {
	signer := newFakeSigner()
	svc, _ := newService(t, &fakeCeremony{state: []byte("state")}, signer)

	_, token, err := svc.SignInAnonymously(context.Background())
	if err != nil {
		t.Fatalf("SignInAnonymously: %v", err)
	}

	issued := signer.sessions[token]
	if got := issued.ExpiresAt.Sub(issued.IssuedAt); got != 15*time.Minute {
		t.Errorf("token lifetime = %s, want the guest TTL of 15m", got)
	}
	if !issued.Anonymous {
		t.Error("token was not signed as anonymous; Session would look for a row")
	}
}

// The inverse of TestSessionForVanishedAccountIsUnauthenticated: an anonymous
// token names nothing on purpose, so the missing row must not sign it out.
func TestSessionForAnonymousTokenNeedsNoAccount(t *testing.T) {
	svc, repo := newService(t, &fakeCeremony{state: []byte("state")}, newFakeSigner())

	guest, token, err := svc.SignInAnonymously(context.Background())
	if err != nil {
		t.Fatalf("SignInAnonymously: %v", err)
	}

	resolved, err := svc.Session(context.Background(), token)
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	if resolved.ID != guest.ID {
		t.Errorf("id = %q, want %q", resolved.ID, guest.ID)
	}
	if !resolved.Anonymous {
		t.Error("resolved session is not marked anonymous")
	}
	if resolved.DisplayName != GuestDisplayName {
		t.Errorf("display name = %q, want %q", resolved.DisplayName, GuestDisplayName)
	}

	// Belt and braces: the repository never gained a row along the way.
	if _, err := repo.ByID(context.Background(), guest.ID); !types.IsNotFound(err) {
		t.Errorf("ByID = %v, want not found", err)
	}
}

// A guest id can never be an account id, so a forged "anon:"-prefixed token is
// not a route into somebody's account -- and an account token is still checked
// against storage.
func TestAnonymousFlagNotIDDecidesTheLookup(t *testing.T) {
	signer := newFakeSigner()
	svc, repo := newService(t, &fakeCeremony{state: []byte("state")}, signer)

	stored := user.User{ID: "real-account", DisplayName: "Alice", CreatedAt: time.Now()}
	if err := repo.Create(context.Background(), stored); err != nil {
		t.Fatalf("seed account: %v", err)
	}

	// A token naming a real account, but flagged anonymous, must not be
	// resolved into that account: the flag decides, and it says "no row".
	token, err := signer.SignSession(domain.Session{
		UserID:    stored.ID,
		IssuedAt:  time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
		Anonymous: true,
	})
	if err != nil {
		t.Fatalf("SignSession: %v", err)
	}

	resolved, err := svc.Session(context.Background(), token)
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	if resolved.DisplayName == stored.DisplayName {
		t.Error("an anonymous token resolved into the stored account it named")
	}
	if !resolved.Anonymous {
		t.Error("resolved session lost the anonymous flag")
	}
}
