# Translating the compendium

This is the **hand-edited** half of the SRD data. Everything here is safe to
change; nothing under `data/srd_5.1/` is.

```
data/translations/ru/spells.json     <- you edit this
        |  cmd/srdgen, via `make data/srd`
        v
data/srd_5.1/i18n/ru/spells.json     <- generated; `make verify` reverts hand-edits
```

The split exists because `data/srd_5.1/` is generated from the vendored dump and
`make data/srd/check` regenerates it and fails on any difference. Before this
directory there was nowhere to type a translation that survived a build.

## Adding a language

Make a directory named after its language tag and put a file in it. That is the
whole procedure -- no code changes, no list to add yourself to.

```
mkdir data/translations/de
```

The tag has to be one `rules.SupportedLocales()` knows
(`internal/domain/rules/locale.go`); `srdgen` warns and fails on a directory it
does not recognise, so a typo like `rus/` cannot quietly go unread. There is no
`en/`: English is the source text, not a translation of anything.

## What a file looks like

One file per collection, named exactly as in `data/srd_5.1/i18n/en/`. Keys are
the slugs from the English bundle beside it.

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

## What the checks will tell you

`make data/srd` regenerates; `make verify` runs the same thing and compares.

- **A slug that is not in the English bundle fails the build**, naming the file
  and the slug. That is almost always a typo -- `"dwarrf"` -- and catching it
  here is the difference between a five-second fix and a word nobody ever sees.
- A directory that is not a known language tag fails the same way.
- Nothing checks how *much* is translated. A locale is allowed to be one line.

Russian is the exception to that last general rule: it is a complete locale,
and `cmd/srdgen` has a test that requires every non-empty English leaf to have a
non-empty Russian counterpart. A new English entry therefore cannot quietly
fall back to English on a Russian sheet.

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

**Filling gaps and rerolling are different commands, and the difference is one
flag.** `-existing` treats every populated leaf as done, which is what you want
when finishing a half-translated file — and is a trap when you mean to redo the
prose, because `-existing` points at the output and every leaf in it already
counts as finished. The run then translates nothing and exits successfully.
`-preserve name` is the reroll: keep the hand-checked names, re-request every
description. `make translate/ru` passes it.

Check the dry run before spending anything. It prints how many leaves are
already translated, and for a reroll that number should be the count of names
alone — 319 of 1654 for the spells. If it is close to the total, `-preserve` is
not doing what you think and the real run would be a no-op.

Review the prose, then `make verify` (the target already runs `make data/srd`).
The final write is atomic: an incomplete run leaves the old translation in place
and its progress in `<output>.checkpoint.json`, so a failed run costs only the
leaves it had not reached. `ru.sources.json` records the pinned model, the
reasoning effort and the terminology references — all three, because any of them
changing changes the output.

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
  that differs, which aborts the run rather than writing it. A test in
  `cmd/srdgen` asserts the same thing over the committed files, because this
  directory is hand-editable and the workbench never sees a hand edit.

  It compares the *set*, not a tally. Counting repeats was the obvious rule and
  it was wrong: English states an area as "5 feet square" and Russian states it
  as `5 на 5 футов`, so the idiom duplicates the number by construction. A
  changed, invented or missing number still changes the set; a dropped repeat
  no longer trips it, which is the narrower risk.

  Both sides strip thousands separators first, because the two languages write
  them differently -- `5,000gp` against `5 000 зм`.

Which files these two checks cover is `proseChecked` in
`cmd/srdgen/translations_test.go`. Adding a file there is a promise that its
Russian has been brought up to the same standard, so a new collection starts
outside the list and joins it once somebody has done that work.

Neither check catches **dropped prose** -- text with no digits in it can vanish
without tripping anything, which is exactly what one model setting did to the
clauses naming other spells. Read the diff.

## What is not translated here

- **Mechanics.** Dice, ranges, bonuses and slot tables live in
  `data/srd_5.1/*.json` and are the same in every language. Even the strings
  that look like prose -- "1 action", "90 feet" -- are stored structured and
  rendered per locale by the client, so a Russian sheet never reads "90 feet".
- **The interface.** Buttons, headings and error messages are in
  `web/locales/*.json`. Different file, same idea. See
  [docs/web.md](../../docs/web.md#localization).
- **The licence notice.** `data/srd_5.1/ATTRIBUTION.md` is generated from
  `cmd/srdgen`, and a translated attribution is a different attribution. See
  [docs/licensing.md](../../docs/licensing.md).
