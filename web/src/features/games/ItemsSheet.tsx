import { useState } from 'react'
import { Link } from 'react-router'

import type { Change, GameEntry, Item } from '@/lib/api'
import { bySlug, getSharedSheet, getSheet, giveItem, grantItem, writeChanges } from '@/lib/api'
import { useResource } from '@/lib/useResource'
import { useAction } from '@/lib/useAction'
import { useLocale, useT } from '@/lib/i18n'
import { Alert, Anchor, Button, Card, Group, ItemIcon, ModalSheet, Select, SHEET_COMBOBOX, Stack, Text } from '@/ui'
import { groupOf, mergeStacks, setTotal, titleCase } from '@/domain'

/**
 * A seated character's items over the game, as one of two lists that do one
 * thing each. Without `others` it is what is used up, with Use on every row:
 * written by the owner as the owner, and by a DM (`master`) through the game,
 * which is the only way a DM touches a character they do not own. With
 * `others` it is everything carried that the catalogue knows, with Transfer on
 * every row, under a "From" and a "To" that are always both shown. `from` is
 * the viewer's own seated characters, since only an owner gives a character's
 * items away; "To" is anybody else at the game, and never whoever "From" is.
 * `at` is the card it was opened on: the giver if it is the viewer's own, the
 * receiver otherwise. Either way one press is the whole errand: it closes the
 * dialog and the caller says what happened. Neither shows coins: those are
 * their own dialog.
 */
export function ItemsSheet({ gameId, from, others, at, master = false, onClose, onDone }: {
  gameId: string; from: readonly GameEntry[]; others?: readonly GameEntry[]; at?: GameEntry; master?: boolean
  onClose: () => void; onDone: (text: string) => void
}) {
  const t = useT()
  const locale = useLocale()
  const own = from.some((each) => each.id === at?.id)
  const [chosen, setChosen] = useState<string | null>(own ? at?.id ?? null : null)
  const [target, setTarget] = useState<string | null>(own ? null : at?.id ?? null)
  const giver = from.find((each) => each.id === chosen) ?? from[0]
  const receivers = (others ?? []).filter((each) => each.id !== giver?.id)
  const to = others === undefined ? undefined : receivers.find((each) => each.id === target) ?? receivers[0]
  const characterId = giver?.character_id ?? ''
  const sheet = useResource(`items:${master}:${locale}:${characterId}`,
    (signal) => master ? getSharedSheet(characterId, signal) : getSheet(characterId, signal))
  const use = useAction((changes: Change[]) => writeChanges(characterId, changes))
  const take = useAction(grantItem)
  const give = useAction(giveItem)
  const s = sheet.data
  const items = bySlug<Item>([...(s?.catalog?.magicItems ?? []), ...(s?.catalog?.equipment ?? [])])
  const named = (slug: string) => s?.catalogNames?.[`equipment:${slug}`] ?? items.get(slug)?.name ?? titleCase(slug)
  const who = (each: GameEntry | undefined) => each?.name || t('common.unnamed')
  // Only what is carried and not worn, and only what the catalogue knows: a custom item has no slug to move by.
  const rows = (s ? mergeStacks(s.equipment) : []).filter((row) => row.item !== undefined && row.count > row.equipped
    && (others !== undefined || groupOf(items.get(row.item)) === 'consumable'))
  const pending = use.pending || take.pending || give.pending
  const error = use.error ?? take.error ?? give.error ?? sheet.error
  const label = others === undefined ? t('equipment.use') : t('game.transfer')
  async function act(slug: string, name: string, total: number) {
    if (s === null || giver === undefined) return
    if (others === undefined) {
      const used = master ? take.run(gameId, giver.id, slug, -1) : use.run(setTotal(s.equipment, slug, total - 1))
      if (await used !== null) onDone(t('game.usedItem', { item: name }))
    } else if (to === undefined) {
      return
    } else if (await give.run(gameId, giver.id, to.id, slug) !== null) onDone(t('game.gave', { item: name, name: who(to) }))
  }
  return <ModalSheet opened onClose={onClose} size="lg"
    title={others === undefined ? t('game.useItemOf', { name: who(giver) }) : t('game.transferItem')}>
    <Stack gap="sm">
      {error !== null && <Alert color="red" title={t('group.actionFailed')}>{error}</Alert>}
      {others !== undefined && giver !== undefined && <Group gap="sm" grow align="flex-start">
        {/* Choosing the receiver as the giver swaps them: the two are never the same character. */}
        <Select label={t('game.transferFrom')} comboboxProps={SHEET_COMBOBOX} allowDeselect={false}
          value={giver.id} data={from.map((each) => ({ value: each.id, label: who(each) }))}
          onChange={(next) => { if (next === to?.id) setTarget(giver.id); setChosen(next) }} />
        <Select label={t('game.transferTo')} comboboxProps={SHEET_COMBOBOX} allowDeselect={false}
          value={to?.id ?? null} onChange={setTarget} data={receivers.map((each) => ({ value: each.id, label: who(each) }))} />
      </Group>}
      {s && rows.length === 0 && <Text size="sm" c="dimmed">{t('sheet.empty')}</Text>}
      {s && rows.map((row) => {
        const slug = row.item ?? ''
        const name = named(slug)
        const carried = row.count - row.equipped
        return <Card key={row.key} withBorder radius="md" padding="xs">
          <Group gap="sm" wrap="nowrap">
            <ItemIcon icon={items.get(slug)?.icon} />
            <Group gap={8} style={{ flex: 1, minWidth: 0 }}>
              <Anchor component={Link} size="sm" fw={600} style={{ overflowWrap: 'anywhere' }}
                to={`/games/${encodeURIComponent(gameId)}/characters/${encodeURIComponent(characterId)}/items/${encodeURIComponent(slug)}`}>{name}</Anchor>
              {carried > 1 && <Text size="sm" c="dimmed">×{carried}</Text>}
            </Group>
            <Button variant="light" disabled={pending || (others !== undefined && to === undefined)} style={{ flexShrink: 0 }}
              aria-label={t('list.rowAction', { label, name })} onClick={() => void act(slug, name, row.count)}>{label}</Button>
          </Group>
        </Card>
      })}
    </Stack>
  </ModalSheet>
}
