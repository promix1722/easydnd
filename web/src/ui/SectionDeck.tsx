import { Paper, SimpleGrid, Stack, Title } from '@mantine/core'
import { Fragment, useState } from 'react'
import type { ReactNode } from 'react'

import { TabDeck } from './TabDeck'
import { useIsDesktop } from './useIsDesktop'

export interface DeckSection {
  key: string
  /**
   * The section's name. It heads the panel on a wide screen, names the tab on
   * a phone, and is the slide's accessible name -- one string in three places
   * rather than three strings that can drift apart.
   *
   * A `full` section is headed by nothing on a wide screen, so there the title
   * exists only to name the tab. That is deliberate: the ability cards want no
   * heading above them on a sheet, and a tab has to say something.
   */
  title: string
  content: ReactNode
  /**
   * Where the section sits on a wide screen, where there is room for all of
   * them at once.
   *
   * `full` is its own row across the page, drawn bare -- no panel chrome and no
   * heading. `panel` is a bordered card headed by the title, in a grid with the
   * panels either side of it. Consecutive `panel` sections share one grid, so
   * the order of `sections` is the order of the page.
   */
  desktop: 'full' | 'panel'
}

export interface SectionDeckProps {
  /**
   * Names the carousel. Mantine gives the root `role="region"` and an
   * `aria-roledescription` of "carousel" already; a landmark called "region"
   * tells a screen reader nothing, and the name is the part only the call site
   * knows.
   */
  label: string
  sections: readonly DeckSection[]
  /** Desktop column count for a run of panels. */
  cols?: number
}

/**
 * A page of sections: all of them at once on a wide screen, one at a time
 * behind a row of tabs on a phone.
 *
 * The sibling of `Columns`, and the difference between them is what a phone
 * does. `Columns` collapses: every section is a disclosure, and reading one
 * means opening it and -- since its accordion allows several -- possibly
 * closing three others first. That is right for a page of two or three panels
 * where the answer is usually in the first. It is wrong for a character sheet,
 * which is seven sections a player leafs between at a table, none of them
 * subordinate to the others. Leafing is what a deck of tabs is, so nothing here
 * opens and nothing closes: the carousel decides what is on screen, and the
 * tabs say what there is and where you are.
 *
 * `Columns` stays for the pages that want the other answer -- see
 * `features/status/StatusPanel.tsx`.
 *
 * The phone half is `TabDeck` and nothing else -- a strip of tabs over a
 * carousel of the panels, kept in step with each other. It lived here first,
 * and moved out when the build screen wanted the same gesture: five stage tabs
 * a player leafs between is the same object as seven sheet sections, and two
 * copies of a two-way embla sync is two copies of the one thing in it that can
 * go subtly wrong. What is left here is the part that is actually about a
 * sheet, which is the wide rendering below.
 *
 * So what this component *is*, is the desktop answer: `TabDeck` has no idea
 * that a section knows where it sits on a wide screen, and no reason to.
 */
export function SectionDeck({ label, sections, cols = 2 }: SectionDeckProps) {
  const isDesktop = useIsDesktop()
  const [active, setActive] = useState(sections[0]?.key ?? '')

  if (isDesktop) {
    return <Stack gap="lg">{wideRows(sections, cols)}</Stack>
  }

  // A key that is no longer on the list would leave the strip with nothing
  // selected and the carousel with nothing to scroll to. Sections are static in
  // practice; this costs one pass and removes the question.
  const value = sections.some((section) => section.key === active)
    ? active
    : (sections[0]?.key ?? '')

  return (
    <TabDeck
      label={label}
      value={value}
      onChange={setActive}
      panels={sections.map((section) => ({
        value: section.key,
        label: section.title,
        /*
          `md` where the wide layout stacks its blocks `lg` apart. A phone is
          the viewport with the least of it to spend and the one holding one
          section at a time, so the separation only has to be enough to say
          "different block" -- which it still is, because the cards *inside* a
          block are `xs` apart. The wide screen keeps `lg`: there the same gap
          is separating things that sit beside other things, and tightening it
          there buys nothing anybody sees.
        */
        content: <Stack gap="md">{section.content}</Stack>,
      }))}
    />
  )
}

/**
 * The wide rendering: every section on the page, in the order they were given.
 *
 * A run of consecutive `panel` sections becomes one grid, so that panels
 * side by side stay side by side and a `full` section between two runs breaks
 * them apart. The panel itself is `Columns`' own -- a bordered `Paper` headed
 * by its title -- because the two are the same object seen on the same screen
 * and a second drawing of it would be a second thing to keep in step.
 */
function wideRows(sections: readonly DeckSection[], cols: number): ReactNode[] {
  const rows: ReactNode[] = []
  let run: DeckSection[] = []

  const flushRun = () => {
    if (run.length === 0) return
    const panels = run
    run = []
    rows.push(
      <SimpleGrid key={`panels-${rows.length}`} cols={{ base: 1, md: cols }} spacing="lg">
        {panels.map((section) => (
          <Paper key={section.key} withBorder p="md" radius="md">
            <Stack gap="sm">
              <Title order={4}>{section.title}</Title>
              {section.content}
            </Stack>
          </Paper>
        ))}
      </SimpleGrid>,
    )
  }

  for (const section of sections) {
    if (section.desktop === 'panel') {
      run.push(section)
      continue
    }
    flushRun()
    // A Fragment rather than a wrapper, so that a section whose content is
    // itself several rows -- Vitals is two -- spaces them by the Stack's own
    // gap instead of gaining a box the wide layout never had.
    rows.push(<Fragment key={section.key}>{section.content}</Fragment>)
  }
  flushRun()

  return rows
}
