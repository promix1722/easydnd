// Package spellicon queues development-time spell icon generation.
//
// One job runs at a time -- the requests it makes cost credit, so the queue
// is bounded to a small worker pool rather than unlimited fan-out -- and
// each account reads back only the state of the last job it asked for,
// because the spell names in a queue can come from a private pack nobody
// else may see.
// The package is a usecase: it holds no net/http, os or storage imports.
// Generating an image and keeping the result are ports the app wires to
// adapters under internal/adapter/imagegen.
package spellicon

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"sync"
	"sync/atomic"
	"time"

	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// The reason slugs the client renders. They are keys into
// web/locales/*.json, never sentences.
const (
	ReasonInvalidRequest = "icon_generation_invalid_request"
	ReasonBusy           = "icon_generation_busy"
	ReasonNotConfigured  = "icon_generation_not_configured"
	// ReasonPackUnconfigured names a namespaced slug whose pack has no
	// image_generation.pack_dirs entry; the store raises it before any paid
	// request so artwork can never be filed into the wrong repository. Its
	// Args carry "pack", the unmapped namespace.
	ReasonPackUnconfigured = "icon_generation_pack_unconfigured"
	ReasonFailed           = "icon_generation_failed"
	ReasonExists           = "icon_generation_exists"
)

// Spell is the slice of a catalogue entry a prompt needs: which icon this is,
// and the English words to draw it from.
type Spell struct {
	Slug   string
	Name   string
	School string
}

// Generator draws one icon from a prompt and returns complete PNG bytes.
// internal/adapter/imagegen/openai is the production implementation.
type Generator interface {
	Generate(ctx context.Context, prompt string) ([]byte, error)
}

// Store keeps finished icons. The slug it validates and resolves is a
// catalogue key: "acid-arrow" or "dnd-2014/acid-arrow". A qualified key's
// namespace is a pack the store has a directory for; a namespace it does not
// know is rejected as a validation error, not treated as missing.
type Store interface {
	// Existing reports whether a finished icon already exists for slug, and
	// the revision that identifies its bytes. A namespaced slug that has no
	// icon of its own falls back to the legacy icon of its last segment.
	Existing(ctx context.Context, slug string) (revision string, exists bool, err error)

	// Save persists a generated PNG as the finished icon for slug and
	// returns the new revision. A failed Save leaves any previous icon in
	// place.
	Save(ctx context.Context, slug string, png []byte) (revision string, err error)
}

// ItemState is the lifecycle of one queue entry, in wire form.
type ItemState string

const (
	ItemQueued     ItemState = "queued"
	ItemGenerating ItemState = "generating"
	ItemDone       ItemState = "done"
	ItemSkipped    ItemState = "skipped"
	ItemFailed     ItemState = "failed"
)

// Item is one queue entry as the client sees it. Reason is set when State
// explains itself (skipped, failed); Revision is the cache-buster the client
// appends to the icon URL once artwork exists.
type Item struct {
	State    ItemState `json:"state"`
	Reason   string    `json:"reason,omitempty"`
	Revision string    `json:"revision,omitempty"`
}

// State is the snapshot the GET endpoint serves and POST returns. Its JSON
// shape is the client contract; field names are fixed.
type State struct {
	Configured bool            `json:"configured"`
	Running    bool            `json:"running"`
	Total      int             `json:"total"`
	Completed  int             `json:"completed"`
	Skipped    int             `json:"skipped"`
	Failed     int             `json:"failed"`
	Items      map[string]Item `json:"items"`
}

// job is the one queue in flight. Its context belongs to the service, not to
// the request that started it, so a finished HTTP response cannot cancel paid
// work; Close cancels and waits instead.
type job struct {
	owner  user.ID
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	state  State
}

// Service owns the queue.
type Service struct {
	generator Generator
	store     Store
	workers   int
	timeout   time.Duration
	log       *slog.Logger

	mu      sync.Mutex
	running bool
	closed  bool
	job     *job
	states  map[user.ID]State
}

// NewService builds the queue. workers bounds how many icons generate at
// once; below one it is clamped rather than left to starve the queue. A nil
// generator means the feature is not configured; a nil logger means the
// default. The store is required.
func NewService(generator Generator, store Store, workers int, timeout time.Duration, log *slog.Logger) *Service {
	if workers < 1 {
		workers = 1
	}
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		generator: generator,
		store:     store,
		workers:   workers,
		timeout:   timeout,
		log:       log,
		states:    map[user.ID]State{},
	}
}

// Start queues one icon per unique spell slug and returns the queue as it
// stands once existing artwork has been counted.
//
// Slugs already holding a finished icon are skipped before any generation
// unless replace is set, so a request that asks for nothing new runs to
// completion even when no generator is configured -- the missing provider is
// only an error when there is paid work to do.
func (s *Service) Start(ctx context.Context, owner user.ID, spells []Spell, replace bool) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return State{}, types.NewServerError("spell icon queue is closed")
	}
	if s.running {
		return State{}, types.NewValidationError("an icon generation job is already running").
			Because(ReasonBusy)
	}

	pending := make([]Spell, 0, len(spells))
	st := State{Configured: s.generator != nil, Running: true, Items: map[string]Item{}}
	for _, spell := range spells {
		if _, dup := st.Items[spell.Slug]; dup {
			continue
		}
		st.Total++
		st.Items[spell.Slug] = Item{State: ItemQueued}

		revision, exists, err := s.store.Existing(ctx, spell.Slug)
		if err != nil {
			var invalid *types.ValidationError
			if errors.As(err, &invalid) {
				return State{}, invalid
			}
			return State{}, types.WrapServerError(err, "check existing icon %q", spell.Slug)
		}
		if exists && !replace {
			st.Items[spell.Slug] = Item{State: ItemSkipped, Reason: ReasonExists, Revision: revision}
			st.Skipped++
			st.Completed++
			continue
		}
		pending = append(pending, spell)
	}

	if len(pending) > 0 && s.generator == nil {
		return State{}, types.NewValidationError("icon generation is not configured").
			Because(ReasonNotConfigured)
	}

	j := &job{owner: owner, state: st}
	j.ctx, j.cancel = context.WithCancel(context.Background())
	j.done = make(chan struct{})
	s.job = j
	s.running = true
	s.states[owner] = st
	go s.work(j, pending)
	return copyState(st), nil
}

// Status returns the state of the job owner last started, or a zero state if
// they never did. Running is global: while another owner's job holds the
// queue, this owner's snapshot still says so, without revealing the queue's
// contents.
func (s *Service) Status(owner user.ID) State {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[owner]
	if s.running && s.job != nil && s.job.owner == owner {
		st = s.job.state
	}
	st.Running = s.running
	st.Configured = s.generator != nil
	return copyState(st)
}

// Close refuses new jobs, cancels the running one and waits for it to finish
// counting what it abandoned.
func (s *Service) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	j := s.job
	if j != nil {
		j.cancel()
	}
	s.mu.Unlock()
	if j != nil {
		<-j.done
	}
}

// work claims pending items through one shared index, without copying the
// pending slice into another queue. Each slot includes generation and saving.
func (s *Service) work(j *job, pending []Spell) {
	defer close(j.done)
	defer j.cancel()
	defer s.finish(j)

	var next atomic.Uint64

	var wg sync.WaitGroup
	for range min(s.workers, len(pending)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j.ctx.Err() == nil {
				index := int(next.Add(1)) - 1
				if index >= len(pending) {
					return
				}
				s.runItem(j, &pending[index])
			}
		}()
	}
	wg.Wait()

	// Whatever never ran -- a Close, or a context cancelled upstream -- is
	// counted failed so completed always reaches total.
	s.mu.Lock()
	for _, item := range pending {
		if j.state.Items[item.Slug].State == ItemQueued || j.state.Items[item.Slug].State == ItemGenerating {
			j.state.Items[item.Slug] = Item{State: ItemFailed, Reason: ReasonFailed}
			j.state.Failed++
			j.state.Completed++
		}
	}
	s.mu.Unlock()
}

// runItem carries one icon from generating through saved, on a per-item
// timeout layered over the job's context.
func (s *Service) runItem(j *job, spell *Spell) {
	s.mark(spell.Slug, ItemGenerating, "")

	ctx := j.ctx
	var cancel context.CancelFunc
	if s.timeout > 0 {
		ctx, cancel = context.WithTimeout(j.ctx, s.timeout)
	}

	prompt := Prompt(spell.Name, spell.School)
	png, err := s.generator.Generate(ctx, prompt)
	if cancel != nil {
		cancel()
	}
	if err == nil {
		var revision string
		revision, err = s.store.Save(j.ctx, spell.Slug, png)
		if err == nil {
			s.mark(spell.Slug, ItemDone, revision)
			return
		}
	}
	// The provider's own error text can carry anything upstream echoed back;
	// the item gets the generic slug, the detail goes to the log.
	s.log.Warn("spell icon generation failed", "slug", spell.Slug, "error", err)
	s.fail(spell.Slug)
}

// mark advances one item; only a settled state counts toward completed.
func (s *Service) mark(slug string, state ItemState, revision string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := s.job.state.Items[slug]
	item.State = state
	item.Revision = revision
	s.job.state.Items[slug] = item
	if state != ItemGenerating {
		s.job.state.Completed++
	}
}

func (s *Service) fail(slug string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := s.job.state.Items[slug]
	item.State = ItemFailed
	item.Reason = ReasonFailed
	s.job.state.Items[slug] = item
	s.job.state.Failed++
	s.job.state.Completed++
}

// finish hands the last state to the owner and frees the queue.
func (s *Service) finish(j *job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j.state.Running = false
	s.states[j.owner] = j.state
	s.job = nil
	s.running = false
}

func copyState(st State) State {
	st.Items = maps.Clone(st.Items)
	if st.Items == nil {
		st.Items = map[string]Item{}
	}
	return st
}
