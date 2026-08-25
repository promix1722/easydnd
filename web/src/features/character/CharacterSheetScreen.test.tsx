import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { resetCatalogCache } from '@/lib/api'
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
/**
 * Every skill in the game, which is what the projection now sends.
 *
 * The bonuses are the fixture's own abilities -- str 18 (+4), dex 16 (+3),
 * con 14 (+2), int 12 (+1), wis 10 (+0), cha 8 (-1) -- plus whatever the
 * training level contributes at proficiency bonus 2. Untrained skills carry
 * the bare modifier, and four of the six abilities give a *different* one, so
 * a panel that dropped a bonus or read it off the neighbouring row would show
 * a wrong number rather than only a wrong label.
 *
 * All four training levels appear, half proficiency included. Nothing in the
 * SRD produces `half` today, but an imported sheet can assert it and the panel
 * has to draw it as its own thing rather than as another kind of proficient.
 */
const SKILLS: Sheet['skills'] = {
  acrobatics: { proficiency: 'none', bonus: 3 },
  'animal-handling': { proficiency: 'none', bonus: 0 },
  arcana: { proficiency: 'none', bonus: 1 },
  athletics: { proficiency: 'half', bonus: 5 },
  deception: { proficiency: 'proficient', bonus: 1 },
  history: { proficiency: 'none', bonus: 1 },
  insight: { proficiency: 'none', bonus: 0 },
  intimidation: { proficiency: 'none', bonus: -1 },
  investigation: { proficiency: 'none', bonus: 1 },
  medicine: { proficiency: 'none', bonus: 0 },
  nature: { proficiency: 'none', bonus: 1 },
  perception: { proficiency: 'proficient', bonus: 2 },
  performance: { proficiency: 'none', bonus: -1 },
  persuasion: { proficiency: 'none', bonus: -1 },
  religion: { proficiency: 'none', bonus: 1 },
  'sleight-of-hand': { proficiency: 'proficient', bonus: 5 },
  stealth: { proficiency: 'expertise', bonus: 7 },
  survival: { proficiency: 'none', bonus: 0 },
}

/**
 * The compendium's half of a skill row: the name to print and the ability it
 * rolls against.
 *
 * "Sleight of Hand" is the one that matters. `titleCase` -- the fallback when
 * this request fails -- renders the slug as "Sleight Of Hand", so asserting on
 * the catalogue spelling is what proves the panel used the catalogue.
 */
const SKILL_CATALOG = [
  { slug: 'acrobatics', name: 'Acrobatics', ability: 'dex' },
  { slug: 'animal-handling', name: 'Animal Handling', ability: 'wis' },
  { slug: 'arcana', name: 'Arcana', ability: 'int' },
  { slug: 'athletics', name: 'Athletics', ability: 'str' },
  { slug: 'deception', name: 'Deception', ability: 'cha' },
  { slug: 'history', name: 'History', ability: 'int' },
  { slug: 'insight', name: 'Insight', ability: 'wis' },
  { slug: 'intimidation', name: 'Intimidation', ability: 'cha' },
  { slug: 'investigation', name: 'Investigation', ability: 'int' },
  { slug: 'medicine', name: 'Medicine', ability: 'wis' },
  { slug: 'nature', name: 'Nature', ability: 'int' },
  { slug: 'perception', name: 'Perception', ability: 'wis' },
  { slug: 'performance', name: 'Performance', ability: 'cha' },
  { slug: 'persuasion', name: 'Persuasion', ability: 'cha' },
  { slug: 'religion', name: 'Religion', ability: 'int' },
  { slug: 'sleight-of-hand', name: 'Sleight of Hand', ability: 'dex' },
  { slug: 'stealth', name: 'Stealth', ability: 'dex' },
  { slug: 'survival', name: 'Survival', ability: 'wis' },
]

const SHEET: Sheet = {
  identity: {
    name: 'Zephyr',
    race: 'half-elf',
    classes: [{ class: 'rogue', level: 1, subclass: 'thief' }],
    level: 1,
    background: 'acolyte',
    experience: 900,
  },
  base: {
    hitPoints: { current: 9, max: 9 },
    languages: ['common'],
    deathSaves: { successes: 0, failures: 0 },
  },
  abilities: {
    scores: { cha: 8, con: 14, dex: 16, int: 12, str: 18, wis: 10 },
    modifiers: { cha: -1, con: 2, dex: 3, int: 1, str: 4, wis: 0 },
  },
  skills: SKILLS,
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
      if (url.includes('/catalog/skills')) return jsonResponse(SKILL_CATALOG)
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
  // Nothing below exists until the projection has arrived. Matched as the
  // heading rather than by text: the name is now also a row in the identity
  // table, so a bare text query finds two.
  await screen.findByRole('heading', { name: 'Zephyr' })
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

/** One ability's whole card: the label, the modifier, the score and the save. */
function card(slug: string): Element | null {
  const at = abilityCardSlugs().indexOf(slug)
  return abilityCards()[at]?.closest('.mantine-Card-root') ?? null
}

/**
 * The save printed inside one ability's card, as "+4" or "-1".
 *
 * Read out of the card rather than off a list of its own, which is the whole
 * point of merging the two: there is no second sequence of six that could be
 * in a different order from the first.
 */
function cardSave(slug: string): string {
  return card(slug)?.textContent?.replace(/^.*Save/, '') ?? ''
}

/** Everything one card prints, so a modifier cannot drift off its own slug. */
function cardText(slug: string): string {
  return card(slug)?.textContent ?? ''
}

beforeEach(() => {
  // getCollection caches per session, so without this the first test's
  // catalogue response would answer every later one -- including the test
  // that needs the request to fail.
  resetCatalogCache()
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

  it('prints each saving throw inside its own ability card', async () => {
    await renderSheet(viewport)

    // Six different bonuses, so a save drawn on the wrong card is a wrong
    // number rather than only a wrong label. The order comes free: there is
    // one list of abilities now, not two that could disagree.
    expect(abilityCardSlugs().map(cardSave)).toEqual(['+4', '+5', '+2', '+3', '+0', '-1'])
  })

  it('keeps the pool, its shield and the dice that refill it together', async () => {
    await renderSheet(viewport)

    // Hit points, temporary hit points and Hit Dice are one subject read in
    // one glance, so they lead the row rather than being spread along it.
    expect(
      screen
        .getAllByText(/^(?:Hit points|Temp HP|Hit Dice|Armor class|Initiative|Proficiency)$/)
        .map((label) => label.textContent ?? ''),
    ).toEqual(['Hit points', 'Temp HP', 'Hit Dice', 'Armor class', 'Initiative', 'Proficiency'])
  })

  it('leads the page with the abilities everything else is derived from', async () => {
    await renderSheet(viewport)

    // Document order, not layout: the abilities are what every number below
    // them is derived from, so they are what the page opens on.
    const strength = screen.getByTitle('Strength')
    const hitPoints = screen.getByText('Hit points')
    const position = strength.compareDocumentPosition(hitPoints)
    expect(position & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })
})

describe('the sheet', () => {
  it('reads a modifier off the slug it belongs to, not off the card next to it', async () => {
    await renderSheet('desktop')

    expect(cardText('str')).toBe('str+418Save+4')
    expect(cardText('dex')).toBe('dex+316Save+5')
    expect(cardText('cha')).toBe('cha-18Save-1')
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
    // No save was projected for it, so the card prints none rather than a +0.
    expect(cardText('luk')).toBe('luk+113')
  })

  it('claims no score it was not sent, and still prints the save beside it', async () => {
    // Scores and saving throws are two projections and neither is promised to
    // hold all six. Now that the card is the only place either is drawn,
    // dropping the card would swallow a save the server did send -- so the
    // card stays and the score is the only thing missing from it.
    mockApi({
      ...SHEET,
      abilities: {
        modifiers: { cha: -1, con: 2, dex: 3, int: 1, wis: 0 },
        scores: { cha: 8, con: 14, dex: 16, int: 12, wis: 10 },
      },
    })
    await renderSheet('desktop')

    expect(abilityCardSlugs()).toEqual(['str', 'dex', 'con', 'int', 'wis', 'cha'])
    // A dash rather than a "+0" that would read as a real modifier of zero.
    expect(cardText('str')).toBe('str--\u00a0Save+4')
    // The modifier is kept for the rest, so a screen pairing scores with
    // modifiers by position rather than by slug would misreport all five.
    expect(cardText('dex')).toBe('dex+316Save+5')
  })
})

/**
 * The skills panel.
 *
 * What these defend is that eighteen rows stay *readable*: that the ones a
 * character is trained in are findable without reading every line, that the
 * training level is carried by something other than colour, and that a number
 * never drifts off the skill it belongs to.
 */
describe('the skills panel', () => {
  /**
   * Each skill row's text, in the order the document draws them.
   *
   * Found through the training mark, which is the one element every row has
   * exactly one of. Walking up from it -- the mark's box, its group, the row
   * -- keeps the name, the ability and the bonus together, so a test can
   * assert that a bonus belongs to *its* skill rather than that the number
   * appears on the page somewhere.
   */
  function skillRows(): string[] {
    // Scoped to the panel: the ability cards draw the same mark for their
    // saving throws, so an unscoped query would return twenty-four rows and
    // every count below would quietly be six too many.
    const panel = screen.getByText(/proficient|Nothing trained/).parentElement
    if (panel === null) throw new Error('the skills panel is not on the page')
    return within(panel)
      .getAllByRole('img', { name: /proficien|Expertise/i })
      .map((mark) => mark.closest('div')?.parentElement?.parentElement?.textContent ?? '')
  }

  function skillNames(): string[] {
    // The ability tag is optional: it is the half of the row that comes from
    // the compendium, so it is missing when that request failed.
    return skillRows().map((row) => row.replace(/(?:STR|DEX|CON|INT|WIS|CHA)?[+-]\d+$/, ''))
  }

  it('draws every skill, not only the ones something trained', async () => {
    await renderSheet('desktop')

    // Eighteen rows, and the twelve untrained ones are the point: the skill a
    // player asks what to roll for is usually one nothing trained.
    expect(skillRows()).toHaveLength(18)
    expect(skillNames()).toContain('Investigation')
    expect(skillNames()).toContain('Nature')
  })

  it('says how each row is trained in words, not only in colour', async () => {
    await renderSheet('desktop')

    // A panel distinguishing eighteen rows by a shade of grey tells a screen
    // reader nothing, and tells a colour-blind reader nothing either.
    const panel = within(screen.getByText(/proficient ·/).parentElement!)
    expect(panel.getByRole('img', { name: /Expertise/ })).toBeInTheDocument()
    expect(panel.getAllByRole('img', { name: /^Proficient/ })).toHaveLength(3)
    expect(panel.getAllByRole('img', { name: /^Not proficient/ })).toHaveLength(13)
    expect(panel.getByRole('img', { name: /^Half proficiency/ })).toBeInTheDocument()
  })

  it('leads with what the character is best at, alphabetical within each level', async () => {
    await renderSheet('desktop')

    // Expertise, then proficient, then half, then the untrained. With six
    // rows the alphabet was the only searchable order; with eighteen, the
    // question is what the character is good at.
    expect(skillNames().slice(0, 5)).toEqual([
      'Stealth',
      'Deception',
      'Perception',
      'Sleight of Hand',
      'Athletics',
    ])
    // The untrained block is alphabetical from there on.
    expect(skillNames().slice(5, 8)).toEqual(['Acrobatics', 'Animal Handling', 'Arcana'])
  })

  it('reads a bonus and an ability off the skill they belong to', async () => {
    await renderSheet('desktop')

    // Paired by row rather than by position: a panel that zipped two lists
    // together would misreport every row after the first mistake.
    const rows = skillRows()
    expect(rows[0]).toBe('StealthDEX+7')
    expect(rows.find((row) => row.startsWith('Athletics'))).toBe('AthleticsSTR+5')
    expect(rows.find((row) => row.startsWith('Intimidation'))).toBe('IntimidationCHA-1')
  })

  it('counts what is trained', async () => {
    await renderSheet('desktop')

    expect(screen.getByText('5 proficient · 1 with expertise')).toBeInTheDocument()
  })

  it('collapses to the trained rows and back', async () => {
    await renderSheet('desktop')

    // It starts showing everything: drawing all eighteen is the point of the
    // panel, and the toggle is for the phone rather than a stored preference.
    expect(skillRows()).toHaveLength(18)

    await userEvent.click(screen.getByRole('button', { name: 'Hide untrained' }))
    expect(skillRows()).toHaveLength(5)
    expect(skillNames()).not.toContain('Arcana')

    await userEvent.click(screen.getByRole('button', { name: 'Show all 18' }))
    expect(skillRows()).toHaveLength(18)
  })

  it('names a skill the way the compendium does', async () => {
    await renderSheet('desktop')

    // titleCase() renders the slug as "Sleight Of Hand". The catalogue is
    // also the only name that is in the negotiated locale.
    expect(skillNames()).toContain('Sleight of Hand')
    expect(skillNames()).not.toContain('Sleight Of Hand')
  })

  // The phone is where eighteen rows costs the most and where the toggle earns
  // its place. Skills is the first Columns section, so the accordion has it
  // open already -- nothing to click before the panel is there.
  it('is open on a phone, and collapses there too', async () => {
    await renderSheet('mobile')

    expect(skillRows()).toHaveLength(18)

    await userEvent.click(screen.getByRole('button', { name: 'Hide untrained' }))
    expect(skillRows()).toHaveLength(5)
  })

  it('still draws the panel when the compendium could not be fetched', async () => {
    // A second request failing costs the ability tags and the proper names.
    // It is not a reason for the sheet to refuse to draw.
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input)
        if (url.includes('/sheet')) return jsonResponse(SHEET)
        if (url.includes('/prompts')) return jsonResponse({ seq: 3, complete: true, prompts: [] })
        if (url.includes('/catalog/skills')) return jsonResponse({ error: { code: 'boom' } }, 500)
        return jsonResponse([])
      }),
    )
    await renderSheet('desktop')

    expect(skillRows()).toHaveLength(18)
    expect(skillNames()).toContain('Sleight Of Hand')
  })
})

describe('an unfinished character', () => {
  it('says on the sheet what is still to choose, and offers the way in', async () => {
    // The same choices the build screen draws, from the same response and
    // named the same way: there is no second notion of "outstanding" for the
    // two pages to disagree about, and no second vocabulary for it either.
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

    expect(screen.getByRole('heading', { name: 'Zephyr' })).toBeInTheDocument()
    expect(screen.queryByText('Still to choose')).not.toBeInTheDocument()
  })
})
