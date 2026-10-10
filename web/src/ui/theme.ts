import { Button, Paper, createTheme, type CSSVariablesResolver, type MantineThemeOverride } from '@mantine/core'

import { BREAKPOINTS, RADIUS_DEFAULT } from '@/theme/tokens'
import type { Palette, Scheme } from '@/theme/palettes'

/**
 * The Mantine theme, built from the framework-free tokens. This file exists so
 * that `@/theme` stays pure TypeScript: tokens are data, this is the binding
 * of that data to the UI framework, and it lives in the one layer allowed to
 * know about Mantine.
 */
export function themeForPalette(palette: Palette): MantineThemeOverride {
  return createTheme({
    primaryColor: 'brand',
    colors: { brand: [...palette.accent] },
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
      Paper: Paper.extend({ styles: { root: { backgroundColor: 'var(--mantine-color-default)' } } }),
    },
  })
}

/**
 * Bind palette surfaces to Mantine's semantic variables and the ramp shades
 * its cards, inputs and table borders read directly. Light and dark remain
 * separate so System mode can react to device changes without a new theme.
 */
export function variablesForPalette(palette: Palette): CSSVariablesResolver {
  return () => ({
    // Empty on purpose: every one of the five is scheme-dependent, which is
    // exactly why a palette is two schemes rather than one.
    variables: {},
    light: {
      ...schemeVariables(palette.light),
      // Cards and inputs read ramp colors directly rather than semantic variables.
      '--mantine-color-white': palette.light.surface,
      '--mantine-color-gray-3': palette.light.border,
      '--mantine-color-gray-4': palette.light.border,
    },
    dark: {
      ...schemeVariables(palette.dark),
      '--mantine-color-dark-6': palette.dark.surface,
      '--mantine-color-dark-4': palette.dark.border,
    },
  })
}

function schemeVariables(scheme: Scheme): Record<string, string> {
  return {
    '--mantine-color-body': scheme.background,
    '--mantine-color-text': scheme.text,
    '--mantine-color-dimmed': scheme.dimmed,
    '--mantine-color-default': scheme.surface,
    '--mantine-color-default-border': scheme.border,
  }
}
