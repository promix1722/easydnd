// Command packlint reads the prose of a pack directory and reports text that
// looks wrong: converter markup that leaked through, punctuation debris,
// tables flattened into a sentence, and -- for every locale that is not the
// pack's default -- what is untranslated, and what disagrees with the source
// on a die or a distance.
//
// It is a reporting tool, not a gate: it exits 0 unless -fail names a check
// that found something. The loader (cmd/pack) owns structural validity; this
// only looks at strings a person will read.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// entry is one slug of an i18n file: name, desc, fields, blocks -- kept loose,
// because the checks only ever want the strings and where they sit.
type entry map[string]any

type finding struct{ check, locale, where, text string }

// pattern is a single-string check. Locale "" runs on every locale.
type pattern struct {
	name, locale string
	re           *regexp.Regexp
	plainOnly    bool
}

var patterns = []pattern{
	{name: "markup-leftover", re: regexp.MustCompile(`\{[@#=]|[{}]`)},
	// A 5etools source tag rendered in place of the thing it sourced:
	// "{@item shovel|phb}" coming out as "a phb".
	{name: "source-code-leak", re: regexp.MustCompile(`\b(phb|xge|tce|dmg|mm|scag|vgm|mtf|erlw)\b`)},
	{name: "stray-punctuation", re: regexp.MustCompile(`:\.|,,|;;|[^.]\.\.([^.]|$)|\( | \)`)},
	{name: "space-before-punctuation", re: regexp.MustCompile(` [,;!?]| \.(\s|$)`)},
	// An empty table cell is "|  |", which is not a typo.
	{name: "double-space", re: regexp.MustCompile(`[^\s|] {2,}[^\s|]`)},
	{name: "html", re: regexp.MustCompile(`</?[a-z][^>]*>|&[a-z]+;|&#\d+;`)},
	// name and fields.* are plain text (docs/packs.md, "Prose formats"); only
	// desc and blocks.* are Markdown.
	{name: "markdown-in-plain-field", re: regexp.MustCompile(`\||\*\*|^#|^- |\n`), plainOnly: true},

	// docs/packs.md and data/translations/README.md: Latin dice, imperial units.
	{name: "cyrillic-dice", locale: "ru", re: regexp.MustCompile(`(^|[^а-яёА-ЯЁ])\d*к\d+`)},
	{name: "metric-units", locale: "ru", re: regexp.MustCompile(`\d\s*(м|метр[а-я]*|км|кг|килограмм[а-я]*)([^а-яё]|$)`)},
	// A scraped table whose cells were joined with spaces: "к6 Афера 1 Я ... 2 Я ...".
	{name: "inline-table", locale: "ru", re: regexp.MustCompile(`[кd]\d+ [А-ЯЁ][^.]{3,40} 1 [А-ЯЁ].* 2 [А-ЯЁ]`)},
	{name: "heading-glued-to-paragraph", locale: "ru", re: regexp.MustCompile(`^[А-ЯЁ]{3,}( [А-ЯЁ]{2,})* [А-ЯЁ][а-яё]`)},
	// dnd.su's "конус холода [cone of cold]" convention.
	{name: "english-in-brackets", locale: "ru", re: regexp.MustCompile(`\[[A-Za-z][A-Za-z’' /-]+\]`)},
	// The glossary's forms are "сложность"/"Сл" and "класс доспеха"/"КД";
	// these are the other spellings the translations have used for them.
	{name: "nonstandard-abbreviation", locale: "ru", re: regexp.MustCompile(`(^|[^A-Za-zА-Яа-яЁё])(DC|AC|HP|СТ|СД|СЛ)([^A-Za-zА-Яа-яЁё]|$)`)},
}

var (
	latinWord = regexp.MustCompile(`[A-Za-z]{3,}`)
	bracketed = regexp.MustCompile(`\[[^\]]*\]`)
	dice      = regexp.MustCompile(`\d*[dк]\d+`)
	distance  = regexp.MustCompile(`(\d+)[ -](?:фут|feet|foot|ft)`)
	number    = regexp.MustCompile(`\d+`)
	separator = regexp.MustCompile(`^\|[-: |]+\|$`)
	word      = regexp.MustCompile(`[\p{L}']+`)
)

func main() {
	in := flag.String("in", "data/srd_5.1", "pack directory")
	only := flag.String("check", "", "print every finding of this check instead of the summary")
	samples := flag.Int("samples", 3, "findings shown per check in the summary")
	fail := flag.String("fail", "", "comma-separated checks that make the exit status 1 when they find anything")
	glossaryPath := flag.String("glossary", "", "flat JSON object of source terms to preferred translations, checked against every translated entry")
	flag.Parse()

	glossary := map[string]string{}
	if *glossaryPath != "" {
		b, err := os.ReadFile(*glossaryPath)
		if err == nil {
			err = json.Unmarshal(b, &glossary)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	findings, coverage, err := lint(*in, glossary)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *only != "" {
		for _, f := range findings {
			if f.check == *only {
				fmt.Printf("%s %s: %s\n", f.locale, f.where, f.text)
			}
		}
	} else {
		report(findings, coverage, *samples)
	}
	for _, f := range findings {
		if *fail != "" && slices.Contains(strings.Split(*fail, ","), f.check) {
			os.Exit(1)
		}
	}
}

func report(findings []finding, coverage []string, samples int) {
	fmt.Println("translated / present in the default locale")
	for _, line := range coverage {
		fmt.Println("  " + line)
	}
	type key struct{ locale, check string }
	groups := map[key][]finding{}
	for _, f := range findings {
		k := key{f.locale, f.check}
		groups[k] = append(groups[k], f)
	}
	keys := make([]key, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].locale != keys[j].locale {
			return keys[i].locale < keys[j].locale
		}
		return keys[i].check < keys[j].check
	})
	for _, k := range keys {
		fmt.Printf("\n%s %s: %d\n", k.locale, k.check, len(groups[k]))
		for _, f := range groups[k][:min(samples, len(groups[k]))] {
			fmt.Printf("  %s: %s\n", f.where, f.text)
		}
	}
}

// lint reads <dir>/i18n/<locale>/*.json. The default locale comes from
// pack-manifest.json and is the source every other locale is compared with.
//
// ponytail: directory packs only. A ZIP or portable-JSON pack needs
// file.LoadPack and a walk over its locale bundles instead of the filesystem.
func lint(dir string, glossary map[string]string) ([]finding, []string, error) {
	var manifest struct {
		DefaultLocale string `json:"defaultLocale"`
	}
	if b, err := os.ReadFile(filepath.Join(dir, "pack-manifest.json")); err == nil {
		if err = json.Unmarshal(b, &manifest); err != nil {
			return nil, nil, fmt.Errorf("pack-manifest.json: %w", err)
		}
	}
	if manifest.DefaultLocale == "" {
		manifest.DefaultLocale = "en"
	}
	locales, err := os.ReadDir(filepath.Join(dir, "i18n"))
	if err != nil {
		return nil, nil, err
	}
	bundles := map[string]map[string]map[string]entry{}
	for _, l := range locales {
		files, err := filepath.Glob(filepath.Join(dir, "i18n", l.Name(), "*.json"))
		if err != nil {
			return nil, nil, err
		}
		bundles[l.Name()] = map[string]map[string]entry{}
		for _, path := range files {
			b, err := os.ReadFile(path)
			if err != nil {
				return nil, nil, err
			}
			var entries map[string]entry
			if err = json.Unmarshal(b, &entries); err != nil {
				return nil, nil, fmt.Errorf("%s: %w", path, err)
			}
			bundles[l.Name()][strings.TrimSuffix(filepath.Base(path), ".json")] = entries
		}
	}
	source, ok := bundles[manifest.DefaultLocale]
	if !ok {
		return nil, nil, fmt.Errorf("no i18n/%s: the default locale has no text", manifest.DefaultLocale)
	}

	var out []finding
	var coverage []string
	for _, locale := range sortedKeys(bundles) {
		for _, collection := range sortedKeys(source) {
			entries := bundles[locale][collection]
			for _, slug := range sortedKeys(entries) {
				out = append(out, lintEntry(locale, collection+"/"+slug, entries[slug])...)
			}
			out = append(out, duplicateNames(locale, collection, entries, source[collection], locale == manifest.DefaultLocale)...)
			if locale == manifest.DefaultLocale {
				continue
			}
			found, line := compare(locale, collection, source[collection], entries, glossary)
			out = append(out, found...)
			coverage = append(coverage, line)
		}
	}
	return out, coverage, nil
}

func lintEntry(locale, where string, e entry) []finding {
	var out []finding
	for _, l := range leaves(e, "") {
		at := where + "/" + l.path
		switch {
		case strings.TrimSpace(l.text) == "":
			out = append(out, finding{"empty-string", locale, at, ""})
			continue
		case strings.TrimSpace(l.text) != l.text:
			out = append(out, finding{"edge-whitespace", locale, at, excerpt(l.text, 0, 0)})
		}
		for _, p := range patterns {
			markdown := strings.HasPrefix(l.path, "desc/") || strings.HasPrefix(l.path, "blocks/")
			if p.locale != "" && p.locale != locale || p.plainOnly && markdown {
				continue
			}
			if m := p.re.FindStringIndex(l.text); m != nil {
				out = append(out, finding{p.name, locale, at, excerpt(l.text, m[0], m[1])})
			}
		}
		// English left inside a Russian sentence. Names are exempt (a proper
		// noun may stay Latin) and so is dnd.su's bracketed original, which
		// has its own check.
		if locale == "ru" && l.path != "name" {
			if words := latinWord.FindAllString(bracketed.ReplaceAllString(l.text, ""), -1); len(words) >= 2 {
				out = append(out, finding{"untranslated-words", locale, at, strings.Join(words[:min(8, len(words))], " ")})
			}
		}
	}
	out = append(out, lintTables(locale, where+"/desc", paragraphs(e["desc"]))...)
	if blocks, ok := e["blocks"].(map[string]any); ok {
		for _, k := range sortedKeys(blocks) {
			out = append(out, lintTables(locale, where+"/blocks/"+k, paragraphs(blocks[k]))...)
		}
	}
	return out
}

// lintTables holds a Markdown field to the one table shape the client can
// render: a row per element, outer pipes on every row, and a separator as the
// second row of each run. Anything else reaches the reader as pipes.
func lintTables(locale, where string, blocks []string) []finding {
	var out []finding
	for i, b := range blocks {
		at := fmt.Sprintf("%s/%d", where, i)
		row := strings.HasPrefix(b, "|")
		switch {
		case !row && strings.Contains(b, " | "):
			out = append(out, finding{"malformed-table", locale, at, "row without outer pipes: " + excerpt(b, 0, 0)})
		case row && !strings.HasSuffix(b, "|"):
			out = append(out, finding{"malformed-table", locale, at, "row not closed by a pipe: " + excerpt(b, 0, 0)})
		case row && (i == 0 || !strings.HasPrefix(blocks[i-1], "|")):
			if i+1 >= len(blocks) || !separator.MatchString(blocks[i+1]) {
				out = append(out, finding{"malformed-table", locale, at, "no |---| separator after the header: " + excerpt(b, 0, 0)})
			}
		}
	}
	return out
}

// glossaryMisses lists the glossary terms the source uses and the translation
// does not. A Russian term counts as present when every word of it begins some
// word of the text once two letters of ending are dropped -- crude, and enough
// to tell "спасбросок" from a translation that never says it.
func glossaryMisses(glossary map[string]string, source, translation string) []string {
	fold := func(s string) []string {
		return word.FindAllString(strings.ReplaceAll(strings.ToLower(s), "ё", "е"), -1)
	}
	src := " " + strings.Join(fold(source), " ") + " "
	have := fold(translation)
	var out []string
	for _, term := range sortedKeys(glossary) {
		if !strings.Contains(src, " "+strings.ToLower(term)+" ") {
			continue
		}
		for _, w := range fold(glossary[term]) {
			// Two letters of ending off a long word, one off a short one:
			// "хиты" has to find "хитов".
			if r := []rune(w); len(r) > 4 {
				w = string(r[:len(r)-2])
			} else if len(r) > 2 {
				w = string(r[:len(r)-1])
			}
			if !slices.ContainsFunc(have, func(h string) bool { return strings.HasPrefix(h, w) }) {
				out = append(out, fmt.Sprintf("%q should read %q", term, glossary[term]))
				break
			}
		}
	}
	return out
}

// compare holds one translated collection against the default locale: what is
// absent, what was copied instead of translated, and what cannot be a
// translation of the same paragraph because its dice or distances differ.
func compare(locale, collection string, source, tr map[string]entry, glossary map[string]string) ([]finding, string) {
	var out []finding
	have, want := map[string]int{}, map[string]int{}
	for _, slug := range sortedKeys(tr) {
		if _, ok := source[slug]; !ok {
			out = append(out, finding{"slug-not-in-default-locale", locale, collection + "/" + slug, ""})
		}
	}
	for _, slug := range sortedKeys(source) {
		src, got := source[slug], tr[slug]
		where := collection + "/" + slug
		name, _ := src["name"].(string)
		for _, field := range sortedKeys(src) {
			want[field]++
			if isEmpty(got[field]) {
				out = append(out, finding{"missing-" + field, locale, where, name})
				continue
			}
			have[field]++
		}
		translated := map[string]string{}
		for _, l := range leaves(got, "") {
			translated[l.path] = l.text
		}
		for _, l := range leaves(src, "") {
			if translated[l.path] == l.text && l.path != "name" && latinWord.MatchString(l.text) {
				out = append(out, finding{"same-as-default-locale", locale, where + "/" + l.path, excerpt(l.text, 0, 0)})
			}
		}
		a, b := paragraphs(src["desc"]), paragraphs(got["desc"])
		if len(a) == 0 || len(b) == 0 {
			continue
		}
		for _, miss := range glossaryMisses(glossary, strings.Join(a, " "), strings.Join(b, " ")) {
			out = append(out, finding{"glossary-term", locale, where, miss})
		}
		if len(a) >= 2*len(b) || len(b) >= 2*len(a) {
			out = append(out, finding{"paragraph-count-differs", locale, where, fmt.Sprintf("%d in the default locale, %d here", len(a), len(b))})
		}
		if d := valueDiff(strings.Join(a, " "), strings.Join(b, " ")); d != "" {
			out = append(out, finding{"values-differ", locale, where, d})
		}
	}
	parts := []string{}
	for _, field := range sortedKeys(want) {
		parts = append(parts, fmt.Sprintf("%s %d/%d", field, have[field], want[field]))
	}
	return out, fmt.Sprintf("%s %-22s %s", locale, collection, strings.Join(parts, "  "))
}

// duplicateNames reports two slugs of one collection that a reader cannot tell
// apart. In a translation that is only a defect when the source names differ
// ("javelin" and "spear" both "копьё"); in the default locale every repeat is
// reported, except in features, where each class repeating "Extra Attack" is
// the data's shape and not a mistake.
func duplicateNames(locale, collection string, entries, source map[string]entry, isSource bool) []finding {
	if isSource && collection == "features" {
		return nil
	}
	byName := map[string][]string{}
	for _, slug := range sortedKeys(entries) {
		if name, _ := entries[slug]["name"].(string); name != "" {
			// "копье" and "копьё" are one word to a reader.
			name = strings.ReplaceAll(strings.ToLower(name), "ё", "е")
			byName[name] = append(byName[name], slug)
		}
	}
	var out []finding
	for _, name := range sortedKeys(byName) {
		slugs := byName[name]
		distinct := map[string]bool{}
		for _, s := range slugs {
			n, _ := source[s]["name"].(string)
			distinct[strings.ToLower(n)] = true
		}
		if len(slugs) > 1 && (isSource || len(distinct) > 1) {
			out = append(out, finding{"duplicate-name", locale, collection + "/" + slugs[0], fmt.Sprintf("%q is also %s", name, strings.Join(slugs[1:], ", "))})
		}
	}
	return out
}

// valueDiff compares the dice and the distances of two descriptions as sets.
// Dice and feet rather than every number, because a harvested translation
// legitimately words levels and counts differently ("starting at 14th level"
// has no number at all on dnd.su) while a die or a range never changes.
func valueDiff(source, translation string) string {
	diceOf := func(s string) map[string]bool {
		out := map[string]bool{}
		for _, d := range dice.FindAllString(s, -1) {
			// "1d8" and "d8" are the same die
			d = strings.ReplaceAll(d, "к", "d")
			if strings.HasPrefix(d, "1d") {
				d = d[1:]
			}
			out[d] = true
		}
		return out
	}
	numbersOf := func(s string) map[string]bool {
		out := map[string]bool{}
		for _, n := range number.FindAllString(s, -1) {
			out[n] = true
		}
		return out
	}
	var diff []string
	a, b := diceOf(source), diceOf(translation)
	for _, k := range sortedKeys(a) {
		if !b[k] {
			diff = append(diff, "-"+k)
		}
	}
	for _, k := range sortedKeys(b) {
		if !a[k] {
			diff = append(diff, "+"+k)
		}
	}
	// A distance only has to survive as a number: "5 feet to 100 feet" is
	// "от 5 до 100 футов", and the unit is said once.
	missing := func(from, in, sign string) {
		have := numbersOf(in)
		seen := map[string]bool{}
		for _, m := range distance.FindAllStringSubmatch(from, -1) {
			if !have[m[1]] && !seen[m[1]] {
				seen[m[1]] = true
				diff = append(diff, sign+m[1]+"ft")
			}
		}
	}
	missing(source, translation, "-")
	missing(translation, source, "+")
	return strings.Join(diff, " ")
}

type leaf struct{ path, text string }

func leaves(v any, path string) []leaf {
	join := func(k string) string {
		if path == "" {
			return k
		}
		return path + "/" + k
	}
	var out []leaf
	switch x := v.(type) {
	case string:
		out = append(out, leaf{path, x})
	case []any:
		for i, c := range x {
			out = append(out, leaves(c, join(fmt.Sprint(i)))...)
		}
	case entry:
		for _, k := range sortedKeys(x) {
			out = append(out, leaves(x[k], join(k))...)
		}
	case map[string]any:
		for _, k := range sortedKeys(x) {
			out = append(out, leaves(x[k], join(k))...)
		}
	}
	return out
}

func paragraphs(v any) []string {
	var out []string
	for _, l := range leaves(v, "") {
		out = append(out, l.text)
	}
	return out
}

func isEmpty(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(x) == ""
	case []any:
		return len(x) == 0
	case map[string]any:
		return len(x) == 0
	}
	return false
}

// excerpt is the match with some context; a zero match is the start of the string.
func excerpt(s string, from, to int) string {
	r := []rune(s)
	a, b := len([]rune(s[:from])), len([]rune(s[:to]))
	return fmt.Sprintf("%q", string(r[max(0, a-40):min(len(r), b+40)]))
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
