import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { resetCatalogCache } from '@/lib/api'
import { renderAt } from '@/test/render'

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
  events?: unknown
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
function mockApi({ prompts = RACE_PROMPT, events = LOG_JUST_CREATED, dropped, created }: Wire = {}) {
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
        return jsonResponse({ seq: 2, sheet: SHEET, ...(dropped ? { dropped } : {}) })
      }
      if (url.includes('/prompts')) return jsonResponse(prompts)
      if (url.includes('/events')) return jsonResponse(events)
      if (url.includes('/sheet')) return jsonResponse(SHEET)
      if (url.includes('/catalog/races')) return jsonResponse(RACES)
      if (url.includes('/catalog/subraces')) return jsonResponse(SUBRACES)
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

beforeEach(() => {
  posted = []
  resetCatalogCache()
  mockApi()
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe.each(['mobile', 'desktop'] as const)('BuildScreen at %s', (viewport) => {
  it('asks the question the server said was next', async () => {
    renderBuild(viewport)
    expect(await screen.findByText('Choose a race')).toBeInTheDocument()
    // Options come from the collection the prompt named.
    expect(await screen.findByRole('button', { name: /Half-Elf/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Dwarf/ })).toBeInTheDocument()
  })

  it('posts the event the prompt specified, not one it decided on', async () => {
    const user = userEvent.setup()
    renderBuild(viewport)

    await user.click(await screen.findByRole('button', { name: /Half-Elf/ }))
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

  it('offers every category as a tab, in order, and disables none', async () => {
    renderBuild(viewport)
    await screen.findByText('Choose a race')

    // Class first after the name: it is the choice the most other choices
    // hang off. Nothing is disabled, because a tab is a place to look as well
    // as a place to answer.
    expect(tabs()).toEqual(['identity', 'class', 'race', 'background', 'abilities'])
    for (const each of screen.getAllByRole('tab')) expect(each).not.toBeDisabled()
  })

  it('shows a tab only the choices that belong to it', async () => {
    const user = userEvent.setup()
    mockApi({ prompts: PARTWAY, events: PARTWAY_LOG })
    renderBuild(viewport)

    // The class tab opens first: it is the first category in order with
    // something required outstanding.
    expect(await screen.findByText('A class')).toBeInTheDocument()
    expect(screen.queryByText('A background')).not.toBeInTheDocument()

    await user.click(tab('race'))
    expect(screen.getByText(/Two to be proficient in/)).toBeInTheDocument()
    expect(screen.queryByText('A class')).not.toBeInTheDocument()
  })

  it('posts the event the prompt named when a choice is answered from the list', async () => {
    const user = userEvent.setup()
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

  it('shows a tab with nothing open its settled rows, and posts nothing', async () => {
    const user = userEvent.setup()
    mockApi({ prompts: PARTWAY, events: PARTWAY_LOG })
    renderBuild(viewport)

    await user.click(await screen.findByRole('tab', { name: 'identity' }))

    expect(screen.getByText('Name')).toBeInTheDocument()
    // "Nothing left here", never "nothing left in identity": the category's
    // word belongs to its tab and appears exactly once on the page.
    expect(screen.getByText('Nothing left here.')).toBeInTheDocument()
    expect(posted).toHaveLength(0)
  })

  it('skips with Next to the next category with something outstanding', async () => {
    const user = userEvent.setup()
    mockApi({ prompts: PARTWAY, events: PARTWAY_LOG })
    renderBuild(viewport)

    await screen.findByText('A class')
    await user.click(screen.getByRole('button', { name: 'Next' }))

    // identity and abilities have nothing open, so Next passes over them.
    expect(tab('race')).toHaveAttribute('aria-selected', 'true')
  })

  it('finishes to the sheet', async () => {
    const user = userEvent.setup()
    renderBuild(viewport)

    await screen.findByText('Choose a race')
    await user.click(screen.getByRole('button', { name: 'Finish' }))

    expect(await screen.findByText('sheet')).toBeInTheDocument()
  })

  it('gives Finish the weight once nothing is left, and stops Next', async () => {
    mockApi({ prompts: FINISHED, events: LEVELLED_LOG })
    renderBuild(viewport)

    const finish = await screen.findByRole('button', { name: 'Finish' })
    expect(finish).toHaveAttribute('data-variant', 'filled')
    expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled()
    // The one prompt the server still poses is the offer of a level, which
    // this client does not offer -- so there is genuinely nothing here.
    expect(screen.getByText('Nothing left here.')).toBeInTheDocument()
  })

  it('prices a change before making it, and makes nothing on Cancel', async () => {
    const user = userEvent.setup()
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
    await user.click(within(settledRow('Race chosen')).getByRole('button', { name: 'Change' }))
    await user.click(await screen.findByRole('button', { name: /Dwarf/ }))
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

  it('commits the same PUT, without the flag, on confirmation', async () => {
    const user = userEvent.setup()
    mockApi({ prompts: PARTWAY, events: PARTWAY_LOG, dropped: [] })
    renderBuild(viewport)

    await user.click(await screen.findByRole('tab', { name: 'race' }))
    await user.click(within(settledRow('Race chosen')).getByRole('button', { name: 'Change' }))
    await user.click(await screen.findByRole('button', { name: /Dwarf/ }))
    await user.click(screen.getByRole('button', { name: /^confirm$/i }))
    await user.click(await screen.findByRole('button', { name: 'Change it' }))

    await waitFor(() => {
      expect(writes()).toHaveLength(1)
    })
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

  it('shows a level already taken, and offers no way to take or unpick one', async () => {
    const user = userEvent.setup()
    mockApi({ prompts: FINISHED, events: LEVELLED_LOG })
    renderBuild(viewport)

    // Advancement is the class story continued, so a level that was taken
    // sits on the class tab rather than under a tab of its own.
    await user.click(await screen.findByRole('tab', { name: 'class' }))
    expect(settledRow('Level gained')).toBeInTheDocument()

    // Read-only, and the standing offer of another one is not on the page at
    // all: level-up does not work, and a question that silently does nothing
    // is worse than a question that is not asked.
    expect(within(settledRow('Level gained')).queryAllByRole('button')).toHaveLength(0)
    expect(screen.queryByText(/Another level/)).not.toBeInTheDocument()
    expect(screen.getByText('Nothing left here.')).toBeInTheDocument()

    // The class itself is still a choice like any other.
    expect(within(settledRow('Class chosen')).getByRole('button', { name: 'Change' }))
      .toBeInTheDocument()
  })

  it('changes a nested answer by dropping its entry, so the question returns', async () => {
    const user = userEvent.setup()
    mockApi({
      prompts: PARTWAY,
      events: ANSWERED_LOG,
      dropped: [{ seq: 3, type: 'race', ref: 'race:half-elf', source: 'race', reason: 'empty' }],
    })
    renderBuild(viewport)

    await user.click(await screen.findByRole('tab', { name: 'race' }))
    // The row reads as what was picked, not as the race it hangs off: the ref
    // names what posed the question, and the answers are the selection.
    await user.click(
      within(settledRow('Half Elf · Ability Bonus')).getByRole('button', { name: 'Change' }),
    )

    // There is no way to re-pose a nested prompt from here -- its options came
    // with a prompt the server stopped emitting -- so the entry is dropped and
    // the question comes back outstanding under its own tab.
    await waitFor(() => {
      expect(posted).toHaveLength(1)
    })
    expect(posted[0]?.method).toBe('DELETE')
    // expectedSeq travels in the query, not in a body.
    expect(posted[0]?.url).toContain('/events/3?expectedSeq=3&dryRun=true')
    expect(posted[0]?.body).toEqual({})
    expect(await screen.findByText(/waiting under whichever tab/)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Remove it' }))

    await waitFor(() => {
      expect(writes()).toHaveLength(1)
    })
    expect(writes()[0]?.url).toContain('/events/3?expectedSeq=3')
    expect(writes()[0]?.url).not.toContain('dryRun')
  })

  it('changes the name by replacing the entry that holds it', async () => {
    const user = userEvent.setup()
    mockApi({ prompts: PARTWAY, events: PARTWAY_LOG, dropped: [] })
    renderBuild(viewport)

    await user.click(await screen.findByRole('tab', { name: 'identity' }))
    await user.click(within(settledRow('Name')).getByRole('button', { name: 'Change' }))

    const field = screen.getByLabelText('Name')
    await user.clear(field)
    await user.type(field, 'Rurik')
    await user.click(screen.getByRole('button', { name: 'Change it' }))

    // Seq 1 is replaceable when the replacement is also an init event, and
    // that is the whole of renaming: nothing after it moves.
    await waitFor(() => {
      expect(posted).toHaveLength(1)
    })
    expect(posted[0]?.url).toContain('/events/1?dryRun=true')
    expect(posted[0]?.body).toMatchObject({
      event: {
        type: 'init',
        changes: [{ path: 'identity.name', value: { kind: 'string', string: 'Rurik' } }],
      },
    })
  })

  it('answers the six scores as their own entry, from the abilities tab', async () => {
    const user = userEvent.setup()
    mockApi({ prompts: UNSCORED, events: LOG_JUST_CREATED })
    renderBuild(viewport)

    // The scores are an ordinary open choice now, not a field on a create
    // form, which is what gives them an entry to point at and change.
    expect(await screen.findByText('Set the six scores')).toBeInTheDocument()
    expect(tab('abilities')).toHaveAttribute('aria-selected', 'true')

    await user.click(screen.getByRole('button', { name: 'Confirm' }))

    await waitFor(() => {
      expect(posted).toHaveLength(1)
    })
    // The standard array, positional against the order the inputs are drawn
    // in -- and the method travels with the answer rather than with creation.
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

/** The row of "already chosen" whose heading is `label`. */
function settledRow(label: string): HTMLElement {
  const heading = screen.getByText(label)
  const row = heading.closest('div')?.parentElement?.parentElement
  if (row === null || row === undefined) throw new Error(`no settled row for ${label}`)
  return row
}

/**
 * Creating, which is the identity tab with nothing behind it yet.
 *
 * Net-new coverage: the create screen this replaced had no tests at all.
 */
describe.each(['mobile', 'desktop'] as const)('a new character at %s', (viewport) => {
  it('asks for a name and nothing else', async () => {
    renderNew(viewport)

    expect(await screen.findByText('What are they called?')).toBeInTheDocument()
    expect(screen.getByLabelText('Name')).toBeInTheDocument()
    // The scores are a question asked of a character that exists, not a field
    // on the form that creates one.
    expect(screen.queryByText('Set the six scores')).not.toBeInTheDocument()
    expect(tabs()).toEqual(['identity', 'class', 'race', 'background', 'abilities'])
  })

  it('creates the character once, with the name alone, when a tab is clicked', async () => {
    const user = userEvent.setup()
    renderNew(viewport)

    await user.type(await screen.findByLabelText('Name'), 'Rurik')
    await user.click(tab('class'))

    await waitFor(() => {
      expect(screen.getByText('Choose a race')).toBeInTheDocument()
    })
    const creates = posted.filter((write) => write.url.endsWith('/v1/characters'))
    expect(creates).toHaveLength(1)
    expect(creates[0]?.body).toEqual({ name: 'Rurik' })

    // The URL was replaced, so nothing on the built screen creates a second one.
    await user.click(screen.getByRole('button', { name: 'Next' }))
    expect(posted.filter((write) => write.url.endsWith('/v1/characters'))).toHaveLength(1)
  })

  it('posts nothing for a blank name, and says why', async () => {
    const user = userEvent.setup()
    renderNew(viewport)

    await screen.findByLabelText('Name')
    await user.click(tab('abilities'))

    expect(posted).toHaveLength(0)
    expect(screen.getByText(/needs a name/)).toBeInTheDocument()
  })
})
