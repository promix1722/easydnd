package repotest

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/promix1722/easydnd/internal/types"
	agentuc "github.com/promix1722/easydnd/internal/usecase/agent"
)

// RunAgentStore verifies what the wizard relies on a store for: that one
// instance at a time holds a turn, that a turn which lost it cannot write,
// and that unused sessions go.
func RunAgentStore(t *testing.T, factory func(*testing.T) agentuc.Store) {
	t.Helper()
	r := factory(t)
	ctx := context.Background()
	event := func(id int, text string) agentuc.AgentEvent {
		return agentuc.AgentEvent{ID: id, Kind: "user", Text: text}
	}
	rec := agentuc.Record{ID: "s1", Owner: "owner", Status: "queued", Revision: 1, Created: time.Now(), Document: []byte(`{"Turns":1}`),
		Events: []agentuc.AgentEvent{event(1, "first")}, Files: []agentuc.AgentFile{{Name: "sheet.txt", MIME: "text/plain", Data: []byte("A hero")}}}
	if ok, err := r.Create(ctx, rec, 1); err != nil || !ok {
		t.Fatal(ok, err)
	}
	other := rec
	other.ID = "s2"
	if ok, err := r.Create(ctx, other, 1); err != nil || ok {
		t.Fatalf("created past the limit: %v %v", ok, err)
	}
	got, err := r.Get(ctx, "s1")
	if err != nil || got.Owner != "owner" || got.Count != 1 || len(got.Events) != 1 || got.Events[0].Text != "first" || len(got.Document) == 0 {
		t.Fatal(got, err)
	}
	if files, err := r.Files(ctx, "s1"); err != nil || len(files) != 1 || string(files[0].Data) != "A hero" {
		t.Fatal(files, err)
	}
	if listed, err := r.List(ctx, "owner"); err != nil || len(listed) != 1 || len(listed[0].Document) == 0 {
		t.Fatal(listed, err)
	}
	if listed, err := r.List(ctx, "stranger"); err != nil || len(listed) != 0 {
		t.Fatal(listed, err)
	}
	if _, err = r.Get(ctx, "missing"); !types.IsNotFound(err) {
		t.Fatal(err)
	}

	// Eight workers ask for the one queued turn; one gets it.
	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok, err := r.Claim(ctx, "a", time.Minute, nil); err == nil && ok {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("%d workers claimed one turn", wins.Load())
	}
	held, _ := r.Get(ctx, "s1")
	if held.Status != "running" || held.Revision != 2 {
		t.Fatal(held)
	}

	// The holder appends and saves; nobody else can.
	if ok, err := r.Append(ctx, "s1", "a", 0, event(2, "delta")); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if ok, err := r.Append(ctx, "s1", "b", 0, event(3, "stranger")); err != nil || ok {
		t.Fatal("another instance appended", err)
	}
	held.Count, held.Events = 2, []agentuc.AgentEvent{event(3, "saved")}
	if ok, err := r.Save(ctx, held, "b"); err != nil || ok {
		t.Fatal("another instance saved", err)
	}
	if ok, err := r.Save(ctx, held, "a"); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if tail, err := r.Tail(ctx, "s1", 1); err != nil || tail.Count != 3 || len(tail.Events) != 2 || tail.Events[1].Text != "saved" || tail.Document != nil {
		t.Fatal(tail, err)
	}

	// The owner's message moves the generation and queues the session again:
	// the turn in flight is fenced, and its lease is gone with its status.
	if err = r.Update(ctx, "s1", func(rec *agentuc.Record) error {
		rec.Generation++
		rec.Revision++
		rec.Status = "queued"
		rec.Events = append(rec.Events, event(4, "message"))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if ok, err := r.Save(ctx, held, "a"); err != nil || ok {
		t.Fatal("a fenced turn saved", err)
	}
	if ok, err := r.Append(ctx, "s1", "a", 0, event(5, "late")); err != nil || ok {
		t.Fatal("a fenced turn appended", err)
	}
	if _, ok, _ := r.Claim(ctx, "b", time.Minute, []string{"s1"}); ok {
		t.Fatal("claimed a session it was told is busy")
	}

	// A lease that ran out is anybody's: this is how a dead process's turn
	// is taken over. A live one is nobody else's.
	if _, ok, err := r.Claim(ctx, "a", time.Millisecond, nil); err != nil || !ok {
		t.Fatal(ok, err)
	}
	time.Sleep(20 * time.Millisecond)
	taken, ok, err := r.Claim(ctx, "b", time.Minute, nil)
	if err != nil || !ok || taken.Generation != 1 || len(taken.Events) != 4 {
		t.Fatal(taken, ok, err)
	}
	if _, ok, _ := r.Claim(ctx, "c", time.Minute, nil); ok {
		t.Fatal("claimed a turn under a live lease")
	}
	if ok, err := r.Save(ctx, taken, "a"); err != nil || ok {
		t.Fatal("the instance that lost its lease saved", err)
	}

	// The sweep spares a running turn and a session in use, and takes the rest
	// with their events and files.
	if n, err := r.Sweep(ctx, -time.Hour); err != nil || n != 0 {
		t.Fatal("swept a running turn", n, err)
	}
	taken.Status = "waiting"
	if ok, err := r.Save(ctx, taken, "b"); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if n, err := r.Sweep(ctx, time.Hour); err != nil || n != 0 {
		t.Fatal("swept a session in use", n, err)
	}
	if n, err := r.Sweep(ctx, -time.Hour); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if _, err = r.Tail(ctx, "s1", 0); !types.IsNotFound(err) {
		t.Fatal(err)
	}
	if ok, err := r.Create(ctx, other, 1); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if err = r.Delete(ctx, "s2"); err != nil {
		t.Fatal(err)
	}
	if _, err = r.Get(ctx, "s2"); !types.IsNotFound(err) {
		t.Fatal(err)
	}
}
