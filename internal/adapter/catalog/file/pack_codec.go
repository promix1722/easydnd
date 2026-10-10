package file

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"

	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

//go:embed pack-schema.json
var packSchema []byte
var schemaOnce sync.Once
var compiledSchema *jsonschema.Schema
var schemaError error

const maxPackBytes = 64 << 20

// PackDocument is the portable, lossless pack representation. Locale keys are
// collection -> local ID -> prose; mechanics labels use the resources collection.
type PackDocument struct {
	Icons      *PackIcons                     `json:"icons,omitempty"`
	Provenance map[string]map[string][]string `json:"provenance,omitempty"`
	Manifest   PackManifest                   `json:"manifest"`
	Entities   map[string]json.RawMessage     `json:"entities"`
	Mechanics  PackMechanics                  `json:"mechanics,omitempty"`
	Locales    map[string]map[string]Bundle   `json:"locales"`
}

func strictJSON(data []byte, out any) error {
	if len(data) > maxPackBytes {
		return fmt.Errorf("pack exceeds %d bytes", maxPackBytes)
	}
	// The standard decoder otherwise accepts duplicate object keys silently.
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := scanJSON(d, 0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	return nil
}
func scanJSON(d *json.Decoder, depth int) error {
	if depth > 64 {
		return fmt.Errorf("JSON exceeds maximum depth")
	}
	t, err := d.Token()
	if err != nil {
		return err
	}
	if number, ok := t.(json.Number); ok {
		value, err := strconv.ParseFloat(number.String(), 64)
		if err != nil || math.Abs(value) > 9007199254740991 {
			return fmt.Errorf("number outside JSON-safe range")
		}
	}
	switch t {
	case json.Delim('{'):
		seen := map[string]bool{}
		for d.More() {
			k, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := k.(string)
			if !ok || seen[key] {
				return fmt.Errorf("duplicate or invalid JSON key %v", k)
			}
			seen[key] = true
			if err := scanJSON(d, depth+1); err != nil {
				return err
			}
		}
		_, err = d.Token()
	case json.Delim('['):
		for d.More() {
			if err := scanJSON(d, depth+1); err != nil {
				return err
			}
		}
		_, err = d.Token()
	}
	return err
}

func DecodePack(data []byte) (*PackDocument, error) {
	var p PackDocument
	if err := strictJSON(data, &p); err != nil {
		return nil, err
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

func (p *PackDocument) Validate() error {
	schemaOnce.Do(func() {
		var doc any
		schemaError = json.Unmarshal(packSchema, &doc)
		if schemaError != nil {
			return
		}
		c := jsonschema.NewCompiler()
		schemaError = c.AddResource("https://easydnd.org/schemas/pack-v1.json", doc)
		if schemaError == nil {
			compiledSchema, schemaError = c.Compile("https://easydnd.org/schemas/pack-v1.json")
		}
	})
	if schemaError != nil {
		return schemaError
	}
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if len(b) > maxPackBytes {
		return fmt.Errorf("pack exceeds %d bytes", maxPackBytes)
	}
	// Everything checked below is a function of these bytes and nothing else,
	// so a document that passed once passes again while it encodes the same.
	// That is most calls: a pack is validated when it is read, again for its
	// digest, and again to be written out, and a second of schema checking
	// each time is what loading the SRD used to cost three times over.
	// ponytail: the set only grows, by 32 bytes per distinct valid pack ever
	// seen by the process; bound it if packs are ever generated in bulk.
	key := sha256.Sum256(b)
	if _, ok := validPacks.Load(key); ok {
		return nil
	}
	if err = p.validateEncoded(b); err != nil {
		return err
	}
	validPacks.Store(key, struct{}{})
	return nil
}

var validPacks sync.Map

func (p *PackDocument) validateEncoded(b []byte) error {
	var value any
	err := json.Unmarshal(b, &value)
	if err != nil {
		return err
	}
	if err = compiledSchema.Validate(value); err != nil {
		return err
	}
	if _, err = semver.StrictNewVersion(p.Manifest.Version); err != nil {
		return fmt.Errorf("pack version: %w", err)
	}
	deps := map[string]bool{}
	for _, dep := range p.Manifest.Dependencies {
		if deps[dep.ID] || dep.ID == p.Manifest.ID {
			return fmt.Errorf("duplicate or self dependency %q", dep.ID)
		}
		deps[dep.ID] = true
		if _, err := semver.NewConstraint(dep.Version); err != nil {
			return err
		}
	}
	for tag, collections := range p.Locales {
		if _, err := language.Parse(tag); err != nil {
			return fmt.Errorf("locale %q: %w", tag, err)
		}
		for collection, bundle := range collections {
			if collection != "resources" && collection != "actions" && collection != "terms" && collection != "sources" {
				if _, ok := collectionTypes[collection]; !ok {
					return fmt.Errorf("unknown locale collection %q", collection)
				}
			}
			for key := range bundle {
				if key == "" {
					return fmt.Errorf("empty locale key")
				}
			}
		}
	}
	if _, ok := p.Locales[p.Manifest.DefaultLocale]; !ok {
		return fmt.Errorf("missing default locale %q", p.Manifest.DefaultLocale)
	}
	for collection, raw := range p.Entities {
		newSlice, ok := collectionTypes[collection]
		if !ok {
			return fmt.Errorf("unknown collection %q", collection)
		}
		if err := strictJSON(raw, newSlice()); err != nil {
			return fmt.Errorf("%s: %w", collection, err)
		}
		var entries []map[string]json.RawMessage
		if err := json.Unmarshal(raw, &entries); err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, e := range entries {
			var id string
			if collection == "class-levels" {
				var cl, sub string
				var level int
				_ = json.Unmarshal(e["class"], &cl)
				_ = json.Unmarshal(e["subclass"], &sub)
				_ = json.Unmarshal(e["level"], &level)
				owned := cl
				if sub != "" {
					owned = sub
				}
				if !validLocalID(owned) {
					return fmt.Errorf("class-level rows must belong to a local class or subclass")
				}
				if cl == "" || level < 1 || level > 100 {
					return fmt.Errorf("invalid class level")
				}
				id = fmt.Sprintf("%s/%s/%d", cl, sub, level)
			} else {
				_ = json.Unmarshal(e["slug"], &id)
				if id == "" || !validLocalID(id) {
					return fmt.Errorf("invalid local ID %q", id)
				}
				if collection == "abilities" {
					if _, ok := rules.ParseAbility(id); !ok || (p.Manifest.ID != pack.BaseID && p.Mechanics.Core == nil) {
						return fmt.Errorf("packs cannot introduce custom ability scores")
					}
				}
				prose, ok := p.Locales[p.Manifest.DefaultLocale][collection][id]
				if !ok || prose.Name == "" {
					return fmt.Errorf("missing default name for %s/%s", collection, id)
				}
			}
			if seen[id] {
				return fmt.Errorf("duplicate %s/%s", collection, id)
			}
			seen[id] = true
		}
		for tag, localized := range p.Locales {
			for id := range localized[collection] {
				if !seen[id] && !foreignProseKey(p, collection, id) {
					return fmt.Errorf("unknown translation %s/%s/%s", tag, collection, id)
				}
			}
		}
	}
	if err := validatePackNamespaces(p); err != nil {
		return err
	}
	if err := p.validateProvenance(); err != nil {
		return err
	}
	if err := p.validateIcons(); err != nil {
		return err
	}
	return validateMechanics(p.Mechanics)
}

// EncodePack excludes physical file paths from the portable representation.
func EncodePack(p *PackDocument) ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	copy := *p
	copy.Manifest.Files = nil
	return json.MarshalIndent(copy, "", "  ")
}

// sortByJSON orders rows by their JSON encoding, encoding each one once. The
// comparison used to encode both sides every time it was asked, which for the
// SRD's few thousand rows was most of what a digest cost.
func sortByJSON(rows []any) {
	type keyed struct {
		key []byte
		row any
	}
	sorted := make([]keyed, len(rows))
	for i, row := range rows {
		key, _ := json.Marshal(row)
		sorted[i] = keyed{key, row}
	}
	slices.SortFunc(sorted, func(x, y keyed) int { return bytes.Compare(x.key, y.key) })
	for i := range sorted {
		rows[i] = sorted[i].row
	}
}

func PackDigest(p *PackDocument) (string, error) {
	b, err := EncodePack(p)
	if err != nil {
		return "", err
	}
	return encodedDigest(b)
}

// encodedDigest is the digest of a pack EncodePack has already written out,
// for a caller that needs both and should not validate and encode it twice.
//
// It is remembered by what it was computed from, for the same reason a valid
// pack is: canonicalising the SRD takes most of a second, and the same bytes
// come back every time the same pack is loaded.
func encodedDigest(b []byte) (string, error) {
	key := sha256.Sum256(b)
	if digest, ok := packDigests.Load(key); ok {
		return digest.(string), nil
	}
	digest, err := canonicalDigest(b)
	if err == nil {
		packDigests.Store(key, digest)
	}
	return digest, err
}

var packDigests sync.Map

func canonicalDigest(b []byte) (string, error) {
	var err error
	// Entity collection order is not semantic. Choices and other arrays remain ordered.
	var doc map[string]any
	if err = json.Unmarshal(b, &doc); err != nil {
		return "", err
	}
	for _, value := range doc["entities"].(map[string]any) {
		a := value.([]any)
		sortByJSON(a)
	}
	if collections, ok := doc["provenance"].(map[string]any); ok {
		for _, value := range collections {
			for _, sources := range value.(map[string]any) {
				slices.SortFunc(sources.([]any), func(a, b any) int { return strings.Compare(a.(string), b.(string)) })
			}
		}
	}
	if m, ok := doc["mechanics"].(map[string]any); ok {
		for _, key := range []string{"resources", "rules", "actions", "overrides"} {
			if rows, ok := m[key].([]any); ok {
				sortByJSON(rows)
			}
		}
	}
	if manifest, ok := doc["manifest"].(map[string]any); ok {
		for _, key := range []string{"dependencies", "requires"} {
			if rows, ok := manifest[key].([]any); ok {
				sortByJSON(rows)
			}
		}
	}
	b, err = json.Marshal(doc)
	if err != nil {
		return "", err
	}
	b, err = jsoncanonicalizer.Transform(b)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(b)
	return hex.EncodeToString(digest[:]), nil
}
func (p *PackDocument) Release() (pack.Release, error) {
	d, err := PackDigest(p)
	return pack.Release{ID: p.Manifest.ID, Version: p.Manifest.Version, Digest: d}, err
}

func LoadPack(path string) (*PackDocument, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer func() { _ = f.Close() }()
		b, err := io.ReadAll(io.LimitReader(f, maxPackBytes+1))
		if err != nil {
			return nil, err
		}
		return DecodePack(b)
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	read := func(name string) ([]byte, error) {
		f, err := root.Open(name)
		if err != nil {
			return nil, err
		}
		defer func() { _ = f.Close() }()
		return io.ReadAll(io.LimitReader(f, maxPackBytes+1))
	}
	p, err := readPackDirectory(read, root.FS())
	if err != nil {
		return nil, err
	}
	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return p, nil
}

// conventionalFiles derives the manifest's files map from the directory layout
// data/pack/srd-5.1 uses: entity collections at the root, mechanics.json,
// provenance.json, i18n/<tag>/<bundle>.json, spell-icons/<slug>.webp and
// item-icons/<label>.webp. A hand-maintained pack then needs no 500-line list
// that goes stale with every icon or language added; a manifest that names its
// files keeps them, which is what portable packs and exports do.
func conventionalFiles(layout fs.FS) (map[string]string, error) {
	files := map[string]string{}
	present := func(name string) (bool, error) {
		_, err := fs.Stat(layout, name)
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return err == nil, err
	}
	for _, name := range MechanicsFiles() {
		if ok, err := present(name); err != nil {
			return nil, err
		} else if ok {
			files["entities/"+strings.TrimSuffix(name, ".json")] = name
		}
	}
	for _, name := range []string{"mechanics.json", "provenance.json"} {
		if ok, err := present(name); err != nil {
			return nil, err
		} else if ok {
			files[strings.TrimSuffix(name, ".json")] = name
		}
	}
	list := func(dir string) ([]fs.DirEntry, error) {
		entries, err := fs.ReadDir(layout, dir)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return entries, err
	}
	tags, err := list(LocaleDir)
	if err != nil {
		return nil, err
	}
	for _, tag := range tags {
		if !tag.IsDir() {
			continue
		}
		bundles, err := list(path.Join(LocaleDir, tag.Name()))
		if err != nil {
			return nil, err
		}
		for _, b := range bundles {
			if name := b.Name(); !b.IsDir() && strings.HasSuffix(name, ".json") {
				files["locales/"+tag.Name()+"/"+strings.TrimSuffix(name, ".json")] = path.Join(LocaleDir, tag.Name(), name)
			}
		}
	}
	for kind, dir := range map[string]string{"spells": "spell-icons", "items": "item-icons"} {
		icons, err := list(dir)
		if err != nil {
			return nil, err
		}
		for _, icon := range icons {
			if name := icon.Name(); !icon.IsDir() && strings.HasSuffix(name, ".webp") {
				files["icons/"+kind+"/"+strings.TrimSuffix(name, ".webp")] = path.Join(dir, name)
			}
		}
	}
	return files, nil
}

// readPackDirectory reads the files a manifest names, or -- when it names none
// and the caller can list the layout -- the files the conventional layout
// holds; see conventionalFiles.
func readPackDirectory(read func(string) ([]byte, error), layout fs.FS) (*PackDocument, error) {
	name := "pack-manifest.json"
	b, err := read(name)
	if os.IsNotExist(err) {
		name = "manifest.json"
		b, err = read(name)
	}
	if err != nil {
		return nil, err
	}
	var manifest PackManifest
	if err = strictJSON(b, &manifest); err != nil {
		return nil, err
	}
	if len(manifest.Files) == 0 && layout != nil {
		if manifest.Files, err = conventionalFiles(layout); err != nil {
			return nil, err
		}
	}
	p := &PackDocument{Manifest: manifest, Entities: map[string]json.RawMessage{}, Locales: map[string]map[string]Bundle{}}
	total := len(b)
	for logical, physical := range manifest.Files {
		if !fs.ValidPath(physical) {
			return nil, fmt.Errorf("invalid pack path %q", physical)
		}
		data, err := read(physical)
		if err != nil {
			return nil, err
		}
		total += len(data)
		if total > maxPackBytes {
			return nil, fmt.Errorf("pack exceeds size limit")
		}
		switch {
		case strings.HasPrefix(logical, "icons/spells/"), strings.HasPrefix(logical, "icons/items/"):
			parts := strings.SplitN(logical, "/", 3)
			id := parts[2]
			if !validLocalID(id) {
				return nil, fmt.Errorf("invalid icon ID %q", id)
			}
			if p.Icons == nil {
				p.Icons = &PackIcons{Spells: map[string][]byte{}, Items: map[string][]byte{}}
			}
			if parts[1] == "items" {
				p.Icons.Items[id] = data
			} else {
				p.Icons.Spells[id] = data
			}
		case logical == "provenance":
			if err = strictJSON(data, &p.Provenance); err != nil {
				return nil, err
			}
		case logical == "mechanics":
			if err = strictJSON(data, &p.Mechanics); err != nil {
				return nil, err
			}
		case strings.HasPrefix(logical, "entities/"):
			p.Entities[strings.TrimPrefix(logical, "entities/")] = data
		case strings.HasPrefix(logical, "locales/"):
			parts := strings.Split(logical, "/")
			if len(parts) != 3 {
				return nil, fmt.Errorf("bad locale file key %q", logical)
			}
			if p.Locales[parts[1]] == nil {
				p.Locales[parts[1]] = map[string]Bundle{}
			}
			var bundle Bundle
			if err = strictJSON(data, &bundle); err != nil {
				return nil, err
			}
			p.Locales[parts[1]][parts[2]] = bundle
		default:
			return nil, fmt.Errorf("unknown pack file key %q", logical)
		}
	}
	return p, nil
}

// SavePackDirectory exports one complete release into a new directory. Existing
// destinations are rejected so a published release cannot be overwritten.
func SavePackDirectory(path string, p *PackDocument) error {
	if err := p.Validate(); err != nil {
		return err
	}
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return err
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return fmt.Errorf("destination already exists or cannot be inspected")
	}
	temp, err := os.MkdirTemp(parent, ".pack-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(temp) }()
	manifest := p.Manifest
	manifest.Files = map[string]string{}
	write := func(logical, name string, v any) error {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		if err = os.MkdirAll(filepath.Join(temp, filepath.Dir(name)), 0755); err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(temp, name), b, 0644); err != nil {
			return err
		}
		manifest.Files[logical] = name
		return nil
	}
	if len(p.Provenance) > 0 {
		if err = write("provenance", "provenance.json", p.Provenance); err != nil {
			return err
		}
	}
	for name, data := range p.Entities {
		if err = write("entities/"+name, name+".json", data); err != nil {
			return err
		}
	}
	if err = write("mechanics", "mechanics.json", p.Mechanics); err != nil {
		return err
	}
	for tag, collections := range p.Locales {
		for name, bundle := range collections {
			if err = write("locales/"+tag+"/"+name, "i18n/"+tag+"/"+name+".json", bundle); err != nil {
				return err
			}
		}
	}
	if p.Icons != nil {
		for kind, icons := range map[string]map[string][]byte{"spells": p.Icons.Spells, "items": p.Icons.Items} {
			dir := strings.TrimSuffix(kind, "s") + "-icons"
			for id, data := range icons {
				name := dir + "/" + id + ".webp"
				if err = os.MkdirAll(filepath.Join(temp, dir), 0755); err != nil {
					return err
				}
				if err = os.WriteFile(filepath.Join(temp, name), data, 0644); err != nil {
					return err
				}
				manifest.Files["icons/"+kind+"/"+id] = name
			}
		}
	}
	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(temp, "manifest.json"), b, 0644); err != nil {
		return err
	}
	return os.Rename(temp, path)
}
