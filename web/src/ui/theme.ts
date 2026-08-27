import { Button, createTheme, type CSSVariablesResolver, type MantineThemeOverride } from '@mantine/core'

import { BREAKPOINTS, PALETTE, RADIUS_DEFAULT } from '@/theme/tokens'
import type { Scheme } from '@/theme/palettes'

/**
 * The Mantine theme, built from the framework-free tokens. This file exists so
 * that `@/theme` stays pure TypeScript: tokens are data, this is the binding
 * of that data to the UI framework, and it lives in the one layer allowed to
 * know about Mantine.
 */
export const theme: MantineThemeOverride = createTheme({
  primaryColor: 'brand',
  colors: { brand: [...PALETTE.accent] },
  defaultRadius: RADIUS_DEFAULT,
  breakpoints: { ...BREAKPOINTS },
  cursorType: 'pointer',
  headings: { fontWeight: '650' },
  components: {
    // One button size for the whole app, decided here rather than at each call
    // site -- which is how the header's way in ended up 26px and the /login
    // page it leads to answered with 36px ones. A call site may still pass
    // `size` when it genuinely means something different; the phone header's
    // section dropdown is the one that does.
    //
    // It is one size at *every* width. A version of this briefly gave phones
    // 44px controls on the touch-target argument, with the numbers in
    // `./app.css`; at 390px the app is mostly controls and inflating all of
    // them cost more in scrolling than it bought in accuracy. What survived
    // from that is the one thing that was a browser fact rather than a
    // judgement: a field's text is 16px on a phone, or iOS Safari zooms the
    // page. See `./app.css`.
    Button: Button.extend({ defaultProps: { size: 'xs' } }),
  },
})

/**
 * The palette's surfaces, bound to the variables Mantine already reads.
 *
 * `createTheme` has no way to say "the page's background colour" -- it takes
 * colour *ramps*, and the body, the panels and the hairlines are not a ramp.
 * `cssVariablesResolver` is the lever for those, and it takes light and dark
 * separately, which is what lets `defaultColorScheme="auto"` keep working
 * untouched: the browser picks the scheme, and each scheme already has its
 * five colours waiting.
 *
 * These are Mantine's *own* variable names rather than a private set, and that
 * is the whole reason this is five lines instead of a stylesheet. Every
 * `Card`, `Table`, `Paper`, `Alert`, `Modal` and `Drawer` in the app already
 * reads them, so a palette reaches all of them without a single per-component
 * override.
 *
 * That argument used to end "-- and without this repo gaining its first CSS
 * file". It has one now, `./app.css`, and it is one rule long: a browser
 * behaviour that no theme value can express. The distinction still holds --
 * values belong here, and the stylesheet is for the things that are not
 * values.
 */
export const cssVariables: CSSVariablesResolver = () => ({
  // Empty on purpose: every one of the five is scheme-dependent, which is
  // exactly why a palette is two schemes rather than one.
  variables: {},
  light: schemeVariables(PALETTE.light),
  dark: schemeVariables(PALETTE.dark),
})

function schemeVariables(scheme: Scheme): Record<string, string> {
  return {
    '--mantine-color-body': scheme.background,
    '--mantine-color-text': scheme.text,
    '--mantine-color-dimmed': scheme.dimmed,
    '--mantine-color-default': scheme.surface,
    '--mantine-color-default-border': scheme.border,
  }
}
