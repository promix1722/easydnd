/**
 * The SRD 5.1 notice, as the browser client says it.
 *
 * This is a *copy*. The canonical wording is the `## SRD 5.1` section of
 * `data/pack/srd-5.1/ATTRIBUTION.md`, the file that travels with the data it
 * covers. A second copy of a licence notice that could silently drift from
 * the first is precisely the failure `docs/licensing.md` warns about, so
 * `attribution.test.ts` reads that file off disk and fails when these two
 * part company. Change the markdown first; this follows.
 *
 * Straight quotes around "SRD 5.1", following that file. The copy under
 * `docs/reference_srd_5.1/` is quoted from upstream and uses curly ones; it
 * is not this project's wording and must not be pasted in here.
 *
 * The two URLs are pulled out so `LegalScreen` can render them as links rather
 * than as bare text a reader has to retype -- the licence asks for the notice,
 * not for it to be inconvenient.
 */

export const SRD_URL = 'https://dnd.wizards.com/resources/systems-reference-document'

export const CC_BY_URL = 'https://creativecommons.org/licenses/by/4.0/legalcode'

/**
 * The notice as one paragraph, with the URLs left in place. Rendered in pieces
 * by the screen; kept whole here so the drift check has one string to compare.
 */
export const SRD_ATTRIBUTION =
  'This work includes material taken from the System Reference Document 5.1 ' +
  `("SRD 5.1") by Wizards of the Coast LLC, available at ${SRD_URL}. ` +
  'The SRD 5.1 is licensed under the Creative Commons Attribution 4.0 ' +
  `International License, available at ${CC_BY_URL}.`

/** Where this project's own code stands, as distinct from the data above. */
export const PROJECT_COPYRIGHT = 'Copyright (c) 2026 The easydnd project'

export const REPO_URL = 'https://github.com/promix1722/easydnd'

export const LICENSE_URL = `${REPO_URL}/blob/main/LICENSE`
