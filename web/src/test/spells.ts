import type { Spell, SpellOfferSearch } from '@/lib/api'

/**
 * The spell half of the catalogue, as the server answers it.
 *
 * Returns undefined for a request that is not about spells, so a test's own
 * stub can fall through to it. A request for the bare collection throws: the
 * server refuses it, and no screen may go back to downloading the list.
 */
export function spellCatalog(spells: readonly Spell[], input: RequestInfo | URL, init?: RequestInit): Response | undefined {
  const url = new URL(String(input), 'http://localhost')
  const json = (data: unknown) => new Response(JSON.stringify(data), { status: 200, headers: { 'Content-Type': 'application/json' } })
  if (url.pathname.endsWith('/spell-filters')) return json({ packs: [], sources: [], schools: [], classes: [] })
  if (url.pathname.endsWith('/spells/search')) {
    const search = JSON.parse(String(init?.body)) as SpellOfferSearch
    const q = (search.q ?? '').toLocaleLowerCase()
    const matches = spells
      .filter((spell) => search.only === undefined || search.only.slugs.includes(spell.slug) || search.only.fitting.some((fit) =>
        spell.level >= fit.minLevel && spell.level <= fit.maxLevel && (!fit.classes?.length || fit.classes.some((name) => spell.classes?.includes(name)))))
      .filter((spell) => !search.exclude?.includes(spell.slug))
      .filter((spell) => spell.name.toLocaleLowerCase().includes(q)
        && (search.level === undefined || spell.level === search.level)
        && (search.school === undefined || spell.school === search.school)
        && (search.class === undefined || spell.classes?.includes(search.class) === true)
        && (search.ritual === undefined || (spell.ritual === true) === search.ritual)
        && (search.concentration === undefined || (spell.concentration === true) === search.concentration))
      .sort((a, b) => a.level - b.level || a.name.localeCompare(b.name))
    const offset = search.offset ?? 0
    return json({ spells: matches.slice(offset, offset + search.limit), total: matches.length })
  }
  if (url.pathname.endsWith('/catalog/spells')) {
    const slugs = url.searchParams.get('slugs')?.split(',')
    if (slugs === undefined) throw new Error(`the whole spell list was requested: ${url.pathname}${url.search}`)
    return json(spells.filter((spell) => slugs.includes(spell.slug)))
  }
  return undefined
}

/** A `fetch` that answers spell requests from `spells` and everything else with an empty list. */
export function spellFetch(spells: readonly Spell[]) {
  return async (input: RequestInfo | URL, init?: RequestInit) =>
    spellCatalog(spells, input, init) ?? new Response('[]', { status: 200, headers: { 'Content-Type': 'application/json' } })
}
