package file

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// Authoring reuses the codec/compiler with immutable request-local registries.
// Authorization is performed by the pack service before documents reach Resolve.
type Authoring struct {
	base        *Registry
	repo        pack.Repository
	cache       sync.Map
	private     sync.Map // immutable import releases, excluded from public pack lists
	builtinOnce sync.Once
	builtins    []pack.Record
	decoded     sync.Map // keyed by actual document bytes, never a caller-supplied digest
	compileMu   sync.Mutex
}

func NewAuthoring(base *Registry, repo pack.Repository) *Authoring {
	a := &Authoring{base: base, repo: repo}
	a.Builtins() // Encode immutable installed releases once, before serving requests.
	locales, _ := base.Locales(context.Background())
	for _, locale := range locales {
		lock := base.DefaultLock()
		if c, err := base.LoadLocked(context.Background(), locale, lock); err == nil {
			a.cache.Store(catalogCacheKey(locale, lock), c)
		}
	}
	return a
}
func (a *Authoring) Default() pack.Lock {
	for _, r := range a.base.defaultLock.Packs {
		if r.ID == pack.BaseID {
			l, _ := a.base.Resolve([]Dependency{{ID: r.ID, Version: r.Version}})
			return l
		}
	}
	return a.base.DefaultLock()
}
func (a *Authoring) Builtins() []pack.Record {
	a.builtinOnce.Do(func() { a.builtins = a.buildBuiltinRecords() })
	out := make([]pack.Record, len(a.builtins))
	for i, record := range a.builtins {
		out[i] = record
		out[i].Releases = make([]pack.Document, len(record.Releases))
		for j, doc := range record.Releases {
			out[i].Releases[j] = pack.Document{Release: doc.Release, Data: bytes.Clone(doc.Data)}
		}
	}
	return out
}

func (a *Authoring) buildBuiltinRecords() []pack.Record {
	out := []pack.Record{}
	for id, versions := range a.base.releases {
		r := pack.Record{ID: id, Title: id}
		if id == pack.BaseID {
			r.Title = "SRD 5.1"
		}
		for _, d := range versions {
			b, _ := EncodePack(d)
			a.decoded.Store(sha256.Sum256(b), decodedRelease{d, a.base.identities[d]})
			r.Releases = append(r.Releases, pack.Document{Release: a.base.identities[d], Data: b})
		}
		sort.Slice(r.Releases, func(i, j int) bool {
			a, _ := semver.StrictNewVersion(r.Releases[i].Release.Version)
			b, _ := semver.StrictNewVersion(r.Releases[j].Release.Version)
			return a.LessThan(b)
		})
		if len(r.Releases) > 0 {
			latest := versions[r.Releases[len(r.Releases)-1].Release.Version]
			if latest.Manifest.Title != "" {
				r.Title = latest.Manifest.Title
			}
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func (a *Authoring) NewDraft(id string) []byte {
	deps := []Dependency{}
	for _, r := range a.Default().Packs {
		deps = append(deps, Dependency{ID: r.ID, Version: r.Version})
	}
	b, _ := json.Marshal(PackDocument{Manifest: PackManifest{SchemaVersion: 1, ID: id, Version: "1.0.0", Edition: "2014", Semantics: "1", DefaultLocale: "en", Dependencies: deps}, Entities: map[string]json.RawMessage{}, Locales: map[string]map[string]Bundle{"en": {}}})
	return b
}
func (a *Authoring) CheckDraft(b []byte, id string) error {
	var typed PackDocument
	if err := strictJSON(b, &typed); err != nil {
		return err
	}
	for collection, raw := range typed.Entities {
		constructor, ok := collectionTypes[collection]
		if !ok {
			return fmt.Errorf("unknown collection %s", collection)
		}
		if err := strictJSON(raw, constructor()); err != nil {
			return fmt.Errorf("%s: %w", collection, err)
		}
	}
	var v map[string]any
	if err := strictJSON(b, &v); err != nil {
		return err
	}
	m, ok := v["manifest"].(map[string]any)
	if !ok || m["id"] != id {
		return fmt.Errorf("manifest.id must match pack identity")
	}
	return nil
}

// Fork rewrites references, never prose. It handles qualified refs, compatibility
// slugs, expression inputs and keyed casting definitions throughout mechanics.
func (a *Authoring) Fork(b []byte, id string, mappings map[string]string) ([]byte, error) {
	return rewritePackIdentity(b, id, mappings, true)
}

func rewritePackIdentity(b []byte, id string, mappings map[string]string, resetVersion bool) ([]byte, error) {
	var v map[string]any
	if err := strictJSON(b, &v); err != nil {
		return nil, err
	}
	m, ok := v["manifest"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing manifest")
	}
	old, ok := m["id"].(string)
	if !ok || old == "" {
		return nil, fmt.Errorf("missing pack identity")
	}
	replacements := map[string]string{}
	for k, v := range mappings {
		replacements[k] = v
	}
	replacements[old] = id
	var rewrite func(string) string
	rewrite = func(s string) string {
		for from, to := range replacements {
			if strings.HasPrefix(s, from+":") && len(strings.Split(s, ":")) == 3 {
				return to + s[len(from):]
			}
			if strings.HasPrefix(s, from+"/") {
				return to + s[len(from):]
			}
			if i := strings.Index(s, ":"+from+"/"); i >= 0 {
				return s[:i+1] + to + s[i+1+len(from):]
			}
		}
		for _, prefix := range []string{"abilities.", "modifier:", "ability:", "class:"} {
			if strings.HasPrefix(s, prefix) {
				return prefix + rewrite(strings.TrimPrefix(s, prefix))
			}
		}
		return s
	}
	var collision error
	var walk func(any) any
	walk = func(x any) any {
		switch t := x.(type) {
		case string:
			return rewrite(t)
		case []any:
			for i, c := range t {
				t[i] = walk(c)
			}
		case map[string]any:
			out := map[string]any{}
			for k, c := range t {
				if k == "text" || k == "desc" || k == "name" {
					out[k] = c
				} else {
					key := rewrite(k)
					if _, exists := out[key]; exists {
						collision = fmt.Errorf("dependency mapping creates duplicate key %s", key)
					}
					out[key] = walk(c)
				}
			}
			return out
		}
		return x
	}
	v["entities"] = walk(v["entities"])
	v["mechanics"] = walk(v["mechanics"])
	if collision != nil {
		return nil, collision
	}
	if deps, ok := m["dependencies"].([]any); ok {
		for _, dep := range deps {
			if d, ok := dep.(map[string]any); ok {
				if key, ok := d["id"].(string); ok && mappings[key] != "" {
					d["id"] = mappings[key]
				}
			}
		}
	}
	m["id"] = id
	if old != id && resetVersion {
		m["version"] = "1.0.0"
	}
	delete(m, "files")
	return json.Marshal(v)
}
func (a *Authoring) registry(docs []pack.Document) (*Registry, error) {
	r := &Registry{releases: map[string]map[string]*PackDocument{}, identities: map[*PackDocument]pack.Release{}, contexts: map[string]*catalog.Catalog{}}
	for _, x := range docs {
		parsed, err := a.decodeRelease(x.Data)
		if err != nil {
			return nil, err
		}
		d, identity := parsed.document, parsed.identity
		if identity != x.Release {
			return nil, fmt.Errorf("release identity does not match document")
		}
		if r.releases[identity.ID] == nil {
			r.releases[identity.ID] = map[string]*PackDocument{}
		}
		if old := r.releases[identity.ID][identity.Version]; old != nil && r.identities[old] != identity {
			return nil, fmt.Errorf("conflicting release")
		}
		r.releases[identity.ID][identity.Version] = d
		r.identities[d] = identity
	}
	return r, nil
}
func (a *Authoring) Resolve(ctx context.Context, docs []pack.Document, roots []pack.Release) (pack.Lock, error) {
	for _, root := range roots {
		if doc, ok := a.private.Load(root); ok {
			docs = append(docs, doc.(pack.Document))
		}
	}
	r, err := a.registry(docs)
	if err != nil {
		return pack.Lock{}, err
	}
	deps := []Dependency{}
	for _, p := range roots {
		deps = append(deps, Dependency{ID: p.ID, Version: p.Version})
	}
	l, err := r.Resolve(deps)
	if err != nil {
		return l, err
	}
	for _, root := range roots {
		if root.Digest != "" {
			found := false
			for _, p := range l.Packs {
				if p == root {
					found = true
				}
			}
			if !found {
				return l, fmt.Errorf("release digest mismatch")
			}
		}
	}
	locales, err := r.Locales(ctx)
	if err != nil {
		return l, err
	}
	for _, locale := range locales {
		if _, err = a.compiled(locale, l, func() (*catalog.Catalog, error) { return r.LoadLocked(ctx, locale, l) }); err != nil {
			return l, err
		}
	}
	return l, nil
}
func (a *Authoring) Validate(ctx context.Context, docs []pack.Document, b []byte) (pack.Document, pack.Lock, error) {
	d, err := DecodePack(b)
	if err != nil {
		return pack.Document{}, pack.Lock{}, err
	}
	identity, err := d.Release()
	if err != nil {
		return pack.Document{}, pack.Lock{}, err
	}
	b, err = EncodePack(d)
	if err != nil {
		return pack.Document{}, pack.Lock{}, err
	}
	out := pack.Document{Release: identity, Data: b}
	filtered := []pack.Document{}
	for _, p := range docs {
		if p.Release.ID != identity.ID {
			filtered = append(filtered, p)
		}
	}
	filtered = append(filtered, out)
	l, err := a.Resolve(ctx, filtered, []pack.Release{identity})
	return out, l, err
}
func (a *Authoring) Load(ctx context.Context, locale rules.Locale) (*catalog.Catalog, error) {
	return a.base.LoadLocked(ctx, locale, a.Default())
}
func (a *Authoring) Locales(ctx context.Context) ([]rules.Locale, error) { return a.base.Locales(ctx) }
func (a *Authoring) LoadLocked(ctx context.Context, locale rules.Locale, l pack.Lock) (*catalog.Catalog, error) {
	if err := l.Validate(); err != nil {
		return nil, err
	}
	return a.compiled(locale, l, func() (*catalog.Catalog, error) {
		r, err := a.registryForLock(ctx, l)
		if err != nil {
			return nil, err
		}
		return r.LoadLocked(ctx, locale, l)
	})
}

func catalogCacheKey(locale rules.Locale, lock pack.Lock) string {
	key, _ := json.Marshal(struct {
		Lock   pack.Lock
		Locale rules.Locale
	}{lock, locale})
	return string(key)
}

// Compilation is shared by exact immutable lock and locale. Authorization stays
// in the service, and Resolve still solves against only currently allowed releases.
func (a *Authoring) compiled(locale rules.Locale, lock pack.Lock, load func() (*catalog.Catalog, error)) (*catalog.Catalog, error) {
	key := catalogCacheKey(locale, lock)
	if v, ok := a.cache.Load(key); ok {
		return v.(*catalog.Catalog), nil
	}
	a.compileMu.Lock()
	defer a.compileMu.Unlock()
	if v, ok := a.cache.Load(key); ok {
		return v.(*catalog.Catalog), nil
	}
	c, err := load()
	if err == nil {
		a.cache.Store(key, c)
	}
	return c, err
}

type decodedRelease struct {
	document *PackDocument
	identity pack.Release
}

func (a *Authoring) decodeRelease(data []byte) (decodedRelease, error) {
	key := sha256.Sum256(data)
	if v, ok := a.decoded.Load(key); ok {
		return v.(decodedRelease), nil
	}
	d, err := DecodePack(data)
	if err != nil {
		return decodedRelease{}, err
	}
	identity, err := d.Release()
	if err != nil {
		return decodedRelease{}, err
	}
	parsed := decodedRelease{d, identity}
	actual, _ := a.decoded.LoadOrStore(key, parsed)
	return actual.(decodedRelease), nil
}

func (a *Authoring) registryForLock(ctx context.Context, l pack.Lock) (*Registry, error) {
	records, err := a.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	records = append(records, a.Builtins()...)
	docs := []pack.Document{}
	for _, p := range l.Packs {
		if doc, ok := a.private.Load(p); ok {
			docs = append(docs, doc.(pack.Document))
			continue
		}
		found := false
		for _, r := range records {
			for _, d := range r.Releases {
				if d.Release == p {
					docs = append(docs, d)
					found = true
				}
			}
		}
		if !found {
			return nil, fmt.Errorf("missing pinned release %s", p.ID)
		}
	}
	return a.registry(docs)
}

// CompilePrivate keeps import definitions out of the discoverable authoring
// library while allowing their pinned characters to use them across locales.
func (a *Authoring) CompilePrivate(ctx context.Context, base pack.Lock, session string, body []byte, locale rules.Locale) (*catalog.Catalog, error) {
	r, err := a.registryForLock(ctx, base)
	if err != nil {
		return nil, err
	}
	c, err := r.CompilePrivate(ctx, base, session, body, locale)
	if err != nil {
		return nil, err
	}
	for _, release := range c.Lock.Packs {
		if release.ID != "import-"+session {
			continue
		}
		data, err := EncodePack(r.releases[release.ID][release.Version])
		if err != nil {
			return nil, err
		}
		a.private.Store(release, pack.Document{Release: release, Data: data})
	}
	return c, nil
}

func (a *Authoring) CustomExample(ctx context.Context, lock pack.Lock, ref rules.Ref) (any, error) {
	r, err := a.registryForLock(ctx, lock)
	if err != nil {
		return nil, err
	}
	return r.CustomExample(ctx, lock, ref)
}

// PrivateReleases identifies generated import definitions already pinned by an
// owned character. Copying it retains these without granting library access.
func (a *Authoring) PrivateReleases(lock pack.Lock) pack.Lock {
	retained := lock.Clone()
	retained.Packs = nil
	for _, release := range lock.Packs {
		if _, ok := a.private.Load(release); ok {
			retained.Packs = append(retained.Packs, release)
		}
	}
	return retained
}

// EditorSchema describes every typed wire field, including recursive expressions.
// Definitions avoid recursive expansion; the UI expands only populated values.
type EditorField struct {
	Type       string                 `json:"type"`
	Integer    bool                   `json:"integer,omitempty"`
	Ref        string                 `json:"ref,omitempty"`
	Properties map[string]EditorField `json:"properties,omitempty"`
	Items      *EditorField           `json:"items,omitempty"`
	Optional   bool                   `json:"optional,omitempty"`
}

func (a *Authoring) Schema() []byte {
	defs := map[string]EditorField{}
	var describe func(reflect.Type) EditorField
	describe = func(t reflect.Type) EditorField {
		if t.Kind() == reflect.Pointer {
			f := describe(t.Elem())
			f.Optional = true
			return f
		}
		if t.Kind() == reflect.Struct {
			key := t.Name()
			if _, ok := defs[key]; !ok {
				defs[key] = EditorField{}
				props := map[string]EditorField{}
				for i := 0; i < t.NumField(); i++ {
					sf := t.Field(i)
					tag := sf.Tag.Get("json")
					if tag == "-" {
						continue
					}
					name := strings.Split(tag, ",")[0]
					if name == "" {
						name = sf.Name
					}
					f := describe(sf.Type)
					f.Optional = f.Optional || strings.Contains(tag, ",omitempty")
					props[name] = f
				}
				defs[key] = EditorField{Type: "object", Properties: props}
			}
			return EditorField{Type: "object", Ref: key}
		}
		switch t.Kind() {
		case reflect.Slice, reflect.Array:
			f := describe(t.Elem())
			return EditorField{Type: "array", Items: &f}
		case reflect.Map:
			f := describe(t.Elem())
			return EditorField{Type: "map", Items: &f}
		case reflect.Bool:
			return EditorField{Type: "boolean"}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return EditorField{Type: "number", Integer: true}
		case reflect.Float32, reflect.Float64:
			return EditorField{Type: "number"}
		default:
			kind := "string"
			if t.Name() == "Ref" {
				kind = "reference"
			}
			return EditorField{Type: kind}
		}
	}
	entities := map[string]EditorField{}
	for k, f := range collectionTypes {
		entities[k] = describe(reflect.TypeOf(f()).Elem())
	}
	root := EditorField{Type: "object", Properties: map[string]EditorField{"manifest": describe(reflect.TypeOf(PackManifest{})), "provenance": describe(reflect.TypeOf(map[string]map[string][]string{})), "entities": {Type: "object", Properties: entities}, "mechanics": describe(reflect.TypeOf(PackMechanics{})), "locales": describe(reflect.TypeOf(map[string]map[string]Bundle{}))}}
	b, _ := json.Marshal(struct {
		Root        EditorField            `json:"root"`
		Definitions map[string]EditorField `json:"definitions"`
	}{root, defs})
	return b
}
