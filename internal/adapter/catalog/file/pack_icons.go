package file

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"sync"

	"golang.org/x/image/webp"

	"github.com/promix1722/easydnd/internal/domain/catalog"
)

// PackIcons travels with the release. JSON encodes the bytes as base64;
// directory and ZIP packs use the manifest's icons/<kind>/<label> entries.
type PackIcons struct {
	Spells map[string][]byte `json:"spells,omitempty"`
	Items  map[string][]byte `json:"items,omitempty"`

	// urls is the artwork as data URLs, built on first use. A pointer, set
	// under iconURLMu, so that copying a PackIcons shares it.
	urls *iconURLs
}

// iconURLs is one pack's artwork in the form a catalogue serves it.
type iconURLs struct{ spells, items map[string]string }

var iconURLMu sync.Mutex

// dataURLs encodes the pack's artwork once. Every catalogue compiled from the
// document shares these strings: a release is immutable once installed, and
// encoding 13 MB of icons again for each lock and locale was most of what the
// server held -- 19 MB a catalogue, six catalogues at startup.
func (i *PackIcons) dataURLs() *iconURLs {
	iconURLMu.Lock()
	defer iconURLMu.Unlock()
	if i.urls == nil {
		u := &iconURLs{spells: make(map[string]string, len(i.Spells)), items: make(map[string]string, len(i.Items))}
		for id, data := range i.Spells {
			u.spells[id] = "data:image/webp;base64," + base64.StdEncoding.EncodeToString(data)
		}
		for label, data := range i.Items {
			u.items[label] = "data:image/webp;base64," + base64.StdEncoding.EncodeToString(data)
		}
		i.urls = u
	}
	return i.urls
}

// Releases are repeatedly validated during resolution. Cache successful image
// decodes by content, bounded independently of the number of imported packs.
var validIconCache = struct {
	sync.Mutex
	hashes map[[32]byte]bool
}{hashes: map[[32]byte]bool{}}

func validateIcon(data []byte) error {
	hash := sha256.Sum256(data)
	validIconCache.Lock()
	cached := validIconCache.hashes[hash]
	validIconCache.Unlock()
	if cached {
		return nil
	}
	cfg, err := webp.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width != 128 || cfg.Height != 128 {
		return fmt.Errorf("must be a 128x128 WebP")
	}
	if _, err := webp.Decode(bytes.NewReader(data)); err != nil {
		return err
	}
	validIconCache.Lock()
	if len(validIconCache.hashes) >= 4096 {
		clear(validIconCache.hashes)
	}
	validIconCache.hashes[hash] = true
	validIconCache.Unlock()
	return nil
}

func (p *PackDocument) validateIcons() error {
	icons := PackIcons{}
	if p.Icons != nil {
		icons = *p.Icons
	}
	for label, data := range icons.Items {
		if !validLocalID(label) {
			return fmt.Errorf("invalid item icon label %q", label)
		}
		if err := validateIcon(data); err != nil {
			return fmt.Errorf("item icon %q: %w", label, err)
		}
	}
	for _, collection := range []string{"equipment", "magic-items"} {
		rows, err := p.itemIconRows(collection)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row.Icon != "" && (!validLocalID(row.Icon) || icons.Items[row.Icon] == nil) {
				return fmt.Errorf("%s %q references missing or invalid item icon %q", collection, row.Slug, row.Icon)
			}
		}
	}
	var spells []struct {
		Slug string `json:"slug"`
	}
	if raw := p.Entities["spells"]; raw != nil {
		if err := json.Unmarshal(raw, &spells); err != nil {
			return err
		}
	}
	ids := map[string]bool{}
	for _, spell := range spells {
		ids[spell.Slug] = true
	}
	for id, data := range icons.Spells {
		if !validLocalID(id) || !ids[id] {
			return fmt.Errorf("icon references unknown spell %q", id)
		}
		if err := validateIcon(data); err != nil {
			return fmt.Errorf("spell icon %q: %w", id, err)
		}
	}
	return nil
}

type itemIconRow struct {
	Slug string `json:"slug"`
	Icon string `json:"icon"`
}

func (p *PackDocument) itemIconRows(collection string) ([]itemIconRow, error) {
	var rows []itemIconRow
	if raw := p.Entities[collection]; raw != nil {
		if err := json.Unmarshal(raw, &rows); err != nil {
			return nil, err
		}
	}
	return rows, nil
}

func applyIcons(c *catalog.Catalog, docs []*PackDocument) error {
	icons := map[string]string{}
	itemIcons := map[string]map[string]string{"equipment": {}, "magic-items": {}}
	palette := map[string]string{}
	for _, p := range docs {
		if p.Icons == nil {
			continue
		}
		urls := p.Icons.dataURLs()
		for id, url := range urls.spells {
			icons[normalizeID(p.Manifest.ID, id)] = url
		}
		assets := urls.items
		// ponytail: labels are not namespaced, so two packs naming the same
		// label leave the later one's picture; qualify them if packs collide.
		maps.Copy(palette, assets)
		for collection, resolved := range itemIcons {
			rows, err := p.itemIconRows(collection)
			if err != nil {
				return err
			}
			for _, row := range rows {
				resolved[normalizeID(p.Manifest.ID, row.Slug)] = assets[row.Icon]
			}
		}
	}
	spells := c.Spells.All()
	for i := range spells {
		spells[i].Icon = icons[spells[i].Slug.String()]
	}
	c.Spells = catalog.NewCollection(spells)
	items := c.Items.All()
	for i := range items {
		items[i].Icon = itemIcons["equipment"][items[i].Slug.String()]
	}
	c.Items = catalog.NewCollection(items)
	magicItems := c.MagicItems.All()
	for i := range magicItems {
		magicItems[i].Icon = itemIcons["magic-items"][magicItems[i].Slug.String()]
	}
	c.MagicItems = catalog.NewCollection(magicItems)
	c.ItemIcons = palette
	return nil
}
