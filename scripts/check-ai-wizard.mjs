// Opt-in, real-provider check. Never imported by default unit tests or CI.
// Uses an already configured development API; creates and edits one character.
import fs from 'node:fs/promises'
import assert from 'node:assert/strict'
const args = Object.fromEntries(
  process.argv
    .slice(2)
    .reduce(
      (pairs, arg, i, all) =>
        arg.startsWith('--') ? [...pairs, [arg.slice(2), all[i + 1]]] : pairs,
      [],
    ),
)
assert(
  (args.pdf || args.session || args['session-file']) && args.expected,
  'Usage: node scripts/check-ai-wizard.mjs --pdf /tmp/Arya.pdf --expected testdata/agent/arya.expected.json [--api http://localhost:18083/v1 --origin http://localhost:8083 --account master]',
)
const api = args.api ?? 'http://localhost:18083/v1'
const origin = args.origin ?? 'http://localhost:8083'
let cookie = ''
async function request(path, body, method = 'GET') {
  const response = await fetch(api + path, {
    method,
    headers: {
      Origin: origin,
      'X-Request-Id': crypto.randomUUID(),
      ...(cookie ? { Cookie: cookie } : {}),
      ...(body && !(body instanceof FormData) ? { 'Content-Type': 'application/json' } : {}),
    },
    body: body instanceof FormData ? body : body ? JSON.stringify(body) : undefined,
  })
  const set = response.headers.get('set-cookie')
  if (set) cookie = set.split(';')[0]
  const data = await response.json()
  assert(response.ok, `${method} ${path}: ${response.status} ${JSON.stringify(data)}`)
  return data
}
const expected = JSON.parse(await fs.readFile(args.expected, 'utf8'))
let view
if (args['session-file']) {
  const session = JSON.parse(await fs.readFile(args['session-file'], 'utf8'))
  cookie = session.cookie
  view = await request(`/agent-sessions/${session.id}`)
} else {
  await request('/dev/login', { account: args.account ?? 'master' }, 'POST')
  if (args.session) {
    view = await request(`/agent-sessions/${args.session}`)
  } else {
    const { defaultRules } = await request('/packs')
    const form = new FormData()
    form.append(
      'files',
      new Blob([await fs.readFile(args.pdf)], { type: 'application/pdf' }),
      args.pdf.split('/').at(-1),
    )
    form.append('rules', JSON.stringify(defaultRules))
    form.append(
      'instructions',
      'Import all pages of this character. Preserve every documented fact, quantity and personal detail. Retain source/rules discrepancies and explain them; preserve unavailable content as typed editable custom options. I explicitly accept leaving undocumented historical choices incomplete. Continue until all documented content is saved in the draft, then call prepare_review with allow_incomplete=true.',
    )
    view = await request('/agent-sessions', form, 'POST')
  }
}
const id = view.session.id
console.log(`Import session ${id}`)
if (view.session.status === 'saved') {
  check(await request(`/characters/${view.session.characterId}/sheet`))
  console.log(`PASS: saved source checks: ${view.session.characterId}`)
  process.exit(0)
}
let seen = 0,
  resumes = 0,
  questions = 0
const deadline = Date.now() + 25 * 60_000
while (Date.now() < deadline && view.session.status !== 'review') {
  view = await request(`/agent-sessions/${id}`)
  for (const event of view.session.events.slice(seen)) {
    if (event.kind === 'progress')
      console.log(`${event.data.field}: ${event.data.value}`.slice(0, 160))
    if (event.kind === 'question') console.log(event.text)
  }
  seen = view.session.events.length
  assert.notEqual(view.session.status, 'failed', 'Provider/import failed')
  if (view.session.status === 'paused') {
    assert(++resumes <= 5, 'Exceeded resumptions')
    await request(
      `/agent-sessions/${id}/control`,
      { revision: view.session.revision, action: 'resume' },
      'POST',
    )
  }
  if (view.session.status === 'waiting') {
    assert(++questions <= 6, 'Repeated unresolved questions')
    await request(
      `/agent-sessions/${id}/control`,
      {
        revision: view.session.revision,
        action: 'message',
        text: 'Continue importing all documented facts. Preserve unknown entities with upsert_custom_option, omitting unknown numeric fields. Do not invent historical choices; I explicitly accept those incomplete. Preserve and explain source discrepancies. After importing all source facts, call prepare_review with allow_incomplete=true; I explicitly accept missing undocumented historical choices.',
      },
      'POST',
    )
  }
  await new Promise((resolve) => setTimeout(resolve, 3000))
}
assert.equal(view.session.status, 'review', 'Import timed out')
export function check(sheet) {
  assert.equal(sheet.identity.name, expected.name)
  assert.deepEqual(sheet.abilities.scores, expected.scores)
  assert.equal(sheet.identity.classes?.[0]?.class, expected.class)
  assert.equal(sheet.identity.classes?.[0]?.level, expected.level)
  assert.equal(sheet.identity.race, expected.race)
  assert.equal(sheet.base.hitPoints.max, expected.hp)
  assert.equal(sheet.status.armorClass, expected.ac)
  for (const name of expected.customNames ?? [])
    assert(
      sheet.customOptions?.some((option) => option.name.toLowerCase() === name.toLowerCase()),
      `Missing editable ${name}`,
    )
  for (const [expectedNames, actualNames] of [
    [expected.cantrips ?? [], sheet.spells.cantrips ?? []],
    [expected.spells ?? [], [...(sheet.spells.known ?? []), ...(sheet.spells.prepared ?? [])]],
  ])
    for (const spell of expectedNames)
      assert(
        actualNames.includes(spell) ||
          sheet.customOptions?.some(
            (option) =>
              option.selected &&
              normalize(option.name) === normalize(spell) &&
              actualNames.includes('custom-' + option.id),
          ),
        `Missing selected spell ${spell}`,
      )
  if (expected.subrace) assert.equal(sheet.identity.subrace, expected.subrace)
  const inventory = [
    ...sheet.equipment.equipped,
    ...sheet.equipment.backpack,
    ...sheet.equipment.loot,
  ]
  for (const [item, count] of Object.entries(expected.inventory ?? {})) {
    const matches = inventory.filter(
      (stack) =>
        stack.item === item ||
        sheet.customOptions?.some(
          (option) =>
            stack.item === 'custom-' + option.id &&
            (expected.inventoryAliases?.[item] ?? []).some(
              (alias) => normalize(alias) === normalize(option.name),
            ),
        ),
    )
    assert.equal(
      matches.reduce((sum, stack) => sum + stack.count, 0),
      count,
      `Quantity of ${item}`,
    )
  }
  for (const [unit, count] of Object.entries(expected.purse ?? {}))
    assert.equal(sheet.equipment.purse?.[unit] ?? 0, count, `Coins: ${unit}`)
  for (const [skill, bonus] of Object.entries(expected.skills ?? {}))
    assert.equal(sheet.skills[skill]?.bonus, bonus, `Skill: ${skill}`)
  for (const [ability, bonus] of Object.entries(expected.saves ?? {}))
    assert.equal(sheet.savingThrows[ability]?.bonus, bonus, `Save: ${ability}`)
  const notes = [JSON.stringify(sheet.customOptions), ...(sheet.importedNotes ?? [])].join('\n')
  for (const note of expected.notes ?? [])
    assert(notes.toLowerCase().includes(note.toLowerCase()), `Missing source discrepancy ${note}`)
}
function normalize(s) {
  return s
    .toLowerCase()
    .replace(/[^a-z0-9]/g, '')
    .replace(/^tashas/, '')
}
check(view.sheet)
view = await request(`/agent-sessions/${id}/finalize`, { revision: view.session.revision }, 'POST')
const characterId = view.session.characterId
assert(characterId)
const before = await request(`/characters/${characterId}/sheet`)
check(before)
let events = await request(`/characters/${characterId}/events`)
const renamed = structuredClone(events.events[0])
renamed.changes = [
  {
    path: 'identity.name',
    op: 'set',
    value: { kind: 'string', string: expected.name + ' edited' },
  },
]
const revision = events.revision ?? events.seq
await request(
  `/characters/${characterId}/events/1`,
  { expectedRevision: revision, expectedSeq: events.seq, event: renamed },
  'PUT',
)
const after = await request(`/characters/${characterId}/sheet`)
after.identity.name = expected.name
assert.deepEqual(after, before, 'Editing the name erased imported facts')
const customs = await request(`/characters/${characterId}/custom-options`)
const option = customs.options.find((option) => option.kind === 'background') ?? customs.options[0]
assert(option, 'No editable custom content')
const originalDescription = option.description
option.description += '\nRound-trip edit check.'
await request(
  `/characters/${characterId}/custom-options`,
  { revision: customs.revision, option },
  'POST',
)
const reopened = await request(`/characters/${characterId}/sheet`)
assert(
  reopened.customOptions.some(
    (saved) => saved.id === option.id && saved.description === option.description,
  ),
  'Custom edit did not survive reopen',
)
assert.deepEqual(reopened.identity.classes, before.identity.classes, 'Custom edit erased class')
const finalEvents = await request(`/characters/${characterId}/events`)
await request(
  `/characters/${characterId}/events/1`,
  {
    expectedRevision: finalEvents.revision,
    expectedSeq: finalEvents.seq,
    event: {
      ...finalEvents.events[0],
      changes: [
        { path: 'identity.name', op: 'set', value: { kind: 'string', string: expected.name } },
      ],
    },
  },
  'PUT',
)
const finalCustoms = await request(`/characters/${characterId}/custom-options`)
await request(
  `/characters/${characterId}/custom-options`,
  { revision: finalCustoms.revision, option: { ...option, description: originalDescription } },
  'POST',
)
console.log(`PASS: source checks, save, reopen, name edit and custom edit: ${characterId}`)
