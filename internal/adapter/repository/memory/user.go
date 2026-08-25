package memory

import (
	"context"
	"slices"
	"sync"

	domain "github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// UserRepository is a concurrency-safe in-process account store.
//
// It is no longer how production stores accounts -- that is
// internal/adapter/repository/postgres. Two jobs remain for it.
//
// It is the development fallback: with no db.url the server runs on this
// and warns, so `make run/server`, `go test ./...` and `make verify` all work
// on a machine with no Postgres. config.validate refuses that combination in
// production, where losing accounts means losing passkeys that cannot be
// reissued.
//
// And it is the reference implementation of domain.Repository. Both adapters
// run internal/adapter/repository/repotest, so the contract is defined by what
// they must agree on rather than by whichever one was written first.
type UserRepository struct {
	mu sync.RWMutex
	// Stored by value so a caller holding a User cannot reach in and mutate
	// our state behind the mutex. The credential slice is copied on the way in
	// and out for the same reason.
	items map[domain.ID]domain.User
	// byCredential maps a raw credential id to its owner, so a usernameless
	// assertion resolves in one lookup instead of a scan. Keyed by
	// string(credentialID) because a []byte cannot be a map key.
	byCredential map[string]domain.ID
	// byIdentity does the same for external identities, keyed by provider and
	// subject together. Subjects are only unique within a provider, so a
	// key of the subject alone would let one provider's subject resolve to an
	// account linked through another.
	byIdentity map[identityKey]domain.ID
}

// identityKey is the composite index key: a subject is only meaningful
// alongside the provider that issued it.
type identityKey struct {
	provider domain.Provider
	subject  string
}

// NewUserRepository returns an empty store.
func NewUserRepository() *UserRepository {
	return &UserRepository{
		items:        make(map[domain.ID]domain.User),
		byCredential: make(map[string]domain.ID),
		byIdentity:   make(map[identityKey]domain.ID),
	}
}

// Create stores u and indexes its credentials, rejecting a duplicate account
// id or a credential id already claimed by someone else.
func (r *UserRepository) Create(_ context.Context, u domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if u.ID == "" {
		return types.NewValidationError("account id must not be empty")
	}
	if _, exists := r.items[u.ID]; exists {
		return types.NewValidationError("account %q already exists", u.ID)
	}
	// Check every credential and identity before writing any of them: a
	// partial index is worse than a rejected write, because the orphaned
	// entries would resolve to an account that was never stored.
	for _, c := range u.Credentials {
		if _, taken := r.byCredential[string(c.ID)]; taken {
			return types.NewValidationError("credential is already registered")
		}
	}
	for _, i := range u.Identities {
		if _, taken := r.byIdentity[keyOf(i.Provider, i.Subject)]; taken {
			return types.NewValidationError("that %s account is already linked", i.Provider)
		}
	}

	r.items[u.ID] = cloneUser(u)
	for _, c := range u.Credentials {
		r.byCredential[string(c.ID)] = u.ID
	}
	for _, i := range u.Identities {
		r.byIdentity[keyOf(i.Provider, i.Subject)] = u.ID
	}
	return nil
}

// EnsureGuest stores a guest's row if it is not already there.
//
// Idempotent by design: a guest reaches this on every group they join, so
// "already present" is the ordinary case. It deliberately does not refresh the
// stored name from u -- the first join is what named them, and letting a later
// call rewrite it would let a guest change what the roster calls them by
// joining a second group.
func (r *UserRepository) EnsureGuest(_ context.Context, u domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if u.ID == "" {
		return types.NewValidationError("account id must not be empty")
	}
	if _, exists := r.items[u.ID]; exists {
		return nil
	}

	// Stored without Anonymous set: the field is synthesised from the session
	// token and repositories never persist it. What marks this row as a guest
	// is the id, which carries user.AnonymousIDPrefix.
	stored := domain.User{ID: u.ID, DisplayName: u.DisplayName, CreatedAt: u.CreatedAt}
	r.items[u.ID] = stored
	return nil
}

// ByID returns the account with the given id.
func (r *UserRepository) ByID(_ context.Context, id domain.ID) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	u, ok := r.items[id]
	if !ok {
		return domain.User{}, types.NewNotFoundError("account not found")
	}
	return cloneUser(u), nil
}

// ByCredentialID returns the account owning the given raw credential id.
func (r *UserRepository) ByCredentialID(_ context.Context, credentialID []byte) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.byCredential[string(credentialID)]
	if !ok {
		return domain.User{}, types.NewNotFoundError("credential not found")
	}
	u, ok := r.items[id]
	if !ok {
		// Only reachable if Create and the index ever disagree, which the
		// all-or-nothing write above is there to prevent.
		return domain.User{}, types.NewNotFoundError("account not found")
	}
	return cloneUser(u), nil
}

// TouchCredential records a successful assertion against a stored credential.
func (r *UserRepository) TouchCredential(_ context.Context, id domain.ID, c domain.Credential) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	u, ok := r.items[id]
	if !ok {
		return types.NewNotFoundError("account not found")
	}
	index := slices.IndexFunc(u.Credentials, func(stored domain.Credential) bool {
		return slices.Equal(stored.ID, c.ID)
	})
	if index < 0 {
		return types.NewNotFoundError("credential not found")
	}

	updated := slices.Clone(u.Credentials)
	// Only the fields an assertion can legitimately move. Rewriting the whole
	// record would let a replayed ceremony overwrite the public key.
	updated[index].SignCount = c.SignCount
	updated[index].BackupState = c.BackupState
	updated[index].LastUsedAt = c.LastUsedAt
	u.Credentials = updated
	r.items[id] = u
	return nil
}

// ByIdentity returns the account holding the given external identity.
func (r *UserRepository) ByIdentity(_ context.Context, provider domain.Provider, subject string) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.byIdentity[keyOf(provider, subject)]
	if !ok {
		return domain.User{}, types.NewNotFoundError("identity not found")
	}
	u, ok := r.items[id]
	if !ok {
		// Only reachable if a write and the index ever disagree, which the
		// all-or-nothing writes are there to prevent.
		return domain.User{}, types.NewNotFoundError("account not found")
	}
	return cloneUser(u), nil
}

// AddIdentity links an external identity to an existing account.
func (r *UserRepository) AddIdentity(_ context.Context, id domain.ID, i domain.Identity) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	u, ok := r.items[id]
	if !ok {
		return types.NewNotFoundError("account not found")
	}
	// Taken by anyone at all, this account included. Re-linking a subject an
	// account already holds would duplicate it in the slice while the index
	// kept pointing at one of them.
	if _, taken := r.byIdentity[keyOf(i.Provider, i.Subject)]; taken {
		return types.NewValidationError("that %s account is already linked", i.Provider)
	}

	u.Identities = append(slices.Clone(u.Identities), i)
	r.items[id] = u
	r.byIdentity[keyOf(i.Provider, i.Subject)] = id
	return nil
}

// TouchIdentity records a successful federated sign-in.
func (r *UserRepository) TouchIdentity(_ context.Context, id domain.ID, i domain.Identity) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	u, ok := r.items[id]
	if !ok {
		return types.NewNotFoundError("account not found")
	}
	index := indexOfIdentity(u.Identities, i.Provider, i.Subject)
	if index < 0 {
		return types.NewNotFoundError("identity not found")
	}

	updated := slices.Clone(u.Identities)
	// Only what a fresh sign-in legitimately re-asserts. Rewriting the whole
	// record would let the provider or subject be moved by a replayed
	// exchange, which is the link itself.
	updated[index].Email = i.Email
	updated[index].EmailVerified = i.EmailVerified
	updated[index].DisplayName = i.DisplayName
	updated[index].LastUsedAt = i.LastUsedAt
	u.Identities = updated
	r.items[id] = u
	return nil
}

// RemoveIdentity unlinks an external identity.
func (r *UserRepository) RemoveIdentity(_ context.Context, id domain.ID, provider domain.Provider, subject string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	u, ok := r.items[id]
	if !ok {
		return types.NewNotFoundError("account not found")
	}
	index := indexOfIdentity(u.Identities, provider, subject)
	if index < 0 {
		return types.NewNotFoundError("identity not found")
	}

	u.Identities = slices.Delete(slices.Clone(u.Identities), index, index+1)
	r.items[id] = u
	delete(r.byIdentity, keyOf(provider, subject))
	return nil
}

func keyOf(provider domain.Provider, subject string) identityKey {
	return identityKey{provider: provider, subject: subject}
}

func indexOfIdentity(identities []domain.Identity, provider domain.Provider, subject string) int {
	return slices.IndexFunc(identities, func(stored domain.Identity) bool {
		return stored.Provider == provider && stored.Subject == subject
	})
}

// cloneUser deep-copies the parts of a User that are reference types, so that
// nothing outside the mutex ends up sharing a backing array with the store.
func cloneUser(u domain.User) domain.User {
	out := u
	out.Credentials = make([]domain.Credential, len(u.Credentials))
	for i, c := range u.Credentials {
		c.ID = slices.Clone(c.ID)
		c.PublicKey = slices.Clone(c.PublicKey)
		c.AAGUID = slices.Clone(c.AAGUID)
		c.Transports = slices.Clone(c.Transports)
		out.Credentials[i] = c
	}
	// Identity holds no reference types of its own, so a slice copy is the
	// whole of it -- but it still needs one, or two callers would share a
	// backing array with the store.
	out.Identities = slices.Clone(u.Identities)
	return out
}

// Compile-time proof that this adapter satisfies the port.
var _ domain.Repository = (*UserRepository)(nil)
