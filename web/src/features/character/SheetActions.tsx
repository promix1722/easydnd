import { useState } from 'react'

import { signed } from '@/domain'
import type { Entry, ResourcePool, SheetAction } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { Badge, BlockList, Button, Group, Markdown, Select, Stack, Text, TextInput, joinProse } from '@/ui'

import { EMPTY_ACTION_FILTERS, hasActionFilters, matchesActionFilters } from './filterActions'

const KINDS = {
  action: 'actions.kind.action',
  'bonus-action': 'actions.kind.bonusAction',
  reaction: 'actions.kind.reaction',
  'free-action': 'actions.kind.freeAction',
} as const

const CATEGORIES = {
  basic: 'actions.category.basic',
  equipment: 'actions.category.equipment',
  feature: 'actions.category.feature',
} as const

/**
 * What the character can do on a turn, as a list that opens.
 *
 * The server decides what is on it -- an equipped weapon, an entry a rule pack
 * tagged as an action, an action the pack gives everybody -- and sends the
 * prose behind each row in the same response, keyed by the row's `origin`. So
 * nothing here knows a class, and opening a row asks nobody for anything.
 *
 * A row with no prose is a statement: a mundane weapon has numbers and no
 * description, and `BlockList` draws that as a fact rather than a control that
 * opens onto nothing.
 *
 * Filters are local state, as they are for a build's spell choices: the list
 * is a dozen rows of one sheet, not a page worth a URL.
 */
export function SheetActions({ actions, entries, pools }: {
  actions: readonly SheetAction[]
  /** The prose behind a row, by its `origin`. Empty on a sheet a write echoed back. */
  entries: ReadonlyMap<string, Entry>
  pools: Readonly<Record<string, ResourcePool>>
}) {
  const t = useT()
  const [filters, setFilters] = useState(EMPTY_ACTION_FILTERS)
  const [opened, setOpened] = useState<string | null>(null)
  const kindName = (kind: string) => (kind in KINDS ? t(KINDS[kind as keyof typeof KINDS]) : kind)

  if (actions.length === 0) return <Text size="sm" c="dimmed">{t('sheet.noActions')}</Text>

  const visible = actions.filter((action) => matchesActionFilters(action, filters))
  const present = (values: Array<string | undefined>) => [...new Set(values.filter((value): value is string => !!value))]

  return (
    <Stack gap="sm">
      <TextInput
        aria-label={t('actions.search')}
        placeholder={t('actions.search')}
        value={filters.query}
        onChange={(event) => setFilters({ ...filters, query: event.currentTarget.value })}
      />
      <Group gap="sm">
        <Select
          aria-label={t('actions.filter.kind')}
          placeholder={t('actions.filter.anyKind')}
          data={present(actions.map((action) => action.kind)).map((kind) => ({ value: kind, label: kindName(kind) }))}
          value={filters.kind}
          onChange={(kind) => setFilters({ ...filters, kind })}
          clearable
        />
        <Select
          aria-label={t('actions.filter.category')}
          placeholder={t('actions.filter.anyCategory')}
          data={present(actions.map((action) => action.category)).filter((category) => category in CATEGORIES)
            .map((category) => ({ value: category, label: t(CATEGORIES[category as keyof typeof CATEGORIES]) }))}
          value={filters.category}
          onChange={(category) => setFilters({ ...filters, category })}
          clearable
        />
      </Group>
      <Group gap="sm" justify="space-between">
        <Text size="sm" c="dimmed" aria-live="polite">{t('actions.count', { count: visible.length })}</Text>
        {hasActionFilters(filters) && (
          <Button variant="subtle" onClick={() => setFilters(EMPTY_ACTION_FILTERS)}>{t('prompt.resetSpellFilters')}</Button>
        )}
      </Group>
      {visible.length === 0 && <Text size="sm" c="dimmed">{t('actions.empty')}</Text>}
      <BlockList
        open={opened}
        onOpen={setOpened}
        items={visible.map((action, at) => {
          const key = action.origin ?? `${action.name}:${at}`
          const pool = action.uses ? pools[action.uses] : undefined
          const facts = [
            action.toHit === undefined ? '' : t('actions.toHit', { bonus: signed(action.toHit) }),
            action.damage,
            action.range ? t('vitals.feet', { distance: action.range }) : '',
            pool ? t('actions.uses', { name: pool.name, max: pool.max }) : '',
            action.notes,
          ].filter(Boolean).join(' · ')
          const desc = entries.get(key)?.desc
          return {
            key,
            header: (
              <Stack gap={2}>
                <Group gap="xs">
                  <Text fw={600}>{action.name}</Text>
                  <Badge variant="light">{kindName(action.kind)}</Badge>
                </Group>
                {facts && <Text size="sm" c="dimmed">{facts}</Text>}
              </Stack>
            ),
            body: desc?.length ? <Markdown size="sm">{joinProse(desc)}</Markdown> : undefined,
          }
        })}
      />
    </Stack>
  )
}
