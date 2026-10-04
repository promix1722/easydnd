package file

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"

	"golang.org/x/image/webp"

	"github.com/promix1722/easydnd/internal/domain/catalog"
)

// PackIcons travels with the release. JSON encodes the bytes as base64;
// directory and ZIP packs use the manifest's icons/spells/<id> file entries.
type PackIcons struct {
	Spells map[string][]byte `json:"spells,omitempty"`
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
	if p.Icons == nil {
		return nil
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
	for id, data := range p.Icons.Spells {
		if !validLocalID(id) || !ids[id] {
			return fmt.Errorf("icon references unknown spell %q", id)
		}
		if err := validateIcon(data); err != nil {
			return fmt.Errorf("spell icon %q: %w", id, err)
		}
	}
	return nil
}

func applyIcons(c *catalog.Catalog, docs []*PackDocument) {
	icons := map[string]string{}
	for _, p := range docs {
		if p.Icons == nil {
			continue
		}
		for id, data := range p.Icons.Spells {
			icons[normalizeID(p.Manifest.ID, id)] = "data:image/webp;base64," + base64.StdEncoding.EncodeToString(data)
		}
	}
	spells := c.Spells.All()
	for i := range spells {
		spells[i].Icon = icons[spells[i].Slug.String()]
	}
	c.Spells = catalog.NewCollection(spells)
}
