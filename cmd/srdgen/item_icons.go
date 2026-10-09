package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/promix1722/easydnd/internal/adapter/catalog/file"
)

// Artwork assignments are authored inputs, just like action tags and slots.
func (g *generator) writeItemIcons(manifest *file.PackManifest) error {
	raw, err := os.ReadFile(filepath.Join(rulesDir, "item-icons.json"))
	if err != nil {
		return err
	}
	var labels map[string]map[string]string
	if err = json.Unmarshal(raw, &labels); err != nil {
		return err
	}
	assets := map[string]bool{}
	labelPattern := regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,119}$`)
	assign := func(collection, slug string) (string, error) {
		label := labels[collection][slug]
		if !labelPattern.MatchString(label) {
			return "", fmt.Errorf("item-icons.json: missing or invalid label for %s/%s", collection, slug)
		}
		delete(labels[collection], slug)
		assets[label] = true
		return label, nil
	}
	for _, collection := range []string{"equipment", "magic-items"} {
		raw, err := os.ReadFile(filepath.Join(g.outDir, collection+".json"))
		if err != nil {
			return err
		}
		var rows any
		if collection == "equipment" {
			var items []file.Item
			if err = json.Unmarshal(raw, &items); err != nil {
				return err
			}
			for i := range items {
				if items[i].Icon, err = assign(collection, items[i].Slug); err != nil {
					return err
				}
			}
			rows = items
		} else {
			var items []file.MagicItem
			if err = json.Unmarshal(raw, &items); err != nil {
				return err
			}
			for i := range items {
				if items[i].Icon, err = assign(collection, items[i].Slug); err != nil {
					return err
				}
			}
			rows = items
		}
		if err = g.write(collection+".json", rows); err != nil {
			return err
		}
	}
	for collection, unused := range labels {
		if collection != "equipment" && collection != "magic-items" || len(unused) != 0 {
			return fmt.Errorf("item-icons.json: unknown collection or entries in %s", collection)
		}
	}
	if err = os.MkdirAll(filepath.Join(g.outDir, "item-icons"), 0755); err != nil {
		return err
	}
	for label := range assets {
		name := label + ".webp"
		source := filepath.Join(defaultOut, "item-icons", name)
		destination := filepath.Join(g.outDir, "item-icons", name)
		data, err := os.ReadFile(source)
		if err != nil {
			return err
		}
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
		manifest.Files["icons/items/"+label] = "item-icons/" + name
	}
	return nil
}
