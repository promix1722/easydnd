import { Box, Card, Stack, Table, Text } from '@mantine/core'
import type { ReactNode } from 'react'

import { useIsDesktop } from './useIsDesktop'

import { useT } from '@/lib/i18n'

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
  const t = useT()
  const isDesktop = useIsDesktop()

  if (items.length === 0) {
    return (
      <Text c="dimmed" size="sm">
        {empty ?? t('dataList.empty')}
      </Text>
    )
  }

  if (isDesktop) {
    return (
      // Uncapped, deliberately. The cap used to live here, and it capped
      // tables and nothing else -- so the two screens that wanted a heading to
      // line up with the table beneath it copied the number by hand, and the
      // character sheet was never capped at all. `ui/Page` now caps the whole
      // content column; see CONTENT_MAX_WIDTH.
      <Box>
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
                {/* A column with no header gets no label, rather than a bare
                    colon on a line of its own. The actions column is the case:
                    it is headerless by design on desktop, where a column of
                    buttons needs no title, and the phone rendering was
                    dutifully writing out ": " for it. */}
                {column.header !== '' && (
                  <Text span c="dimmed" size="sm">
                    {column.header}:{' '}
                  </Text>
                )}
                {column.render(item)}
              </Text>
            ))}
          </Stack>
        </Card>
      ))}
    </Stack>
  )
}
