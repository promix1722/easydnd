import { describe, expect, it } from 'vitest'

import {
  collectionOfKind,
  describeChange,
  eventLabel,
  formatAt,
  formatValue,
  pickLabel,
  promptLabel,
} from './index'

describe('eventLabel', () => {
  it('reads every event type back as prose', () => {
    expect(eventLabel('init')).toBe('Created')
    expect(eventLabel('change')).toBe('Adjusted')
    expect(eventLabel('race')).toBe('Race chosen')
    expect(eventLabel('subrace')).toBe('Subrace chosen')
    expect(eventLabel('background')).toBe('Background chosen')
    expect(eventLabel('class')).toBe('Class chosen')
    expect(eventLabel('subclass')).toBe('Subclass chosen')
    expect(eventLabel('level')).toBe('Level gained')
    expect(eventLabel('feat')).toBe('Feat taken')
    expect(eventLabel('note')).toBe('Note')
  })

  // The server may learn an event kind before this client does, and a log that
  // refuses to draw the one event you opened it for is worse than a plain one.
  it('draws an unknown type rather than refusing it', () => {
    expect(eventLabel('multiclass')).toBe('Multiclass')
  })
})

describe('collectionOfKind', () => {
  it('names the collection that would name a reference', () => {
    expect(collectionOfKind('race')).toBe('races')
    expect(collectionOfKind('class')).toBe('classes')
    expect(collectionOfKind('feat')).toBe('feats')
  })

  // The same table now answers the build screen's other question -- which
  // collection a prompt's options come out of -- so the kinds a log never
  // stores are in it too. It used to be two tables in two files, and the one
  // missing an entry was whichever one you were not looking at.
  it('covers the kinds only a prompt names', () => {
    expect(collectionOfKind('language')).toBe('languages')
    expect(collectionOfKind('item')).toBe('equipment')
    expect(collectionOfKind('damage-type')).toBe('damage-types')
  })

  it('is null where the compendium serves no collection', () => {
    expect(collectionOfKind('multiclass')).toBeNull()
    expect(collectionOfKind('')).toBeNull()
  })
})

describe('promptLabel', () => {
  it('drops the namespace the character poses its own prompts under', () => {
    expect(promptLabel('character/race')).toBe('Race')
    expect(promptLabel('character/alignment')).toBe('Alignment')
  })

  // The index distinguishes one repeat of a choice from the next; the source
  // stays, because "Proficiency" alone does not say which grant is answered.
  it('keeps the source and drops the index', () => {
    expect(promptLabel('rogue/proficiency/0')).toBe('Rogue · Proficiency')
    expect(promptLabel('skill-versatility/proficiency/0')).toBe('Skill Versatility · Proficiency')
    expect(promptLabel('half-elf/ability-bonus/0')).toBe('Half Elf · Ability Bonus')
  })
})

describe('pickLabel', () => {
  it('title-cases an ordinary option key', () => {
    expect(pickLabel('skill-stealth')).toBe('Skill Stealth')
    expect(pickLabel('dex')).toBe('Dex')
  })

  // A nested option is named by a path, and its own answer is in the same
  // event, so naming the branch is enough.
  it('names the branch a nested option took', () => {
    expect(pickLabel('rogue-expertise-1/expertise/0/0')).toBe('Expertise')
  })
})

describe('formatValue', () => {
  it('prints every kind the wire carries', () => {
    expect(formatValue({ kind: 'int', int: 15 })).toBe('15')
    expect(formatValue({ kind: 'string', string: 'Zephyr' })).toBe('Zephyr')
    expect(formatValue({ kind: 'bool', bool: true })).toBe('yes')
    expect(formatValue({ kind: 'bool', bool: false })).toBe('no')
    expect(formatValue({ kind: 'slug', slug: 'point-buy' })).toBe('Point Buy')
    expect(formatValue({ kind: 'slugs', slugs: ['dex', 'con'] })).toBe('Dex, Con')
    expect(formatValue({ kind: 'dice', dice: '1d8' })).toBe('1d8')
    expect(formatValue({ kind: 'none' })).toBe('')
    expect(formatValue(undefined)).toBe('')
  })
})

describe('describeChange', () => {
  it('reads a change as the address, the operation and the value', () => {
    expect(
      describeChange({ path: 'abilities.dex', op: 'set', value: { kind: 'int', int: 15 } }),
    ).toBe('abilities.dex set 15')
  })

  it('leaves the value off when there is none to print', () => {
    expect(describeChange({ path: 'feats', op: 'remove', value: { kind: 'none' } })).toBe(
      'feats remove',
    )
  })
})

describe('formatAt', () => {
  // Not defensive: Service.Create seeds the init event without stamping At, so
  // the first event of every character arrives with no time.
  it('has something to print when an event has no time', () => {
    expect(formatAt(undefined)).toBe('--')
    expect(formatAt('')).toBe('--')
  })

  it('renders a stamped time in the local timezone', () => {
    expect(formatAt('2026-08-23T21:26:44Z')).toBe(new Date('2026-08-23T21:26:44Z').toLocaleString())
  })

  // Losing it would hide exactly the kind of bad record this page exists for.
  it('prints an unparseable time as it arrived', () => {
    expect(formatAt('yesterday')).toBe('yesterday')
  })
})
