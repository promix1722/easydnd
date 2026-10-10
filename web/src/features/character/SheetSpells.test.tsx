import { screen, waitFor, within } from '@testing-library/react'
import { beforeEach, expect, it, vi } from 'vitest'

import type { CharacterEvent, Prompt, Sheet, Spell } from '@/lib/api'
import { renderAt } from '@/test/render'
import { spellCatalog } from '@/test/spells'
import { setupUser } from '@/test/user'

import { SheetSpells } from './SheetSpells'
import { jsonResponse } from '@/test/api'

/**
 * The Spells tab, with the server behind `fetch`: the catalogue answers spell
 * requests, and a hand-written log and prompt answer the preparation flow.
 */
const spells: Spell[] = [
  { slug: 'bless', name: 'Bless', level: 1, concentration: true, desc: ['Up to three creatures gain a d4.'] },
  { slug: 'cure-wounds', name: 'Cure Wounds', level: 1, desc: ['Heals.'] },
  { slug: 'aid', name: 'Aid', level: 2, desc: ['Bolsters.'] },
  { slug: 'guidance', name: 'Guidance', level: 0, desc: ['A d4 on a check.'] },
]
const sheet = (prepared: string[]): Sheet => ({
  identity: { name: 'Mara', level: 3, experience: 0, classes: [{ class: 'cleric', level: 3 }] },
  base: {} as Sheet['base'], abilities: {} as Sheet['abilities'], skills: {}, savingThrows: {},
  status: {} as Sheet['status'], equipment: { equipped: [], backpack: [], loot: [] }, resources: {},
  spells: { sources: [{ source: 'class:cleric', class: 'cleric', ability: 'wis', cantrips: ['guidance'], prepared, preparationLimit: 2 }] },
  catalogNames: { 'classes:cleric': 'Cleric' },
  catalog: { skills: [], spells: spells.map(({ slug, name, level, concentration }) => ({ slug, name, level, ...(concentration ? { concentration } : {}) })) },
})
const prompt: Prompt = {
  choice: { prompt: 'cleric/spell/prepared/3', kind: 'spell', choose: 2,
    from: { kind: 'explicit', options: ['bless', 'cure-wounds', 'aid'].map((slug) => ({ key: slug, kind: 'ref', ref: `spell:${slug}` })) } },
  purpose: 'prepared', upTo: true, source: 'class:cleric', group: 'class', level: 3,
  event: { type: 'level', ref: 'class:cleric', level: 3 }, heldOnly: false, optional: true,
}
const writes: { method: string; url: string; body: unknown }[] = []

/** A log with the preparation answered at entry 4, or not answered at all. */
function serve(answered: string[] | null) {
  const events: CharacterEvent[] = [{ seq: 1, type: 'init' }, { seq: 2, type: 'class', ref: 'class:cleric' }, { seq: 3, type: 'level', ref: 'class:cleric', level: 3 }]
  if (answered !== null) events.push({ seq: 4, type: 'level', ref: 'class:cleric', level: 3, choices: [{ prompt: 'cleric/spell/prepared/3', picks: answered }] })
  const json = (data: unknown) => jsonResponse(data)
  return async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = new URL(String(input), 'http://localhost')
    const method = init?.method ?? 'GET'
    if (method !== 'GET' && url.pathname.includes('/events')) {
      writes.push({ method, url: url.pathname.replace('/v1', '') + url.search.replace(/[?&]locale=en/, ''), body: init?.body ? JSON.parse(String(init.body)) : null })
      return json({ seq: 5, revision: 5 })
    }
    if (url.pathname.endsWith('/events')) return json({ seq: events.length, revision: events.length, events })
    if (url.pathname.endsWith('/prompts')) {
      const posed = url.searchParams.get('before') === '4' || answered === null
      return json({ seq: events.length, revision: events.length, prompts: posed ? [prompt] : [], complete: true })
    }
    return spellCatalog(spells, input, init) ?? json([])
  }
}
const onChanged = vi.fn()
beforeEach(() => { writes.length = 0; onChanged.mockClear() })

for (const viewport of ['desktop', 'mobile'] as const) {
  it(`draws a caster's spells under level headings and opens one's text on ${viewport}`, async () => {
    vi.stubGlobal('fetch', vi.fn(serve(null)))
    const user = setupUser()
    renderAt(viewport, <SheetSpells sheet={sheet(['bless'])} />)
    expect(screen.getByText('Cleric')).toBeInTheDocument()
    const cantrips = screen.getByRole('region', { name: 'Cantrips: 1' })
    expect(within(cantrips).getByRole('article', { name: 'Guidance' })).toBeInTheDocument()
    expect(within(cantrips).queryByRole('heading')).not.toBeInTheDocument()
    const prepared = screen.getByRole('region', { name: 'Prepared spells (up to 2)' })
    expect(within(prepared).getByRole('heading', { name: 'Level 1' })).toBeInTheDocument()
    // Read only: nothing on a sheet without its owner can be pressed.
    expect(screen.queryByRole('button', { name: /^(Add|Remove) / })).not.toBeInTheDocument()
    await user.click(within(prepared).getByRole('button', { name: 'Bless' }))
    await waitFor(() => expect(screen.getByText('Up to three creatures gain a d4.')).toBeInTheDocument())
    if (viewport === 'mobile') {
      await user.click(screen.getByRole('button', { name: 'Back' }))
      expect(screen.queryByText('Up to three creatures gain a d4.')).not.toBeInTheDocument()
    }
  })
}

it('prepares a spell the first time by appending the answer', async () => {
  vi.stubGlobal('fetch', vi.fn(serve(null)))
  const user = setupUser()
  renderAt('desktop', <SheetSpells sheet={sheet([])} characterId="c1" onChanged={onChanged} />)
  const available = await screen.findByRole('region', { name: 'Available spells' })
  await user.click(await within(available).findByRole('button', { name: 'Add Bless' }))
  await waitFor(() => expect(onChanged).toHaveBeenCalled())
  expect(writes).toEqual([{ method: 'POST', url: '/characters/c1/events', body: {
    expectedSeq: 3, expectedRevision: 3, events: [{ type: 'level', ref: 'class:cleric', level: 3, choices: [{ prompt: 'cleric/spell/prepared/3', picks: ['bless'] }] }] } }])
})

it('unprepares by replacing the answering entry', async () => {
  vi.stubGlobal('fetch', vi.fn(serve(['bless', 'aid'])))
  const user = setupUser()
  renderAt('desktop', <SheetSpells sheet={sheet(['bless', 'aid'])} characterId="c1" onChanged={onChanged} />)
  // The section is drawn again once the question arrives, so find the button, not the region.
  await user.click(await screen.findByRole('button', { name: 'Remove Aid' }))
  await waitFor(() => expect(writes).toHaveLength(1))
  expect(writes[0]).toMatchObject({ method: 'PUT', url: '/characters/c1/events/4', body: { expectedSeq: 4, event: { choices: [{ prompt: 'cleric/spell/prepared/3', picks: ['bless'] }] } } })
  expect(screen.getByText('Selected: 2 / 2')).toBeInTheDocument()
  // The list is full, so what is available cannot be added until something goes.
  const available = screen.getByRole('region', { name: 'Available spells' })
  expect(await within(available).findByRole('button', { name: 'Add Cure Wounds' })).toBeDisabled()
})

it('deletes the answering entry when the last prepared spell goes', async () => {
  vi.stubGlobal('fetch', vi.fn(serve(['bless'])))
  const user = setupUser()
  renderAt('desktop', <SheetSpells sheet={sheet(['bless'])} characterId="c1" onChanged={onChanged} />)
  await user.click(await screen.findByRole('button', { name: 'Remove Bless' }))
  await waitFor(() => expect(writes).toHaveLength(1))
  expect(writes[0]).toMatchObject({ method: 'DELETE', url: '/characters/c1/events/4?expectedSeq=4&expectedRevision=4' })
})
