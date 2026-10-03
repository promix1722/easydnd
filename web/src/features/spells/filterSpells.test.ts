import { describe, expect, it } from 'vitest'
import type { Spell } from '@/lib/api'
import { EMPTY_SPELL_FILTERS, matchesSpellFilters, type SpellFilterValues } from './filterSpells'

const spell: Spell = {
  slug: 'detect-magic', name: 'Обнаружение магии', level: 1, school: 'divination',
  classes: ['wizard', 'cleric'], castingTime: { kind: 'action' },
  concentration: true, ritual: true, components: { verbal: true, somatic: true },
}

describe('builder spell filters', () => {
  it('combines every filter and matches localized names without case sensitivity', () => {
    expect(matchesSpellFilters(spell, {
      query: ' МАГИИ ', level: '1', school: 'divination', casterClass: 'cleric',
      time: 'action', concentration: true, ritual: true, noMaterial: true,
    })).toBe(true)
  })

  it.each<Partial<SpellFilterValues>>([
    { query: 'fire' }, { level: '0' }, { school: 'evocation' },
    { casterClass: 'warlock' }, { time: 'over-time' },
  ])('excludes mismatches: %j', (filters) => {
    expect(matchesSpellFilters(spell, { ...EMPTY_SPELL_FILTERS, ...filters })).toBe(false)
  })

  it('distinguishes cantrips, absent flags and material components', () => {
    const cantrip: Spell = { slug: 'light', name: 'Light', level: 0, components: { material: true } }
    expect(matchesSpellFilters(cantrip, { ...EMPTY_SPELL_FILTERS, level: '0' })).toBe(true)
    for (const filters of [{ concentration: true }, { ritual: true }, { noMaterial: true }]) {
      expect(matchesSpellFilters(cantrip, { ...EMPTY_SPELL_FILTERS, ...filters })).toBe(false)
    }
  })
})
