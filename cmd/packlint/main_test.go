package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// One pack with one of each defect: the test is that every check fires on the
// text it was written for, and that clean text stays quiet.
func TestLintFindsTheDefectsItWasWrittenFor(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("pack-manifest.json", `{"defaultLocale":"en"}`)
	write("i18n/en/backgrounds.json", `{
		"folk-hero": {"name": "Folk Hero", "desc": [
			"Equipment:. A set of items, a phb, and a pouch",
			"See {@book the PHB}.",
			"d10 | Defining Event",
			"| Lever | Up |",
			"| 1 | Legs extend. |",
			"Make a saving throw.",
			"It deals 2d6 fire damage within 30 feet."]},
		"clean": {"name": "Clean", "desc": ["Nothing to see within 10 feet.", "| d4 | Result |", "|---|---|", "| 1 | Nothing |"]},
		"bold": {"name": "**Bold**"},
		"untranslated": {"name": "Untranslated"}
	}`)
	write("i18n/ru/backgrounds.json", `{
		"folk-hero": {"name": "Народный герой", "desc": [
			"Владение навыками: Выживание , Уход за животными .",
			"к6 Афера 1 Я мухлюю в играх. 2 Я подделываю монеты.",
			"Наносит 2к8 урона огнём в пределах 20 футов, заклинание конус холода [cone of cold] со СТ 10 и Channel Divinity."]},
		"clean": {"name": "Чистый", "desc": ["Ничего в пределах 10 футов.", "| d4 | Итог |", "|---|---|", "| 1 | Ничего |"]},
		"ghost": {"name": "Призрак"}
	}`)

	findings, coverage, err := lint(dir, map[string]string{"saving throw": "спасбросок", "feet": "фут"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, f := range findings {
		if strings.HasPrefix(f.where, "backgrounds/clean") {
			t.Errorf("clean entry reported: %+v", f)
		}
		got[f.locale+" "+f.check] = true
	}
	for _, want := range []string{
		"en stray-punctuation", "en source-code-leak", "en markup-leftover", "en malformed-table",
		"en markdown-in-plain-field", "ru nonstandard-abbreviation", "ru untranslated-words", "ru glossary-term",
		"ru space-before-punctuation", "ru cyrillic-dice", "ru inline-table", "ru english-in-brackets",
		"ru values-differ", "ru missing-name", "ru slug-not-in-default-locale",
	} {
		if !got[want] {
			t.Errorf("no finding for %q; got %v", want, got)
		}
	}
	if len(coverage) != 1 || coverage[0] != "ru backgrounds            desc 2/2  name 2/4" {
		t.Errorf("coverage = %q", coverage)
	}
}
