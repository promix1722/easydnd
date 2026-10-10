package file

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/promix1722/easydnd/internal/domain/rules"
)

func provenancePack(t *testing.T) *PackDocument {
	t.Helper()
	p, err := LoadPack("../../../../data/pack/srd-5.1")
	if err != nil {
		t.Fatal(err)
	}
	p.Manifest.Sources = map[string]string{"srd-5.1": "SRD 5.1", "phb": "Player's Handbook"}
	p.Provenance = map[string]map[string][]string{"spells": {"magic-missile": {"srd-5.1", "phb"}}}
	p.Locales["ru"]["sources"] = Bundle{"phb": Prose{Name: "Книга игрока"}}
	return p
}

func TestBuiltinVersionsKeepTheirProvenance(t *testing.T) {
	t.Parallel()
	p := provenancePack(t)
	paths := []string{}
	for _, version := range []string{"1.9.0", "1.10.0"} {
		p.Manifest.Version, p.Manifest.Title = version, "Library "+version
		dir := filepath.Join(t.TempDir(), "pack")
		if err := SavePackDirectory(dir, p); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, dir)
	}
	reg, err := NewRegistry(paths, []Dependency{{ID: "srd-2014", Version: "^1.0.0"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	record := NewAuthoring(reg, nil).Builtins()[0]
	if record.Title != "Library 1.10.0" || record.Releases[0].Release.Version != "1.9.0" || record.Releases[1].Release.Version != "1.10.0" {
		t.Fatalf("wrong release order/title: %+v", record)
	}
	lock, err := reg.Resolve([]Dependency{{ID: "srd-2014", Version: "1.9.0"}})
	if err != nil {
		t.Fatal(err)
	}
	cat, err := reg.LoadLocked(context.Background(), "en", lock)
	if err != nil {
		t.Fatal(err)
	}
	spell, _ := cat.Spells.Get("magic-missile")
	if spell.Provenance.Version != "1.9.0" || spell.Provenance.PackTitle != "Library 1.9.0" {
		t.Fatalf("older release acquired newer provenance: %+v", spell.Provenance)
	}
}

func TestProvenanceRoundTripAndFork(t *testing.T) {
	t.Parallel()
	p := provenancePack(t)
	first, err := PackDigest(p)
	if err != nil {
		t.Fatal(err)
	}
	p.Provenance["spells"]["magic-missile"] = []string{"phb", "srd-5.1"}
	second, err := PackDigest(p)
	if err != nil || first != second {
		t.Fatalf("source ordering changed digest: %v", err)
	}
	dir := filepath.Join(t.TempDir(), "pack")
	if err := SavePackDirectory(dir, p); err != nil {
		t.Fatal(err)
	}
	b, err := EncodePack(p)
	if err != nil {
		t.Fatal(err)
	}
	a := &Authoring{}
	fork, err := a.Fork(b, "personal", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = DecodePack(fork)
	if err != nil {
		t.Fatal(err)
	}
	// A forked independent core retains its books with the new owner identity.
	file := filepath.Join(t.TempDir(), "personal.json")
	if err := os.WriteFile(file, fork, 0600); err != nil {
		t.Fatal(err)
	}
	reg, err := NewRegistry([]string{file}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	cat, err := reg.Load(context.Background(), rules.Locale("ru"))
	if err != nil {
		t.Fatal(err)
	}
	spell, ok := cat.Spells.Get("personal/magic-missile")
	if !ok || spell.Provenance == nil {
		t.Fatal("missing provenance")
	}
	info := spell.Provenance
	if info.PackID != "personal" || info.Sources[0].ID != "personal:phb" || info.Sources[0].Name != "Книга игрока" {
		t.Fatalf("incorrect fork provenance: %+v", info)
	}
	loaded, err := LoadPack(dir)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := PackDigest(loaded)
	if err != nil || actual != first {
		t.Fatalf("directory roundtrip: %s %v", actual, err)
	}
}
func TestProvenanceRejectsUnknownEntitiesAndSources(t *testing.T) {
	t.Parallel()
	p := provenancePack(t)
	p.Provenance["spells"]["missing"] = []string{"phb"}
	if err := p.Validate(); err == nil {
		t.Fatal("accepted unknown entity")
	}
	delete(p.Provenance["spells"], "missing")
	p.Provenance["spells"]["magic-missile"] = []string{"missing"}
	if err := p.Validate(); err == nil {
		t.Fatal("accepted unknown book")
	}
}
func TestZIPImportPortableEquivalence(t *testing.T) {
	t.Parallel()
	p := provenancePack(t)
	dir := filepath.Join(t.TempDir(), "pack")
	if err := SavePackDirectory(dir, p); err != nil {
		t.Fatal(err)
	}
	for _, prefix := range []string{"", "pack/"} {
		t.Run(prefix, func(t *testing.T) {
			var buf bytes.Buffer
			w := zip.NewWriter(&buf)
			err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(dir, path)
				f, err := w.Create(prefix + filepath.ToSlash(rel))
				if err != nil {
					return err
				}
				b, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				_, err = f.Write(b)
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			if err = w.Close(); err != nil {
				t.Fatal(err)
			}
			b, err := (&Authoring{}).ImportZIP(buf.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			got, err := DecodePack(b)
			if err != nil {
				t.Fatal(err)
			}
			wantDigest, _ := PackDigest(p)
			gotDigest, _ := PackDigest(got)
			if wantDigest != gotDigest {
				t.Fatal("ZIP changed content")
			}
		})
	}
}
func TestZIPRejectsUnsafeAmbiguousAndIncompleteArchives(t *testing.T) {
	t.Parallel()
	manifest, _ := json.Marshal(PackManifest{Files: map[string]string{"entities/spells": "missing.json"}})
	cases := map[string][]string{"traversal": {"../manifest.json"}, "absolute": {"/manifest.json"}, "duplicate": {"manifest.json", "manifest.json"}, "multiple": {"one/manifest.json", "two/manifest.json"}, "nested": {"a/b/manifest.json"}, "missing": {"manifest.json"}}
	for name, paths := range cases {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			w := zip.NewWriter(&buf)
			for _, path := range paths {
				f, err := w.Create(path)
				if err != nil {
					t.Fatal(err)
				}
				_, _ = f.Write(manifest)
			}
			_ = w.Close()
			if _, err := (&Authoring{}).ImportZIP(buf.Bytes()); err == nil {
				t.Fatal("accepted invalid archive")
			}
		})
	}
}

func TestZIPRejectsSymlinksAndExpansionLimits(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		count   int
		size    uint64
		symlink bool
	}{
		{name: "symlink", count: 1, symlink: true}, {name: "expanded-size", count: 1, size: maxPackBytes + 1}, {name: "entry-count", count: 4097},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := zip.NewWriter(&buf)
			for i := 0; i < tc.count; i++ {
				header := &zip.FileHeader{Name: fmt.Sprintf("file-%d", i), UncompressedSize64: tc.size, Method: zip.Store}
				if tc.symlink {
					header.SetMode(os.ModeSymlink | 0777)
				}
				if _, err := w.CreateRaw(header); err != nil {
					t.Fatal(err)
				}
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := (&Authoring{}).ImportZIP(buf.Bytes()); err == nil {
				t.Fatal("accepted unsafe archive")
			}
		})
	}
}
