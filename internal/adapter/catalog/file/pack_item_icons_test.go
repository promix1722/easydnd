package file

import (
	"bytes"
	"encoding/json"
	"image"
	"os"
	"strings"
	"testing"

	"github.com/HugoSmits86/nativewebp"

	"github.com/promix1722/easydnd/internal/domain/rules"
)

func itemIconPack(t *testing.T) *PackDocument {
	t.Helper()
	p := iconPack(t)
	p.Icons.Items = map[string][]byte{"blade": p.Icons.Spells["guiding-mark"]}
	additional := map[string]json.RawMessage{"equipment": json.RawMessage(`[
		{"slug":"blade-a","category":"srd-2014:equipment-category:weapon","cost":{"amount":1,"unit":"gp"},"icon":"blade"},
		{"slug":"blade-b","category":"srd-2014:equipment-category:weapon","cost":{"amount":2,"unit":"gp"},"icon":"blade"}
	]`), "magic-items": json.RawMessage(`[{"slug":"magic-blade","category":"srd-2014:equipment-category:weapon","rarity":"uncommon","icon":"blade"}]`)}
	for collection, raw := range additional {
		var existing, extra []json.RawMessage
		if p.Entities[collection] != nil {
			if err := json.Unmarshal(p.Entities[collection], &existing); err != nil {
				t.Fatal(err)
			}
		}
		if err := json.Unmarshal(raw, &extra); err != nil {
			t.Fatal(err)
		}
		merged, err := json.Marshal(append(existing, extra...))
		if err != nil {
			t.Fatal(err)
		}
		p.Entities[collection] = merged
	}
	for collection, bundle := range map[string]Bundle{"equipment": {"blade-a": {Name: "Blade A"}, "blade-b": {Name: "Blade B"}}, "magic-items": {"magic-blade": {Name: "Magic Blade"}}} {
		if p.Locales["en"][collection] == nil {
			p.Locales["en"][collection] = Bundle{}
		}
		for slug, prose := range bundle {
			p.Locales["en"][collection][slug] = prose
		}
	}
	return p
}

func TestSharedItemIconsArePackLocal(t *testing.T) {
	base, err := LoadPack("../../../../data/srd_5.1")
	if err != nil {
		t.Fatal(err)
	}
	a, b := itemIconPack(t), itemIconPack(t)
	b.Manifest.ID = "other"
	b.Icons.Items["blade"], err = os.ReadFile("../../../../data/srd_5.1/spell-icons/fireball.webp")
	if err != nil {
		t.Fatal(err)
	}
	c, err := compilePacks([]*PackDocument{base, a, b}, rules.LocaleEN, iconLock(t, base, a, b))
	if err != nil {
		t.Fatal(err)
	}
	first, _ := c.Items.Get("example/blade-a")
	shared, _ := c.Items.Get("example/blade-b")
	magic, _ := c.MagicItems.Get("example/magic-blade")
	other, _ := c.Items.Get("other/blade-a")
	if !strings.HasPrefix(first.Icon, "data:image/webp;base64,") || first.Icon != shared.Icon || first.Icon != magic.Icon || first.Icon == other.Icon {
		t.Fatal("shared artwork lost or mixed between packs")
	}
	before, err := PackDigest(a)
	if err != nil {
		t.Fatal(err)
	}
	a.Icons.Items["blade"] = b.Icons.Items["blade"]
	after, err := PackDigest(a)
	if err != nil || before == after {
		t.Fatal("digest ignores item artwork")
	}
}

func TestInvalidItemIcons(t *testing.T) {
	for _, mode := range []string{"missing assets", "missing label", "qualified label", "bad label", "invalid image", "wrong size", "bad base64"} {
		t.Run(mode, func(t *testing.T) {
			p := itemIconPack(t)
			switch mode {
			case "missing assets":
				p.Icons = nil
			case "missing label":
				delete(p.Icons.Items, "blade")
			case "qualified label":
				p.Entities["equipment"] = json.RawMessage(strings.ReplaceAll(string(p.Entities["equipment"]), `"icon":"blade"`, `"icon":"other/blade"`))
			case "bad label":
				p.Icons.Items["../blade"] = p.Icons.Items["blade"]
			case "invalid image":
				p.Icons.Items["blade"] = []byte("not an image")
			case "wrong size":
				var small bytes.Buffer
				if err := nativewebp.Encode(&small, image.NewNRGBA(image.Rect(0, 0, 64, 64)), nil); err != nil {
					t.Fatal(err)
				}
				p.Icons.Items["blade"] = small.Bytes()
			case "bad base64":
				data, err := EncodePack(p)
				if err != nil {
					t.Fatal(err)
				}
				var v map[string]any
				if err = json.Unmarshal(data, &v); err != nil {
					t.Fatal(err)
				}
				v["icons"].(map[string]any)["items"].(map[string]any)["blade"] = "!"
				data, _ = json.Marshal(v)
				if _, err = DecodePack(data); err == nil {
					t.Fatal("accepted bad base64")
				}
				return
			}
			if err := p.Validate(); err == nil {
				t.Fatal("accepted invalid item artwork")
			}
		})
	}
}
