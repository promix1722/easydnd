import type { GameEntry } from '@/lib/api'

/** An entry's pools in the shape the sheet's own panel draws. */
export const poolsOf = (entry: GameEntry) =>
  (entry.resources ?? []).map(({ slot_level, ...pool }) => ({ ...pool, ...(slot_level ? { slotLevel: slot_level } : {}) }))
