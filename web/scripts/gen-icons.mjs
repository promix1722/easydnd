#!/usr/bin/env node
/**
 * Renders the favicon and the PWA icon set from the palette.
 *
 * The manifest needs raster icons (Chrome will not treat an SVG-only manifest
 * as reliably installable), but a binary blob in git that nobody can regenerate
 * is worse than a script. This rasterises the same d20 mark the favicon draws,
 * using only node:zlib -- no image toolchain, no dependency to keep current.
 *
 * **It reads the palette straight out of the TypeScript.** That is not a trick:
 * `.nvmrc` is 24, Node has stripped types without a flag since 22.18, and
 * `tsconfig.app.json` already sets `erasableSyntaxOnly`, so every file under
 * `src/` is *already* constrained to exactly the syntax the stripper accepts.
 * The alternative -- a `.js` palette beside a hand-written `.d.ts` -- would be
 * two files to keep in step, and `check-layers.mjs` only walks `.ts`/`.tsx`, so
 * `theme/` would have gained a file nothing checked. Parsing the TypeScript with
 * a regex was the other option, and a gate that fails when somebody reformats a
 * file is a gate that gets switched off.
 *
 * Run `make web/icons` after changing the mark or `PALETTE_NAME`.
 *
 * Usage:
 *   node scripts/gen-icons.mjs              write into public/
 *   node scripts/gen-icons.mjs --out DIR    write into DIR instead
 *   node scripts/gen-icons.mjs --check      compare against what is committed
 */
import { deflateSync, inflateSync } from 'node:zlib'
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

import { PALETTE } from '../src/theme/tokens.ts'

const PUBLIC = fileURLToPath(new URL('../public', import.meta.url))

/** `#99051d` -> `[0x99, 0x05, 0x1d]`. */
function rgb(hex) {
  return [1, 3, 5].map((at) => Number.parseInt(hex.slice(at, at + 2), 16))
}

const BRAND = rgb(PALETTE.brand)
const INK = rgb(PALETTE.ink)
const SS = 4 // supersampling factor, for antialiasing

/** Vertices of a point-up hexagon, as fractions of the canvas. */
function hexagon(cx, cy, r) {
  return Array.from({ length: 6 }, (_, i) => {
    const angle = (Math.PI / 3) * i - Math.PI / 2
    return [cx + r * Math.cos(angle), cy + r * Math.sin(angle)]
  })
}

function distanceToSegment(x, y, [ax, ay], [bx, by]) {
  const dx = bx - ax
  const dy = by - ay
  const lengthSq = dx * dx + dy * dy
  const t = lengthSq === 0 ? 0 : Math.max(0, Math.min(1, ((x - ax) * dx + (y - ay) * dy) / lengthSq))
  return Math.hypot(x - (ax + t * dx), y - (ay + t * dy))
}

function nearAnySegment(x, y, segments, halfWidth) {
  return segments.some(([a, b]) => distanceToSegment(x, y, a, b) <= halfWidth)
}

/**
 * @param size    output edge length in px
 * @param maskable full-bleed background (a maskable icon is cropped to a circle
 *                 by the launcher, so its art must sit inside the safe zone)
 */
function drawIcon(size, maskable) {
  const n = size * SS
  const pixels = Buffer.alloc(n * n * 4)

  const cornerRadius = maskable ? 0 : n * 0.22
  // Maskable icons lose up to 20% on every edge, so the mark shrinks to fit.
  const markRadius = n * (maskable ? 0.26 : 0.34)
  const cx = n / 2
  const cy = n / 2

  const outer = hexagon(cx, cy, markRadius)
  // Face-on, a d20's silhouette is a hexagon with the top face showing as a
  // smaller triangle in the middle, joined to alternating corners by three
  // struts. Without those struts it reads as a plain hexagon.
  const inner = [0, 2, 4].map((i) => {
    const angle = (Math.PI / 3) * i - Math.PI / 2
    return [cx + markRadius * 0.52 * Math.cos(angle), cy + markRadius * 0.52 * Math.sin(angle)]
  })
  const segments = [
    ...outer.map((point, i) => [point, outer[(i + 1) % 6]]),
    ...inner.map((point, i) => [point, inner[(i + 1) % 3]]),
    ...[0, 2, 4].map((i) => [outer[i], inner[i / 2]]),
  ]
  const stroke = n * 0.022

  for (let y = 0; y < n; y++) {
    for (let x = 0; x < n; x++) {
      const px = x + 0.5
      const py = y + 0.5

      let colour = null
      if (insideRoundedRect(px, py, n, cornerRadius)) colour = BRAND
      if (colour && nearAnySegment(px, py, segments, stroke)) colour = INK

      const offset = (y * n + x) * 4
      if (colour) {
        pixels[offset] = colour[0]
        pixels[offset + 1] = colour[1]
        pixels[offset + 2] = colour[2]
        pixels[offset + 3] = 255
      }
    }
  }

  return downsample(pixels, n, size)
}

function insideRoundedRect(x, y, n, radius) {
  if (radius === 0) return true
  const dx = Math.max(radius - x, x - (n - radius), 0)
  const dy = Math.max(radius - y, y - (n - radius), 0)
  return dx * dx + dy * dy <= radius * radius
}

/** Box-filters the supersampled buffer down to the target size. */
function downsample(pixels, n, size) {
  const out = Buffer.alloc(size * size * 4)
  for (let y = 0; y < size; y++) {
    for (let x = 0; x < size; x++) {
      let r = 0
      let g = 0
      let b = 0
      let a = 0
      for (let sy = 0; sy < SS; sy++) {
        for (let sx = 0; sx < SS; sx++) {
          const offset = ((y * SS + sy) * n + (x * SS + sx)) * 4
          const alpha = pixels[offset + 3]
          r += pixels[offset] * alpha
          g += pixels[offset + 1] * alpha
          b += pixels[offset + 2] * alpha
          a += alpha
        }
      }
      const offset = (y * size + x) * 4
      // Premultiplied average, so edge pixels do not darken toward black.
      out[offset] = a === 0 ? 0 : Math.round(r / a)
      out[offset + 1] = a === 0 ? 0 : Math.round(g / a)
      out[offset + 2] = a === 0 ? 0 : Math.round(b / a)
      out[offset + 3] = Math.round(a / (SS * SS))
    }
  }
  return out
}

function crc32(buffer) {
  let crc = 0xffffffff
  for (const byte of buffer) {
    crc ^= byte
    for (let i = 0; i < 8; i++) crc = crc & 1 ? (crc >>> 1) ^ 0xedb88320 : crc >>> 1
  }
  return (crc ^ 0xffffffff) >>> 0
}

function chunk(type, data) {
  const length = Buffer.alloc(4)
  length.writeUInt32BE(data.length)
  const body = Buffer.concat([Buffer.from(type, 'ascii'), data])
  const crc = Buffer.alloc(4)
  crc.writeUInt32BE(crc32(body))
  return Buffer.concat([length, body, crc])
}

function encodePng(rgba, size) {
  const ihdr = Buffer.alloc(13)
  ihdr.writeUInt32BE(size, 0)
  ihdr.writeUInt32BE(size, 4)
  ihdr[8] = 8 // bit depth
  ihdr[9] = 6 // colour type: RGBA
  // Each scanline is prefixed with filter type 0 (none).
  const raw = Buffer.alloc(size * (size * 4 + 1))
  for (let y = 0; y < size; y++) {
    raw[y * (size * 4 + 1)] = 0
    rgba.copy(raw, y * (size * 4 + 1) + 1, y * size * 4, (y + 1) * size * 4)
  }
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk('IHDR', ihdr),
    chunk('IDAT', deflateSync(raw, { level: 9 })),
    chunk('IEND', Buffer.alloc(0)),
  ])
}

/**
 * The favicon, in the same two colours.
 *
 * Hand-authored geometry -- the d20 the mark has always been -- with only the
 * two colours substituted, so this stays a drawing rather than becoming a
 * second rasteriser that has to agree with the first.
 */
function favicon() {
  return `<!-- Generated by \`make web/icons\` from src/theme/palettes.ts. Do not edit. -->
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" width="64" height="64">
  <rect width="64" height="64" rx="14" fill="${PALETTE.brand}"/>
  <g fill="none" stroke="${PALETTE.ink}" stroke-width="3" stroke-linejoin="round" stroke-linecap="round">
    <path d="M32 10 51.05 21 51.05 43 32 54 12.95 43 12.95 21Z"/>
    <path d="M32 20.56 41.91 37.72 22.09 37.72Z"/>
    <path d="M32 10 32 20.56M51.05 43 41.91 37.72M12.95 43 22.09 37.72"/>
  </g>
</svg>
`
}

/**
 * The pixels back out of a PNG this script wrote.
 *
 * `--check` compares decoded pixels rather than file bytes, and the reason is
 * specific: `deflateSync` is deterministic for a given zlib but is not promised
 * to be stable *across Node versions*, so a byte diff would be a gate that goes
 * red on a laptop whose Node differs from CI's -- failing for a reason that has
 * nothing to do with the icons. Decoding is trivial here because this encoder
 * only ever writes filter type 0, so each scanline is its leading zero byte
 * followed by raw RGBA.
 */
function decodePng(file, size) {
  let at = 8 // past the signature
  const parts = []
  while (at < file.length) {
    const length = file.readUInt32BE(at)
    const type = file.toString('ascii', at + 4, at + 8)
    if (type === 'IDAT') parts.push(file.subarray(at + 8, at + 8 + length))
    at += 12 + length
  }
  const raw = inflateSync(Buffer.concat(parts))
  const stride = size * 4
  const out = Buffer.alloc(size * stride)
  for (let y = 0; y < size; y++) {
    raw.copy(out, y * stride, y * (stride + 1) + 1, (y + 1) * (stride + 1))
  }
  return out
}

const args = process.argv.slice(2)
const checking = args.includes('--check')
const outFlag = args.indexOf('--out')
const out = outFlag === -1 ? PUBLIC : args[outFlag + 1]

const targets = [
  ['icon-192.png', 192, false],
  ['icon-512.png', 512, false],
  ['icon-maskable-512.png', 512, true],
  ['apple-touch-icon.png', 180, true],
]

if (checking) {
  const stale = []

  const committed = readFileSync(`${PUBLIC}/favicon.svg`, 'utf8')
  if (committed !== favicon()) stale.push('favicon.svg')

  for (const [name, size, maskable] of targets) {
    const wanted = drawIcon(size, maskable)
    let found
    try {
      found = decodePng(readFileSync(`${PUBLIC}/icons/${name}`), size)
    } catch {
      stale.push(`icons/${name} (unreadable)`)
      continue
    }
    if (!found.equals(wanted)) stale.push(`icons/${name}`)
  }

  if (stale.length > 0) {
    for (const name of stale) console.error(`  differs: ${name}`)
    process.exit(1)
  }
  console.log(`icons current (palette: ${PALETTE.name})`)
} else {
  mkdirSync(`${out}/icons`, { recursive: true })
  writeFileSync(`${out}/favicon.svg`, favicon())
  console.log(`wrote favicon.svg (${PALETTE.name})`)
  for (const [name, size, maskable] of targets) {
    writeFileSync(`${out}/icons/${name}`, encodePng(drawIcon(size, maskable), size))
    console.log(`wrote icons/${name} (${size}x${size})`)
  }
}
