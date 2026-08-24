package memory_test

import (
	"context"
	"testing"

	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/adapter/repository/repotest"
	domain "github.com/promix1722/easydnd/internal/domain/user"
)

// The port contract lives in repotest and is shared with the Postgres adapter,
// so that the two cannot drift into disagreeing about which error a bad call
// produces. What remains here is what only an in-process store can be asked.
func TestUserRepository(t *testing.T) {
	repotest.RunUserRepository(t, func(*testing.T) domain.Repository {
		return memory.NewUserRepository()
	})
}

// Callers get copies, so nothing outside the mutex shares a backing array with
// the store. The Postgres adapter gets this for free -- every read decodes
// fresh values -- which is why it is not part of the shared contract.
func TestReadsAreIsolatedFromTheStore(t *testing.T) {
	ctx := context.Background()
	r := memory.NewUserRepository()

	if err := r.Create(ctx, repotest.Account("alice", "cred-a")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	first, err := r.ByID(ctx, "alice")
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	first.Credentials[0].PublicKey[0] = 'X'
	first.Credentials[0].SignCount = 999

	second, err := r.ByID(ctx, "alice")
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if string(second.Credentials[0].PublicKey) != "key-cred-a" || second.Credentials[0].SignCount != 0 {
		t.Fatal("a caller mutated the store through a returned value")
	}
}

// Exercised by `go test -race`: the mutex, not the database, is what makes this
// store safe under concurrent sign-ins.
func TestConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	r := memory.NewUserRepository()
	if err := r.Create(ctx, repotest.Account("alice", "cred-a")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	done := make(chan struct{})
	for i := range 8 {
		go func(n int) {
			defer func() { done <- struct{}{} }()
			for range 50 {
				_, _ = r.ByCredentialID(ctx, []byte("cred-a"))
				_ = r.TouchCredential(ctx, "alice", domain.Credential{
					ID: []byte("cred-a"), SignCount: uint32(n),
				})
			}
		}(i)
	}
	for range 8 {
		<-done
	}
}

// The store hands out copies, so a caller mutating what it read must not reach
// back into it. Only an in-process store can get this wrong -- Postgres cannot
// share a backing array with anybody -- so it stays here rather than in the
// shared contract, beside its credential sibling above.
func TestIdentityReadsAreIsolatedFromTheStore(t *testing.T) {
	repo := memory.NewUserRepository()
	ctx := context.Background()

	stored := domain.User{
		ID:          "acct-1",
		DisplayName: "Rogue",
		Identities:  []domain.Identity{{Provider: domain.ProviderGoogle, Subject: "sub-1"}},
	}
	if err := repo.Create(ctx, stored); err != nil {
		t.Fatalf("Create: %v", err)
	}

	first, err := repo.ByID(ctx, "acct-1")
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	first.Identities[0].Subject = "tampered"

	second, err := repo.ByID(ctx, "acct-1")
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if second.Identities[0].Subject != "sub-1" {
		t.Fatalf("store was mutated through a returned slice: %q", second.Identities[0].Subject)
	}
}
