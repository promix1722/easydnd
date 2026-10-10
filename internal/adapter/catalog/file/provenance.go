package file

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

func (p *PackDocument) validateProvenance() error {
	for id, name := range p.Manifest.Sources {
		if !validLocalID(strings.ReplaceAll(id, ".", "-")) || strings.TrimSpace(name) == "" {
			return fmt.Errorf("invalid source %q", id)
		}
	}
	for _, bundles := range p.Locales {
		for id := range bundles["sources"] {
			if _, ok := p.Manifest.Sources[id]; !ok {
				return fmt.Errorf("unknown source translation %q", id)
			}
		}
	}
	for collection, entries := range p.Provenance {
		raw, ok := p.Entities[collection]
		if !ok || collection == "class-levels" {
			return fmt.Errorf("unknown provenance collection %q", collection)
		}
		var rows []struct {
			Slug string `json:"slug"`
		}
		if err := json.Unmarshal(raw, &rows); err != nil {
			return err
		}
		ids := map[string]bool{}
		for _, r := range rows {
			ids[r.Slug] = true
		}
		for id, sources := range entries {
			if !ids[id] {
				return fmt.Errorf("unknown provenance entity %s/%s", collection, id)
			}
			seen := map[string]bool{}
			for _, source := range sources {
				if _, ok := p.Manifest.Sources[source]; !ok || seen[source] {
					return fmt.Errorf("invalid source %q for %s/%s", source, collection, id)
				}
				seen[source] = true
			}
		}
	}
	return nil
}

func applyProvenance(c *catalog.Catalog, docs []*PackDocument, locale rules.Locale, lock pack.Lock) {
	metadata := map[string]map[string]*catalog.Provenance{}
	for _, p := range docs {
		var release pack.Release
		for _, r := range lock.Packs {
			if r.ID == p.Manifest.ID {
				release = r
			}
		}
		title := p.Manifest.Title
		if title == "" {
			title = p.Manifest.ID
			if title == pack.BaseID {
				title = "SRD 5.1"
			}
		}
		sources := localeBundle(p, locale, "sources")
		for collection, raw := range p.Entities {
			if collection == "class-levels" {
				continue
			}
			var rows []struct {
				Slug   string `json:"slug"`
				Source string `json:"source"`
			}
			_ = json.Unmarshal(raw, &rows)
			if metadata[collection] == nil {
				metadata[collection] = map[string]*catalog.Provenance{}
			}
			for _, row := range rows {
				ids := slices.Clone(p.Provenance[collection][row.Slug])
				if len(ids) == 0 && row.Source != "" {
					ids = []string{row.Source}
				}
				slices.Sort(ids)
				info := &catalog.Provenance{PackID: release.ID, PackTitle: title, Version: release.Version, Digest: release.Digest}
				for _, id := range ids {
					name := p.Manifest.Sources[id]
					if name == "" {
						name = id
					}
					if translated := sources[normalizeID(p.Manifest.ID, id)].Name; translated != "" {
						name = translated
					}
					info.Sources = append(info.Sources, catalog.BookSource{ID: release.ID + ":" + id, Name: name})
				}
				metadata[collection][normalizeID(p.Manifest.ID, row.Slug)] = info
			}
		}
	}
	c.Races = tagCollection(c.Races, metadata["races"])
	c.Subraces = tagCollection(c.Subraces, metadata["subraces"])
	c.Traits = tagCollection(c.Traits, metadata["traits"])
	c.Classes = tagCollection(c.Classes, metadata["classes"])
	c.Subclasses = tagCollection(c.Subclasses, metadata["subclasses"])
	c.Features = tagCollection(c.Features, metadata["features"])
	c.Backgrounds = tagCollection(c.Backgrounds, metadata["backgrounds"])
	c.Feats = tagCollection(c.Feats, metadata["feats"])
	c.Items = tagCollection(c.Items, metadata["equipment"])
	c.MagicItems = tagCollection(c.MagicItems, metadata["magic-items"])
	c.Spells = tagCollection(c.Spells, metadata["spells"])
}

// Every tagged wire entity embeds Entry; reflection is confined to this adapter.
func tagCollection[T catalog.Keyed](collection catalog.Collection[T], metadata map[string]*catalog.Provenance) catalog.Collection[T] {
	rows := collection.All()
	for i := range rows {
		entry := reflect.ValueOf(&rows[i]).Elem().FieldByName("Entry").Addr().Interface().(*catalog.Entry)
		entry.Provenance = metadata[entry.Slug.String()]
	}
	return catalog.NewCollection(rows)
}
