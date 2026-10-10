package file_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	file "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// overlay is a descriptions-only pack: no entities, locale bundles keyed by
// canonical references into srd-2014. The private easydnd-2014 pack is one.
func overlay(bundles map[string]file.Bundle) *file.PackDocument {
	return &file.PackDocument{
		Manifest: file.PackManifest{SchemaVersion: 1, ID: "overlay", Version: "1.0.0", Edition: "2014", Semantics: "1", DefaultLocale: "en",
			Dependencies: []file.Dependency{{ID: "srd-2014", Version: ">=1.0.0"}}},
		Entities: map[string]json.RawMessage{},
		Locales:  map[string]map[string]file.Bundle{"en": bundles},
	}
}

func TestOverlayFillsProseTheBaseLacks(t *testing.T) {
	path := writeDocument(t, overlay(map[string]file.Bundle{
		"classes": {"srd-2014:class:wizard": {Desc: []string{"A scholar of the arcane."}}},
	}))
	r, err := file.NewRegistry([]string{basePath(), path}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, locale := range []rules.Locale{rules.LocaleEN, rules.LocaleRU} {
		cat, err := r.Load(context.Background(), locale)
		if err != nil {
			t.Fatal(err)
		}
		wizard, ok := cat.Classes.Get("wizard")
		if !ok || strings.Join(wizard.Desc, "") != "A scholar of the arcane." {
			t.Fatalf("%s: wizard = %+v, want the overlay's description", locale, wizard)
		}
		if locale == rules.LocaleRU && wizard.Name == "Wizard" {
			t.Fatal("the overlay's English description replaced the base's Russian name")
		}
	}
	if packs := file.NewAuthoring(r, nil).Default().Packs; len(packs) != 2 {
		t.Fatalf("Default() = %v, want the base and the overlay", packs)
	}
}

func TestOverlayCannotRestateOrInventProse(t *testing.T) {
	for name, bundles := range map[string]map[string]file.Bundle{
		"restates a description": {"spells": {"srd-2014:spell:fireball": {Desc: []string{"boom"}}}},
		"restates a name":        {"spells": {"srd-2014:spell:fireball": {Name: "Fire Ball"}}},
		"names a missing entity": {"spells": {"srd-2014:spell:no-such-spell": {Desc: []string{"?"}}}},
	} {
		if _, err := file.NewRegistry([]string{basePath(), writeDocument(t, overlay(bundles))}, nil, ""); err == nil {
			t.Errorf("%s: compiled", name)
		}
	}
}

func TestOverlayKeysMustNameADeclaredDependency(t *testing.T) {
	for name, bundles := range map[string]map[string]file.Bundle{
		"undeclared pack":  {"spells": {"other:spell:fireball": {Desc: []string{"x"}}}},
		"bare slug":        {"spells": {"fireball": {Desc: []string{"x"}}}},
		"wrong collection": {"classes": {"srd-2014:spell:fireball": {Desc: []string{"x"}}}},
	} {
		b, _ := json.Marshal(overlay(bundles))
		if _, err := file.DecodePack(b); err == nil {
			t.Errorf("%s: validated", name)
		}
	}
}

// A directory pack lists its files in the manifest or, when it names none,
// by layout: the hand-maintained SRD pack relies on that for 650 icons and
// every language it will ever add.
func TestDirectoryPackNeedsNoFilesMap(t *testing.T) {
	base, err := file.LoadPack(basePath())
	if err != nil {
		t.Fatal(err)
	}
	if base.Icons == nil || base.Icons.Spells["fireball"] == nil || len(base.Icons.Items) == 0 || base.Locales["ru"]["spells"] == nil {
		t.Fatal("convention did not adopt the SRD pack's icons and locales")
	}
	doc := overlay(map[string]file.Bundle{
		"classes": {"srd-2014:class:wizard": {Desc: []string{"A scholar of the arcane."}}},
		"spells":  {"srd-2014:spell:fireball": {Fields: map[string]string{"custom": "x"}}},
	})
	dir := filepath.Join(t.TempDir(), "pack")
	if err := file.SavePackDirectory(dir, doc); err != nil {
		t.Fatal(err)
	}
	want, _ := file.PackDigest(doc)
	manifest := filepath.Join(dir, "pack-manifest.json")
	write := func(files map[string]string) {
		t.Helper()
		m := doc.Manifest
		m.Files = files
		b, _ := json.Marshal(m)
		if err := os.WriteFile(manifest, b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(nil)
	byLayout, err := file.LoadPack(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := file.PackDigest(byLayout); got != want {
		t.Fatalf("layout-derived pack digest = %s, want %s", got, want)
	}
	write(map[string]string{"locales/en/classes": "i18n/en/classes.json"})
	explicit, err := file.LoadPack(dir)
	if err != nil {
		t.Fatal(err)
	}
	if explicit.Locales["en"]["spells"] != nil {
		t.Fatal("an explicit files map did not win over the layout")
	}
}
