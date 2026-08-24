# SRD 5.1 — machine-readable sources

Research notes for `easydnd` (character creation / level-up / battle tracker, **2014 rules**).

Goal: find the D&D 5e SRD 5.1 in a **parsed** form (JSON / Markdown), not the PDF.
Verified 2026-08-23. Everything described here is downloadable with [`fetch-srd.sh`](./fetch-srd.sh),
which has already been run — see [`data/`](./data/).

---

## TL;DR

| Need | Use |
| --- | --- |
| Structured game data to drive the app | **[`5e-bits/5e-database`](https://github.com/5e-bits/5e-database) → `src/2014/en/*.json`** — 25 files, 3.7 MB, vendored at [`data/5e-database-2014-en/`](./data/5e-database-2014-en/) |
| Human-readable rules prose (tooltips, rules text, search) | **[`gabrielrega/cc-srd5`](https://github.com/gabrielrega/cc-srd5) → `cc-srd5.md`** — 1.8 MB, CC-BY-4.0, vendored at [`data/cc-srd5/`](./data/cc-srd5/) |
| Canonical text to check anything against | Official PDF: <https://media.wizards.com/2023/downloads/dnd/SRD_CC_v5.1.pdf> (3.0 MB, CC-BY-4.0) |

There is **no official non-PDF release of SRD 5.1**. Wizards published it only as a PDF
(CC-BY-4.0 since 2023-01-27). Every JSON/Markdown version is a community conversion.

---

## Licensing — read this first

SRD 5.1 exists under **two** licenses, and community repos are split between them:

* **CC-BY-4.0** (Wizards' 2023 re-release) — permissive, no OGL "Section 15" chain, no
  Product Identity carve-out. This is what you want for a web app.
* **OGL 1.0a** (the original 2016 release, and SRD 5.0 before it) — usable, but drags in
  the OGL boilerplate and a Product Identity list you must not reproduce.

The two documents are *nearly* identical in content, so an OGL-sourced dataset is a fine
technical reference — but if you redistribute the data, prefer a CC-BY-sourced one, or
re-derive from the CC-BY PDF.

Required attribution when shipping CC-BY SRD 5.1 content:

> This work includes material taken from the System Reference Document 5.1 ("SRD 5.1") by
> Wizards of the Coast LLC and available at
> https://dnd.wizards.com/resources/systems-reference-document. The SRD 5.1 is licensed
> under the Creative Commons Attribution 4.0 International License available at
> https://creativecommons.org/licenses/by/4.0/legalcode.

Not legal advice.

---

## Candidates evaluated

| Repo | Format | SRD ver. | Stated license | Last push | Verdict |
| --- | --- | --- | --- | --- | --- |
| [5e-bits/5e-database](https://github.com/5e-bits/5e-database) | JSON (25 files) + TS schemas | 5.1 (`src/2014`) | code MIT, data "OGL 1.0a" | 2026-08-17 | **Primary.** Best-normalized data by far |
| [gabrielrega/cc-srd5](https://github.com/gabrielrega/cc-srd5) | Markdown (1 file) | 5.1 | **CC-BY-4.0** | 2026-05-12 | **Primary for prose.** Actively maintained |
| [open5e/open5e-api](https://github.com/open5e/open5e-api) | JSON (Django fixtures) | 5.1 + 5.2 + 3rd party | mixed per-document | 2026-08-23 | Good fallback; SRD lives in `data/v1/wotc-srd/` and `data/v2/wizards-of-the-coast/srd-2014/` |
| [Tabyltop/CC-SRD](https://github.com/Tabyltop/CC-SRD) | JSON / HTML / TXT | 5.1 | **CC-BY-4.0** | 2024-01-21 | Faithful CC-BY dump, but the 13 MB JSON is a *document tree*, not game data. Useful as a clean CC-BY re-derivation base |
| [BTMorton/dnd-5e-srd](https://github.com/BTMorton/dnd-5e-srd) | MD / JSON / YAML | **5.0**, not 5.1 | OGL 1.0a | 2026-06-19 | Skip. Its `LICENSE` says "System Reference Document 5.0" |
| [palikhov/cc-srd5-1](https://github.com/palikhov/cc-srd5-1) | Markdown | 5.1 | CC-BY-4.0 | 2023-02-23 | Stale fork of `gabrielrega/cc-srd5` |
| [vorpalhex/srd_spells](https://github.com/vorpalhex/srd_spells) | JSON (spells only) | 5.1 | unstated | archived 2023 | Skip — archived, and 5e-database covers it |
| [ucffool/OGL-SRD5](https://github.com/ucffool/OGL-SRD5) · [cjrh/OGL-SRD5](https://github.com/cjrh/OGL-SRD5) | Markdown | 5.1 | OGL | 2018 / 2019 | Dead forks of the old `oldmanumby/DND.SRD.Wiki` (now 404) |
| [soryy708/dnd5-srd](https://github.com/soryy708/dnd5-srd) | JSON (npm) | 5.x | MIT | 2019 | Skip — stale |

---

## Primary source: `5e-bits/5e-database`

The dataset behind the public [D&D 5e API](https://www.dnd5eapi.co/) (`/api/2014/...`,
also GraphQL). Content is already split by concern and cross-referenced by `index` slugs
plus `url` back-references — i.e. it is a relational model, not scraped prose. That is
exactly what a character builder needs.

Verified contents of `data/5e-database-2014-en/` (all 25 files parse as JSON arrays):

```
5e-SRD-Ability-Scores.json           6      5e-SRD-Magic-Schools.json        8
5e-SRD-Alignments.json               9      5e-SRD-Monsters.json           334
5e-SRD-Backgrounds.json              1      5e-SRD-Proficiencies.json      117
5e-SRD-Classes.json                 12      5e-SRD-Races.json                9
5e-SRD-Conditions.json              15      5e-SRD-Rule-Sections.json       33
5e-SRD-Damage-Types.json            13      5e-SRD-Rules.json                6
5e-SRD-Equipment-Categories.json    39      5e-SRD-Skills.json              18
5e-SRD-Equipment.json              237      5e-SRD-Spells.json             319
5e-SRD-Feats.json                    1      5e-SRD-Subclasses.json          12
5e-SRD-Features.json               407      5e-SRD-Subraces.json             4
5e-SRD-Languages.json               16      5e-SRD-Traits.json              38
5e-SRD-Levels.json                 290      5e-SRD-Weapon-Properties.json   11
5e-SRD-Magic-Items.json            362
```

The small counts are correct for the SRD, not gaps: the SRD ships exactly **1 background**
(Acolyte), **1 feat** (Grappler), and **1 subclass per class** (12).

Mapping to the three features you want:

* **Character creation** — `Races` + `Subraces` + `Traits`, `Classes`
  (`hit_die`, `proficiency_choices`, `saving_throws`, `starting_equipment_options`,
  `multi_classing`), `Backgrounds`, `Skills`, `Proficiencies`, `Equipment`.
* **Level-up** — `Levels` (290 entries = per-class, per-level: proficiency bonus, features
  gained, spell slots, class-specific counters like rages / sneak-attack dice) plus
  `Features` (407) and `Subclasses`.
* **Battle tracker** — `Monsters` (334 full stat blocks: AC, HP + hit dice, speeds, ability
  scores, saves, skills, immunities, senses, CR, actions / legendary actions), `Conditions`,
  `Damage-Types`, `Spells` (319, with `damage_at_slot_level` / `damage_at_character_level`
  scaling tables), `Rule-Sections`.

`data/5e-database-2014-en/schemas/*.ts` are the upstream **Zod** schemas — one per JSON file,
declaring every field, its type, and whether it is optional. They are TypeScript and this is a
Go project, so nothing here runs; treat them as the machine-checked spec for each file's shape
and transcribe them into Go structs. See [Why the `.ts` files](#why-the-ts-files) below.
`schemas/_common.ts` is the shared `src/schemas/common.ts` that every other schema imports.

Caveats:
* README states the underlying material is **OGL 1.0a**, not CC-BY. Fine for internal use;
  revisit before redistributing the raw JSON.
* Data is 2014-rules; upstream `src/2024/` holds SRD 5.2 for when you add 2024 rules.
* Descriptions are plain strings — no markup — so rules text renders flat.

## Primary source for prose: `gabrielrega/cc-srd5`

Single 1.8 MB Markdown file (`cc-srd5.md`, 46k lines) — the SRD 5.1 PDF converted, typo-fixed,
and reorganized with anchors (`{#chapter-races}`), under **CC-BY-4.0**. Also vendored:
`changes-50-to-51.md` (diff between the OGL 5.0 and CC 5.1 text) and the license files.

One caveat that matters for a game tool: the maintainer **renames residual Product Identity
terms** (e.g. "eyestalker" for the *deck of illusions* entry, "maze demon", generic deity
names). Great for a publisher, mildly off-canon for a tracker — so use this for rules prose
and keep `5e-database` as the source of truth for names and statistics, or diff against the
official PDF where a name matters.

### Why the `.ts` files

Each `schemas/<file>.ts` is a [Zod](https://zod.dev) declaration of the matching
`5e-SRD-<file>.json`. Example, `5e-SRD-Spells.ts`:

```ts
export const SpellSchema = z.strictObject({
  index: z.string(),
  name: z.string(),
  level: z.number(),
  concentration: z.boolean(),
  higher_level: z.array(z.string()).optional(),   // <- not always present
  damage: SpellDamageSchema.optional(),           // <- not always present
  school: APIReferenceSchema,
  classes: z.array(APIReferenceSchema),
  ...
});
```

They are **not a dependency** — the JSON stands alone and the app never loads these. Their value
is that they answer the one question you cannot answer by eyeballing JSON: *which fields are
optional*. `.optional()` on 319 spells means `higher_level` is absent on some of them, so the Go
field must be a pointer or slice with `omitempty` rather than a plain value that silently reads
as zero. Guessing that from a few sample records is exactly how a level-up calculator ends up
quietly treating "no cantrip scaling" as "0 damage".

Practical uses, in order of payoff:

1. **Write Go structs from them.** `z.string()` → `string`, `z.number()` → `int`/`float64`,
   `z.array(X)` → `[]X`, `z.strictObject` → a struct, `.optional()` → pointer + `,omitempty`.
   `APIReferenceSchema` (in `_common.ts`) is the `{index, name, url}` cross-reference stub that
   appears everywhere — write it once and reuse it.
2. **Spot the irregular shapes early.** `ChoiceSchema` / `OptionSchema` in `_common.ts` are
   recursive discriminated unions (`option_type: "reference" | "choice" | "string" |
   "ability_bonus" | ...`) driving `proficiency_choices` and `starting_equipment_options` —
   i.e. the character-creation picker. That is the hardest part of the dataset to model, and the
   schema tells you the full variant list up front instead of you discovering variant #9 in
   production.
3. **Detect upstream drift.** `z.strictObject` rejects unknown keys, so if a future
   `fetch-srd.sh` run pulls JSON with a new field, diffing the schemas shows you what changed.

If you would rather not hand-translate: feed a JSON file to `quicktype` or `gojsonstruct` to
generate Go structs, then use these schemas to correct the optionality, which generators get
wrong when a field happens to be present in every sampled record.

---

## Also worth knowing

* **Live API instead of vendoring** — <https://www.dnd5eapi.co/api/2014/spells/fireball>
  works today (verified). Fine for prototyping; vendor the JSON for production so you are
  not coupled to someone's uptime.
* **Open5e** (<https://api.open5e.com>) — broader (3rd-party OGL/CC content too: Tome of
  Beasts, Level Up A5E). `data/v1/wotc-srd/Document.json` labels the SRD content OGL;
  `data/v2/wizards-of-the-coast/` splits `srd-2014` / `srd-2024`. Reach for it if you later
  want non-SRD content.
* **SRD 5.2 (2024 rules)** — <https://www.dndbeyond.com/srd>, CC-BY-4.0, and already mirrored
  in `5e-bits/5e-database` `src/2024/` and Open5e `srd-2024/`. Not needed now, but the same
  pipeline covers it.
* **What the SRD does *not* include** — most subclasses, most feats, most backgrounds, most
  races, and any named settings/monsters. A "complete" character builder is not possible from
  SRD alone; plan for user-supplied homebrew content.

## Refreshing

```bash
bash docs/reference_srd_5.1/fetch-srd.sh
```

Sources are pinned to each repo's default branch, so re-running picks up upstream fixes.
