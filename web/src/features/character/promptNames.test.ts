import { describe, expect, it } from 'vitest'

import type { Prompt } from '@/lib/api'

import { choiceName } from './promptNames'

function prompt(over: Partial<Prompt> & { choice: Prompt['choice'] }): Prompt {
  return {
    group: 'race',
    optional: false,
    advances: false,
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
    expect(choiceName(of('race'))).toBe('A race')
    expect(choiceName(of('background'))).toBe('A background')
    expect(choiceName(of('subclass'))).toBe('An archetype')
    expect(choiceName(of('bond'))).toBe('A bond')
  })

  it('counts in words, and agrees with itself about the plural', () => {
    expect(choiceName(of('language', 1))).toBe('One more language')
    expect(choiceName(of('language', 2))).toBe('Two more languages')
    expect(choiceName(of('spell', 4))).toBe('Four spells')
    // Past the words it has, it counts in numbers rather than in nothing.
    expect(choiceName(of('spell', 9))).toBe('9 spells')
  })

  it('says which way a proficiency question runs', () => {
    expect(choiceName(of('proficiency', 2))).toBe('Two to be proficient in')
    expect(choiceName(of('proficiency', 2, { heldOnly: true }))).toBe(
      'Two to double your proficiency in',
    )
  })

  it('tells the two ability-score questions apart by what they offer', () => {
    // The six a character starts with offer nothing to pick between; the
    // improvement a level grants offers a choice like any other.
    expect(choiceName(of('ability-scores', 6))).toBe('Six ability scores')
    expect(
      choiceName(
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
    expect(choiceName(of('dark-gift', 1))).toBe('One of Dark Gift')
  })
})
