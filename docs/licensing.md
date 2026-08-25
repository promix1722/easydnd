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
| `data/srd_5.1/` | Generated from `5e-bits/5e-database`, whose game material upstream states is **OGL 1.0a**. Ships in the deploy tarball. |
| `docs/reference_srd_5.1/data/cc-srd5/` | Vendored SRD 5.1 prose, **CC-BY-4.0**. |
| `docs/reference_srd_5.1/data/5e-database-2014-en/` | Vendored source dump, OGL-declared by upstream. |
| `docs/reference_hexsheet/` | A real exported character sheet, kept as a shape reference. |

Dependency licenses are a separate matter again: the Go modules in `go.mod` and
the npm packages in `web/package-lock.json` carry their own terms, and this
repository ships no aggregated `NOTICE` for them.

## SRD 5.1 attribution

This work includes material taken from the System Reference Document 5.1
("SRD 5.1") by Wizards of the Coast LLC, available at
<https://dnd.wizards.com/resources/systems-reference-document>. The SRD 5.1 is
licensed under the Creative Commons Attribution 4.0 International License,
available at <https://creativecommons.org/licenses/by/4.0/legalcode>.

That paragraph exists in several places in this repo. The **canonical** copy is
the `attribution` constant in [`cmd/srdgen/main.go`](../cmd/srdgen/main.go) --
it is the only one CI checks, because `make data/srd/check` regenerates
`data/srd_5.1/` and fails on any difference. The copies in
`docs/reference_srd_5.1/` are quoted from upstream and use curly quotes; this
one follows srdgen. If the wording ever needs to change, change the constant
first and let the generator propagate it.

There is now one more copy, and it is the first that reaches a visitor's
browser: `SRD_ATTRIBUTION` in
[`web/src/features/legal/attribution.ts`](../web/src/features/legal/attribution.ts),
rendered on `/legal`. A browser cannot read a file at the repository root, so
the copy is unavoidable; what is avoidable is its drifting. Two checks in
series, both inside `make verify`, stop that:

```
attribution.ts --(web/src/features/legal/attribution.test.ts)--> data/srd_5.1/ATTRIBUTION.md
                                                                          |
                                                     (make data/srd/check) |
                                                                          v
                                                                cmd/srdgen/main.go
```

The test reads the generated file off disk and compares it with the client's
string, ignoring only the markdown autolink brackets and the hard wrapping --
neither of which is a difference in wording. So the rule is unchanged: change
the Go constant first and let the generator propagate. The client is a leaf that
a test drags along behind it.

## Where the detail lives

| Document | Covers |
| --- | --- |
| [docs/reference_srd_5.1/README.md](reference_srd_5.1/README.md) | The fullest treatment: CC-BY-4.0 vs OGL 1.0a, why every machine-readable SRD is a community conversion, and what to check before redistributing |
| [docs/reference_srd_5.1/data/ATTRIBUTION.md](reference_srd_5.1/data/ATTRIBUTION.md) | Per-dataset provenance for each vendored source |
| [data/srd_5.1/ATTRIBUTION.md](../data/srd_5.1/ATTRIBUTION.md) | Generated; travels with the data it covers, including into the deploy tarball |
| [web/src/features/legal/attribution.ts](../web/src/features/legal/attribution.ts) | The copy the browser shows, on `/legal`. Pinned to the generated notice by `attribution.test.ts` |

One trap worth naming: `docs/reference_srd_5.1/data/cc-srd5/LICENSING.md` is
**upstream's** notice, vendored along with the data, and it names its own author.
It is not this project's license and should not be read as one.

## Known gaps

Recorded rather than quietly carried:

- **The notice is on the public chrome only.** `/legal` now carries the MIT
  notice and the SRD 5.1 attribution in full, reached from a footer in
  `web/src/shell/LandingShell.tsx` -- so the older and larger gap, that nothing
  user-visible on easydnd.org displayed the notice at all, is closed. What
  remains is that the footer is on the *landing* chrome alone: `/`, `/login`,
  `/status` and `/legal`. A signed-in visitor, who is the one actually reading
  SRD-derived material on a character sheet, has no link to it from anywhere.
  This used to be a layout that forbade it: `MobileShell` spent its only
  `AppShell.Footer` slot on the tab bar. That bar became a dropdown in the
  header, so the slot is free and both signed-in shells could now carry a
  footer -- closing this is a decision nobody has made rather than a thing that
  cannot be done. Either a footer in both shells, or an "About" entry beside
  the account icon. Open.
- **The English prose is sourced from the OGL-declared dump.** Both mechanics and
  prose currently come from `5e-bits/5e-database` (`src/2014/en`). The mechanics
  -- dice, ranges, bonuses, slot tables -- are facts and carry thin copyright;
  the prose under `i18n/en/` is the exposure. The clean fix is re-sourcing those
  descriptions from the CC-BY-4.0 `gabrielrega/cc-srd5` and keeping only
  mechanics from `5e-database`, which needs slug-to-heading matching in
  `cmd/srdgen`. Tracked in the generated attribution, not yet done.
- **No OGL 1.0a text is vendored.** `docs/reference_srd_5.1/fetch-srd.sh` pulls
  a license file for `cc-srd5` but none for `5e-database`. If that material
  really is OGL, the license's own notice requirements are unmet.
- **`data/srd_5.1/manifest.json` records `source` but no license field**, so the
  shipped data does not state its own terms in machine-readable form.
