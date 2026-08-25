package memory

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/types"
)

// FolderRepository is a concurrency-safe in-process folder store.
type FolderRepository struct {
	mu sync.RWMutex
	// Stored by value: a Folder is four scalar fields with nothing a caller
	// could reach into, so unlike a Character it needs no clone on the way
	// out.
	items map[domain.FolderID]domain.Folder
	// nextID counts issued identifiers, as the character store does, so the
	// ids a test sees are predictable.
	nextID int
}

// NewFolderRepository returns an empty store.
func NewFolderRepository() *FolderRepository {
	return &FolderRepository{items: make(map[domain.FolderID]domain.Folder)}
}

// EnsureDefault returns owner's default folder, creating it on first use.
//
// The scan and the create happen under one write lock, which is the reason
// this is a repository method at all. Split across a Get and a Create in the
// application layer, two requests arriving together for a new account would
// both find no default and both make one, and an account would be left with two
// folders it cannot delete.
func (r *FolderRepository) EnsureDefault(_ context.Context, owner domain.OwnerID) (domain.Folder, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.defaultOf(owner); ok {
		return existing, nil
	}
	return r.insert(owner, domain.DefaultFolderName, true), nil
}

// Create stores a new folder for owner.
func (r *FolderRepository) Create(
	_ context.Context, owner domain.OwnerID, name string,
) (domain.Folder, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.insert(owner, name, false), nil
}

// Get returns the folder with the given ID.
func (r *FolderRepository) Get(_ context.Context, id domain.FolderID) (domain.Folder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	f, ok := r.items[id]
	if !ok {
		return domain.Folder{}, types.NewNotFoundError("folder %q", id)
	}
	return f, nil
}

// List returns every folder owned by owner, the default one first.
//
// The default leads because it is the folder a client shows first and the one
// a new account has by itself; sorting it in among the rest by name would make
// where it lands depend on what its owner renamed it to.
func (r *FolderRepository) List(_ context.Context, owner domain.OwnerID) ([]domain.Folder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]domain.Folder, 0, len(r.items))
	for _, f := range r.items {
		if f.Owner != owner {
			continue
		}
		out = append(out, f)
	}
	// Map iteration order is randomised; callers deserve a stable order.
	slices.SortFunc(out, func(a, b domain.Folder) int {
		if a.Default != b.Default {
			if a.Default {
				return -1
			}
			return 1
		}
		return strings.Compare(a.ID.String(), b.ID.String())
	})
	return out, nil
}

// Rename changes a folder's name.
func (r *FolderRepository) Rename(_ context.Context, id domain.FolderID, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	f, ok := r.items[id]
	if !ok {
		return types.NewNotFoundError("folder %q", id)
	}
	// The default folder is renameable. What an account cannot lose is the
	// folder, not the word on it.
	f.Name = name
	r.items[id] = f
	return nil
}

// Delete removes a folder, refusing the default one.
//
// It says nothing about the characters filed in it. Deleting those is the
// application layer's job, because they live in another store and a repository
// that wrote to both would be two repositories sharing a name.
func (r *FolderRepository) Delete(_ context.Context, id domain.FolderID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	f, ok := r.items[id]
	if !ok {
		return types.NewNotFoundError("folder %q", id)
	}
	if f.Default {
		return types.NewValidationError("folder %q is the default folder and cannot be deleted", id)
	}
	delete(r.items, id)
	return nil
}

// defaultOf finds owner's default folder. Callers hold the lock.
func (r *FolderRepository) defaultOf(owner domain.OwnerID) (domain.Folder, bool) {
	for _, f := range r.items {
		if f.Owner == owner && f.Default {
			return f, true
		}
	}
	return domain.Folder{}, false
}

// insert stores a new folder and returns it. Callers hold the write lock.
func (r *FolderRepository) insert(owner domain.OwnerID, name string, isDefault bool) domain.Folder {
	r.nextID++
	f := domain.Folder{
		ID:      domain.FolderID(fmt.Sprintf("fld_%06d", r.nextID)),
		Owner:   owner,
		Name:    name,
		Default: isDefault,
	}
	r.items[f.ID] = f
	return f
}
