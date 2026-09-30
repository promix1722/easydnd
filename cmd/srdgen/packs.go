package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// writePack incorporates explicitly authored mechanics missing from the upstream
// compendium. Generated output is still exclusively owned by srdgen.
func (g *generator) writePack(locales []rules.Locale) error {
	input := "data/rules/2014"
	raw, err := os.ReadFile(filepath.Join(input, "mechanics.json"))
	if err != nil {
		return err
	}
	var mechanics file.PackMechanics
	if err = json.Unmarshal(raw, &mechanics); err != nil {
		return err
	}
	if err = g.write("mechanics.json", mechanics); err != nil {
		return err
	}
	versionRaw, err := os.ReadFile(filepath.Join(input, "release.json"))
	if err != nil {
		return err
	}
	var release struct {
		Version string `json:"version"`
	}
	if err = json.Unmarshal(versionRaw, &release); err != nil {
		return err
	}
	manifest := file.PackManifest{SchemaVersion: 1, ID: "srd-2014", Version: release.Version, Edition: "2014", Semantics: "1", DefaultLocale: "en", Source: "SRD 5.1", Attribution: attribution, Requires: []string{"effects.v1", "resources.v1", "progressions.v1"}, Files: map[string]string{"mechanics": "mechanics.json"}}
	for _, name := range file.MechanicsFiles() {
		manifest.Files["entities/"+strings.TrimSuffix(name, ".json")] = name
	}
	for _, locale := range locales {
		for _, name := range file.ProseFiles() {
			manifest.Files["locales/"+locale.String()+"/"+strings.TrimSuffix(name, ".json")] = filepath.ToSlash(filepath.Join("i18n", locale.String(), name))
		}
		resourceInput := filepath.Join(input, "resources.en.json")
		if locale != rules.DefaultLocale {
			resourceInput = filepath.Join(g.transDir, locale.String(), "resources.json")
		}
		raw, err := os.ReadFile(resourceInput)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		var prose file.Bundle
		if err = json.Unmarshal(raw, &prose); err != nil {
			return err
		}
		name := filepath.ToSlash(filepath.Join("i18n", locale.String(), "resources.json"))
		if err = g.write(name, prose); err != nil {
			return err
		}
		manifest.Files["locales/"+locale.String()+"/resources"] = name
	}
	if err = g.write("pack-manifest.json", manifest); err != nil {
		return err
	}
	_, err = file.LoadPack(g.outDir)
	return err
}
