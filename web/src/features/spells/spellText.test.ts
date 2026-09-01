import { describe, expect, it } from 'vitest'

import { testT } from '@/test/i18n'

import {
  castingTimeText,
  componentsAbbrev,
  durationText,
  levelText,
  rangeText,
} from './spellText'

describe('spellText', () => {
  it('renders every casting time kind', () => {
    expect(castingTimeText(testT, { kind: 'action', amount: 1 })).toBe('1 action')
    expect(castingTimeText(testT, { kind: 'bonus-action', amount: 1 })).toBe('1 bonus action')
    expect(castingTimeText(testT, { kind: 'reaction', amount: 1 })).toBe('1 reaction')
    expect(castingTimeText(testT, { kind: 'over-time', amount: 10, unit: 'minute' })).toBe(
      '10 minutes',
    )
    expect(castingTimeText(testT, undefined)).toBe('')
  })

  it('renders every range kind', () => {
    expect(rangeText(testT, { kind: 'self' })).toBe('Self')
    expect(rangeText(testT, { kind: 'touch' })).toBe('Touch')
    expect(rangeText(testT, { kind: 'distance', distance: 1 })).toBe('1 foot')
    expect(rangeText(testT, { kind: 'distance', distance: 90 })).toBe('90 feet')
    expect(rangeText(testT, { kind: 'sight' })).toBe('Sight')
    expect(rangeText(testT, { kind: 'unlimited' })).toBe('Unlimited')
    expect(rangeText(testT, { kind: 'special' })).toBe('Special')
  })

  it('renders every duration kind, wrapping "up to"', () => {
    expect(durationText(testT, { kind: 'instantaneous' })).toBe('Instantaneous')
    expect(durationText(testT, { kind: 'timed', amount: 1, unit: 'hour' })).toBe('1 hour')
    expect(durationText(testT, { kind: 'timed', amount: 1, unit: 'minute', upTo: true })).toBe(
      'Up to 1 minute',
    )
    expect(durationText(testT, { kind: 'timed', amount: 8, unit: 'hour' })).toBe('8 hours')
    expect(durationText(testT, { kind: 'timed', amount: 7, unit: 'day' })).toBe('7 days')
    expect(durationText(testT, { kind: 'timed', amount: 1, unit: 'round' })).toBe('1 round')
    expect(durationText(testT, { kind: 'until-dispelled' })).toBe('Until dispelled')
    expect(durationText(testT, { kind: 'special' })).toBe('Special')
  })

  it('falls back to the raw kind for one it has not heard of', () => {
    expect(rangeText(testT, { kind: 'astral' })).toBe('astral')
  })

  it('abbreviates components in V, S, M order', () => {
    expect(componentsAbbrev(testT, { verbal: true, somatic: true, material: true })).toBe('V, S, M')
    expect(componentsAbbrev(testT, { verbal: true })).toBe('V')
    expect(componentsAbbrev(testT, {})).toBe('')
    expect(componentsAbbrev(testT, undefined)).toBe('')
  })

  it('names cantrips and levels', () => {
    expect(levelText(testT, 0)).toBe('Cantrip')
    expect(levelText(testT, 3)).toBe('Level 3')
  })
})
