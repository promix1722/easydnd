#!/usr/bin/env node
/**
 * Fails when an import crosses a layer boundary the wrong way.
 *
 * The Go side enforces its dependency rule mechanically (`make lint/layers`
 * greps the module graph, and depguard denies the same imports at lint time),
 * on the grounds that a documented convention nobody can run is a convention
 * that rots. This is the frontend's equivalent. It is a grep rather than a
 * lint plugin for the same reason the Go one is: it needs no configuration
 * surface, and its failure message can say what to do instead.
 *
 *   theme -> lib -> ui -> shell -> features -> routes
 *
 * Imports point left. Run with `npm run lint:layers`.
 */
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join, relative, sep } from 'node:path'
import { fileURLToPath } from 'node:url'

const SRC = fileURLToPath(new URL('../src', import.meta.url))

/**
 * Each rule owns a directory under src/ and lists what code there may not
 * import. `deny` entries are matched as prefixes of the import specifier.
 */
const RULES = [
  {
    dir: 'theme',
    deny: ['react', '@mantine/', '@/'],
    why: 'theme/ holds framework-free tokens; the Mantine binding lives in ui/theme.ts',
  },
  {
    dir: 'domain',
    deny: ['react', 'react-dom', 'react-router', '@mantine/', '@/ui', '@/lib', '@/shell', '@/features', '@/routes'],
    why: 'domain/ is pure rules: no UI framework, no transport, no I/O',
  },
  {
    dir: 'lib',
    deny: ['@mantine/', '@/ui', '@/shell', '@/features', '@/routes'],
    why: 'lib/ sits below the UI; it must not reach upward',
  },
  {
    dir: 'ui',
    deny: ['@/shell', '@/features', '@/routes'],
    why: 'ui/ is the design system; it must not know about screens',
  },
  {
    dir: 'shell',
    deny: ['@mantine/', '@/features', '@/routes'],
    why: 'shell/ is layout only; import chrome primitives from @/ui',
  },
  {
    dir: 'features',
    deny: ['@mantine/', '@/shell', '@/routes'],
    why: 'features/ are screens; they compose @/ui and never own the chrome',
  },
  {
    dir: 'routes',
    deny: ['@mantine/'],
    why: 'routes/ wires shell and features together; visuals come from @/ui',
  },
]

/**
 * The one directory allowed to import Mantine. Listed separately from RULES
 * so that a new top-level directory is denied by default rather than silently
 * exempt.
 */
const MANTINE_ALLOWED = 'ui'

const IMPORT_RE = /(?:^|\n)\s*(?:import|export)[\s\S]*?from\s*['"]([^'"]+)['"]|import\(\s*['"]([^'"]+)['"]\s*\)/g

function* walk(dir) {
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry)
    if (statSync(full).isDirectory()) {
      yield* walk(full)
    } else if (/\.(ts|tsx)$/.test(entry)) {
      yield full
    }
  }
}

function importsOf(source) {
  const found = []
  for (const match of source.matchAll(IMPORT_RE)) {
    const specifier = match[1] ?? match[2]
    if (specifier) found.push(specifier)
  }
  return found
}

const violations = []

for (const file of walk(SRC)) {
  const rel = relative(SRC, file)
  const top = rel.split(sep)[0]
  // Tests exercise the layer they live next to and may reach for helpers.
  if (/\.test\.tsx?$/.test(rel) || top === 'test') continue

  const rule = RULES.find((candidate) => candidate.dir === top)
  const specifiers = importsOf(readFileSync(file, 'utf8'))

  for (const specifier of specifiers) {
    if (specifier.startsWith('@mantine/') && top !== MANTINE_ALLOWED) {
      violations.push({
        file: rel,
        specifier,
        why: `only src/${MANTINE_ALLOWED}/ may import Mantine; import from '@/ui' instead (re-export it there if missing)`,
      })
      continue
    }
    if (!rule) continue
    const denied = rule.deny.find((prefix) => specifier === prefix || specifier.startsWith(prefix))
    if (denied) {
      violations.push({ file: rel, specifier, why: rule.why })
    }
  }
}

if (violations.length > 0) {
  console.error('LAYER VIOLATION\n')
  for (const violation of violations) {
    console.error(`  src/${violation.file}`)
    console.error(`    imports '${violation.specifier}'`)
    console.error(`    ${violation.why}\n`)
  }
  process.exit(1)
}

console.log('layers clean')
