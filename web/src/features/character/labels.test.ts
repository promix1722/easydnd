import { describe, expect, it } from 'vitest'

import { testT } from '@/test/i18n'

import { describeChange, eventLabel, formatAt, formatValue } from './labels'

/**
 * These moved here with the tables they exercise.
 *
 * They were in `src/domain/`, which is where the pure helpers they sit beside
 * still are -- but every assertion below is about a *word*, and the words left
 * that directory when the app learned a second language. What they check is
 * unchanged: that each table is complete, that an unknown key is drawn rather
 * than refused, and that a value prints the way the wire spells it.
 *
 * `testT` is a real English i18next instance, so an assertion here is against
 * what somebody actually reads rather than against a key.
 */
describe('eventLabel', () => {
  it('reads every event type back as prose', () => {
    expect(eventLabel(testT, 'init')).toBe('Created')
    expect(eventLabel(testT, 'change')).toBe('Adjusted')
    expect(eventLabel(testT, 'race')).toBe('Race chosen')
    expect(eventLabel(testT, 'subrace')).toBe('Subrace chosen')
    expect(eventLabel(testT, 'background')).toBe('Background chosen')
    expect(eventLabel(testT, 'class')).toBe('Class chosen')
    expect(eventLabel(testT, 'subclass')).toBe('Subclass chosen')
    expect(eventLabel(testT, 'level')).toBe('Level gained')
    expect(eventLabel(testT, 'feat')).toBe('Feat taken')
    expect(eventLabel(testT, 'note')).toBe('Note')
  })

  // The server may learn an event kind before this client does, and a log that
  // refuses to draw the one event you opened it for is worse than a plain one.
  it('draws an unknown type rather than refusing it', () => {
    expect(eventLabel(testT, 'multiclass')).toBe('Multiclass')
  })
})

describe('formatValue', () => {
  it('prints every kind the wire carries', () => {
    expect(formatValue(testT, { kind: 'int', int: 15 })).toBe('15')
    expect(formatValue(testT, { kind: 'string', string: 'Zephyr' })).toBe('Zephyr')
    expect(formatValue(testT, { kind: 'bool', bool: true })).toBe('yes')
    expect(formatValue(testT, { kind: 'bool', bool: false })).toBe('no')
    expect(formatValue(testT, { kind: 'slug', slug: 'point-buy' })).toBe('Point Buy')
    expect(formatValue(testT, { kind: 'slugs', slugs: ['dex', 'con'] })).toBe('Dex, Con')
    expect(formatValue(testT, { kind: 'dice', dice: '1d8' })).toBe('1d8')
    expect(formatValue(testT, { kind: 'none' })).toBe('')
    expect(formatValue(testT, undefined)).toBe('')
  })
})

describe('describeChange', () => {
  it('reads a change as the address, the operation and the value', () => {
    expect(
      describeChange(testT, { path: 'abilities.dex', op: 'set', value: { kind: 'int', int: 15 } }),
    ).toBe('abilities.dex set 15')
  })

  it('leaves the value off when there is none to print', () => {
    expect(describeChange(testT, { path: 'feats', op: 'remove', value: { kind: 'none' } })).toBe(
      'feats remove',
    )
  })
})

describe('formatAt', () => {
  // Not defensive: Service.Create seeds the init event without stamping At, so
  // the first event of every character arrives with no time.
  it('has something to print when an event has no time', () => {
    expect(formatAt('en', undefined)).toBe('--')
    expect(formatAt('en', '')).toBe('--')
  })

  it('renders a stamped time in the local timezone', () => {
    // Against the app's locale, not the browser's -- which is the whole reason
    // this took a locale when it moved. `toLocaleString('en')` is what a reader
    // who chose English sees, whatever the machine is configured for.
    expect(formatAt('en', '2026-08-23T21:26:44Z')).toBe(
      new Date('2026-08-23T21:26:44Z').toLocaleString('en'),
    )
  })

  // Losing it would hide exactly the kind of bad record this page exists for.
  it('prints an unparseable time as it arrived', () => {
    expect(formatAt('en', 'yesterday')).toBe('yesterday')
  })
})
