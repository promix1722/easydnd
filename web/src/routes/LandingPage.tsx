import { useId } from 'react'

import { Carousel, Paper, Stack, Text, Title, useIsDesktop } from '@/ui'

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
 * The captions below are **sample copy**, and deliberately so. They are here to
 * settle the shape -- how long a line runs before it wraps badly on a phone,
 * how a heading and a sentence sit together in a panel that has to fill the
 * window -- rather than to be the words this page ships with. Two of them
 * describe what the app does; the third, `Run an adventure`, describes intent,
 * because the battle tracker is not built. That is the one to keep honest,
 * because a landing page that promises it is the only thing on easydnd.org
 * that would.
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
  /** The heading, and the panel's accessible name by way of it. */
  title: string
  /** Sample copy. See the note above. */
  caption: string
}

const SLIDES: readonly Slide[] = [
  {
    key: 'build',
    title: 'Build a character',
    caption:
      'Answer one question at a time -- race, class, background, and the skills ' +
      'each of them opens -- and the sheet fills itself in. Every modifier, save ' +
      'and proficiency is derived, so there is no arithmetic to get wrong.',
  },
  {
    key: 'group',
    title: 'Join a group',
    caption:
      'Start a table and send one invite link. Everybody who follows it lands in ' +
      'the same group, as owner, DM or player, and each rank can do what a table ' +
      'expects of it and no more.',
  },
  {
    key: 'adventure',
    title: 'Run an adventure',
    caption:
      'Roll initiative once and track the fight from there: hit points, ' +
      'conditions, and whose turn it is, for the whole table at the same time.',
  },
]

export function LandingPage() {
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

  return (
    // Named, because a landmark called "region" tells a screen reader nothing.
    // Mantine gives the root `role="region"` and an `aria-roledescription` of
    // "carousel" already; the name is the part only this call site knows.
    <Carousel
      aria-label="What easydnd is for"
      height={FILL_MAIN}
      slideGap="md"
      withIndicators
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
      {SLIDES.map((slide) => (
        // Labelled *by* the heading rather than carrying a second copy of it in
        // an `aria-label`: the words are on screen now, and two spellings of
        // one name is how they come to disagree.
        <Carousel.Slide key={slide.key} aria-labelledby={`${headingId}-${slide.key}`}>
          <Paper withBorder radius="md" h="100%" p="xl">
            {/* Centred in the panel rather than sitting at its top, because the
                panel is as tall as the window and text pinned to the ceiling of
                it reads as a mistake. `maw` keeps the caption to a readable
                measure on a wide screen. */}
            <Stack h="100%" justify="center" align="center" gap="md">
              <Title id={`${headingId}-${slide.key}`} order={2} ta="center">
                {slide.title}
              </Title>
              <Text c="dimmed" ta="center" maw={480}>
                {slide.caption}
              </Text>
            </Stack>
          </Paper>
        </Carousel.Slide>
      ))}
    </Carousel>
  )
}
