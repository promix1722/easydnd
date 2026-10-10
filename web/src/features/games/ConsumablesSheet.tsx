import { useState } from 'react'

import type { GameEntry } from '@/lib/api'
import { patchGameEntry } from '@/lib/api'
import { useAction } from '@/lib/useAction'
import { useT } from '@/lib/i18n'
import { Alert, ModalSheet, Stack } from '@/ui'
import { ResourcePools } from '../character/ResourcePools'

/**
 * One entry's consumables, in a dialog of their own. A press is drawn at once
 * from a local count and saved behind it, with its own action: sharing the
 * roster's would grey every control on the page for the length of a request.
 */
export function ConsumablesSheet({ gameId, entry, onClose, onChange }: {
  gameId: string; entry: GameEntry; onClose: () => void; onChange: () => void
}) {
  const t = useT()
  const pools = (entry.resources ?? []).map(({ slot_level, ...pool }) => ({ ...pool, ...(slot_level ? { slotLevel: slot_level } : {}) }))
  const save = useAction(patchGameEntry)
  const [local, setLocal] = useState<Record<string, number>>({})
  // A local count has done its job once the server reports the same one; kept
  // longer it would hide a long rest called while the dialog is open.
  const settled = pools.filter((pool) => local[pool.id] === pool.used)
  if (settled.length > 0) setLocal(Object.fromEntries(Object.entries(local).filter(([id]) => !settled.some((pool) => pool.id === id))))
  const forget = (id: string) => setLocal((previous) => Object.fromEntries(Object.entries(previous).filter(([key]) => key !== id)))
  async function spend(id: string, used: number) {
    setLocal((previous) => ({ ...previous, [id]: used }))
    if (await save.run(gameId, entry.id, { used: { [id]: used } }) === null) forget(id)
    else onChange()
  }
  return <ModalSheet opened onClose={onClose} title={t('sheet.consumables')}>
    <Stack gap="sm">
      {save.error !== null && <Alert color="red" title={t('group.actionFailed')}>{save.error}</Alert>}
      <ResourcePools pools={pools.map((pool) => ({ ...pool, used: local[pool.id] ?? pool.used }))}
        {...(entry.can_edit ? { onChange: (id: string, used: number) => void spend(id, used) } : {})} />
    </Stack>
  </ModalSheet>
}
