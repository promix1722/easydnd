import { Accordion, Box, Group, Paper, SimpleGrid, Stack, Title } from '@mantine/core'
import type { ReactNode } from 'react'

import { useIsDesktop } from './useIsDesktop'

export interface ColumnsSection {
  key: string
  title: string
  content: ReactNode
  /**
   * A control belonging to the section as a whole, drawn on the title's own
   * line rather than inside the content.
   *
   * It is here rather than at the top of `content` because that is a
   * different claim: something in the content is *about* the content, and a
   * filter or a toggle is about the panel. Drawn as the first row of the body
   * it also leaves the title line empty across a whole panel width, which is
   * the layout this exists to avoid.
   */
  aside?: ReactNode
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
 * A character sheet is a dozen panels that all want to be visible at once on a
 * wide screen and would be a mile of scrolling on a phone. Collapsing is the
 * mobile answer, and putting it here keeps every sheet-shaped screen
 * consistent instead of each one inventing its own.
 */
export function Columns({ sections, cols = 2, defaultOpen }: ColumnsProps) {
  const isDesktop = useIsDesktop()

  if (isDesktop) {
    return (
      <SimpleGrid cols={{ base: 1, md: cols }} spacing="lg">
        {sections.map((section) => (
          <Paper key={section.key} withBorder p="md" radius="md">
            <Stack gap="sm">
              <Group justify="space-between" align="center" wrap="nowrap" gap="xs">
                <Title order={4}>{section.title}</Title>
                {section.aside}
              </Group>
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
          {/*
            The aside sits *beside* the control, not inside it. Accordion.Control
            is itself a button, and a button inside a button is neither valid
            markup nor clickable -- the outer one swallows the press. Mantine's
            own "accordion with controls" pattern is this: the control flexes,
            the sibling keeps its width.
          */}
          {section.aside === undefined ? (
            <Accordion.Control>{section.title}</Accordion.Control>
          ) : (
            <Group wrap="nowrap" gap={0} pr="md">
              <Accordion.Control>{section.title}</Accordion.Control>
              <Box style={{ flex: 'none' }}>{section.aside}</Box>
            </Group>
          )}
          <Accordion.Panel>{section.content}</Accordion.Panel>
        </Accordion.Item>
      ))}
    </Accordion>
  )
}
