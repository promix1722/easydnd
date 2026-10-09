package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"
	"golang.org/x/text/language"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// Registry keeps immutable releases and contexts. Only explicitly configured
// roots affect the default; archived releases remain available for old locks.
type Registry struct {
	releases    map[string]map[string]*PackDocument
	identities  map[*PackDocument]pack.Release
	defaultLock pack.Lock
	mu          sync.Mutex
	contexts    map[string]*catalog.Catalog
}

func NewRegistry(paths []string, roots []Dependency, archive string, folders ...PackFolder) (*Registry, error) {
	r := &Registry{releases: map[string]map[string]*PackDocument{}, contexts: map[string]*catalog.Catalog{}, identities: map[*PackDocument]pack.Release{}}
	// A digest validates, encodes and canonicalises the whole pack, and loading
	// one asks for it up to three times and then for the encoding again to
	// archive it. Nothing here changes a document once it is read, so each is
	// encoded and identified once.
	type identity struct {
		release pack.Release
		encoded []byte
	}
	known := map[*PackDocument]identity{}
	encode := func(p *PackDocument) (identity, error) {
		if id, ok := known[p]; ok {
			return id, nil
		}
		b, err := EncodePack(p)
		if err != nil {
			return identity{}, err
		}
		digest, err := encodedDigest(b)
		if err != nil {
			return identity{}, err
		}
		id := identity{pack.Release{ID: p.Manifest.ID, Version: p.Manifest.Version, Digest: digest}, b}
		known[p] = id
		return id, nil
	}
	identify := func(p *PackDocument) (pack.Release, error) {
		id, err := encode(p)
		return id.release, err
	}
	install := func(p *PackDocument) error {
		release, err := identify(p)
		if err != nil {
			return err
		}
		if r.releases[p.Manifest.ID] == nil {
			r.releases[p.Manifest.ID] = map[string]*PackDocument{}
		}
		if old := r.releases[p.Manifest.ID][p.Manifest.Version]; old != nil && r.identities[old] != release {
			return fmt.Errorf("immutable release %s@%s changed", release.ID, release.Version)
		}
		r.releases[p.Manifest.ID][p.Manifest.Version] = p
		r.identities[p] = release
		return nil
	}

	if archive != "" {
		if err := os.MkdirAll(archive, 0755); err != nil {
			return nil, err
		}
		files, err := filepath.Glob(filepath.Join(archive, "*.json"))
		if err != nil {
			return nil, err
		}
		for _, path := range files {
			p, err := LoadPack(path)
			if err != nil {
				return nil, err
			}
			release, err := identify(p)
			if err != nil {
				return nil, err
			}
			if filepath.Base(path) != release.Digest+".json" {
				return nil, fmt.Errorf("archived pack digest mismatch: %s", path)
			}
			if err = install(p); err != nil {
				return nil, err
			}
		}
	}
	register := func(p *PackDocument) error {
		if err := install(p); err != nil {
			return err
		}
		if archive != "" {
			release, err := identify(p)
			if err != nil {
				return err
			}
			name := filepath.Join(archive, release.Digest+".json")
			if _, err = os.Stat(name); os.IsNotExist(err) {
				id, err := encode(p)
				if err != nil {
					return err
				}
				b := id.encoded
				temp, err := os.CreateTemp(archive, ".pack-")
				if err != nil {
					return err
				}
				tmpName := temp.Name()
				_, writeErr := temp.Write(b)
				syncErr := temp.Sync()
				closeErr := temp.Close()
				if writeErr != nil || syncErr != nil || closeErr != nil {
					_ = os.Remove(tmpName)
					return fmt.Errorf("archiving pack: write=%v sync=%v close=%v", writeErr, syncErr, closeErr)
				}
				if err = os.Rename(tmpName, name); err != nil {
					_ = os.Remove(tmpName)
					return err
				}
			}
		}
		return nil
	}
	configured := map[string]string{}
	configuredReleases := []Dependency{}
	for _, path := range paths {
		p, err := LoadPack(path)
		if err != nil {
			return nil, err
		}
		if err = register(p); err != nil {
			return nil, err
		}
		if previous, ok := configured[p.Manifest.ID]; ok && previous != p.Manifest.Version && len(roots) == 0 {
			return nil, fmt.Errorf("multiple configured versions of %s require explicit default_packs", p.Manifest.ID)
		}
		configured[p.Manifest.ID] = p.Manifest.Version
		configuredReleases = append(configuredReleases, Dependency{ID: p.Manifest.ID, Version: p.Manifest.Version})
	}
	if len(roots) == 0 {
		for id, version := range configured {
			roots = append(roots, Dependency{ID: id, Version: version})
		}
	}
	for _, folder := range folders {
		p, err := loadPackFolder(folder)
		if err != nil {
			return nil, fmt.Errorf("autoload pack %s: %w", folder.Path, err)
		}
		if err := register(p); err != nil {
			return nil, fmt.Errorf("autoload pack %s: %w", folder.Path, err)
		}
		configuredReleases = append(configuredReleases, Dependency{ID: p.Manifest.ID, Version: p.Manifest.Version})
	}
	lock, err := r.Resolve(roots)
	if err != nil {
		return nil, err
	}
	r.defaultLock = lock
	// Validate every selected locale before readiness, not on its first request.
	locales, err := r.Locales(context.Background())
	if err != nil {
		return nil, err
	}
	for _, locale := range locales {
		if _, err = r.Load(context.Background(), locale); err != nil {
			return nil, err
		}
	}
	// An installed but currently unselected addon must also compile before
	// readiness, so selecting it later cannot discover an unresolved graph.
	for _, root := range configuredReleases {
		own, err := r.Resolve([]Dependency{root})
		if err != nil {
			return nil, err
		}
		for _, locale := range locales {
			if _, err := r.LoadLocked(context.Background(), locale, own); err != nil {
				return nil, err
			}
		}
	}
	return r, nil
}

func (r *Registry) Resolve(roots []Dependency) (pack.Lock, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Backtracking ensures a later dependency can constrain an earlier selection.
	var solve func(map[string]*PackDocument, []Dependency) (map[string]*PackDocument, error)
	solve = func(selected map[string]*PackDocument, pending []Dependency) (map[string]*PackDocument, error) {
		pending = slices.Clone(pending)
		slices.SortFunc(pending, func(a, b Dependency) int {
			if n := strings.Compare(a.ID, b.ID); n != 0 {
				return n
			}
			return strings.Compare(a.Version, b.Version)
		})
		if len(pending) == 0 {
			return selected, nil
		}
		dep := pending[0]
		constraint, err := semver.NewConstraint(dep.Version)
		if err != nil {
			return nil, err
		}
		if p := selected[dep.ID]; p != nil {
			v, _ := semver.StrictNewVersion(p.Manifest.Version)
			if !constraint.Check(v) {
				return nil, fmt.Errorf("dependency conflict for %s", dep.ID)
			}
			return solve(selected, pending[1:])
		}
		candidates := make([]*PackDocument, 0)
		for _, p := range r.releases[dep.ID] {
			v, _ := semver.StrictNewVersion(p.Manifest.Version)
			if constraint.Check(v) {
				candidates = append(candidates, p)
			}
		}
		slices.SortFunc(candidates, func(a, b *PackDocument) int {
			av, _ := semver.StrictNewVersion(a.Manifest.Version)
			bv, _ := semver.StrictNewVersion(b.Manifest.Version)
			if n := av.Compare(bv); n != 0 {
				return -n
			}
			return strings.Compare(a.Manifest.Version, b.Manifest.Version)
		})
		var dependencyError error
		for _, p := range candidates {
			next := map[string]*PackDocument{}
			for k, v := range selected {
				next[k] = v
			}
			next[dep.ID] = p
			todo := append(slices.Clone(pending[1:]), p.Manifest.Dependencies...)
			if found, err := solve(next, todo); err == nil {
				return found, nil
			} else {
				dependencyError = err
			}
		}
		if dependencyError != nil {
			return nil, dependencyError
		}
		return nil, fmt.Errorf("no installed release satisfies %s %s", dep.ID, dep.Version)
	}
	selected, err := solve(map[string]*PackDocument{}, roots)
	if err != nil {
		return pack.Lock{}, err
	}
	lock := pack.Lock{Semantics: pack.Semantics}
	for _, p := range selected {
		if lock.Edition == "" {
			lock.Edition = p.Manifest.Edition
		}
		if p.Manifest.Edition != lock.Edition {
			return pack.Lock{}, fmt.Errorf("incompatible rules editions")
		}
		release := r.identities[p]
		lock.Packs = append(lock.Packs, release)
	}
	slices.SortFunc(lock.Packs, func(a, b pack.Release) int { return strings.Compare(a.ID, b.ID) })
	if _, err := r.ordered(lock); err != nil {
		return pack.Lock{}, err
	}
	return lock, lock.Validate()
}

func (r *Registry) ordered(lock pack.Lock) ([]*PackDocument, error) {
	if err := lock.Validate(); err != nil {
		return nil, err
	}
	selected := map[string]*PackDocument{}
	for _, p := range lock.Packs {
		doc := r.releases[p.ID][p.Version]
		if doc == nil {
			return nil, fmt.Errorf("missing pinned release %s@%s", p.ID, p.Version)
		}
		got := r.identities[doc]
		if got != p || doc.Manifest.Edition != lock.Edition {
			return nil, fmt.Errorf("pinned release mismatch %s", p.ID)
		}
		selected[p.ID] = doc
	}
	var out []*PackDocument
	state := map[string]int{}
	var visit func(string) error
	visit = func(id string) error {
		if state[id] == 1 {
			return fmt.Errorf("pack dependency cycle at %s", id)
		}
		if state[id] == 2 {
			return nil
		}
		doc := selected[id]
		if doc == nil {
			return fmt.Errorf("missing dependency %s", id)
		}
		state[id] = 1
		for _, dep := range doc.Manifest.Dependencies {
			target := selected[dep.ID]
			if target == nil {
				return fmt.Errorf("missing dependency %s", dep.ID)
			}
			constraint, err := semver.NewConstraint(dep.Version)
			if err != nil {
				return err
			}
			v, _ := semver.StrictNewVersion(target.Manifest.Version)
			if !constraint.Check(v) {
				return fmt.Errorf("incompatible dependency %s", dep.ID)
			}
			if err := visit(dep.ID); err != nil {
				return err
			}
		}
		state[id] = 2
		out = append(out, doc)
		return nil
	}
	for _, p := range lock.Packs {
		if err := visit(p.ID); err != nil {
			return nil, err
		}
	}
	return out, nil
}
func (r *Registry) DefaultLock() pack.Lock { return r.defaultLock.Clone() }
func (r *Registry) Locales(context.Context) ([]rules.Locale, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	set := map[string]bool{"en": true}
	for _, versions := range r.releases {
		for _, p := range versions {
			for tag := range p.Locales {
				set[tag] = true
			}
		}
	}
	out := make([]rules.Locale, 0, len(set))
	for tag := range set {
		out = append(out, rules.Locale(tag))
	}
	slices.Sort(out)
	return out, nil
}
func (r *Registry) Load(ctx context.Context, locale rules.Locale) (*catalog.Catalog, error) {
	return r.LoadLocked(ctx, locale, r.defaultLock)
}
func (r *Registry) LoadLocked(_ context.Context, locale rules.Locale, lock pack.Lock) (*catalog.Catalog, error) {
	if _, err := language.Parse(locale.String()); err != nil {
		return nil, fmt.Errorf("invalid locale")
	}
	keyBytes, _ := json.Marshal(struct {
		Lock   RulesLock
		Locale string
	}{LockOf(lock), locale.String()})
	key := string(keyBytes)
	r.mu.Lock()
	defer r.mu.Unlock()
	if c := r.contexts[key]; c != nil {
		return c, nil
	}
	docs, err := r.ordered(lock)
	if err != nil {
		return nil, err
	}
	c, err := compilePacks(docs, locale, lock)
	if err != nil {
		return nil, err
	}
	r.contexts[key] = c
	return c, nil
}

func localeBundle(p *PackDocument, locale rules.Locale, collection string) Bundle {
	base := p.Locales[p.Manifest.DefaultLocale][collection]
	tag := locale.String()
	chain := []string{}
	for tag != "" {
		chain = append(chain, tag)
		pos := strings.LastIndex(tag, "-")
		if pos < 0 {
			break
		}
		tag = tag[:pos]
	}
	for i := len(chain) - 1; i >= 0; i-- {
		base = resolve(p.Locales[chain[i]][collection], base)
	}
	out := Bundle{}
	for key, value := range base {
		id := normalizeID(p.Manifest.ID, key)
		if collection == "abilities" {
			id = normalizeAbility(key)
		}
		out[id] = value
	}
	return out
}

func compilePacks(docs []*PackDocument, locale rules.Locale, lock pack.Lock) (*catalog.Catalog, error) {
	// Replacement cores also share localized ability identities. Report the
	// actionable core conflict before merging their entities and translations.
	coreProviders := 0
	for _, p := range docs {
		if p.Mechanics.Core != nil {
			coreProviders++
			if coreProviders > 1 {
				return nil, fmt.Errorf("multiple core rule providers")
			}
		}
	}
	entities := map[string][]any{}
	prose := map[string]Bundle{}
	mechanics := PackMechanics{Casting: map[string]CastingProfile{}}
	for _, p := range docs {
		rows, err := normalizedEntities(p)
		if err != nil {
			return nil, err
		}
		for name, values := range rows {
			entities[name] = append(entities[name], values...)
		}
		for _, filename := range append(ProseFiles(), "resources.json", "actions.json") {
			name := strings.TrimSuffix(filename, ".json")
			if prose[name] == nil {
				prose[name] = Bundle{}
			}
			for key, value := range localeBundle(p, locale, name) {
				// A second pack may fill what the first left blank -- an
				// overlay's descriptions -- but never restate it. A field
				// counts as set once the locale or its fallback supplies it.
				if existing, exists := prose[name][key]; exists {
					merged, err := fillProse(existing, value)
					if err != nil {
						return nil, fmt.Errorf("localized identity %s/%s: %w", name, key, err)
					}
					value = merged
				}
				prose[name][key] = value
			}
		}
		m, err := normalizeMechanics(p)
		if err != nil {
			return nil, err
		}
		if m.Core != nil {
			mechanics.Core = m.Core
		}
		mechanics.Actions = append(mechanics.Actions, m.Actions...)
		mechanics.Resources = append(mechanics.Resources, m.Resources...)
		mechanics.Rules = append(mechanics.Rules, m.Rules...)
		mechanics.SpellBenefits = append(mechanics.SpellBenefits, m.SpellBenefits...)
		mechanics.ChoiceRequirements = append(mechanics.ChoiceRequirements, m.ChoiceRequirements...)
		for id, c := range m.Casting {
			if _, ok := mechanics.Casting[id]; ok {
				return nil, fmt.Errorf("duplicate casting profile %s", id)
			}
			mechanics.Casting[id] = c
		}
	}
	if mechanics.Core == nil {
		return nil, fmt.Errorf("rules context has no core policy")
	}
	if err := applyOverrides(docs, &mechanics); err != nil {
		return nil, err
	}
	if err := validateMechanics(mechanics); err != nil {
		return nil, err
	}
	if err := validateReferences(docs, entities, mechanics, prose); err != nil {
		return nil, err
	}
	if err := validateChoices(entities, mechanics); err != nil {
		return nil, err
	}
	if err := orderRules(&mechanics); err != nil {
		return nil, err
	}
	slices.SortFunc(mechanics.Resources, func(a, b ResourceDefinition) int { return strings.Compare(a.ID, b.ID) })
	slices.SortFunc(mechanics.Actions, func(a, b ActionDefinition) int { return strings.Compare(a.ID, b.ID) })
	files := map[string][]byte{}
	for _, filename := range MechanicsFiles() {
		name := strings.TrimSuffix(filename, ".json")
		values := entities[name]
		if values == nil {
			values = []any{}
		}
		b, err := json.Marshal(values)
		if err != nil {
			return nil, err
		}
		files[filename] = b
	}
	for _, filename := range ProseFiles() {
		name := strings.TrimSuffix(filename, ".json")
		b, err := json.Marshal(prose[name])
		if err != nil {
			return nil, err
		}
		files["i18n/en/"+filename] = b
	}
	files[FileManifest], _ = json.Marshal(Manifest{Ruleset: lock.Edition, Locales: []string{"en"}})
	c, err := NewMemorySource(files).Load(context.Background(), locale)
	if err != nil {
		return nil, err
	}
	c.Lock = lock.Clone()
	applyProvenance(c, docs, locale, lock)
	if err := applyIcons(c, docs); err != nil {
		return nil, err
	}
	c.Mechanics, err = mechanics.domain(prose["resources"], prose["actions"])
	if err != nil {
		return nil, err
	}
	// Relationships are indexed from children, allowing addons to extend parents.
	classes := c.Classes.All()
	for i := range classes {
		for _, sub := range c.Subclasses.All() {
			if sub.Class == classes[i].Slug && !slices.Contains(classes[i].Subclasses, sub.Slug) {
				classes[i].Subclasses = append(classes[i].Subclasses, sub.Slug)
			}
		}
	}
	c.Classes = catalog.NewCollection(classes)
	races := c.Races.All()
	for i := range races {
		for _, sub := range c.Subraces.All() {
			if sub.Race == races[i].Slug && !slices.Contains(races[i].Subraces, sub.Slug) {
				races[i].Subraces = append(races[i].Subraces, sub.Slug)
			}
		}
	}
	c.Races = catalog.NewCollection(races)
	return c, nil
}

func normalizeInput(packID, value string) string {
	for _, prefix := range []string{"ability:", "modifier:"} {
		if strings.HasPrefix(value, prefix) {
			return prefix + normalizeAbility(strings.TrimPrefix(value, prefix))
		}
	}
	if r, ok := rules.ParseRef(value); ok && r.Kind == rules.RefAbility {
		return "ability:" + normalizeAbility(value)
	}
	if len(strings.Split(value, ":")) == 3 {
		if r, ok := rules.ParseRef(value); ok {
			return r.Kind.String() + ":" + r.Slug.String()
		}
	}
	for _, prefix := range []string{"class:", "ability:", "modifier:"} {
		if strings.HasPrefix(value, prefix) {
			return prefix + normalizeID(packID, strings.TrimPrefix(value, prefix))
		}
	}
	return value
}
func normalizeExpression(packID string, e *Expression) {
	if e == nil {
		return
	}
	if e.Op == "read" {
		e.Ref = normalizeInput(packID, e.Ref)
	}
	for i := range e.Args {
		normalizeExpression(packID, &e.Args[i])
	}
}
func normalizeMechanics(p *PackDocument) (PackMechanics, error) {
	b, err := json.Marshal(p.Mechanics)
	if err != nil {
		return PackMechanics{}, err
	}
	var m PackMechanics
	if err = json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	for i := range m.Resources {
		v := &m.Resources[i]
		v.ID = normalizeID(p.Manifest.ID, v.ID)
		v.Owner = Ref(normalizeRef(p.Manifest.ID, string(v.Owner)))
		v.Input = normalizeInput(p.Manifest.ID, v.Input)
		normalizeExpression(p.Manifest.ID, v.Capacity)
		if v.SharedKey != "" {
			v.SharedKey = normalizeID(p.Manifest.ID, v.SharedKey)
		}
		for j := range v.Recovery {
			normalizeExpression(p.Manifest.ID, &v.Recovery[j].Amount)
			normalizeExpression(p.Manifest.ID, v.Recovery[j].When)
		}
	}
	for i := range m.Rules {
		v := &m.Rules[i]
		v.ID = normalizeID(p.Manifest.ID, v.ID)
		v.Owner = Ref(normalizeRef(p.Manifest.ID, string(v.Owner)))
		normalizeExpression(p.Manifest.ID, v.When)
		for j := range v.Effects {
			e := &v.Effects[j]
			e.Ref = Ref(normalizeRef(p.Manifest.ID, string(e.Ref)))
			normalizeExpression(p.Manifest.ID, &e.Value)
			if strings.HasPrefix(e.Target, "abilities.") {
				e.Target = "abilities." + normalizeAbility(strings.TrimPrefix(e.Target, "abilities."))
			}
		}
		raw, _ := json.Marshal(v.Choices)
		var choices any
		_ = json.Unmarshal(raw, &choices)
		choices = normalizeValue(p.Manifest.ID, "", choices)
		raw, _ = json.Marshal(choices)
		if err = json.Unmarshal(raw, &v.Choices); err != nil {
			return m, err
		}
	}
	for i := range m.Actions {
		a := &m.Actions[i]
		a.ID = normalizeID(p.Manifest.ID, a.ID)
		a.Owner = Ref(normalizeRef(p.Manifest.ID, string(a.Owner)))
		normalizeExpression(p.Manifest.ID, a.When)
		for j := range a.Costs {
			cost := &a.Costs[j]
			cost.Resource = normalizeID(p.Manifest.ID, cost.Resource)
			normalizeExpression(p.Manifest.ID, &cost.Amount)
		}
	}
	for i := range m.ChoiceRequirements {
		requirement := &m.ChoiceRequirements[i]
		requirement.Prompt = normalizeID(p.Manifest.ID, requirement.Prompt)
		requirement.Pick = normalizeID(p.Manifest.ID, requirement.Pick)
		for j := range requirement.AnyProficiency {
			requirement.AnyProficiency[j] = normalizeID(p.Manifest.ID, requirement.AnyProficiency[j])
		}
	}
	for i := range m.SpellBenefits {
		b := &m.SpellBenefits[i]
		b.ID = normalizeID(p.Manifest.ID, b.ID)
		b.Owner = Ref(normalizeRef(p.Manifest.ID, string(b.Owner)))
		b.Class = normalizeID(p.Manifest.ID, b.Class)
		b.Ability = normalizeAbility(b.Ability)
		for j := range b.Spells {
			b.Spells[j] = normalizeID(p.Manifest.ID, b.Spells[j])
		}
	}
	casting := map[string]CastingProfile{}
	for id, v := range m.Casting {
		v.List = normalizeID(p.Manifest.ID, v.List)
		v.Ability = normalizeAbility(v.Ability)
		v.Resource = normalizeID(p.Manifest.ID, v.Resource)
		casting[normalizeID(p.Manifest.ID, id)] = v
	}
	m.Casting = casting
	return m, nil
}
