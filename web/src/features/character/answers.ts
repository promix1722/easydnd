import { kindOf } from '@/domain'
import type { Stage } from '@/domain'
import type { Answer, CharacterEvent, Prompt } from '@/lib/api'

import type { Scores } from './AbilityScoresForm'
import { promptKey } from './blocks'
import type { Asking } from './blocks'
import { NEW_NAME_PROMPT } from './buildModel'
import type { Reposed } from './buildModel'
import type { SettledRow } from './settled'

/**
 * The question a dropped entry puts back, named from the entry itself.
 *
 * An answer to a nested prompt cannot be re-posed directly, so opening it drops
 * it and the server emits that prompt again -- under the slug the entry
 * answered, which is the one thing the entry does say.
 */
export function reaskedKey(row: SettledRow): string | null {
  const answered = (row.event.choices ?? [])[0]?.prompt
  return answered === undefined ? null : promptKey(answered)
}

/**
 * The question behind a settled block, where there is one to put again.
 *
 * Null is an answer rather than a failure: a nested prompt cannot be re-posed
 * from here, and the block says so and offers the drop instead.
 */
export function isSpellChoice(row: SettledRow): boolean {
  return row.stage === 'spells' || row.stage === 'cantrips'
}

export function askingFor(row: SettledRow, reposed: Reposed | null): Asking | null {
  const prompt = reask(row) ?? (reposed?.seq === row.seq ? reposed.prompt : null)
  return prompt === null ? null : { prompt, replaces: row }
}

/**
 * The questions whose answer is a value on the sheet rather than a pick.
 *
 * The server's projector calls these the character's *inputs* -- a name, the
 * six ability scores, an alignment -- and applies them before anything derives
 * from them. They are the one family of answer that arrives as an addressed
 * change, because there is nothing for them to hang off: no catalogue entry is
 * being named, and no grant posed them. The prompt says the entry is a
 * `change`; what it cannot say is which path, so the path lives here.
 *
 * A name and the scores have a form each and never reach `eventFor`. An
 * alignment is an ordinary choose-one-of-these, which is exactly why it was
 * wrong for so long: the prompt is namespaced `character/`, like a race and a
 * class, and this screen took the namespace for the shape and posted a
 * `change` event that named an alignment and changed nothing. The server
 * accepted it, attributed it to no prompt, and the alignment stayed unset --
 * the worst kind of failure, which is the silent one.
 */
const INPUTS: readonly {
  prompt: string
  path: string
  kind: string
  /** Absent where the answer is written rather than picked from a set. */
  collection?: string
}[] = [
  {
    prompt: 'character/alignment',
    path: 'identity.alignment',
    kind: 'alignment',
    collection: 'alignment',
  },
  // The desired level and the ruleset are the character's own questions about
  // itself. Both are answered as changes by a form of their own -- see
  // AnswerSurface, which routes them by slug -- so `kind` here only has to
  // survive the round trip through `inputPrompt`.
  { prompt: 'character/desired-level', path: 'identity.desiredLevel', kind: 'level' },
  { prompt: 'character/ruleset', path: 'identity.ruleset', kind: 'text' },
  // The four the player answers in their own words. They are inputs for the
  // same reason the alignment is -- they settle a value and name nothing in
  // the compendium -- and they have no collection because there is nothing to
  // choose between. See features/character/promptNames for what each is called.
  { prompt: 'character/personality-trait', path: 'identity.personalityTraits', kind: 'personality' },
  { prompt: 'character/ideal', path: 'identity.ideals', kind: 'ideal' },
  { prompt: 'character/bond', path: 'identity.bonds', kind: 'bond' },
  { prompt: 'character/flaw', path: 'identity.flaws', kind: 'flaw' },
]

/** The prompt an entry's changes settle, where they settle one. */
function inputOf(event: CharacterEvent): (typeof INPUTS)[number] | undefined {
  const paths = new Set((event.changes ?? []).map((change) => change.path))
  return INPUTS.find((input) => paths.has(input.path))
}

/** The question an input poses, so that answering it again is one mechanism. */
function inputPrompt(input: (typeof INPUTS)[number], stage: Stage): Prompt {
  return {
    choice: {
      prompt: input.prompt,
      choose: 1,
      kind: input.kind,
      from:
        input.collection === undefined
          ? { kind: 'explicit' }
          : { kind: 'collection', collection: input.collection },
    },
    group: stage === 'personal' || stage === 'rules' ? 'identity' : stage,
    optional: false,
      event: { type: 'change' },
    heldOnly: false,
  }
}

/**
 * The question behind a settled entry, posed again -- or null where it cannot
 * be.
 *
 * An entry that names a catalogue reference can be re-asked from the entry
 * alone: the kind of the reference says which collection the answers come
 * from, which is a compendium question rather than a rule. So can the two
 * forms, which pose themselves. What cannot is an answer to a nested prompt --
 * a rogue's Expertise, a half-elf's ability bonuses -- because the options
 * that made it up arrived with a prompt the server stopped emitting the moment
 * it was answered, and rebuilding them here would be this client deciding what
 * an answer means.
 */
/**
 * The entry kinds whose options are narrowed by another entry the character
 * holds, and which this client therefore cannot re-pose.
 *
 * Race, class and background draw on their whole collection, so the question
 * is rebuildable from the answer alone. These two are not.
 */
const NARROWED = ['subrace', 'subclass']

export function reask(row: SettledRow): Prompt | null {
  const event = row.event
  if (event.type === 'init') {
    return { ...NEW_NAME_PROMPT, group: row.stage }
  }
  // Checked before the reference, because an entry with both is a follow-up:
  // the ref names what posed the question, and re-asking *that* would change
  // the wrong selection.
  if ((event.choices ?? []).length > 0) return null
  if (event.ref !== undefined) {
    const kind = kindOf(event.ref)
    // A subrace and a subclass are narrowed by the entry above them: the
    // server offers a half-elf's subraces and a rogue's archetypes, not every
    // one in the compendium. Rebuilding the question here would offer all of
    // them -- Berserker, Champion, Devotion to a rogue -- so it is not
    // rebuilt. Opening the block drops the entry instead, which reaches the
    // same place from the other side: the server poses the question again,
    // with the right list, and the drop is priced like any other.
    if (NARROWED.includes(kind)) return null
    return {
      choice: {
        prompt: `character/${kind}`,
        choose: 1,
        kind,
        from: { kind: 'collection', collection: kind },
      },
      group: row.stage,
      optional: false,
      event: {
        type: event.type,
        ...(event.level !== undefined ? { level: event.level } : {}),
      },
      heldOnly: false,
    }
  }
  const input = inputOf(event)
  if (input !== undefined) {
    return inputPrompt(input, row.stage)
  }
  if (isScores(event)) {
    return {
      choice: {
        prompt: 'character/abilities',
        choose: 6,
        kind: 'ability-scores',
        from: { kind: 'explicit' },
      },
      group: row.stage,
      optional: false,
          event: { type: event.type },
      heldOnly: false,
    }
  }
  return null
}

function isScores(event: CharacterEvent): boolean {
  return (event.changes ?? []).some((change) => change.path.startsWith('abilities.'))
}

/**
 * The scores as they were *stored*, for a form that is changing them.
 *
 * Deliberately not the projected ones on the sheet. Those have the racial
 * bonuses added, and seeding a form from them would add the bonuses a second
 * time the moment it was saved.
 */
export function maybeScores(row: SettledRow | null): { scores?: Scores; method?: string } {
  if (row === null || !isScores(row.event)) return {}
  const scores: Scores = {}
  let method: string | undefined
  for (const change of row.event.changes ?? []) {
    const [head, tail] = change.path.split('.')
    if (head !== 'abilities' || tail === undefined) continue
    if (tail === 'method') method = change.value.slug ?? change.value.string
    else if (change.value.int !== undefined) scores[tail] = change.value.int
  }
  return { scores, ...(method !== undefined ? { method } : {}) }
}

/**
 * What an entry wrote to one path, in the order it wrote it.
 *
 * The list is stored as a `set` followed by `add`s -- see `WrittenForm` -- so
 * reading it back is reading the values in order, and nothing here has to know
 * which op was which.
 */
function linesOf(event: CharacterEvent, path: string): string[] {
  return (event.changes ?? [])
    .filter((change) => change.path === path)
    .flatMap((change) => (change.value.string === undefined ? [] : [change.value.string]))
}

/**
 * What a settled written answer says, for the form that is changing it.
 *
 * Nothing at all for every other kind of entry, which is what keeps the prop
 * off surfaces that would have no use for it.
 */
export function maybeLines(row: SettledRow | null): { lines?: readonly string[] } {
  if (row === null) return {}
  const input = inputOf(row.event)
  if (input === undefined || input.collection !== undefined) return {}
  return { lines: linesOf(row.event, input.path) }
}

/** A name, as the entry that carries one. */
export function initEventFor(name: string): CharacterEvent {
  return {
    type: 'init',
    changes: [
      { path: 'identity.name', op: 'set', value: { kind: 'string', string: name } },
    ],
  }
}

/** The entry a prompt said its answer travels in, filled in with the answer. */
export function eventFor(prompt: Prompt, answers: readonly Answer[]): CharacterEvent {
  // The question itself is the first answer; anything after it answers a
  // branch the first one opened, resolved in the same card.
  const picks = answers[0]?.picks ?? []

  // An input settles a value on the sheet, so its answer is the change that
  // settles it rather than a pick attached to an entry that means nothing.
  const input = INPUTS.find((each) => each.prompt === prompt.choice.prompt)
  if (input !== undefined) {
    return {
      type: prompt.event.type,
      changes: [
        { path: input.path, op: 'set', value: { kind: 'slug', slug: picks[0] ?? '' } },
      ],
    }
  }

  const event: CharacterEvent = {
    type: prompt.event.type,
    ...(prompt.event.ref !== undefined ? { ref: prompt.event.ref } : {}),
    ...(prompt.event.level !== undefined ? { level: prompt.event.level } : {}),
  }
  // A prompt that selects a catalogue entry carries its answer in the event's
  // ref; every other prompt carries it in the choices.
  //
  // All of them, in the order they were given. The server validates a batch
  // answer by answer against a log that grows as each lands, so a branch and
  // what the branch opened travel together: the second is legal because the
  // first one arrived.
  if (selectsTheEventItself(prompt)) {
    event.ref = `${refKindFor(prompt)}:${picks[0] ?? ''}`
  } else {
    event.choices = answers.map((answer) => ({ prompt: answer.prompt, picks: answer.picks }))
  }
  return event
}

/**
 * Whether the answer goes in the event's ref rather than its choices.
 *
 * "Choose a race" is answered by *being* a race event that names one, not by
 * an answer attached to a race event that names nothing.
 *
 * The `character/` namespace is not the test, though it reads like one: the
 * character poses its own inputs under it too, and those are neither. Those
 * are taken by `eventFor` before this is asked.
 */
function selectsTheEventItself(prompt: Prompt): boolean {
  return prompt.choice.prompt.startsWith('character/') || prompt.choice.prompt.endsWith('/subclass')
}

function refKindFor(prompt: Prompt): string {
  const set = prompt.choice.from
  if (set.kind === 'collection' && set.collection !== undefined) return set.collection
  const first = set.options?.[0]
  return first?.ref?.split(':')[0] ?? 'race'
}
