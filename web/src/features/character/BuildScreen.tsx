import { useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router'

import {
  appendEvents,
  createCharacter,
  deleteEvent,
  getEvents,
  getPrompts,
  getSheet,
  replaceEvent,
} from '@/lib/api'
import type {
  ApiFieldError,
  CharacterEvent,
  Dropped,
  Prompt,
  PromptsResponse,
  Sheet,
} from '@/lib/api'
import { useAction } from '@/lib/useAction'
import { useResource } from '@/lib/useResource'
import {
  Alert,
  Badge,
  Button,
  Group,
  Loader,
  ModalSheet,
  Stack,
  TabRow,
  Text,
  Title,
} from '@/ui'

import {
  blockOrder,
  blocksFor,
  inheritPlace,
  keyFor,
  reclaimPlace,
  settledKey,
} from './blocks'
import type { Asking } from './blocks'
import { resolveRefNames } from './refNames'
import { settledByStage } from './settled'
import type { SettledRow } from './settled'
import { StagePanel } from './StagePanel'
import type { Scores } from './AbilityScoresForm'

import {
  STAGES,
  STAGE_LABELS,
  answerable,
  eventLabel,
  kindOf,
  pickLabel,
  promptLabel,
  stageOf,
} from '@/domain'
import type { Stage } from '@/domain'

/** Everything one build screen reads, in one round of requests. */
interface BuildView {
  prompts: PromptsResponse
  events: CharacterEvent[]
  sheet: Sheet | null
  names: Map<string, string>
}

const EMPTY_VIEW: BuildView = {
  prompts: { seq: 0, complete: false, prompts: [] },
  events: [],
  sheet: null,
  names: new Map(),
}

/** A change, priced before it is paid for. */
interface Preview {
  row: SettledRow
  /** Null for a removal: there is nothing to put back. */
  event: CharacterEvent | null
  dropped: Dropped[]
  names: Map<string, string>
}

/**
 * Creating a character and changing one, which are the same screen because
 * they are the same question to the API.
 *
 * It is still a loop rather than a wizard. Prompts nest -- answering the "two
 * skills" branch of a rogue's Expertise is what brings the two-skill prompt
 * into existence -- so the number of steps is not knowable until the last one
 * is answered, and there is nothing to enumerate. What the tabs enumerate is
 * the server's own prompt *groups*, which are fixed: five categories a
 * question can belong to, not five steps to walk through. Every tab is
 * reachable at any time, and nothing on one can be answered before the server
 * asks it.
 *
 * Three requests, deliberately, because they answer three different questions.
 * `/prompts` says what is still open, `/events` says what was decided and in
 * which entry, and `/sheet` says what all that adds up to -- and the sheet
 * cannot be folded from the log in the browser, because an ability score
 * improvement's increments are derived at projection time.
 *
 * Nothing here decides what an answer means. The server says which event
 * carries it and this copies that verbatim, and the *group* an answer belongs
 * to is written by the server too -- so a dropped answer reappears under its
 * own tab without this screen routing it anywhere.
 */
export function BuildScreen() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  // The folder the party list was filtered to when New character was pressed.
  // Only /characters/new carries it; absent means the account's default, which
  // the server resolves.
  const [search] = useSearchParams()
  const folder = search.get('folder') ?? undefined
  const isNew = id === ''

  const build = useResource<BuildView>(`build:${id}`, async (signal) => {
    if (id === '') return EMPTY_VIEW
    const [prompts, log, sheet] = await Promise.all([
      getPrompts(id, signal),
      getEvents(id, signal),
      getSheet(id, signal),
    ])
    return { prompts, events: log.events, sheet, names: await resolveRefNames(log.events) }
  })

  const create = useAction(createCharacter)
  const answer = useAction(appendEvents)
  const revise = useAction(replaceEvent)
  const remove = useAction(deleteEvent)

  const [chosenStage, setChosenStage] = useState<Stage | null>(null)
  const [openKey, setOpenKey] = useState<string | null>(null)
  const [seeded, setSeeded] = useState(false)
  const [askedOn, setAskedOn] = useState<Stage | null>(null)
  const [nameDraft, setNameDraft] = useState('')
  const [nameError, setNameError] = useState<string | undefined>(undefined)
  const [preview, setPreview] = useState<Preview | null>(null)

  // Held once and mutated, rather than replaced: see BlockOrder. The list is
  // redrawn by new data, never by the memory of where the old data sat.
  const [order] = useState(blockOrder)

  const view = build.data ?? EMPTY_VIEW
  // Advancement is dropped before anything looks at it, so no tab, no list
  // and no Next can reach a question this client cannot honestly answer.
  const open = view.prompts.prompts.filter((prompt) => answerable(prompt.group))
  // Until a tab is clicked the screen opens on the first thing left to do,
  // and moves on as things are answered. That is the loop, kept: a player who
  // never touches a tab is walked through the questions in order, and one who
  // does is pinned where they put themselves.
  const stage = chosenStage ?? firstUnfinished(open)

  // A different tab is a different question, so the block that was open is
  // closed. During render rather than in an effect, so the new tab is never
  // painted once with the old tab's question under it.
  if (askedOn !== stage) {
    setAskedOn(stage)
    setOpenKey(null)
  }
  // The one thing that opens itself, and it opens once. A character that does
  // not exist has a single block on the page and nothing behind it, so there
  // is no other question being pre-empted -- and a front door whose only row
  // is shut reads as broken. Seeded rather than derived, so that closing it
  // closes it. After the reset above, which fires on the very first render.
  if (isNew && !seeded) {
    setSeeded(true)
    setOpenKey(NEW_NAME_KEY)
  }

  const openHere = open.filter((prompt) => stageOf(prompt.group) === stage)
  const settled = settledByStage(view)
  // Before there is a character there is no `/prompts` response, so the
  // identity tab poses the first question itself: see NEW_NAME_PROMPT.
  // The order is the screen's memory of where things are, and `blocksFor`
  // both reads it and writes to it: whatever is new keeps the place the level
  // ordering just gave it, so the next answer moves nothing already on screen.
  const blocks = blocksFor(settled.get(stage) ?? [], isNew ? [NEW_NAME_PROMPT] : openHere, order)
  const opened = blocks.find((block) => block.key === openKey) ?? null
  // What the open block is asking, which is a fact about the block rather than
  // a second piece of state. A settled block whose question cannot be put
  // again has none, and says so where its surface would have been.
  const asking: Asking | null =
    opened === null
      ? null
      : opened.kind === 'open'
        ? { prompt: opened.prompt, replaces: null }
        : askingFor(opened.row)

  const done = () => {
    setOpenKey(null)
    setPreview(null)
    // Pinned to wherever the answer was given. The screen opens on the first
    // category with something to do, and that is the whole of the help it
    // offers: answering one question is not a request to be moved on to
    // another, and a player who has just chosen a class is usually looking at
    // what the class brought with it.
    setChosenStage(stage)
    // Behind the list rather than instead of it: the write is already
    // confirmed, and what is left is to find out what else it changed.
    build.refresh()
  }

  /** Creates the character the identity tab is describing, and moves on. */
  const createCharacterFromDraft = async () => {
    if (create.pending) return
    if (nameDraft.trim() === '') {
      setNameError('A character needs a name to be created under.')
      return
    }
    const created = await create.run({
      name: nameDraft.trim(),
      ...(folder ? { folder } : {}),
    })
    // replace: true, because the URL of a character that does not exist is
    // not a place the Back button should return anyone to.
    if (created) await navigate(`/characters/${created.id}/build`, { replace: true })
  }

  const goToStage = (next: Stage) => {
    if (next === stage) return
    // Nothing else can be answered before the character exists, so a tab click
    // is the same gesture as pressing Next: make it, then go.
    if (isNew) {
      void createCharacterFromDraft()
      return
    }
    setChosenStage(next)
  }

  /** Sends one appended entry, then rereads everything. */
  const append = async (event: CharacterEvent) => {
    const answered = openKey
    const written = await answer.run(id, view.prompts.seq, [event])
    if (written === null) return
    // The entry that just answered a question takes that question's place.
    // A single appended event is the log's new head, so the response's seq
    // names it -- and without this the answer would appear at the bottom of
    // the list while the question it answered vanished from the middle.
    inheritPlace(order, answered, settledKey(written.seq))
    done()
  }

  /**
   * Prices a change, and either makes it or asks.
   *
   * The dry run is the same call the commit makes with `dryRun` set: a price
   * quoted by a second code path is a price that can disagree with what
   * happens, and one that disagrees is worse than none.
   *
   * What the price decides is whether there is anything to ask about. A change
   * that costs nothing else is simply made -- confirming every change taught
   * players to confirm without reading, which is exactly the habit the one
   * change that *does* cost something needs them not to have.
   */
  const price = async (row: SettledRow, event: CharacterEvent | null) => {
    const result =
      event === null
        ? await remove.run(id, row.seq, view.prompts.seq, true)
        : await revise.run(id, row.seq, view.prompts.seq, event, true)
    if (result === null) return
    const dropped = result.dropped ?? []
    if (dropped.length === 0) {
      await write(row, event)
      return
    }
    setPreview({ row, event, dropped, names: await resolveRefNames(dropped) })
  }

  /** Makes the change, whether it was asked about or not. */
  const write = async (row: SettledRow, event: CharacterEvent | null) => {
    // expectedSeq is sent again, so a log that moved between the price and
    // this write is the existing sequence conflict rather than a silent
    // commit of a price that was quoted against a different log.
    const written =
      event === null
        ? await remove.run(id, row.seq, view.prompts.seq, false)
        : await revise.run(id, row.seq, view.prompts.seq, event, false)
    if (written === null) return
    // A removal is how a question that cannot be re-posed gets asked again, so
    // the question that comes back takes the answer's place in the list.
    if (event === null) reclaimPlace(order, settledKey(row.seq))
    done()
  }

  const commit = async () => {
    if (preview === null) return
    await write(preview.row, preview.event)
  }

  /**
   * Backing out of the question, and out of the block that asked it.
   *
   * A block whose question was being put again the long way round has nothing
   * to show once the drop is refused -- it was never re-posed -- so it closes
   * with the dialog. One that is being answered stays open, because the answer
   * being reconsidered is still on screen.
   */
  const cancelPreview = () => {
    if (preview?.event === null) setOpenKey(null)
    setPreview(null)
  }

  /**
   * Opening a block, which for a settled one is putting its question again.
   *
   * Every decided block opens onto the question that decided it, and there is
   * no second gesture anywhere on this screen. Where the question cannot be
   * re-posed -- a nested prompt, whose options came with a prompt the server
   * stopped emitting -- putting it again means dropping the entry, which is
   * the same thing reached from the other side: the question comes back
   * outstanding, in the block's own place. That costs nothing else in the
   * usual case, and where it does, `price` asks first.
   */
  const openBlock = (key: string | null) => {
    setOpenKey(key)
    const block = key === null ? null : blocks.find((each) => each.key === key)
    if (block?.kind !== 'settled') return
    const question = reask(block.row)
    if (question === null) {
      void price(block.row, null)
      return
    }
    // A rename starts from the name it is changing rather than from nothing.
    if (question.choice.kind === 'text') setNameDraft(block.row.value)
  }

  const submitEvent = (asked: Asking, event: CharacterEvent) => {
    if (asked.replaces === null) void append(event)
    else void price(asked.replaces, event)
  }

  if (build.loading) {
    return (
      <Group gap="xs">
        <Loader size="sm" />
        <Text size="sm" c="dimmed">
          Working out what is next...
        </Text>
      </Group>
    )
  }
  if (build.error !== null) {
    return (
      <Alert color="red" title="Could not load this character">
        <Stack gap="xs" align="flex-start">
          <Text size="sm">{build.error}</Text>
          <Button variant="light" onClick={build.reload}>
            Try again
          </Button>
        </Stack>
      </Alert>
    )
  }

  const nextStage = stageAfter(stage, open)
  const failure = create.error ?? answer.error ?? revise.error ?? remove.error
  const fields: readonly ApiFieldError[] =
    create.fields.length > 0
      ? create.fields
      : answer.fields.length > 0
        ? answer.fields
        : revise.fields

  return (
    <Stack gap="lg">
      <div>
        <Title order={2}>{isNew ? 'New character' : `Build · ${title(view)}`}</Title>
        <Text c="dimmed" size="sm">
          {isNew
            ? 'A name is all it takes to start. Everything else is a question, asked once there is somebody to ask it about.'
            : view.prompts.complete
              ? 'Everything required is answered. What is left is optional -- and a level.'
              : 'Answer what is open, in any order.'}
        </Text>
      </div>

      {failure !== null && (
        <Alert color="red" title="The server did not accept that">
          <Stack gap={4}>
            <Text size="sm">{failure}</Text>
            {fields.map((field) => (
              <Text key={field.field} size="xs" c="dimmed">
                {field.message ?? field.rule}
              </Text>
            ))}
          </Stack>
        </Alert>
      )}

      <TabRow
        tabs={STAGES.map((name) => ({ value: name, label: STAGE_LABELS[name] }))}
        value={stage}
        onChange={(next) => goToStage(next as Stage)}
        actions={
          !isNew && (
            <Button
              variant={view.prompts.complete ? 'filled' : 'light'}
              onClick={() => void navigate(`/characters/${id}`)}
            >
              Finish
            </Button>
          )
        }
      >
        <StagePanel
          blocks={blocks}
          openKey={openKey}
          onOpen={openBlock}
          asking={asking}
          names={view.names}

          onAnswerPicks={(asked, picks) => submitEvent(asked, eventFor(asked.prompt, picks))}
          onNameChange={(next) => {
            setNameDraft(next)
            setNameError(undefined)
          }}
          onAnswerName={(asked, next) => {
            if (isNew) void createCharacterFromDraft()
            else submitEvent(asked, initEventFor(next))
          }}
          onAnswerChanges={(asked, changes) =>
            submitEvent(asked, { type: asked.prompt.event.type, changes })
          }
          pending={create.pending || answer.pending || revise.pending || remove.pending}
          fields={fields}
          {...(nextStage === null ? {} : { onNext: () => goToStage(nextStage) })}
          {...(isNew || asking?.prompt.choice.kind === 'text' ? { name: nameDraft } : {})}
          {...maybeScores(asking?.replaces ?? null)}
        />
      </TabRow>

      {nameError !== undefined && (
        <Text size="sm" c="red">
          {nameError}
        </Text>
      )}

      {/*
        Only ever open because something would be lost. A change that costs
        nothing else is simply made -- see `price` -- so this dialog means one
        thing and never cries wolf.
      */}
      <ModalSheet
        opened={preview !== null}
        onClose={() => cancelPreview()}
        title={preview?.event === null ? 'Put that question again?' : 'Change this?'}
      >
        {preview !== null && (
          <Stack gap="md">
            <Text size="sm">
              {`${preview.dropped.length === 1 ? 'One other answer' : `${preview.dropped.length} other answers`} `}
              {preview.event === null
                ? 'depend on this one and cannot survive it being asked again.'
                : 'depend on this and cannot survive the change.'}
            </Text>

            {preview.dropped.length > 0 && (
              <Stack gap="xs">
                {preview.dropped.map((entry) => (
                  <div key={entry.seq}>
                    <Group gap={6}>
                      <Text size="sm" fw={500}>
                        {eventLabel(entry.type)}
                        {entry.ref !== undefined && `: ${preview.names.get(entry.ref) ?? entry.ref}`}
                      </Text>
                      <Badge size="xs" variant="light" color="gray">
                        {REASONS[entry.reason] ?? entry.reason}
                      </Badge>
                    </Group>
                    {(entry.lost ?? []).map((lost) => (
                      <Text key={lost.prompt} size="xs" c="dimmed">
                        {promptLabel(lost.prompt)}
                        {lost.picks !== undefined && `: ${lost.picks.map(pickLabel).join(', ')}`}
                      </Text>
                    ))}
                  </div>
                ))}
                <Text size="xs" c="dimmed">
                  Nothing is lost that cannot be answered again: each of these becomes an open
                  question, waiting under whichever tab it belongs to.
                </Text>
              </Stack>
            )}

            <Group>
              <Button
                color="red"
                loading={revise.pending || remove.pending}
                onClick={() => void commit()}
              >
                {preview.event === null ? 'Ask it again' : 'Change it'}
              </Button>
              <Button variant="subtle" onClick={() => cancelPreview()}>
                Cancel
              </Button>
            </Group>
          </Stack>
        )}
      </ModalSheet>
    </Stack>
  )
}

/** How a drop reason reads, without borrowing a category's word. */
const REASONS: Record<string, string> = {
  'not-offered': 'no longer offered',
  'answers-dropped': 'answers no longer legal',
  empty: 'nothing left in it',
}

/**
 * The prompt a character with no log at all is asked.
 *
 * The server emits the real one -- `character/init` -- as soon as a character
 * exists to have an empty log. Before that there is no character to ask about
 * and no request to make, so the identity tab poses the same question itself
 * and the answer is a creation rather than an append.
 */
const NEW_NAME_PROMPT: Prompt = {
  choice: { prompt: 'character/init', choose: 1, kind: 'text', from: { kind: 'explicit' } },
  group: 'identity',
  optional: false,
  advances: false,
  event: { type: 'init' },
  heldOnly: false,
}

const NEW_NAME_KEY = keyFor({ prompt: NEW_NAME_PROMPT, replaces: null })

/**
 * The question behind a settled block, where there is one to put again.
 *
 * Null is an answer rather than a failure: a nested prompt cannot be re-posed
 * from here, and the block says so and offers the drop instead.
 */
function askingFor(row: SettledRow): Asking | null {
  const prompt = reask(row)
  return prompt === null ? null : { prompt, replaces: row }
}

/** The header's subject: what the character is called, or that it is not. */
function title(view: BuildView): string {
  const name = view.sheet?.identity.name ?? ''
  return name === '' ? 'Unnamed' : name
}

/**
 * The first category with something required still open.
 *
 * Optional prompts do not count. A finished character always has one -- the
 * offer of another level -- so counting them would mean no character was ever
 * anywhere but wherever that offer lives.
 */
function firstUnfinished(prompts: readonly Prompt[]): Stage {
  const required = new Set(
    prompts.filter((p) => !p.optional).flatMap((p) => [stageOf(p.group)].filter(isStage)),
  )
  const any = new Set(prompts.flatMap((p) => [stageOf(p.group)].filter(isStage)))
  return STAGES.find((s) => required.has(s)) ?? STAGES.find((s) => any.has(s)) ?? 'identity'
}

/**
 * The next category with something still open, wrapping round.
 *
 * Required questions first, and anything at all only if none is left: a
 * finished character always has an optional prompt somewhere, and a Next that
 * walked to it would never let anybody stop.
 */
function stageAfter(stage: Stage, prompts: readonly Prompt[]): Stage | null {
  const groups = (only: (p: Prompt) => boolean) =>
    new Set(prompts.filter(only).flatMap((p) => [stageOf(p.group)].filter(isStage)))
  const from = STAGES.indexOf(stage)
  const order = [...STAGES.slice(from + 1), ...STAGES.slice(0, from)]
  const required = groups((p) => !p.optional)
  return order.find((s) => required.has(s)) ?? order.find((s) => groups(() => true).has(s)) ?? null
}

function isStage(stage: Stage | null): stage is Stage {
  return stage !== null
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
const INPUTS: readonly { prompt: string; path: string; kind: string; collection: string }[] = [
  {
    prompt: 'character/alignment',
    path: 'identity.alignment',
    kind: 'alignment',
    collection: 'alignment',
  },
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
      from: { kind: 'collection', collection: input.collection },
    },
    group: stage,
    optional: false,
    advances: false,
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
function reask(row: SettledRow): Prompt | null {
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
    return {
      choice: {
        prompt: `character/${kind}`,
        choose: 1,
        kind,
        from: { kind: 'collection', collection: kind },
      },
      group: row.stage,
      optional: false,
      advances: false,
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
      advances: false,
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
function maybeScores(row: SettledRow | null): { scores?: Scores; method?: string } {
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

/** A name, as the entry that carries one. */
function initEventFor(name: string): CharacterEvent {
  return {
    type: 'init',
    changes: [{ path: 'identity.name', op: 'set', value: { kind: 'string', string: name } }],
  }
}

/** The entry a prompt said its answer travels in, filled in with the answer. */
function eventFor(prompt: Prompt, picks: string[]): CharacterEvent {
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
  if (selectsTheEventItself(prompt)) {
    event.ref = `${refKindFor(prompt)}:${picks[0] ?? ''}`
  } else {
    event.choices = [{ prompt: prompt.choice.prompt, picks }]
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
