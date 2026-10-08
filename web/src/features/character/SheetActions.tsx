import { useState } from 'react'

import { signed } from '@/domain'
import type { Entry, ResourcePool, SheetAction } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { Badge, BlockList, Button, Group, Markdown, Stack, Text, TextInput, joinProse } from '@/ui'

import { DEFAULT_ACTION_FILTERS, hasActionFilters, matchesActionFilters, toggled } from './filterActions'

const KINDS = {
  action: 'actions.kind.action',
  'bonus-action': 'actions.kind.bonusAction',
  reaction: 'actions.kind.reaction',
  'free-action': 'actions.kind.freeAction',
} as const

const CATEGORIES = {
  equipment: 'actions.category.equipment',
  feature: 'actions.category.feature',
  basic: 'actions.category.basic',
} as const

/** Buttons in the order the tables above are written, whatever order the sheet lists its actions in. */
const inOrder = (order: object) => (a: string, b: string) => Object.keys(order).indexOf(a) - Object.keys(order).indexOf(b)

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
 * is a dozen rows of one sheet, not a page worth a URL. It opens with the
 * basic actions switched off -- see `DEFAULT_ACTION_FILTERS`.
 */
export function SheetActions({ actions, entries, pools }: {
  actions: readonly SheetAction[]
  /** The prose behind a row, by its `origin`. Empty on a sheet a write echoed back. */
  entries: ReadonlyMap<string, Entry>
  pools: Readonly<Record<string, ResourcePool>>
}) {
  const t = useT()
  const [filters, setFilters] = useState(DEFAULT_ACTION_FILTERS)
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
      {/*
        One button per value, each pressed or not on its own, rather than a
        select: a turn is "my action and my bonus action", which a control
        that holds one value cannot say. Only what this sheet has is offered.
        Both sets share a line where there is room, the wider gap telling them
        apart, and wrap as two on a phone.
      */}
      <Group gap="md">
      <Group gap="xs" role="group" aria-label={t('actions.filter.kind')}>
        {present(actions.map((action) => action.kind)).sort(inOrder(KINDS)).map((kind) => (
          <FilterButton
            key={kind}
            label={kindName(kind)}
            on={!filters.offKinds.includes(kind)}
            onToggle={() => setFilters({ ...filters, offKinds: toggled(filters.offKinds, kind) })}
          />
        ))}
      </Group>
      <Group gap="xs" role="group" aria-label={t('actions.filter.category')}>
        {present(actions.map((action) => action.category)).filter((category) => category in CATEGORIES).sort(inOrder(CATEGORIES)).map((category) => (
          <FilterButton
            key={category}
            label={t(CATEGORIES[category as keyof typeof CATEGORIES])}
            on={!filters.offCategories.includes(category)}
            onToggle={() => setFilters({ ...filters, offCategories: toggled(filters.offCategories, category) })}
          />
        ))}
      </Group>
      </Group>
      <Group gap="sm" justify="space-between">
        <Text size="sm" c="dimmed" aria-live="polite">{t('actions.count', { count: visible.length })}</Text>
        {hasActionFilters(filters) && (
          <Button variant="subtle" onClick={() => setFilters(DEFAULT_ACTION_FILTERS)}>{t('prompt.resetSpellFilters')}</Button>
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

/** A filter value that is pressed while it is let through, in the app's pressed-button idiom. */
function FilterButton({ label, on, onToggle }: { label: string; on: boolean; onToggle: () => void }) {
  return (
    <Button size="xs" variant={on ? 'light' : 'default'} aria-pressed={on} onClick={onToggle}>
      {label}
    </Button>
  )
}
