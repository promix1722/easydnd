import { describe, expect, it } from 'vitest'

import { DEFAULT_ACTION_FILTERS, hasActionFilters, matchesActionFilters, toggled } from './filterActions'

const dash = { source: 'derived', origin: 'action:dash', kind: 'action', category: 'basic', name: 'Dash' }
const sword = { source: 'derived', origin: 'item:longsword', kind: 'action', category: 'equipment', name: 'Longsword' }
const wind = { source: 'derived', origin: 'feature:second-wind', kind: 'bonus-action', category: 'feature', name: 'Second Wind' }

const ALL = { ...DEFAULT_ACTION_FILTERS, offCategories: [] }
const only = (filters: typeof DEFAULT_ACTION_FILTERS) =>
  [dash, sword, wind].filter((a) => matchesActionFilters(a, filters)).map((a) => a.name)

describe('action filters', () => {
  it('opens on the character\'s own actions, with the basic ones switched off', () => {
    expect.soft(hasActionFilters(DEFAULT_ACTION_FILTERS)).toBe(false)
    expect.soft(only(DEFAULT_ACTION_FILTERS)).toEqual(['Longsword', 'Second Wind'])
    expect.soft(only(ALL)).toEqual(['Dash', 'Longsword', 'Second Wind'])
    expect.soft(hasActionFilters(ALL)).toBe(true)
  })

  it('switches each part of the turn and each source off on its own', () => {
    expect.soft(only({ ...ALL, query: ' wIn ' })).toEqual(['Second Wind'])
    expect.soft(only({ ...ALL, offKinds: ['action'] })).toEqual(['Second Wind'])
    expect.soft(only({ ...ALL, offCategories: ['feature', 'equipment'] })).toEqual(['Dash'])
    expect.soft(only({ ...ALL, offKinds: ['bonus-action'], offCategories: ['basic'] })).toEqual(['Longsword'])
  })

  it('toggles a value in and out', () => {
    expect.soft(toggled(['basic'], 'basic')).toEqual([])
    expect.soft(toggled(['basic'], 'feature')).toEqual(['basic', 'feature'])
  })
})
