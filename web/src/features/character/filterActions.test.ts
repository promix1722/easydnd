import { describe, expect, it } from 'vitest'

import { EMPTY_ACTION_FILTERS, hasActionFilters, matchesActionFilters } from './filterActions'

const dash = { source: 'derived', origin: 'action:dash', kind: 'action', category: 'basic', name: 'Dash' }
const wind = { source: 'derived', origin: 'feature:second-wind', kind: 'bonus-action', category: 'feature', name: 'Second Wind' }

describe('action filters', () => {
  it('lets everything through when empty', () => {
    expect(hasActionFilters(EMPTY_ACTION_FILTERS)).toBe(false)
    expect([dash, wind].filter((a) => matchesActionFilters(a, EMPTY_ACTION_FILTERS))).toHaveLength(2)
  })

  it('narrows by name, by part of the turn and by where the action comes from', () => {
    const only = (filters: Partial<typeof EMPTY_ACTION_FILTERS>) =>
      [dash, wind].filter((a) => matchesActionFilters(a, { ...EMPTY_ACTION_FILTERS, ...filters })).map((a) => a.name)
    expect.soft(only({ query: ' wIn ' })).toEqual(['Second Wind'])
    expect.soft(only({ kind: 'action' })).toEqual(['Dash'])
    expect.soft(only({ category: 'feature' })).toEqual(['Second Wind'])
    expect.soft(only({ kind: 'action', category: 'feature' })).toEqual([])
    expect.soft(hasActionFilters({ ...EMPTY_ACTION_FILTERS, kind: 'reaction' })).toBe(true)
  })
})
