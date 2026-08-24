import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'

import {
  appendEvents,
  bySlug,
  getCollection,
  getEntries,
  getPrompts,
  truncateEvents,
} from '@/lib/api'
import type { CharacterEvent, Entry, Prompt, PromptsResponse } from '@/lib/api'
import { useAction } from '@/lib/useAction'
import { useResource } from '@/lib/useResource'
import { Alert, Anchor, Badge, Button, Group, Loader, Stack, Text, Title } from '@/ui'

import { PromptCard } from './PromptCard'

/**
 * Which catalogue collection a prompt's options are drawn from.
 *
 * The server says this for the sets it draws from a collection, and for
 * explicit sets the options carry typed refs. This maps a reference kind to
 * the collection that holds it, which is the one piece of the vocabulary the
 * client has to know.
 */
const COLLECTION_OF: Record<string, string> = {
  race: 'races',
  subrace: 'subraces',
  trait: 'traits',
  class: 'classes',
  subclass: 'subclasses',
  feature: 'features',
  background: 'backgrounds',
  feat: 'feats',
  item: 'equipment',
  'magic-item': 'magic-items',
  spell: 'spells',
  language: 'languages',
  proficiency: 'proficiencies',
  alignment: 'alignments',
  skill: 'skills',
  condition: 'conditions',
  'damage-type': 'damage-types',
}

/**
 * The build loop: creation and level-up, which are the same thing.
 *
 * It is a loop rather than an N-step wizard, and it has to be. Prompts nest --
 * choosing the "two skills" branch of a rogue's Expertise is what brings the
 * two-skill prompt into existence -- so the total number of steps is not
 * knowable until the last one is answered. Progress is shown by group instead.
 *
 * Nothing here decides what an answer means. The server says which event
 * carries it, and this copies that verbatim; a client that decided for itself
 * that a first level is a class event and a fourth is a level event would be
 * reimplementing the rules in the browser.
 */
export function BuildScreen() {
  const { id = '' } = useParams()
  const navigate = useNavigate()

  const prompts = useResource<PromptsResponse>(`prompts:${id}`, (signal) => getPrompts(id, signal))
  const answer = useAction(appendEvents)
  const undo = useAction(truncateEvents)

  const current = firstOpen(prompts.data)

  const submit = useCallback(
    async (picks: string[]) => {
      if (!prompts.data || !current) return
      const event: CharacterEvent = {
        type: current.event.type,
        ...(current.event.ref !== undefined ? { ref: current.event.ref } : {}),
        ...(current.event.level !== undefined ? { level: current.event.level } : {}),
      }
      // A prompt that selects a catalogue entry carries its answer in the
      // event's ref; every other prompt carries it in the choices.
      if (selectsTheEventItself(current)) {
        const slug = picks[0] ?? ''
        event.ref = `${refKindFor(current)}:${slug}`
      } else {
        event.choices = [{ prompt: current.choice.prompt, picks }]
      }
      const written = await answer.run(id, prompts.data.seq, [event])
      if (written) prompts.reload()
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [id, current, prompts.data?.seq],
  )

  const goBack = async () => {
    if (!prompts.data || prompts.data.seq <= 1) return
    const written = await undo.run(id, prompts.data.seq, prompts.data.seq - 1)
    if (written) prompts.reload()
  }

  if (prompts.loading) {
    return (
      <Group gap="xs">
        <Loader size="sm" />
        <Text size="sm" c="dimmed">
          Working out what is next...
        </Text>
      </Group>
    )
  }
  if (prompts.error !== null) {
    return (
      <Alert color="red" title="Could not load this character">
        <Stack gap="xs" align="flex-start">
          <Text size="sm">{prompts.error}</Text>
          <Button variant="light" onClick={prompts.reload}>
            Try again
          </Button>
        </Stack>
      </Alert>
    )
  }

  const data = prompts.data
  if (!data) return null

  return (
    <Stack gap="lg" maw={720}>
      <Group justify="space-between" align="flex-start">
        <div>
          <Title order={2}>Build</Title>
          <Text c="dimmed" size="sm">
            {data.complete
              ? 'Everything required is answered. What is left is optional -- and a level.'
              : 'One question at a time.'}
          </Text>
        </div>
        <Group gap="xs">
          <Button variant="subtle" onClick={() => void goBack()} disabled={data.seq <= 1} loading={undo.pending}>
            Back
          </Button>
          <Anchor component={Link} to={`/characters/${id}`}>
            <Button variant="light">Sheet</Button>
          </Anchor>
        </Group>
      </Group>

      <Group gap="xs">
        {groupProgress(data.prompts).map((stage) => (
          <Badge key={stage.name} variant={stage.done ? 'filled' : 'light'} color={stage.done ? 'green' : 'gray'}>
            {stage.name}
          </Badge>
        ))}
      </Group>

      {answer.error !== null && (
        <Alert color="red" title="The server did not accept that">
          <Stack gap={4}>
            <Text size="sm">{answer.error}</Text>
            {answer.fields.map((field) => (
              <Text key={field.field} size="xs" c="dimmed">
                {field.message ?? field.rule}
              </Text>
            ))}
          </Stack>
        </Alert>
      )}

      {current ? (
        <PromptWithOptions
          prompt={current}
          pending={answer.pending}
          onAnswer={(picks) => void submit(picks)}
        />
      ) : (
        <Alert color="green" title="Nothing left to decide">
          <Stack gap="xs" align="flex-start">
            <Text size="sm">This character is finished, and cannot advance further.</Text>
            <Button onClick={() => void navigate(`/characters/${id}`)}>
              Open the sheet
            </Button>
          </Stack>
        </Alert>
      )}

      {data.complete && current !== null && (
        <Text size="xs" c="dimmed">
          Skipping an optional question is fine -- it stays open, and you can come back.
        </Text>
      )}
    </Stack>
  )
}

/**
 * Loads the entries a prompt's options refer to, then renders it.
 *
 * Split out so that the fetch is keyed by the prompt: React remounts it when
 * the prompt changes, which is exactly the lifecycle the options have.
 */
function PromptWithOptions({
  prompt,
  pending,
  onAnswer,
}: {
  prompt: Prompt
  pending: boolean
  onAnswer: (picks: string[]) => void
}) {
  const [entries, setEntries] = useState<Map<string, Entry>>(new Map())

  useEffect(() => {
    let live = true
    void loadEntries(prompt).then((loaded) => {
      if (live) setEntries(loaded)
    })
    return () => {
      live = false
    }
  }, [prompt])

  return <PromptCard prompt={prompt} entries={entries} pending={pending} onAnswer={onAnswer} />
}

/** Fetches the catalogue entries a prompt's options name. */
async function loadEntries(prompt: Prompt): Promise<Map<string, Entry>> {
  const set = prompt.choice.from
  if (set.kind === 'collection' && set.collection !== undefined) {
    const collection = COLLECTION_OF[set.collection]
    if (collection === undefined) return new Map()
    return bySlug(await getCollection<Entry>(collection))
  }

  // Explicit options: gather the refs they name, grouped by collection.
  const wanted = new Map<string, Set<string>>()
  const visit = (options: readonly { kind: string; ref?: string; items?: unknown[] }[]) => {
    for (const option of options) {
      if (option.kind === 'ref' && option.ref !== undefined) {
        const [kind, slug] = option.ref.split(':')
        const collection = COLLECTION_OF[kind ?? '']
        if (collection === undefined || slug === undefined) continue
        const bucket = wanted.get(collection) ?? new Set<string>()
        bucket.add(slug)
        wanted.set(collection, bucket)
      }
      if (option.items) visit(option.items as { kind: string; ref?: string }[])
    }
  }
  visit(set.options ?? [])

  const loaded = await Promise.all(
    [...wanted].map(([collection, slugs]) => getEntries<Entry>(collection, [...slugs])),
  )
  return bySlug(loaded.flat())
}

/** The first prompt still worth asking: required ones first, then optional. */
function firstOpen(data: PromptsResponse | null): Prompt | null {
  if (!data) return null
  return data.prompts.find((p) => !p.optional) ?? data.prompts[0] ?? null
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

/** Progress by stage, since the number of steps is not knowable. */
function groupProgress(prompts: readonly Prompt[]): { name: string; done: boolean }[] {
  const stages = ['identity', 'abilities', 'race', 'background', 'class']
  const open = new Set(prompts.filter((p) => !p.optional).map((p) => p.group))
  return stages.map((name) => ({ name, done: !open.has(name) }))
}
