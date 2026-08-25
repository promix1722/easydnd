import { useEffect, useState } from 'react'

import { bySlug, getCollection, getEntries } from '@/lib/api'
import type { ApiFieldError, Change, Entry, Prompt } from '@/lib/api'
import { Badge, Button, Group, Paper, Stack, Text } from '@/ui'

import { AbilityScoresForm } from './AbilityScoresForm'
import type { Scores } from './AbilityScoresForm'
import { NameForm } from './NameForm'
import { offersOptions } from './options'
import { OutstandingChoices } from './OutstandingChoices'
import { PromptCard } from './PromptCard'
import type { SettledRow } from './settled'

import { collectionOfKind, kindOf, slugOf } from '@/domain'

/** The question on screen, and the entry answering it would replace. */
export interface Asking {
  prompt: Prompt
  /** Null when the answer is an append: a question being asked for the first time. */
  replaces: SettledRow | null
}

export interface StagePanelProps {
  /** Entries already made on this tab, one per selection. */
  settled: readonly SettledRow[]
  /** Questions still open on this tab. */
  prompts: readonly Prompt[]
  names: ReadonlyMap<string, string>
  asking: Asking | null
  onAsk: (asking: Asking | null) => void
  /** Re-ask the question behind a settled entry. */
  onChange: (row: SettledRow) => void
  onAnswerPicks: (asking: Asking, picks: string[]) => void
  /** The name draft, which lives above this panel: see NameForm. */
  onNameChange: (name: string) => void
  onAnswerName: (asking: Asking, name: string) => void
  onAnswerChanges: (asking: Asking, changes: Change[]) => void
  pending: boolean
  fields: readonly ApiFieldError[]
  /** What the character is called, so renaming starts from it. */
  name?: string
  /** The scores as the log stored them -- not as the sheet projects them. */
  scores?: Scores
  method?: string
}

/**
 * One tab: what has been decided, what is still open, and the surface for
 * whichever question is in hand.
 *
 * The two sections are titled "already chosen" and "still to choose" on every
 * tab, and neither says which tab it is on. The category's word appears
 * exactly once in the document -- in the tab itself -- so that looking for
 * "race" on this page finds the tab and nothing else.
 *
 * A tab with nothing open shows only its settled rows. That is not the screen
 * hiding a step: it is the whole of the model, which is that nothing can be
 * answered before it is asked. The way to make a question appear is to change
 * the entry that would open it.
 */
export function StagePanel({
  settled,
  prompts,
  names,
  asking,
  onAsk,
  onChange,
  onAnswerPicks,
  onNameChange,
  onAnswerName,
  onAnswerChanges,
  pending,
  fields,
  name,
  scores,
  method,
}: StagePanelProps) {
  return (
    <Stack gap="md">
      {settled.length > 0 && (
        <Paper withBorder p="md" radius="md">
          <Stack gap="xs">
            <Text size="xs" c="dimmed" tt="uppercase">
              already chosen
            </Text>
            {settled.map((row) => (
              <Group key={row.seq} justify="space-between" gap="xs" wrap="nowrap" align="flex-start">
                <div>
                  <Group gap={6}>
                    <Text size="xs" c="dimmed" tt="uppercase">
                      {row.label}
                    </Text>
                    {row.level !== undefined && (
                      <Badge size="xs" variant="light">
                        Level {row.level}
                      </Badge>
                    )}
                  </Group>
                  <Text size="sm">{row.value}</Text>
                </div>
                {/*
                  A level already taken is shown and not touched. Replacing or
                  removing one drives the same machinery that cannot take one
                  in the first place, so the row is a fact about the character
                  rather than a control -- which is also what an imported
                  character's levels are. Everything else keeps its Change.
                */}
                {row.event.type !== 'level' && (
                  <Button size="compact-xs" variant="subtle" onClick={() => onChange(row)}>
                    Change
                  </Button>
                )}
              </Group>
            ))}
          </Stack>
        </Paper>
      )}

      <Paper withBorder p="md" radius="md">
        <Stack gap="xs">
          <Text size="xs" c="dimmed" tt="uppercase">
            still to choose
          </Text>
          <OutstandingChoices
            prompts={prompts}
            names={names}
            {...(asking?.replaces === null ? { selected: asking.prompt.choice.prompt } : {})}
            onOpen={(prompt) => onAsk({ prompt, replaces: null })}
            empty={settled.length > 0 ? 'Nothing left here.' : 'Nothing to answer yet.'}
          />
        </Stack>
      </Paper>

      {asking !== null && (
        <AnswerSurface
          asking={asking}
          pending={pending}
          fields={fields}
          {...(name !== undefined ? { name } : {})}
          {...(scores !== undefined ? { scores } : {})}
          {...(method !== undefined ? { method } : {})}
          onPicks={(picks) => onAnswerPicks(asking, picks)}
          onNameChange={onNameChange}
          onName={(next) => onAnswerName(asking, next)}
          onChanges={(changes) => onAnswerChanges(asking, changes)}
        />
      )}
    </Stack>
  )
}

/**
 * Picks the surface a question is answered on, by the kind of question it is.
 *
 * This is the only place in the client that knows a prompt kind implies a
 * control, and it exists so that `PromptCard` does not have to. That card
 * renders "choose N of these", which is every prompt the compendium poses,
 * because the server synthesises "which race?" into the compendium's own
 * grammar. The two questions that are genuinely not that shape -- a name, and
 * six numbers -- get a form each, and the card is left alone.
 *
 * `ability-scores` covers two questions. The improvement a level grants offers
 * options (raise two scores, or take a feat) and is a choice like any other;
 * the six a character starts with offer none, and are a form. The option set
 * is what tells them apart, and it is the server's own answer to "what may be
 * picked here" rather than a slug this client has memorised.
 */
function AnswerSurface({
  asking,
  pending,
  fields,
  name,
  scores,
  method,
  onPicks,
  onNameChange,
  onName,
  onChanges,
}: {
  asking: Asking
  pending: boolean
  fields: readonly ApiFieldError[]
  name?: string
  scores?: Scores
  method?: string
  onPicks: (picks: string[]) => void
  onNameChange: (name: string) => void
  onName: (name: string) => void
  onChanges: (changes: Change[]) => void
}) {
  const { prompt, replaces } = asking
  const submitLabel = replaces === null ? 'Confirm' : 'Change it'
  const { kind } = prompt.choice

  if (kind === 'text') {
    return (
      <NameForm
        value={name ?? ''}
        onValueChange={onNameChange}
        pending={pending}
        {...maybeError(fields, 'name')}
        submitLabel={submitLabel}
        onSubmit={onName}
      />
    )
  }

  if (kind === 'ability-scores' && !offersOptions(prompt)) {
    return (
      <AbilityScoresForm
        {...(scores !== undefined ? { scores } : {})}
        {...(method !== undefined ? { method } : {})}
        pending={pending}
        fields={fields}
        submitLabel={submitLabel}
        onSubmit={onChanges}
      />
    )
  }

  return <PromptWithOptions prompt={prompt} pending={pending} onAnswer={onPicks} />
}

function maybeError(fields: readonly ApiFieldError[], suffix: string): { error?: string } {
  const found = fields.find((field) => field.field.endsWith(suffix))
  if (found === undefined) return {}
  const message = found.message ?? found.rule
  return message === undefined ? {} : { error: message }
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
    const collection = collectionOfKind(set.collection)
    if (collection === null) return new Map()
    return bySlug(await getCollection<Entry>(collection))
  }

  // Explicit options: gather the refs they name, grouped by collection.
  const wanted = new Map<string, Set<string>>()
  const visit = (options: readonly { kind: string; ref?: string; items?: unknown[] }[]) => {
    for (const option of options) {
      if (option.kind === 'ref' && option.ref !== undefined) {
        const collection = collectionOfKind(kindOf(option.ref))
        if (collection === null) continue
        const bucket = wanted.get(collection) ?? new Set<string>()
        bucket.add(slugOf(option.ref))
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
