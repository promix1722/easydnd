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
	input := rulesDir
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
	manifest := file.PackManifest{SchemaVersion: 1, ID: "srd-2014", Version: release.Version, Edition: "2014", Semantics: "1", DefaultLocale: "en", Source: "SRD 5.1", Attribution: attribution, Requires: []string{"effects.v1", "resources.v1", "actions.v1", "progressions.v1"}, Files: map[string]string{"mechanics": "mechanics.json"}}
	for _, name := range file.MechanicsFiles() {
		manifest.Files["entities/"+strings.TrimSuffix(name, ".json")] = name
	}
	for _, locale := range locales {
		for _, name := range file.ProseFiles() {
			manifest.Files["locales/"+locale.String()+"/"+strings.TrimSuffix(name, ".json")] = filepath.ToSlash(filepath.Join("i18n", locale.String(), name))
		}
		// Resource and action names belong to the authored mechanics, so
		// their English is authored beside them.
		for _, bundle := range []string{"resources", "actions"} {
			bundleInput := filepath.Join(input, bundle+".en.json")
			if locale != rules.DefaultLocale {
				bundleInput = filepath.Join(g.transDir, locale.String(), bundle+".json")
			}
			raw, err := os.ReadFile(bundleInput)
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
			name := filepath.ToSlash(filepath.Join("i18n", locale.String(), bundle+".json"))
			if err = g.write(name, prose); err != nil {
				return err
			}
			manifest.Files["locales/"+locale.String()+"/"+bundle] = name
		}
	}

	manifest.Title = "SRD 5.1"
	manifest.Sources = map[string]string{"srd-5.1": "SRD 5.1"}
	provenance := map[string]map[string][]string{}
	for _, collection := range []string{"races", "subraces", "traits", "classes", "subclasses", "features", "backgrounds", "feats", "equipment", "magic-items", "spells"} {
		data, err := os.ReadFile(filepath.Join(g.outDir, collection+".json"))
		if err != nil {
			return err
		}
		var rows []struct {
			Slug string `json:"slug"`
		}
		if err = json.Unmarshal(data, &rows); err != nil {
			return err
		}
		provenance[collection] = map[string][]string{}
		for _, row := range rows {
			provenance[collection][row.Slug] = []string{"srd-5.1"}
		}
	}
	if err = g.write("provenance.json", provenance); err != nil {
		return err
	}
	var spells []struct {
		Slug string `json:"slug"`
	}
	spellData, err := os.ReadFile(filepath.Join(g.outDir, "spells.json"))
	if err != nil {
		return err
	}
	if err = json.Unmarshal(spellData, &spells); err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Join(g.outDir, "spell-icons"), 0755); err != nil {
		return err
	}
	for _, spell := range spells {
		name := spell.Slug + ".webp"
		source := filepath.Join(g.iconDir, name)
		destination := filepath.Join(g.outDir, "spell-icons", name)
		data, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		// The default input is committed inside this pack. Never truncate that
		// input when regenerating in place; only copy into a different output.
		sourceInfo, err := os.Stat(source)
		if err != nil {
			return err
		}
		destInfo, destErr := os.Stat(destination)
		if destErr != nil || !os.SameFile(sourceInfo, destInfo) {
			if err = os.WriteFile(destination, data, 0644); err != nil {
				return err
			}
		}
		manifest.Files["icons/spells/"+spell.Slug] = "spell-icons/" + name
	}
	if err = g.writeItemIcons(&manifest); err != nil {
		return err
	}
	manifest.Files["provenance"] = "provenance.json"
	if err = g.write("pack-manifest.json", manifest); err != nil {
		return err
	}
	_, err = file.LoadPack(g.outDir)
	return err
}
