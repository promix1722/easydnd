/**
 * Design tokens. Deliberately free of React and Mantine so that the numbers
 * are importable from anywhere -- including the media-query hook, plain tests,
 * and eventually a non-web client -- without dragging a UI framework along.
 *
 * The breakpoint strings are duplicated in postcss.config.cjs, because PostCSS
 * cannot import TypeScript. Change both together.
 */

export const BREAKPOINTS = {
  xs: '36em',
  sm: '48em',
  md: '62em',
  lg: '75em',
  xl: '88em',
} as const

/**
 * The single boundary between the mobile and the desktop rendering. Everything
 * that switches layout switches here, so there is exactly one number to argue
 * about rather than one per component.
 */
export const DESKTOP_FROM = BREAKPOINTS.md

export const DESKTOP_MEDIA_QUERY = `(min-width: ${DESKTOP_FROM})`

/** Deep red, the closest thing to a house colour a D&D book has. */
export const BRAND = [
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
] as const

export const RADIUS_DEFAULT = 'md'
