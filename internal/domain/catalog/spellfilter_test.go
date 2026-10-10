package catalog

import "testing"

func TestSpellProvenanceFilters(t *testing.T) {
	spell := Spell{Entry: Entry{Name: "Test", Provenance: &Provenance{PackID: "personal", Sources: []BookSource{{ID: "personal:phb"}, {ID: "personal:xge"}}}}}
	for _, tc := range []struct {
		filter SpellFilter
		match  bool
	}{
		{SpellFilter{}, true}, {SpellFilter{PackIDs: []string{"personal"}, Sources: []string{"personal:xge"}}, true},
		{SpellFilter{PackIDs: []string{"other"}, Sources: []string{"personal:xge"}}, false}, {SpellFilter{Sources: []string{"other:xge"}}, false},
	} {
		if got := tc.filter.Matches(spell); got != tc.match {
			t.Fatalf("%+v: %v", tc.filter, got)
		}
	}
	spell.Provenance = nil
	if (SpellFilter{Sources: []string{"personal:xge"}}).Matches(spell) {
		t.Fatal("untagged spell matched a source")
	}
}
