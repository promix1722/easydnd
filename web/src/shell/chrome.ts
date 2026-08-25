/**
 * Dimensions the three shells share.
 *
 * The header height lives here because it had already drifted: the landing
 * chrome drew 56 and the phone chrome 52, so signing in on a phone moved the
 * whole page up four pixels -- a flinch at the exact moment a visitor is
 * looking at the app for the first time. One number, one place, no flinch.
 */
export const HEADER_HEIGHT = 56
