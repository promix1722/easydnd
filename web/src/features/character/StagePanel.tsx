import { useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'

import { describeField, describeError } from '@/lib/api'
import type { Answer, ApiFieldError, Change, Entry, Equipment, Prompt } from '@/lib/api'
import { useT, useLocale } from '@/lib/i18n'
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
import { EquipmentSummary } from './EquipmentSummary'
import { WrittenForm } from './WrittenForm'

import { loadEntries } from './choiceEntries'

export interface StagePanelProps {
  /** Everything on this tab: what was decided, and what is still asked. */
  blocks: readonly Block[]
  equipment?: Equipment
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
   * Where to go when required choices here are complete.
   *
   * Absent when there is nowhere to go, which is what makes the button the
   * end of the list rather than a fixture of it.
   */
  onNext?: () => void
  /** Focus Next after the last answer on this tab has been saved. */
  focusNext?: boolean
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
   * There is no character yet. Rules can be chosen as a draft and the name
   * creates the character; level waits until it exists.
   */
  posing?: boolean
  rulesSelected?: boolean
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
 * A block opens when pressed, or when the build screen advances to it after
 * an answer. Its answering surface receives focus so keyboard users can
 * continue without finding the next block themselves.
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
  equipment,
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
  focusNext = false,
  name,
  scores,
  method,
  lines,
  level,
  posing = false,
  rulesSelected = false,
}: StagePanelProps) {
  const t = useT()
  const nextRef = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    if (!focusNext) return
    const next = nextRef.current
    next?.focus({ preventScroll: true })
    next?.scrollIntoView?.({ block: 'nearest' })
  }, [focusNext])
  const surface = (asked: Asking) => (
    <FocusedAnswer label={choiceName(t, asked.prompt)}>
      <AnswerSurface
        asking={asked}
        pending={pending}
        fields={fields}
        {...(name !== undefined ? { name } : {})}
        {...(scores !== undefined ? { scores } : {})}
        {...(method !== undefined ? { method } : {})}
        {...(lines !== undefined ? { lines } : {})}
        {...(level !== undefined ? { level } : {})}
        rulesSelected={rulesSelected}
        onPicks={(answers) => onAnswerPicks(asked, answers)}
        onNameChange={onNameChange}
        onName={(next) => onAnswerName(asked, next)}
        onChanges={(changes) => onAnswerChanges(asked, changes)}
      />
    </FocusedAnswer>
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
    // Level needs a character, while rules and name can be chosen beforehand.
    const waiting = posing && block.prompt.choice.prompt === 'character/desired-level'
    return {
      key: block.key,
      header: <OpenHeader prompt={block.prompt} names={names} />,
      highlighted: !waiting,
      ...(waiting ? {} : { body: open && asking !== null ? surface(asking) : null }),
    }
  }

  const nothingRequired = blocks.every((block) => block.kind === 'settled' || block.prompt.optional ||
    (rulesSelected && block.prompt.choice.prompt === 'character/ruleset'))

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
      {equipment !== undefined && <EquipmentSummary equipment={equipment} names={names} />}
      {blocks.length === 0 && equipment === undefined ? (
        <Text size="sm" c="dimmed">
          {t('stagePanel.nothingYet')}
        </Text>
      ) : null}
      {/* Optional questions may remain open; Next still lets the player visit
          the next tab without making those answers required. */}
      {nothingRequired && onNext !== undefined && (
        // In a Group rather than aligned by the Stack: aligning the stack to
        // its start shrink-wraps every child, and the list is one of them --
        // so the whole panel would take its width from whichever block
        // happens to be open, and change width as blocks are opened and shut.
        <Group>
          <Button ref={nextRef} variant="light" onClick={onNext}>
            {t('stagePanel.next')}
          </Button>
        </Group>
      )}
    </Stack>
  )
}

/** Focus when Mantine's expanding panel can actually receive keyboard input. */
function FocusedAnswer({ label, children }: { label: string; children: ReactNode }) {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const surface = ref.current
    if (surface === null) return
    const panel = surface.closest('[role="region"]')
    let frame = 0
    let focused = false
    let waitingOnOptions = false
    const observer = new MutationObserver(() => {
      if (!focused) {
        cancelAnimationFrame(frame)
        frame = requestAnimationFrame(focus)
      }
    })
    const focus = () => {
      if (focused || !surface.isConnected || surface.closest('[inert], [aria-hidden="true"]')) return
      const field = surface.querySelector<HTMLElement>('input:not(:disabled), textarea:not(:disabled), select:not(:disabled), button:not(:disabled)')
      if (waitingOnOptions && document.activeElement !== surface) {
        // The player moved away while the choices loaded; leave their focus.
        observer.disconnect()
        return
      }
      if (field !== null && surface.contains(document.activeElement) && document.activeElement !== surface) {
        focused = true
        observer.disconnect()
        return
      }
      const target = field ?? surface
      target.focus({ preventScroll: true })
      if (document.activeElement === target) {
        if (field !== null) {
          focused = true
          observer.disconnect()
        } else waitingOnOptions = true
        surface.scrollIntoView?.({ block: 'nearest' })
      }
    }
    if (panel !== null) observer.observe(panel, { attributes: true, attributeFilter: ['aria-hidden', 'inert', 'style'] })
    observer.observe(surface, { subtree: true, childList: true, attributes: true, attributeFilter: ['disabled', 'style'] })
    focus()
    return () => { observer.disconnect(); cancelAnimationFrame(frame) }
  }, [])
  return <div ref={ref} tabIndex={-1} role="group" aria-label={label}>{children}</div>
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
  rulesSelected,
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
  rulesSelected: boolean
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
    return <RulesetForm pending={pending} selected={rulesSelected} onSubmit={onChanges} />
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

  return <PromptWithOptions prompt={prompt} initialAnswers={replaces?.event.choices ?? []} pending={pending} onAnswer={onPicks} />
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
  initialAnswers,
  pending,
  onAnswer,
}: {
  prompt: Prompt
  initialAnswers: readonly Answer[]
  pending: boolean
  onAnswer: (answers: Answer[]) => void
}) {
  const t = useT()
  const locale = useLocale()
  const [entries, setEntries] = useState<Map<string, Entry>>(new Map())
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [attempt, setAttempt] = useState(0)

  const [request, setRequest] = useState({ prompt, locale, attempt })
  if (request.prompt !== prompt || request.locale !== locale || request.attempt !== attempt) {
    setRequest({ prompt, locale, attempt })
    setLoading(true)
    setError(null)
  }

  useEffect(() => {
    let live = true
    void loadEntries(prompt).then((loaded) => {
      if (live) { setEntries(loaded); setLoading(false) }
    }).catch((cause: unknown) => {
      if (live) { setError(describeError(t, cause)); setLoading(false) }
    })
    return () => {
      live = false
    }
  }, [prompt, locale, attempt, t])

  return <Stack gap="sm">
    {loading && <Text size="sm">{t('page.loadingEllipsis')}</Text>}
    {error !== null && <><Text c="red">{error}</Text><Button onClick={() => setAttempt((n) => n + 1)}>{t('page.retry')}</Button></>}
    <PromptCard prompt={prompt} initialAnswers={initialAnswers} entries={entries} pending={pending || loading || error !== null} onAnswer={onAnswer} />
  </Stack>
}
