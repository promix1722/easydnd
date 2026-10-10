package memory

import (
	"context"
	"encoding/json"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
	"slices"
	"sort"
	"sync"
)

type PackRepository struct {
	mu      sync.Mutex
	records map[string]pack.Record
	shares  map[string]map[string]pack.Share
	private map[pack.Release]pack.Document
	grants  map[user.ID][]string
}

func NewPackRepository() *PackRepository {
	return &PackRepository{records: map[string]pack.Record{}, shares: map[string]map[string]pack.Share{}, private: map[pack.Release]pack.Document{}, grants: map[user.ID][]string{}}
}
func clonePack[T any](v T) T {
	b, _ := json.Marshal(v)
	var out T
	_ = json.Unmarshal(b, &out)
	return out
}
func (r *PackRepository) ListFor(_ context.Context, owner user.ID, ids []string) ([]pack.Record, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []pack.Record{}
	for _, v := range r.records {
		if (owner != "" && v.Owner == owner) || slices.Contains(ids, v.ID) {
			out = append(out, clonePack(v))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func (r *PackRepository) Get(_ context.Context, id string) (pack.Record, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.records[id]
	if !ok {
		return v, types.NewNotFoundError("pack not found")
	}
	return clonePack(v), nil
}
func (r *PackRepository) Save(_ context.Context, v pack.Record, expected int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	old := r.records[v.ID]
	if old.Revision != expected || (expected > 0 && old.Owner != v.Owner) {
		return types.NewValidationError("stale pack revision").Because("pack.stale")
	}
	v.Revision = expected + 1
	r.records[v.ID] = clonePack(v)
	return nil
}
func (r *PackRepository) Shares(_ context.Context, g string) ([]pack.Share, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []pack.Share{}
	for _, v := range r.shares[g] {
		out = append(out, clonePack(v))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Pack < out[j].Pack })
	return out, nil
}
func (r *PackRepository) PutShare(_ context.Context, v pack.Share) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.shares[v.Group] == nil {
		r.shares[v.Group] = map[string]pack.Share{}
	}
	r.shares[v.Group][v.Pack] = clonePack(v)
	return nil
}
func (r *PackRepository) DeleteShare(_ context.Context, g, p string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.shares[g], p)
	return nil
}
func (r *PackRepository) Grants(_ context.Context, u user.ID) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string{}, r.grants[u]...), nil
}
func (r *PackRepository) SetGrants(_ context.Context, u user.ID, packs []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	packs = append([]string{}, packs...)
	sort.Strings(packs)
	r.grants[u] = slices.Compact(packs)
	return nil
}
func (r *PackRepository) PutPrivate(_ context.Context, d pack.Document) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.private[d.Release] = clonePack(d)
	return nil
}
func (r *PackRepository) GetPrivate(_ context.Context, release pack.Release) (pack.Document, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.private[release]
	if !ok {
		return pack.Document{}, types.NewNotFoundError("private release not found")
	}
	return clonePack(d), nil
}
