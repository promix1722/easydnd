package hexsheet

import (
	"testing"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// The name index is the one part of the importer that is guesswork about
// spelling rather than a lookup, so each of its four rules is pinned by a case
// the reference export actually contains. Every one of these was found by
// running the resolver over the real file, not invented.
func TestIndexFind(t *testing.T) {
	cat := loadTestCatalog(t)
	items := newIndex(cat.Items, func(i catalog.Item) string { return i.Name })

	tests := []struct {
		name string
		in   string
		want rules.Slug
		rule string
	}{
		{"exact", "Leather Armor", "leather-armor", "1: exact"},
		{"case insensitive", "leather armor", "leather-armor", "1: exact"},
		{"apostrophe kept", "Thieves' Tools", "thieves-tools", "1: exact"},
		{"parenthetical kept", "Oil (flask)", "oil-flask", "1: exact"},
		{"comma inverted", "Hooded Lantern", "lantern-hooded", "2: inversion"},
		{"comma inverted with suffix", "Hempen Rope (50 feet)", "rope-hempen-50-feet", "2: inversion"},
		{"srd spelling still works", "Lantern, hooded", "lantern-hooded", "1: exact"},
		{"plural", "Arrows", "arrow", "3: singular"},
		{"comma in a number", "Ball Bearings (bag of 1,000)", "ball-bearings-bag-of-1000", "1: exact"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := items.find(tt.in)
			if !ok {
				t.Fatalf("find(%q) did not resolve; rule %s should have matched", tt.in, tt.rule)
			}
			if got != tt.want {
				t.Errorf("find(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// Nothing should resolve to a near-miss. An importer that quietly turns an
// unknown item into a plausible one is worse than an importer that reports it.
func TestIndexRejectsUnknownNames(t *testing.T) {
	cat := loadTestCatalog(t)
	backgrounds := newIndex(cat.Backgrounds, func(b catalog.Background) string { return b.Name })

	// SRD 5.1 publishes exactly one background, and Urchin is not it.
	if slug, ok := backgrounds.find("Urchin"); ok {
		t.Errorf("Urchin resolved to %q; SRD 5.1 publishes only Acolyte", slug)
	}
	items := newIndex(cat.Items, func(i catalog.Item) string { return i.Name })
	for _, name := range []string{"", "   ", "Vorpal Sword of Nonsense"} {
		if slug, ok := items.find(name); ok {
			t.Errorf("find(%q) resolved to %q, want no match", name, slug)
		}
	}
}

func TestVariantsGeneratesTheInvertedForm(t *testing.T) {
	got := variants("Rope, hempen (50 feet)")
	want := "hempen rope (50 feet)"
	found := false
	for _, v := range got {
		if v == want {
			found = true
		}
	}
	if !found {
		t.Errorf("variants() = %v, want it to include %q", got, want)
	}
}
