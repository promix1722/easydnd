#!/usr/bin/env node
/**
 * Fails when the message catalogue and the code disagree.
 *
 * Every user-facing word in this client lives in `web/locales/*.json` and is
 * reached by key, so that translating the app means editing a data file and
 * never opening a component. What that buys in accessibility to a translator
 * it costs in drift: nothing in the language stops a key being renamed in one
 * place and not the other.
 *
 * Half of that is already covered. `useT` is typed from `en.json`, so a key
 * the catalogue does not define will not compile. The compiler cannot see the
 * other half -- a key the catalogue defines that nothing renders any more --
 * and it has nothing at all to say about Russian.
 *
 * So this script is deliberately not a duplicate of the type: it is the two
 * directions the type cannot check, plus a coverage number worth glancing at.
 *
 *   - a key used in src/ that en.json does not define      -> fail
 *   - a key en.json defines that src/ never uses           -> fail
 *   - a key ru.json defines that en.json does not          -> fail
 *   - how much of en.json ru.json has translated           -> report
 *
 * Russian being incomplete is **never** a failure. A partial locale is the
 * normal state of a growing one -- the whole design falls back per key so that
 * it works -- and a build that went red as English grew would make adding a
 * caption feel like breaking the translation.
 *
 * Run with `npm run check:messages`.
 */
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join, relative, sep } from 'node:path'
import { fileURLToPath } from 'node:url'

const SRC = fileURLToPath(new URL('../src', import.meta.url))
const LOCALES = fileURLToPath(new URL('../locales', import.meta.url))

/**
 * i18next stores a plural as several keys and reads it as one: the catalogue
 * holds `foo_one` and `foo_other`, and the call is `t('foo', { count })`.
 * Both sides of every comparison below are reduced to the base key, or a
 * plural would read as one missing key and two unused ones.
 */
const PLURAL_SUFFIX = /_(zero|one|two|few|many|other)$/

const base = (key) => key.replace(PLURAL_SUFFIX, '')

/** `t('some.key')` and `t('some.key', { … })`. Single or double quoted. */
const USE_RE = /\bt\(\s*['"]([^'"]+)['"]/g

/**
 * Any quoted string at all.
 *
 * The two questions this script asks want different evidence, and running both
 * off `t(` would answer one of them wrongly. A key is *missing* when something
 * calls `t` with it -- that is a call, and `USE_RE` sees calls. A key is
 * *unused* when the string appears nowhere in src/ at all, which is a weaker
 * test on purpose: `ui/sections.ts` holds `'section.characters'` in a table and
 * the navbar, the tab bar and the breadcrumb each translate it where they draw
 * it, so no `t('section.characters')` is ever written down. Demanding one would
 * make a table of keys impossible to keep, which is the pattern this codebase
 * already prefers -- see STAGE_LABELS and SECTIONS.
 *
 * The looser test costs nothing, because the strict direction is already
 * covered by something better than a grep: `useT` is typed from en.json, so a
 * key that is not in the catalogue does not compile.
 */
const MENTION_RE = /['"]([A-Za-z][\w.]*)['"]/g

/**
 * Comments, so that a doc comment showing how to call `t` is not read as a
 * call to it. `src/lib/i18n/useT.ts` documents itself with three examples and
 * would otherwise report three missing keys, which is exactly the kind of
 * false alarm that teaches people to stop reading a check's output.
 *
 * Block comments wholesale; line comments only where the line is nothing but a
 * comment. Stripping `//` mid-line would eat the rest of any line holding a
 * URL, and a `t(` call after one on the same line would vanish silently --
 * which is a worse failure than the one being prevented.
 */
function stripComments(source) {
  return source.replace(/\/\*[\s\S]*?\*\//g, '').replace(/^\s*\/\/.*$/gm, '')
}

/**
 * Every source file, tests excluded.
 *
 * `check-layers.mjs` skips them too, and here for a sharper reason: a test may
 * build a fixture catalogue of its own to exercise the merge -- `i18n.test.tsx`
 * does exactly that -- and those keys are not the app's. Counting them would
 * demand `en.json` define words no screen ever shows.
 */
function* walk(dir) {
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry)
    if (statSync(full).isDirectory()) yield* walk(full)
    else if (/\.(ts|tsx)$/.test(entry) && !/\.test\.tsx?$/.test(entry)) yield full
  }
}

function load(name) {
  const path = join(LOCALES, `${name}.json`)
  try {
    return JSON.parse(readFileSync(path, 'utf8'))
  } catch (cause) {
    console.error(`cannot read locales/${name}.json: ${cause.message}`)
    process.exit(1)
  }
}

const en = load('en')
const ru = load('ru')

// Where each key is called, so a failure can name a file rather than a key.
const usedAt = new Map()
// Every key-shaped literal anywhere in src/, for the unused check.
const mentioned = new Set()
for (const file of walk(SRC)) {
  const rel = `src/${relative(SRC, file).split(sep).join('/')}`
  const source = stripComments(readFileSync(file, 'utf8'))
  for (const match of source.matchAll(USE_RE)) {
    const key = base(match[1])
    if (!usedAt.has(key)) usedAt.set(key, new Set())
    usedAt.get(key).add(rel)
  }
  for (const match of source.matchAll(MENTION_RE)) mentioned.add(base(match[1]))
}

const defined = new Set(Object.keys(en).map(base))
const problems = []

for (const [key, files] of usedAt) {
  if (!defined.has(key)) {
    problems.push(`  '${key}' is used but locales/en.json does not define it\n      ${[...files].join('\n      ')}`)
  }
}

/**
 * Keys nothing in src/ spells out, because they are composed at runtime.
 *
 * `error.*` is the whole of it. The server names a reason -- "group.name.required"
 * -- and `lib/api/errors.ts` prefixes it and looks it up, so the literal
 * "error.group.name.required" appears nowhere for a grep to find. The set is
 * not open-ended: it is exactly the reasons `internal/` can emit, and that
 * pairing is checked from the Go side by `errors.ts`'s own `known()` falling
 * back rather than rendering a slug.
 *
 * Deliberately a prefix rather than an exemption list, and deliberately only
 * this one: every other key in the catalogue is written down at its call site
 * and stays checkable.
 */
const COMPOSED = ['error.']

const composed = (key) => COMPOSED.some((prefix) => key.startsWith(prefix))

for (const key of defined) {
  if (!mentioned.has(key) && !composed(key)) {
    problems.push(`  '${key}' is in locales/en.json but nothing uses it`)
  }
}

for (const key of Object.keys(ru)) {
  if (!defined.has(base(key))) {
    problems.push(`  '${key}' is in locales/ru.json but not in locales/en.json`)
  }
}

if (problems.length > 0) {
  console.error('MESSAGE CATALOGUE IS OUT OF SYNC\n')
  console.error(problems.sort().join('\n'))
  console.error('')
  console.error('  Every caption lives in web/locales/. Add the key to en.json when you')
  console.error('  add the string, and delete it when the last caller goes -- a catalogue')
  console.error('  full of keys nothing renders is a catalogue nobody can translate.')
  console.error('')
  process.exit(1)
}

const translated = [...defined].filter((key) =>
  Object.keys(ru).some((each) => base(each) === key),
).length
const percent = defined.size === 0 ? 100 : Math.round((translated / defined.size) * 100)

console.log(`messages in sync: ${defined.size} keys`)
console.log(`ru: ${translated}/${defined.size} translated (${percent}%) -- the rest falls back to English`)
