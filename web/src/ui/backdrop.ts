import type { CSSProperties } from 'react'

import tile from '@/assets/backdrop-tile.webp'

/**
 * The page's ground: a tiled sheet of hand-drawn D&D marginalia -- dice, swords,
 * scrolls, a dragon, a wizard's hat -- washed down to almost nothing.
 *
 * The app was flat theme colour everywhere except the landing carousel, so
 * signing in was a cut from three photographs to a blank sheet. A photograph was
 * tried here first and is the wrong kind of picture for the job: it has a
 * subject, and a subject behind a table of characters competes with it. A
 * pattern has none. It is texture, it repeats, and it says "this is a game about
 * dungeons" without asking to be looked at.
 *
 * **Washed, not dimmed.** The wash is `--mantine-color-body` -- the page's own
 * background colour -- at 88%, over the tile. That is also what makes one
 * declaration serve both colour schemes: the variable is white in the light one
 * and near-black in the dark, so the drawing is lightened or darkened towards
 * whichever page it is sitting on and every foreground colour keeps the contrast
 * it was chosen against. In the dark scheme it comes out the other way round --
 * dark lines on a ground a shade lighter than the page -- which is the same
 * drawing and reads the same way.
 *
 * `color-mix` rather than a pair of `rgba()` literals for the same reason: the
 * wash has to be the theme's colour, and the theme's colour is a variable.
 *
 * The tile is 1024px square and drawn at half that, so the doodles are small
 * enough to read as texture rather than as illustration. It repeats in both
 * directions, which is what a 170KB file buys over a photograph that has to
 * cover a viewport: one tile serves every page at every size.
 *
 * It goes on the **main box only**. The header and the navbar keep their own
 * flat backgrounds: they are chrome, they sit over the content, and one sheet of
 * pattern running under all three turns three surfaces into one.
 */
export const PAGE_BACKDROP: CSSProperties = {
  backgroundImage: `linear-gradient(
      color-mix(in srgb, var(--mantine-color-body) 88%, transparent),
      color-mix(in srgb, var(--mantine-color-body) 88%, transparent)
    ), url(${tile})`,
  // Per layer, in the order above: the wash stretches over the box, the drawing
  // tiles under it at half its natural size.
  backgroundRepeat: 'no-repeat, repeat',
  backgroundSize: 'auto, 512px 512px',
  // Scrolling with the page rather than `fixed`. iOS Safari sizes a fixed
  // background against the document rather than the viewport; with a repeating
  // tile it is also the honest behaviour, since there is no framing to hold
  // still.
  backgroundAttachment: 'scroll',
}

/**
 * Every page takes it, with no exceptions.
 *
 * There used to be two carve-outs and both are gone. The die's screen opted out
 * because a canvas with its own lit floor is already a picture, and the landing
 * carousel opted out because it is three photographs; the answer in both cases
 * is that one ground under every page is worth more than either exception. The
 * wash is 88% of the page's own colour, so what shows through behind a canvas or
 * between two panels is texture rather than a second picture.
 *
 * There is deliberately no `backdropFor(pathname)` any more. It existed only to
 * name the exceptions, and a function that now returns the same value for every
 * path is a lookup pretending to be a decision -- so the three shells spread
 * `PAGE_BACKDROP` directly and there is nothing left to keep in step.
 */
