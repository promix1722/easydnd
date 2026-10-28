import type { GameEntry, ItemHit } from '@/lib/api'
import { grantItem } from '@/lib/api'
import { CatalogScope } from '@/lib/api/catalogScope'
import { useAction } from '@/lib/useAction'
import { useT } from '@/lib/i18n'
import { Alert, ModalSheet, Stack } from '@/ui'
import { AddItems } from '../character/ItemPicker'

/**
 * What a DM hands one player, from that player's "…" menu: the sheet's own
 * item search over that character's catalogue, in a dialog named for them. An
 * Add closes it: a DM hands out one thing and goes back to the table. Nothing
 * on the roster shows an inventory, so the gift is said in the tracker's notice.
 */
export function GiveItemSheet({ gameId, entry, characterId, onClose, onDone }: {
  gameId: string; entry: GameEntry; characterId: string; onClose: () => void; onDone: (text: string) => void
}) {
  const t = useT()
  const grant = useAction(grantItem)
  const name = entry.name || t('common.unnamed')
  async function give(hit: ItemHit) {
    if (await grant.run(gameId, entry.id, hit.slug) === null) return
    onDone(t('game.gave', { item: hit.name, name }))
  }
  // The search is over the receiver's own catalogue: their rule packs decide what exists for them.
  return <ModalSheet opened onClose={onClose} size="lg" title={t('game.giveItemTo', { name })}>
    <Stack gap="sm">
      {grant.error !== null && <Alert color="red" title={t('group.actionFailed')}>{grant.error}</Alert>}
      <CatalogScope.Provider value={`/shared/${encodeURIComponent(characterId)}/catalog`}>
        <AddItems label={t('game.giveItem')} disabled={grant.pending} onAdd={(hit) => void give(hit)} onClose={onClose}
          detailsTo={(hit) => `/games/${encodeURIComponent(gameId)}/characters/${encodeURIComponent(characterId)}/items/${encodeURIComponent(hit.slug)}`} />
      </CatalogScope.Provider>
    </Stack>
  </ModalSheet>
}
