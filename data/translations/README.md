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
