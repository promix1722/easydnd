import { screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'

import { BuildScreen } from './BuildScreen'

/**
 * The build screen, wired to responses in the shape the real API sends.
 *
 * The payloads below were captured from a running server rather than
 * invented, because the thing worth testing here is that the screen posts
 * what the server asked for -- and a hand-written fixture would only prove
 * the screen agrees with my memory of the contract.
 */

const RACE_PROMPT = {
  seq: 1,
  complete: false,
  prompts: [
    {
      choice: {
        prompt: 'character/race',
        choose: 1,
        kind: 'race',
        from: { kind: 'collection', collection: 'race' },
      },
      group: 'race',
      optional: false,
      advances: false,
      event: { type: 'race' },
      heldOnly: false,
    },
  ],
}

const RACES = [
  { slug: 'half-elf', name: 'Half-Elf', speed: 30 },
  { slug: 'dwarf', name: 'Dwarf', speed: 25 },
]

const SUBRACES = [{ slug: 'hill-dwarf', name: 'Hill Dwarf' }]

/**
 * The init entry, as creation now writes it: a name and nothing else.
 *
 * `source` is the server's, not the client's. It is what puts this row on the
 * identity tab without anything here inferring a category from `init`.
 */
const INIT = {
  seq: 1,
  type: 'init',
  source: 'identity',
  changes: [{ path: 'identity.name', op: 'set', value: { kind: 'string', string: 'Zephyr' } }],
}

const LOG_JUST_CREATED = { seq: 1, events: [INIT] }

/** A half-elf partway through: race settled, its follow-ups still open. */
const PARTWAY = {
  seq: 3,
  complete: false,
  prompts: [
    {
      choice: {
        prompt: 'skill-versatility/proficiency/0',
        choose: 2,
        kind: 'proficiency',
        from: {
          kind: 'explicit',
          options: [
            { key: 'skill-acrobatics', kind: 'ref', ref: 'skill:acrobatics' },
            { key: 'skill-insight', kind: 'ref', ref: 'skill:insight' },
          ],
        },
      },
      source: 'race:half-elf',
      group: 'race',
      optional: false,
      advances: false,
      event: { type: 'race', ref: 'race:half-elf' },
      heldOnly: false,
    },
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
        prompt: 'character/class',
        choose: 1,
        kind: 'class',
        from: { kind: 'collection', collection: 'class' },
      },
      group: 'class',
      optional: false,
      advances: false,
      event: { type: 'class', level: 1 },
      heldOnly: false,
    },
  ],
}

const PARTWAY_LOG = {
  seq: 3,
  events: [
    INIT,
    { seq: 2, type: 'race', source: 'race', ref: 'race:half-elf' },
    { seq: 3, type: 'subrace', source: 'race', ref: 'subrace:hill-dwarf' },
  ],
}

/** A follow-up entry: the half-elf's ability bonuses, answered separately. */
const ANSWERED_LOG = {
  seq: 3,
  events: [
    INIT,
    { seq: 2, type: 'race', source: 'race', ref: 'race:half-elf' },
    {
      seq: 3,
      type: 'race',
      source: 'race',
      ref: 'race:half-elf',
      choices: [{ prompt: 'half-elf/ability-bonus/0', picks: ['dex', 'con'] }],
    },
  ],
}

/** Two levels of rogue, so there is a level to un-take. */
const LEVELLED_LOG = {
  seq: 5,
  events: [
    INIT,
    { seq: 2, type: 'race', source: 'race', ref: 'race:half-elf' },
    { seq: 3, type: 'background', source: 'background', ref: 'background:acolyte' },
    { seq: 4, type: 'class', source: 'class', ref: 'class:rogue', level: 1 },
    { seq: 5, type: 'level', source: 'advance', ref: 'class:rogue', level: 2 },
  ],
}

/** What the server asks next once the race is settled: a different category. */
const AFTER_RACE = { seq: 2, complete: false, prompts: [PARTWAY.prompts[1]] }

/**
 * An alignment, which is the one *input* asked as a choice.
 *
 * Captured from the running server: it hangs off the background, it is
 * optional, and the entry it names is a plain `change` -- there is no ref for
 * the answer to travel in, because an alignment is a value on the sheet rather
 * than a catalogue entry the character points at.
 */
const ALIGNMENT = {
  seq: 2,
  complete: false,
  prompts: [
    {
      choice: {
        prompt: 'character/alignment',
        choose: 1,
        kind: 'alignment',
        from: { kind: 'collection', collection: 'alignment' },
      },
      group: 'background',
      optional: true,
      advances: false,
      event: { type: 'change' },
      heldOnly: false,
    },
  ],
}

const ALIGNMENTS = [
  { slug: 'neutral', name: 'Neutral' },
  { slug: 'lawful-good', name: 'Lawful Good' },
]

const BACKGROUND_LOG = {
  seq: 2,
  events: [INIT, { seq: 2, type: 'background', source: 'background', ref: 'background:acolyte' }],
}

/** The same character with the alignment answered, as the server stores it. */
const ALIGNED_LOG = {
  seq: 3,
  events: [
    ...BACKGROUND_LOG.events,
    {
      seq: 3,
      type: 'change',
      source: 'background',
      changes: [{ path: 'identity.alignment', op: 'set', value: { kind: 'slug', slug: 'neutral' } }],
    },
  ],
}

/**
 * The race tab partway: the race settled, two questions open under it.
 *
 * The language is listed first and the subrace second, which is the order the
 * screen has to keep: answering the subrace must not promote its entry above
 * the language just because entries sort before questions.
 */
const RACE_LOG = {
  seq: 2,
  events: [INIT, { seq: 2, type: 'race', source: 'race', ref: 'race:half-elf' }],
}

const LANGUAGE_PROMPT = {
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
}

const SUBRACE_PROMPT = {
  choice: {
    prompt: 'character/subrace',
    choose: 1,
    kind: 'subrace',
    from: { kind: 'collection', collection: 'subrace' },
  },
  group: 'race',
  optional: false,
  advances: false,
  event: { type: 'subrace' },
  heldOnly: false,
}

const DWARF_SKILL_PROMPT = {
  choice: {
    prompt: 'hill-dwarf/proficiency/0',
    choose: 1,
    kind: 'proficiency',
    from: { kind: 'explicit', options: [{ key: 'skill-insight', kind: 'ref', ref: 'skill:insight' }] },
  },
  source: 'subrace:hill-dwarf',
  group: 'race',
  optional: false,
  advances: false,
  event: { type: 'subrace', ref: 'subrace:hill-dwarf' },
  heldOnly: false,
}

const OPEN_UNDER_RACE = { seq: 2, complete: false, prompts: [LANGUAGE_PROMPT, SUBRACE_PROMPT] }

/** What the subrace brought with it, and what it left alone. */
const AFTER_SUBRACE = { seq: 3, complete: false, prompts: [LANGUAGE_PROMPT, DWARF_SKILL_PROMPT] }

const SUBRACED_LOG = {
  seq: 3,
  events: [
    ...RACE_LOG.events,
    { seq: 3, type: 'subrace', source: 'race', ref: 'subrace:hill-dwarf' },
  ],
}

/** Nothing required left: the only prompt is the standing offer of a level. */
const FINISHED = {
  seq: 5,
  complete: true,
  prompts: [
    {
      choice: {
        prompt: 'character/level',
        choose: 1,
        kind: 'level',
        from: { kind: 'collection', collection: 'class' },
      },
      group: 'advance',
      optional: true,
      advances: true,
      event: { type: 'level' },
      heldOnly: false,
    },
  ],
}

/** The six scores as their own open question, answered from the abilities tab. */
const UNSCORED = {
  seq: 1,
  complete: false,
  prompts: [
    {
      choice: {
        prompt: 'character/abilities',
        choose: 6,
        kind: 'ability-scores',
        from: { kind: 'explicit' },
      },
      group: 'abilities',
      optional: false,
      advances: false,
      event: { type: 'change' },
      heldOnly: false,
    },
  ],
}

const SHEET = {
  identity: { name: 'Zephyr', level: 1 },
  base: { hitPoints: { current: 9, max: 9 }, deathSaves: { successes: 0, failures: 0 } },
  abilities: { scores: {}, modifiers: {} },
  skills: {},
  savingThrows: {},
  status: { armorClass: 10, initiative: 0, proficiencyBonus: 2, passivePerception: 10 },
  equipment: { equipped: [], backpack: [], loot: [] },
  resources: {},
  spells: {},
  actions: [],
}

interface Wire {
  prompts?: unknown
  /** What `/prompts` answers once something has been written. */
  then?: unknown
  events?: unknown
  /** What `/events` answers once something has been written. */
  thenEvents?: unknown
  dropped?: unknown[]
  created?: unknown
}

let posted: { url: string; method: string; body: unknown }[] = []

/**
 * Answers everything one build screen asks for.
 *
 * `/events` and `/sheet` are not optional extras here: the screen reads all
 * three in one round, so a mock that answers only `/prompts` leaves it in its
 * loading state and every test in the file times out saying nothing useful.
 */
function mockApi({
  prompts = RACE_PROMPT,
  then,
  events = LOG_JUST_CREATED,
  thenEvents,
  dropped,
  created,
}: Wire = {}) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      const method = init?.method ?? 'GET'
      if (method !== 'GET') {
        posted.push({ url, method, body: JSON.parse(String(init?.body ?? '{}')) })
        if (url.endsWith('/v1/characters')) {
          return jsonResponse(created ?? { id: 'chr_000009', seq: 1, sheet: SHEET })
        }
        // The log's new head, which for a single appended event names that
        // event -- the screen reads it to know what its answer was stored as.
        const head = (thenEvents as { seq?: number } | undefined)?.seq ?? 2
        return jsonResponse({ seq: head, sheet: SHEET, ...(dropped ? { dropped } : {}) })
      }
      if (url.includes('/prompts')) {
        return jsonResponse(then !== undefined && posted.length > 0 ? then : prompts)
      }
      if (url.includes('/events')) {
        return jsonResponse(thenEvents !== undefined && posted.length > 0 ? thenEvents : events)
      }
      if (url.includes('/sheet')) return jsonResponse(SHEET)
      if (url.includes('/catalog/races')) return jsonResponse(RACES)
      if (url.includes('/catalog/subraces')) return jsonResponse(SUBRACES)
      if (url.includes('/catalog/alignments')) return jsonResponse(ALIGNMENTS)
      return jsonResponse([])
    }),
  )
}

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

function renderBuild(viewport: 'mobile' | 'desktop') {
  return renderAt(
    viewport,
    <MemoryRouter initialEntries={['/characters/chr_000001/build']}>
      <Routes>
        <Route path="/characters/:id/build" element={<BuildScreen />} />
        <Route path="/characters/:id" element={<div>sheet</div>} />
      </Routes>
    </MemoryRouter>,
  )
}

function renderNew(viewport: 'mobile' | 'desktop') {
  return renderAt(
    viewport,
    <MemoryRouter initialEntries={['/characters/new']}>
      <Routes>
        <Route path="/characters/new" element={<BuildScreen />} />
        <Route path="/characters/:id/build" element={<BuildScreen />} />
        <Route path="/characters/:id" element={<div>sheet</div>} />
      </Routes>
    </MemoryRouter>,
  )
}

const tabs = () => screen.getAllByRole('tab').map((tab) => tab.textContent ?? '')
const tab = (name: string) => screen.getByRole('tab', { name })
const writes = () => posted.filter((write) => !write.url.includes('dryRun'))

/**
 * One block on the tab, by what its header says.
 *
 * Every block is a disclosure control, decided or open alike, which is the
 * whole of the redesign: there is one gesture on this screen and it is
 * pressing the thing you want to deal with.
 */
const block = (name: RegExp) => screen.getByRole('button', { name })

beforeEach(() => {
  // No resetCatalogCache and no unstubAllGlobals: src/test/setup.ts does both
  // after every test in the suite, which is where they have to be anyway.
  posted = []
  mockApi()
})

/**
 * One viewport. Nothing this screen draws branches on width -- only `Columns`,
 * `DataList`, `ModalSheet` and `RootShell` do, and the suite runs without CSS,
 * so a responsive prop cannot move the DOM either. The one exception is the
 * sheet that prices a change, which is a `ModalSheet`; the two tests that open
 * it are in their own block at the foot of this file. See docs/web.md.
 */
describe('BuildScreen', () => {
  const viewport = 'desktop'

  it('names the question the server said was next, and opens it when pressed', async () => {
    const user = setupUser()
    renderBuild(viewport)

    // Named rather than asked, and shut until somebody asks for it: the screen
    // does not know which of the open choices anybody came here to make.
    const race = await screen.findByRole('button', { name: /A race/ })
    expect(screen.queryByRole('button', { name: 'Half-Elf' })).not.toBeInTheDocument()

    await user.click(race)

    // Options come from the collection the prompt named.
    expect(await screen.findByRole('button', { name: 'Half-Elf' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Dwarf' })).toBeInTheDocument()
  })

  it('posts the event the prompt specified, not one it decided on', async () => {
    const user = setupUser()
    renderBuild(viewport)

    await user.click(await screen.findByRole('button', { name: /A race/ }))
    await user.click(await screen.findByRole('button', { name: 'Half-Elf' }))
    await user.click(screen.getByRole('button', { name: /^confirm$/i }))

    await waitFor(() => {
      expect(posted).toHaveLength(1)
    })
    const write = posted[0]
    expect(write?.url).toContain('/characters/chr_000001/events')
    // The sequence the client believed the log ended at: without it, two
    // clients editing one character would clobber each other silently.
    expect(write?.body).toMatchObject({
      expectedSeq: 1,
      events: [{ type: 'race', ref: 'race:half-elf' }],
    })
  })

  it('answers in place, and lets new questions arrive underneath', async () => {
    const user = setupUser()
    mockApi({
      prompts: OPEN_UNDER_RACE,
      then: AFTER_SUBRACE,
      events: RACE_LOG,
      thenEvents: SUBRACED_LOG,
    })
    renderBuild(viewport)

    await user.click(await screen.findByRole('tab', { name: 'race' }))
    const drawn = () => screen.getByRole('tabpanel').textContent ?? ''
    await waitFor(() => {
      expect(drawn()).toContain('A subrace')
    })
    const before = drawn()
    expect(before.indexOf('Race chosen')).toBeLessThan(before.indexOf('One more language'))
    expect(before.indexOf('One more language')).toBeLessThan(before.indexOf('A subrace'))

    await user.click(screen.getByRole('button', { name: /A subrace/ }))
    await user.click(await screen.findByRole('button', { name: 'Hill Dwarf' }))
    await user.click(screen.getByRole('button', { name: /^confirm$/i }))

    await waitFor(() => {
      expect(drawn()).toContain('Subrace chosen')
    })
    const after = drawn()
    // The answer is where the question was -- under the language, not promoted
    // above it because entries sort before questions -- and what the subrace
    // brought with it is at the bottom. Nothing else moved.
    expect(after.indexOf('Race chosen')).toBeLessThan(after.indexOf('One more language'))
    expect(after.indexOf('One more language')).toBeLessThan(after.indexOf('Subrace chosen'))
    expect(after.indexOf('Subrace chosen')).toBeLessThan(after.indexOf('to be proficient in'))
  })

  it('offers the way on at the end of a finished tab', async () => {
    const user = setupUser()
    mockApi({ prompts: PARTWAY, events: PARTWAY_LOG })
    renderBuild(viewport)

    // The class tab has something open, so there is nothing to move on from.
    await screen.findByText('A class')
    expect(screen.queryByRole('button', { name: 'Next' })).not.toBeInTheDocument()

    // The identity tab is finished: the name is settled and nothing is open.
    await user.click(tab('identity'))
    await user.click(await screen.findByRole('button', { name: 'Next' }))

    // On to the next category with something required outstanding, which is
    // the class -- identity is where we were and abilities has nothing open.
    expect(tab('class')).toHaveAttribute('aria-selected', 'true')
  })

  it('leaves nothing open once an answer has landed', async () => {
    const user = setupUser()
    renderBuild(viewport)

    await user.click(await screen.findByRole('button', { name: /A race/ }))
    await user.click(await screen.findByRole('button', { name: 'Half-Elf' }))
    await user.click(screen.getByRole('button', { name: /^confirm$/i }))

    // Answering is finishing with a question, not moving to the next one: the
    // list comes back shut, and the player says what they want to do next.
    await waitFor(() => {
      expect(screen.queryByRole('button', { name: 'Half-Elf' })).not.toBeInTheDocument()
    })
  })

  it('closes what was open when the tab changes', async () => {
    const user = setupUser()
    mockApi({ prompts: PARTWAY, events: PARTWAY_LOG })
    renderBuild(viewport)

    await user.click(await screen.findByRole('tab', { name: 'race' }))
    await user.click(block(/Two to be proficient in/))
    expect(await screen.findByRole('button', { name: /Acrobatics/ })).toBeInTheDocument()

    // A different tab is a different question, so the one in hand is dropped
    // rather than waiting underneath for a return that may never come.
    await user.click(tab('background'))
    await user.click(tab('race'))
    expect(screen.queryByRole('button', { name: /Acrobatics/ })).not.toBeInTheDocument()
  })

  it('offers every category as a tab, in order, and disables none', async () => {
    renderBuild(viewport)
    await screen.findByText('A race')

    // Class first after the name -- it is the choice the most other choices
    // hang off -- and the scores straight after it, because they are what the
    // class was picked for. Nothing is disabled, because a tab is a place to
    // look as well as a place to answer.
    expect(tabs()).toEqual(['identity', 'class', 'abilities', 'race', 'background'])
    for (const each of screen.getAllByRole('tab')) expect(each).not.toBeDisabled()
  })

  // One mount, walked across three tabs: each shows only what belongs to it,
  // and looking never posts. Splitting this would mount the same PARTWAY
  // character twice to read two of the three.
  it('shows a tab only the choices that belong to it, and posts nothing for looking', async () => {
    const user = setupUser()
    mockApi({ prompts: PARTWAY, events: PARTWAY_LOG })
    renderBuild(viewport)

    // The class tab opens first: it is the first category in order with
    // something required outstanding.
    expect.soft(await screen.findByText('A class')).toBeInTheDocument()
    expect.soft(screen.queryByText('A background')).not.toBeInTheDocument()

    await user.click(tab('race'))
    expect.soft(screen.getByText(/Two to be proficient in/)).toBeInTheDocument()
    expect.soft(screen.queryByText('A class')).not.toBeInTheDocument()

    await user.click(screen.getByRole('tab', { name: 'identity' }))
    expect.soft(screen.getByText('Name')).toBeInTheDocument()
    // "Nothing left here", never "nothing left in identity": the category's
    // word belongs to its tab and appears exactly once on the page.
    expect.soft(screen.getByText('Nothing left here.')).toBeInTheDocument()

    expect.soft(posted).toHaveLength(0)
  })

  it('posts the event the prompt named when a choice is answered from the list', async () => {
    const user = setupUser()
    mockApi({ prompts: PARTWAY, events: PARTWAY_LOG })
    renderBuild(viewport)

    await user.click(await screen.findByRole('tab', { name: 'race' }))
    await user.click(screen.getByRole('button', { name: /Two to be proficient in/ }))
    await user.click(await screen.findByRole('button', { name: /Acrobatics/ }))
    await user.click(screen.getByRole('button', { name: /Insight/ }))
    await user.click(screen.getByRole('button', { name: /^confirm$/i }))

    await waitFor(() => {
      expect(writes()).toHaveLength(1)
    })
    // The follow-up hangs off the entry that opened it, so it carries the
    // same ref -- which the client copies rather than works out.
    expect(writes()[0]?.body).toMatchObject({
      expectedSeq: 3,
      events: [
        {
          type: 'race',
          ref: 'race:half-elf',
          choices: [
            { prompt: 'skill-versatility/proficiency/0', picks: ['skill-acrobatics', 'skill-insight'] },
          ],
        },
      ],
    })
  })

  it('stays on the tab an answer was given on, whatever is asked next', async () => {
    const user = setupUser()
    mockApi({ then: AFTER_RACE })
    renderBuild(viewport)

    expect(await screen.findByRole('tab', { name: 'race' })).toHaveAttribute(
      'aria-selected',
      'true',
    )
    await user.click(await screen.findByRole('button', { name: /A race/ }))
    await user.click(await screen.findByRole('button', { name: 'Half-Elf' }))
    await user.click(screen.getByRole('button', { name: /^confirm$/i }))

    // The only question left is on another tab, and the screen still does not
    // go there: answering is not a request to be moved on, and what a class
    // or a race brought with it is usually the thing you want to look at.
    await waitFor(() => {
      expect(screen.queryByRole('button', { name: 'Half-Elf' })).not.toBeInTheDocument()
    })
    expect(tab('race')).toHaveAttribute('aria-selected', 'true')
    expect(screen.queryByText('A background')).not.toBeInTheDocument()
  })

  it('finishes to the sheet', async () => {
    const user = setupUser()
    renderBuild(viewport)

    await screen.findByText('A race')
    await user.click(screen.getByRole('button', { name: 'Finish' }))

    expect(await screen.findByText('sheet')).toBeInTheDocument()
  })

  // Two readings of the same finished character, from one mount: what the
  // bottom row offers, and what the class tab says about the level it took.
  // Neither writes anything, so re-mounting to ask the second question would
  // buy nothing. expect.soft so a failure in the first still reports the rest.
  it('offers Finish, and shows a level already taken as a fact rather than an offer', async () => {
    const user = setupUser()
    mockApi({ prompts: FINISHED, events: LEVELLED_LOG })
    renderBuild(viewport)

    // Finish is the only control on the row: there is no Next, because the
    // order to answer things in is the player's and the tabs already say what
    // the categories are.
    const finish = await screen.findByRole('button', { name: 'Finish' })
    expect.soft(finish).toHaveAttribute('data-variant', 'filled')
    expect.soft(screen.queryByRole('button', { name: 'Next' })).not.toBeInTheDocument()
    // The one prompt the server still poses is the offer of a level, which
    // this client does not offer -- so there is genuinely nothing here.
    expect.soft(screen.getByText('Nothing left here.')).toBeInTheDocument()

    // Advancement is the class story continued, so a level that was taken
    // sits on the class tab rather than under a tab of its own.
    await user.click(screen.getByRole('tab', { name: 'class' }))
    expect.soft(screen.getByText('Level gained')).toBeInTheDocument()

    // Read-only, and the standing offer of another one is not on the page at
    // all: level-up does not work, and a question that silently does nothing
    // is worse than a question that is not asked. A level that was taken is a
    // fact about the character, so it is not even a block that opens.
    expect.soft(screen.queryByRole('button', { name: /Level gained/ })).not.toBeInTheDocument()
    expect.soft(screen.queryByText(/Another level/)).not.toBeInTheDocument()
    expect.soft(screen.getByText('Nothing left here.')).toBeInTheDocument()

    // The class itself is still a choice like any other.
    expect.soft(block(/Class chosen/)).toBeInTheDocument()
  })

  it('makes a change that costs nothing else without asking about it', async () => {
    const user = setupUser()
    mockApi({ prompts: PARTWAY, events: PARTWAY_LOG, dropped: [] })
    renderBuild(viewport)

    await user.click(await screen.findByRole('tab', { name: 'race' }))
    await user.click(block(/Race chosen/))
    await user.click(await screen.findByRole('button', { name: 'Dwarf' }))
    await user.click(screen.getByRole('button', { name: /^confirm$/i }))

    // The price is still quoted -- the same PUT with the flag -- and it comes
    // back as nothing, so the change is simply made. Confirming every change
    // teaches people to confirm without reading, which is the habit the one
    // change that does cost something needs them not to have.
    await waitFor(() => {
      expect(writes()).toHaveLength(1)
    })
    expect(screen.queryByText(/depend on this/)).not.toBeInTheDocument()
    expect(posted[0]?.url).toContain('dryRun=true')
    const commit = writes()[0]
    expect(commit?.method).toBe('PUT')
    expect(commit?.url).toContain('/characters/chr_000001/events/2')
    expect(commit?.url).not.toContain('dryRun')
    // expectedSeq goes again, so a log that moved since the preview is a
    // sequence conflict rather than a silent commit of a stale price.
    expect(commit?.body).toMatchObject({
      expectedSeq: 3,
      event: { type: 'race', ref: 'race:dwarf' },
    })
  })

  it('changes the name by replacing the entry that holds it', async () => {
    const user = setupUser()
    mockApi({ prompts: PARTWAY, events: PARTWAY_LOG, dropped: [] })
    renderBuild(viewport)

    await user.click(await screen.findByRole('tab', { name: 'identity' }))
    await user.click(block(/Name/))

    // The field starts from the name it is changing rather than from nothing.
    const field = screen.getByLabelText('Name')
    expect(field).toHaveValue('Zephyr')
    await user.clear(field)
    await user.type(field, 'Rurik')
    await user.click(screen.getByRole('button', { name: 'Change it' }))

    // Seq 1 is replaceable when the replacement is also an init event, and
    // that is the whole of renaming: nothing after it moves. Nothing depends
    // on a name either, so nobody is asked whether they meant it.
    await waitFor(() => {
      expect(writes()).toHaveLength(1)
    })
    expect(posted[0]?.url).toContain('/events/1?dryRun=true')
    expect(writes()[0]?.url).toContain('/events/1')
    expect(writes()[0]?.body).toMatchObject({
      event: {
        type: 'init',
        changes: [{ path: 'identity.name', value: { kind: 'string', string: 'Rurik' } }],
      },
    })
  })

  it('settles an alignment as the change that settles it, not as a reference', async () => {
    const user = setupUser()
    mockApi({ prompts: ALIGNMENT, events: BACKGROUND_LOG })
    renderBuild(viewport)

    await user.click(await screen.findByRole('button', { name: /An alignment/ }))
    await user.click(await screen.findByRole('button', { name: 'Neutral' }))
    await user.click(screen.getByRole('button', { name: /^confirm$/i }))

    await waitFor(() => {
      expect(posted).toHaveLength(1)
    })
    // An alignment is one of the character's inputs: it is a value on the
    // sheet, so the entry that settles it is the change that sets it. The
    // prompt is namespaced `character/` like a race is, and posting a `change`
    // event that named an alignment was accepted, attributed to no prompt and
    // silently changed nothing.
    expect(posted[0]?.body).toEqual({
      expectedSeq: 2,
      events: [
        {
          type: 'change',
          changes: [
            { path: 'identity.alignment', op: 'set', value: { kind: 'slug', slug: 'neutral' } },
          ],
        },
      ],
    })
  })

  it('puts the alignment question again from the entry that settled it', async () => {
    const user = setupUser()
    mockApi({ prompts: { seq: 3, complete: false, prompts: [] }, events: ALIGNED_LOG, dropped: [] })
    renderBuild(viewport)

    await user.click(await screen.findByRole('tab', { name: 'background' }))
    // It reads as what was decided rather than as the patch that recorded it.
    await user.click(block(/Alignment/))

    await user.click(await screen.findByRole('button', { name: 'Lawful Good' }))
    await user.click(screen.getByRole('button', { name: /^confirm$/i }))

    await waitFor(() => {
      expect(writes()).toHaveLength(1)
    })
    expect(posted[0]?.url).toContain('/events/3?dryRun=true')
    expect(writes()[0]?.body).toMatchObject({
      event: {
        type: 'change',
        changes: [
          { path: 'identity.alignment', op: 'set', value: { kind: 'slug', slug: 'lawful-good' } },
        ],
      },
    })
  })

  it('answers the six scores as their own entry, from the abilities tab', async () => {
    const user = setupUser()
    mockApi({ prompts: UNSCORED, events: LOG_JUST_CREATED })
    renderBuild(viewport)

    // The scores are an ordinary open choice now, not a field on a create
    // form, which is what gives them an entry to point at and change.
    await user.click(await screen.findByRole('button', { name: /Six ability scores/ }))
    expect(tab('abilities')).toHaveAttribute('aria-selected', 'true')

    // The array is dealt out rather than typed: six printed numbers, and the
    // decision is which ability gets which. Nothing can be confirmed until
    // all six have been put somewhere.
    expect(await screen.findByRole('button', { name: 'Place all six' })).toBeDisabled()
    const place = async (value: string, ability: RegExp) => {
      await user.click(screen.getByRole('button', { name: value }))
      await user.click(screen.getByRole('button', { name: ability }))
    }
    await place('15', /Strength/)
    await place('14', /Dexterity/)
    await place('13', /Constitution/)
    await place('12', /Intelligence/)
    await place('10', /Wisdom/)
    await place('8', /Charisma/)

    await user.click(screen.getByRole('button', { name: 'Confirm' }))

    await waitFor(() => {
      expect(posted).toHaveLength(1)
    })
    // The array as it was dealt out, and the method travels with the answer
    // rather than with creation.
    expect(posted[0]?.body).toEqual({
      expectedSeq: 1,
      events: [
        {
          type: 'change',
          changes: [
            { path: 'abilities.method', op: 'set', value: { kind: 'slug', slug: 'standard-array' } },
            { path: 'abilities.str', op: 'set', value: { kind: 'int', int: 15 } },
            { path: 'abilities.dex', op: 'set', value: { kind: 'int', int: 14 } },
            { path: 'abilities.con', op: 'set', value: { kind: 'int', int: 13 } },
            { path: 'abilities.int', op: 'set', value: { kind: 'int', int: 12 } },
            { path: 'abilities.wis', op: 'set', value: { kind: 'int', int: 10 } },
            { path: 'abilities.cha', op: 'set', value: { kind: 'int', int: 8 } },
          ],
        },
      ],
    })
  })
})

/**
 * Creating, which is the identity tab with nothing behind it yet.
 *
 * Net-new coverage: the create screen this replaced had no tests at all.
 */
describe('a new character', () => {
  const viewport = 'desktop'

  it('asks for a name and nothing else', async () => {
    renderNew(viewport)

    // The one block that opens itself: there is nothing behind it, and a front
    // door whose only row is shut reads as broken.
    expect(await screen.findByText('What are they called?')).toBeInTheDocument()
    expect(screen.getByLabelText('Name')).toBeInTheDocument()
    // The scores are a question asked of a character that exists, not a field
    // on the form that creates one.
    expect(screen.queryByText(/ability scores/)).not.toBeInTheDocument()
    expect(tabs()).toEqual(['identity', 'class', 'abilities', 'race', 'background'])
  })

  it('creates the character once, with the name alone, when a tab is clicked', async () => {
    const user = setupUser()
    renderNew(viewport)

    await user.type(await screen.findByLabelText('Name'), 'Rurik')
    await user.click(tab('class'))

    await waitFor(() => {
      expect(screen.getByText('A race')).toBeInTheDocument()
    })
    const creates = posted.filter((write) => write.url.endsWith('/v1/characters'))
    expect(creates).toHaveLength(1)
    expect(creates[0]?.body).toEqual({ name: 'Rurik' })

    // The URL was replaced, so nothing on the built screen creates a second one.
    await user.click(tab('background'))
    expect(posted.filter((write) => write.url.endsWith('/v1/characters'))).toHaveLength(1)
  })

  it('posts nothing for a blank name, and says why', async () => {
    const user = setupUser()
    renderNew(viewport)

    await screen.findByLabelText('Name')
    await user.click(tab('abilities'))

    expect(posted).toHaveLength(0)
    expect(screen.getByText(/needs a name/)).toBeInTheDocument()
  })
})

/**
 * The two that do need both widths.
 *
 * `ModalSheet` is a `Modal` on a desktop and a `Drawer` on a phone
 * (ModalSheet.tsx:24), and these are the only tests on this screen that open
 * one. `src/ui/ModalSheet.test.tsx` covers the swap itself; what these cover is
 * that the price and its buttons survive being drawn in either container.
 */
describe.each(['mobile', 'desktop'] as const)('pricing a change at %s', (viewport) => {
  it('prices a change before making it, and makes nothing on Cancel', async () => {
    const user = setupUser()
    mockApi({
      prompts: PARTWAY,
      events: PARTWAY_LOG,
      dropped: [
        {
          seq: 3,
          type: 'subrace',
          ref: 'subrace:hill-dwarf',
          source: 'race',
          reason: 'not-offered',
        },
      ],
    })
    renderBuild(viewport)

    await user.click(await screen.findByRole('tab', { name: 'race' }))
    // Pressing the decided block is the change gesture: it puts the question
    // again where the answer is, rather than opening a surface elsewhere.
    await user.click(block(/Race chosen/))
    await user.click(await screen.findByRole('button', { name: 'Dwarf' }))
    await user.click(screen.getByRole('button', { name: /^confirm$/i }))

    // The dry run names the orphan before the click that would orphan it.
    expect(await screen.findByText(/Subrace chosen: Hill Dwarf/)).toBeInTheDocument()
    expect(screen.getByText('no longer offered')).toBeInTheDocument()
    expect(posted).toHaveLength(1)
    expect(posted[0]?.method).toBe('PUT')
    expect(posted[0]?.url).toContain('/events/2?dryRun=true')

    await user.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(writes()).toHaveLength(0)
  })

  it('puts a nested question again by dropping its entry, and says what that costs', async () => {
    const user = setupUser()
    mockApi({
      prompts: PARTWAY,
      events: ANSWERED_LOG,
      dropped: [{ seq: 3, type: 'race', ref: 'race:half-elf', source: 'race', reason: 'empty' }],
    })
    renderBuild(viewport)

    await user.click(await screen.findByRole('tab', { name: 'race' }))
    // The block reads as what was picked, not as the race it hangs off: the
    // ref names what posed the question, and the answers are the selection.
    await user.click(block(/Half Elf · Ability Bonus/))

    // A nested prompt cannot be re-posed from here -- its options came with a
    // prompt the server stopped emitting -- so opening the block drops the
    // entry instead, which reaches the same place: the question comes back
    // outstanding. No second gesture, and no button to find.
    await waitFor(() => {
      expect(posted).toHaveLength(1)
    })
    expect(posted[0]?.method).toBe('DELETE')
    // expectedSeq travels in the query, not in a body.
    expect(posted[0]?.url).toContain('/events/3?expectedSeq=3&dryRun=true')
    expect(posted[0]?.body).toEqual({})

    // This one costs another answer, so it is asked about before it is made.
    expect(await screen.findByText(/waiting under whichever tab/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Ask it again' }))

    await waitFor(() => {
      expect(writes()).toHaveLength(1)
    })
    expect(writes()[0]?.url).toContain('/events/3?expectedSeq=3')
    expect(writes()[0]?.url).not.toContain('dryRun')
  })
})
