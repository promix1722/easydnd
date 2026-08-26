/**
 * The colour the app is wearing, as data.
 *
 * This is a **development tool, not a setting.** There is no picker, no
 * environment variable and nothing to strip from a production build: you change
 * one word in `./tokens.ts`, Vite repaints, and you are looking at the other
 * one. A user never chooses a palette, because a user was never the audience --
 * the point is to be able to try the app in four skins while designing it
 * without editing forty files, and to have exactly one answer to "what colour
 * is this app" when the question is asked by the favicon generator, the PWA
 * manifest and the browser's own chrome.
 *
 * Framework-free on purpose, like its neighbour: `scripts/gen-icons.mjs` reads
 * this file from plain Node to rasterise the icon set, and it could not if
 * there were a React or Mantine import anywhere in the graph. The layer check
 * enforces that (`theme/` denies `react`, `@mantine/`, `@tabler/` and `@/`),
 * and `tsconfig.app.json`'s `erasableSyntaxOnly` is what makes Node able to
 * run it at all -- see the generator's own comment.
 *
 * **A palette defines both colour schemes.** `ui/AppTheme.tsx` runs
 * `defaultColorScheme="auto"`, so the app never gets to choose which scheme a
 * visitor sees; one that defined only `light` would be a palette that is
 * unreadable to half the people who open it.
 */

/**
 * One colour scheme's five surfaces. Mantine derives everything else -- hover
 * states, disabled text, the shade a `light` variant button sits on -- from
 * these plus the accent ramp, which is why five is the whole list rather than
 * an arbitrary subset of a longer one.
 */
export interface Scheme {
  /** The page behind everything. Mantine's `--mantine-color-body`. */
  background: string
  /** The panel that sits on it: Card, Paper, Modal, Drawer. */
  surface: string
  /** Body text. */
  text: string
  /** Labels, captions, a table's column headers -- Mantine's `c="dimmed"`. */
  dimmed: string
  /** Hairlines: table rules, card borders, inputs. */
  border: string
}

export interface Palette {
  name: PaletteName
  /**
   * Ten steps, lightest first: Mantine's accent ramp, verbatim. Ten because
   * that is what `createTheme`'s `colors` takes, not because ten shades were
   * wanted.
   */
  accent: readonly [string, string, string, string, string, string, string, string, string, string]
  /**
   * *The* brand colour: the one the mark is cut from, and the one that reaches
   * the favicon, the installed-app icons, the PWA manifest and the browser's
   * own `theme-color`.
   *
   * Written out rather than held as an index into `accent`, because an index
   * read back under `noUncheckedIndexedAccess` is `string | undefined` and
   * every consumer would have to answer for a case that cannot happen. It must
   * be one of the ten, and `palettes.test.ts` is what holds it to that.
   */
  brand: string
  /** The two-tone mark's light field: the strokes drawn over `brand`. */
  ink: string
  light: Scheme
  dark: Scheme
}

export type PaletteName = 'dragon' | 'parchment' | 'midnight' | 'moss'

/**
 * The one the app ships in, and the reason the other three can exist without
 * risk: its schemes are Mantine's own defaults written out longhand, and its
 * accent is the deep red this project has always used. Switching away and back
 * is therefore a no-op rather than an approximation of what used to be there.
 */
const dragon: Palette = {
  name: 'dragon',
  accent: [
    '#ffeaec',
    '#fdd4d6',
    '#f2a7ab',
    '#e8777d',
    '#e04f57',
    '#dc3740',
    '#db2b35',
    '#c21e29',
    '#ae1624',
    '#99051d',
  ],
  brand: '#99051d',
  ink: '#fff6e8',
  light: {
    background: '#ffffff',
    surface: '#ffffff',
    text: '#000000',
    dimmed: '#868e96',
    border: '#ced4da',
  },
  dark: {
    background: '#242424',
    surface: '#2e2e2e',
    text: '#c9c9c9',
    dimmed: '#828282',
    border: '#424242',
  },
}

/** The book look: warm ochre on cream, and a dark scheme that is warm too. */
const parchment: Palette = {
  name: 'parchment',
  accent: [
    '#fdf5e6',
    '#f7e8c8',
    '#efd79b',
    '#e6c46b',
    '#dcb345',
    '#d6a62c',
    '#d29f20',
    '#b98a15',
    '#a4790d',
    '#8d6605',
  ],
  brand: '#8d6605',
  ink: '#fffaf0',
  light: {
    background: '#fbf7ef',
    surface: '#fffdf8',
    text: '#2b2318',
    dimmed: '#7a6a55',
    border: '#e3d8c4',
  },
  dark: {
    background: '#1c1813',
    surface: '#262019',
    text: '#ece3d4',
    dimmed: '#a4947c',
    border: '#3c3327',
  },
}

/** Indigo, and dark-first: the scheme it was drawn for is the dark one. */
const midnight: Palette = {
  name: 'midnight',
  accent: [
    '#eef0ff',
    '#d9dcf7',
    '#b0b6ec',
    '#848ce1',
    '#5f6ad8',
    '#4854d3',
    '#3b48d2',
    '#2d3aba',
    '#2632a6',
    '#1c2791',
  ],
  brand: '#1c2791',
  ink: '#eef0ff',
  light: {
    background: '#f7f8fc',
    surface: '#ffffff',
    text: '#14172b',
    dimmed: '#6b7091',
    border: '#d5d8e8',
  },
  dark: {
    background: '#0f1120',
    surface: '#191c30',
    text: '#dcdff0',
    dimmed: '#8b90ae',
    border: '#2c3050',
  },
}

/** Deep green, for a palette that is neither warm nor red. */
const moss: Palette = {
  name: 'moss',
  accent: [
    '#eff8ef',
    '#dcefdd',
    '#b6dfb9',
    '#8dcd92',
    '#6cbe72',
    '#57b45e',
    '#4aaf52',
    '#399a42',
    '#2d8938',
    '#1c772b',
  ],
  brand: '#1c772b',
  ink: '#f2fbf2',
  light: {
    background: '#f7faf6',
    surface: '#ffffff',
    text: '#16231a',
    dimmed: '#66796a',
    border: '#d2e0d3',
  },
  dark: {
    background: '#101610',
    surface: '#1a221b',
    text: '#dbe7dc',
    dimmed: '#869185',
    border: '#2b352c',
  },
}

export const PALETTES: Readonly<Record<PaletteName, Palette>> = {
  dragon,
  parchment,
  midnight,
  moss,
}
