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
// Usage:
//
//	srdgen [-in docs/reference_srd_5.1/data/5e-database-2014-en] [-out data/srd_5.1]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

const (
	defaultIn  = "docs/reference_srd_5.1/data/5e-database-2014-en"
	defaultOut = "data/srd_5.1"
)

// maxWarnings bounds how much unconverted data is tolerated before the run
// fails. Zero would be ideal, but a hard zero turns a harmless upstream
// addition into a broken build; a small allowance with a printed list keeps
// the failure informative. Silent truncation is what this guards against.
const maxWarnings = 0

func main() {
	in := flag.String("in", defaultIn, "vendored 5e-database directory")
	out := flag.String("out", defaultOut, "output data directory")
	flag.Parse()

	g := newGenerator(*in, *out)
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
	inDir  string
	outDir string

	// prose accumulates the English bundle for each mechanics file, keyed by
	// that file's name so the two stay aligned by construction.
	prose map[string]file.Bundle

	// terms collects prose that belongs to no single entry: the suggested
	// ideals and personality traits that appear inside choice options.
	terms file.Bundle

	counts   map[string]int
	warnings []string
}

func newGenerator(in, out string) *generator {
	return &generator{
		inDir:  in,
		outDir: out,
		prose:  make(map[string]file.Bundle),
		terms:  make(file.Bundle),
		counts: make(map[string]int),
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
	for _, dir := range []string{
		g.outDir,
		filepath.Join(g.outDir, file.LocaleDir, rules.LocaleEN.String()),
		filepath.Join(g.outDir, file.LocaleDir, rules.LocaleRU.String()),
	} {
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
		g.classes, g.classLevels, g.subclasses, g.features,
		g.backgrounds, g.feats,
		g.equipment, g.magicItems, g.spells,
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}

	// Prose last, so a mechanics failure aborts before half-writing bundles.
	for _, name := range file.MechanicsFiles() {
		bundle := g.prose[name]
		if bundle == nil {
			bundle = file.Bundle{}
		}
		if err := g.write(filepath.Join(file.LocaleDir, rules.LocaleEN.String(), name), bundle); err != nil {
			return err
		}
	}
	if err := g.write(filepath.Join(file.LocaleDir, rules.LocaleEN.String(), file.FileTerms), g.terms); err != nil {
		return err
	}
	if err := g.writeRussianScaffold(); err != nil {
		return err
	}

	if err := g.writeRaw("ATTRIBUTION.md", attribution); err != nil {
		return err
	}

	return g.writeManifest()
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

// writeRussianScaffold seeds the ru locale.
//
// It is deliberately tiny: a handful of hand-checked translations rather than
// a machine-translated dump. Its job is to make the per-key fallback a path
// that real data exercises, so a regression in the merge shows up in the test
// suite rather than in production the day someone switches language.
func (g *generator) writeRussianScaffold() error {
	dir := filepath.Join(file.LocaleDir, rules.LocaleRU.String())

	abilities := file.Bundle{
		"str": {Name: "СИЛ", Fields: map[string]string{file.ProseFullName: "Сила"}},
		"dex": {Name: "ЛОВ", Fields: map[string]string{file.ProseFullName: "Ловкость"}},
		"con": {Name: "ТЕЛ", Fields: map[string]string{file.ProseFullName: "Телосложение"}},
		"int": {Name: "ИНТ", Fields: map[string]string{file.ProseFullName: "Интеллект"}},
		"wis": {Name: "МДР", Fields: map[string]string{file.ProseFullName: "Мудрость"}},
		"cha": {Name: "ХАР", Fields: map[string]string{file.ProseFullName: "Харизма"}},
	}
	if err := g.write(filepath.Join(dir, file.FileAbilities), abilities); err != nil {
		return err
	}

	// Only the names are translated here. Every description falls back to
	// English key by key, which is exactly the partial state a growing locale
	// lives in.
	races := file.Bundle{
		"dwarf":      {Name: "Дварф"},
		"elf":        {Name: "Эльф"},
		"halfling":   {Name: "Полурослик"},
		"human":      {Name: "Человек"},
		"dragonborn": {Name: "Драконорождённый"},
		"gnome":      {Name: "Гном"},
		"half-elf":   {Name: "Полуэльф"},
		"half-orc":   {Name: "Полуорк"},
		"tiefling":   {Name: "Тифлинг"},
	}
	if err := g.write(filepath.Join(dir, file.FileRaces), races); err != nil {
		return err
	}

	terms := file.Bundle{
		"castingTime.action":       {Name: "1 действие"},
		"castingTime.bonus-action": {Name: "1 бонусное действие"},
		"castingTime.reaction":     {Name: "1 реакция"},
		"range.self":               {Name: "На себя"},
		"range.touch":              {Name: "Касание"},
		"duration.instantaneous":   {Name: "Мгновенная"},
	}
	return g.write(filepath.Join(dir, file.FileTerms), terms)
}

func (g *generator) writeManifest() error {
	locales := []string{rules.LocaleEN.String(), rules.LocaleRU.String()}
	return g.write(file.FileManifest, file.Manifest{
		Ruleset: "2014",
		Source:  "5e-bits/5e-database src/2014/en, vendored at " + defaultIn,
		Locales: locales,
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
