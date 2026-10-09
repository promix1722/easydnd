package file

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"github.com/HugoSmits86/nativewebp"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"image"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/promix1722/easydnd/internal/domain/rules"
)

func iconPack(t *testing.T) *PackDocument {
	t.Helper()
	p, err := LoadPack("testdata/tactician.json")
	if err != nil {
		t.Fatal(err)
	}
	icon, err := os.ReadFile("../../../../data/pack/srd-5.1/spell-icons/magic-missile.webp")
	if err != nil {
		t.Fatal(err)
	}
	p.Icons = &PackIcons{Spells: map[string][]byte{"guiding-mark": icon}}
	return p
}

func TestRepositoryArtworkJoinsPackAndSurvivesIdentityOverride(t *testing.T) {
	p := iconPack(t)
	art := p.Icons.Spells["guiding-mark"]
	p.Icons = nil
	root := t.TempDir()
	if err := SavePackDirectory(filepath.Join(root, "pack"), p); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "spell-icons")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "guiding-mark.webp"), art, 0600); err != nil {
		t.Fatal(err)
	}
	// Artwork for spells absent from this pack is not adopted.
	if err := os.WriteFile(filepath.Join(dir, "unknown.webp"), []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadPackFolder(PackFolder{Path: root, ID: "renamed"})
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Manifest.ID != "renamed" || len(loaded.Icons.Spells) != 1 || !bytes.Equal(loaded.Icons.Spells["guiding-mark"], art) {
		t.Fatal("repository artwork lost")
	}
	encoded, err := EncodePack(loaded)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodePack(encoded)
	if err != nil || !bytes.Equal(decoded.Icons.Spells["guiding-mark"], art) {
		t.Fatalf("export lost artwork: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "guiding-mark.webp"), []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPackFolder(PackFolder{Path: root}); err == nil {
		t.Fatal("accepted invalid artwork")
	}
	// An explicit pack icon takes precedence over the repository fallback.
	p.Icons = &PackIcons{Spells: map[string][]byte{"guiding-mark": art}}
	other := t.TempDir()
	if err := SavePackDirectory(filepath.Join(other, "pack"), p); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(dir, filepath.Join(other, "spell-icons")); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPackFolder(PackFolder{Path: other}); err != nil {
		t.Fatalf("fallback replaced explicit artwork: %v", err)
	}
}

func TestIconPackRoundTrips(t *testing.T) {
	p := itemIconPack(t)
	before, err := PackDigest(p)
	if err != nil {
		t.Fatal(err)
	}
	data, err := EncodePack(p)
	if err != nil {
		t.Fatal(err)
	}
	portable, err := DecodePack(data)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(p.Icons.Spells["guiding-mark"], portable.Icons.Spells["guiding-mark"]) {
		t.Fatal("JSON lost artwork")
	}
	dir := filepath.Join(t.TempDir(), "pack")
	if err := SavePackDirectory(dir, portable); err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	err = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		name, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		w, err := zw.Create("pack/" + filepath.ToSlash(name))
		if err != nil {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	data, err = (&Authoring{}).ImportZIP(archive.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	zipped, err := DecodePack(data)
	if err != nil {
		t.Fatal(err)
	}
	directory, err := LoadPack(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []*PackDocument{directory, zipped} {
		digest, err := PackDigest(q)
		if err != nil || digest != before {
			t.Fatalf("changed release: %s %v", digest, err)
		}
	}
	forked, err := rewritePackIdentity(data, "another-pack", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	q, err := DecodePack(forked)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(p.Icons.Spells["guiding-mark"], q.Icons.Spells["guiding-mark"]) {
		t.Fatal("fork changed bytes")
	}
	old, err := PackDigest(p)
	if err != nil {
		t.Fatal(err)
	}
	p.Icons.Spells = nil
	missing, err := PackDigest(p)
	if err != nil || old == missing {
		t.Fatal("digest ignores artwork")
	}
}

func TestInvalidPackArtwork(t *testing.T) {
	for _, mode := range []string{"unknown spell", "qualified ID", "truncated WebP", "wrong size", "bad base64"} {
		t.Run(mode, func(t *testing.T) {
			p := iconPack(t)
			switch mode {
			case "unknown spell":
				p.Icons.Spells["absent"] = p.Icons.Spells["guiding-mark"]
			case "qualified ID":
				p.Icons.Spells["example/guiding-mark"] = p.Icons.Spells["guiding-mark"]
			case "truncated WebP":
				p.Icons.Spells["guiding-mark"] = p.Icons.Spells["guiding-mark"][:32]
			case "wrong size":
				var small bytes.Buffer
				if err := nativewebp.Encode(&small, image.NewNRGBA(image.Rect(0, 0, 64, 64)), nil); err != nil {
					t.Fatal(err)
				}
				p.Icons.Spells["guiding-mark"] = small.Bytes()
			case "bad base64":
				data, _ := EncodePack(p)
				var v map[string]any
				_ = json.Unmarshal(data, &v)
				v["icons"].(map[string]any)["spells"].(map[string]any)["guiding-mark"] = "!"
				data, _ = json.Marshal(v)
				if _, err := DecodePack(data); err == nil {
					t.Fatal("accepted bad base64")
				}
				return
			}
			if err := p.Validate(); err == nil {
				t.Fatal("accepted invalid artwork")
			}
		})
	}
}

func TestIconNamespacesAndArchivedRelease(t *testing.T) {
	base, err := LoadPack("../../../../data/pack/srd-5.1")
	if err != nil {
		t.Fatal(err)
	}
	a := iconPack(t)
	b := iconPack(t)
	b.Manifest.ID = "other"
	b.Icons.Spells["guiding-mark"], err = os.ReadFile("../../../../data/pack/srd-5.1/spell-icons/fireball.webp")
	if err != nil {
		t.Fatal(err)
	}
	c, err := compilePacks([]*PackDocument{base, a, b}, rules.LocaleEN, iconLock(t, base, a, b))
	if err != nil {
		t.Fatal(err)
	}
	first, _ := c.Spells.Get("example/guiding-mark")
	second, _ := c.Spells.Get("other/guiding-mark")
	if first.Icon == second.Icon || !strings.HasPrefix(first.Icon, "data:image/webp;base64,") {
		t.Fatal("mixed namespaces")
	}
	old := *base
	oldIcons := *base.Icons
	oldIcons.Spells = nil
	old.Icons = &oldIcons
	old.Manifest.Version = "1.1.0"
	oldDir := filepath.Join(t.TempDir(), "old")
	if err = SavePackDirectory(oldDir, &old); err != nil {
		t.Fatal(err)
	}
	archive := t.TempDir()
	r, err := NewRegistry([]string{oldDir}, nil, archive)
	if err != nil {
		t.Fatal(err)
	}
	lock := r.DefaultLock()
	r, err = NewRegistry([]string{"../../../../data/pack/srd-5.1"}, nil, archive)
	if err != nil {
		t.Fatal(err)
	}
	archived, err := r.LoadLocked(context.Background(), rules.LocaleEN, lock)
	if err != nil {
		t.Fatal(err)
	}
	spell, _ := archived.Spells.Get("magic-missile")
	if spell.Icon != "" {
		t.Fatal("old release changed")
	}
	fresh, err := r.Load(context.Background(), rules.LocaleEN)
	if err != nil {
		t.Fatal(err)
	}
	spell, _ = fresh.Spells.Get("magic-missile")
	if spell.Icon == "" {
		t.Fatal("new release lost icon")
	}
}

func iconLock(t *testing.T, docs ...*PackDocument) pack.Lock {
	t.Helper()
	lock := pack.Lock{Edition: "2014", Semantics: "1"}
	for _, doc := range docs {
		r, err := doc.Release()
		if err != nil {
			t.Fatal(err)
		}
		lock.Packs = append(lock.Packs, r)
	}
	slices.SortFunc(lock.Packs, func(a, b pack.Release) int { return strings.Compare(a.ID, b.ID) })
	return lock
}
