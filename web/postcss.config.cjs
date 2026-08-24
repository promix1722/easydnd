// Mantine ships its styles as PostCSS with custom mixins and variables; without
// this preset the responsive helpers (`light-dark()`, `@mixin smaller-than`)
// pass through unprocessed and every breakpoint silently stops working.
module.exports = {
  plugins: {
    'postcss-preset-mantine': {},
    'postcss-simple-vars': {
      variables: {
        // Keep in lockstep with BREAKPOINTS in src/theme/tokens.ts.
        'mantine-breakpoint-xs': '36em',
        'mantine-breakpoint-sm': '48em',
        'mantine-breakpoint-md': '62em',
        'mantine-breakpoint-lg': '75em',
        'mantine-breakpoint-xl': '88em',
      },
    },
  },
}
