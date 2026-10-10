# Licensing

Two licenses meet in this repository. The code is the project's own and is MIT.
The D&D material the code reads is **not** the project's to license, and the
distinction is load-bearing rather than pedantic -- easydnd.org is public.

Not legal advice.

## The project's own work -- MIT

Everything written for this project is [MIT](../LICENSE): the Go service, the
browser client, the generator, the deploy scripts and the prose documentation.

## What MIT does not cover

A root `LICENSE` reads as covering the whole tree, so the exceptions have to be
said out loud. These directories hold third-party material carrying its own
terms, and MIT does not apply to them:

| Path | Terms |
| --- | --- |
| `data/pack/srd-5.1/` | The compendium. Derived once from `5e-bits/5e-database`, whose game material upstream states is **OGL 1.0a**, and hand-maintained since. Ships in the deploy tarball. |
| `docs/reference_hexsheet/` | A real exported character sheet, kept as a shape reference. |

The upstream dump is no longer vendored: it was the generator's input, and the
generator is gone. [docs/reference_srd_5.1/README.md](reference_srd_5.1/README.md)
still records where it came from and what the licence question was.

### What the compendium contains, and what it does not

The pack is **SRD 5.1 extended**: every SRD entry, plus the *mechanics and
names* of the other 2014 books -- the spells, subclasses, feats, backgrounds
and the artificer that the SRD leaves out. `provenance.json` tags each row
with its source (`srd-5.1`, `phb`, `xge`, `tce`); the manifest's `sources`
map carries those as bare ids, with no book titles and no translations. A row
tagged with a non-SRD source ships **no prose**: no description, no material
component text, no "at higher levels". Dice, ranges, levels, prerequisites and
a name are the facts a rules engine needs and carry thin copyright; the text
is the exposure, and it is not in this repository.

That text exists in the private `easydnd-2014` repository, as a
descriptions-only overlay pack (see [packs.md](packs.md#prose-overlays)),
and it is never part of a release. A visitor to easydnd.org sees those entries
with a blank description -- by design, not by accident.

It can be installed on the server by hand as a **private** pack
(`deploy/push-private-pack.sh`, see
[packs.md](packs.md#common-and-private-disk-packs)), and is then served only
to the accounts named in `auth.superadmins` and to the groups one of them
shares it with -- never in the default catalogue, never to a guest or an
ordinary account. That is a narrower thing than a public deployment and a
wider one than a private checkout, and whether it is acceptable is the pack
owner's decision, not the code's: `NOTICE.md` in that repository still says
the pack is not to be pointed at a public deployment, and should be brought in
line with whatever is decided.

The one borderline case is named rather than hidden: the 309 ideals, bonds,
flaws and personality traits of the non-SRD backgrounds are stored as `terms`,
and a term's *name* is the sentence itself, because the engine requires every
entity to have a name and the overlay cannot add entities. They ship.

Dependency licenses are a separate matter again: the Go modules in `go.mod` and
the npm packages in `web/package-lock.json` carry their own terms, and this
repository ships no aggregated `NOTICE` for them.

## Pack artifacts

The base `pack-manifest.json` carries the SRD source and attribution. Portable
and directory exports preserve that metadata and the locale bundles;
repackaging does not change the underlying terms described here.
`mechanics.json` is authored project content -- the pools, rules and the
actions open to everybody. One bundle beside it is not: `i18n/en/actions.json`
is SRD 5.1 text, the combat actions from the SRD's rule sections, and travels
under the same attribution as the rest of the data, as do its translations.
The Tactician fixture under `internal/adapter/catalog/file/testdata/` is a test
input and illustrative project content, not shipped data and not a claim that
its subclass appears in the SRD. Custom packs can carry their own
source/attribution; the loader does not determine their publication rights.

## SRD 5.1 attribution

This work includes material taken from the System Reference Document 5.1
("SRD 5.1") by Wizards of the Coast LLC, available at
<https://dnd.wizards.com/resources/systems-reference-document>. The SRD 5.1 is
licensed under the Creative Commons Attribution 4.0 International License,
available at <https://creativecommons.org/licenses/by/4.0/legalcode>.

That paragraph exists in several places in this repo. The **canonical** copy is
the `## SRD 5.1` section of
[`data/pack/srd-5.1/ATTRIBUTION.md`](../data/pack/srd-5.1/ATTRIBUTION.md): it
is the one that travels with the data it covers, into the deploy tarball
included. The `attribution` field of `pack-manifest.json` carries the same
text, so an export of the pack carries it too. The copy in
`docs/reference_srd_5.1/README.md` is quoted from upstream and uses curly
quotes; this one uses straight ones. If the wording ever needs to change,
change the markdown file and the manifest together.

There is one more copy, and it is the first that reaches a visitor's browser:
`SRD_ATTRIBUTION` in
[`web/src/features/legal/attribution.ts`](../web/src/features/legal/attribution.ts),
rendered on `/legal`. A browser cannot read a file at the repository root, so
the copy is unavoidable; what is avoidable is its drifting. One check inside
`make verify` stops that:

```
attribution.ts --(web/src/features/legal/attribution.test.ts)--> data/pack/srd-5.1/ATTRIBUTION.md
```

The test reads the markdown file off disk, takes its `## SRD 5.1` section and
compares it with the client's string, ignoring only the markdown autolink
brackets and the hard wrapping -- neither of which is a difference in wording.
So the rule is: change the file first. The client is a leaf that a test drags
along behind it.

## Where the detail lives

| Document | Covers |
| --- | --- |
| [docs/reference_srd_5.1/README.md](reference_srd_5.1/README.md) | The fullest treatment: CC-BY-4.0 vs OGL 1.0a, why every machine-readable SRD is a community conversion, and what to check before redistributing |
| [data/pack/srd-5.1/ATTRIBUTION.md](../data/pack/srd-5.1/ATTRIBUTION.md) | Canonical; travels with the data it covers, including into the deploy tarball |
| [web/src/features/legal/attribution.ts](../web/src/features/legal/attribution.ts) | The copy the browser shows, on `/legal`. Pinned to the markdown file by `attribution.test.ts` |

## Known gaps

Recorded rather than quietly carried:

- **The notice is on the public chrome only.** `/legal` now carries the MIT
  notice and the SRD 5.1 attribution in full, reached from a footer in
  `web/src/shell/LandingShell.tsx` -- so the older and larger gap, that nothing
  user-visible on easydnd.org displayed the notice at all, is closed. What
  remains is that the footer is on the *landing* chrome alone: `/`, `/login`
  and `/legal`. A signed-in visitor, who is the one actually reading
  SRD-derived material on a character sheet, has no link to it from anywhere.
  This used to be a layout that forbade it: `MobileShell` spent its only
  `AppShell.Footer` slot on the tab bar. That bar became a dropdown in the
  header, so the slot is free and both signed-in shells could now carry a
  footer -- closing this is a decision nobody has made rather than a thing that
  cannot be done. Either a footer in both shells, or an "About" entry beside
  the account icon. Open.
- **The SRD prose was sourced from the OGL-declared dump.** Both the mechanics
  and the English prose of the SRD rows were derived from `5e-bits/5e-database`
  (`src/2014/en`). The mechanics -- dice, ranges, bonuses, slot tables -- are
  facts and carry thin copyright; the prose under `i18n/en/` is the exposure.
  The clean fix is re-sourcing those descriptions from the CC-BY-4.0
  `gabrielrega/cc-srd5`, which with a hand-maintained pack is an edit of the
  bundles, paragraph by paragraph, rather than a generator change. Not done.
- **The images are AI-generated and unmentioned above.** The landing
  photographs and backdrop tile (`web/src/assets/*.webp`), the spell icons
  (`data/pack/srd-5.1/spell-icons/`, one per spell, SRD and non-SRD alike,
  generated through OpenAI's image API) and the item icons are generated
  images. OpenAI's terms assign its output to the customer, so the project
  treats them as its own and they fall under MIT with the rest -- but their
  provenance is recorded here rather than implied, and a jurisdiction that
  denies copyright to generated images would make them public domain rather
  than the project's. Either way nothing restricts shipping them.
- **No OGL 1.0a text is carried.** The dump the SRD rows were derived from is
  OGL-declared by upstream and its licence text was never vendored. If that
  material really is OGL, the license's own notice requirements are unmet.
- **`data/pack/srd-5.1/pack-manifest.json` records `source` and an `attribution`
  text but no license field**, so the shipped data does not state its own terms
  in machine-readable form.

Homebrew JSON export retains the pack's `source` and `attribution`. Importing or
duplicating a pack changes its identity and creates a private draft but preserves
those fields and its localized prose. Group sharing grants members access to
export published content; it does not change the content's license.
