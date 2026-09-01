// Spell icons: the two ends of the pipeline that `cmd/llm images` sits in
// the middle of. `make spell-icons` chains all three; see docs/web.md.
//
//   node scripts/spell-icons.mjs prompts <out.json>   build the batch prompts
//   node scripts/spell-icons.mjs convert <png-dir>    1024px PNG -> 128px webp
//
// Prompts are built from the SRD mechanics (slug, school) and the English
// names, one flat {slug: prompt} object -- the shape `llm images -in` reads.
// Convert skips a webp that already exists, mirroring the generator's own
// resume: rerolling one icon means deleting its webp and its cached PNG.
//
// sharp is a devDependency used only here, never by the build.

import { mkdirSync, readFileSync, readdirSync, writeFileSync, existsSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..', '..')
const OUT_DIR = join(ROOT, 'web', 'public', 'spells')
const ICON_SIZE = 128

// One color voice per school, so the set reads as a system rather than 319
// separate commissions. The words are prompt vocabulary, not UI tokens.
const SCHOOL_PALETTES = {
  abjuration: 'protective deep blue and silver',
  conjuration: 'violet and gold',
  divination: 'pale cyan and white',
  enchantment: 'rose pink and soft purple',
  evocation: 'fiery red and orange',
  illusion: 'iridescent teal and lavender',
  necromancy: 'sickly green and bone white',
  transmutation: 'amber and copper',
}

function prompt(name, school) {
  const palette = SCHOOL_PALETTES[school] ?? 'muted arcane'
  return (
    `Flat vector emblem icon representing the D&D spell "${name}". ` +
    `Simple bold geometric shapes, centered single subject, ${palette} color palette, ` +
    `subtle shading, no text, no letters, no border, transparent background. ` +
    `Consistent minimal game-icon style.`
  )
}

function buildPrompts(outPath) {
  const spells = JSON.parse(readFileSync(join(ROOT, 'data', 'srd_5.1', 'spells.json'), 'utf8'))
  const prose = JSON.parse(
    readFileSync(join(ROOT, 'data', 'srd_5.1', 'i18n', 'en', 'spells.json'), 'utf8'),
  )
  const out = {}
  for (const spell of spells) {
    out[spell.slug] = prompt(prose[spell.slug]?.name ?? spell.slug, spell.school)
  }
  mkdirSync(dirname(outPath), { recursive: true })
  writeFileSync(outPath, JSON.stringify(out, null, 1))
  console.log(`wrote ${Object.keys(out).length} prompts to ${outPath}`)
}

async function convert(pngDir) {
  const { default: sharp } = await import('sharp')
  mkdirSync(OUT_DIR, { recursive: true })
  let converted = 0
  let skipped = 0
  for (const file of readdirSync(pngDir).filter((name) => name.endsWith('.png')).sort()) {
    const dst = join(OUT_DIR, file.replace(/\.png$/, '.webp'))
    if (existsSync(dst)) {
      skipped++
      continue
    }
    await sharp(join(pngDir, file)).resize(ICON_SIZE, ICON_SIZE).webp({ quality: 82 }).toFile(dst)
    converted++
  }
  console.log(`converted ${converted}, skipped ${skipped} (already in ${OUT_DIR})`)
}

const [mode, target] = process.argv.slice(2)
if (mode === 'prompts' && target) buildPrompts(target)
else if (mode === 'convert' && target) await convert(target)
else {
  console.error('usage: spell-icons.mjs prompts <out.json> | convert <png-dir>')
  process.exit(2)
}
