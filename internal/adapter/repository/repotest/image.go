package repotest

import (
	"bytes"
	"context"
	"sync"
	"testing"

	"github.com/promix1722/easydnd/internal/domain/imageasset"
	"github.com/promix1722/easydnd/internal/types"
)

// RunImageRepository verifies the revision-gated seed contract both adapters
// must honour: a stored image survives a repeat of the same seed untouched,
// and only a new revision rewrites it.
func RunImageRepository(t *testing.T, factory func(*testing.T) imageasset.Repository) {
	t.Helper()
	r := factory(t)
	ctx := context.Background()

	if _, err := r.Get(ctx, "dnd-2014/acid-splash"); !types.IsNotFound(err) {
		t.Fatalf("missing image: want NotFound, got %v", err)
	}

	seed := imageasset.Image{
		Slug:        "dnd-2014/acid-splash",
		Revision:    "rev-a",
		ContentType: "image/webp",
		Data:        []byte{0x52, 'I', 'F', 'F', 0x00, 'W', 'E', 'B', 'P'},
	}
	want := bytes.Clone(seed.Data)
	if err := r.Upsert(ctx, seed); err != nil {
		t.Fatal(err)
	}
	got, err := r.Get(ctx, seed.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if got.Revision != seed.Revision || got.ContentType != seed.ContentType || !bytes.Equal(got.Data, want) {
		t.Fatalf("round trip: got %+v", got)
	}

	// Callers own neither the bytes they handed in nor the bytes they got
	// out: mutating either must not rewrite what is stored.
	seed.Data[0] = 'x'
	got.Data[0] = 'x'
	again, err := r.Get(ctx, seed.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(again.Data, want) {
		t.Fatal("repository leaked mutable bytes")
	}

	// A new revision replaces the stored image.
	regen := imageasset.Image{
		Slug:        seed.Slug,
		Revision:    "rev-b",
		ContentType: "image/webp",
		Data:        []byte{0x52, 'I', 'F', 'F', 0x01, 'W', 'E', 'B', 'P'},
	}
	if err := r.Upsert(ctx, regen); err != nil {
		t.Fatal(err)
	}
	got, err = r.Get(ctx, seed.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if got.Revision != "rev-b" || !bytes.Equal(got.Data, regen.Data) {
		t.Fatalf("new revision did not replace: got %+v", got)
	}

	// Re-importing an unchanged seed is a no-op, and a write arriving under
	// a revision that is already stored is skipped rather than applied --
	// revision equality is the whole change signal.
	if err := r.Upsert(ctx, regen); err != nil {
		t.Fatal(err)
	}
	conflict := regen
	conflict.Data = []byte("bytes that disagree with their revision")
	if err := r.Upsert(ctx, conflict); err != nil {
		t.Fatal(err)
	}
	got, err = r.Get(ctx, seed.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if got.Revision != "rev-b" || !bytes.Equal(got.Data, regen.Data) {
		t.Fatalf("stored revision was rewritten: got %+v", got)
	}

	// Concurrent repeats of the same import must not corrupt the row; the
	// race detector is what this loop is really for.
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := r.Upsert(ctx, regen); err != nil {
				t.Errorf("concurrent seed import: %v", err)
			}
		}()
	}
	wg.Wait()
	if got, err := r.Get(ctx, seed.Slug); err != nil || !bytes.Equal(got.Data, regen.Data) {
		t.Fatalf("after concurrent upserts: %v, %+v", err, got)
	}
}
