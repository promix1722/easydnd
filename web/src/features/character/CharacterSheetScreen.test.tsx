import { screen, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { bySlug } from '@/lib/api'
import type { CatalogSkill, Sheet } from '@/lib/api'
import { renderAt } from '@/test/render'
import type { Viewport } from '@/test/viewport'

import { CharacterSheetScreen } from './CharacterSheetScreen'
import { SkillsPanel } from './SkillsPanel'

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
 * Two groups on purpose: what the sheet reads off this response is only
 * whether it is empty, so a fixture with one prompt in one group would pass
 * against a screen that had gone looking at the group.
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

/**
 * What each ability card is headed with.
 *
 * The abbreviation, not the slug. The card used to print `{ability}` with
 * `text-transform: uppercase` over it, so the document said "str" and the
 * screen said "STR" -- which meant a screen reader heard the slug. It prints
 * the catalogue's abbreviation now, because "Сила" does not shorten to СИЛ by
 * uppercasing it.
 */
function abilityCardLabels(): string[] {
  return abilityCards().map((label) => label.textContent ?? '')
}

/** One ability's whole card: the label, the modifier, the score and the save. */
function card(slug: string): Element | null {
  const at = abilityCardLabels().indexOf(slug.toUpperCase())
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
  // The catalogue cache and the stubbed fetch are both emptied by
  // src/test/setup.ts after every test -- including for the test that needs
  // the catalogue request to fail -- so there is nothing to undo here.
  mockApi(SHEET)
})

/**
 * Six readings of one sheet, in one test and one mount.
 *
 * They used to be four tests at two viewports -- eight mounts of the whole
 * sheet for assertions that never touch it twice. This whole file now runs at
 * desktop: what branches on width is `ui/SectionDeck`, which draws the six
 * sections `SheetBody` builds, and `SheetBody` itself, which orders the first
 * of them two ways. Both are tested where they live -- `SectionDeck.test.tsx`
 * and `SheetBody.test.tsx`. What is left here is the seam: that the screen
 * fetches the projection, the prompts and the compendium and hands all three
 * on.
 *
 * expect.soft rather than expect: six separate tests reported six separate
 * failures, and a merged test that stopped at the first would report one. The
 * saving is the mount, not the assertions, so there is no reason to give that
 * up as well.
 */
it('draws the sheet in the order a player reads it', async () => {
  await renderSheet('desktop')

  expect.soft(abilityCardLabels()).toEqual(['STR', 'DEX', 'CON', 'INT', 'WIS', 'CHA'])

  // Six different bonuses, so a save drawn on the wrong card is a wrong number
  // rather than only a wrong label. The order comes free: there is one list of
  // abilities now, not two that could disagree.
  expect.soft(abilityCardLabels().map(cardSave)).toEqual(['+4', '+5', '+2', '+3', '+0', '-1'])

  // Hit points, temporary hit points and Hit Dice are one subject read in one
  // glance, so they lead the row rather than being spread along it.
  expect
    .soft(
      screen
        .getAllByText(/^(?:Hit points|Temp HP|Hit Dice|Armor class|Initiative|Proficiency)$/)
        .map((label) => label.textContent ?? ''),
    )
    .toEqual(['Hit points', 'Temp HP', 'Hit Dice', 'Armor class', 'Initiative', 'Proficiency'])

  // Document order, not layout: the abilities are what every number below them
  // is derived from, so they are what the page opens on.
  const position = screen.getByTitle('Strength').compareDocumentPosition(screen.getByText('Hit points'))
  expect.soft(position & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()

  // Each card prints its own modifier: paired by slug, not by position, so a
  // sheet that zipped two lists together would misreport every card after the
  // first mistake.
  expect.soft(cardText('str')).toBe('STR+418Save+4')
  expect.soft(cardText('dex')).toBe('DEX+316Save+5')
  expect.soft(cardText('cha')).toBe('CHA-18Save-1')

  // Nothing outstanding on this fixture, so the header offers no way in to the
  // build screen. The button's presence is the whole of the message.
  expect.soft(screen.queryByRole('link', { name: 'Answer what is left' })).not.toBeInTheDocument()
})

describe('the sheet', () => {
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

    expect(abilityCardLabels()).toEqual(['STR', 'DEX', 'CON', 'INT', 'WIS', 'CHA', 'LUK'])
    // No save was projected for it, so the card prints none rather than a +0.
    expect(cardText('luk')).toBe('LUK+113')
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

    expect(abilityCardLabels()).toEqual(['STR', 'DEX', 'CON', 'INT', 'WIS', 'CHA'])
    // A dash rather than a "+0" that would read as a real modifier of zero.
    expect(cardText('str')).toBe('STR--\u00a0Save+4')
    // The modifier is kept for the rest, so a screen pairing scores with
    // modifiers by position rather than by slug would misreport all five.
    expect(cardText('dex')).toBe('DEX+316Save+5')
  })
})

/**
 * The skills panel.
 *
 * What these defend is that eighteen rows stay *readable*: that the ones a
 * character is trained in are findable without reading every line, that the
 * training level is carried by something other than colour, and that a number
 * never drifts off the skill it belongs to.
 *
 * Rendered as the panel rather than as the whole sheet, which is the shape
 * ProficienciesPanel.test.tsx and Vitals.test.tsx already use for their own
 * panels. Mounting the sheet for these cost about seven times as much and
 * brought along twenty-four Mantine Tooltips no assertion here ever opens. The
 * test that follows keeps the full sheet, because what it is about is the
 * panel being wired into it.
 */
describe('the skills panel', () => {
  function renderPanel(catalog: Map<string, CatalogSkill> | null = bySlug(SKILL_CATALOG)) {
    return renderAt('desktop', <SkillsPanel skills={SKILLS} catalog={catalog} />)
  }

  /**
   * Each skill row's text, in the order the document draws them.
   *
   * Found through the training mark, which is the one element every row has
   * exactly one of. Walking up from it -- the mark's box, its group, the row
   * -- keeps the name, the ability and the bonus together, so a test can
   * assert that a bonus belongs to *its* skill rather than that the number
   * appears on the page somewhere.
   *
   * No scoping needed here: the ability cards draw the same mark for their
   * saving throws, but only the panel is mounted.
   */
  function skillRows(): string[] {
    return screen
      .getAllByRole('img', { name: /proficien|Expertise/i })
      .map((mark) => mark.closest('div')?.parentElement?.parentElement?.textContent ?? '')
  }

  function namesOf(rows: string[]): string[] {
    // The ability tag is optional: it is the half of the row that comes from
    // the compendium, so it is missing when that request failed.
    return rows.map((row) => row.replace(/(?:STR|DEX|CON|INT|WIS|CHA)?[+-]\d+$/, ''))
  }

  function skillNames(): string[] {
    return namesOf(skillRows())
  }

  /**
   * Everything the default panel prints, from one mount.
   *
   * Five read-only assertions that shared a fixture and re-mounted for each,
   * merged the way docs/web.md asks. expect.soft so the merged test still
   * reports every failure rather than stopping at the first.
   */
  it('draws every skill, in the order and the words a player reads them', () => {
    renderPanel()

    // Eighteen rows, and the twelve untrained ones are the point: the skill a
    // player asks what to roll for is usually one nothing trained.
    const names = namesOf(skillRows())
    expect.soft(names).toHaveLength(18)
    expect.soft(names).toContain('Investigation')
    expect.soft(names).toContain('Nature')

    // A panel distinguishing eighteen rows by a shade of grey tells a screen
    // reader nothing, and tells a colour-blind reader nothing either.
    expect.soft(screen.getByRole('img', { name: /Expertise/ })).toBeInTheDocument()
    expect.soft(screen.getAllByRole('img', { name: /^Proficient/ })).toHaveLength(3)
    expect.soft(screen.getAllByRole('img', { name: /^Not proficient/ })).toHaveLength(13)
    expect.soft(screen.getByRole('img', { name: /^Half proficiency/ })).toBeInTheDocument()

    // Expertise, then proficient, then half, then the untrained. With six
    // rows the alphabet was the only searchable order; with eighteen, the
    // question is what the character is good at.
    expect
      .soft(names.slice(0, 5))
      .toEqual(['Stealth', 'Deception', 'Perception', 'Sleight of Hand', 'Athletics'])
    // The untrained block is alphabetical from there on.
    expect.soft(names.slice(5, 8)).toEqual(['Acrobatics', 'Animal Handling', 'Arcana'])

    // Paired by row rather than by position: a panel that zipped two lists
    // together would misreport every row after the first mistake.
    const rows = skillRows()
    expect.soft(rows[0]).toBe('StealthDEX+7')
    expect.soft(rows.find((row) => row.startsWith('Athletics'))).toBe('AthleticsSTR+5')
    expect.soft(rows.find((row) => row.startsWith('Intimidation'))).toBe('IntimidationCHA-1')

    expect.soft(screen.getByText('5 proficient · 1 with expertise')).toBeInTheDocument()

    // titleCase() renders the slug as "Sleight Of Hand". The catalogue is
    // also the only name that is in the negotiated locale.
    expect.soft(names).toContain('Sleight of Hand')
    expect.soft(names).not.toContain('Sleight Of Hand')
  })

  // All eighteen, always, with nothing to press first. A "Hide untrained"
  // toggle used to collapse this to the trained five; drawing every skill is
  // the point of the panel, so a control whose job was to undo that was
  // answering the question the rows exist to answer by removing them.
  it('draws every skill, and offers no way to hide any of them', () => {
    renderPanel()

    expect.soft(skillRows()).toHaveLength(18)
    expect.soft(skillNames()).toContain('Arcana')
    expect.soft(screen.queryAllByRole('button')).toHaveLength(0)
  })

  it('still draws every row when there is no compendium to name them by', () => {
    renderPanel(null)

    const names = namesOf(skillRows())
    expect(names).toHaveLength(18)
    // Falls back to titleCase() over the slug, and loses the ability tag.
    expect(names).toContain('Sleight Of Hand')
  })
})

/**
 * The panel inside the sheet.
 *
 * Everything above renders SkillsPanel on its own, so this is what still proves
 * it is wired into the page: that a failed compendium request reaches it as a
 * null catalogue rather than as a blank sheet.
 *
 * The phone rendering used to be here too, as the one mobile test in the file.
 * It has moved to `SheetBody.test.tsx`, which mounts the body from props rather
 * than the screen from a mocked fetch -- the body is where the two renderings
 * differ, and the filter reaching its rows through the deck is what that test
 * is about.
 */
describe('the skills panel, in the sheet', () => {
  function sheetRows(): HTMLElement[] {
    // Scoped to the panel: the ability cards draw the same mark for their
    // saving throws, so an unscoped query would return twenty-four rows and
    // every count below would quietly be six too many.
    const panel = screen.getByText(/proficient|Nothing trained/).parentElement
    if (panel === null) throw new Error('the skills panel is not on the page')
    return within(panel).getAllByRole('img', { name: /proficien|Expertise/i })
  }

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

    expect(sheetRows()).toHaveLength(18)
    expect(screen.getByText(/Sleight Of Hand/)).toBeInTheDocument()
  })
})

describe('an unfinished character', () => {
  it('offers the way in, and does not list what is left', async () => {
    // The sheet says the character is unfinished by carrying the way to finish
    // it, and says it in one place. What is still open is enumerated on the
    // screen that answers it -- listing the questions above the sheet put the
    // build screen's work on the page nobody came to build on.
    mockApi(SHEET, OPEN)
    await renderSheet('desktop')

    const answer = screen.getByRole('link', { name: 'Answer what is left' })
    expect(answer).toHaveAttribute('href', '/characters/chr_000001/build')
    expect(screen.queryByText(/A background/)).not.toBeInTheDocument()
    expect(screen.queryByText(/1 more language/)).not.toBeInTheDocument()
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
    // No list of prompts means no button: a failed `/prompts` draws nothing
    // rather than something wrong.
    expect(screen.queryByRole('link', { name: 'Answer what is left' })).not.toBeInTheDocument()
  })
})
