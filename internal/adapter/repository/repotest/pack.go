package repotest

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/promix1722/easydnd/internal/domain/pack"
)

// RunPackRepository verifies CAS and lossless storage in both adapters.
// The factory creates the actor and group used by durable foreign keys.
func RunPackRepository(t *testing.T, factory func(*testing.T) pack.Repository) {
	t.Helper()
	r := factory(t)
	ctx := context.Background()
	original := pack.Record{ID: "home-test", Owner: "anon:author", Title: "Sample", Draft: []byte(`{"manifest":{"id":"home-test"}}`), Releases: []pack.Document{{Release: pack.Release{ID: "home-test", Version: "1.0.0", Digest: "digest"}, Data: []byte(`{"content":"immutable"}`)}}}
	if err := r.Save(ctx, original, 0); err != nil {
		t.Fatal(err)
	}
	got, err := r.Get(ctx, original.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Revision != 1 || string(got.Draft) != string(original.Draft) {
		t.Fatal(got)
	}
	got.Draft[0] = 'x'
	again, err := r.Get(ctx, original.ID)
	if err != nil {
		t.Fatal(err)
	}
	if string(again.Draft) != string(original.Draft) {
		t.Fatal("read leaked mutable bytes")
	}
	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			copy := original
			copy.Title = "Updated"
			if r.Save(ctx, copy, 1) == nil {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("CAS committed %d writers", wins.Load())
	}
	share := pack.Share{Group: "test-table", Pack: original.ID, Contributor: original.Owner, Lock: pack.Lock{Edition: "2014", Semantics: "1", Packs: []pack.Release{original.Releases[0].Release}}}
	if err = r.PutShare(ctx, share); err != nil {
		t.Fatal(err)
	}
	shares, err := r.Shares(ctx, share.Group)
	if err != nil || len(shares) != 1 {
		t.Fatal(shares, err)
	}
	if err = r.DeleteShare(ctx, share.Group, share.Pack); err != nil {
		t.Fatal(err)
	}
	shares, err = r.Shares(ctx, share.Group)
	if err != nil || len(shares) != 0 {
		t.Fatal(shares, err)
	}
	got, err = r.Get(ctx, original.ID)
	if err != nil || string(got.Releases[0].Data) != string(original.Releases[0].Data) {
		t.Fatal("unsharing damaged release", err)
	}
}
