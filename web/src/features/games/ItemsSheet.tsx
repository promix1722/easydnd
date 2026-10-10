import { useState } from 'react'
import { Link } from 'react-router'

import type { Change, GameEntry, Item } from '@/lib/api'
import { bySlug, getSheet, giveItem, writeChanges } from '@/lib/api'
import { useResource } from '@/lib/useResource'
import { useAction } from '@/lib/useAction'
import { useLocale, useT } from '@/lib/i18n'
import { Alert, Anchor, Button, Card, Group, ItemIcon, ModalSheet, Select, SHEET_COMBOBOX, Stack, Text } from '@/ui'
import { groupOf, mergeStacks, setTotal, titleCase } from '@/domain'

/**
 * A player's own items over the game, as one of two lists that do one thing
 * each. Without `to` it is what is used up, with Use on every row. With `to`
 * it is everything carried that the catalogue knows, with Transfer on every
 * row -- opened from the *receiver's* card, so who it goes to was already said
 * by where it was pressed. Either way one press is the whole errand: it
 * closes the dialog and the tracker says what happened. `from` is the player's own seated
 * characters to take from, never the receiver itself; a transfer always names
 * its source in a "From" field, a choice when there is more than one.
 * Neither shows coins: those are their own dialog.
 */
export function ItemsSheet({ gameId, from, to, onClose, onDone }: {
  gameId: string; from: readonly GameEntry[]; to?: GameEntry; onClose: () => void; onDone: (text: string) => void
}) {
  const t = useT()
  const locale = useLocale()
  const [chosen, setChosen] = useState<string | null>(null)
  const giver = from.find((each) => each.id === chosen) ?? from[0]
  const characterId = giver?.character_id ?? ''
  const sheet = useResource(`items:${locale}:${characterId}`, (signal) => getSheet(characterId, signal))
  const use = useAction((changes: Change[]) => writeChanges(characterId, changes))
  const give = useAction(giveItem)
  const s = sheet.data
  const items = bySlug<Item>([...(s?.catalog?.magicItems ?? []), ...(s?.catalog?.equipment ?? [])])
  const named = (slug: string) => s?.catalogNames?.[`equipment:${slug}`] ?? items.get(slug)?.name ?? titleCase(slug)
  const who = (each: GameEntry | undefined) => each?.name || t('common.unnamed')
  // Only what is carried and not worn, and only what the catalogue knows: a custom item has no slug to move by.
  const rows = (s ? mergeStacks(s.equipment) : []).filter((row) => row.item !== undefined && row.count > row.equipped
    && (to !== undefined || groupOf(items.get(row.item)) === 'consumable'))
  const pending = use.pending || give.pending
  const error = use.error ?? give.error ?? sheet.error
  const label = to === undefined ? t('equipment.use') : t('game.transfer')
  async function act(slug: string, name: string, total: number) {
    if (s === null || giver === undefined) return
    if (to === undefined) {
      if (await use.run(setTotal(s.equipment, slug, total - 1)) !== null) onDone(t('game.usedItem', { item: name }))
    } else if (await give.run(gameId, giver.id, to.id, slug) !== null) onDone(t('game.gave', { item: name, name: who(to) }))
  }
  return <ModalSheet opened onClose={onClose} size="lg"
    title={to === undefined ? t('game.useItemOf', { name: who(giver) }) : t('game.transferItemOf', { name: who(to) })}>
    <Stack gap="sm">
      {error !== null && <Alert color="red" title={t('group.actionFailed')}>{error}</Alert>}
      {to !== undefined && giver !== undefined && <Select label={t('game.transferFrom')} comboboxProps={SHEET_COMBOBOX} allowDeselect={false}
        value={giver.id} onChange={setChosen} data={from.map((each) => ({ value: each.id, label: who(each) }))} />}
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
            <Button variant="light" disabled={pending} style={{ flexShrink: 0 }}
              aria-label={t('list.rowAction', { label, name })} onClick={() => void act(slug, name, row.count)}>{label}</Button>
          </Group>
        </Card>
      })}
    </Stack>
  </ModalSheet>
}
