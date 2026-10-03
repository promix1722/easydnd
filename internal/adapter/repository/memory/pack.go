package memory

import (
	"context"
	"encoding/json"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/types"
	"sort"
	"sync"
)

type PackRepository struct {
	mu      sync.Mutex
	records map[string]pack.Record
	shares  map[string]map[string]pack.Share
}

func NewPackRepository() *PackRepository {
	return &PackRepository{records: map[string]pack.Record{}, shares: map[string]map[string]pack.Share{}}
}
func clonePack[T any](v T) T {
	b, _ := json.Marshal(v)
	var out T
	_ = json.Unmarshal(b, &out)
	return out
}
func (r *PackRepository) List(context.Context) ([]pack.Record, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []pack.Record{}
	for _, v := range r.records {
		out = append(out, clonePack(v))
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
