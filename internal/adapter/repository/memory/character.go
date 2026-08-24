// Package memory provides map-backed repositories.
//
// It exists so the service compiles, runs and deploys with zero
// infrastructure. State is per-process and lost on restart.
//
// For accounts that is now only the development fallback: production stores
// them in internal/adapter/repository/postgres, and the two adapters are held
// to one contract by internal/adapter/repository/repotest. For characters and
// the folders they are filed in it is still the whole story: both are reachable
// over the API and both are lost on restart. A SQL sibling replaces either one
// without any change above this layer, exactly as the account store did.
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

// CharacterRepository is a concurrency-safe in-process character store.
//
// It models the storage shape DND.md specifies -- a character's whole log as
// one record -- so the optimistic-concurrency contract on Append is exercised
// here exactly as a SQL implementation would have to exercise it.
type CharacterRepository struct {
	mu sync.RWMutex
	// Stored by value, so a caller holding a Character cannot reach in and
	// mutate our state behind the mutex. The Log's event slice is cloned on
	// the way in and out for the same reason.
	items map[domain.ID]domain.Character
	// nextID counts issued identifiers. A real store would use the database's
	// own generator; a counter keeps this one deterministic for tests.
	nextID int
}

// NewCharacterRepository returns an empty store.
func NewCharacterRepository() *CharacterRepository {
	return &CharacterRepository{items: make(map[domain.ID]domain.Character)}
}

// Create stores a new empty character for owner, filed in folder.
func (r *CharacterRepository) Create(
	_ context.Context, owner domain.OwnerID, folder domain.FolderID,
) (domain.Character, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nextID++
	c := domain.Character{
		ID:     domain.ID(fmt.Sprintf("chr_%06d", r.nextID)),
		Owner:  owner,
		Folder: folder,
	}
	r.items[c.ID] = c
	return c, nil
}

// Get returns the character with the given ID.
func (r *CharacterRepository) Get(_ context.Context, id domain.ID) (domain.Character, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	c, ok := r.items[id]
	if !ok {
		return domain.Character{}, types.NewNotFoundError("character %q", id)
	}
	return clone(c), nil
}

// List returns every character owned by owner, sorted by ID.
//
// Whole characters, not summaries: a summary's name, level and class line are
// projections of the log, and a repository has no catalogue to project
// against. The application layer folds these with character.Summarize.
func (r *CharacterRepository) List(_ context.Context, owner domain.OwnerID) ([]domain.Character, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]domain.Character, 0, len(r.items))
	for _, c := range r.items {
		if c.Owner != owner {
			continue
		}
		out = append(out, clone(c))
	}
	// Map iteration order is randomised; callers deserve a stable order.
	slices.SortFunc(out, func(a, b domain.Character) int {
		return strings.Compare(a.ID.String(), b.ID.String())
	})
	return out, nil
}

// SetFolder files a character in another folder.
//
// There is no expectedSeq here and there should not be: a folder is not part of
// the log, so moving a character races with nothing that appends to it.
func (r *CharacterRepository) SetFolder(
	_ context.Context, id domain.ID, folder domain.FolderID,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.items[id]
	if !ok {
		return types.NewNotFoundError("character %q", id)
	}
	c.Folder = folder
	r.items[id] = c
	return nil
}

// Append adds events to a character's log, rejecting a stale expectedSeq.
func (r *CharacterRepository) Append(_ context.Context, id domain.ID, expectedSeq int, events ...domain.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.items[id]
	if !ok {
		return types.NewNotFoundError("character %q", id)
	}
	if got := c.Log.LastSeq(); got != expectedSeq {
		return types.NewValidationError("character %q is at sequence %d, not %d", id, got, expectedSeq)
	}
	// Append to a copy, so a rejected batch cannot leave the stored log
	// half-written.
	updated := domain.Log{Events: slices.Clone(c.Log.Events)}
	if err := updated.Append(events...); err != nil {
		return err
	}
	c.Log = updated
	r.items[id] = c
	return nil
}

// Truncate drops every event after afterSeq, rejecting a stale expectedSeq.
//
// The concurrency check is the same one Append makes and for the same reason:
// the whole log is one record, so two clients that read, modify and write it
// would otherwise have the later write discard the earlier silently. It
// matters more here, not less -- the write being discarded is a deletion.
func (r *CharacterRepository) Truncate(_ context.Context, id domain.ID, expectedSeq, afterSeq int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.items[id]
	if !ok {
		return types.NewNotFoundError("character %q", id)
	}
	if got := c.Log.LastSeq(); got != expectedSeq {
		return types.NewValidationError("character %q is at sequence %d, not %d", id, got, expectedSeq)
	}
	// Truncate a copy, so a rejected request cannot leave the stored log
	// half-trimmed.
	updated := domain.Log{Events: slices.Clone(c.Log.Events)}
	if err := updated.Truncate(afterSeq); err != nil {
		return err
	}
	c.Log = updated
	r.items[id] = c
	return nil
}

// Rewrite replaces a character's whole log, rejecting a stale expectedSeq.
//
// It is neither an append nor a truncation: replacing one entry can drop
// entries after it, so the sequence numbers close up and the stored slice is
// a different length in either direction. The caller hands over a log it has
// already rebuilt and revalidated; what is left here is the concurrency check
// and one last Validate, because a store that will accept a malformed log is
// a store that will hand one back.
func (r *CharacterRepository) Rewrite(_ context.Context, id domain.ID, expectedSeq int, log domain.Log) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.items[id]
	if !ok {
		return types.NewNotFoundError("character %q", id)
	}
	if got := c.Log.LastSeq(); got != expectedSeq {
		return types.NewValidationError("character %q is at sequence %d, not %d", id, got, expectedSeq)
	}
	if err := log.Validate(); err != nil {
		return err
	}
	// Cloned on the way in for the same reason it is cloned on the way out:
	// the caller must not keep a handle on our backing array.
	c.Log = domain.Log{Events: slices.Clone(log.Events)}
	r.items[id] = c
	return nil
}

// Delete removes a character.
func (r *CharacterRepository) Delete(_ context.Context, id domain.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.items[id]; !ok {
		return types.NewNotFoundError("character %q", id)
	}
	delete(r.items, id)
	return nil
}

// clone deep-copies the parts of a Character that a caller could otherwise
// mutate through a shared backing array.
func clone(c domain.Character) domain.Character {
	c.Log.Events = slices.Clone(c.Log.Events)
	return c
}
