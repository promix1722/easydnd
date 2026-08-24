#!/usr/bin/env node
/**
 * Renders the PWA icon set into public/icons/.
 *
 * The manifest needs raster icons (Chrome will not treat an SVG-only manifest
 * as reliably installable), but a binary blob in git that nobody can regenerate
 * is worse than a script. This rasterises the same d20 mark the favicon draws,
 * using only node:zlib -- no image toolchain, no dependency to keep current.
 *
 * Run with `npm run icons` after changing the mark or the brand colour.
 */
import { deflateSync } from 'node:zlib'
import { mkdirSync, writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const OUT = fileURLToPath(new URL('../public/icons', import.meta.url))

const BRAND = [0x99, 0x05, 0x1d]
const INK = [0xff, 0xf6, 0xe8]
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

mkdirSync(OUT, { recursive: true })

const targets = [
  ['icon-192.png', 192, false],
  ['icon-512.png', 512, false],
  ['icon-maskable-512.png', 512, true],
  ['apple-touch-icon.png', 180, true],
]

for (const [name, size, maskable] of targets) {
  writeFileSync(`${OUT}/${name}`, encodePng(drawIcon(size, maskable), size))
  console.log(`wrote public/icons/${name} (${size}x${size})`)
}
