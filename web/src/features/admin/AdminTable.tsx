import type { ReactNode } from 'react'
import { useEffect, useState } from 'react'

import { useT } from '@/lib/i18n'
import { Box, Button, DataList, Group, PageBody, Select, Stack, Text, TextInput, pageState } from '@/ui'

import type { DataListProps } from '@/ui'
import type { usePaged } from './usePaged'

/** One width for every filter, so the row does not re-wrap as values change. */
const FILTER_WIDTH = { w: 260, miw: 0, maw: '100%' } as const

/**
 * A text filter that commits through a short pause rather than per keystroke:
 * the committed value is the request, and one per letter would race.
 */
export function FilterBox({ label, value, onCommit }: {
  label: string
  value: string
  onCommit: (value: string) => void
}) {
  const [draft, setDraft] = useState(value)
  useEffect(() => {
    const trimmed = draft.trim()
    if (trimmed === value) return undefined
    const handle = setTimeout(() => onCommit(trimmed), 300)
    return () => clearTimeout(handle)
    // onCommit is recreated per render; the timer only needs draft and value.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [draft, value])

  return (
    <TextInput
      {...FILTER_WIDTH}
      aria-label={label}
      placeholder={label}
      value={draft}
      onChange={(event) => setDraft(event.currentTarget.value)}
    />
  )
}

/** A pick-one filter; cleared means "any", which is what `placeholder` says. */
export function FilterSelect({ label, placeholder, options, value, onChange }: {
  label: string
  placeholder: string
  options: { value: string; label: string }[]
  value: string | null
  onChange: (value: string | null) => void
}) {
  return (
    <Select
      {...FILTER_WIDTH}
      aria-label={label}
      placeholder={placeholder}
      data={options}
      value={value}
      onChange={onChange}
      clearable
    />
  )
}

/**
 * The half both admin tabs share: filters that never unmount, a count, the
 * table dimmed while a new answer is in flight, and "Load more".
 */
export function AdminTable<T>({ filters, paged, count, loadFailed, ...table }: {
  filters: ReactNode
  paged: ReturnType<typeof usePaged<T>>
  count: (total: number) => string
  loadFailed: string
} & Omit<DataListProps<T>, 'items'>) {
  const t = useT()
  const { page, rows } = paged
  const state = pageState(
    { data: page, error: paged.error, loading: paged.loading },
    { title: loadFailed, fallback: t('error.unknown'), onRetry: paged.reload },
  )

  return (
    <Stack gap="md">
      <Group gap="sm">{filters}</Group>
      <PageBody state={state}>
        {page !== null && (
          <Box aria-busy={paged.stale} style={{ opacity: paged.stale ? 0.55 : 1, transition: 'opacity 120ms' }}>
            <Stack gap="md">
              <Text size="sm" c="dimmed">{count(page.total)}</Text>
              <DataList items={rows} {...table} />
              {rows.length < page.total && (
                <Group justify="center">
                  <Button variant="light" loading={paged.loadingMore} onClick={() => void paged.loadMore()}>
                    {t('admin.loadMore')}
                  </Button>
                </Group>
              )}
            </Stack>
          </Box>
        )}
      </PageBody>
    </Stack>
  )
}
