# Attribution

The JSON files in this directory are the project's compendium, maintained by
hand: `make pack/check` loads them through the real pack loader, and
`make data/lint/check` holds the prose to its gated checks. They were first
derived from the 5e-bits/5e-database dump of the SRD 5.1 (OGL-declared
upstream) and have been edited in this repository since.

Entries whose provenance names `phb`, `xge` or `tce` carry game mechanics and
names only -- no descriptions. That text is not open-licensed and is not part
of this repository.

The WebP files under spell-icons/ and item-icons/ are AI-generated artwork
owned by the project. Equipment and magic items share item icons through the
`icon` label on each row.

## SRD 5.1

This work includes material taken from the System Reference Document 5.1
("SRD 5.1") by Wizards of the Coast LLC, available at
<https://dnd.wizards.com/resources/systems-reference-document>. The SRD 5.1 is
licensed under the Creative Commons Attribution 4.0 International License,
available at <https://creativecommons.org/licenses/by/4.0/legalcode>.

## Provenance, and an open question

Both the mechanics and the English prose here are currently derived from
[`5e-bits/5e-database`](https://github.com/5e-bits/5e-database) (`src/2014/en`),
whose code is MIT but whose **game material is stated as OGL 1.0a**, not CC-BY-4.0.
`docs/reference_srd_5.1/data/ATTRIBUTION.md` records that, with an explicit
"review before redistributing this JSON outside the project".

The *mechanics* — dice, ranges, bonuses, slot tables — are facts and carry thin
copyright. The **prose** under `i18n/en/` is the exposure. The clean fix is to
re-source those descriptions from
[`gabrielrega/cc-srd5`](https://github.com/gabrielrega/cc-srd5), which is
CC-BY-4.0, keeping only mechanics from `5e-database`. That requires slug-to-heading
matching in the generator and is **tracked but not yet done**.

Not legal advice. Flagged here because the project's own research notes raise it
and easydnd.org is public.
