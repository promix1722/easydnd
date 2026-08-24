import { screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { resetCatalogCache } from '@/lib/api'
import { renderAt } from '@/test/render'
import type { Viewport } from '@/test/viewport'

import { CharacterLogScreen } from './CharacterLogScreen'

/**
 * The log screen, wired to a log captured from the real router rather than
 * invented: a half-elf rogue built through the API up to second level, which
 * is the only way to be sure the screen renders what the server actually
 * stores -- prompt slugs that are paths, picks that name a nested prompt, and
 * an init event with no timestamp at all.
 */
const LOG = {
  seq: 8,
  events: [
    {
      seq: 1,
      type: 'init',
      changes: [
        { path: 'identity.name', op: 'set', value: { kind: 'string', string: 'Zephyr' } },
        { path: 'abilities.method', op: 'set', value: { kind: 'slug', slug: 'point-buy' } },
        { path: 'abilities.dex', op: 'set', value: { kind: 'int', int: 15 } },
      ],
    },
    {
      seq: 2,
      type: 'race',
      at: '2026-08-23T21:26:44Z',
      ref: 'race:half-elf',
      choices: [
        { prompt: 'half-elf/ability-bonus/0', picks: ['dex', 'con'] },
        { prompt: 'skill-versatility/proficiency/0', picks: ['skill-acrobatics', 'skill-insight'] },
      ],
    },
    { seq: 3, type: 'background', at: '2026-08-23T21:26:44Z', ref: 'background:acolyte' },
    { seq: 4, type: 'class', at: '2026-08-23T21:26:44Z', ref: 'class:rogue', level: 1 },
    {
      seq: 5,
      type: 'level',
      at: '2026-08-23T21:26:44Z',
      ref: 'class:rogue',
      level: 1,
      choices: [
        { prompt: 'rogue-expertise-1/expertise/0', picks: ['rogue-expertise-1/expertise/0/0'] },
        { prompt: 'rogue-expertise-1/expertise/0/0', picks: ['skill-stealth'] },
      ],
    },
    {
      seq: 6,
      type: 'change',
      at: '2026-08-23T21:26:44Z',
      choices: [{ prompt: 'character/alignment', picks: ['chaotic-good'] }],
    },
    { seq: 7, type: 'level', at: '2026-08-23T21:26:44Z', ref: 'class:rogue', level: 2 },
    { seq: 8, type: 'note', at: '2026-08-23T21:26:44Z', note: 'Joined the party in Waterdeep.' },
  ],
}

const RACES = [{ slug: 'half-elf', name: 'Half-Elf' }]
const CLASSES = [{ slug: 'rogue', name: 'Rogue' }]
const BACKGROUNDS = [{ slug: 'acolyte', name: 'Acolyte' }]

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function mockApi() {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/events')) return jsonResponse(LOG)
      if (url.includes('/catalog/races')) return jsonResponse(RACES)
      if (url.includes('/catalog/classes')) return jsonResponse(CLASSES)
      if (url.includes('/catalog/backgrounds')) return jsonResponse(BACKGROUNDS)
      return jsonResponse([])
    }),
  )
}

function renderScreen(viewport: Viewport) {
  return renderAt(
    viewport,
    <MemoryRouter initialEntries={['/characters/chr_000001/log']}>
      <Routes>
        <Route path="/characters/:id/log" element={<CharacterLogScreen />} />
      </Routes>
    </MemoryRouter>,
  )
}

beforeEach(() => {
  resetCatalogCache()
  mockApi()
})

afterEach(() => {
  vi.unstubAllGlobals()
})

// The two renderings share no markup, so neither proves the other.
describe.each(['desktop', 'mobile'] as const)('the log at %s', (viewport) => {
  it('lists every stored event, in the order it was written', async () => {
    renderScreen(viewport)

    await waitFor(() => expect(screen.getByText('Created')).toBeInTheDocument())
    expect(screen.getByText('Race chosen')).toBeInTheDocument()
    expect(screen.getByText('Background chosen')).toBeInTheDocument()
    expect(screen.getByText('Class chosen')).toBeInTheDocument()
    expect(screen.getAllByText('Level gained')).toHaveLength(2)
    expect(screen.getByText('Adjusted')).toBeInTheDocument()
    expect(screen.getByText('Note')).toBeInTheDocument()
    expect(
      screen.getByText('8 events · the record the sheet is projected from'),
    ).toBeInTheDocument()
  })

  it('names a reference the way the compendium does, not the way the slug does', async () => {
    renderScreen(viewport)

    // "Half-Elf" is the catalogue's name; title-casing the slug would give
    // "Half Elf", so this asserts the lookup actually happened.
    await waitFor(() => expect(screen.getByText('Half-Elf')).toBeInTheDocument())
    expect(screen.getByText('Acolyte')).toBeInTheDocument()
    expect(screen.getAllByText('Rogue').length).toBeGreaterThan(0)
  })
})

describe('the log', () => {
  it('shows the level an event applies at', async () => {
    renderScreen('desktop')

    await waitFor(() => expect(screen.getByText('Level 2')).toBeInTheDocument())
  })

  it('reads a prompt path back as a heading and its picks as answers', async () => {
    renderScreen('desktop')

    await waitFor(() => expect(screen.getByText('Half Elf · Ability Bonus')).toBeInTheDocument())
    expect(screen.getByText('Dex, Con')).toBeInTheDocument()
    expect(screen.getByText('Skill Versatility · Proficiency')).toBeInTheDocument()
    expect(screen.getByText('Skill Acrobatics, Skill Insight')).toBeInTheDocument()
  })

  // Keeping both would print the choice twice: once saying "Expertise" and
  // once saying which skill it was spent on.
  it('shows a nested choice once, as its answer rather than as its branch', () => {
    renderScreen('desktop')

    return waitFor(() => {
      expect(screen.getByText('Rogue Expertise 1 · Expertise')).toBeInTheDocument()
      expect(screen.getByText('Skill Stealth')).toBeInTheDocument()
      expect(screen.queryByText('Expertise')).not.toBeInTheDocument()
    })
  })

  it('prints the changes the init event carries', async () => {
    renderScreen('desktop')

    await waitFor(() => expect(screen.getByText('identity.name set Zephyr')).toBeInTheDocument())
    expect(screen.getByText('abilities.dex set 15')).toBeInTheDocument()
    expect(screen.getByText('abilities.method set Point Buy')).toBeInTheDocument()
  })

  it('shows a note as it was written', async () => {
    renderScreen('desktop')

    await waitFor(() =>
      expect(screen.getByText('Joined the party in Waterdeep.')).toBeInTheDocument(),
    )
  })

  // Service.Create seeds the init event without stamping a time, so the very
  // first row of every log has none to show.
  it('has something to draw for the event with no timestamp', async () => {
    renderScreen('desktop')

    await waitFor(() => expect(screen.getByText('Created')).toBeInTheDocument())
    const stamped = new Date('2026-08-23T21:26:44Z').toLocaleString()
    expect(screen.getAllByText(stamped)).toHaveLength(7)
    expect(screen.getAllByText('--').length).toBeGreaterThan(0)
  })

  // The sheet is a projection of this log, so a log that only renders when the
  // compendium answers would be missing exactly when it is wanted.
  it('falls back to the slug when the compendium cannot be reached', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input)
        if (url.includes('/events')) return jsonResponse(LOG)
        return jsonResponse({ error: { code: 'internal', message: 'no' } }, 500)
      }),
    )
    renderScreen('desktop')

    await waitFor(() => expect(screen.getByText('Half Elf')).toBeInTheDocument())
    expect(screen.getByText('Race chosen')).toBeInTheDocument()
  })

  it('offers a retry when the log itself cannot be read', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse({ error: { code: 'internal', message: 'nope' } }, 500)),
    )
    renderScreen('desktop')

    await waitFor(() => expect(screen.getByText('Could not load this log')).toBeInTheDocument())
    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument()
  })
})
