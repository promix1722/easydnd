# Translating the compendium

The compendium's prose lives **in the pack**, beside the mechanics it
describes: `data/pack/srd-5.1/i18n/<locale>/`, one file per collection. You edit
those files directly. There is no generator in between and nothing that
reverts a hand edit; what guards the result is the loader (`make pack/check`)
and the prose lint (`make data/lint/check`), both run by `make verify`.

This directory holds what a translator needs that is *not* a translation:

- `ru.glossary.json` -- the locked D&D vocabulary the LLM workbench is held to,
  so "saving throw" is `спасбросок` in every file and not three different things.
- `ru.sources.json` -- which model, at which reasoning effort, against which
  references, produced the Russian prose. Any of the three changing changes
  the output, so all three are recorded.

## Adding a language

Make a directory named after its language tag and put a file in it. That is the
whole procedure -- no code changes, no list to add yourself to, nothing in the
manifest: the loader reads whatever `i18n/` holds.

```
mkdir data/pack/srd-5.1/i18n/de
```

The name has to parse as a language tag or the load fails, and it has to be one
`rules.SupportedLocales()` lists (`internal/domain/rules/locale.go`) for
anything to ask for it -- `rus/` is a valid tag that nothing will ever
request, so check the spelling. `en/` is the source text, not a translation of
anything.

## What a file looks like

One file per collection, named exactly as in `data/pack/srd-5.1/i18n/en/`. Keys
are the slugs from the English bundle beside it.

```json
{
  "acid-arrow": {
    "name": "Кислотная стрела",
    "desc": ["Мерцающая зелёная стрела устремляется к цели..."],
    "fields": { "material": "Порошок из листьев ревеня и желудок гадюки." },
    "blocks": {
      "higherLevel": ["Когда вы накладываете это заклинание, используя ячейку..."]
    }
  }
}
```

**Every field is optional, and so is every entry.** A file with three spells in
it is a valid file. A spell with only a `name` is a valid entry. Anything you
leave out is shown in English -- per key, not per file, so a translated name sits
happily above an untranslated description. That is the normal state of a
language somebody is still working through, and it is the case the loader was
built for (`internal/adapter/catalog/file/locale.go`).

So there is never a reason to copy English text in just to fill a gap. Leave it
out and it falls back on its own.

Entries that have a name and no description are not gaps to fill: the
non-SRD rows (those `provenance.json` tags `phb`, `xge` or `tce`) ship their
mechanics and names only, by design -- see
[docs/licensing.md](../../docs/licensing.md). Their text comes from the private
`easydnd-2014` pack, in every language it carries.

## What the checks will tell you

- **A slug that is not in the English bundle fails the load**, naming the
  collection and the slug. That is almost always a typo -- `"dwarrf"` -- and
  catching it here is the difference between a five-second fix and a word
  nobody ever sees.
- A directory that is not a known language tag fails the same way.
- `make data/lint` reads the bundles the way a player would and reports what
  the loader cannot know is wrong: English left inside a sentence, two entries
  with the same name, a `DC` that should be `Сл`, a table row without its
  pipes, a doubled space, whitespace at the edge of a paragraph, a glossary
  term the translation never uses. `make verify` runs the subset of those
  checks that is at zero (`data/lint/check`). See
  `docs/packs.md#linting-the-prose`.

Russian is complete, and the gate keeps it so: `missing-name`, `missing-desc`,
`missing-fields` and `missing-blocks` are in the gated list, so an English leaf
with no Russian counterpart fails `make verify` rather than quietly putting
English prose on a Russian sheet. Nothing checks how much any *other* locale
has translated; a locale is allowed to be one line.

Nothing tidies a hand edit for you either. A trailing space, a doubled full
stop, an empty paragraph -- the lint names them and you fix them; there is no
normaliser that would silently rewrite the file behind you.

## Updating the Russian translation

The development workbench preserves every populated leaf already in the output,
uses the checked-in glossary for D&D terminology, and checkpoints each accepted
API response beside the output. Rerunning after a failure only requests the
leaves that are still missing.

```sh
make translate/ru                                  # the spells, model and effort pinned
make translate/ru TRANSLATE_FLAGS=-dry-run         # counts only: no key, no spend
make translate/ru TRANSLATE_REASONING=high         # compare settings
```

It writes straight into `data/pack/srd-5.1/i18n/ru/spells.json`.

**Filling gaps and rerolling are different commands, and the difference is one
flag.** `-existing` treats every populated leaf as done, which is what you want
when finishing a half-translated file — and is a trap when you mean to redo the
prose, because `-existing` points at the output and every leaf in it already
counts as finished. The run then translates nothing and exits successfully.
`-preserve name` is the reroll: keep the hand-checked names, re-request every
description. `make translate/ru` passes it.

Check the dry run before spending anything. It prints how many leaves are
already translated, and for a reroll that number should be the count of names
alone. If it is close to the total, `-preserve` is not doing what you think and
the real run would be a no-op.

Review the prose, then `make verify`. The final write is atomic: an incomplete
run leaves the old translation in place and its progress in
`<output>.checkpoint.json`, so a failed run costs only the leaves it had not
reached. Update `ru.sources.json` when the model, the reasoning effort or the
references change.

## How numbers and units are written

The bullet below is about the *structured* values -- a spell's range field, its
casting time -- which never pass through here at all. Prose is different: a
description says "a 20-foot-radius sphere" in the middle of a sentence, and that
measurement is text like any other. Three rules, and all three are enforced
rather than hoped for.

- **Dice keep Latin notation: `8d6`, not `8к6`.** This reverses what the
  workbench used to ask for. Latin is what players read at the table, and it
  has a second benefit: with `к` gone, every remaining digit in a sentence is a
  real quantity, which is what makes the numeric check below cheap and precise.
- **Imperial measurements stay imperial, with Russian unit words**: `30 футов`,
  `радиусом 20 футов`, never metres or kilograms. The client renders the
  structured range as `футов` (`spell.range.feet`), so prose that converted to
  metres put `36 метров` in a description directly beneath `120 футов` in the
  facts panel of the same spell. It did this in 39 places.
- **Numbers survive exactly.** `valuesMatch` in `cmd/llm/translate.go` compares
  the numbers and dice in the source against the translation and rejects a leaf
  that differs, which aborts the run rather than writing it. The lint's
  `values-differ` check asserts the same thing over the committed files, and it
  is gated, because the bundles are hand-editable and the workbench never sees
  a hand edit.

  It compares the *set*, not a tally. Counting repeats was the obvious rule and
  it was wrong: English states an area as "5 feet square" and Russian states it
  as `5 на 5 футов`, so the idiom duplicates the number by construction. A
  changed, invented or missing number still changes the set; a dropped repeat
  no longer trips it, which is the narrower risk.

  Both sides strip thousands separators first, because the two languages write
  them differently -- `5,000gp` against `5 000 зм`.

Neither check catches **dropped prose** -- text with no digits in it can vanish
without tripping anything, which is exactly what one model setting did to the
clauses naming other spells. Read the diff.

## What is not translated here

- **Mechanics.** Dice, ranges, bonuses and slot tables live in
  `data/pack/srd-5.1/*.json` and are the same in every language. Even the strings
  that look like prose -- "1 action", "90 feet" -- are stored structured and
  rendered per locale by the client, so a Russian sheet never reads "90 feet".
- **The interface.** Buttons, headings and error messages are in
  `web/locales/*.json`. Different file, same idea. See
  [docs/web.md](../../docs/web.md#localization).
- **The licence notice.** `data/pack/srd-5.1/ATTRIBUTION.md` is the canonical
  SRD 5.1 attribution, and a translated attribution is a different attribution.
  See [docs/licensing.md](../../docs/licensing.md).
