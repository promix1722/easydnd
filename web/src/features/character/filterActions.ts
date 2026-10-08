import type { SheetAction } from '@/lib/api'

export interface ActionFilterValues {
  query: string
  /** The part of a turn: `action`, `bonus-action`, `reaction`, `free-action`. */
  kind: string | null
  /** Why the character has it: `basic`, `equipment`, `feature`. */
  category: string | null
}

export const EMPTY_ACTION_FILTERS: ActionFilterValues = { query: '', kind: null, category: null }

/** The sheet's whole action list is already here, so filtering it asks nobody. */
export function matchesActionFilters(action: SheetAction, filters: ActionFilterValues): boolean {
  return action.name.toLocaleLowerCase().includes(filters.query.trim().toLocaleLowerCase())
    && (filters.kind === null || action.kind === filters.kind)
    && (filters.category === null || action.category === filters.category)
}

export function hasActionFilters(filters: ActionFilterValues): boolean {
  return filters.query.trim() !== '' || filters.kind !== null || filters.category !== null
}
