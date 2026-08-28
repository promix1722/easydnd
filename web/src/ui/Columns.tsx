import { Accordion, Paper, SimpleGrid, Stack, Title } from '@mantine/core'
import type { ReactNode } from 'react'

import { useIsDesktop } from './useIsDesktop'

export interface ColumnsSection {
  key: string
  title: string
  content: ReactNode
}

export interface ColumnsProps {
  sections: readonly ColumnsSection[]
  /** Desktop column count. Mobile is always a single accordion. */
  cols?: number
  /** Section keys expanded by default on mobile. */
  defaultOpen?: readonly string[]
}

/**
 * Side-by-side panels on desktop, a collapsible accordion on mobile.
 *
 * Panels that all want to be visible at once on a wide screen would be a mile
 * of scrolling on a phone, and collapsing is one of the two answers to that.
 * It is the right one where a page has a few panels, the first of them is
 * usually what was come for, and the rest are detail worth having but not worth
 * scrolling past.
 *
 * `SectionDeck` is the other answer, for the case where the sections are peers
 * and a reader moves between them rather than down them. The character sheet
 * used to be here and is there now; see docs/web.md.
 */
export function Columns({ sections, cols = 2, defaultOpen }: ColumnsProps) {
  const isDesktop = useIsDesktop()

  if (isDesktop) {
    return (
      <SimpleGrid cols={{ base: 1, md: cols }} spacing="lg">
        {sections.map((section) => (
          <Paper key={section.key} withBorder p="md" radius="md">
            <Stack gap="sm">
              <Title order={4}>{section.title}</Title>
              {section.content}
            </Stack>
          </Paper>
        ))}
      </SimpleGrid>
    )
  }

  return (
    <Accordion multiple defaultValue={defaultOpen ? [...defaultOpen] : [sections[0]?.key ?? '']}>
      {sections.map((section) => (
        <Accordion.Item key={section.key} value={section.key}>
          <Accordion.Control>{section.title}</Accordion.Control>
          <Accordion.Panel>{section.content}</Accordion.Panel>
        </Accordion.Item>
      ))}
    </Accordion>
  )
}
