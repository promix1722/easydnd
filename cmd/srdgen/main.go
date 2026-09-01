// Command srdgen converts the vendored 5e-database SRD dump into the data
// files this project reads.
//
// It exists so that refreshing the upstream data is a re-runnable command with
// a reviewable diff, rather than a one-off edit nobody can reproduce. Both the
// tool and its output are committed; `make data/srd/check` fails the build if
// they disagree.
//
// The conversion does three things the raw dump does not:
//
//   - splits language-neutral mechanics from translatable prose, writing the
//     first to the directory root and the second to i18n/<locale>/
//   - normalises rule strings ("1 action", "90 feet", "Up to 1 minute") into
//     structured values, so they can be compared and re-rendered per locale
//   - types every cross-reference as "kind:slug", using the upstream URL to
//     tell a skill from a proficiency of the same name
//
// Translations are an *input*, not an edit of the output. `data/srd_5.1/` is
// generated and `make data/srd/check` reverts anything typed into it, so a
// translator works in `data/translations/<locale>/<collection>.json` and this
// command merges that over the English bundle. See writeLocales.
//
// Usage:
//
//	srdgen [-in docs/reference_srd_5.1/data/5e-database-2014-en]
//	       [-out data/srd_5.1]
//	       [-translations data/translations]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

const (
	defaultIn           = "docs/reference_srd_5.1/data/5e-database-2014-en"
	defaultOut          = "data/srd_5.1"
	defaultTranslations = "data/translations"
)

// maxWarnings bounds how much unconverted data is tolerated before the run
// fails. Zero would be ideal, but a hard zero turns a harmless upstream
// addition into a broken build; a small allowance with a printed list keeps
// the failure informative. Silent truncation is what this guards against.
const maxWarnings = 0

func main() {
	in := flag.String("in", defaultIn, "vendored 5e-database directory")
	out := flag.String("out", defaultOut, "output data directory")
	translations := flag.String("translations", defaultTranslations,
		"hand-edited translation directory, one subdirectory per locale")
	flag.Parse()

	g := newGenerator(*in, *out, *translations)
	if err := g.run(); err != nil {
		fmt.Fprintf(os.Stderr, "srdgen: %v\n", err)
		os.Exit(1)
	}

	for _, w := range g.warnings {
		fmt.Fprintf(os.Stderr, "srdgen: warning: %s\n", w)
	}
	if len(g.warnings) > maxWarnings {
		fmt.Fprintf(os.Stderr, "srdgen: %d warnings exceeds the allowance of %d\n", len(g.warnings), maxWarnings)
		os.Exit(1)
	}

	total := 0
	for _, n := range g.counts {
		total += n
	}
	fmt.Printf("srdgen: wrote %d entries across %d files to %s\n", total, len(g.counts), *out)
}

// generator holds one conversion run.
type generator struct {
	inDir    string
	outDir   string
	transDir string

	// prose accumulates the English bundle for each mechanics file, keyed by
	// that file's name so the two stay aligned by construction.
	prose map[string]file.Bundle

	// terms collects prose that belongs to no single entry: the suggested
	// ideals and personality traits that appear inside choice options.
	terms file.Bundle

	counts   map[string]int
	warnings []string
}

func newGenerator(in, out, translations string) *generator {
	return &generator{
		inDir:    in,
		outDir:   out,
		transDir: translations,
		prose:    make(map[string]file.Bundle),
		terms:    make(file.Bundle),
		counts:   make(map[string]int),
	}
}

func (g *generator) warnf(format string, a ...any) {
	g.warnings = append(g.warnings, fmt.Sprintf(format, a...))
}

// put records an entry's prose under the bundle for one mechanics file.
func (g *generator) put(dataFile, slug string, p file.Prose) {
	if g.prose[dataFile] == nil {
		g.prose[dataFile] = make(file.Bundle)
	}
	g.prose[dataFile][slug] = p
}

// text records a standalone prose string under a key.
func (g *generator) text(key, s string) {
	if key == "" {
		return
	}
	g.terms[key] = file.Prose{Name: s}
}

// run performs the conversion.
func (g *generator) run() error {
	locales, err := g.locales()
	if err != nil {
		return err
	}

	dirs := []string{g.outDir}
	for _, locale := range locales {
		dirs = append(dirs, filepath.Join(g.outDir, file.LocaleDir, locale.String()))
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
	}

	// Each step reads one upstream file and writes one mechanics file,
	// accumulating prose as it goes.
	steps := []func() error{
		g.abilities, g.skills, g.alignments, g.languages,
		g.conditions, g.damageTypes, g.magicSchools, g.weaponProperties,
		g.proficiencies, g.equipmentCategories,
		g.races, g.subraces, g.traits,
		g.classes, g.classLevels, g.subclasses, g.backgrounds, g.features,
		g.feats,
		g.equipment, g.magicItems, g.spells,
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}

	// Prose last, so a mechanics failure aborts before half-writing bundles.
	if err := g.writeLocales(locales); err != nil {
		return err
	}

	if err := g.writeRaw("ATTRIBUTION.md", attribution); err != nil {
		return err
	}

	return g.writeManifest(locales)
}

// attribution ships with the data rather than beside it, so the notice travels
// with the files it covers -- including into the deploy tarball.
const attribution = `# Attribution

The files in this directory are **generated** by ` + "`" + `cmd/srdgen` + "`" + ` from the vendored
dump at ` + "`" + `docs/reference_srd_5.1/data/` + "`" + `. Do not edit them by hand: ` + "`" + `make verify` + "`" + `
regenerates them into a temporary directory and fails on any difference.

## SRD 5.1

This work includes material taken from the System Reference Document 5.1
("SRD 5.1") by Wizards of the Coast LLC, available at
<https://dnd.wizards.com/resources/systems-reference-document>. The SRD 5.1 is
licensed under the Creative Commons Attribution 4.0 International License,
available at <https://creativecommons.org/licenses/by/4.0/legalcode>.

## Provenance, and an open question

Both the mechanics and the English prose here are currently derived from
[` + "`" + `5e-bits/5e-database` + "`" + `](https://github.com/5e-bits/5e-database) (` + "`" + `src/2014/en` + "`" + `),
whose code is MIT but whose **game material is stated as OGL 1.0a**, not CC-BY-4.0.
` + "`" + `docs/reference_srd_5.1/data/ATTRIBUTION.md` + "`" + ` records that, with an explicit
"review before redistributing this JSON outside the project".

The *mechanics* — dice, ranges, bonuses, slot tables — are facts and carry thin
copyright. The **prose** under ` + "`" + `i18n/en/` + "`" + ` is the exposure. The clean fix is to
re-source those descriptions from
[` + "`" + `gabrielrega/cc-srd5` + "`" + `](https://github.com/gabrielrega/cc-srd5), which is
CC-BY-4.0, keeping only mechanics from ` + "`" + `5e-database` + "`" + `. That requires slug-to-heading
matching in the generator and is **tracked but not yet done**.

Not legal advice. Flagged here because the project's own research notes raise it
and easydnd.org is public.
`

/*
locales reports which languages to emit: English, plus every subdirectory of
the translations tree that names a locale this build knows.

Adding a language is therefore adding a directory. Nothing here is a list of
languages -- `rules.SupportedLocales()` says which tags are legal, the
filesystem says which ones anybody has started, and neither is a code change
when a translator begins.

An unknown directory is a warning rather than a failure, and the distinction
matters: a typo like `data/translations/rus/` would otherwise be a directory
somebody fills in for a week before noticing it is never read.
*/
func (g *generator) locales() ([]rules.Locale, error) {
	out := []rules.Locale{rules.DefaultLocale}

	entries, err := os.ReadDir(g.transDir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, fmt.Errorf("reading %s: %w", g.transDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		locale := rules.Locale(entry.Name())
		switch {
		case locale == rules.DefaultLocale:
			// English is generated, never translated: the dump is already in
			// it, so a directory here would be an edit of the source text
			// wearing a translator's clothes.
			g.warnf("%s: %s is the source language and is not translated",
				g.transDir, locale)
		case !locale.IsSupported():
			g.warnf("%s: %q is not a locale this build knows; see rules.SupportedLocales",
				g.transDir, entry.Name())
		default:
			out = append(out, locale)
		}
	}
	slices.SortFunc(out, func(a, b rules.Locale) int { return strings.Compare(a.String(), b.String()) })
	return out, nil
}

/*
writeLocales emits the English bundles, and each translation beside them.

A translation is written as it was given -- validated, sorted and canonically
formatted, but **not merged with English**. That is deliberate, and it took a
wrong turn to find: merging here writes a full copy of all 960 KB of English
prose into every locale directory, which grows the tree by about a megabyte per
language and makes a translation diff unreadable -- every line of it English the
translator never touched.

It is also unnecessary. `internal/adapter/catalog/file` already merges per key
at load time -- `resolve` over the preferred locale and `rules.DefaultLocale` --
and that path is what makes a partial locale work at all. A locale directory
holding only what somebody has actually translated is the honest shape of the
thing, and it is the shape the loader was built for.

So what this command contributes is not the merge. It is that
`data/srd_5.1/i18n/<locale>/` stays *generated*: hand-edits there are reverted
by `make data/srd/check`, and the file a translator edits is checked against the
English bundle on the way through.
*/
func (g *generator) writeLocales(locales []rules.Locale) error {
	english := make(map[string]file.Bundle, len(g.prose)+1)
	for _, name := range file.MechanicsFiles() {
		bundle := g.prose[name]
		if bundle == nil {
			bundle = file.Bundle{}
		}
		english[name] = bundle
	}
	english[file.FileTerms] = g.terms

	for _, name := range file.ProseFiles() {
		path := filepath.Join(file.LocaleDir, rules.DefaultLocale.String(), name)
		if err := g.write(path, english[name]); err != nil {
			return err
		}
	}

	for _, locale := range locales {
		if locale == rules.DefaultLocale {
			continue
		}
		translation, err := g.translationFor(locale, english)
		if err != nil {
			return err
		}
		for _, name := range file.ProseFiles() {
			bundle, ok := translation[name]
			if !ok {
				continue
			}
			path := filepath.Join(file.LocaleDir, locale.String(), name)
			if err := g.write(path, bundle); err != nil {
				return err
			}
		}
	}
	return nil
}

// translationFor reads one locale's hand-edited files, checking as it goes.
//
// A slug the English bundle does not define is a warning, and `maxWarnings` is
// zero -- so a typo in a translation fails the build with the slug printed,
// rather than being silently dropped into a file nothing reads. That check is
// most of why the translations live in their own tree: it can only be made
// against the generated English, which is the thing being translated.
func (g *generator) translationFor(
	locale rules.Locale, english map[string]file.Bundle,
) (map[string]file.Bundle, error) {
	out := make(map[string]file.Bundle, len(english))
	if locale == rules.DefaultLocale {
		return out, nil
	}

	dir := filepath.Join(g.transDir, locale.String())
	for _, name := range file.ProseFiles() {
		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}

		var bundle file.Bundle
		if err := json.Unmarshal(raw, &bundle); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
		for slug := range bundle {
			if _, ok := english[name][slug]; !ok {
				g.warnf("%s: %q is not a slug in %s", path, slug, name)
			}
		}
		out[name] = bundle
	}
	return out, nil
}

func (g *generator) writeManifest(locales []rules.Locale) error {
	tags := make([]string, 0, len(locales))
	for _, locale := range locales {
		tags = append(tags, locale.String())
	}
	return g.write(file.FileManifest, file.Manifest{
		Ruleset: "2014",
		Source:  "5e-bits/5e-database src/2014/en, vendored at " + defaultIn,
		Locales: tags,
		Counts:  g.counts,
	})
}

// read decodes one upstream file.
func read[T any](g *generator, name string) ([]T, error) {
	path := filepath.Join(g.inDir, name)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var out []T
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decoding %s: %w", path, err)
	}
	return out, nil
}

// write emits one output file with deterministic formatting, so that
// regenerating unchanged input produces an empty diff.
func (g *generator) write(name string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding %s: %w", name, err)
	}
	raw = append(raw, '\n')
	path := filepath.Join(g.outDir, name)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// writeRaw emits a non-JSON file verbatim.
func (g *generator) writeRaw(name, content string) error {
	path := filepath.Join(g.outDir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// emit writes a mechanics file, sorting by slug and recording the count.
func emit[T any](g *generator, name string, items []T, slug func(T) string) error {
	slices.SortFunc(items, func(a, b T) int {
		switch as, bs := slug(a), slug(b); {
		case as < bs:
			return -1
		case as > bs:
			return 1
		default:
			return 0
		}
	})
	g.counts[name] = len(items)
	return g.write(name, items)
}
