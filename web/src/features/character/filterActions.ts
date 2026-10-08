import type { SheetAction } from '@/lib/api'

export interface ActionFilterValues {
  query: string
  /** Parts of a turn switched off: `action`, `bonus-action`, `reaction`, `free-action`. */
  offKinds: readonly string[]
  /** Sources switched off: `basic`, `equipment`, `feature`. */
  offCategories: readonly string[]
}

/**
 * What a sheet opens on: everything the character has of their own. The basic
 * actions are the same fourteen rows on every sheet, so they start switched
 * off rather than standing between a player and their own abilities.
 *
 * The filters hold what is *off*, not what is on, so a kind a pack adds later
 * is shown without anybody listing it here.
 */
export const DEFAULT_ACTION_FILTERS: ActionFilterValues = { query: '', offKinds: [], offCategories: ['basic'] }

/** The sheet's whole action list is already here, so filtering it asks nobody. */
export function matchesActionFilters(action: SheetAction, filters: ActionFilterValues): boolean {
  return action.name.toLocaleLowerCase().includes(filters.query.trim().toLocaleLowerCase())
    && !filters.offKinds.includes(action.kind)
    && !filters.offCategories.includes(action.category ?? '')
}

/** Whether the filters have been moved from where a sheet opens. */
export function hasActionFilters(filters: ActionFilterValues): boolean {
  const same = (a: readonly string[], b: readonly string[]) => a.length === b.length && a.every((value) => b.includes(value))
  return filters.query.trim() !== ''
    || !same(filters.offKinds, DEFAULT_ACTION_FILTERS.offKinds)
    || !same(filters.offCategories, DEFAULT_ACTION_FILTERS.offCategories)
}

/** Switches one value on or off. */
export function toggled(off: readonly string[], value: string): string[] {
  return off.includes(value) ? off.filter((other) => other !== value) : [...off, value]
}
