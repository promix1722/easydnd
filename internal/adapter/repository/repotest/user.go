package repotest

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	domain "github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// NewUserRepository builds an empty store for one subtest.
//
// It takes *testing.T so an implementation can register cleanup -- the Postgres
// one truncates between subtests -- and so a setup failure fails the right test.
type NewUserRepository func(t *testing.T) domain.Repository

// Account builds a user with one credential per id, matching what the in-memory
// adapter's own tests used before they moved here.
//
// Times are whole seconds because timestamptz is microsecond-precision: a
// time.Now() written and read back is not the value that went in. For the same
// reason every comparison below uses time.Time.Equal and never == or
// reflect.DeepEqual -- pgx decodes into the local zone, so two instants that
// are equal can still differ in their Location.
func Account(id string, credentialIDs ...string) domain.User {
	credentials := make([]domain.Credential, 0, len(credentialIDs))
	for _, c := range credentialIDs {
		credentials = append(credentials, domain.Credential{
			ID:        []byte(c),
			PublicKey: []byte("key-" + c),
			CreatedAt: time.Unix(0, 0).UTC(),
		})
	}
	return domain.User{
		ID:          domain.ID(id),
		DisplayName: id,
		CreatedAt:   time.Unix(0, 0).UTC(),
		Credentials: credentials,
	}
}

// RunUserRepository runs the whole port contract against one implementation.
func RunUserRepository(t *testing.T, newRepo NewUserRepository) {
	t.Helper()

	tests := []struct {
		name string
		run  func(t *testing.T, r domain.Repository)
	}{
		{"CreateAndLookup", testCreateAndLookup},
		{"CreateRejectsEmptyID", testCreateRejectsEmptyID},
		{"CreateRejectsDuplicateAccount", testCreateRejectsDuplicateAccount},
		{"CreateRejectsCredentialClaimedByAnother", testCreateRejectsCredentialClaimedByAnother},
		{"RejectedCreateLeavesNothingBehind", testRejectedCreateLeavesNothingBehind},
		{"ByIDMissing", testByIDMissing},
		{"ByCredentialIDMissing", testByCredentialIDMissing},
		{"TouchCredentialUpdatesOnlyTheMutableFields", testTouchCredentialUpdatesOnlyTheMutableFields},
		{"TouchCredentialOfAnotherAccount", testTouchCredentialOfAnotherAccount},
		{"TouchCredentialMissingAccount", testTouchCredentialMissingAccount},
		{"CredentialFieldsRoundTrip", testCredentialFieldsRoundTrip},
		{"ZeroLastUsedRoundTrips", testZeroLastUsedRoundTrips},

		{"ByIdentity", testByIdentity},
		{"ByIdentityMissing", testByIdentityMissing},
		{"IdentityIndexIsScopedToItsProvider", testIdentityIndexIsScopedToItsProvider},
		{"CreateRejectsIdentityClaimedByAnother", testCreateRejectsIdentityClaimedByAnother},
		{"RejectedCreateLeavesNoIdentityBehind", testRejectedCreateLeavesNoIdentityBehind},
		{"AddIdentity", testAddIdentity},
		{"AddIdentityToMissingAccount", testAddIdentityToMissingAccount},
		{"AddIdentityRejectsDuplicate", testAddIdentityRejectsDuplicate},
		{"TouchIdentityUpdatesOnlyTheMutableFields", testTouchIdentityUpdatesOnlyTheMutableFields},
		{"TouchIdentityOfAnotherAccount", testTouchIdentityOfAnotherAccount},
		{"RemoveIdentity", testRemoveIdentity},
		{"RemoveIdentityOfAnotherAccount", testRemoveIdentityOfAnotherAccount},
		{"IdentityFieldsRoundTrip", testIdentityFieldsRoundTrip},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t, newRepo(t))
		})
	}
}

func testCreateAndLookup(t *testing.T, r domain.Repository) {
	ctx := context.Background()
	if err := r.Create(ctx, Account("alice", "cred-a")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	byID, err := r.ByID(ctx, "alice")
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if byID.DisplayName != "alice" {
		t.Errorf("DisplayName = %q", byID.DisplayName)
	}

	byCred, err := r.ByCredentialID(ctx, []byte("cred-a"))
	if err != nil {
		t.Fatalf("ByCredentialID: %v", err)
	}
	if byCred.ID != "alice" {
		t.Errorf("ID = %q, want alice", byCred.ID)
	}
}

func testCreateRejectsEmptyID(t *testing.T, r domain.Repository) {
	err := r.Create(context.Background(), Account("", "cred-a"))
	var want *types.ValidationError
	if !errors.As(err, &want) {
		t.Fatalf("err = %v, want *types.ValidationError", err)
	}
}

func testCreateRejectsDuplicateAccount(t *testing.T, r domain.Repository) {
	ctx := context.Background()
	if err := r.Create(ctx, Account("alice", "cred-a")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	err := r.Create(ctx, Account("alice", "cred-b"))

	var want *types.ValidationError
	if !errors.As(err, &want) {
		t.Fatalf("err = %v, want *types.ValidationError", err)
	}
}

// A credential id identifies exactly one account: the usernameless sign-in
// looks the account up by it and has nothing else to disambiguate with.
func testCreateRejectsCredentialClaimedByAnother(t *testing.T, r domain.Repository) {
	ctx := context.Background()
	if err := r.Create(ctx, Account("alice", "shared")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := r.Create(ctx, Account("bob", "shared")); err == nil {
		t.Fatal("a second account claimed an existing credential")
	}
}

// A rejected Create must leave nothing behind: a credential resolving to an
// account that was never stored is a sign-in that fails in a way nobody can
// explain.
func testRejectedCreateLeavesNothingBehind(t *testing.T, r domain.Repository) {
	ctx := context.Background()
	if err := r.Create(ctx, Account("alice", "shared")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// bob's SECOND credential collides; his first must not survive.
	if err := r.Create(ctx, Account("bob", "fresh", "shared")); err == nil {
		t.Fatal("Create succeeded despite a colliding credential")
	}

	if _, err := r.ByCredentialID(ctx, []byte("fresh")); !types.IsNotFound(err) {
		t.Fatalf("err = %v, want *types.NotFoundError -- a partial write leaked", err)
	}
	if _, err := r.ByID(ctx, "bob"); !types.IsNotFound(err) {
		t.Fatalf("err = %v, want *types.NotFoundError", err)
	}
}

func testByIDMissing(t *testing.T, r domain.Repository) {
	if _, err := r.ByID(context.Background(), "nobody"); !types.IsNotFound(err) {
		t.Fatalf("err = %v, want *types.NotFoundError", err)
	}
}

func testByCredentialIDMissing(t *testing.T, r domain.Repository) {
	if _, err := r.ByCredentialID(context.Background(), []byte("nobody")); !types.IsNotFound(err) {
		t.Fatalf("err = %v, want *types.NotFoundError", err)
	}
}

func testTouchCredentialUpdatesOnlyTheMutableFields(t *testing.T, r domain.Repository) {
	ctx := context.Background()
	if err := r.Create(ctx, Account("alice", "cred-a")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	used := time.Unix(1700000000, 0).UTC()
	err := r.TouchCredential(ctx, "alice", domain.Credential{
		ID:         []byte("cred-a"),
		PublicKey:  []byte("attacker-supplied"),
		SignCount:  42,
		LastUsedAt: used,
	})
	if err != nil {
		t.Fatalf("TouchCredential: %v", err)
	}

	found, err := r.ByID(ctx, "alice")
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	stored := found.Credentials[0]
	if stored.SignCount != 42 {
		t.Errorf("SignCount = %d, want 42", stored.SignCount)
	}
	if !stored.LastUsedAt.Equal(used) {
		t.Errorf("LastUsedAt = %v, want %v", stored.LastUsedAt, used)
	}
	// The public key is what verifies every future assertion. A replayed
	// ceremony must not be able to swap it.
	if string(stored.PublicKey) != "key-cred-a" {
		t.Errorf("PublicKey = %q -- TouchCredential overwrote it", stored.PublicKey)
	}
}

// One account must not be able to move another account's sign count.
func testTouchCredentialOfAnotherAccount(t *testing.T, r domain.Repository) {
	ctx := context.Background()
	if err := r.Create(ctx, Account("alice", "cred-a")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := r.Create(ctx, Account("bob", "cred-b")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	err := r.TouchCredential(ctx, "alice", domain.Credential{
		ID: []byte("cred-b"), SignCount: 99, LastUsedAt: time.Unix(2, 0).UTC(),
	})
	if !types.IsNotFound(err) {
		t.Fatalf("err = %v, want *types.NotFoundError", err)
	}

	bob, err := r.ByID(ctx, "bob")
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if bob.Credentials[0].SignCount != 0 {
		t.Errorf("SignCount = %d -- alice moved bob's counter", bob.Credentials[0].SignCount)
	}
}

func testTouchCredentialMissingAccount(t *testing.T, r domain.Repository) {
	err := r.TouchCredential(context.Background(), "nobody",
		domain.Credential{ID: []byte("cred-a")})
	if !types.IsNotFound(err) {
		t.Fatalf("err = %v, want *types.NotFoundError", err)
	}
}

// Every field a relying party needs must survive the round trip. A public key
// or a sign count that comes back wrong is a passkey that stops verifying, and
// there is no account recovery to fall back on.
func testCredentialFieldsRoundTrip(t *testing.T, r domain.Repository) {
	ctx := context.Background()
	created := time.Unix(1600000000, 0).UTC()
	used := time.Unix(1700000000, 0).UTC()

	want := domain.Credential{
		ID:              []byte{0x00, 0x01, 0xff, 0xfe},
		PublicKey:       []byte{0xa5, 0x01, 0x02, 0x03},
		AttestationType: "packed",
		Transports:      []string{"internal", "hybrid"},
		AAGUID:          []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SignCount:       math.MaxUint32,
		BackupEligible:  true,
		BackupState:     true,
		CreatedAt:       created,
		LastUsedAt:      used,
	}

	err := r.Create(ctx, domain.User{
		ID: "alice", DisplayName: "alice", CreatedAt: created,
		Credentials: []domain.Credential{want},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := r.ByCredentialID(ctx, want.ID)
	if err != nil {
		t.Fatalf("ByCredentialID: %v", err)
	}
	if len(found.Credentials) != 1 {
		t.Fatalf("Credentials = %d, want 1", len(found.Credentials))
	}
	got := found.Credentials[0]

	if string(got.ID) != string(want.ID) {
		t.Errorf("ID = %x, want %x", got.ID, want.ID)
	}
	if string(got.PublicKey) != string(want.PublicKey) {
		t.Errorf("PublicKey = %x, want %x", got.PublicKey, want.PublicKey)
	}
	if got.AttestationType != want.AttestationType {
		t.Errorf("AttestationType = %q, want %q", got.AttestationType, want.AttestationType)
	}
	if len(got.Transports) != len(want.Transports) {
		t.Fatalf("Transports = %v, want %v", got.Transports, want.Transports)
	}
	for i := range want.Transports {
		if got.Transports[i] != want.Transports[i] {
			t.Errorf("Transports[%d] = %q, want %q", i, got.Transports[i], want.Transports[i])
		}
	}
	if string(got.AAGUID) != string(want.AAGUID) {
		t.Errorf("AAGUID = %x, want %x", got.AAGUID, want.AAGUID)
	}
	// The whole reason sign_count is a bigint rather than an integer.
	if got.SignCount != want.SignCount {
		t.Errorf("SignCount = %d, want %d", got.SignCount, want.SignCount)
	}
	if got.BackupEligible != want.BackupEligible || got.BackupState != want.BackupState {
		t.Errorf("backup flags = %v/%v, want %v/%v",
			got.BackupEligible, got.BackupState, want.BackupEligible, want.BackupState)
	}
	// Equal, not ==: timestamptz is microsecond-precision and pgx decodes into
	// the local zone, so two equal instants can differ in their Location.
	if !got.CreatedAt.Equal(want.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, want.CreatedAt)
	}
	if !got.LastUsedAt.Equal(want.LastUsedAt) {
		t.Errorf("LastUsedAt = %v, want %v", got.LastUsedAt, want.LastUsedAt)
	}
}

// A credential that has never been asserted carries the zero time. Postgres
// stores that as NULL, and it must come back as the zero time rather than as
// the year 1 or as a decoding error.
func testZeroLastUsedRoundTrips(t *testing.T, r domain.Repository) {
	ctx := context.Background()
	if err := r.Create(ctx, Account("alice", "cred-a")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	found, err := r.ByID(ctx, "alice")
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if !found.Credentials[0].LastUsedAt.IsZero() {
		t.Errorf("LastUsedAt = %v, want the zero time", found.Credentials[0].LastUsedAt)
	}
}

// Identity builds a linked external account. Whole seconds, for the reason
// Account gives: timestamptz is microsecond-precision and pgx decodes into the
// local zone, so every comparison below uses time.Time.Equal.
func Identity(subject string) domain.Identity {
	return domain.Identity{
		Provider:  domain.ProviderGoogle,
		Subject:   subject,
		Email:     subject + "@example.test",
		CreatedAt: time.Unix(0, 0).UTC(),
	}
}

// withIdentities is Account plus links, for the cases that need both kinds of
// proof on one record.
func withIdentities(u domain.User, identities ...domain.Identity) domain.User {
	u.Identities = append(u.Identities, identities...)
	return u
}

func testByIdentity(t *testing.T, r domain.Repository) {
	ctx := context.Background()
	if err := r.Create(ctx, withIdentities(Account("alice", "cred-a"), Identity("sub-1"))); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := r.ByIdentity(ctx, domain.ProviderGoogle, "sub-1")
	if err != nil {
		t.Fatalf("ByIdentity: %v", err)
	}
	if found.ID != "alice" {
		t.Fatalf("resolved %q, want alice", found.ID)
	}
	// The account arrives whole: both ways in, not just the one looked up by.
	if len(found.Credentials) != 1 || len(found.Identities) != 1 {
		t.Fatalf("account has %d credentials and %d identities, want 1 and 1",
			len(found.Credentials), len(found.Identities))
	}
	if found.SignInMethods() != 2 {
		t.Fatalf("SignInMethods() = %d, want 2", found.SignInMethods())
	}
}

func testByIdentityMissing(t *testing.T, r domain.Repository) {
	_, err := r.ByIdentity(context.Background(), domain.ProviderGoogle, "nobody")
	if !types.IsNotFound(err) {
		t.Fatalf("ByIdentity for an unknown subject: got %v, want NotFound", err)
	}
}

// A subject is only unique within its issuer. Keyed on the subject alone, one
// provider's subject would resolve to an account linked through another --
// which is a sign-in as the wrong person.
func testIdentityIndexIsScopedToItsProvider(t *testing.T, r domain.Repository) {
	ctx := context.Background()

	google := domain.Identity{Provider: domain.ProviderGoogle, Subject: "shared", CreatedAt: time.Unix(0, 0).UTC()}
	other := domain.Identity{Provider: "other", Subject: "shared", CreatedAt: time.Unix(0, 0).UTC()}

	if err := r.Create(ctx, withIdentities(Account("alice", "cred-a"), google)); err != nil {
		t.Fatalf("Create alice: %v", err)
	}
	if err := r.Create(ctx, withIdentities(Account("bob", "cred-b"), other)); err != nil {
		t.Fatalf("Create bob with the same subject under another provider: %v", err)
	}

	first, err := r.ByIdentity(ctx, domain.ProviderGoogle, "shared")
	if err != nil || first.ID != "alice" {
		t.Fatalf("google/shared resolved to %q (%v), want alice", first.ID, err)
	}
	second, err := r.ByIdentity(ctx, "other", "shared")
	if err != nil || second.ID != "bob" {
		t.Fatalf("other/shared resolved to %q (%v), want bob", second.ID, err)
	}
}

func testCreateRejectsIdentityClaimedByAnother(t *testing.T, r domain.Repository) {
	ctx := context.Background()
	if err := r.Create(ctx, withIdentities(Account("alice", "cred-a"), Identity("sub-1"))); err != nil {
		t.Fatalf("Create: %v", err)
	}

	err := r.Create(ctx, withIdentities(Account("bob", "cred-b"), Identity("sub-1")))
	var validation *types.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("Create with a linked identity: got %v, want a validation error", err)
	}
}

// A partial write leaves index entries resolving to an account that was never
// stored -- the same rule the credential case pins, and the database enforces
// it here rather than a pre-check.
func testRejectedCreateLeavesNoIdentityBehind(t *testing.T, r domain.Repository) {
	ctx := context.Background()
	if err := r.Create(ctx, withIdentities(Account("alice", "cred-a"), Identity("taken"))); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// The first identity is free, the second is taken. Neither must land.
	err := r.Create(ctx, withIdentities(Account("bob", "cred-b"), Identity("fresh"), Identity("taken")))
	if err == nil {
		t.Fatal("Create succeeded, want a validation error")
	}
	if _, err := r.ByIdentity(ctx, domain.ProviderGoogle, "fresh"); !types.IsNotFound(err) {
		t.Fatalf("orphaned identity survived: got %v, want NotFound", err)
	}
	if _, err := r.ByID(ctx, "bob"); !types.IsNotFound(err) {
		t.Fatalf("rejected account survived: got %v, want NotFound", err)
	}
}

func testAddIdentity(t *testing.T, r domain.Repository) {
	ctx := context.Background()
	if err := r.Create(ctx, Account("alice", "cred-a")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := r.AddIdentity(ctx, "alice", Identity("sub-1")); err != nil {
		t.Fatalf("AddIdentity: %v", err)
	}

	found, err := r.ByIdentity(ctx, domain.ProviderGoogle, "sub-1")
	if err != nil {
		t.Fatalf("ByIdentity: %v", err)
	}
	if found.ID != "alice" || found.SignInMethods() != 2 {
		t.Fatalf("account %q has %d sign-in methods, want alice and 2",
			found.ID, found.SignInMethods())
	}
	if got := found.Identities[0]; got.Email != "sub-1@example.test" {
		t.Errorf("identity did not round-trip: %+v", got)
	}
}

func testAddIdentityToMissingAccount(t *testing.T, r domain.Repository) {
	err := r.AddIdentity(context.Background(), "nobody", Identity("sub-1"))
	if !types.IsNotFound(err) {
		t.Fatalf("AddIdentity to a missing account: got %v, want NotFound", err)
	}
}

// Taken by anyone at all, this account included: re-linking a subject an
// account already holds would duplicate it while the index kept pointing at
// one of them.
func testAddIdentityRejectsDuplicate(t *testing.T, r domain.Repository) {
	ctx := context.Background()
	if err := r.Create(ctx, withIdentities(Account("alice", "cred-a"), Identity("sub-1"))); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := r.Create(ctx, Account("bob", "cred-b")); err != nil {
		t.Fatalf("Create bob: %v", err)
	}

	for _, target := range []domain.ID{"alice", "bob"} {
		err := r.AddIdentity(ctx, target, Identity("sub-1"))
		var validation *types.ValidationError
		if !errors.As(err, &validation) {
			t.Errorf("AddIdentity to %q: got %v, want a validation error", target, err)
		}
	}

	// And it still belongs to whoever had it.
	found, err := r.ByIdentity(ctx, domain.ProviderGoogle, "sub-1")
	if err != nil || found.ID != "alice" {
		t.Fatalf("identity moved to %q (%v), want alice", found.ID, err)
	}
}

// provider and subject are the link itself, and created_at is history. A
// replayed exchange must not be able to move any of them.
func testTouchIdentityUpdatesOnlyTheMutableFields(t *testing.T, r domain.Repository) {
	ctx := context.Background()
	original := Identity("sub-1")
	if err := r.Create(ctx, withIdentities(Account("alice", "cred-a"), original)); err != nil {
		t.Fatalf("Create: %v", err)
	}

	used := time.Unix(1_700_000_000, 0).UTC()
	err := r.TouchIdentity(ctx, "alice", domain.Identity{
		Provider:      domain.ProviderGoogle,
		Subject:       "sub-1",
		Email:         "renamed@example.test",
		EmailVerified: true,
		DisplayName:   "Renamed Upstream",
		CreatedAt:     time.Unix(999, 0).UTC(),
		LastUsedAt:    used,
	})
	if err != nil {
		t.Fatalf("TouchIdentity: %v", err)
	}

	found, err := r.ByID(ctx, "alice")
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	got := found.Identities[0]
	if got.Email != "renamed@example.test" || !got.EmailVerified || got.DisplayName != "Renamed Upstream" {
		t.Errorf("mutable fields not updated: %+v", got)
	}
	if !got.LastUsedAt.Equal(used) {
		t.Errorf("LastUsedAt = %s, want %s", got.LastUsedAt, used)
	}
	if !got.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt moved to %s, want %s", got.CreatedAt, original.CreatedAt)
	}
	if got.Provider != domain.ProviderGoogle || got.Subject != "sub-1" {
		t.Errorf("the link itself moved: %+v", got)
	}

	if err := r.TouchIdentity(ctx, "alice", Identity("never-linked")); !types.IsNotFound(err) {
		t.Errorf("TouchIdentity for an unlinked subject: got %v, want NotFound", err)
	}
}

// One account must not be able to touch another's identity.
func testTouchIdentityOfAnotherAccount(t *testing.T, r domain.Repository) {
	ctx := context.Background()
	if err := r.Create(ctx, withIdentities(Account("alice", "cred-a"), Identity("sub-1"))); err != nil {
		t.Fatalf("Create alice: %v", err)
	}
	if err := r.Create(ctx, Account("bob", "cred-b")); err != nil {
		t.Fatalf("Create bob: %v", err)
	}

	err := r.TouchIdentity(ctx, "bob", domain.Identity{
		Provider: domain.ProviderGoogle, Subject: "sub-1", Email: "hijack@example.test",
	})
	if !types.IsNotFound(err) {
		t.Fatalf("TouchIdentity across accounts: got %v, want NotFound", err)
	}

	found, err := r.ByID(ctx, "alice")
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if found.Identities[0].Email != "sub-1@example.test" {
		t.Errorf("another account rewrote the identity: %+v", found.Identities[0])
	}
}

func testRemoveIdentity(t *testing.T, r domain.Repository) {
	ctx := context.Background()
	if err := r.Create(ctx, withIdentities(Account("alice", "cred-a"), Identity("sub-1"))); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := r.RemoveIdentity(ctx, "alice", domain.ProviderGoogle, "sub-1"); err != nil {
		t.Fatalf("RemoveIdentity: %v", err)
	}
	if _, err := r.ByIdentity(ctx, domain.ProviderGoogle, "sub-1"); !types.IsNotFound(err) {
		t.Fatalf("identity still resolves after unlink: %v", err)
	}

	found, err := r.ByID(ctx, "alice")
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if len(found.Identities) != 0 || len(found.Credentials) != 1 {
		t.Fatalf("after unlink: %d identities, %d credentials; want 0 and 1",
			len(found.Identities), len(found.Credentials))
	}

	// Freed, so somebody else can link it now.
	if err := r.AddIdentity(ctx, "alice", Identity("sub-1")); err != nil {
		t.Fatalf("relink after unlink: %v", err)
	}

	if err := r.RemoveIdentity(ctx, "alice", domain.ProviderGoogle, "never-linked"); !types.IsNotFound(err) {
		t.Errorf("RemoveIdentity for an unlinked subject: got %v, want NotFound", err)
	}
}

// Unlinking is the one operation that could strand an account, so it must not
// be reachable across accounts either.
func testRemoveIdentityOfAnotherAccount(t *testing.T, r domain.Repository) {
	ctx := context.Background()
	if err := r.Create(ctx, withIdentities(Account("alice", "cred-a"), Identity("sub-1"))); err != nil {
		t.Fatalf("Create alice: %v", err)
	}
	if err := r.Create(ctx, Account("bob", "cred-b")); err != nil {
		t.Fatalf("Create bob: %v", err)
	}

	if err := r.RemoveIdentity(ctx, "bob", domain.ProviderGoogle, "sub-1"); !types.IsNotFound(err) {
		t.Fatalf("RemoveIdentity across accounts: got %v, want NotFound", err)
	}
	if _, err := r.ByIdentity(ctx, domain.ProviderGoogle, "sub-1"); err != nil {
		t.Fatalf("alice lost her identity to bob's unlink: %v", err)
	}
}

// Every field, through storage and back. The zero LastUsedAt is the
// interesting one: it is SQL NULL rather than the year 1, so that "never used"
// is recorded as an absence in both adapters.
func testIdentityFieldsRoundTrip(t *testing.T, r domain.Repository) {
	ctx := context.Background()
	full := domain.Identity{
		Provider:      domain.ProviderGoogle,
		Subject:       "104729-a-long-opaque-subject",
		Email:         "someone@example.test",
		EmailVerified: true,
		DisplayName:   "Someone With A Name",
		CreatedAt:     time.Unix(1_600_000_000, 0).UTC(),
		LastUsedAt:    time.Unix(1_700_000_000, 0).UTC(),
	}
	bare := domain.Identity{
		Provider:  "other",
		Subject:   "minimal",
		CreatedAt: time.Unix(1_600_000_000, 0).UTC(),
	}

	if err := r.Create(ctx, withIdentities(Account("alice", "cred-a"), full, bare)); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := r.ByID(ctx, "alice")
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if len(found.Identities) != 2 {
		t.Fatalf("got %d identities, want 2", len(found.Identities))
	}

	bySubject := map[string]domain.Identity{}
	for _, i := range found.Identities {
		bySubject[i.Subject] = i
	}

	got := bySubject[full.Subject]
	if got.Provider != full.Provider || got.Email != full.Email ||
		got.EmailVerified != full.EmailVerified || got.DisplayName != full.DisplayName {
		t.Errorf("full identity did not round-trip: %+v", got)
	}
	if !got.CreatedAt.Equal(full.CreatedAt) || !got.LastUsedAt.Equal(full.LastUsedAt) {
		t.Errorf("times did not round-trip: %+v", got)
	}

	empty := bySubject[bare.Subject]
	if empty.Email != "" || empty.EmailVerified || empty.DisplayName != "" {
		t.Errorf("absent fields came back set: %+v", empty)
	}
	if !empty.LastUsedAt.IsZero() {
		t.Errorf("never-used identity came back with LastUsedAt = %s", empty.LastUsedAt)
	}
}
