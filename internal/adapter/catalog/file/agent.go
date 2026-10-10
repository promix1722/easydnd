package file

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// CompilePrivate compiles a session's complete custom definition set. Identity,
// version, dependencies and edition come from the server, never the model.
// Installed releases are immutable and excluded from the default catalogue.
func (r *Registry) CompilePrivate(_ context.Context, base pack.Lock, session string, body []byte, locale rules.Locale) (*catalog.Catalog, error) {
	if len(body) > 128<<10 {
		return nil, fmt.Errorf("custom definitions too large")
	}
	var p PackDocument
	if err := strictJSON(body, &p); err != nil {
		return nil, err
	}
	if len(p.Mechanics.Overrides) > 0 || p.Mechanics.Core != nil {
		return nil, fmt.Errorf("custom definitions cannot override core rules")
	}
	id := "import-" + session
	lock := base.Clone()
	lock.Packs = nil
	deps := []Dependency{}
	for _, release := range base.Packs {
		if release.ID != id {
			lock.Packs = append(lock.Packs, release)
			deps = append(deps, Dependency{ID: release.ID, Version: release.Version})
		}
	}
	digest := sha256.Sum256(body)
	p.Manifest = PackManifest{SchemaVersion: 1, ID: id, Version: fmt.Sprintf("0.0.0-%x", digest[:12]), Edition: base.Edition, Semantics: base.Semantics, DefaultLocale: locale.String(), Dependencies: deps, Requires: []string{"effects.v1", "resources.v1", "actions.v1", "progressions.v1"}}
	if p.Locales == nil {
		p.Locales = map[string]map[string]Bundle{}
	}
	if p.Locales[locale.String()] == nil {
		if p.Locales["en"] != nil {
			p.Manifest.DefaultLocale = "en"
		} else {
			return nil, fmt.Errorf("custom prose needs the session locale or English")
		}
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	release, err := p.Release()
	if err != nil {
		return nil, err
	}
	lock.Packs = append(lock.Packs, release)
	slices.SortFunc(lock.Packs, func(a, b pack.Release) int { return strings.Compare(a.ID, b.ID) })
	if err := lock.Validate(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	docs, err := r.ordered(base)
	if err != nil {
		return nil, err
	}
	filtered := []*PackDocument{}
	for _, doc := range docs {
		if doc.Manifest.ID != id {
			filtered = append(filtered, doc)
		}
	}
	filtered = append(filtered, &p)
	cat, err := compilePacks(filtered, locale, lock)
	if err != nil {
		return nil, err
	}
	// Validate every available base locale before publication, so a copied or
	// shared character cannot become unreadable when its reader changes language.
	for _, doc := range filtered {
		for tag := range doc.Locales {
			if _, err := compilePacks(filtered, rules.Locale(tag), lock); err != nil {
				return nil, err
			}
		}
	}
	if r.releases[id] == nil {
		r.releases[id] = map[string]*PackDocument{}
	}
	r.releases[id][release.Version] = &p
	r.identities[&p] = release
	return cat, nil
}

// CustomExample provides the actual pack wire shape for an existing entity,
// avoiding a second, incomplete schema for custom spells/classes/items.
func (r *Registry) CustomExample(_ context.Context, lock pack.Lock, ref rules.Ref) (any, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	docs, err := r.ordered(lock)
	if err != nil {
		return nil, err
	}
	for _, p := range docs {
		for collection, data := range p.Entities {
			var entries []map[string]json.RawMessage
			if err := json.Unmarshal(data, &entries); err != nil {
				return nil, err
			}
			for _, entry := range entries {
				var slug string
				_ = json.Unmarshal(entry["slug"], &slug)
				if rules.QualifiedSlug(p.Manifest.ID, slug) != ref.Slug {
					continue
				}
				kindCollection := map[string]string{"spell": "spells", "race": "races", "class": "classes", "subclass": "subclasses", "subrace": "subraces", "feat": "feats", "feature": "features", "trait": "traits", "background": "backgrounds", "item": "equipment", "magic-item": "magic-items"}
				if kindCollection[ref.Kind.String()] != collection {
					continue
				}
				return map[string]any{"collection": collection, "entity": entry, "prose": p.Locales[p.Manifest.DefaultLocale][collection][slug], "instructions": "Use a new local slug and supply entities and locales in a complete private pack body; omit manifest. External references must be canonical. Only encode mechanics explicitly present in the source."}, nil
			}
		}
	}
	return nil, fmt.Errorf("custom example unavailable for %s", strings.TrimSpace(ref.String()))
}
