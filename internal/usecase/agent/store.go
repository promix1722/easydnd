package agent

import (
	"context"
	"encoding/json"
	"slices"
	"sort"
	"sync"
	"time"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/types"
)

// Record is a session as a store keeps it: the columns a store decides by, the
// events, and everything else as one document only this package reads.
type Record struct {
	ID         string
	Owner      domain.OwnerID
	Status     string
	Revision   int
	Generation int
	Finished   bool
	Created    time.Time
	Document   []byte
	// Count is how many events the store held when the record was read. A
	// write stores the Events past it; ids are 1..n, so that is the new ones.
	Count  int
	Events []AgentEvent
	// Files is written only: the attachments a Create or Update adds.
	Files []AgentFile
}

// Store keeps sessions where every API process can reach them. A turn belongs
// to the instance that claimed it: Save and Append are refused -- false, no
// error -- once the lease is somebody else's or the generation has moved, and
// the caller drops what it was doing.
type Store interface {
	// Create refuses, with false, once the store holds limit sessions.
	Create(ctx context.Context, rec Record, limit int) (bool, error)
	// Get returns the whole session and counts as using it.
	Get(ctx context.Context, id string) (Record, error)
	// Tail is Get for a poll: no document, and only the events past after.
	Tail(ctx context.Context, id string, after int) (Record, error)
	List(ctx context.Context, owner domain.OwnerID) ([]Record, error)
	Files(ctx context.Context, id string) ([]AgentFile, error)
	// Update changes a session under its row lock, whoever holds the lease.
	Update(ctx context.Context, id string, change func(*Record) error) error
	// Claim takes one session that is queued, or running under a lease that
	// ran out, and marks it running for instance. busy are sessions the
	// instance is still unwinding and must not be handed again.
	Claim(ctx context.Context, instance string, lease time.Duration, busy []string) (Record, bool, error)
	Save(ctx context.Context, rec Record, instance string) (bool, error)
	Append(ctx context.Context, id, instance string, generation int, e AgentEvent) (bool, error)
	Delete(ctx context.Context, id string) error
	// Sweep deletes the sessions nobody has used for idle.
	Sweep(ctx context.Context, idle time.Duration) (int, error)
}

// ErrSessionNotFound is what every Store answers for a session it lacks.
func ErrSessionNotFound() error {
	return types.NewNotFoundError("import session not found").Because("agent.notFound")
}

type memorySession struct {
	rec        Record
	files      []AgentFile
	touched    time.Time
	leaseOwner string
	leaseUntil time.Time
}

// MemoryStore is the Store of a process without a database: development, the
// CLI and tests.
type MemoryStore struct {
	mu    sync.Mutex
	items map[string]*memorySession
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{items: map[string]*memorySession{}} }

func cloneEvents(events []AgentEvent) []AgentEvent {
	out := []AgentEvent{}
	_ = json.Unmarshal(raw(events), &out)
	return out
}

func (m *memorySession) view(from int, document bool) Record {
	out := m.rec
	out.Count = len(m.rec.Events)
	out.Events, out.Files = nil, nil
	if from >= 0 && from <= out.Count {
		out.Events = cloneEvents(m.rec.Events[from:])
	}
	out.Document = nil
	if document {
		out.Document = slices.Clone(m.rec.Document)
	}
	return out
}

// write applies what a Save or Update may change, as the SQL does.
func (m *memorySession) write(rec Record) {
	m.rec.Status, m.rec.Revision, m.rec.Generation, m.rec.Finished = rec.Status, rec.Revision, rec.Generation, rec.Finished
	m.rec.Document = slices.Clone(rec.Document)
	for _, e := range cloneEvents(rec.Events) {
		if e.ID > len(m.rec.Events) {
			m.rec.Events = append(m.rec.Events, e)
		}
	}
	for _, f := range rec.Files {
		m.files = append(m.files, AgentFile{Name: f.Name, MIME: f.MIME, Data: slices.Clone(f.Data)})
	}
	if rec.Status != "running" {
		m.leaseOwner, m.leaseUntil = "", time.Time{}
	}
	m.touched = time.Now()
}

func (r *MemoryStore) Create(_ context.Context, rec Record, limit int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.items) >= limit {
		return false, nil
	}
	m := &memorySession{rec: Record{ID: rec.ID, Owner: rec.Owner, Created: rec.Created}}
	m.write(rec)
	r.items[rec.ID] = m
	return true, nil
}
func (r *MemoryStore) Get(_ context.Context, id string) (Record, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.items[id]
	if m == nil {
		return Record{}, ErrSessionNotFound()
	}
	m.touched = time.Now()
	return m.view(0, true), nil
}
func (r *MemoryStore) Tail(_ context.Context, id string, after int) (Record, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.items[id]
	if m == nil {
		return Record{}, ErrSessionNotFound()
	}
	return m.view(after, false), nil
}
func (r *MemoryStore) List(_ context.Context, owner domain.OwnerID) ([]Record, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []Record{}
	for _, m := range r.items {
		if m.rec.Owner == owner {
			out = append(out, m.view(-1, true))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.Before(out[j].Created) })
	return out, nil
}
func (r *MemoryStore) Files(_ context.Context, id string) ([]AgentFile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.items[id]
	if m == nil {
		return nil, ErrSessionNotFound()
	}
	out := []AgentFile{}
	for _, f := range m.files {
		out = append(out, AgentFile{Name: f.Name, MIME: f.MIME, Data: slices.Clone(f.Data)})
	}
	return out, nil
}
func (r *MemoryStore) Update(_ context.Context, id string, change func(*Record) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.items[id]
	if m == nil {
		return ErrSessionNotFound()
	}
	rec := m.view(0, true)
	if err := change(&rec); err != nil {
		return err
	}
	m.write(rec)
	return nil
}
func (r *MemoryStore) Claim(_ context.Context, instance string, lease time.Duration, busy []string) (Record, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	var found *memorySession
	for _, m := range r.items {
		free := m.rec.Status == "queued" || m.rec.Status == "running" && m.leaseUntil.Before(now)
		if free && !slices.Contains(busy, m.rec.ID) && (found == nil || m.touched.Before(found.touched)) {
			found = m
		}
	}
	if found == nil {
		return Record{}, false, nil
	}
	found.rec.Status = "running"
	found.rec.Revision++
	found.leaseOwner, found.leaseUntil, found.touched = instance, now.Add(lease), now
	return found.view(0, true), true, nil
}
func (r *MemoryStore) held(id, instance string, generation int) *memorySession {
	m := r.items[id]
	if m == nil || m.leaseOwner != instance || m.rec.Generation != generation {
		return nil
	}
	return m
}
func (r *MemoryStore) Save(_ context.Context, rec Record, instance string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.held(rec.ID, instance, rec.Generation)
	if m == nil {
		return false, nil
	}
	m.write(rec)
	return true, nil
}
func (r *MemoryStore) Append(_ context.Context, id, instance string, generation int, e AgentEvent) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.held(id, instance, generation)
	if m == nil || e.ID != len(m.rec.Events)+1 {
		return false, nil
	}
	m.rec.Events = append(m.rec.Events, cloneEvents([]AgentEvent{e})...)
	m.touched = time.Now()
	return true, nil
}
func (r *MemoryStore) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.items, id)
	return nil
}
func (r *MemoryStore) Sweep(_ context.Context, idle time.Duration) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now, n := time.Now(), 0
	for id, m := range r.items {
		if m.touched.Before(now.Add(-idle)) && (m.rec.Status != "running" || m.leaseUntil.Before(now)) {
			delete(r.items, id)
			n++
		}
	}
	return n, nil
}
