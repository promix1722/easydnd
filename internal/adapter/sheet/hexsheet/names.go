package hexsheet

import (
	"regexp"
	"strings"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// index resolves a display name to a catalogue slug.
//
// Nothing else in the codebase does this, and for good reason: internally
// every reference is a slug, and names are locale-dependent prose. An import
// is the one place a name is all there is, so the lookup lives here rather
// than in the catalogue, where it would invite use from code that has a slug
// available.
//
// The index is built from whatever locale the catalogue was loaded in, so a
// Russian catalogue resolves Russian names. Slugs are locale-independent, so
// rule 4 below works regardless.
type index struct {
	bySlug map[rules.Slug]bool
	byName map[string]rules.Slug
}

// newIndex builds a lookup over one collection.
func newIndex[T catalog.Keyed](c catalog.Collection[T], name func(T) string) *index {
	idx := &index{
		bySlug: make(map[rules.Slug]bool),
		byName: make(map[string]rules.Slug),
	}
	for _, entry := range c.All() {
		slug := entry.Key()
		idx.bySlug[slug] = true
		for _, variant := range variants(name(entry)) {
			// First writer wins, so an entry's own name always beats another
			// entry's generated variant.
			if _, taken := idx.byName[variant]; !taken {
				idx.byName[variant] = slug
			}
		}
	}
	return idx
}

// find resolves a name, reporting whether it matched.
//
// Four rules, in order, each one earned by a real line of the reference
// export rather than invented in advance:
//
//  1. exact, case- and space-insensitive -- almost everything
//  2. comma inversion: the SRD writes "Lantern, hooded" where a sheet writes
//     "Hooded Lantern"
//  3. singular: a sheet carries "Arrows", the compendium lists "Arrow"
//  4. the slugified name, for entries whose prose name has drifted from the
//     slug the generator built
func (i *index) find(name string) (rules.Slug, bool) {
	key := normalize(name)
	if key == "" {
		return "", false
	}
	if slug, ok := i.byName[key]; ok {
		return slug, true
	}
	if singular := strings.TrimSuffix(key, "s"); singular != key {
		if slug, ok := i.byName[singular]; ok {
			return slug, true
		}
	}
	if slug := rules.Slug(slugify(name)); i.bySlug[slug] {
		return slug, true
	}
	return "", false
}

// commaName matches an SRD name of the form "Rope, hempen (50 feet)".
var commaName = regexp.MustCompile(`^([^,]+),\s*([^(]+?)\s*(\(.*\))?$`)

// variants returns the keys a name should be indexed under: the name itself,
// and its comma-inverted form where it has one.
func variants(name string) []string {
	key := normalize(name)
	if key == "" {
		return nil
	}
	out := []string{key}
	if m := commaName.FindStringSubmatch(key); m != nil {
		inverted := strings.TrimSpace(m[2] + " " + m[1])
		if m[3] != "" {
			inverted += " " + m[3]
		}
		if inverted != key {
			out = append(out, normalize(inverted))
		}
	}
	return out
}

// spaces collapses any run of whitespace.
var spaces = regexp.MustCompile(`\s+`)

// normalize is the form names are compared in: lower case, single-spaced,
// trimmed. Punctuation is kept, because "Thieves' Tools" and "Thieves Tools"
// are the same name but "Oil (flask)" and "Oil" are not.
func normalize(s string) string {
	return spaces.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), " ")
}

// nonSlug is every run of characters a slug cannot contain.
var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// slugify renders a name the way cmd/srdgen does, so that a name whose prose
// has drifted from its slug still resolves.
func slugify(s string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), "-"), "-")
}
