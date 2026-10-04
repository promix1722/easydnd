package spellicon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
)

// fakeGen answers Generate immediately, or blocks on a channel until the
// test releases it, so ordering assertions stay deterministic.
type fakeGen struct {
	mu      sync.Mutex
	prompts []string
	err     error
	block   chan struct{}
}

func (f *fakeGen) Generate(ctx context.Context, prompt string) ([]byte, error) {
	f.mu.Lock()
	f.prompts = append(f.prompts, prompt)
	f.mu.Unlock()
	if f.block != nil {
		select {
		case <-f.block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if f.err != nil {
		return nil, f.err
	}
	return []byte("png:" + prompt), nil
}

func (f *fakeGen) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.prompts)
}

type fakeStore struct {
	mu       sync.Mutex
	existing map[string]string
	saved    map[string]string
	existErr error
	saveErr  error
}

func (f *fakeStore) Existing(_ context.Context, slug string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.existErr != nil {
		return "", false, f.existErr
	}
	rev, ok := f.existing[slug]
	return rev, ok, nil
}

func (f *fakeStore) Save(_ context.Context, slug string, png []byte) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.saveErr != nil {
		return "", f.saveErr
	}
	if f.saved == nil {
		f.saved = map[string]string{}
	}
	f.saved[slug] = "rev:" + slug
	return "rev:" + slug, nil
}

func newFakeStore() *fakeStore {
	return &fakeStore{existing: map[string]string{}}
}

func newTestService(gen Generator, store Store) *Service {
	return NewService(gen, store, 10, time.Minute, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func waitDone(t *testing.T, s *Service, owner user.ID) State {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		st := s.Status(owner)
		if !st.Running {
			return st
		}
		if time.Now().After(deadline) {
			t.Fatalf("job never finished: %+v", st)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestStartGeneratesSkipsAndDedupes(t *testing.T) {
	store := newFakeStore()
	store.existing["shield"] = "old-rev"
	gen := &fakeGen{}
	s := newTestService(gen, store)
	defer s.Close()

	spells := []Spell{
		{Slug: "fireball", Name: "Fireball", School: "evocation"},
		{Slug: "shield", Name: "Shield", School: "abjuration"},
		{Slug: "fireball", Name: "Fireball", School: "evocation"}, // duplicate
	}
	st, err := s.Start(context.Background(), "owner", spells, false)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Configured || !st.Running {
		t.Fatalf("expected configured running state, got %+v", st)
	}
	if st.Total != 2 || st.Skipped != 1 || st.Completed != 1 {
		t.Fatalf("want total=2 skipped=1 completed=1, got %+v", st)
	}
	if got := st.Items["shield"]; got.State != ItemSkipped || got.Reason != ReasonExists || got.Revision != "old-rev" {
		t.Fatalf("shield should be skipped with its revision, got %+v", got)
	}
	if got := st.Items["fireball"]; got.State != ItemQueued {
		t.Fatalf("fireball should start queued, got %+v", got)
	}

	final := waitDone(t, s, "owner")
	if final.Completed != 2 || final.Failed != 0 {
		t.Fatalf("job should finish 2/2 without failures, got %+v", final)
	}
	item := final.Items["fireball"]
	if item.State != ItemDone || item.Revision != "rev:fireball" {
		t.Fatalf("fireball should be done with revision, got %+v", item)
	}
	if gen.calls() != 1 {
		t.Fatalf("generator should have run once, ran %d", gen.calls())
	}
	if store.saved["fireball"] == "" {
		t.Fatal("store.Save never ran for fireball")
	}
}

func TestBusyWhileRunning(t *testing.T) {
	gen := &fakeGen{block: make(chan struct{})}
	s := newTestService(gen, newFakeStore())
	defer s.Close()

	_, err := s.Start(context.Background(), "owner", []Spell{{Slug: "a", Name: "A"}}, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Start(context.Background(), "owner", []Spell{{Slug: "b", Name: "B"}}, false)
	var verr *types.ValidationError
	if !errors.As(err, &verr) || verr.Reason != ReasonBusy {
		t.Fatalf("want busy validation error, got %v", err)
	}

	// A different owner sees the queue as running but learns nothing of its
	// contents -- the slugs could come from a private pack.
	other := s.Status("other")
	if !other.Running || other.Total != 0 || len(other.Items) != 0 {
		t.Fatalf("other owner's state should be an empty running shell, got %+v", other)
	}

	close(gen.block)
	waitDone(t, s, "owner")
}

func TestMissingKeyRejectedOnlyWhenGenerationNeeded(t *testing.T) {
	store := newFakeStore()
	store.existing["shield"] = "old-rev"
	s := newTestService(nil, store)
	defer s.Close()

	// All-existing is free: no provider needed.
	st, err := s.Start(context.Background(), "o", []Spell{{Slug: "shield", Name: "S", School: "abjuration"}}, false)
	if err != nil {
		t.Fatalf("all-existing request should succeed unconfigured, got %v", err)
	}
	if st.Configured {
		t.Fatal("state should report unconfigured")
	}
	final := waitDone(t, s, "o")
	if final.Items["shield"].State != ItemSkipped || final.Completed != 1 {
		t.Fatalf("shield should be skipped, got %+v", final)
	}

	// Real work with no provider is the rejection.
	_, err = s.Start(context.Background(), "o", []Spell{{Slug: "fireball", Name: "F", School: "evocation"}}, false)
	var verr *types.ValidationError
	if !errors.As(err, &verr) || verr.Reason != ReasonNotConfigured {
		t.Fatalf("want not-configured rejection, got %v", err)
	}
}

func TestReplaceRegeneratesExisting(t *testing.T) {
	store := newFakeStore()
	store.existing["shield"] = "old-rev"
	gen := &fakeGen{}
	s := newTestService(gen, store)
	defer s.Close()

	st, err := s.Start(context.Background(), "o", []Spell{{Slug: "shield", Name: "S", School: "abjuration"}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if st.Items["shield"].State != ItemQueued {
		t.Fatalf("replace should queue an existing icon, got %+v", st.Items["shield"])
	}
	final := waitDone(t, s, "o")
	if final.Items["shield"].State != ItemDone || final.Items["shield"].Revision != "rev:shield" {
		t.Fatalf("shield should be regenerated, got %+v", final.Items["shield"])
	}
	if gen.calls() != 1 {
		t.Fatal("generator should have run for the replaced icon")
	}
}

func TestGenerationFailureMarksItem(t *testing.T) {
	gen := &fakeGen{err: errors.New("provider exploded with secret detail")}
	s := newTestService(gen, newFakeStore())
	defer s.Close()

	_, err := s.Start(context.Background(), "o", []Spell{{Slug: "a", Name: "A"}, {Slug: "b", Name: "B"}}, false)
	if err != nil {
		t.Fatal(err)
	}
	final := waitDone(t, s, "o")
	if final.Failed != 2 || final.Completed != 2 {
		t.Fatalf("both items should have failed, got %+v", final)
	}
	for _, slug := range []string{"a", "b"} {
		item := final.Items[slug]
		if item.State != ItemFailed || item.Reason != ReasonFailed {
			t.Fatalf("%s should be failed with generic reason, got %+v", slug, item)
		}
		// The provider's own text must not leak into the state.
		if strings.Contains(item.Reason, "secret detail") {
			t.Fatalf("provider error leaked into item reason: %+v", item)
		}
	}
}

func TestSaveFailureMarksItem(t *testing.T) {
	store := newFakeStore()
	store.saveErr = errors.New("disk full")
	s := newTestService(&fakeGen{}, store)
	defer s.Close()

	if _, err := s.Start(context.Background(), "o", []Spell{{Slug: "a", Name: "A"}}, false); err != nil {
		t.Fatal(err)
	}
	final := waitDone(t, s, "o")
	if final.Items["a"].State != ItemFailed || final.Failed != 1 {
		t.Fatalf("save failure should fail the item, got %+v", final.Items["a"])
	}
}

func TestCloseCancelsQueuedWork(t *testing.T) {
	gen := &fakeGen{block: make(chan struct{})}
	s := newTestService(gen, newFakeStore())

	if _, err := s.Start(context.Background(), "o",
		[]Spell{{Slug: "a", Name: "A"}, {Slug: "b", Name: "B"}, {Slug: "c", Name: "C"}}, false); err != nil {
		t.Fatal(err)
	}
	// Wait until generation is actually in flight, then shut down.
	deadline := time.Now().Add(5 * time.Second)
	for gen.calls() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("generation never started")
		}
		time.Sleep(time.Millisecond)
	}
	s.Close()

	final := s.Status("o")
	if final.Running {
		t.Fatal("Close must wait for the job")
	}
	if final.Completed != final.Total || final.Total != 3 {
		t.Fatalf("every queued item must be counted, got %+v", final)
	}
	if final.Failed != 3 {
		t.Fatalf("cancelled items count as failed, got %+v", final)
	}

	// After Close the service refuses new work.
	if _, err := s.Start(context.Background(), "o", []Spell{{Slug: "x", Name: "X"}}, false); err == nil {
		t.Fatal("Start after Close should fail")
	}
}

func TestRequestContextDoesNotOwnTheJob(t *testing.T) {
	gen := &fakeGen{}
	s := newTestService(gen, newFakeStore())
	defer s.Close()

	ctx, cancel := context.WithCancel(context.Background())
	if _, err := s.Start(ctx, "o", []Spell{{Slug: "a", Name: "A"}}, false); err != nil {
		t.Fatal(err)
	}
	cancel() // the HTTP request ended; the queue must still finish
	final := waitDone(t, s, "o")
	if final.Items["a"].State != ItemDone {
		t.Fatalf("request cancellation must not cancel the queue, got %+v", final.Items["a"])
	}
}

func TestStoreValidationErrorSurfaces(t *testing.T) {
	store := newFakeStore()
	store.existErr = types.NewValidationError("invalid icon slug %q", "../evil").Because(ReasonInvalidRequest)
	s := newTestService(&fakeGen{}, store)
	defer s.Close()

	_, err := s.Start(context.Background(), "o", []Spell{{Slug: "../evil"}}, false)
	var verr *types.ValidationError
	if !errors.As(err, &verr) || verr.Reason != ReasonInvalidRequest {
		t.Fatalf("want invalid_request, got %v", err)
	}
}

func TestPerOwnerStates(t *testing.T) {
	gen := &fakeGen{}
	s := newTestService(gen, newFakeStore())
	defer s.Close()

	if _, err := s.Start(context.Background(), "alice", []Spell{{Slug: "a", Name: "A"}}, false); err != nil {
		t.Fatal(err)
	}
	waitDone(t, s, "alice")
	if _, err := s.Start(context.Background(), "bob", []Spell{{Slug: "b", Name: "B"}}, false); err != nil {
		t.Fatal(err)
	}
	waitDone(t, s, "bob")

	alice := s.Status("alice")
	if _, ok := alice.Items["a"]; !ok || len(alice.Items) != 1 {
		t.Fatalf("alice should keep her own last job, got %+v", alice.Items)
	}
	bob := s.Status("bob")
	if _, ok := bob.Items["b"]; !ok || len(bob.Items) != 1 {
		t.Fatalf("bob should keep his own last job, got %+v", bob.Items)
	}
}

// The wire shape is a client contract: the State fields marshal to exactly
// configured, running, total, completed, skipped, failed, items, and items
// carry state plus optional reason and revision.
func TestStateJSONShape(t *testing.T) {
	data, err := json.Marshal(State{
		Configured: true,
		Total:      2,
		Completed:  1,
		Skipped:    1,
		Items: map[string]Item{
			"a": {State: ItemDone, Revision: "r1"},
			"b": {State: ItemSkipped, Reason: ReasonExists},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	want := []string{"configured", "running", "total", "completed", "skipped", "failed", "items"}
	for _, key := range want {
		if _, ok := wire[key]; !ok {
			t.Fatalf("state JSON missing %q: %s", key, data)
		}
	}
	items := wire["items"].(map[string]any)
	done := items["a"].(map[string]any)
	if done["state"] != "done" || done["revision"] != "r1" {
		t.Fatalf("done item shape wrong: %s", data)
	}
	if _, ok := done["reason"]; ok {
		t.Fatalf("reason must be omitted when empty: %s", data)
	}
	skipped := items["b"].(map[string]any)
	if skipped["state"] != "skipped" || skipped["reason"] != "icon_generation_exists" {
		t.Fatalf("skipped item shape wrong: %s", data)
	}
	// A zero state marshals items as {}, not null.
	zero, _ := json.Marshal(State{Items: map[string]Item{}})
	var zeroMap map[string]any
	if err := json.Unmarshal(zero, &zeroMap); err != nil {
		t.Fatal(err)
	}
	if z, ok := zeroMap["items"].(map[string]any); !ok || len(z) != 0 {
		t.Fatalf("empty items must marshal {}, got %s", zero)
	}
}

func TestPromptMatchesRuneArtDirection(t *testing.T) {
	got := Prompt("Fireball", "evocation")
	want := `Hand-drawn glowing rune icon for the D&D spell "Fireball", in the style of a fantasy RPG spellbook pictogram: one continuous freehand line like a chalk sigil lit from within, fiery orange-red neon light with a brighter core and a faint halo around the strokes, slightly rough edges, meant to glow against a dark background. Single centered subject, readable at small size. No text, no letters, no border, no frame, transparent background.`
	if got != want {
		t.Fatalf("prompt drifted from the art direction:\n%s", got)
	}
	// Namespaced schools take the last segment; unknown schools fall back.
	if p := Prompt("X", "dnd-2014/conjuration"); !strings.Contains(p, "emerald-teal") {
		t.Fatalf("namespaced school lost its palette: %s", p)
	}
	if p := Prompt("X", ""); !strings.Contains(p, "muted arcane") {
		t.Fatalf("unknown school lost the fallback palette: %s", p)
	}
}
