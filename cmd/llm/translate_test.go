package main

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestLoadMatchingLeavesPreservesOnlySourcePaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.json")
	if err := os.WriteFile(path, []byte(`{"known":"перевод","extra":"лишнее"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	if err := loadMatchingLeaves(path, nil, map[string]string{"/known": "source"}, out); err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(out, map[string]string{"/known": "перевод"}) {
		t.Fatalf("got %v", out)
	}
}

// The reroll case: keep the hand-checked names, re-request everything else.
// Without -preserve, pointing -existing at the output file preserves the whole
// file and the rerun translates nothing.
func TestLoadMatchingLeavesKeepsOnlyNamedLeaves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.json")
	body := `{"fireball":{"name":"Огненный шар","desc":["старый перевод"],"fields":{"material":"сера"}}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	source := map[string]string{
		"/fireball/name":            "Fireball",
		"/fireball/desc/0":          "A bright streak flashes...",
		"/fireball/fields/material": "a tiny ball of bat guano",
	}
	out := map[string]string{}
	if err := loadMatchingLeaves(path, []string{"name"}, source, out); err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(out, map[string]string{"/fireball/name": "Огненный шар"}) {
		t.Fatalf("got %v, want the name alone", out)
	}
}

func TestCheckpointRejectsStaleOrBrokenLeaves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.json")
	data := `{"/good":"{{n}} существ","/broken":"без токена","/stale":"старое"}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	source := map[string]string{"/good": "{{n}} creatures", "/broken": "{{n}} creatures"}
	if err := loadCheckpoint(path, source, out); err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(out, map[string]string{"/good": "{{n}} существ"}) {
		t.Fatalf("got %v", out)
	}
}

// One fixture covering both payload shapes the tool meets: a nested SRD prose
// bundle and a flat i18next catalogue, plus a key that needs pointer escaping.
const fixture = `{
  "acid-arrow": {
    "name": "Acid Arrow",
    "desc": ["A shimmering green arrow.", "It streaks toward a target."],
    "fields": {"material": "Powdered rhubarb leaf."},
    "blocks": {"higherLevel": ["The damage increases by 1d4."]}
  },
  "account.added": "Added {{when}}",
  "build.warn_one": "{{count}} change",
  "build.warn_other": "{{count}} changes",
  "odd/key": "slashed",
  "empty": ""
}`

func TestCollectAndSplice(t *testing.T) {
	var doc any
	if err := json.Unmarshal([]byte(fixture), &doc); err != nil {
		t.Fatal(err)
	}

	leaves := map[string]string{}
	collect(doc, "", leaves)

	want := map[string]string{
		"/acid-arrow/name":                 "Acid Arrow",
		"/acid-arrow/desc/0":               "A shimmering green arrow.",
		"/acid-arrow/desc/1":               "It streaks toward a target.",
		"/acid-arrow/fields/material":      "Powdered rhubarb leaf.",
		"/acid-arrow/blocks/higherLevel/0": "The damage increases by 1d4.",
		"/account.added":                   "Added {{when}}",
		"/build.warn_one":                  "{{count}} change",
		"/build.warn_other":                "{{count}} changes",
		"/odd~1key":                        "slashed",
	}
	if !maps.Equal(leaves, want) {
		t.Fatalf("collect:\n got %v\nwant %v", leaves, want)
	}

	tr := map[string]string{}
	for p, s := range leaves {
		tr[p] = strings.ToUpper(s)
	}
	doc = splice(doc, "", tr)

	got := map[string]string{}
	collect(doc, "", got)
	for p, s := range got {
		if s != strings.ToUpper(want[p]) {
			t.Errorf("%s: got %q, want the uppercased source", p, s)
		}
	}

	// Structure, keys and the empty leaf must survive untouched.
	m := doc.(map[string]any)
	if m["empty"] != "" {
		t.Errorf("empty leaf changed to %v", m["empty"])
	}
	if _, ok := m["acid-arrow"].(map[string]any)["blocks"].(map[string]any)["higherLevel"].([]any); !ok {
		t.Error("nested structure did not survive the splice")
	}
}

func TestValuesMatch(t *testing.T) {
	for _, tc := range []struct {
		name                string
		source, translation string
		ok                  bool
	}{
		{"placeholder kept", "Added {{when}}", "Добавлено {{when}}", true},
		{"nothing to keep", "plain", "просто", true},
		{"placeholders reordered", "{{a}} and {{b}}", "{{b}} и {{a}}", true},
		{"placeholder dropped", "Added {{when}}", "Добавлено", false},
		{"placeholder renamed", "Added {{when}}", "Добавлено {{че}}", false},
		{"placeholder repeat lost", "{{n}} of {{n}}", "{{n}}", false},

		// The half that makes this a rules-text gate rather than a
		// placeholder check.
		{"dice kept", "8d6 fire damage", "8d6 урона огнём", true},
		{"dice transposed", "8d6 fire damage", "6d8 урона огнём", false},
		{"bare die kept", "takes d4 damage", "получает d4 урона", true},
		{"bare die grown a 1", "takes d4 damage", "получает 1d4 урона", false},
		{"distance changed", "within 120 feet", "в пределах 12 футов", false},
		{"distance dropped", "within 120 feet", "в пределах видимости", false},
		{"numbers reordered", "1 of 20", "20 из 1", true},
		{"dice not read as two numbers", "8d6", "8 и 6", false},
		// "N feet square" is an N-by-N area, and Russian says so literally.
		// Counting repeats rejected four correct translations on the first run.
		{"square idiom repeats a number", "an area 5 feet square", "область 5 на 5 футов", true},
		{"repeat still needs the number to exist", "an area 5 feet square", "область 5 на 10 футов", false},
	} {
		if got := valuesMatch(tc.source, tc.translation); got != tc.ok {
			t.Errorf("%s: valuesMatch(%q, %q) = %v, want %v", tc.name, tc.source, tc.translation, got, tc.ok)
		}
	}
}

func TestChunk(t *testing.T) {
	leaves := map[string]string{
		"/a": "xxxx", "/b": "xxxx", "/c": "xxxx", "/d": "xxxx", "/e": "xxxx",
	}

	byCount := chunk(leaves, 2, 1000)
	if got := len(byCount); got != 3 {
		t.Fatalf("leaf cap: got %d chunks, want 3", got)
	}
	byChars := chunk(leaves, 100, 8)
	if got := len(byChars); got != 3 {
		t.Fatalf("char cap: got %d chunks, want 3", got)
	}

	var all []string
	for _, c := range byChars {
		all = append(all, c...)
	}
	slices.Sort(all)
	if want := []string{"/a", "/b", "/c", "/d", "/e"}; !reflect.DeepEqual(all, want) {
		t.Fatalf("chunks lost leaves: got %v", all)
	}
}

func TestChunkKeepsAnEntryWhole(t *testing.T) {
	// Two spells of three leaves each. Both caps below would have cut through
	// the middle of the second spell when the caps were hard.
	leaves := map[string]string{
		"/fireball/blocks/higherLevel/0": "xxxx",
		"/fireball/desc/0":               "xxxx",
		"/fireball/name":                 "xxxx",
		"/mending/blocks/higherLevel/0":  "xxxx",
		"/mending/desc/0":                "xxxx",
		"/mending/name":                  "xxxx",
	}
	for _, caps := range []struct{ leaves, chars int }{{4, 1000}, {100, 16}} {
		for _, c := range chunk(leaves, caps.leaves, caps.chars) {
			counted := map[string]int{}
			for _, p := range c {
				counted[entryOf(p)]++
			}
			for entry, got := range counted {
				if got != 3 {
					t.Errorf("caps %v: entry %q split across requests, %d of 3 leaves here", caps, entry, got)
				}
			}
		}
	}

	// An entry larger than the caps still travels in one request rather than
	// being cut in half -- the caps are soft, which is the whole point.
	big := map[string]string{
		"/wish/desc/0": strings.Repeat("x", 50),
		"/wish/desc/1": strings.Repeat("x", 50),
	}
	if got := chunk(big, 1, 10); len(got) != 1 {
		t.Errorf("oversized entry split into %d requests, want 1", len(got))
	}
}
