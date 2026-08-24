import { Button, createTheme, type MantineThemeOverride } from '@mantine/core'

import { BRAND, BREAKPOINTS, RADIUS_DEFAULT } from '@/theme/tokens'

/**
 * The Mantine theme, built from the framework-free tokens. This file exists so
 * that `@/theme` stays pure TypeScript: tokens are data, this is the binding
 * of that data to the UI framework, and it lives in the one layer allowed to
 * know about Mantine.
 */
export const theme: MantineThemeOverride = createTheme({
  primaryColor: 'brand',
  colors: { brand: [...BRAND] },
  defaultRadius: RADIUS_DEFAULT,
  breakpoints: { ...BREAKPOINTS },
  cursorType: 'pointer',
  headings: { fontWeight: '650' },
  components: {
    // One button size for the whole app, decided here rather than at each call
    // site -- which is how the header's way in ended up 26px and the /login
    // page it leads to answered with 36px ones. A call site may still pass
    // `size` when it genuinely means something different; none does today, and
    // a second size appearing in three files is exactly how the drift started.
    Button: Button.extend({ defaultProps: { size: 'xs' } }),
  },
})
