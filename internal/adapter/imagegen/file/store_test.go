package file

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HugoSmits86/nativewebp"

	"github.com/promix1722/easydnd/internal/types"
	"github.com/promix1722/easydnd/internal/usecase/spellicon"
)

// testPNG renders a small image with translucent pixels, so the round trip
// exercises alpha preservation rather than passing on an opaque square.
func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := range 32 {
		for x := range 32 {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 8), G: uint8(y * 8), B: 200, A: uint8(128 + y)})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func newDirs(t *testing.T) (store *Store, out, cache string) {
	t.Helper()
	out = t.TempDir()
	cache = t.TempDir()
	return New(out, cache, nil), out, cache
}

// newPackStore builds a store with one mapped pack namespace beside the SRD
// base, the shape image_generation.pack_dirs produces in development.
func newPackStore(t *testing.T) (store *Store, out, pack, cache string) {
	t.Helper()
	out = t.TempDir()
	pack = t.TempDir()
	cache = t.TempDir()
	return New(out, cache, map[string]string{"dnd-2014": pack}), out, pack, cache
}

func writeTestFile(t *testing.T, dir, name string, data []byte) {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSaveWritesWebpAndCachePNG(t *testing.T) {
	s, out, cache := newDirs(t)
	pngData := testPNG(t)

	rev, err := s.Save(context.Background(), "acid-arrow", pngData)
	if err != nil {
		t.Fatal(err)
	}
	if rev == "" {
		t.Fatal("Save must return a revision")
	}

	webp, err := os.ReadFile(filepath.Join(out, "acid-arrow.webp"))
	if err != nil {
		t.Fatalf("published icon missing: %v", err)
	}
	img, err := nativewebp.Decode(bytes.NewReader(webp))
	if err != nil {
		t.Fatalf("published file is not a webp: %v", err)
	}
	if img.Bounds().Dx() != IconSize || img.Bounds().Dy() != IconSize {
		t.Fatalf("want %dx%d icon, got %v", IconSize, IconSize, img.Bounds())
	}
	// Alpha survived the scale and the lossless encode.
	_, _, _, a := img.At(0, 0).RGBA()
	if a == 0xffff {
		t.Fatal("transparency was lost in the encode")
	}

	cached, err := os.ReadFile(filepath.Join(cache, "acid-arrow.png"))
	if err != nil {
		t.Fatalf("png master not cached: %v", err)
	}
	if !bytes.Equal(cached, pngData) {
		t.Fatal("cached master must be the exact provider bytes")
	}
}

func TestExistingExactThenLegacyFallback(t *testing.T) {
	s, out, pack, _ := newPackStore(t)

	if rev, ok, err := s.Existing(context.Background(), "acid-arrow"); ok || rev != "" || err != nil {
		t.Fatalf("empty pack should miss, got %q %v %v", rev, ok, err)
	}

	// A flat legacy file in the SRD base answers its own slug and any
	// namespaced slug whose last segment matches.
	writeTestFile(t, out, "acid-splash.webp", []byte("legacy"))
	if rev, ok, err := s.Existing(context.Background(), "acid-splash"); !ok || rev == "" || err != nil {
		t.Fatalf("legacy slug should exist, got %q %v %v", rev, ok, err)
	}
	if _, ok, err := s.Existing(context.Background(), "dnd-2014/acid-splash"); !ok || err != nil {
		t.Fatalf("namespaced slug should fall back to legacy art, got %v %v", ok, err)
	}
	if _, ok, err := s.Existing(context.Background(), "dnd-2014/other"); ok || err != nil {
		t.Fatalf("unrelated slug must miss, got %v %v", ok, err)
	}

	// The pack's own file wins over the legacy fallback.
	writeTestFile(t, pack, "acid-splash.webp", []byte("namespaced"))
	rev, ok, err := s.Existing(context.Background(), "dnd-2014/acid-splash")
	if !ok || err != nil {
		t.Fatal(err)
	}
	if rev != revision([]byte("namespaced")) {
		t.Fatal("exact file must win over legacy fallback")
	}
}

func TestReadResolvesCanonicalSlug(t *testing.T) {
	s, out, pack, _ := newPackStore(t)
	writeTestFile(t, out, "acid-splash.webp", []byte("legacy-bytes"))
	writeTestFile(t, pack, "booming-blade.webp", []byte("pack-bytes"))

	// A pack file reads under its qualified key.
	img, err := s.Read(context.Background(), "dnd-2014/booming-blade")
	if err != nil {
		t.Fatal(err)
	}
	if img.Slug != "dnd-2014/booming-blade" || string(img.Data) != "pack-bytes" {
		t.Fatalf("pack read must report its qualified key, got %+v", img)
	}

	// A namespaced slug with no icon of its own falls back to the flat SRD
	// file, and reports the canonical bare key the bytes answer to.
	img, err = s.Read(context.Background(), "dnd-2014/acid-splash")
	if err != nil {
		t.Fatal(err)
	}
	if img.Slug != "acid-splash" {
		t.Fatalf("fallback read must report the canonical key, got %q", img.Slug)
	}
	if img.ContentType != "image/webp" || string(img.Data) != "legacy-bytes" {
		t.Fatalf("unexpected image payload %+v", img)
	}
	if img.Revision != revision([]byte("legacy-bytes")) {
		t.Fatal("revision must be the sha256 of the resolved bytes")
	}
}

func TestReadMissingIsNotFound(t *testing.T) {
	s, _, _ := newDirs(t)
	_, err := s.Read(context.Background(), "nothing-here")
	var nf *types.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("missing read must be NotFoundError, got %v", err)
	}
}

func TestListReturnsPackKeys(t *testing.T) {
	s, out, pack, _ := newPackStore(t)
	writeTestFile(t, out, "a.webp", []byte("1"))
	writeTestFile(t, out, "notes.txt", []byte("not an icon"))
	writeTestFile(t, out, "..bad.webp", []byte("not a slug"))
	// A directory left inside the SRD base from before pack roots existed is
	// debris now: the pack namespace owns that key, and listing the stale copy
	// twice would seed it a second time under a name nothing reads.
	writeTestFile(t, out, filepath.Join("dnd-2014", "stale.webp"), []byte("old layout"))
	writeTestFile(t, pack, "b.webp", []byte("2"))

	keys, err := s.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "dnd-2014/b"}
	if len(keys) != len(want) {
		t.Fatalf("want %v, got %v", want, keys)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("want %v, got %v", want, keys)
		}
	}
}

func TestListMissingDirIsAnError(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "never-created"), "cache", nil)
	// A missing seed pack must fail startup import, not seed silence.
	if _, err := s.List(context.Background()); err == nil {
		t.Fatal("missing pack must error")
	}
	if _, ok, err := s.Existing(context.Background(), "x"); ok || err != nil {
		t.Fatalf("missing pack must still miss cleanly, got %v %v", ok, err)
	}
}

// A pack root that does not exist yet is the empty pack it stands for: List
// skips it, and the first generation into the namespace creates it. Only the
// SRD base is allowed to fail startup.
func TestMissingPackRootListsEmptyAndSavesLazily(t *testing.T) {
	out := t.TempDir()
	pack := filepath.Join(t.TempDir(), "not-yet-there")
	cache := t.TempDir()
	s := New(out, cache, map[string]string{"dnd-2014": pack})

	writeTestFile(t, out, "acid-arrow.webp", []byte("srd"))
	keys, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("missing pack root must list as empty, got %v", err)
	}
	if len(keys) != 1 || keys[0] != "acid-arrow" {
		t.Fatalf("want only the SRD key, got %v", keys)
	}
	if _, ok, err := s.Existing(context.Background(), "dnd-2014/mage-hand"); ok || err != nil {
		t.Fatalf("missing pack root must miss cleanly, got %v %v", ok, err)
	}

	if _, err := s.Save(context.Background(), "dnd-2014/mage-hand", testPNG(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(pack, "mage-hand.webp")); err != nil {
		t.Fatalf("first save must create the pack root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cache, "dnd-2014", "mage-hand.png")); err != nil {
		t.Fatalf("png master must stay qualified in the cache: %v", err)
	}
}
func TestSlugValidationRejectsTraversal(t *testing.T) {
	s, out, _ := newDirs(t)
	writeTestFile(t, out, "real.webp", []byte("x"))
	for _, bad := range []string{"", "..", "../real", "a/../b", "a//b", ".hidden", "a b", "/abs", "a/b/c"} {
		_, _, err := s.Existing(context.Background(), bad)
		var ve *types.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("slug %q: want ValidationError, got %v", bad, err)
		}
	}
	_, err := s.Save(context.Background(), "../escape", testPNG(t))
	var ve *types.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("Save must validate the slug, got %v", err)
	}
}

func TestSaveIsAtomicOnEncodeFailure(t *testing.T) {
	s, out, _ := newDirs(t)
	writeTestFile(t, out, "spell.webp", []byte("old-icon"))

	if _, err := s.Save(context.Background(), "spell", []byte("not a png")); err == nil {
		t.Fatal("garbage input must fail")
	}
	got, err := os.ReadFile(filepath.Join(out, "spell.webp"))
	if err != nil || string(got) != "old-icon" {
		t.Fatalf("failed save must keep the old icon, got %q %v", got, err)
	}
	// No temp debris either.
	entries, _ := os.ReadDir(out)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}

func TestSaveRejectsBeforeWriteOnBadSlug(t *testing.T) {
	s, out, _ := newDirs(t)
	if _, err := s.Save(context.Background(), "bad slug", testPNG(t)); err == nil {
		t.Fatal("invalid slug must fail")
	}
	entries, _ := os.ReadDir(out)
	if len(entries) != 0 {
		t.Fatalf("nothing should be written for a bad slug, got %v", entries)
	}
}

// A namespaced save must land in its own pack's directory flat by local slug
// and touch nothing in the SRD base -- the pack root is another repository,
// and the base tree must not grow pack folders back.
func TestSaveRoutesToTheOwningPack(t *testing.T) {
	s, out, pack, cache := newPackStore(t)

	rev, err := s.Save(context.Background(), "dnd-2014/blade-ward", testPNG(t))
	if err != nil {
		t.Fatal(err)
	}
	if rev == "" {
		t.Fatal("Save must return a revision")
	}
	if _, err := os.Stat(filepath.Join(pack, "blade-ward.webp")); err != nil {
		t.Fatalf("pack icon missing from its own root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cache, "dnd-2014", "blade-ward.png")); err != nil {
		t.Fatalf("png master must be cached under the qualified slug: %v", err)
	}

	base, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(base) != 0 {
		t.Fatalf("a namespaced save must not touch the SRD base, found %v", base)
	}

	// The two namespaces are isolated: the same local name in each is a
	// different file under a different key.
	if _, err := s.Save(context.Background(), "shared-name", testPNG(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(context.Background(), "dnd-2014/shared-name", testPNG(t)); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(out, "shared-name.webp"),
		filepath.Join(pack, "shared-name.webp"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected icon at %s: %v", path, err)
		}
	}
}

// A namespace pack_dirs does not map is a configuration error raised before
// any paid request -- never a silent write into the SRD tree.
func TestUnknownNamespaceIsRefusedWithoutWriting(t *testing.T) {
	s, out, pack, _ := newPackStore(t)
	for _, call := range []func() error{
		func() error { _, _, err := s.Existing(context.Background(), "home-9/fireball"); return err },
		func() error { _, err := s.Read(context.Background(), "home-9/fireball"); return err },
		func() error { _, err := s.Save(context.Background(), "home-9/fireball", testPNG(t)); return err },
	} {
		err := call()
		var ve *types.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("want ValidationError for an unmapped namespace, got %v", err)
		}
		if ve.Reason != spellicon.ReasonPackUnconfigured {
			t.Fatalf("want pack_unconfigured reason, got %q", ve.Reason)
		}
		if ve.Args["pack"] != "home-9" {
			t.Fatalf("reason must carry the pack arg, got %v", ve.Args)
		}
	}

	for _, dir := range []string{out, pack} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("nothing may be written for an unmapped namespace, found %v in %s", entries, dir)
		}
	}
}

// The pack on disk is what survives a restart: a store built over the same
// directories -- the process's second life -- lists and reads the qualified
// keys the first one's writes left behind.
func TestRestartListsPackWritesQualified(t *testing.T) {
	out := t.TempDir()
	pack := t.TempDir()
	cache := t.TempDir()
	s := New(out, cache, map[string]string{"dnd-2014": pack})

	if _, err := s.Save(context.Background(), "dnd-2014/booming-blade", testPNG(t)); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, out, "acid-arrow.webp", []byte("srd"))

	restarted := New(out, cache, map[string]string{"dnd-2014": pack})
	keys, err := restarted.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"acid-arrow", "dnd-2014/booming-blade"}
	if len(keys) != len(want) || keys[0] != want[0] || keys[1] != want[1] {
		t.Fatalf("restart must list qualified keys, want %v got %v", want, keys)
	}
	img, err := restarted.Read(context.Background(), "dnd-2014/booming-blade")
	if err != nil {
		t.Fatal(err)
	}
	if img.Slug != "dnd-2014/booming-blade" {
		t.Fatalf("restart must read the qualified key, got %q", img.Slug)
	}
}
