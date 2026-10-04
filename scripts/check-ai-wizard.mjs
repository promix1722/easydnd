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
    form.append('instructions', 'Import this character sheet.')
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
    // A question is the wizard saying the sheet does not settle something. Take
    // its first suggestion, as a user in a hurry would; with none offered, say
    // so and let it leave the choice open.
    const question = view.session.events.findLast((event) => event.kind === 'question')
    const text =
      question?.options?.[0] ??
      'The sheet is all I have. Leave that choice open and finish the import.'
    console.log(`> ${text}`)
    await request(
      `/agent-sessions/${id}/control`,
      { revision: view.session.revision, action: 'message', text },
      'POST',
    )
  }
  await new Promise((resolve) => setTimeout(resolve, 3000))
}
assert.equal(view.session.status, 'review', 'Import timed out')
// A pack installed under its own id namespaces every slug ("dnd-2014/rogue").
// The expectations are written in the local names a sheet uses.
// Declarations rather than constants: a session that is already saved is
// checked above this point in the file.
function local(slug) {
  return String(slug ?? '')
    .split('/')
    .pop()
}
function localKeys(object) {
  return Object.fromEntries(Object.entries(object ?? {}).map(([key, value]) => [local(key), value]))
}
export function check(sheet) {
  assert.equal(sheet.identity.name, expected.name)
  assert.deepEqual(sheet.abilities.scores, expected.scores)
  assert.equal(local(sheet.identity.classes?.[0]?.class), expected.class)
  assert.equal(sheet.identity.classes?.[0]?.level, expected.level)
  assert.equal(local(sheet.identity.race), expected.race)
  // The point of the import: what the selected rules have is imported as
  // itself. A subclass or background kept as a custom entry has a "custom-"
  // slug and fails here.
  if (expected.subrace) assert.equal(local(sheet.identity.subrace), expected.subrace)
  if (expected.subclass)
    assert.equal(local(sheet.identity.classes?.[0]?.subclass), expected.subclass)
  if (expected.background) assert.equal(local(sheet.identity.background), expected.background)
  assert.equal(sheet.base.hitPoints.max, expected.hp)
  assert.equal(sheet.status.armorClass, expected.ac)
  const customs = (sheet.customOptions ?? []).filter(
    (option) => option.kind !== 'note' && option.selected !== false,
  )
  const allowed = (expected.allowedCustom ?? []).map(normalize)
  for (const option of customs)
    assert(
      allowed.includes(normalize(option.name)),
      `Custom ${option.kind} "${option.name}" should be native`,
    )
  const cantrips = (sheet.spells.cantrips ?? []).map(local)
  const spells = [...(sheet.spells.known ?? []), ...(sheet.spells.prepared ?? [])].map(local)
  for (const [expectedNames, actualNames] of [
    [expected.cantrips ?? [], cantrips],
    [expected.spells ?? [], spells],
  ])
    for (const spell of expectedNames)
      assert(actualNames.includes(spell), `Missing selected spell ${spell}`)
  const inventory = [
    ...sheet.equipment.equipped,
    ...sheet.equipment.backpack,
    ...sheet.equipment.loot,
  ]
  for (const [item, count] of Object.entries(expected.inventory ?? {})) {
    // The pack carries some items under two slugs; either is the item.
    const slugs = [item, ...(expected.itemAlternatives?.[item] ?? [])]
    const matches = inventory.filter(
      (stack) =>
        slugs.includes(local(stack.item)) ||
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
  const skills = localKeys(sheet.skills)
  for (const [skill, bonus] of Object.entries(expected.skills ?? {}))
    assert.equal(skills[skill]?.bonus, bonus, `Skill: ${skill}`)
  for (const [ability, bonus] of Object.entries(expected.saves ?? {}))
    assert.equal(sheet.savingThrows[ability]?.bonus, bonus, `Save: ${ability}`)
  const notes = [JSON.stringify(sheet.customOptions), ...(sheet.importedNotes ?? [])].join('\n')
  for (const note of expected.notes ?? [])
    assert(notes.toLowerCase().includes(note.toLowerCase()), `Missing source discrepancy ${note}`)
}
// What the saved character is made of: an ordinary build, with printed values
// pinned only where the expectation says the sheet really differs from the
// rules, and no required question left open that it does not name.
async function checkBuild(characterId) {
  const { events } = await request(`/characters/${characterId}/events`)
  const pinned = events
    .filter((event) => event.observed)
    .flatMap((event) => event.changes ?? [])
    .map((change) => change.path.split('.').map(local).join('.'))
    .filter((path) => /^(skills|savingThrows|status|base)\.|^proficiencies$/.test(path))
  for (const path of pinned)
    assert(
      (expected.allowedOverrides ?? []).some((prefix) => path === prefix || path.startsWith(prefix + '.')),
      `Printed value still pinned over the build: ${path}`,
    )
  const { prompts } = await request(`/characters/${characterId}/prompts`)
  // A prompt named after a class carries the pack's namespace in front.
  for (const prompt of prompts.filter((prompt) => !prompt.optional))
    assert(
      (expected.openPrompts ?? []).some(
        (id) => prompt.choice.prompt === id || prompt.choice.prompt.endsWith('/' + id),
      ),
      `Required choice left open: ${prompt.choice.prompt}`,
    )
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
await checkBuild(characterId)
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
// A fully native import has nothing custom to edit, which is the goal rather
// than a gap in the check.
const option = customs.options.find((option) => option.kind === 'background') ?? customs.options[0]
const originalDescription = option?.description
if (option) {
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
}
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
if (option) {
  const finalCustoms = await request(`/characters/${characterId}/custom-options`)
  await request(
    `/characters/${characterId}/custom-options`,
    { revision: finalCustoms.revision, option: { ...option, description: originalDescription } },
    'POST',
  )
}
console.log(`PASS: source checks, native build, save, reopen and edits: ${characterId}`)
