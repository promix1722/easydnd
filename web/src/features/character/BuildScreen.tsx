import { useState } from 'react'
import { useNavigate, useParams } from 'react-router'

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

import { resolveRefNames } from './refNames'
import { settledByStage } from './settled'
import type { SettledRow } from './settled'
import { StagePanel } from './StagePanel'
import type { Asking } from './StagePanel'
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
  const [asking, setAsking] = useState<Asking | null>(null)
  const [askedOn, setAskedOn] = useState<Stage | null>(null)
  const [nameDraft, setNameDraft] = useState('')
  const [nameError, setNameError] = useState<string | undefined>(undefined)
  const [preview, setPreview] = useState<Preview | null>(null)

  const view = build.data ?? EMPTY_VIEW
  // Advancement is dropped before anything looks at it, so no tab, no list
  // and no Next can reach a question this client cannot honestly answer.
  const open = view.prompts.prompts.filter((prompt) => answerable(prompt.group))
  // Until a tab is clicked the screen opens on the first thing left to do,
  // and moves on as things are answered. That is the loop, kept: a player who
  // never touches a tab is walked through the questions in order, and one who
  // does is pinned where they put themselves.
  const stage = chosenStage ?? firstUnfinished(open)

  // A different tab is a different question, so the one in hand is dropped.
  // During render rather than in an effect, so the new tab is never painted
  // once with the old tab's question under it.
  if (askedOn !== stage) {
    setAskedOn(stage)
    setAsking(null)
  }

  const openHere = open.filter((prompt) => stageOf(prompt.group) === stage)
  const settled = settledByStage(view)
  const inHand: Asking | null =
    asking ??
    (isNew
      ? { prompt: NEW_NAME_PROMPT, replaces: null }
      : openHere[0] !== undefined
        ? { prompt: openHere[0], replaces: null }
        : null)

  const done = () => {
    setAsking(null)
    setPreview(null)
    build.reload()
  }

  /** Creates the character the identity tab is describing, and moves on. */
  const createCharacterFromDraft = async () => {
    if (create.pending) return
    if (nameDraft.trim() === '') {
      setNameError('A character needs a name to be created under.')
      return
    }
    const created = await create.run({ name: nameDraft.trim() })
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
    const written = await answer.run(id, view.prompts.seq, [event])
    if (written) done()
  }

  /**
   * Prices a replacement, without making it.
   *
   * The same call the commit makes, with `dryRun` set: a preview produced by
   * a second code path is a preview that can disagree with what happens, and
   * a preview that disagrees is worse than none.
   */
  const startPreview = async (row: SettledRow, event: CharacterEvent | null) => {
    const result =
      event === null
        ? await remove.run(id, row.seq, view.prompts.seq, true)
        : await revise.run(id, row.seq, view.prompts.seq, event, true)
    if (result === null) return
    const dropped = result.dropped ?? []
    setPreview({ row, event, dropped, names: await resolveRefNames(dropped) })
  }

  const commit = async () => {
    if (preview === null) return
    const { row, event } = preview
    // expectedSeq is sent again, so a log that moved between the preview and
    // this click is the existing sequence conflict rather than a silent
    // commit of a price that was quoted against a different log.
    const written =
      event === null
        ? await remove.run(id, row.seq, view.prompts.seq, false)
        : await revise.run(id, row.seq, view.prompts.seq, event, false)
    if (written) done()
  }

  /** [Change]: re-ask the question behind an entry, where it can be re-asked. */
  const changeRow = (row: SettledRow) => {
    const prompt = reask(row)
    if (prompt === null) {
      // An answer to a nested prompt cannot be re-posed from the client -- the
      // options that made it up came with a prompt the server has stopped
      // emitting. Dropping the entry is the same outcome reached the other way
      // round: the question comes back outstanding, in its own category.
      void startPreview(row, null)
      return
    }
    if (prompt.choice.kind === 'text') setNameDraft(row.value)
    setChosenStage(row.stage)
    setAsking({ prompt, replaces: row })
  }

  const submitEvent = (asked: Asking, event: CharacterEvent) => {
    if (asked.replaces === null) void append(event)
    else void startPreview(asked.replaces, event)
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
          <Group gap="xs" wrap="nowrap">
            <Button
              variant="default"
              disabled={nextStage === null}
              onClick={() => {
                if (nextStage !== null) goToStage(nextStage)
              }}
            >
              Next
            </Button>
            {!isNew && (
              <Button
                variant={view.prompts.complete ? 'filled' : 'light'}
                onClick={() => void navigate(`/characters/${id}`)}
              >
                Finish
              </Button>
            )}
          </Group>
        }
      >
        <StagePanel
          settled={settled.get(stage) ?? []}
          prompts={openHere}
          names={view.names}
          asking={inHand}
          onAsk={setAsking}
          onChange={changeRow}
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
          {...(isNew || asking?.prompt.choice.kind === 'text' ? { name: nameDraft } : {})}
          {...maybeScores(asking?.replaces ?? null)}
        />
      </TabRow>

      {nameError !== undefined && (
        <Text size="sm" c="red">
          {nameError}
        </Text>
      )}

      <ModalSheet
        opened={preview !== null}
        onClose={() => setPreview(null)}
        title={preview?.event === null ? 'Remove this?' : 'Change this?'}
      >
        {preview !== null && (
          <Stack gap="md">
            <Text size="sm">
              {preview.dropped.length === 0
                ? 'Nothing else in the log depends on this. Everything after it survives untouched.'
                : `${preview.dropped.length === 1 ? 'One entry' : `${preview.dropped.length} entries`} depend on this and cannot survive the change.`}
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
                color={preview.dropped.length === 0 ? 'green' : 'red'}
                loading={revise.pending || remove.pending}
                onClick={() => void commit()}
              >
                {preview.event === null ? 'Remove it' : 'Change it'}
              </Button>
              <Button variant="subtle" onClick={() => setPreview(null)}>
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

/** The next category, after this one, with something required still open. */
function stageAfter(stage: Stage, prompts: readonly Prompt[]): Stage | null {
  const required = new Set(
    prompts.filter((p) => !p.optional).flatMap((p) => [stageOf(p.group)].filter(isStage)),
  )
  const from = STAGES.indexOf(stage)
  const order = [...STAGES.slice(from + 1), ...STAGES.slice(0, from)]
  return order.find((s) => required.has(s)) ?? null
}

function isStage(stage: Stage | null): stage is Stage {
  return stage !== null
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
