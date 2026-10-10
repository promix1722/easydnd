package repotest

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
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
	// A disk pack is shared without ever having been stored here.
	disk := pack.Share{Group: "test-table", Pack: "disk-pack", Contributor: original.Owner, Lock: pack.Lock{Edition: "2014", Semantics: "1", Packs: []pack.Release{{ID: "disk-pack", Version: "1.0.0", Digest: "d0"}}}}
	if err = r.PutShare(ctx, disk); err != nil {
		t.Fatal("share a pack with no stored record: ", err)
	}
	if err = r.DeleteShare(ctx, disk.Group, disk.Pack); err != nil {
		t.Fatal(err)
	}

	// Grants are a set per account, replaced whole and read back sorted.
	if err = r.SetGrants(ctx, original.Owner, []string{"disk-b", "disk-a", "disk-b"}); err != nil {
		t.Fatal(err)
	}
	if granted, err := r.Grants(ctx, original.Owner); err != nil || !slices.Equal(granted, []string{"disk-a", "disk-b"}) {
		t.Fatalf("Grants = %v, %v", granted, err)
	}
	if err = r.SetGrants(ctx, original.Owner, nil); err != nil {
		t.Fatal(err)
	}
	if granted, err := r.Grants(ctx, original.Owner); err != nil || len(granted) != 0 {
		t.Fatalf("Grants after clearing = %v, %v", granted, err)
	}
	if granted, err := r.Grants(ctx, "nobody"); err != nil || len(granted) != 0 {
		t.Fatalf("Grants of a stranger = %v, %v", granted, err)
	}

	private := pack.Document{Release: pack.Release{ID: "import-s1", Version: "0.0.0-abc", Digest: "d1"}, Data: []byte(`{"private":true}`)}
	if _, err := r.GetPrivate(ctx, private.Release); !types.IsNotFound(err) {
		t.Fatalf("GetPrivate before PutPrivate = %v, want not found", err)
	}
	if err := r.PutPrivate(ctx, private); err != nil {
		t.Fatal(err)
	}
	if err := r.PutPrivate(ctx, private); err != nil {
		t.Fatalf("PutPrivate twice: %v", err)
	}
	kept, err := r.GetPrivate(ctx, private.Release)
	if err != nil || string(kept.Data) != string(private.Data) || kept.Release != private.Release {
		t.Fatalf("GetPrivate = %+v, %v", kept, err)
	}
	// The lock names bytes: the same version under another digest is not it.
	other := private.Release
	other.Digest = "d2"
	if _, err := r.GetPrivate(ctx, other); !types.IsNotFound(err) {
		t.Fatalf("GetPrivate with another digest = %v, want not found", err)
	}
	// And a private release is not a listed pack.
	records, err := r.ListFor(ctx, "", []string{private.Release.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Fatal("a private release was listed")
	}
	// A listing is somebody's packs or named ones, never the table.
	for _, c := range []struct {
		owner string
		ids   []string
		want  int
	}{{string(original.Owner), nil, 1}, {"anon:stranger", nil, 0}, {"", []string{original.ID}, 1}, {"anon:stranger", []string{original.ID}, 1}, {"", nil, 0}} {
		got, err := r.ListFor(ctx, user.ID(c.owner), c.ids)
		if err != nil || len(got) != c.want {
			t.Fatalf("ListFor(%q, %v) = %d records, %v; want %d", c.owner, c.ids, len(got), err, c.want)
		}
	}
}
