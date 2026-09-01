import { useId } from 'react'

import {
  Box,
  Carousel,
  D20Roll,
  Paper,
  Stack,
  Text,
  Title,
  useCarouselGestures,
  useIsDesktop,
} from '@/ui'

import { useT } from '@/lib/i18n'
import type { MessageKey } from '@/lib/i18n'

import ship from '@/assets/landing-ship.webp'
import valley from '@/assets/landing-valley.webp'
import volcano from '@/assets/landing-volcano.webp'

/**
 * What a signed-out visitor sees at `/`: what this app is for, three panels of
 * it.
 *
 * The three are the whole of easydnd's ambition in the order you meet them --
 * build a character, join a group, run an adventure. The first two are built;
 * the third is not. None of them is a link even so, because every one of them
 * is behind the sign-in boundary: a panel that led to `/groups` would bounce a
 * signed-out visitor straight back to this page. The header's "Log in" remains
 * the only control here, in the corner `shell/SignInActions.tsx` keeps it, and
 * it carries the visitor where they were going.
 *
 * The captions are the page's own copy now, replacing the sample text that held
 * the shape while the layout was settled. Two of them describe what the app
 * does; the third, `Run sessions`, describes intent -- the battle tracker is not
 * built -- which makes it the one to keep honest, because a landing page that
 * promises it is the only thing on easydnd.org that would.
 *
 * That third caption also spends a word the rest of this project spends
 * elsewhere: a *session* here is being signed in, and a sitting at a table is a
 * **game** (see the README). The landing copy is aimed at somebody who has
 * neither, and to them "session" reads as the evening they are being sold, so
 * the words differ on purpose. Nothing inside the app follows it -- the
 * navigation, the routes and every string behind sign-in still say Games.
 *
 * The hero mark that used to be this page is gone. `ui/DragonMark.tsx` still
 * exists and is still the app's hero art, but the header wordmark already names
 * the app in the corner a visitor looks at to know where they are, and a mark
 * above a carousel is two heroes competing for the same glance.
 *
 * One panel at a time, filling the width. An earlier draft let the neighbours
 * peek, because three *empty* bordered rectangles at full width are
 * indistinguishable from one and a swipe appeared to do nothing. The captions
 * are what retired that: a panel that says something is already distinguishable
 * from the panel beside it, and the peek was only ever paying for the absence.
 *
 * Height. The mark was optically centred: a box of the viewport less *twice*
 * the header offset, so its middle landed on `50dvh` rather than in the middle
 * of what the chrome left. That trick is gone with the thing it served. A
 * carousel is a block, not a figure -- it should fill what the header, the
 * footer and the shell's own padding leave, which is exactly `AppShell`'s main
 * content box. Every term is `AppShell`'s own custom property, so changing the
 * header height, the footer height or the padding in `shell/LandingShell.tsx`
 * resizes this with it, and nothing here repeats a number that lives there.
 * The `, 0rem` fallbacks matter: `routes/Private.tsx` renders this page too,
 * and a deep link is not obliged to arrive inside a shell.
 *
 * The floor is the old `min-height`-not-`height` argument in its new home.
 * `height` on a carousel is a hard height, so a short landscape phone would
 * squeeze a heading and a sentence into a couple of hundred pixels; below the
 * floor the page grows and scrolls instead of clipping.
 *
 * The whole thing is wrapped in `calc(...)` rather than being a bare `max(...)`
 * because Mantine's `rem()` passes a string through untouched only when it
 * starts with `calc(` or `clamp(`. A string beginning `max(` is split on its
 * spaces and each piece converted, which produces nonsense. `LandingPage.test`
 * pins the result for that reason.
 */
const FILL_MAIN =
  'calc(max(320px, 100dvh' +
  ' - var(--app-shell-header-offset, 0rem)' +
  ' - var(--app-shell-footer-offset, 0rem)' +
  ' - var(--app-shell-padding, 0rem) * 2))'

interface Slide {
  /** Stable key, and the stem of the heading's id. */
  key: string
  /** The heading's message key, and the panel's accessible name by way of it. */
  title: MessageKey
  /**
   * Sample copy, as a key. See the note above.
   *
   * Absent on the die, which carries its heading and nothing else -- there is
   * nothing to say about a toy that pressing it does not say. See `DICE_SLIDE`.
   */
  caption?: MessageKey
  /**
   * The panel's own picture, filling it behind the words.
   *
   * One per panel rather than one behind the whole carousel: the art belongs to
   * the thing the panel is about, so it travels with the panel as you swipe. A
   * panel without one is drawn exactly as it was; all three have one now, and
   * the die -- which is a phone's -- keeps its canvas instead.
   *
   * Decorative, and so unnamed: the heading says what the panel is, and a
   * screen reader stopping to describe the scenery would be reading out the
   * wallpaper.
   *
   * ponytail: one size, 1920 wide, ~250KB of WebP, served to a phone as well.
   * A `<picture>` with a narrow variant is the upgrade if the landing page's
   * transfer size ever shows up as a complaint.
   */
  image?: string
  /**
   * Which way round the words go on this picture.
   *
   * A photograph is neither light nor dark, and the three here are not even the
   * same kind of picture: the valley is green and white, the ship blue and
   * white, the volcano red and black. So the pair is chosen per picture rather
   * than derived from the colour scheme -- `onLight` is black in a white halo,
   * `onDark` is white in a black one. See `INK`.
   *
   * Only meaningful with an `image`; a panel without one keeps the theme's own
   * colours, which is what a panel on the page background should have.
   */
  ink?: keyof typeof INK
}

/**
 * The two ways a word can sit on a photograph, and there are only two.
 *
 * A halo rather than a wash over the picture: the art was put there to be seen,
 * and a sheet of translucent white over it is not a photograph any more. The
 * glow is on the letters, so it is as wide as the letters and nothing else is
 * touched.
 *
 * Three stops rather than one, tight to loose: the tight pair is what actually
 * separates a stroke from a busy background -- a lava flow is high-frequency,
 * and a single soft shadow washes across it without ever getting dark enough at
 * the edge of a letter. The loose one carries the group of words as a block.
 *
 * ponytail: two variants, picked by eye per picture. Darkening the photograph
 * instead was tried on the valley and reverted -- the picture is what the panel
 * is for. A fourth photograph gets whichever of the two suits it; the upgrade,
 * if that ever stops working, is sampling the image where the text lands rather
 * than a third entry here.
 */
const INK = {
  onLight: {
    color: 'var(--mantine-color-black)',
    textShadow:
      '0 0 2px var(--mantine-color-white), 0 0 6px var(--mantine-color-white), 0 0 18px var(--mantine-color-white)',
  },
  onDark: {
    color: 'var(--mantine-color-white)',
    textShadow:
      '0 0 2px var(--mantine-color-black), 0 0 6px var(--mantine-color-black), 0 0 18px var(--mantine-color-black)',
  },
} as const

const SLIDES: readonly Slide[] = [
  // Green and white, and white words on it: the same answer as the volcano,
  // because a halo dark enough to read against holds over a sunlit meadow too.
  {
    key: 'build',
    title: 'landing.build.title',
    caption: 'landing.build.caption',
    image: valley,
    ink: 'onDark',
  },
  // Blue and white, and the same answer.
  {
    key: 'group',
    title: 'landing.group.title',
    caption: 'landing.group.caption',
    image: ship,
    ink: 'onLight',
  },
  // Red and black: the one picture that wants the other way round.
  {
    key: 'adventure',
    title: 'landing.adventure.title',
    caption: 'landing.adventure.caption',
    image: volcano,
    ink: 'onDark',
  },
]

/**
 * A fourth panel, on a phone only, and the only thing on this page you can
 * press.
 *
 * The three above describe; this one does something, which is the whole
 * argument for it. Every section of the app is behind sign-in, so a visitor
 * who is curious has nothing to try -- and a die is the one piece of this
 * product that works with no account, no character and no table. It is a toy
 * rather than a feature: it rolls, it says what it rolled, and it keeps
 * nothing.
 *
 * Phone only because it is a *thumb* toy. On a desktop it would be a large
 * ornament you click with a mouse, on the page where a visitor is deciding
 * whether to sign up, and the three panels that say what the app is for should
 * not have to share that decision with a fidget.
 */
const DICE_SLIDE: Slide = {
  key: 'roll',
  title: 'landing.roll.title',
}

export function LandingPage() {
  const t = useT()
  // Two carousels could share a page one day -- a preview beside the real one,
  // say -- and duplicated heading ids would point every panel at the first.
  const headingId = useId()

  // One of the few places outside a @/ui primitive that asks the viewport, and
  // it is asking about the input rather than the layout: the arrows are the
  // only way through this carousel for a pointer, and a phone has no pointer.
  // There they are two 44px controls sitting on top of the panel they are
  // covering, duplicating the swipe that a touchscreen already offers -- so
  // they go, and the indicators below still say how many panels there are.
  const withControls = useIsDesktop()

  // The arrow keys and the wheel, which Mantine does not offer a carousel that
  // nothing has focused yet. This page is the case for it: the carousel is the
  // whole of it, and on a laptop those two are what a visitor reaches for
  // first. See `ui/carouselGestures.ts` for what it declines to take.
  const gestures = useCarouselGestures()

  // The same viewport answer, used for the other thing this page varies. See
  // DICE_SLIDE above for why the die is a phone's and not a pointer's.
  const slides = withControls ? SLIDES : [...SLIDES, DICE_SLIDE]

  return (
    // Named, because a landmark called "region" tells a screen reader nothing.
    // Mantine gives the root `role="region"` and an `aria-roledescription` of
    // "carousel" already; the name is the part only this call site knows.
    <Carousel
      {...gestures}
      aria-label={t('landing.label')}
      height={FILL_MAIN}
      slideGap="md"
      withIndicators
      // Mantine ships these as "Previous slide" / "Next slide", in English and
      // out of reach of the catalogue. They are the arrows' only accessible
      // name, so they are named here instead.
      previousControlProps={{ 'aria-label': t('landing.previous') }}
      nextControlProps={{ 'aria-label': t('landing.next') }}
      withControls={withControls}
      // Bigger than the 26px default. On the viewport that draws them these are
      // the only way through the carousel for anybody not using the arrow keys,
      // they sit over a panel rather than beside it, and 26px is under every
      // published minimum for a pointer target.
      controlSize={44}
      styles={{
        // Mantine's indicators are white at 0.6 opacity, which is drawn for a
        // carousel of photographs. Over a pale panel on a pale page they are
        // invisible, and an invisible indicator is worse than none -- it says
        // there is one panel. The primary colour reads on both schemes, which
        // white does not, and `AppTheme` runs `defaultColorScheme="auto"`.
        indicator: { backgroundColor: 'var(--mantine-primary-color-filled)' },
      }}
      // Looping so the third panel leads back to the first: there is no
      // ordering to preserve here -- three things the app does, not three
      // steps -- so a dead end at either edge would only be a dead end.
      emblaOptions={{ loop: true }}
    >
      {slides.map((slide) => {
        // The die's panel is the one that is a toy rather than a sentence: a
        // heading and then the die itself, no caption under it. It used to
        // carry no words at all and borrow its name from an `aria-label`; now
        // that the heading is on screen it is named the way every other panel
        // is, by the words a visitor can see.
        const bare = slide.key === DICE_SLIDE.key

        return (
          <Carousel.Slide key={slide.key} aria-labelledby={`${headingId}-${slide.key}`}>
            {/* The picture fills the panel, `cover`, and nothing is laid over
                it: no wash, no tint, no blur -- the contrast is on the letters,
                see `INK`.

                No padding on the panel itself, at either width. The words carry
                their own inset, which is what makes the quarter below a quarter
                of the *panel* rather than a quarter of whatever is left inside
                its padding -- a phone's panel and a laptop's would otherwise
                start their text at two different fractions. The die needs the
                same thing for a different reason: its scene sizes itself to the
                box it is given, and a canvas that overflowed the slide would
                sit on top of the panel beside it. */}
            <Paper
              withBorder
              radius="md"
              h="100%"
              p={0}
              style={{
                overflow: 'hidden',
                ...(slide.image === undefined
                  ? {}
                  : {
                      backgroundImage: `url(${slide.image})`,
                      backgroundSize: 'cover',
                      // Pinned to the top edge rather than centred. `cover` on a
                      // panel taller than the picture's aspect crops what does
                      // not fit, and centring takes that crop off both ends --
                      // which is the right default for a photograph nobody
                      // composed for this box, and the wrong one for these,
                      // which are framed to be read from the top down.
                      backgroundPosition: 'top center',
                    }),
              }}
            >
              {bare ? (
                /* The scene takes the whole panel and the heading floats over
                   it, so the die rolls behind its own name rather than in the
                   box underneath it. Stacking the two cost the die the height of
                   a heading on the one viewport where the panel is narrowest,
                   and a d20 with less room to travel in is a duller throw.

                   `pointerEvents: 'none'` is what makes this safe rather than
                   clever: the die is grabbed and flung with pointer events on
                   the canvas, and a heading that swallowed a `pointerdown`
                   would be a dead strip across the top of the toy. */
                <Box h="100%" style={{ position: 'relative' }}>
                  <D20Roll />
                  <Title
                    id={`${headingId}-${slide.key}`}
                    order={2}
                    ta="center"
                    pt="md"
                    px="md"
                    style={{
                      position: 'absolute',
                      insetInline: 0,
                      top: 0,
                      pointerEvents: 'none',
                    }}
                  >
                    {t(slide.title)}
                  </Title>
                </Box>
              ) : (
                /* On a wide screen the words start a quarter of the way down
                   the panel and a third of the way across it -- rather than dead
                   centre, which puts them over the middle of the picture, which
                   is where a photograph's subject is. The two fractions differ
                   because they are answering different things: the horizontal
                   one is about where the subject stands in these photographs,
                   the vertical one about a block that grows downward from its
                   heading and should not end up sitting in the lower half.

                   On a phone the same spacer is a sixth. The panel there is a
                   tall column holding one block, so a quarter of it is a
                   screenful of picture above the first line -- and the top edge
                   itself, which this tried before, leaves the heading nothing to
                   sit against.

                   The spacer above is a *fixed* quarter of the panel, so every
                   heading in the carousel starts on one line whatever the
                   caption under it does. Sharing the leftover space in a 1:2
                   ratio, which is what this did first, measures from the middle
                   of a block whose height is the length of its own caption --
                   so the three headings sat at three different heights and the
                   words jumped as you swiped. `flex-basis` in percent is read
                   against the panel's height here, unlike a percentage padding,
                   which resolves against width even when it is `padding-top`.

                   Below the breakpoint the block spans the panel: a third of
                   390px is not a margin, it is a column too narrow to set type
                   in. The inset it keeps is its own, so the quarter above is
                   measured against the whole panel at every width. */
                <Stack
                  h="100%"
                  gap={0}
                  // The colour and the halo the picture asks for -- see `INK`
                  // and the per-slide `ink` above. Set on the stack rather than
                  // on each line, which is also what stops `Text`'s `dimmed`
                  // grey from surviving into a place it cannot be read (see the
                  // caption below), and what keeps heading and caption in one
                  // treatment instead of two that drift.
                  {...(slide.image === undefined ? {} : { style: INK[slide.ink ?? 'onLight'] })}
                >
                  {/* A quarter on a wide screen, a sixth on a phone. The
                      fraction is smaller there because the panel is a tall
                      column holding one block of words: the same quarter is a
                      screenful of picture before the first line, while the top
                      edge itself leaves the heading nothing to sit against. */}
                  <Box style={{ flex: `0 0 ${withControls ? 25 : 100 / 6}%` }} />

                  <Box px={withControls ? 'xl' : 'md'} style={{ display: 'flex' }}>
                    {withControls ? <Box style={{ flex: 1 }} /> : null}

                    <Stack
                      gap="md"
                      style={{ flex: withControls ? '0 1 480px' : 1, minWidth: 0 }}
                    >
                      {/* Ranged left, which is the same edge the justified
                          caption starts on -- a centred heading over a block
                          with two straight sides reads as a heading belonging
                          to something else. */}
                      <Title id={`${headingId}-${slide.key}`} order={slide.key === 'build' ? 1 : 2}>
                        {t(slide.title)}
                      </Title>
                      {/* Dimmed only where there is nothing behind it. Over a
                          picture the caption inherits the stack's ink, because
                          grey on a photograph is the thing that could not be
                          read.

                          Ranged left, on the heading's own edge. Justified is
                          what this was, and it cost more than it bought at this
                          measure: even hyphenated, four or five words to a line
                          means the spaces stretch visibly and no two lines set
                          the same colour -- worse over a photograph, where every
                          gap shows a different piece of picture. */}
                      {slide.caption ? (
                        <Text {...(slide.image === undefined ? { c: 'dimmed' } : {})}>
                          {t(slide.caption)}
                        </Text>
                      ) : null}
                    </Stack>

                    {withControls ? <Box style={{ flex: 2 }} /> : null}
                  </Box>
                </Stack>
              )}
            </Paper>
          </Carousel.Slide>
        )
      })}
    </Carousel>
  )
}
