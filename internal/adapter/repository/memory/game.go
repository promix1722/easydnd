package memory

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/promix1722/easydnd/internal/domain/character"
	domain "github.com/promix1722/easydnd/internal/domain/game"
	"github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/types"
)

// SharedRepository is a concurrency-safe in-process store for the characters
// groups have shared.
//
// In memory is not a placeholder here in the way it is for accounts, which
// have a Postgres sibling. Every row points at a character id, and a character
// id is the counter in CharacterRepository below -- so this store is exactly
// as durable as the thing it refers to, on purpose. A SQL sibling arrives when
// characters do, and not before.
type SharedRepository struct {
	mu sync.RWMutex
	// Keyed by group, then character, which is the shape of every question
	// asked of it: what is at this table, and is this character at it.
	items map[group.ID]map[character.ID]domain.Shared
	// order remembers the sequence characters were shared in per group, so a
	// listing does not depend on map iteration order.
	order map[group.ID][]character.ID
}

// NewSharedRepository returns an empty pool.
func NewSharedRepository() *SharedRepository {
	return &SharedRepository{
		items: make(map[group.ID]map[character.ID]domain.Shared),
		order: make(map[group.ID][]character.ID),
	}
}

// Share puts s in its group's pool.
func (r *SharedRepository) Share(_ context.Context, s domain.Shared) error {
	switch {
	case s.Group == "":
		return types.NewValidationError("a shared character must name a group")
	case s.Character == "":
		return types.NewValidationError("a shared character must name a character")
	case s.Owner == "":
		return types.NewValidationError("a shared character must name an owner")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	pool, ok := r.items[s.Group]
	if !ok {
		pool = make(map[character.ID]domain.Shared)
		r.items[s.Group] = pool
	}
	if _, exists := pool[s.Character]; exists {
		return types.NewValidationError("character %q is already shared with this group", s.Character)
	}
	pool[s.Character] = s
	r.order[s.Group] = append(r.order[s.Group], s.Character)
	return nil
}

// Unshare takes c out of g's pool.
func (r *SharedRepository) Unshare(_ context.Context, g group.ID, c character.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	pool, ok := r.items[g]
	if !ok {
		return types.NewNotFoundError("character %q is not shared with this group", c)
	}
	if _, exists := pool[c]; !exists {
		return types.NewNotFoundError("character %q is not shared with this group", c)
	}
	r.drop(g, c)
	return nil
}

// drop removes one character from one pool. The caller holds the lock.
func (r *SharedRepository) drop(g group.ID, c character.ID) {
	delete(r.items[g], c)
	if len(r.items[g]) == 0 {
		delete(r.items, g)
	}
	rest := slices.DeleteFunc(r.order[g], func(id character.ID) bool { return id == c })
	if len(rest) == 0 {
		delete(r.order, g)
		return
	}
	r.order[g] = rest
}

// List returns g's whole pool, in the order characters were shared.
func (r *SharedRepository) List(_ context.Context, g group.ID) ([]domain.Shared, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	pool := r.items[g]
	out := make([]domain.Shared, 0, len(pool))
	for _, id := range r.order[g] {
		if s, ok := pool[id]; ok {
			out = append(out, s)
		}
	}
	return out, nil
}

// IsShared reports whether c is in g's pool.
func (r *SharedRepository) IsShared(_ context.Context, g group.ID, c character.ID) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.items[g][c]
	return ok, nil
}

// GroupsSharing returns every group c has been shared into, sorted so that the
// answer does not depend on map order.
func (r *SharedRepository) GroupsSharing(_ context.Context, c character.ID) ([]group.ID, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []group.ID
	for g, pool := range r.items {
		if _, ok := pool[c]; ok {
			out = append(out, g)
		}
	}
	slices.Sort(out)
	return out, nil
}

// UnshareEverywhere removes c from every pool.
func (r *SharedRepository) UnshareEverywhere(_ context.Context, c character.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for g, pool := range r.items {
		if _, ok := pool[c]; ok {
			r.drop(g, c)
		}
	}
	return nil
}

// ClearGroup empties g's pool.
func (r *SharedRepository) ClearGroup(_ context.Context, g group.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.items, g)
	delete(r.order, g)
	return nil
}

// GameRepository is a concurrency-safe in-process game store.
//
// Games are stored by value and their rosters are cloned on the way in and
// out, so a caller holding one cannot reach in and mutate state behind the
// mutex -- the rule CharacterRepository follows for the same reason.
type GameRepository struct {
	mu      sync.RWMutex
	items   map[domain.ID]domain.Game
	rosters map[domain.ID][]domain.Entry
}

// NewGameRepository returns an empty store.
func NewGameRepository() *GameRepository {
	return &GameRepository{
		items:   make(map[domain.ID]domain.Game),
		rosters: make(map[domain.ID][]domain.Entry),
	}
}

// Create stores g.
func (r *GameRepository) Create(_ context.Context, g domain.Game) error {
	switch {
	case g.ID == "":
		return types.NewValidationError("game id must not be empty")
	case g.Group == "":
		return types.NewValidationError("a game must belong to a group")
	case g.Name == "":
		return types.NewValidationError("game name must not be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.items[g.ID]; exists {
		return types.NewValidationError("game %q already exists", g.ID)
	}
	r.items[g.ID] = g
	return nil
}

// ByID returns the game.
func (r *GameRepository) ByID(_ context.Context, id domain.ID) (domain.Game, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	g, ok := r.items[id]
	if !ok {
		return domain.Game{}, types.NewNotFoundError("game %q", id)
	}
	return g, nil
}

// ListFor returns every game at g's table, most recently created first.
func (r *GameRepository) ListFor(_ context.Context, g group.ID) ([]domain.Game, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]domain.Game, 0)
	for _, item := range r.items {
		if item.Group == g {
			out = append(out, item)
		}
	}
	// Newest first, with the id as a tiebreak: two games created inside one
	// clock tick would otherwise come back in map order, which is randomised.
	slices.SortFunc(out, func(a, b domain.Game) int {
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return b.CreatedAt.Compare(a.CreatedAt)
		}
		return strings.Compare(string(a.ID), string(b.ID))
	})
	return out, nil
}

// Rename changes the game's name.
func (r *GameRepository) Rename(_ context.Context, id domain.ID, name string) error {
	if name == "" {
		return types.NewValidationError("game name must not be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	g, ok := r.items[id]
	if !ok {
		return types.NewNotFoundError("game %q", id)
	}
	g.Name = name
	r.items[id] = g
	return nil
}

// Delete removes the game and its roster.
func (r *GameRepository) Delete(_ context.Context, id domain.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.items[id]; !ok {
		return types.NewNotFoundError("game %q", id)
	}
	delete(r.items, id)
	delete(r.rosters, id)
	return nil
}

// DeleteForGroup removes every game at g's table.
func (r *GameRepository) DeleteForGroup(_ context.Context, g group.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, item := range r.items {
		if item.Group == g {
			delete(r.items, id)
			delete(r.rosters, id)
		}
	}
	return nil
}

// AddCharacters seats cs at id's table, leaving those already seated alone.
func (r *GameRepository) AddCharacters(
	_ context.Context, id domain.ID, cs []character.ID, at time.Time,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.items[id]; !ok {
		return types.NewNotFoundError("game %q", id)
	}
	// Build the whole new roster before committing it, so a rejected batch
	// leaves nothing half-written.
	roster := slices.Clone(r.rosters[id])
	seated := make(map[character.ID]struct{}, len(roster))
	for _, e := range roster {
		seated[e.Character] = struct{}{}
	}
	for _, c := range cs {
		if c == "" {
			return types.NewValidationError("a roster entry must name a character")
		}
		if _, ok := seated[c]; ok {
			continue
		}
		seated[c] = struct{}{}
		roster = append(roster, domain.Entry{Character: c, AddedAt: at})
	}
	r.rosters[id] = roster
	return nil
}

// RemoveCharacter takes c off id's roster.
func (r *GameRepository) RemoveCharacter(
	_ context.Context, id domain.ID, c character.ID,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.items[id]; !ok {
		return types.NewNotFoundError("game %q", id)
	}
	roster := r.rosters[id]
	rest := slices.DeleteFunc(slices.Clone(roster), func(e domain.Entry) bool {
		return e.Character == c
	})
	if len(rest) == len(roster) {
		return types.NewNotFoundError("character %q is not in this game", c)
	}
	r.rosters[id] = rest
	return nil
}

// Characters returns id's roster, in the order characters were added.
func (r *GameRepository) Characters(_ context.Context, id domain.ID) ([]domain.Entry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, ok := r.items[id]; !ok {
		return nil, types.NewNotFoundError("game %q", id)
	}
	return slices.Clone(r.rosters[id]), nil
}

// RemoveFromGroupGames drops c from every game at g's table.
func (r *GameRepository) RemoveFromGroupGames(
	_ context.Context, g group.ID, c character.ID,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, item := range r.items {
		if item.Group != g {
			continue
		}
		r.rosters[id] = slices.DeleteFunc(slices.Clone(r.rosters[id]), func(e domain.Entry) bool {
			return e.Character == c
		})
	}
	return nil
}

var (
	_ domain.SharedRepository = (*SharedRepository)(nil)
	_ domain.Repository       = (*GameRepository)(nil)
)
