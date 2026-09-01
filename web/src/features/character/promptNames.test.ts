import { describe, expect, it } from 'vitest'
import type { Prompt } from '@/lib/api'
import { testT } from '@/test/i18n'

import { choiceName } from './promptNames'

function prompt(over: Partial<Prompt> & { choice: Prompt['choice'] }): Prompt {
  return {
    group: 'race',
    optional: false,
    event: { type: 'race' },
    heldOnly: false,
    ...over,
  }
}

function of(kind: string, choose = 1, over: Partial<Prompt> = {}): Prompt {
  return prompt({
    choice: { prompt: `character/${kind}`, choose, kind, from: { kind: 'explicit' } },
    ...over,
  })
}

describe('choiceName', () => {
  it('names a choice rather than ordering one', () => {
    // "A race", not "Choose a race". The name heads the block whether or not
    // anybody presses it, and a list of imperatives reads as a list of orders.
    expect(choiceName(testT, of('race'))).toBe('A race')
    expect(choiceName(testT, of('background'))).toBe('A background')
    expect(choiceName(testT, of('subclass'))).toBe('An archetype')
    expect(choiceName(testT, of('bond'))).toBe('A bond')
  })

  // The counts used to be spelled out -- "Two more languages" -- and are
  // digits now. That is not a style change: a Russian numeral agrees in gender
  // with the noun it counts (два языка, две черты), so a shared table of
  // spelled numbers is a composition bug waiting for a translator. The plural
  // form of the *noun* still has to be right, which is what these pin.
  it('counts, and agrees with itself about the plural', () => {
    expect(choiceName(testT, of('language', 1))).toBe('1 more language')
    expect(choiceName(testT, of('language', 2))).toBe('2 more languages')
    expect(choiceName(testT, of('spell', 1))).toBe('1 spell')
    expect(choiceName(testT, of('spell', 9))).toBe('9 spells')
  })

  it('says which way a proficiency question runs', () => {
    expect(choiceName(testT, of('proficiency', 2))).toBe('2 to be proficient in')
    expect(choiceName(testT, of('proficiency', 2, { heldOnly: true }))).toBe(
      '2 to double your proficiency in',
    )
  })

  it('tells the two ability-score questions apart by what they offer', () => {
    // The six a character starts with offer nothing to pick between; the
    // improvement a level grants offers a choice like any other.
    expect(choiceName(testT, of('ability-scores', 6))).toBe('6 ability scores')
    expect(
      choiceName(
      testT,
      prompt({
          choice: {
            prompt: 'rogue/asi/4',
            choose: 1,
            kind: 'ability-scores',
            from: { kind: 'explicit', options: [{ key: 'feat', kind: 'ref', ref: 'feat:grappler' }] },
          },
        }),
      ),
    ).toBe('An improvement to your scores, or a feat')
  })

  it('names a kind it has never heard of rather than dropping it', () => {
    // The server may grow a kind before the browser does, and a choice you
    // cannot see is worse than a choice named plainly.
    expect(choiceName(testT, of('dark-gift', 1))).toBe('1 of Dark Gift')
  })
})
