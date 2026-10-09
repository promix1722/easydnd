package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/promix1722/easydnd/internal/adapter/catalog/file"
)

func TestItemIconAssignments(t *testing.T) {
	art, err := os.ReadFile("../../data/srd_5.1/spell-icons/magic-missile.webp")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, mapping string
		wantErr       bool
	}{
		{"shared", `{"equipment":{"sword":"blade"},"magic-items":{"magic-sword":"blade"}}`, false},
		{"missing", `{"equipment":{},"magic-items":{"magic-sword":"blade"}}`, true},
		{"stale", `{"equipment":{"sword":"blade","absent":"blade"},"magic-items":{"magic-sword":"blade"}}`, true},
		{"unsafe", `{"equipment":{"sword":"../blade"},"magic-items":{"magic-sword":"blade"}}`, true},
		{"missing artwork", `{"equipment":{"sword":"absent"},"magic-items":{"magic-sword":"blade"}}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			for _, dir := range []string{rulesDir, filepath.Join(defaultOut, "item-icons"), "generated"} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
			}
			for name, data := range map[string][]byte{
				filepath.Join(rulesDir, "item-icons.json"):            []byte(tc.mapping),
				filepath.Join(defaultOut, "item-icons", "blade.webp"): art,
				"generated/equipment.json":                            []byte(`[{"slug":"sword","category":"weapon","cost":{"amount":1,"unit":"gp"}}]`),
				"generated/magic-items.json":                          []byte(`[{"slug":"magic-sword","category":"weapon"}]`),
			} {
				if err := os.WriteFile(name, data, 0644); err != nil {
					t.Fatal(err)
				}
			}
			g := newGenerator("", "generated", "")
			manifest := file.PackManifest{Files: map[string]string{}}
			err := g.writeItemIcons(&manifest)
			if (err != nil) != tc.wantErr {
				t.Fatalf("writeItemIcons = %v, want error %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if len(manifest.Files) != 1 || manifest.Files["icons/items/blade"] != "item-icons/blade.webp" {
				t.Fatal("shared asset was not stored once")
			}
			for _, collection := range []string{"equipment", "magic-items"} {
				raw, err := os.ReadFile(filepath.Join(g.outDir, collection+".json"))
				if err != nil {
					t.Fatal(err)
				}
				var rows []struct{ Icon string }
				if err := json.Unmarshal(raw, &rows); err != nil || len(rows) != 1 || rows[0].Icon != "blade" {
					t.Fatalf("missing assignment in %s: %s (%v)", collection, raw, err)
				}
			}
		})
	}
}
