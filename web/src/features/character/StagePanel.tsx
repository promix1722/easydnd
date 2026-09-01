import { useEffect, useState } from 'react'

import { bySlug, getCollection, getEntries , describeField } from '@/lib/api'
import type { Answer, ApiFieldError, Change, Entry, OptionSet, Prompt } from '@/lib/api'
import { useT } from '@/lib/i18n'
import type { Translate } from '@/lib/i18n'
import { Badge, BlockList, Button, Group, Loader, Stack, Text } from '@/ui'
import type { BlockListItem } from '@/ui'

import { AbilityScoresForm } from './AbilityScoresForm'
import type { Scores } from './AbilityScoresForm'
import { groupByLevel } from './blocks'
import type { Asking, Block } from './blocks'
import { DesiredLevelForm } from './DesiredLevelForm'
import { NameForm } from './NameForm'
import { RulesetForm } from './RulesetForm'
import { offersOptions } from './options'
import { PromptCard } from './PromptCard'
import { choiceName, writtenAs } from './promptNames'
import { refName } from './refNames'
import type { SettledRow } from './settled'
import { WrittenForm } from './WrittenForm'

import { collectionOfKind, kindOf, slugOf } from '@/domain'

export interface StagePanelProps {
  /** Everything on this tab: what was decided, and what is still asked. */
  blocks: readonly Block[]
  openKey: string | null
  onOpen: (key: string | null) => void
  /** The question the open block is asking, where it has one. */
  asking: Asking | null
  names: ReadonlyMap<string, string>
  onAnswerPicks: (asking: Asking, answers: Answer[]) => void
  /** The name draft, which lives above this panel: see NameForm. */
  onNameChange: (name: string) => void
  onAnswerName: (asking: Asking, name: string) => void
  onAnswerChanges: (asking: Asking, changes: Change[]) => void
  pending: boolean
  fields: readonly ApiFieldError[]
  /**
   * Where to go when there is nothing left here.
   *
   * Absent when there is nowhere to go, which is what makes the button the
   * end of the list rather than a fixture of it.
   */
  onNext?: () => void
  /** What the character is called, so renaming starts from it. */
  name?: string
  /** The scores as the log stored them -- not as the sheet projects them. */
  scores?: Scores
  method?: string
  /** What is already written, where the open question is one that is written. */
  lines?: readonly string[]
  /** The character's current level, for the desired-level form to start from. */
  level?: number
  /**
   * There is no character yet, so the only question that can be answered is
   * the one that creates it. The rest of the identity tab is drawn, so the
   * page says up front what it will ask, and does not open.
   */
  posing?: boolean
}

/**
 * One tab: every choice on it as a block, and the one that is open.
 *
 * There used to be two sections here -- what was decided above, what is left
 * below -- and, detached at the bottom, whichever question was in hand. Three
 * places for one thing. A choice is now a block that opens onto its own
 * answering surface, and a decided choice is the same block with an answer in
 * it, which is what it always was.
 *
 * Nothing is open until it is pressed. The screen no longer picks a question
 * for the player, because it has no way of knowing which of five open choices
 * they came here to make -- and a surface that opens itself is one they have
 * to close.
 *
 * No block names the tab it is on. The category's word appears exactly once in
 * the document -- in the tab itself -- so that looking for "race" on this page
 * finds the tab and nothing else. That is why the empty line is "Nothing left
 * here" rather than "nothing left in race".
 *
 * A tab with nothing open shows only its decided blocks. That is not the
 * screen hiding a step: it is the whole of the model, which is that nothing
 * can be answered before it is asked. The way to make a question appear is to
 * change the entry that would open it.
 */
export function StagePanel({
  blocks,
  openKey,
  onOpen,
  asking,
  names,
  onAnswerPicks,
  onNameChange,
  onAnswerName,
  onAnswerChanges,
  pending,
  fields,
  onNext,
  name,
  scores,
  method,
  lines,
  level,
  posing = false,
}: StagePanelProps) {
  const t = useT()
  const surface = (asked: Asking) => (
    <AnswerSurface
      asking={asked}
      pending={pending}
      fields={fields}
      {...(name !== undefined ? { name } : {})}
      {...(scores !== undefined ? { scores } : {})}
      {...(method !== undefined ? { method } : {})}
      {...(lines !== undefined ? { lines } : {})}
      {...(level !== undefined ? { level } : {})}
      onPicks={(answers) => onAnswerPicks(asked, answers)}
      onNameChange={onNameChange}
      onName={(next) => onAnswerName(asked, next)}
      onChanges={(changes) => onAnswerChanges(asked, changes)}
    />
  )

  const itemFor = (block: Block): BlockListItem => {
    const open = block.key === openKey
    if (block.kind === 'settled') {
      const header = <SettledHeader row={block.row} />
      if (!block.changeable) return { key: block.key, header }
      return {
        key: block.key,
        header,
        // Only the open block's body is ever drawn, so a closed one is a
        // placeholder that says nothing except that this block opens.
        body: open ? (asking === null ? <Reasking /> : surface(asking)) : null,
      }
    }
    // Before the character exists only the question that creates it can be
    // answered, so the other two are drawn as what they are: questions coming,
    // with nothing to open. A block with no body is a statement -- the same
    // rendering a level already taken gets.
    const waiting = posing && block.prompt.choice.prompt !== 'character/init'
    return {
      key: block.key,
      header: <OpenHeader prompt={block.prompt} names={names} />,
      highlighted: !waiting,
      ...(waiting ? {} : { body: open && asking !== null ? surface(asking) : null }),
    }
  }

  const nothingOpen = blocks.every((block) => block.kind === 'settled')

  return (
    <Stack gap="sm">
      {/*
        One list per level rather than a tag on every card: the class story is
        read level by level, and the heading says once what each card used to
        repeat. Blocks that belong to no level -- everything outside the class
        story -- come first, with no heading at all. Every list shares the one
        open key, so one block is open across the whole tab, exactly as before.
      */}
      {groupByLevel(blocks).map((group) => (
        <Stack key={group.level ?? 'unlevelled'} gap={6}>
          {group.level !== undefined && (
            <Text size="xs" c="dimmed" tt="uppercase" fw={600}>
              {t('block.level', { level: group.level })}
            </Text>
          )}
          <BlockList items={group.blocks.map(itemFor)} open={openKey} onOpen={onOpen} />
        </Stack>
      ))}
      {blocks.length === 0 ? (
        <Text size="sm" c="dimmed">
          {t('stagePanel.nothingYet')}
        </Text>
      ) : null}
      {/*
        Under the list, and only once the list has nothing left to answer. It
        is not navigation -- the tabs are, and they are always there -- it is
        the end of a piece of work saying where the next piece is, at the
        moment that is the only thing left to say. It names no category,
        because the category's word belongs to its tab.
      */}
      {nothingOpen && onNext !== undefined && (
        // In a Group rather than aligned by the Stack: aligning the stack to
        // its start shrink-wraps every child, and the list is one of them --
        // so the whole panel would take its width from whichever block
        // happens to be open, and change width as blocks are opened and shut.
        <Group>
          <Button variant="light" onClick={onNext}>
            {t('stagePanel.next')}
          </Button>
        </Group>
      )}
    </Stack>
  )
}

/** What was decided, and what it was decided to be. The level it belongs to
 * is said once by the heading over its group, not repeated per card. */
function SettledHeader({ row }: { row: SettledRow }) {
  return (
    <div>
      <Text size="xs" c="dimmed" tt="uppercase">
        {row.label}
      </Text>
      <Text size="sm">{row.value}</Text>
    </div>
  )
}

/**
 * A choice still to make, named rather than asked.
 *
 * The name and what posed it are the whole of the header, and they are also
 * the whole of the question -- which is why the surfaces underneath no longer
 * print a heading of their own. "from Half-Elf" is what makes a list of
 * questions legible: two skills from a race and two from a class are different
 * questions.
 */
function OpenHeader({ prompt, names }: { prompt: Prompt; names: ReadonlyMap<string, string> }) {
  const t = useT()

  return (
    <Group gap={8} wrap="nowrap" justify="space-between" w="100%">
      <Text size="sm" fw={600} style={{ whiteSpace: 'normal', textAlign: 'left' }}>
        {choiceName(t, prompt)}
        {prompt.source !== undefined && (
          <Text span size="xs" c="dimmed" fw={400}>
            {' '}
            · from {refName(prompt.source, names)}
          </Text>
        )}
      </Text>
      {prompt.optional && (
        <Badge size="xs" variant="light" color="gray">
          {t('block.optional')}
        </Badge>
      )}
    </Group>
  )
}

/**
 * A decided choice whose question is being put again the long way round.
 *
 * An answer to a nested prompt -- a rogue's Expertise, a half-elf's ability
 * bonuses -- came with a prompt the server stopped emitting the moment it was
 * answered, so there is nothing here to re-pose directly. The screen drops the
 * entry instead, which brings the question back outstanding in this block's
 * own place: the same thing every other decided block does when it is opened,
 * and the reason none of them needs a button to do it.
 */
function Reasking() {
  const t = useT()
  return (
    <Group gap="xs">
      <Loader size="sm" />
      <Text size="sm" c="dimmed">
        {t('stagePanel.reasking')}
      </Text>
    </Group>
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
 *
 * The four roleplaying questions are told apart the same way, and for the same
 * reason: a trait is written rather than picked, so its prompt arrives with
 * nothing to pick between. `writtenAs` says which path one settles.
 */
function AnswerSurface({
  asking,
  pending,
  fields,
  name,
  scores,
  method,
  lines,
  level,
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
  lines?: readonly string[]
  level?: number
  onPicks: (answers: Answer[]) => void
  onNameChange: (name: string) => void
  onName: (name: string) => void
  onChanges: (changes: Change[]) => void
}) {
  const t = useT()
  const { prompt, replaces } = asking
  const submitLabel = replaces === null ? t('answer.confirm') : t('answer.changeIt')
  const { kind } = prompt.choice

  // The two questions the character poses about itself rather than out of the
  // compendium, told apart by their own slugs: a desired level is a number,
  // and a ruleset is a recorded, final fact. Before the kind map, because the
  // ruleset arrives as `text` and would otherwise be a name.
  if (prompt.choice.prompt === 'character/desired-level') {
    return (
      <DesiredLevelForm
        initial={declaredLevel(replaces) ?? level ?? 1}
        pending={pending}
        submitLabel={submitLabel}
        onSubmit={onChanges}
      />
    )
  }
  if (prompt.choice.prompt === 'character/ruleset') {
    return <RulesetForm pending={pending} submitLabel={submitLabel} onSubmit={onChanges} />
  }

  if (kind === 'text') {
    return (
      <NameForm
        value={name ?? ''}
        onValueChange={onNameChange}
        pending={pending}
        {...maybeError(t, fields, 'name')}
        submitLabel={submitLabel}
        onSubmit={onName}
      />
    )
  }

  const written = writtenAs(prompt)
  if (written !== undefined && !offersOptions(prompt)) {
    return (
      <WrittenForm
        {...(lines !== undefined ? { lines } : {})}
        path={written.path}
        noun={t(written.noun)}
        pending={pending}
        submitLabel={submitLabel}
        onSubmit={onChanges}
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

/** The level a settled declaration stated, read back for the form changing it. */
function declaredLevel(replaces: SettledRow | null): number | undefined {
  const change = (replaces?.event.changes ?? []).find(
    (each) => each.path === 'identity.desiredLevel',
  )
  return change?.value.int
}

function maybeError(
  t: Translate,
  fields: readonly ApiFieldError[],
  suffix: string,
): { error?: string } {
  const found = fields.find((field) => field.field.endsWith(suffix))
  return found === undefined ? {} : { error: describeField(t, found) }
}

/**
 * Loads the entries a prompt's options refer to, then renders it.
 *
 * Split out so that the fetch is keyed by the prompt: React remounts it when
 * the prompt changes, which is exactly the lifecycle the options have. It is
 * also why `BlockList` mounts a body only while its block is open -- a tab of
 * collapsed blocks would otherwise fetch a collection apiece on every paint.
 */
function PromptWithOptions({
  prompt,
  pending,
  onAnswer,
}: {
  prompt: Prompt
  pending: boolean
  onAnswer: (answers: Answer[]) => void
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

/**
 * Fetches the catalogue entries a prompt's options name.
 *
 * Branches included, because a branch is drawn in the same card as the
 * question that offered it -- so its options need names before anything is
 * posted, not after the server poses it as a prompt of its own. A branch
 * drawing on a whole collection, like the improvement's "or a feat", pulls
 * that collection in the same pass.
 */
async function loadEntries(prompt: Prompt): Promise<Map<string, Entry>> {
  const whole = new Set<string>()
  const wanted = new Map<string, Set<string>>()

  const visitSet = (set: OptionSet) => {
    if (set.kind === 'collection' && set.collection !== undefined) {
      const collection = collectionOfKind(set.collection)
      if (collection !== null) whole.add(collection)
      return
    }
    for (const option of set.options ?? []) {
      if (option.kind === 'ref' && option.ref !== undefined) {
        const collection = collectionOfKind(kindOf(option.ref))
        if (collection === null) continue
        const bucket = wanted.get(collection) ?? new Set<string>()
        bucket.add(slugOf(option.ref))
        wanted.set(collection, bucket)
      }
      if (option.items !== undefined) visitSet({ kind: 'explicit', options: option.items })
      if (option.choice !== undefined) visitSet(option.choice.from)
    }
  }
  visitSet(prompt.choice.from)

  const loaded = await Promise.all([
    ...[...whole].map((collection) => getCollection<Entry>(collection)),
    ...[...wanted].map(([collection, slugs]) => getEntries<Entry>(collection, [...slugs])),
  ])
  return bySlug(loaded.flat())
}
