/**
 * Dimensions the three shells share.
 *
 * The header height lives here because it had already drifted: the landing
 * chrome drew 56 and the phone chrome 52, so signing in on a phone moved the
 * whole page up four pixels -- a flinch at the exact moment a visitor is
 * looking at the app for the first time. One number, one place, no flinch.
 */
export const HEADER_HEIGHT = 56

/**
 * The strips of screen the hardware covers, as CSS the browser fills in.
 *
 * These are zero everywhere except an installed app on a device with a notch or
 * a home indicator -- and that case is the whole reason they exist. Launched
 * from a home screen there is no browser chrome to occupy the top of the
 * screen, so `viewport-fit=cover` in index.html hands the page the full display
 * including the parts under the status bar. Without the matching padding the
 * header simply sits beneath the notch, which is invisible in a browser tab and
 * obvious the moment anybody installs.
 *
 * The fallbacks are load-bearing rather than defensive: `env()` with no second
 * argument is invalid on a browser that does not know the variable, and an
 * invalid value inside `calc()` poisons the whole declaration -- so the header
 * would lose its height rather than gain nothing.
 */
export const SAFE_TOP = 'env(safe-area-inset-top, 0px)'
export const SAFE_BOTTOM = 'env(safe-area-inset-bottom, 0px)'

/**
 * What the header occupies: its own height, plus whatever is over it.
 *
 * The bar grows upward into the status bar and paints there, while the row of
 * controls inside stays HEADER_HEIGHT tall -- which is why this pairs with
 * `paddingTop: SAFE_TOP` on the same element rather than replacing it.
 *
 * The `calc()` wrapper is not decoration, and handing AppShell a bare
 * `env(safe-area-inset-top, 0px)` as a height would not work. Mantine runs
 * every size through its `rem()` converter, which returns a `calc(...)` string
 * untouched but splits anything containing a comma and converts each part --
 * and `env()` with a fallback contains one. It would come back mangled.
 *
 * A note for anyone reading a test: jsdom's CSS parser drops a bare `env()`
 * from a standard property, so `paddingTop: SAFE_TOP` is simply absent from the
 * DOM under vitest while being correct in every browser. Values inside
 * `calc()`, and custom properties, survive.
 */
export const HEADER_BOX = `calc(${HEADER_HEIGHT}px + ${SAFE_TOP})`
