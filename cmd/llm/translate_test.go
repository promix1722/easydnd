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
	if err := loadMatchingLeaves(path, map[string]string{"/known": "source"}, out); err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(out, map[string]string{"/known": "перевод"}) {
		t.Fatalf("got %v", out)
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

func TestPlaceholdersMatch(t *testing.T) {
	for _, tc := range []struct {
		source, translation string
		ok                  bool
	}{
		{"Added {{when}}", "Добавлено {{when}}", true},
		{"plain", "просто", true},
		{"{{a}} and {{b}}", "{{b}} и {{a}}", true},
		{"Added {{when}}", "Добавлено", false},
		{"Added {{when}}", "Добавлено {{че}}", false},
		{"{{n}} of {{n}}", "{{n}}", false},
	} {
		if got := placeholdersMatch(tc.source, tc.translation); got != tc.ok {
			t.Errorf("placeholdersMatch(%q, %q) = %v, want %v", tc.source, tc.translation, got, tc.ok)
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
