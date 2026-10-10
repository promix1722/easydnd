import type { Spell, SpellOfferSearch } from '@/lib/api'

export interface SpellFilterValues {
  packIds?: string[]
  sources?: string[]
  query: string
  level: string | null
  school: string | null
  casterClass: string | null
  time: string | null
  concentration: boolean
  ritual: boolean
  noMaterial: boolean
}

export const EMPTY_SPELL_FILTERS: SpellFilterValues = {
  query: '', level: null, school: null, casterClass: null, time: null,
  concentration: false, ritual: false, noMaterial: false,
}

/** Same predicates as catalogue search, applied only to a prompt's eligible pool. */
export function matchesSpellFilters(spell: Spell, filters: SpellFilterValues): boolean {
  const provenance = spell.provenance
  return (!(filters.packIds?.length) || !!provenance && filters.packIds.includes(provenance.packId))
    && (!(filters.sources?.length) || !!provenance && provenance.sources.some((s) => filters.sources?.includes(s.id)))
    && spell.name.toLocaleLowerCase().includes(filters.query.trim().toLocaleLowerCase())
    && (filters.level === null || spell.level === Number(filters.level))
    && (filters.school === null || spell.school === filters.school)
    && (filters.casterClass === null || spell.classes?.includes(filters.casterClass) === true)
    && (filters.time === null || spell.castingTime?.kind === filters.time)
    && (!filters.concentration || spell.concentration === true)
    && (!filters.ritual || spell.ritual === true)
    && (!filters.noMaterial || spell.components?.material !== true)
}

export function hasSpellFilters(filters: SpellFilterValues): boolean {
 return Object.entries(filters).some(([key, value]) => Array.isArray(value) ? value.length > 0 : value !== EMPTY_SPELL_FILTERS[key as keyof SpellFilterValues])
}

/** The same filters as the request a server-side search takes. */
export function spellFilterSearch(filters: SpellFilterValues): Omit<SpellOfferSearch, 'limit'> {
  const q = filters.query.trim()
  return {
    ...(q === '' ? {} : { q }),
    ...(filters.level === null ? {} : { level: Number(filters.level) }),
    ...(filters.school === null ? {} : { school: filters.school }),
    ...(filters.casterClass === null ? {} : { class: filters.casterClass }),
    ...(filters.time === null ? {} : { castingTime: filters.time }),
    ...(filters.concentration ? { concentration: true } : {}),
    ...(filters.ritual ? { ritual: true } : {}),
    ...(filters.noMaterial ? { material: false } : {}),
    ...(filters.packIds?.length ? { packs: filters.packIds } : {}),
    ...(filters.sources?.length ? { sources: filters.sources } : {}),
  }
}
