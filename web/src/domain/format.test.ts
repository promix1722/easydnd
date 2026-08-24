import { describe, expect, it } from 'vitest'

import { classLine, kindOf, signed, slugOf, titleCase } from './index'

describe('signed', () => {
  it('prints a bonus the way a sheet does', () => {
    expect(signed(3)).toBe('+3')
    expect(signed(-1)).toBe('-1')
    // Zero is a bonus, not an absence: a sheet prints +0.
    expect(signed(0)).toBe('+0')
  })
})

describe('titleCase', () => {
  it('reads a slug back as words', () => {
    expect(titleCase('half-elf')).toBe('Half Elf')
    expect(titleCase('sleight-of-hand')).toBe('Sleight Of Hand')
    expect(titleCase('stealth')).toBe('Stealth')
  })
})

describe('references', () => {
  it('splits a typed reference', () => {
    expect(kindOf('race:half-elf')).toBe('race')
    expect(slugOf('race:half-elf')).toBe('half-elf')
  })

  it('leaves an untyped value alone rather than losing it', () => {
    expect(slugOf('half-elf')).toBe('half-elf')
  })
})

describe('classLine', () => {
  it('renders a single class', () => {
    expect(classLine([{ class: 'rogue', level: 3 }])).toBe('Rogue 3')
  })

  it('renders a multiclass in the order taken', () => {
    expect(
      classLine([
        { class: 'cleric', level: 2 },
        { class: 'wizard', level: 1 },
      ]),
    ).toBe('Cleric 2 / Wizard 1')
  })

  it('has something to say about a character with no class yet', () => {
    expect(classLine([])).toBe('--')
    expect(classLine(undefined)).toBe('--')
  })
})
