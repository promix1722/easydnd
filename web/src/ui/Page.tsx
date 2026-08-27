import {
  Alert,
  Anchor,
  Box,
  Button,
  Group,
  Loader,
  Skeleton,
  Stack,
  Text,
  Title,
  VisuallyHidden,
} from '@mantine/core'
import type { ReactNode } from 'react'
import { Link, useLocation } from 'react-router'

import { CONTENT_MAX_WIDTH, ROW_HEIGHT } from '@/theme/tokens'

import type { PageState } from './pageState'
import { useT } from '@/lib/i18n'

import { sectionFor } from './sections'

export interface Crumb {
  /** What it reads. `null` while the name it comes from is still being fetched. */
  label: string | null
  /** Where it goes. The last crumb never has one: a page is not a link to itself. */
  to?: string
}

export interface PageProps {
  /**
   * The trail *below* the section.
   *
   * The section crumb is never passed -- `Page` derives it from the URL, which
   * is what makes it impossible for a screen to start its trail somewhere the
   * navbar disagrees with. `[]` is a section root, and draws the section's own
   * name as the heading.
   */
  trail: readonly Crumb[]
  /** Beside the heading: a rank, "Read only". */
  badge?: ReactNode
  /** One dimmed line beneath the heading. */
  subtitle?: ReactNode
  /** Acts on the entity the page is about, and on nothing else. */
  actions?: ReactNode
  /** Defaults to ready. */
  state?: PageState
  children?: ReactNode
}

/**
 * The shape every section's pages share: a trail, a heading, and a body.
 *
 * Six decisions live here, and they are here rather than in each screen
 * because the three sections had already drifted apart while doing the same
 * job -- one replaced the whole screen while loading, another drew a spinner
 * above its table, and a third wrapped its title in a layout row that held
 * nothing else.
 *
 * **The last crumb is the heading.** There is no separate title line: the
 * trail ends in the page you are on, drawn large. So a page has exactly one
 * `role="heading"`, at level 2, exactly as it did before -- and the parents
 * above it are a real `<nav aria-label="Breadcrumb">` of links.
 *
 * **A section root renders no `<nav>` at all.** With an empty `trail` there is
 * one crumb, it is the heading, and there is nothing above it to navigate to.
 * That is what makes "the trail replaces the title" need no special case for
 * the three list screens: they render what they always rendered, plus a glyph.
 *
 * **The current page is not repeated inside the trail.** A breadcrumb that
 * ends in a non-link copy of the heading directly beneath it says the same
 * name twice to a screen reader. The nav is the path *to* here; the heading is
 * here.
 *
 * **A crumb whose name has not arrived is a `Skeleton` with a hidden
 * "Loading".** An `<h2>` with no accessible name is a hole in the page, and a
 * heading that appears a beat late makes the whole page jump. The skeleton is
 * a `span`, because a `div` inside a heading is invalid markup.
 *
 * **`loading` and `failed` replace the body, never the header.** This is what
 * retired the early returns that used to swap out whole screens: you still
 * know where you are when the thing you came for will not load, and "Try
 * again" sits beneath a trail rather than alone on a blank page.
 *
 * **The content cap is applied here, once.** See `CONTENT_MAX_WIDTH`. It is
 * left-aligned rather than centred on purpose -- centred content slides
 * sideways every time the desktop navbar is hidden or shown, which turns a
 * navigation control into something that also moves the thing you were reading.
 *
 * **It does not branch on viewport, and must not.** The actions wrap under the
 * heading on a narrow screen because the row is allowed to wrap, and the cap
 * is simply inert below 1024px. The list of components that genuinely swap
 * markup at the breakpoint stays at five -- `Columns`, `DataList`,
 * `ModalSheet`, `TabDeck` and `RootShell` -- and `Page.test.tsx` pins that by comparing
 * the two renderings byte for byte, the way `TabRow.test.tsx` does.
 */
export function Page({
  trail,
  badge,
  subtitle,
  actions,
  state,
  children,
}: PageProps) {
  const { pathname } = useLocation()
  const t = useT()
  const section = sectionFor(pathname)

  const crumbs: Crumb[] = section
    ? [{ label: t(section.label), to: section.to }, ...trail]
    : [...trail]

  const here = crumbs.at(-1)
  const parents = crumbs.slice(0, -1)
  const SectionIcon = section?.icon

  /*
   * What is left of the header once the phone has dropped the section's name.
   *
   * Hiding the section root's heading below `md` left the row it sat in
   * behind: `ROW_HEIGHT` of nothing, plus the stack's gap, above every list in
   * the app -- which on a 390px screen is the most expensive blank space
   * there is. So the row is hidden with its only occupant, and the header
   * block with the row when the subtitle has gone too.
   *
   * A fact about the props rather than about the width: what these decide is
   * whether there is anything *to* draw on a phone, and the breakpoint stays
   * in `visibleFrom`. `Page` still renders one tree at every width.
   */
  const headingIsSectionOnly = parents.length === 0 && SectionIcon !== undefined
  const rowBlankOnPhone = headingIsSectionOnly && badge === undefined && actions === undefined
  const headerBlankOnPhone = rowBlankOnPhone && subtitle === undefined

  return (
    <Box maw={CONTENT_MAX_WIDTH}>
      <Stack gap="lg">
        <Stack gap={4} {...(headerBlankOnPhone ? { visibleFrom: DESKTOP_ONLY } : {})}>
          {/*
            The whole trail is one line, and every part of it is the same size.

            The section's name used to be drawn as a heading on its list screen
            and as a small crumb on everything below it -- the same word, two
            sizes, changing as you walked in. It is one size now, and the page's
            own name follows it on the same line rather than underneath.

            That line is smaller than the `h2` this used to be, because it is
            carrying two names instead of one and an `h2` pair plus a badge plus
            an action cluster is more heading than any of these pages needs.
          */}
          <Group
            justify="space-between"
            // Centred on the trail rather than pinned to the top of the row.
            // The heading line is `ROW_HEIGHT` tall and its text sits in the
            // middle of that, so a `flex-start` action hung a button above the
            // words it belongs to -- a few pixels, and enough to read as two
            // rows that failed to line up.
            align="center"
            wrap="wrap"
            gap="xs"
            // The same row height the navbar's entries have, so the heading and
            // the navbar entry naming the same section sit on one line. See
            // ROW_HEIGHT.
            mih={ROW_HEIGHT}
            {...(rowBlankOnPhone ? { visibleFrom: DESKTOP_ONLY } : {})}
          >
            <Group gap="xs" align="center" wrap="wrap" mih={ROW_HEIGHT}>
              {/*
                A section root's heading *is* the section's name, so on a phone
                it is the same word the chrome's selector is already showing an
                inch above it -- the duplication this hides, for the same reason
                the section crumb is hidden on a detail page.

                The cost is real and worth stating: below `md` these pages have
                no heading in the accessibility tree, and the control that names
                them is a button rather than an `h2`. The alternative was
                printing the word twice on a 390px screen.
              */}
              {parents.length === 0 && SectionIcon !== undefined && (
                <Group gap="xs" align="center" wrap="nowrap" visibleFrom={DESKTOP_ONLY}>
                  <SectionIcon size={GLYPH} aria-hidden />
                  <Title order={2} fz={HEADING_SIZE}>
                    {here === undefined ? null : <CrumbLabel label={here.label} size="lg" />}
                  </Title>
                </Group>
              )}

              {parents.length > 0 && (
                <nav aria-label={t('page.breadcrumb')}>
                  <Group component="ol" gap="xs" wrap="wrap" style={LIST_RESET}>
                    {parents.map((crumb, index) => {
                      const isSection = index === 0 && SectionIcon !== undefined
                      return (
                        <Group
                          component="li"
                          key={crumb.to ?? index}
                          gap="xs"
                          wrap="nowrap"
                        >
                          {/*
                            The section crumb is a glyph on a phone and a glyph
                            plus its word on a desktop.

                            The word goes because the phone's one row of chrome
                            already names the section you are in, an inch above
                            -- so spelling it out again spends a 390px line
                            restating what is already on screen. The *way back*
                            is not a restatement, though, and dropping the crumb
                            entirely took it with it: from a group's page there
                            was no way back to Groups but the browser's own. So
                            the glyph stays and carries the link.

                            Deeper crumbs -- a group on a shared character's
                            sheet -- keep their words at both widths, because
                            those are real parents rather than a restatement of
                            the chrome.

                            Done with `visibleFrom` rather than a branch on
                            purpose: `Page` renders one tree at every width and
                            this keeps it that way.
                          */}
                          {crumb.to === undefined ? (
                            <>
                              {isSection && <SectionIcon size={GLYPH} aria-hidden />}
                              <CrumbLabel label={crumb.label} size="lg" />
                            </>
                          ) : (
                            <Anchor
                              component={Link}
                              to={crumb.to}
                              fz={HEADING_SIZE}
                              fw={650}
                              // The word is the accessible name at both widths,
                              // because below `md` it is not drawn and a link
                              // whose only content is an `aria-hidden` glyph has
                              // no name at all.
                              {...(isSection ? { 'aria-label': crumb.label ?? undefined } : {})}
                            >
                              <Group gap="xs" wrap="nowrap">
                                {isSection && <SectionIcon size={GLYPH} aria-hidden />}
                                {isSection ? (
                                  <Box visibleFrom={DESKTOP_ONLY}>
                                    <CrumbLabel label={crumb.label} size="lg" />
                                  </Box>
                                ) : (
                                  <CrumbLabel label={crumb.label} size="lg" />
                                )}
                              </Group>
                            </Anchor>
                          )}
                          {/* Decoration, not content: the list already carries
                              the nesting, and a screen reader announcing
                              "slash" between every pair is noise. It goes with
                              the word it separated. */}
                          <Text
                            span
                            c="dimmed"
                            fz={HEADING_SIZE}
                            aria-hidden
                            {...(isSection ? { visibleFrom: DESKTOP_ONLY } : {})}
                          >
                            /
                          </Text>
                        </Group>
                      )
                    })}
                  </Group>
                </nav>
              )}

              {/* The heading holds the page's own name and nothing else, so its
                  accessible name is that name -- not the whole trail. Drawn here
                  for every page except a section root, which draws its own
                  above so that the phone can drop it. */}
              {!(parents.length === 0 && SectionIcon !== undefined) && (
                <Title order={2} fz={HEADING_SIZE}>
                  {here === undefined ? null : <CrumbLabel label={here.label} size="lg" />}
                </Title>
              )}
              {badge}
            </Group>
            {actions !== undefined && (
              <Group gap="xs" wrap="nowrap">
                {actions}
              </Group>
            )}
          </Group>

          {subtitle !== undefined && (
            <Text c="dimmed" size="sm">
              {subtitle}
            </Text>
          )}
        </Stack>

        <Body state={state ?? READY}>{children}</Body>
      </Stack>
    </Box>
  )
}

const READY: PageState = { kind: 'ready' }

/**
 * The trail's size, which is the heading's size, which is one size.
 *
 * Smaller on a wide screen than on a narrow one, and that is the way round it
 * sounds wrong: desktop is where the line carries the *most* -- a section, a
 * separator, a page name, a badge -- because the phone drops the section crumb
 * entirely. A responsive value rather than a branch, so the tree stays one tree.
 */
const HEADING_SIZE = { base: 'h2', md: 'h3' } as const

/**
 * One glyph size for the trail -- the navbar's, exactly.
 *
 * A section's glyph appears twice at once on a section root: in the navbar and
 * again beside the heading, a divider apart. At 20 against the navbar's 18 the
 * pair read as two sizes of the same mark rather than one mark said twice.
 */
const GLYPH = 18

/** Matches `DESKTOP_FROM`; the app has one breakpoint and this is it. */
const DESKTOP_ONLY = 'md' 

// `Group component="ol"` still draws a browser's default list bullets and
// padding; Mantine styles the flex box, not the element it was asked for.
const LIST_RESET = { listStyle: 'none', margin: 0, padding: 0 } as const

/** A crumb's text, or a placeholder that still has a name. */
function CrumbLabel({ label, size }: { label: string | null; size: 'sm' | 'lg' }) {
  const t = useT()
  if (label !== null) return <>{label}</>
  return (
    <>
      <VisuallyHidden>{t('page.loading')}</VisuallyHidden>
      <Skeleton
        component="span"
        width={size === 'lg' ? 220 : 90}
        height="1em"
        style={{ display: 'inline-block', verticalAlign: 'middle' }}
      />
    </>
  )
}

/**
 * The body, or what is standing in for it.
 *
 * Both blocks are the markup the screens already wrote inline -- lifted rather
 * than redesigned, so this change moved where they live without changing what
 * anybody sees.
 */
function Body({ state, children }: { state: PageState; children: ReactNode }) {
  const t = useT()
  if (state.kind === 'loading') {
    return (
      <Group gap="xs">
        <Loader size="sm" />
        <Text size="sm" c="dimmed">
          {state.what ?? t('page.loadingEllipsis')}
        </Text>
      </Group>
    )
  }

  if (state.kind === 'failed') {
    return (
      <Alert color="red" title={state.title}>
        <Stack gap="xs" align="flex-start">
          <Text size="sm">{state.detail}</Text>
          {state.onRetry !== undefined && (
            <Button variant="light" onClick={state.onRetry}>
              {t('page.retry')}
            </Button>
          )}
        </Stack>
      </Alert>
    )
  }

  return <>{children}</>
}
