/// <reference types="node" />
//
// The reference above is why `types` in tsconfig.app.json can stay a statement
// about application code: `src/` is browser code and should not be able to
// spell `process` or `node:fs`. This one file needs to read the repository, so
// it asks for node's types where it uses them rather than switching them on
// for all of `src/`.

import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { SRD_ATTRIBUTION } from './attribution'

/**
 * The client's copy of the SRD 5.1 notice must not drift from the generated
 * one.
 *
 * `docs/licensing.md` names the `attribution` constant in `cmd/srdgen/main.go`
 * as canonical, on the grounds that it is the only copy CI checks -- `make
 * data/srd/check` regenerates `data/srd_5.1/` and fails on any difference.
 * That pins the generated file to the Go constant. This pins the browser
 * client to the generated file, so the three move together or `make verify`
 * fails. Without it, `attribution.ts` would be a fourth copy of a licence
 * notice with nothing watching it, which is the exact failure that document
 * warns about for the copies it does track.
 *
 * Reaching outside `web/` is something only a test may do, and
 * `scripts/check-layers.mjs` permits it by skipping `*.test.*`. A Vite `?raw`
 * import was the obvious alternative and does not work: it is refused by the
 * dev server's `fs.allow`, and the fix for that -- opening the repository root
 * to the dev server, which nginx exposes in this project's dev setup -- would
 * put `config.dev.yaml` one directory from a browser. Reading the file in a
 * test costs nothing and ships nothing.
 *
 * Resolved from `process.cwd()` rather than from `import.meta.url`: under the
 * jsdom environment vitest rewrites that to an `http:` URL. Vitest sets the
 * working directory to the one its config lives in, so this is `web/` however
 * the suite was invoked -- and jsdom is kept rather than switched to node for
 * this file, because `test/setup.ts`'s shared teardown reaches for `window`.
 */
const ATTRIBUTION_MD = resolve(process.cwd(), '../data/srd_5.1/ATTRIBUTION.md')

/**
 * The paragraph under `## SRD 5.1`, as prose. The generated file is markdown:
 * its URLs are autolinks in angle brackets and its lines are hard-wrapped,
 * neither of which is a difference in wording.
 */
function generatedNotice(): string {
  const source = readFileSync(ATTRIBUTION_MD, 'utf8')
  const section = /\n## SRD 5\.1\n([\s\S]*?)(?:\n## |$)/.exec(source)
  if (!section?.[1]) throw new Error(`no "## SRD 5.1" section in ${ATTRIBUTION_MD}`)

  return section[1]
    .replace(/<(https?:[^>]+)>/g, '$1')
    .replace(/\s+/g, ' ')
    .trim()
}

describe('SRD_ATTRIBUTION', () => {
  it('matches the notice cmd/srdgen generates', () => {
    expect(SRD_ATTRIBUTION).toBe(generatedNotice())
  })
})
