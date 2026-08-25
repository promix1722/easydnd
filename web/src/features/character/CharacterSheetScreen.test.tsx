import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { Sheet } from '@/lib/api'
import { renderAt } from '@/test/render'
import type { Viewport } from '@/test/viewport'

import { CharacterSheetScreen } from './CharacterSheetScreen'

/**
 * The sheet, wired to a projection whose ability keys arrive **alphabetically**
 * -- cha, con, dex, int, str, wis.
 *
 * That is not a stylistic choice in the fixture, it is the contract: the API
 * sends these as objects keyed by slug and a Go map serialises its keys
 * sorted, so alphabetical is what this screen is really handed. A fixture
 * written str-first would pass whether the screen ordered anything or not,
 * which is the only way a test of ordering can be worthless.
 *
 * The scores are six different numbers and the modifiers six different
 * bonuses, so a card in the wrong place is a wrong number rather than only a
 * wrong label.
 */
const SHEET: Sheet = {
  identity: { name: 'Zephyr', race: 'half-elf', classes: [{ class: 'rogue', level: 1 }], level: 1 },
  base: {
    hitPoints: { current: 9, max: 9 },
    languages: ['common'],
    deathSaves: { successes: 0, failures: 0 },
  },
  abilities: {
    scores: { cha: 8, con: 14, dex: 16, int: 12, str: 18, wis: 10 },
    modifiers: { cha: -1, con: 2, dex: 3, int: 1, str: 4, wis: 0 },
  },
  skills: { stealth: { proficiency: 'expertise', bonus: 7 } },
  savingThrows: {
    cha: { proficient: false, bonus: -1 },
    con: { proficient: false, bonus: 2 },
    dex: { proficient: true, bonus: 5 },
    int: { proficient: true, bonus: 3 },
    str: { proficient: false, bonus: 4 },
    wis: { proficient: false, bonus: 0 },
  },
  status: { armorClass: 15, initiative: 3, proficiencyBonus: 2, passivePerception: 12 },
  equipment: { equipped: [], backpack: [], loot: [] },
  resources: {},
  spells: {},
  actions: [],
}

/** The canonical six plus the one slug this client does not know. */
const ABILITY_TITLE = /^(?:Strength|Dexterity|Constitution|Intelligence|Wisdom|Charisma|Luk)$/
const ABILITY_SLUG = /^(?:str|dex|con|int|wis|cha|luk)$/

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

/**
 * The prompts a character still has outstanding.
 *
 * Two groups on purpose: the sheet shows the whole list rather than a
 * category's worth, and it is the same component the build screen's tabs draw
 * a filtered slice of.
 */
const OPEN = {
  seq: 3,
  complete: false,
  prompts: [
    {
      choice: {
        prompt: 'character/background',
        choose: 1,
        kind: 'background',
        from: { kind: 'collection', collection: 'background' },
      },
      group: 'background',
      optional: false,
      advances: false,
      event: { type: 'background' },
      heldOnly: false,
    },
    {
      choice: {
        prompt: 'half-elf/language/0',
        choose: 1,
        kind: 'language',
        from: { kind: 'collection', collection: 'language' },
      },
      source: 'race:half-elf',
      group: 'race',
      optional: false,
      advances: false,
      event: { type: 'race', ref: 'race:half-elf' },
      heldOnly: false,
    },
  ],
}

function mockApi(sheet: Sheet, prompts: unknown = { seq: 3, complete: true, prompts: [] }) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/sheet')) return jsonResponse(sheet)
      if (url.includes('/prompts')) return jsonResponse(prompts)
      return jsonResponse([])
    }),
  )
}

async function renderSheet(viewport: Viewport) {
  const result = renderAt(
    viewport,
    <MemoryRouter initialEntries={['/characters/chr_000001']}>
      <Routes>
        <Route path="/characters/:id" element={<CharacterSheetScreen />} />
        <Route path="/characters/:id/build" element={<div>build</div>} />
      </Routes>
    </MemoryRouter>,
  )
  // Nothing below exists until the projection has arrived.
  await screen.findByText('Zephyr')
  return result
}

/**
 * The ability cards' labels, in the order the document draws them.
 *
 * The cards are the only place that titles a slug with the ability's full
 * name, which is what separates them from the saving throws further down --
 * both print the bare slug, and both are meant to be in the same order.
 */
function abilityCards(): HTMLElement[] {
  return screen.queryAllByTitle(ABILITY_TITLE)
}

function abilityCardSlugs(): string[] {
  return abilityCards().map((label) => label.textContent ?? '')
}

/** The saving throws' labels, in document order: every slug that is not a card. */
function savingThrowSlugs(): string[] {
  const cards = new Set<Element>(abilityCards())
  return screen
    .getAllByText(ABILITY_SLUG)
    .filter((label) => !cards.has(label))
    .map((label) => label.textContent ?? '')
}

/**
 * Puts the saving throws on screen wherever they are drawn.
 *
 * On desktop they are a panel beside the skills and are already there. On
 * mobile `Columns` is an accordion, only the first section starts open, and
 * Mantine unmounts a closed panel -- so the saves have to be opened before
 * there is any order to read.
 */
async function openSavingThrows(): Promise<void> {
  const control = screen.queryByRole('button', { name: /^Saving throws$/ })
  if (control !== null) await userEvent.click(control)
}

/** Everything one card prints, so a modifier cannot drift off its own slug. */
function cardText(slug: string): string {
  const at = abilityCardSlugs().indexOf(slug)
  return abilityCards()[at]?.parentElement?.textContent ?? ''
}

beforeEach(() => {
  mockApi(SHEET)
})

afterEach(() => {
  vi.unstubAllGlobals()
})

// Mobile collapses the saving throws behind an accordion control and desktop
// does not, so neither rendering proves the other.
describe.each(['desktop', 'mobile'] as const)('the sheet at %s', (viewport) => {
  it('draws the six abilities in the order a sheet prints them, not the order they arrived', async () => {
    await renderSheet(viewport)

    expect(abilityCardSlugs()).toEqual(['str', 'dex', 'con', 'int', 'wis', 'cha'])
  })

  it('prints the saving throws in that same order', async () => {
    await renderSheet(viewport)
    await openSavingThrows()

    expect(savingThrowSlugs()).toEqual(['str', 'dex', 'con', 'int', 'wis', 'cha'])
  })

  it('leads the stat row with hit points, then armor class', async () => {
    await renderSheet(viewport)

    // Hit points move between one glance and the next; armor class is settled
    // at the start of a fight. First position belongs to the one read most.
    expect(
      screen
        .getAllByText(/^(?:Hit points|Armor class|Initiative|Proficiency)$/)
        .map((label) => label.textContent ?? ''),
    ).toEqual(['Hit points', 'Armor class', 'Initiative', 'Proficiency'])
  })
})

describe('the sheet', () => {
  it('reads a modifier off the slug it belongs to, not off the card next to it', async () => {
    await renderSheet('desktop')

    expect(cardText('str')).toBe('str+418')
    expect(cardText('dex')).toBe('dex+316')
    expect(cardText('cha')).toBe('cha-18')
  })

  it('keeps an ability the six do not cover, and draws it last', async () => {
    // An unrecognised ability means the server and this client disagree about
    // the game. Dropping it silently would hide exactly the bug worth seeing.
    mockApi({
      ...SHEET,
      abilities: {
        scores: { ...SHEET.abilities.scores, luk: 13 },
        modifiers: { ...SHEET.abilities.modifiers, luk: 1 },
      },
    })
    await renderSheet('desktop')

    expect(abilityCardSlugs()).toEqual(['str', 'dex', 'con', 'int', 'wis', 'cha', 'luk'])
    expect(cardText('luk')).toBe('luk+113')
  })

  it('draws nothing at all for an ability the projection is missing', async () => {
    // The modifier is kept, so a screen pairing scores with modifiers by
    // position rather than by slug would misreport all five of the rest.
    mockApi({
      ...SHEET,
      abilities: {
        ...SHEET.abilities,
        scores: { cha: 8, con: 14, dex: 16, int: 12, wis: 10 },
      },
    })
    await renderSheet('desktop')

    // Five cards, in canonical order, and no blank one claiming a score that
    // is not there.
    expect(abilityCardSlugs()).toEqual(['dex', 'con', 'int', 'wis', 'cha'])
    expect(screen.queryByTitle('Strength')).not.toBeInTheDocument()
    expect(cardText('dex')).toBe('dex+316')
    // The saves are a separate projection and still have all six.
    await openSavingThrows()
    expect(savingThrowSlugs()).toEqual(['str', 'dex', 'con', 'int', 'wis', 'cha'])
  })
})

describe('an unfinished character', () => {
  it('says on the sheet what is still to choose, and offers the way in', async () => {
    // The same list the build screen draws, from the same response: there is
    // no second notion of "outstanding" for the two pages to disagree about.
    mockApi(SHEET, OPEN)
    await renderSheet('desktop')

    expect(screen.getByText(/A background/)).toBeInTheDocument()
    expect(screen.getByText(/One more language/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Answer these' })).toBeInTheDocument()
  })

  it('says nothing at all when there is nothing left', async () => {
    await renderSheet('desktop')

    expect(screen.queryByText('Still to choose')).not.toBeInTheDocument()
  })

  it('still draws the sheet when the prompts could not be fetched', async () => {
    // A sheet that refuses to draw because a second request failed is a page
    // failing for a reason it is not about.
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input)
        if (url.includes('/sheet')) return jsonResponse(SHEET)
        if (url.includes('/prompts')) return jsonResponse({ error: { code: 'boom' } }, 500)
        return jsonResponse([])
      }),
    )
    await renderSheet('desktop')

    expect(screen.getByText('Zephyr')).toBeInTheDocument()
    expect(screen.queryByText('Still to choose')).not.toBeInTheDocument()
  })
})
