package auth

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// TestGeneratedDisplayNamesAreValid walks the whole cross product, which is
// what lets newDisplayName treat its normalizeDisplayName call as a formality.
//
// It is also the only test that guards users_display_name_len, the CHECK on
// users.display_name bounding it to 1..64 characters: the Postgres adapter
// tests need TEST_DATABASE_URL and are skipped by `make verify`, so a word long
// enough to break the constraint would otherwise reach a real database first.
func TestGeneratedDisplayNamesAreValid(t *testing.T) {
	for _, adjective := range displayNameAdjectives {
		for _, noun := range displayNameNouns {
			candidate := adjective + " " + noun

			got, err := normalizeDisplayName(candidate)
			if err != nil {
				t.Fatalf("normalizeDisplayName(%q): %v", candidate, err)
			}
			// Unchanged, not merely accepted: a word carrying stray
			// whitespace would still normalize, and would still be wrong.
			if got != candidate {
				t.Fatalf("normalizeDisplayName(%q) = %q, want it unchanged", candidate, got)
			}
			if n := utf8.RuneCountInString(got); n < MinDisplayName || n > MaxDisplayName {
				t.Fatalf("%q is %d runes, outside %d..%d", got, n, MinDisplayName, MaxDisplayName)
			}
		}
	}
}

// TestWordListsAreSortedAndUnique keeps the lists reviewable. A duplicate is
// invisible in fifty lines of string literals and silently doubles that word's
// share of the draw; sorting is what makes the duplicate visible in the first
// place.
func TestWordListsAreSortedAndUnique(t *testing.T) {
	for name, words := range map[string][]string{
		"adjectives": displayNameAdjectives,
		"nouns":      displayNameNouns,
	} {
		if !slices.IsSorted(words) {
			t.Errorf("%s are not sorted", name)
		}
		seen := make(map[string]bool, len(words))
		for _, word := range words {
			if seen[word] {
				t.Errorf("%s contains %q twice", name, word)
			}
			seen[word] = true
		}
	}
}

// TestNewDisplayNameVaries is deliberately weak: it asserts nothing about the
// distribution, because that would be a flaky test about crypto/rand. What it
// catches is the realistic failure -- an index that is always zero, which
// would name every account in the deployment the same thing.
func TestNewDisplayNameVaries(t *testing.T) {
	seen := make(map[string]bool)
	for range 50 {
		name, err := newDisplayName()
		if err != nil {
			t.Fatalf("newDisplayName: %v", err)
		}
		if strings.Count(name, " ") != 1 {
			t.Fatalf("newDisplayName() = %q, want two words", name)
		}
		seen[name] = true
	}
	if len(seen) < 2 {
		t.Fatalf("50 draws produced %d distinct name(s): %v", len(seen), seen)
	}
}
