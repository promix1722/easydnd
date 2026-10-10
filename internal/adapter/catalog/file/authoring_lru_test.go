package file

import (
	"context"
	"fmt"
	"testing"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/pack"
)

// A catalogue for a lock with a database pack in it is kept, but not forever:
// there is one per lock and locale anybody has opened, and an unbounded cache
// of them is a leak that grows with users.
func TestCompiledCataloguesAreBoundedAndLeastRecentlyUsedGoesFirst(t *testing.T) {
	a := &Authoring{
		base:  &Registry{releases: map[string]map[string]*PackDocument{}, identities: map[*PackDocument]pack.Release{}},
		cache: map[string]*catalog.Catalog{},
	}
	compiles := 0
	open := func(i int) {
		t.Helper()
		lock := pack.Lock{Packs: []pack.Release{{ID: fmt.Sprintf("homebrew-%d", i), Version: "1.0.0", Digest: "d"}}}
		if _, err := a.compiled(context.Background(), "en", lock, func() (*catalog.Catalog, error) {
			compiles++
			return &catalog.Catalog{}, nil
		}); err != nil {
			t.Fatal(err)
		}
	}

	for i := range maxCompiledCatalogues {
		open(i)
	}
	open(0) // used again, so it is no longer the oldest
	open(maxCompiledCatalogues)
	if len(a.cache) != maxCompiledCatalogues || len(a.recent) != maxCompiledCatalogues {
		t.Fatalf("kept %d catalogues (%d in order), want %d", len(a.cache), len(a.recent), maxCompiledCatalogues)
	}

	before := compiles
	open(0)
	if compiles != before {
		t.Error("the recently used catalogue was evicted")
	}
	open(1)
	if compiles != before+1 {
		t.Error("the least recently used catalogue was still kept")
	}
}
