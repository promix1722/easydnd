// Convert approved PNG artwork to the pack's fixed 128px, lossless WebP format.
// Run from any directory: node web/scripts/item-icons.mjs
import assert from 'node:assert/strict'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import sharp from 'sharp'

const root = join(dirname(fileURLToPath(import.meta.url)), '..', '..')
const mapping = JSON.parse(await readFile(join(root, 'data/rules/2014/item-icons.json'), 'utf8'))
const labels = [...new Set(Object.values(mapping).flatMap(Object.values))].sort()
const samples = new Set(['sword', 'shield', 'leather-armor', 'backpack', 'potion', 'rope'])
const output = join(root, 'data/pack/srd-5.1/item-icons')
await mkdir(output, { recursive: true })
for (const label of labels) {
  assert.match(label, /^[a-z0-9][a-z0-9-]{0,119}$/)
  const source = join(root, 'output/imagegen', samples.has(label) ? 'item-samples' : 'item-icons', `${label}.png`)
  const input = sharp(await readFile(source))
  const meta = await input.metadata()
  assert.equal(meta.width, meta.height, `${label}: expected square artwork`)
  assert.equal(meta.hasAlpha, true, `${label}: expected transparent artwork`)
  const data = await input.resize(128, 128, { kernel: 'nearest' }).webp({ lossless: true }).toBuffer()
  const { info, data: pixels } = await sharp(data).ensureAlpha().raw().toBuffer({ resolveWithObject: true })
  assert.equal(info.width, 128)
  assert.equal(info.height, 128)
  assert(pixels.some((value, at) => at % 4 === 3 && value === 0), `${label}: missing transparency`)
  assert(pixels.some((value, at) => at % 4 === 3 && value === 255), `${label}: missing opaque artwork`)
  // The converter owns only these generated outputs; it never rewrites source PNGs.
  await writeFile(join(output, `${label}.webp`), data)
}
console.log(`Converted and checked ${labels.length} item icons (128×128, transparent WebP).`)
