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
 * `domain/` sits beside `theme/` at the bottom: pure rules, no framework, no
 * transport. It holds no user-facing words either -- those live in
 * web/locales/*.json -- so the functions there return message keys and let the
 * caller translate.
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
    deny: ['react', '@mantine/', '@tabler/', '@/'],
    why: 'theme/ holds framework-free tokens; the Mantine binding lives in ui/theme.ts',
  },
  {
    dir: 'domain',
    deny: ['react', 'react-dom', 'react-router', '@mantine/', '@tabler/', '@/ui', '@/lib', '@/shell', '@/features', '@/routes'],
    why: 'domain/ is pure rules: no UI framework, no transport, no I/O',
  },
  {
    dir: 'lib',
    deny: ['@mantine/', '@tabler/', '@/ui', '@/shell', '@/features', '@/routes'],
    why: 'lib/ sits below the UI; it must not reach upward',
  },
  {
    dir: 'ui',
    deny: ['@/shell', '@/features', '@/routes'],
    // react-router is permitted here, and deliberately rather than by omission:
    // `ui/Page` builds a breadcrumb, and a crumb that is not a link is not a
    // crumb. The vendor ban below exists because Mantine and Tabler *are* the
    // look, and being able to swap them is the point of `@/ui`; a route is not
    // a look, and `ui/sections.ts` has to be reachable from `features/`, which
    // may not import `@/shell`.
    why: 'ui/ is the design system; it must not know about screens (react-router is fine: a route is not a look)',
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
 * Vendors that exactly one directory may import.
 *
 * Listed separately from RULES so that a new top-level directory is denied by
 * default rather than silently exempt -- and as a list rather than one name so
 * that adding a second vendor is an edit here rather than a hole. `@tabler/`
 * arrived exactly that way: nothing denied it, so every layer could have
 * imported icons directly until it was named. `embla-carousel` was the same
 * hole one package wider -- `@mantine/carousel` is guarded by the first entry,
 * but the engine it wraps ships its own types and nothing stopped a feature
 * reaching for them.
 *
 * `dir` is a path under src/, not a layer: `lib/i18n` is a directory inside a
 * layer, and the whole of `lib/` may not import i18next just because part of
 * it does.
 *
 * `three` and `cannon-es` are the newest entries and the most important ones
 * to hold, because they are by far the largest thing this app depends on. They
 * are reachable from exactly one file -- `ui/D20Scene.tsx`, which is loaded
 * dynamically and must stay that way. A feature importing either directly
 * would pull a hundred and eighty kilobytes into the main bundle without
 * anything failing, which is precisely the silent kind of regression this
 * check exists to make loud.
 */
const VENDOR_RULES = [
  {
    dir: 'ui',
    packages: ['@mantine/', '@tabler/', 'embla-carousel', 'three', 'cannon-es'],
    why: (vendor) => `only src/ui/ may import ${vendor}*; import from '@/ui' instead (re-export it there if missing)`,
  },
  {
    // The same argument as the design system, for the same reason. A translator
    // is a vendor the whole app would otherwise reach for directly, and
    // `useTranslation()` in sixty files is `useTranslation()` in sixty files to
    // change. `@/lib/i18n` re-exports `useT`, which also carries the key type.
    dir: 'lib/i18n',
    packages: ['i18next', 'react-i18next'],
    why: (vendor) => `only src/lib/i18n/ may import ${vendor}*; import { useT } from '@/lib/i18n' instead`,
  },
]

/** True when `rel` names a file inside `dir` (or is `dir` itself, one level up). */
function inDir(rel, dir) {
  return rel === dir || rel.startsWith(dir.split('/').join(sep) + sep)
}

/**
 * The one file allowed to pull in a stylesheet, and the only one that ever
 * should be.
 *
 * `src/ui/app.css` decides how big a control is at each width, and it can only
 * be the last word on that if it is also the *only* word: a second stylesheet
 * anywhere would be an unlayered rule of equal weight whose winner is decided
 * by whichever the bundler emitted last. Naming the importer rather than the
 * directory is deliberate -- `ui/` is where the design system is assembled,
 * but `AppTheme` is where it is switched on, and a stylesheet imported by a
 * component would load only when that component happens to be mounted.
 */
const STYLESHEET_IMPORTER = join('ui', 'AppTheme.tsx')

const IMPORT_RE = /(?:^|\n)\s*(?:import|export)[\s\S]*?from\s*['"]([^'"]+)['"]|import\(\s*['"]([^'"]+)['"]\s*\)/g

/**
 * A side-effect import -- `import './app.css'` -- which the rule above cannot
 * see, because it requires a `from`.
 *
 * That was a real hole rather than a theoretical one: every `import
 * '@mantine/core/styles.css'` in this repo was invisible to the vendor rule,
 * so the one thing `VENDOR_RULES` exists to stop was reachable from any
 * layer as long as you wanted it for its side effect. It is a separate pattern
 * rather than a wider version of the first because a side-effect import has no
 * binding, so nothing downstream of here can treat the two alike.
 */
const SIDE_EFFECT_RE = /(?:^|\n)\s*import\s*['"]([^'"]+)['"]/g

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
  for (const match of source.matchAll(SIDE_EFFECT_RE)) {
    if (match[1]) found.push(match[1])
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
    // Before the vendor rules, because a stylesheet is not a vendor package and
    // the reason it is restricted is different: `src/ui/app.css` can only be the
    // last word on what it says if it is also the only word.
    if (/\.css$/.test(specifier) && rel !== STYLESHEET_IMPORTER) {
      violations.push({
        file: rel,
        specifier,
        why: `only src/${STYLESHEET_IMPORTER} may import a stylesheet; see src/ui/app.css for why there is exactly one`,
      })
      continue
    }
    let claimed = false
    for (const vendorRule of VENDOR_RULES) {
      const vendor = vendorRule.packages.find((prefix) => specifier.startsWith(prefix))
      if (!vendor) continue
      if (!inDir(rel, vendorRule.dir)) {
        violations.push({ file: rel, specifier, why: vendorRule.why(vendor) })
      }
      claimed = true
      break
    }
    if (claimed) continue
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

/**
 * The second rule this script enforces, and the reason it is here rather than
 * in a lint plugin: it is about a fact of the build, not a fact of the source.
 *
 * The suite runs test files without isolating them from one another, which is
 * worth about two and a half times the speed (see the `test` block in
 * vite.config.ts). One module registry per worker is the whole trick, and it is
 * also exactly what `vi.mock` cannot survive: whichever file loads a module
 * first decides what every later file sees, so a mock registered by the second
 * file is simply too late. It does not fail loudly. It hands the test the real
 * module and the assertion fails somewhere else, in a way that depends on the
 * order the files happened to run in -- which is the worst kind of red.
 *
 * There used to be a list of exceptions here, run as a second project with
 * isolation on. It cost 2.4s of every run to isolate one file, because vitest
 * schedules an isolated project ahead of everything else and nothing overlaps
 * it. The exception is gone and so is the list: no test file mocks a module,
 * and a component that needs a dependency swapped takes it as an argument.
 */
const mocking = []
for (const file of walk(SRC)) {
  if (!/\.test\.tsx?$/.test(file)) continue
  if (/\bvi\.mock\s*\(/.test(readFileSync(file, 'utf8'))) {
    mocking.push(`src/${relative(SRC, file).split(sep).join('/')}`)
  }
}

if (mocking.length > 0) {
  console.error('\nvi.mock IS NOT AVAILABLE IN THIS SUITE\n')
  for (const file of mocking) {
    console.error(`  ${file}`)
  }
  console.error('')
  console.error('  The test files share one module registry, so whichever file loads a')
  console.error('  module first decides what every later file gets and a mock registered')
  console.error('  second is ignored -- silently, and only sometimes, depending on the')
  console.error('  order the files ran in.')
  console.error('')
  console.error('  Pass the dependency in instead. InviteSheet takes an optional')
  console.error('  `copyLink` prop that defaults to the real `copyText`, and its test')
  console.error('  hands over a `vi.fn()`; nothing global is touched and nothing leaks')
  console.error('  into the next file. See docs/web.md#the-test-suite-does-not-isolate-test-files.')
  console.error('')
  process.exit(1)
}

console.log('no vi.mock in the suite')
