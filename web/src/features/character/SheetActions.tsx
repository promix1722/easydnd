import { useState } from 'react'

import { signed } from '@/domain'
import type { Entry, ResourcePool, SheetAction } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { Accordion, Badge, BlockList, Group, Markdown, Stack, Text, joinProse } from '@/ui'

import { WeaponStats } from './WeaponStats'

const KINDS = {
  action: 'actions.kind.action',
  'bonus-action': 'actions.kind.bonusAction',
  reaction: 'actions.kind.reaction',
  'free-action': 'actions.kind.freeAction',
} as const

/** Where an action comes from, in the order the groups are drawn. */
const CATEGORIES = {
  equipment: 'actions.category.equipment',
  feature: 'actions.category.feature',
  basic: 'actions.category.basic',
} as const
type Category = keyof typeof CATEGORIES

/**
 * What the character can do on a turn, as groups that fold, of rows that open.
 *
 * The server decides what is on it -- an equipped weapon, an entry a rule pack
 * tagged as an action, an action the pack gives everybody -- and sends the
 * prose behind each row in the same response, keyed by the row's `origin`. So
 * nothing here knows a class, and opening a row asks nobody for anything.
 *
 * One group per source, each folded or not on its own, and every one folded
 * to begin with: the tab opens as three headings with their counts, and the
 * player unfolds the one they came for. There is no search and no filter: a
 * sheet has a dozen actions of its own, and the groups are the only question
 * worth asking of them.
 *
 * A row with no prose is a statement: a mundane weapon has numbers and no
 * description, and `BlockList` draws that as a fact rather than a control that
 * opens onto nothing.
 */
export function SheetActions({ actions, entries, pools }: {
  actions: readonly SheetAction[]
  /** The prose behind a row, by its `origin`. Empty on a sheet a write echoed back. */
  entries: ReadonlyMap<string, Entry>
  pools: Readonly<Record<string, ResourcePool>>
}) {
  const t = useT()
  const [unfolded, setUnfolded] = useState<string[]>([])
  const [opened, setOpened] = useState<string | null>(null)
  const kindName = (kind: string) => (kind in KINDS ? t(KINDS[kind as keyof typeof KINDS]) : kind)

  if (actions.length === 0) return <Text size="sm" c="dimmed">{t('sheet.noActions')}</Text>

  // An action with no source, or one this table does not know, is the character's own.
  const categoryOf = (action: SheetAction): Category => action.category !== undefined && action.category in CATEGORIES ? action.category as Category : 'feature'
  const groups = (Object.keys(CATEGORIES) as Category[])
    .map((category) => ({ category, rows: actions.filter((action) => categoryOf(action) === category) }))
    .filter((group) => group.rows.length > 0)

  return (
    <Accordion multiple value={unfolded} onChange={setUnfolded}>
      {groups.map(({ category, rows }) => (
        <Accordion.Item key={category} value={category}>
          <Accordion.Control>
            <Group gap="xs">
              <Text fw={600}>{t(CATEGORIES[category])}</Text>
              <Text size="sm" c="dimmed">{rows.length}</Text>
            </Group>
          </Accordion.Control>
          {/* Mounted only while unfolded, as a BlockList body is: a folded group is not on the page. */}
          <Accordion.Panel>{unfolded.includes(category) && (
      <BlockList
        outlined
        open={opened}
        onOpen={setOpened}
        items={rows.map((action, at) => {
          const key = action.origin ?? `${action.name}:${at}`
          const pool = action.uses ? pools[action.uses] : undefined
          const facts = [
            pool ? t('actions.uses', { name: pool.name, max: pool.max }) : '',
            action.notes,
          ].filter(Boolean).join(' · ')
          const desc = entries.get(key)?.desc
          return {
            key,
            header: (
              // Wraps: on a phone a name and three figures do not share a line,
              // and squeezed on to one they are drawn over each other.
              <Group gap="xs" justify="space-between">
                <Stack gap={2} style={{ minWidth: 0 }}>
                  <Group gap="xs">
                    <Text fw={600}>{action.name}</Text>
                    {/* An action is the default and says nothing; only the other parts of a turn are marked. */}
                    {action.kind !== 'action' && <Badge variant="light">{kindName(action.kind)}</Badge>}
                  </Group>
                  {facts && <Text size="sm" c="dimmed">{facts}</Text>}
                </Stack>
                <WeaponStats
                  damage={action.damage}
                  hit={action.toHit === undefined ? undefined : signed(action.toHit)}
                  range={action.range ? t('vitals.feet', { distance: action.range }) : undefined}
                />
              </Group>
            ),
            body: desc?.length ? <Markdown size="sm">{joinProse(desc)}</Markdown> : undefined,
          }
        })}
      />
          )}</Accordion.Panel>
        </Accordion.Item>
      ))}
    </Accordion>
  )
}
