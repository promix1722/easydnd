import { useId } from 'react'

import { BRAND } from '@/theme/tokens'

/**
 * The red dragon badge: an angular dragon's head in profile, inside a hexagon.
 *
 * The head is cut from straight segments rather than curves -- a faceted,
 * abstract read rather than a heraldic one -- so it holds its shape when the
 * landing page scales it down and there are no beziers to flatten.
 *
 * This is the app's *hero* mark, distinct from the d20 in `public/favicon.svg`
 * that the tab, the installed icons and the header wordmark share. The d20 is
 * drawn to survive 16px; this one is drawn to carry a page, which is a
 * different brief and so a different piece of art. They share a palette and a
 * point-up hexagonal silhouette so they read as siblings.
 *
 * Inlined rather than served from `public/`, which is the opposite of what
 * `shell/Wordmark.tsx` does. Wordmark's reasoning -- that the browser has
 * already fetched the file for the tab -- is true of the favicon and of
 * nothing else; a hero mark fetched separately would be a round trip before
 * the landing page has anything to show. There is no `vite-plugin-svgr` here,
 * so an inline component is also the only way to get one without a dependency.
 *
 * The colours are literals rather than `currentColor`. It is a two-tone badge
 * whose contrast is the whole point, and it carries its own cream field so it
 * reads on either colour scheme -- `AppTheme` runs `defaultColorScheme="auto"`,
 * so a mark that inherited the page's colours would vanish on one of them.
 *
 * Geometry is a 128-unit square: a point-up hexagon of circumradius 56 centred
 * on (64, 64), the same construction `scripts/gen-icons.mjs` uses for the d20.
 * Note the frame pinches toward the top and bottom vertices, so the art is
 * composed to the hexagon rather than to a bounding box -- the horn tip stops
 * short of the upper-right edge for that reason, not by accident: at its x it
 * is about three units inside the frame's stroke.
 */

/** The deepest step of the palette -- the red `public/favicon.svg` is drawn in. */
const RED = BRAND[9]
/** Parchment. A literal because `favicon.svg` cannot import a token either. */
const CREAM = '#fff6e8'

export interface DragonMarkProps {
  /**
   * Rendered edge length. Any CSS length, so a caller can clamp it against the
   * viewport rather than picking one number for every screen.
   */
  size?: number | string
  /** The accessible name. See the `role="img"` note below. */
  title?: string
}

export function DragonMark({ size = 220, title = 'easydnd' }: DragonMarkProps) {
  // Two of these can share a page (a mark and a favicon preview, say), and a
  // duplicated id would point every label at the first one.
  const titleId = useId()

  return (
    // Named rather than `aria-hidden`, unlike Wordmark's `alt=""`. Wordmark is
    // decorative because the word "easydnd" sits right beside it; here the
    // mark is the only thing identifying the app on a page with no text, so
    // hiding it would leave a screen reader with an empty main region.
    <svg
      viewBox="0 0 128 128"
      role="img"
      aria-labelledby={titleId}
      // Sized through style rather than the width/height attributes: a caller
      // passes a CSS clamp, and only the style property is reliably CSS.
      style={{ width: size, height: size, maxWidth: '100%', display: 'block' }}
    >
      <title id={titleId}>{title}</title>

      {/* The frame. Stroked on the centreline, so it spans 5..123 of the 128. */}
      <path
        d="M 64 8 L 112.5 36 L 112.5 92 L 64 120 L 15.5 92 L 15.5 36 Z"
        fill={CREAM}
        stroke={RED}
        strokeWidth="6"
        strokeLinejoin="round"
      />

      {/* The head, as overlapping solid masses rather than one path: each can
          be nudged without unpicking its neighbours, and the union of same
          coloured fills needs no winding rule to behave. */}
      <g fill={RED}>
        {/* cranium, horn and neck, cut in one run of straight segments */}
        <path d="M 28 70 L 38 57 L 53 48 L 69 33 L 91 27 L 73 47 L 94 50 L 72 60 L 91 72 L 64 65 L 79 91 L 58 70 L 40 75 Z" />
        {/* the dropped lower jaw */}
        <path d="M 32 80 L 45 85 L 59 80 L 72 88 L 56 94 L 41 91 Z" />
        {/* upper fangs, two subpaths in one fill */}
        <path d="M 38 72 L 43 72 L 40.5 78 Z M 47 70 L 52 69 L 50 76 Z" />
      </g>
    </svg>
  )
}
