import { useState } from 'react'

import type { GameEntry, ItemHit } from '@/lib/api'
import { grantItem } from '@/lib/api'
import { CatalogScope } from '@/lib/api/catalogScope'
import { useAction } from '@/lib/useAction'
import { useT } from '@/lib/i18n'
import { Alert, Select } from '@/ui'
import { AddItems } from '../character/ItemPicker'

/**
 * What a DM hands out, at the foot of the page: who gets it, then the sheet's
 * own item search over that character's catalogue. An Add closes the search:
 * a DM hands out one thing and goes back to the table. Nothing on the roster
 * shows an inventory, so the gift is said in the tracker's notice.
 */
export function GrantItems({ gameId, players, onGiven }: { gameId: string; players: readonly GameEntry[]; onGiven: (text: string) => void }) {
  const t = useT()
  const [chosen, setChosen] = useState<string | null>(null)
  const to = players.find((entry) => entry.id === chosen) ?? players[0]
  const grant = useAction(grantItem)
  // Remounting the search is what closes it.
  const [round, setRound] = useState(0)
  if (to?.character_id === undefined) return null
  const name = (entry: GameEntry) => entry.name || t('common.unnamed')
  async function give(hit: ItemHit) {
    if (to === undefined) return
    if (await grant.run(gameId, to.id, hit.slug) === null) return
    onGiven(t('game.gave', { item: hit.name, name: name(to) }))
    setRound((previous) => previous + 1)
  }
  // The search is over the receiver's own catalogue: their rule packs decide what exists for them.
  return <CatalogScope.Provider value={`/shared/${encodeURIComponent(to.character_id)}/catalog`}>
    <AddItems key={round} label={t('game.giveItem')} disabled={grant.pending} onAdd={(hit) => void give(hit)}
      detailsTo={(hit) => `/games/${encodeURIComponent(gameId)}/characters/${encodeURIComponent(to.character_id ?? '')}/items/${encodeURIComponent(hit.slug)}`}>
      <Select label={t('game.giveItemTo')} allowDeselect={false} value={to.id} onChange={setChosen}
        data={players.map((entry) => ({ value: entry.id, label: name(entry) }))} />
      {grant.error !== null && <Alert color="red" title={t('group.actionFailed')}>{grant.error}</Alert>}
    </AddItems>
  </CatalogScope.Provider>
}
