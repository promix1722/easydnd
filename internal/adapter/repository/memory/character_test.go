package memory_test

import (
	"context"
	"sync"
	"testing"

	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/types"
)

// The assignment is the conformance proof: if the repository ever drifts from
// the port, this fails to compile rather than failing at wiring time.
var _ domain.Repository = (*memory.CharacterRepository)(nil)

const owner domain.OwnerID = "usr_1"

func TestCreateAssignsDistinctIDs(t *testing.T) {
	repo := memory.NewCharacterRepository()
	ctx := context.Background()

	first, err := repo.Create(ctx, owner)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	second, err := repo.Create(ctx, owner)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if first.ID == second.ID {
		t.Errorf("Create() issued duplicate ID %q", first.ID)
	}
	if first.Owner != owner {
		t.Errorf("Create() owner = %q, want %q", first.Owner, owner)
	}
	if first.Log.Len() != 0 {
		t.Errorf("Create() log length = %d, want 0", first.Log.Len())
	}
}

func TestGetReportsNotFound(t *testing.T) {
	repo := memory.NewCharacterRepository()

	_, err := repo.Get(context.Background(), "chr_missing")
	if !types.IsNotFound(err) {
		t.Errorf("Get() error = %v, want a NotFoundError", err)
	}
}

func TestListFiltersByOwner(t *testing.T) {
	repo := memory.NewCharacterRepository()
	ctx := context.Background()

	if _, err := repo.Create(ctx, owner); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := repo.Create(ctx, "usr_2"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repo.List(ctx, owner)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("List() length = %d, want 1", len(got))
	}
	if got[0].Owner != owner {
		t.Errorf("List() owner = %q, want %q", got[0].Owner, owner)
	}
}

func TestAppendNumbersEventsFromOne(t *testing.T) {
	repo := memory.NewCharacterRepository()
	ctx := context.Background()

	c, err := repo.Create(ctx, owner)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	err = repo.Append(ctx, c.ID, 0,
		domain.Event{Type: domain.EventInit},
		domain.Event{Type: domain.EventRace},
	)
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	stored, err := repo.Get(ctx, c.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stored.Log.Len() != 2 {
		t.Fatalf("log length = %d, want 2", stored.Log.Len())
	}
	for i, e := range stored.Log.Events {
		if e.Seq != i+1 {
			t.Errorf("event %d sequence = %d, want %d", i, e.Seq, i+1)
		}
	}
	if err := stored.Log.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

// A stale expectedSeq is the exact case that makes a whole-log-in-one-record
// store lose writes, so it gets its own test rather than riding along.
func TestAppendRejectsStaleSequence(t *testing.T) {
	repo := memory.NewCharacterRepository()
	ctx := context.Background()

	c, err := repo.Create(ctx, owner)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := repo.Append(ctx, c.ID, 0, domain.Event{Type: domain.EventInit}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	// A second writer still believes the log is empty.
	err = repo.Append(ctx, c.ID, 0, domain.Event{Type: domain.EventRace})
	if err == nil {
		t.Fatal("Append() with a stale sequence succeeded, want an error")
	}

	stored, err := repo.Get(ctx, c.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stored.Log.Len() != 1 {
		t.Errorf("log length = %d, want 1: the rejected batch must not be written", stored.Log.Len())
	}
}

func TestGetReturnsACopy(t *testing.T) {
	repo := memory.NewCharacterRepository()
	ctx := context.Background()

	c, err := repo.Create(ctx, owner)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := repo.Append(ctx, c.ID, 0, domain.Event{Type: domain.EventInit, Note: "original"}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	got, err := repo.Get(ctx, c.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	got.Log.Events[0].Note = "mutated"

	again, err := repo.Get(ctx, c.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if again.Log.Events[0].Note != "original" {
		t.Errorf("note = %q, want %q: Get must not share its backing array", again.Log.Events[0].Note, "original")
	}
}

func TestDeleteReportsNotFound(t *testing.T) {
	repo := memory.NewCharacterRepository()

	if err := repo.Delete(context.Background(), "chr_missing"); !types.IsNotFound(err) {
		t.Errorf("Delete() error = %v, want a NotFoundError", err)
	}
}

// Exercised by `go test -race`, which is how the Makefile runs the suite.
func TestCharacterRepositoryConcurrent(t *testing.T) {
	repo := memory.NewCharacterRepository()
	ctx := context.Background()

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := repo.Create(ctx, owner)
			if err != nil {
				return
			}
			_ = repo.Append(ctx, c.ID, 0, domain.Event{Type: domain.EventInit})
			_, _ = repo.Get(ctx, c.ID)
			_, _ = repo.List(ctx, owner)
		}()
	}
	wg.Wait()

	got, err := repo.List(ctx, owner)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 8 {
		t.Errorf("List() length = %d, want 8", len(got))
	}
}

// Truncate is the undo primitive. The invariant it enforces is not
// "append-only" -- that would make going back a step impossible -- but
// "append, or drop a suffix; never edit the middle".
func TestTruncateDropsASuffix(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewCharacterRepository()

	c, err := repo.Create(ctx, "owner")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := repo.Append(ctx, c.ID, 0,
		domain.Event{Type: domain.EventInit},
		domain.Event{Type: domain.EventRace},
		domain.Event{Type: domain.EventClass},
	); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	if err := repo.Truncate(ctx, c.ID, 3, 1); err != nil {
		t.Fatalf("Truncate() error = %v", err)
	}
	got, err := repo.Get(ctx, c.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Log.Len() != 1 || got.Log.Events[0].Type != domain.EventInit {
		t.Errorf("log = %+v, want just the init event", got.Log.Events)
	}
}

func TestTruncateRejectsStaleAndImpossibleRequests(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewCharacterRepository()

	c, err := repo.Create(ctx, "owner")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := repo.Append(ctx, c.ID, 0,
		domain.Event{Type: domain.EventInit},
		domain.Event{Type: domain.EventRace},
	); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	// A stale expected sequence means another client has written since this
	// one read. The write being discarded here is a deletion, so the check
	// matters more than it does for an append, not less.
	if err := repo.Truncate(ctx, c.ID, 1, 1); err == nil {
		t.Error("Truncate() accepted a stale sequence")
	}
	// The init event is not a step you can go back past.
	if err := repo.Truncate(ctx, c.ID, 2, 0); err == nil {
		t.Error("Truncate() dropped the init event")
	}
	// Truncating to the future is a client bug, not a no-op.
	if err := repo.Truncate(ctx, c.ID, 2, 5); err == nil {
		t.Error("Truncate() accepted a sequence past the end of the log")
	}
	// A rejected request must leave the log untouched.
	got, err := repo.Get(ctx, c.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Log.Len() != 2 {
		t.Errorf("log length = %d, want 2 after three rejected truncations", got.Log.Len())
	}

	if err := repo.Truncate(ctx, "no-such-character", 1, 1); !types.IsNotFound(err) {
		t.Errorf("Truncate() error = %v, want a NotFoundError", err)
	}
}

// Rewrite is the write behind a replacement: neither an append nor a
// truncation, because replacing one entry can drop entries after it and the
// stored slice comes back a different length.
func TestRewriteReplacesTheWholeLog(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewCharacterRepository()

	c, err := repo.Create(ctx, "owner")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := repo.Append(ctx, c.ID, 0,
		domain.Event{Type: domain.EventInit},
		domain.Event{Type: domain.EventRace},
		domain.Event{Type: domain.EventClass},
	); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	shorter, err := domain.Rebuild([]domain.Event{
		{Type: domain.EventInit},
		{Type: domain.EventBackground},
	})
	if err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}
	if err := repo.Rewrite(ctx, c.ID, 3, shorter); err != nil {
		t.Fatalf("Rewrite() error = %v", err)
	}
	got, err := repo.Get(ctx, c.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Log.Len() != 2 || got.Log.Events[1].Type != domain.EventBackground {
		t.Errorf("log = %+v, want the rewritten two entries", got.Log.Events)
	}

	// The caller keeps no handle on the store's slice.
	shorter.Events[1].Note = "mutated after the write"
	again, err := repo.Get(ctx, c.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if again.Log.Events[1].Note != "" {
		t.Error("Rewrite() stored the caller's own backing array")
	}
}

func TestRewriteRejectsStaleAndMalformedWrites(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewCharacterRepository()

	c, err := repo.Create(ctx, "owner")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := repo.Append(ctx, c.ID, 0,
		domain.Event{Type: domain.EventInit},
		domain.Event{Type: domain.EventRace},
	); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	good := domain.Log{Events: []domain.Event{{Seq: 1, Type: domain.EventInit}}}

	// The write being discarded by a stale sequence here is the whole
	// history, which is why this check matters most on this method.
	if err := repo.Rewrite(ctx, c.ID, 1, good); err == nil {
		t.Error("Rewrite() accepted a stale sequence")
	}
	// A store that accepts a malformed log is a store that hands one back.
	bad := domain.Log{Events: []domain.Event{{Seq: 4, Type: domain.EventInit}}}
	if err := repo.Rewrite(ctx, c.ID, 2, bad); err == nil {
		t.Error("Rewrite() accepted a log whose sequence numbers do not run 1..n")
	}
	if err := repo.Rewrite(ctx, "no-such-character", 0, good); !types.IsNotFound(err) {
		t.Errorf("Rewrite() error = %v, want a NotFoundError", err)
	}

	got, err := repo.Get(ctx, c.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Log.Len() != 2 {
		t.Errorf("log length = %d, want 2 after two rejected rewrites", got.Log.Len())
	}
}
