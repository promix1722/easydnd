package pack_test

import (
	"context"
	"runtime"
	"testing"

	file "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/domain/user"
	uc "github.com/promix1722/easydnd/internal/usecase/pack"
)

const mb = 1 << 20

// live is the heap that survives a collection: what the process actually holds.
func live() uint64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

// allocated is how many bytes one call of fn allocates, averaged over n.
func allocated(n int, fn func()) uint64 {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for range n {
		fn()
	}
	runtime.ReadMemStats(&after)
	return (after.TotalAlloc - before.TotalAlloc) / uint64(n)
}

// The pack is tens of megabytes and every signed-in request asks which packs
// the caller may use. Answering must not copy it: v1.1.0 did, three times per
// spell-browser request, and a 1 GB host ran out of memory within a minute.
// The figures are logged so a change in either direction shows up in -v.
func TestMemoryPackRequestsDoNotCopyOrLeak(t *testing.T) {
	ctx := context.Background()
	start := live()
	base, err := file.NewRegistry([]string{"../../../data/pack/srd-5.1"}, nil, "", file.PackFolder{Path: overlayDir(t), Restricted: true})
	if err != nil {
		t.Fatal(err)
	}
	repo := memory.NewPackRepository()
	users := memory.NewUserRepository()
	engine := file.NewAuthoring(base, repo)
	s := uc.NewService(repo, engine, memory.NewGroupRepository(users), users)
	s.SetSuperadmins([]string{"root"})
	held := live()
	t.Logf("held after startup: %d MB", (held-start)/mb)

	var overlay pack.Release
	for _, r := range engine.Builtins() {
		if r.ID == "overlay" {
			overlay = r.Releases[0].Release
		}
	}
	lock, err := s.Resolve(ctx, "root", []pack.Release{overlay})
	if err != nil {
		t.Fatal(err)
	}
	load := func() {
		for _, locale := range []rules.Locale{rules.LocaleEN, rules.LocaleRU} {
			if _, err := engine.LoadLocked(ctx, locale, lock); err != nil {
				t.Fatal(err)
			}
		}
	}
	load()
	warm := live()
	t.Logf("held after the overlay's lock is loaded in two locales: %d MB (+%d)", (warm-start)/mb, (int64(warm)-int64(held))/mb)

	list := allocated(20, func() { _, _ = s.List(ctx, user.ID("root")) })
	resolve := allocated(20, func() { _, _ = s.Resolve(ctx, "root", []pack.Release{overlay}) })
	cached := allocated(20, load)
	t.Logf("allocated per call: List %d KB, Resolve %d KB, LoadLocked x2 (cached) %d KB", list/1024, resolve/1024, cached/1024)

	for range 200 {
		_, _ = s.List(ctx, "root")
		_, _ = s.Resolve(ctx, "root", []pack.Release{overlay})
		load()
	}
	after := live()
	t.Logf("held after 200 more rounds: %d MB (%+d)", (after-start)/mb, (int64(after)-int64(warm))/mb)

	if list > mb || resolve > mb {
		t.Errorf("a pack request copies the pack: List %d KB, Resolve %d KB, want under 1 MB each", list/1024, resolve/1024)
	}
	if after > warm+8*mb {
		t.Errorf("repeating the same requests grew the heap by %d MB", (after-warm)/mb)
	}
	runtime.KeepAlive(engine)
}
