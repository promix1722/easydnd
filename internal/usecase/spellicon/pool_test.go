package spellicon

// The pool tests assert on observed behaviour -- how many Generate calls
// overlap, what the counters say, whether Close returns -- never on the
// worker-count constant itself, so changing the default cannot silently
// retire them.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// trackingGen records how many Generate calls overlap and optionally parks
// each one on a channel until the test releases it, so the pool's bound is
// measured rather than assumed. errFor keys match the spell's quoted name
// inside the prompt.
type trackingGen struct {
	mu       sync.Mutex
	inFlight int
	max      int
	calls    int
	release  chan struct{} // when non-nil, every call waits here until closed
	started  chan struct{} // one signal per entered call; buffered, never blocks
	errFor   map[string]error
}

func newTrackingGen() *trackingGen {
	return &trackingGen{started: make(chan struct{}, 64)}
}

func (g *trackingGen) Generate(ctx context.Context, prompt string) ([]byte, error) {
	g.mu.Lock()
	g.inFlight++
	g.calls++
	if g.inFlight > g.max {
		g.max = g.inFlight
	}
	g.mu.Unlock()
	defer func() {
		g.mu.Lock()
		g.inFlight--
		g.mu.Unlock()
	}()

	select {
	case g.started <- struct{}{}:
	default:
	}
	if g.release != nil {
		select {
		case <-g.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	for key, err := range g.errFor {
		if strings.Contains(prompt, `"`+key+`"`) {
			return nil, err
		}
	}
	return []byte("png:" + prompt), nil
}

// waitForStarts blocks until n Generate calls have entered, so the test can
// observe a saturated pool rather than racing it.
func waitForStarts(t *testing.T, g *trackingGen, n int) {
	t.Helper()
	timer := time.After(5 * time.Second)
	for i := range n {
		select {
		case <-g.started:
		case <-timer:
			t.Fatalf("only %d of %d Generate calls entered", i, n)
		}
	}
}

func newPoolService(gen Generator, store Store, workers int) *Service {
	return NewService(gen, store, workers, time.Minute,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func iconSpells(n int) []Spell {
	spells := make([]Spell, 0, n)
	for i := range n {
		spells = append(spells, Spell{
			Slug:   fmt.Sprintf("spell-%02d", i),
			Name:   fmt.Sprintf("Spell %02d", i),
			School: "evocation",
		})
	}
	return spells
}

// Ten calls run at once and not one more: the eleventh spell stays queued
// until a worker frees its slot, then joins without waiting for the rest.
func TestPoolRunsTenAtOnceAndQueuesTheRest(t *testing.T) {
	gen := newTrackingGen()
	gen.release = make(chan struct{})
	s := newPoolService(gen, newFakeStore(), 10)
	defer s.Close()

	if _, err := s.Start(context.Background(), "o", iconSpells(25), false); err != nil {
		t.Fatal(err)
	}
	waitForStarts(t, gen, 10)

	gen.mu.Lock()
	calls, max := gen.calls, gen.max
	gen.mu.Unlock()
	if calls != 10 || max != 10 {
		t.Fatalf("want exactly 10 calls in flight, got calls=%d max=%d", calls, max)
	}

	// The queue reports ten generating and fifteen still queued.
	st := s.Status("o")
	generating, queued := 0, 0
	for _, item := range st.Items {
		switch item.State {
		case ItemGenerating:
			generating++
		case ItemQueued:
			queued++
		}
	}
	if generating != 10 || queued != 15 {
		t.Fatalf("want 10 generating and 15 queued, got %+v", st)
	}

	// Free one slot while the other nine remain blocked. Another item must
	// start without waiting for the entire first wave.
	select {
	case gen.release <- struct{}{}:
	case <-time.After(5 * time.Second):
		t.Fatal("could not release an active generation")
	}
	waitForStarts(t, gen, 1)
	st = s.Status("o")
	if st.Completed != 1 {
		t.Fatalf("one slot should have completed while the others are blocked: %+v", st)
	}
	gen.mu.Lock()
	calls, max = gen.calls, gen.max
	gen.mu.Unlock()
	if calls != 11 || max != 10 {
		t.Fatalf("freed slot should start exactly one new image: calls=%d max=%d", calls, max)
	}

	close(gen.release)
	final := waitDone(t, s, "o")
	if final.Completed != 25 || final.Failed != 0 {
		t.Fatalf("all 25 items should finish, got %+v", final)
	}
	gen.mu.Lock()
	defer gen.mu.Unlock()
	if gen.calls != 25 {
		t.Fatalf("every queued item should have generated, got %d calls", gen.calls)
	}
	if gen.max > 10 {
		t.Fatalf("pool exceeded its bound: %d calls overlapped", gen.max)
	}
}

// A failed item does not take its neighbours with it, and the pool reaches
// its bound without exceeding it.
func TestPoolFailuresAreIndependent(t *testing.T) {
	gen := newTrackingGen()
	gen.release = make(chan struct{})
	gen.errFor = map[string]error{"Spell 03": errors.New("provider exploded with detail")}
	s := newPoolService(gen, newFakeStore(), 4)
	defer s.Close()

	if _, err := s.Start(context.Background(), "o", iconSpells(8), false); err != nil {
		t.Fatal(err)
	}
	waitForStarts(t, gen, 4) // all four workers busy, four items still queued
	close(gen.release)

	final := waitDone(t, s, "o")
	if final.Completed != 8 || final.Failed != 1 || final.Skipped != 0 {
		t.Fatalf("want completed=8 failed=1, got %+v", final)
	}
	if item := final.Items["spell-03"]; item.State != ItemFailed || item.Reason != ReasonFailed {
		t.Fatalf("the failing item should carry the generic reason, got %+v", item)
	}
	done := 0
	for slug, item := range final.Items {
		if slug != "spell-03" && item.State == ItemDone {
			done++
		}
	}
	if done != 7 {
		t.Fatalf("the other seven items should be done, got %+v", final.Items)
	}
	gen.mu.Lock()
	defer gen.mu.Unlock()
	if gen.calls != 8 || gen.max != 4 {
		t.Fatalf("want 8 calls peaking at the 4-worker bound, got calls=%d max=%d", gen.calls, gen.max)
	}
}

// Close cancels the in-flight calls, waits for every worker to leave, and
// accounts for the items that never started. If any worker were leaked or
// the cancel did not propagate, Close would hang rather than return.
func TestPoolCloseCancelsAndCountsUnstarted(t *testing.T) {
	gen := newTrackingGen()
	gen.release = make(chan struct{})
	s := newPoolService(gen, newFakeStore(), 3)

	if _, err := s.Start(context.Background(), "o", iconSpells(9), false); err != nil {
		t.Fatal(err)
	}
	waitForStarts(t, gen, 3)

	s.Close() // must return; a stuck or leaked worker deadlocks here

	final := s.Status("o")
	if final.Running || final.Completed != final.Total || final.Total != 9 {
		t.Fatalf("Close must leave every item counted, got %+v", final)
	}
	if final.Failed != 9 {
		t.Fatalf("cancelled in-flight and unstarted items all count failed, got %+v", final)
	}
	gen.mu.Lock()
	defer gen.mu.Unlock()
	if gen.calls != 3 {
		t.Fatalf("no item may start a paid call after cancellation, got %d calls", gen.calls)
	}
	if gen.inFlight != 0 {
		t.Fatalf("every worker must have left Generate, %d still inside", gen.inFlight)
	}

	if _, err := s.Start(context.Background(), "o", iconSpells(1), false); err == nil {
		t.Fatal("Start after Close should fail")
	}
}

// Skipped, done and failed each count exactly once toward completed, and the
// store sees one Save per generated icon even with the pool at full width.
func TestPoolCountsEveryItemOnce(t *testing.T) {
	store := newFakeStore()
	for _, slug := range []string{"spell-00", "spell-04", "spell-08"} {
		store.existing[slug] = "old-rev"
	}
	gen := newTrackingGen()
	gen.errFor = map[string]error{"Spell 05": errors.New("boom")}
	s := newPoolService(gen, store, 10)
	defer s.Close()

	st, err := s.Start(context.Background(), "o", iconSpells(15), false)
	if err != nil {
		t.Fatal(err)
	}
	if st.Total != 15 || st.Skipped != 3 || st.Completed != 3 {
		t.Fatalf("three existing icons should be counted skipped up front, got %+v", st)
	}

	final := waitDone(t, s, "o")
	if final.Completed != 15 || final.Failed != 1 || final.Skipped != 3 {
		t.Fatalf("want completed=15 failed=1 skipped=3, got %+v", final)
	}
	gen.mu.Lock()
	calls := gen.calls
	gen.mu.Unlock()
	if calls != 12 {
		t.Fatalf("only the twelve non-existing icons are generated, got %d calls", calls)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.saved) != 11 {
		t.Fatalf("every generated icon but the failed one is saved once, got %v", store.saved)
	}
}

// A pool of zero would leave the queue with nobody to drain it; the
// constructor clamps workers to at least one rather than hanging every job.
func TestPoolClampsWorkers(t *testing.T) {
	s := newPoolService(&fakeGen{}, newFakeStore(), 0)
	defer s.Close()

	if _, err := s.Start(context.Background(), "o", iconSpells(2), false); err != nil {
		t.Fatal(err)
	}
	final := waitDone(t, s, "o")
	if final.Completed != 2 {
		t.Fatalf("a clamped pool still drains, got %+v", final)
	}
}
