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
		return domain.Folder{}, types.NewNotFoundError("folder %q", id).Because("folder.notFound")
	}
	return f, nil
}

// List returns every folder owned by owner, the default one first.
//
// The default leads because it is the folder a client shows first and the one
// a new account has by itself; sorting it in among the rest by name would make
// where it lands depend on what its owner renamed it to, and sorting it in by
// Position would let it wander every time its owner rearranged the others.
//
// The rest go by Position, with the identifier breaking a tie. The tiebreak is
// not decoration: two folders can share a position only if something wrote them
// that way, and a sort that left them in map order would make a listing
// flicker between two orders on consecutive reads.
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
		if a.Position != b.Position {
			return a.Position - b.Position
		}
		return strings.Compare(a.ID.String(), b.ID.String())
	})
	return out, nil
}

// Reorder sets the order of owner's non-default folders.
//
// The whole run is rewritten under one write lock, so there is no window in
// which half the folders carry the new order and half the old. It is also why
// the argument is the complete set rather than a single move: a "move this one
// up" arriving against a list that has changed underneath its sender is a
// request nothing can honour correctly, while a final order either matches what
// the account has or does not.
//
// A folder belonging to somebody else fails the set comparison rather than
// getting its own error, which is deliberate: from this owner's side it is
// simply an id they do not have.
func (r *FolderRepository) Reorder(
	_ context.Context, owner domain.OwnerID, ids []domain.FolderID,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	movable := make(map[domain.FolderID]struct{})
	for id, f := range r.items {
		if f.Owner == owner && !f.Default {
			movable[id] = struct{}{}
		}
	}
	if len(ids) != len(movable) {
		return types.NewValidationError(
			"the order must name all %d of your folders, and it names %d",
			len(movable), len(ids))
	}
	for _, id := range ids {
		if _, ok := movable[id]; !ok {
			return types.NewValidationError(
				"folder %q is not one of yours to order, or is named twice", id)
		}
		// Removed as it is seen, so a repeat fails the check above on its
		// second appearance rather than silently displacing a folder that
		// the caller left out.
		delete(movable, id)
	}

	// Numbered from one, leaving zero to the default folder -- which is
	// sorted first regardless, so the number is only ever cosmetic there.
	for i, id := range ids {
		f := r.items[id]
		f.Position = i + 1
		r.items[id] = f
	}
	return nil
}

// Rename changes a folder's name.
func (r *FolderRepository) Rename(_ context.Context, id domain.FolderID, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	f, ok := r.items[id]
	if !ok {
		return types.NewNotFoundError("folder %q", id).Because("folder.notFound")
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
		return types.NewNotFoundError("folder %q", id).Because("folder.notFound")
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
//
// A new folder lands last, which is the only position that needs no decision
// from whoever made it: they asked for a folder, not for a place in the list.
func (r *FolderRepository) insert(owner domain.OwnerID, name string, isDefault bool) domain.Folder {
	r.nextID++
	f := domain.Folder{
		ID:       domain.FolderID(fmt.Sprintf("fld_%06d", r.nextID)),
		Owner:    owner,
		Name:     name,
		Default:  isDefault,
		Position: r.countOf(owner),
	}
	r.items[f.ID] = f
	return f
}

// countOf counts owner's folders. Callers hold the lock.
func (r *FolderRepository) countOf(owner domain.OwnerID) int {
	n := 0
	for _, f := range r.items {
		if f.Owner == owner {
			n++
		}
	}
	return n
}
