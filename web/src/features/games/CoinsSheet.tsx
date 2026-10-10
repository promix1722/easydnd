import { useState } from 'react'

import type { GameEntry } from '@/lib/api'
import { adjustCoins, getSharedSheet, getSheet, writeChanges } from '@/lib/api'
import { useResource } from '@/lib/useResource'
import { useAction } from '@/lib/useAction'
import { useT } from '@/lib/i18n'
import { Alert, Button, Group, ModalSheet, NumberInput, Stack, Text } from '@/ui'
import { COINS, setCoin } from '@/domain'

/**
 * A seated character's coins, and nothing else of theirs: the same dialog for
 * the DM and for its owner. Five rows, one coin under another, edited freely
 * and written once by Save -- a purse is counted, then agreed, and a write per
 * keystroke would be an entry in the character's log for every digit.
 *
 * The two write differently behind the one button. An owner sets their own
 * totals; a DM sends the difference, so coins the player spent in the same
 * moment are not put back.
 */
export function CoinsSheet({ gameId, entry, characterId, master, onClose }: {
  gameId: string; entry: GameEntry; characterId: string; master: boolean; onClose: () => void
}) {
  const t = useT()
  const sheet = useResource(`purse:${master}:${characterId}`, (signal) => master ? getSharedSheet(characterId, signal) : getSheet(characterId, signal))
  const [draft, setDraft] = useState<Record<string, number>>({})
  const purse = sheet.data?.equipment.purse ?? {}
  const changed = COINS.filter((coin) => draft[coin] !== undefined && draft[coin] !== (purse[coin] ?? 0))
  const save = useAction(async () => {
    if (!master) return writeChanges(characterId, changed.map((coin) => setCoin(coin, draft[coin] ?? 0)))
    for (const coin of changed) await adjustCoins(gameId, entry.id, coin, (draft[coin] ?? 0) - (purse[coin] ?? 0))
  })
  const error = save.error ?? sheet.error
  return <ModalSheet opened onClose={onClose} size="sm" title={t('game.coinsOf', { name: entry.name || t('common.unnamed') })}
    onSubmit={() => void save.run().then((result) => { if (result !== null) onClose(); else sheet.refresh() })}>
    <Stack gap="sm">
      {error !== null && <Alert color="red" title={t('group.actionFailed')}>{error}</Alert>}
      {COINS.map((coin) => (
        <Group key={coin} justify="space-between" wrap="nowrap">
          <Text size="sm">{t(`equipment.coin.${coin}`)}</Text>
          <NumberInput aria-label={t(`equipment.coin.${coin}`)} w={140} min={0} allowDecimal={false}
            disabled={sheet.data === null || save.pending} value={draft[coin] ?? purse[coin] ?? 0}
            onChange={(value) => setDraft((previous) => ({ ...previous, [coin]: Math.max(0, Math.trunc(Number(value)) || 0) }))} />
        </Group>
      ))}
      <Group justify="flex-end">
        <Button variant="default" onClick={onClose}>{t('common.cancel')}</Button>
        <Button type="submit" disabled={changed.length === 0} loading={save.pending}>{t('common.save')}</Button>
      </Group>
    </Stack>
  </ModalSheet>
}
