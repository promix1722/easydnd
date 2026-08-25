import { Box, Card, Stack, Table, Text } from '@mantine/core'
import type { ReactNode } from 'react'

import { useIsDesktop } from './useIsDesktop'

/**
 * The widest a table gets on desktop.
 *
 * Every table here is narrow content -- a name, a rank, a couple of controls --
 * and letting one span a 2560px monitor puts its first and last cell an eye
 * movement apart.
 */
export const MAX_TABLE_WIDTH = 1024

export interface DataListColumn<T> {
  key: string
  header: string
  render: (item: T) => ReactNode
  /**
   * Marks the column that identifies the row. On mobile it becomes the card's
   * heading instead of another labelled field; exactly one column should set
   * it.
   */
  primary?: boolean
}

export interface DataListProps<T> {
  items: readonly T[]
  columns: ReadonlyArray<DataListColumn<T>>
  getKey: (item: T) => string
  empty?: ReactNode
  onSelect?: (item: T) => void
}

/**
 * A table on desktop, a stack of cards on mobile.
 *
 * A table that merely scrolls sideways on a phone is unreadable at the table
 * where this app is actually used, so the mobile rendering re-labels each cell
 * instead of shrinking it.
 */
export function DataList<T>({ items, columns, getKey, empty, onSelect }: DataListProps<T>) {
  const isDesktop = useIsDesktop()

  if (items.length === 0) {
    return (
      <Text c="dimmed" size="sm">
        {empty ?? 'Nothing here yet.'}
      </Text>
    )
  }

  if (isDesktop) {
    return (
      // Capped rather than full-bleed: a four-column table stretched across a
      // 2560px monitor puts its first and last cell an eye movement apart, and
      // every table in this app is narrow content. Set here so that no screen
      // has to remember to do it.
      <Box maw={MAX_TABLE_WIDTH}>
        <Table highlightOnHover={onSelect !== undefined} verticalSpacing="sm">
          <Table.Thead>
            <Table.Tr>
              {columns.map((column) => (
                <Table.Th key={column.key}>{column.header}</Table.Th>
              ))}
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {items.map((item) => (
              <Table.Tr
                key={getKey(item)}
                onClick={onSelect ? () => onSelect(item) : undefined}
                style={onSelect ? { cursor: 'pointer' } : undefined}
              >
                {columns.map((column) => (
                  <Table.Td key={column.key}>{column.render(item)}</Table.Td>
                ))}
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      </Box>
    )
  }

  const primary = columns.find((column) => column.primary)
  const rest = columns.filter((column) => !column.primary)

  return (
    <Stack gap="sm">
      {items.map((item) => (
        <Card
          key={getKey(item)}
          withBorder
          padding="md"
          onClick={onSelect ? () => onSelect(item) : undefined}
          style={onSelect ? { cursor: 'pointer' } : undefined}
        >
          {primary && (
            <Text fw={600} {...(rest.length > 0 ? { mb: 'xs' as const } : {})}>
              {primary.render(item)}
            </Text>
          )}
          <Stack gap={4}>
            {rest.map((column) => (
              <Text key={column.key} size="sm">
                <Text span c="dimmed" size="sm">
                  {column.header}:{' '}
                </Text>
                {column.render(item)}
              </Text>
            ))}
          </Stack>
        </Card>
      ))}
    </Stack>
  )
}
