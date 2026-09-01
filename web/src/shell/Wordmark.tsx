import { Link } from 'react-router'

import { Group, Title } from '@/ui'

/** Rendered edge length of the d20, the same in every header. */
const MARK = 28

/** The corner is a link, and a link in a header should not look like one. */
const HOME = { textDecoration: 'none', color: 'inherit' } as const

/**
 * The mark and the name, top left of every header.
 *
 * One component rather than a `<Title>` repeated in three shells, because the
 * corner a visitor uses to know where they are should not drift between the
 * logged-out header, the desktop navbar shell and the phone one.
 *
 * The icon is the same d20 as `public/favicon.svg` and the installed PWA's
 * icons -- served as a static asset rather than inlined, so the browser reuses
 * the copy it already fetched for the tab.
 *
 * `order` sets the caption's heading level and nothing else. The mark is one
 * size in every header, because it is the same corner in every header: it used
 * to scale with `order`, so a phone drew 24px signed out and 28px signed in and
 * the icon changed size at the moment of signing in. Same reason `HEADER_HEIGHT`
 * is one number -- see ./chrome.ts.
 *
 * `caption={false}` draws the mark alone. That is the phone header, where the row
 * has to carry the section dropdown and two account controls as well, and the
 * word is the one thing on it that tells a visitor something they already know
 * -- they are looking at the app they opened. The mark stays, because the
 * corner still needs to be recognisably this app's. It is a prop rather than a
 * second component for the reason above: one corner, one definition.
 *
 * **The corner is a link home**, in all three shells, because that is what a
 * logo in that corner has meant on every site since there were sites -- and it
 * is one destination on both sides of the sign-in boundary: `/` is the carousel
 * signed out and the character list signed in. It replaced `/legal`'s "Back to
 * easydnd" button, which was a second way home drawn only on that page; a
 * licence notice should not need chrome of its own to get out of.
 *
 * `color: inherit` and no underline, because the mark and the wordmark are the
 * link's own appearance -- a blue underlined "easydnd" in the corner would be
 * the browser's default styling showing through, not a decision. See `HOME`.
 */
export function Wordmark({ order = 3, caption = true }: { order?: 3 | 4; caption?: boolean }) {
  // Alone, the mark is the only thing identifying the app, so it takes the
  // name. Beside the caption it is decorative -- announcing it as well would
  // only make a screen reader say easydnd twice.
  //
  // Returned bare rather than in a Group of one, which is not tidiness: a
  // Group is a flex row with a gap, and a gap after the last child still
  // occupies the row. Next to a control that carries its own padding, that
  // read as the mark and the menu having drifted apart.
  if (!caption)
    return (
      <Link to="/" style={HOME}>
        <img src="/favicon.svg" alt="easydnd" width={MARK} height={MARK} />
      </Link>
    )

  return (
    // The link wraps both, so the whole corner is the target rather than the
    // word alone. It takes its accessible name from the Title inside it, which
    // is why the image beside it stays `alt=""`.
    <Link to="/" style={HOME}>
      <Group gap="xs" wrap="nowrap">
        <img src="/favicon.svg" alt="" width={MARK} height={MARK} />
        <Title order={order}>easydnd</Title>
      </Group>
    </Link>
  )
}
