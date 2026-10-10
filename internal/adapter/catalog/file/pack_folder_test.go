package file_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	file "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/domain/pack"
	packuc "github.com/promix1722/easydnd/internal/usecase/pack"
)

func TestIndependentCoresReportSelectionConflict(t *testing.T) {
	t.Parallel()
	r, err := file.NewRegistry([]string{basePath()}, nil, "", file.PackFolder{Path: basePath(), ID: "another-core"})
	if err != nil {
		t.Fatal(err)
	}
	engine := file.NewAuthoring(r, nil)
	roots := []pack.Release{}
	docs := []pack.Document{}
	for _, record := range engine.Builtins() {
		docs = append(docs, record.Releases...)
	}
	for _, record := range engine.Builtins() {
		release := record.Releases[0].Release
		if _, err := engine.Resolve(context.Background(), docs, []pack.Release{release}); err != nil {
			t.Fatalf("individual core should be usable: %v", err)
		}
		roots = append(roots, release)
	}
	for _, selection := range [][]pack.Release{roots, {roots[1], roots[0]}} {
		_, err := engine.Resolve(context.Background(), docs, selection)
		if err == nil {
			t.Fatal("accepted two core rule providers")
		}
		if diagnostic := packuc.Diagnose(err); diagnostic.Reason != "pack.coreConflict" {
			t.Fatalf("expected actionable core conflict, got %+v", diagnostic)
		}
	}
}

func TestAutoloadIndependentCoreAndArchive(t *testing.T) {
	t.Parallel()
	archive := t.TempDir()
	r, err := file.NewRegistry([]string{basePath()}, nil, archive, file.PackFolder{Path: basePath(), ID: "another-core"})
	if err != nil {
		t.Fatal(err)
	}
	if lock := r.DefaultLock(); len(lock.Packs) != 1 || lock.Packs[0].ID != "srd-2014" {
		t.Fatalf("autoload changed default: %+v", lock)
	}
	records := file.NewAuthoring(r, nil).Builtins()
	if len(records) != 2 || records[0].ID != "another-core" || records[1].ID != "srd-2014" {
		t.Fatalf("expected two independent packs, got %+v", records)
	}
	if records[0].Releases[0].Release.Version != records[1].Releases[0].Release.Version {
		t.Fatal("identity override changed the version")
	}
	lock, err := r.Resolve([]file.Dependency{{ID: "another-core", Version: "*"}})
	if err != nil {
		t.Fatal(err)
	}
	cat, err := r.LoadLocked(context.Background(), "ru", lock)
	if err != nil || len(cat.Spells.All()) == 0 {
		t.Fatalf("load independent core: %v", err)
	}
	for _, spell := range cat.Spells.All() {
		if spell.Provenance.PackID != "another-core" {
			t.Fatal("spell retained original pack identity")
		}
	}
	restarted, err := file.NewRegistry([]string{basePath()}, nil, archive)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.LoadLocked(context.Background(), "en", lock); err != nil {
		t.Fatalf("autoload release missing from archive: %v", err)
	}
}

func TestAutoloadRepositoryFolder(t *testing.T) {
	t.Parallel()
	p, err := file.LoadPack(addonPath())
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := file.SavePackDirectory(filepath.Join(root, "pack"), p); err != nil {
		t.Fatal(err)
	}
	r, err := file.NewRegistry([]string{basePath()}, nil, "", file.PackFolder{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.DefaultLock().Packs) != 1 {
		t.Fatal("autoload addon became a default root")
	}
	if _, err := r.Resolve([]file.Dependency{{ID: p.Manifest.ID, Version: p.Manifest.Version}}); err != nil {
		t.Fatal(err)
	}
	// A malformed root manifest must not be hidden by the valid pack/ child.
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := file.NewRegistry([]string{basePath()}, nil, "", file.PackFolder{Path: root}); err == nil {
		t.Fatal("accepted malformed root manifest")
	}
}

func TestAutoloadRejectsInvalidFolders(t *testing.T) {
	t.Parallel()
	for _, folder := range []file.PackFolder{
		{Path: filepath.Join(t.TempDir(), "missing")},
		{Path: t.TempDir()},
		{Path: addonPath()},
		{Path: basePath(), ID: "Invalid ID"},
	} {
		if _, err := file.NewRegistry([]string{basePath()}, nil, "", folder); err == nil {
			t.Fatalf("accepted invalid folder: %+v", folder)
		}
	}
}
