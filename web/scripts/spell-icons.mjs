// Offline PNG -> 128px WebP conversion for `make spell-icons`.
// Prompts and image generation live in the Go service and shared CLI.
//   node scripts/spell-icons.mjs convert <png-dir>

import { mkdirSync, readdirSync, existsSync, renameSync, rmSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..', '..')
// Portable seed pack; the service imports these WebPs into the image repository.
const SPELL_ICONS_DIR = join(ROOT, 'data', 'spell-icons')
const ICON_SIZE = 128


// Publish only a complete conversion, retaining the old image on failure.
async function pngToWebp(pngPath, webpPath) {
  const { default: sharp } = await import('sharp')
  mkdirSync(dirname(webpPath), { recursive: true })
  const tmp = `${webpPath}.tmp-${process.pid}-${Math.random().toString(36).slice(2)}`
  try {
    await sharp(pngPath).resize(ICON_SIZE, ICON_SIZE).webp({ quality: 82 }).toFile(tmp)
    renameSync(tmp, webpPath)
  } finally {
    rmSync(tmp, { force: true })
  }
}

async function convert(pngDir) {
  mkdirSync(SPELL_ICONS_DIR, { recursive: true })
  let converted = 0
  let skipped = 0
  for (const file of readdirSync(pngDir).filter((name) => name.endsWith('.png')).sort()) {
    const dst = join(SPELL_ICONS_DIR, file.replace(/\.png$/, '.webp'))
    if (existsSync(dst)) {
      skipped++
      continue
    }
    await pngToWebp(join(pngDir, file), dst)
    converted++
  }
  console.log(`converted ${converted}, skipped ${skipped} (already in ${SPELL_ICONS_DIR})`)
}

const [mode, target] = process.argv.slice(2)
if (mode === 'convert' && target) await convert(target)
else {
  console.error('usage: spell-icons.mjs convert <png-dir>')
  process.exit(2)
}
