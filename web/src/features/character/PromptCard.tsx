import { useState } from 'react'

import type { Answer, Choice, Entry, Option, Prompt } from '@/lib/api'
import { useT } from '@/lib/i18n'
import type { Translate } from '@/lib/i18n'
import { Button, Group, Stack, Text } from '@/ui'

import { choosableOptions, optionLabel } from './options'
import type { Choosable } from './options'

export interface PromptCardProps {
  prompt: Prompt
  /** Resolved catalogue entries, for options that name one. */
  entries: Map<string, Entry>
  pending: boolean
  /**
   * Every answer the card collected, parent first.
   *
   * More than one when a branch was answered here: the server validates a
   * batch entry by entry against a log that grows as it goes, so the branch
   * answer opens the prompt the answer after it satisfies.
   */
  onAnswer: (answers: Answer[]) => void
}

/**
 * Renders one prompt's options and collects an answer.
 *
 * There is deliberately one component for every kind of prompt rather than one
 * per kind. Every prompt in this application is the same shape -- choose N of
 * these, where each option carries the key that names it -- because the server
 * synthesises "which race?" into the compendium's own grammar rather than
 * inventing a second one. A per-kind component set would be several files that
 * differ only in their labels.
 *
 * It does not say what the question is. The block it opens inside is headed by
 * the choice's own name, and a card that repeated it -- "Two more languages",
 * then "Choose 2 more languages" -- would be asking twice.
 *
 * # A choice inside a choice is answered here
 *
 * Some options are themselves questions: "a martial weapon and a shield, or
 * two martial weapons", or the improvement's "raise two scores, or take a
 * feat". Picking one used to confirm it, post it, wait for the server to pose
 * the branch, and draw it as a *second* block further down the tab -- so a
 * question the player was in the middle of answering became two questions in
 * two places.
 *
 * It resolves here instead. The branch's whole inner choice already ships
 * inside the option (`Option.choice`), so picking it draws its options in this
 * same card with nothing fetched and nothing posted, and Confirm sends the
 * branch answer and its contents together in one event.
 */
export function PromptCard({ prompt, entries, pending, onAnswer }: PromptCardProps) {
  const t = useT()

  // One piece of state, because a new prompt has to reset all of it at once:
  // the questions this card is walking, the answers it has, and the picks in
  // hand. Carrying any of them over would silently pre-select something the
  // player never chose. Reset during render rather than in an effect, so the
  // new prompt is never painted once wearing the old one's answer.
  const [progress, setProgress] = useState<Progress>(() => begin(t, prompt, entries))
  if (progress.for !== prompt.choice.prompt) setProgress(begin(t, prompt, entries))

  const stage = progress.stages[progress.answers.length] ?? prompt.choice
  // The stage wears the prompt's envelope: `held` and `heldOnly` are about the
  // character rather than about which question is on screen, and the server
  // reports `held` across a prompt's branches for exactly this reason.
  const asked: Prompt = { ...prompt, choice: stage }
  const options = choosableOptions(t, asked, entries)

  const target = stage.choose
  const picked = progress.picked
  const ready = picked.length === target
  // "Choose one" is a different question from "choose two", and the
  // difference is what a click on a third option means.
  const one = target === 1

  // Two points into Strength is how "+2 to one ability" is written, and the
  // prompt is what says so. It used to be inferred from the choice's kind,
  // which let a half-elf spend both of their +1s on one score -- their two are
  // the same kind over the same options and must go to two *different*
  // scores.
  const repeatable = stage.repeatable === true

  const toggle = (key: string) => {
    setProgress((current) =>
      settle(t, prompt, entries, {
        ...current,
        picked: pick(current.picked, key, { target, one, repeatable }),
      }),
    )
  }

  return (
    <Stack gap="md">
      {progress.answers.length > 0 && (
        <Text size="xs" c="dimmed">
          {progress.answers
            .map((answer) => labelsOf(t, prompt, progress.stages, answer, entries))
            .join(t('option.bundleJoin'))}
        </Text>
      )}

      <Stack gap="xs">
        {options.map((option) => {
          const count = picked.filter((k) => k === option.key).length
          // Once as many are picked as are wanted, the rest go grey: the
          // question has been answered, and an option that still looks
          // pressable but does nothing reads as a broken button. Not where
          // only one is wanted -- there the rest are how you change it.
          const spent = ready && !one && count === 0
          return (
            <Button
              key={option.key}
              variant={count > 0 ? 'filled' : 'default'}
              justify="space-between"
              // The description underneath makes a picked option two or three
              // lines tall, and a button that fixes its own height would crop
              // it. Padded rather than sized.
              h="auto"
              py="xs"
              disabled={option.disabled || spent}
              onClick={() => toggle(option.key)}
              rightSection={
                option.disabled ? (
                  <Text size="xs" c="dimmed">
                    {option.reason}
                  </Text>
                ) : null
              }
            >
              <Stack gap={2} style={{ textAlign: 'left' }}>
                <Text size="sm" style={{ whiteSpace: 'normal' }}>
                  {/*
                    Two points into Strength reads "Strength +2". It used to
                    read "Strength +1" with a ×2 badge beside it, which is the
                    number the option is worth and the number of them, left for
                    the player to multiply.
                  */}
                  {stacked(t, stage, option.key, count, entries) ?? option.label}
                </Text>
                {/*
                  Only under the one that was picked. Every option carrying its
                  own paragraph turns a list of six into a page nobody reads,
                  and the same text cut to fit one line stops mid-word -- so it
                  is shown where it is being decided about, in full.
                */}
                {count > 0 && option.detail !== undefined && (
                  <Text size="xs" style={{ whiteSpace: 'normal', opacity: 0.8 }}>
                    {option.detail}
                  </Text>
                )}
              </Stack>
            </Button>
          )
        })}
        {options.length === 0 && (
          <Text size="sm" c="dimmed">
            {t('prompt.nothingOffered')}
          </Text>
        )}
      </Stack>

      {repeatable && <Text size="xs" c="dimmed">{t('prompt.pointsToSpend')}</Text>}

      <Group>
        <Button
          onClick={() => onAnswer([...progress.answers, { prompt: stage.prompt, picks: [...picked] }])}
          disabled={!ready}
          loading={pending}
        >
          {ready ? t('answer.confirm') : t('prompt.chooseMore', { count: target - picked.length })}
        </Button>
        {/*
          Back to the top of the question, not just to an empty list. Once a
          branch has been taken, clearing the picks inside it would leave the
          player in a branch they may not have wanted with no way out.
        */}
        {(picked.length > 0 || progress.answers.length > 0) && (
          <Button variant="subtle" onClick={() => setProgress(begin(t, prompt, entries))}>
            {t('prompt.clear')}
          </Button>
        )}
      </Group>
    </Stack>
  )
}

/**
 * One card's walk through a question and whatever it opened.
 *
 * `stages` is the questions asked, `answers` the ones already given, so the
 * live question is always `stages[answers.length]`. `for` is the prompt this
 * belongs to, which is what makes a stale walk detectable on the way in.
 */
interface Progress {
  for: string
  stages: Choice[]
  answers: Answer[]
  picked: string[]
}

function begin(t: Translate, prompt: Prompt, entries: Map<string, Entry>): Progress {
  return settle(t, prompt, entries, {
    for: prompt.choice.prompt,
    stages: [prompt.choice],
    answers: [],
    picked: only(prompt, choosableOptions(t, prompt, entries)),
  })
}

/**
 * Takes every stage the picks in hand have already finished.
 *
 * A stage is finished when it has as many picks as it wants, and taking it
 * means recording its answer and drawing whatever it opened. It loops because
 * a stage can answer itself on arrival -- `only` picks the one option a
 * question of one option has, and the improvement's "raise your scores" is
 * exactly that: a branch selector with a single branch, which is not a
 * question and should never have been a click.
 *
 * Bounded because it is a loop over data from the network. Nothing in the
 * compendium nests remotely this deep; a cycle would be a bug, and hanging the
 * card is a worse way to report one than stopping.
 */
function settle(
  t: Translate,
  prompt: Prompt,
  entries: Map<string, Entry>,
  from: Progress,
): Progress {
  let out = from
  for (let depth = 0; depth < 8; depth += 1) {
    const stage = out.stages[out.answers.length]
    if (stage === undefined || out.picked.length !== stage.choose) return out
    const opened = opens(stage, out.picked)
    if (opened.length === 0) return out

    const answers = [...out.answers, { prompt: stage.prompt, picks: out.picked }]
    const stages = [...out.stages, ...opened]
    const following = stages[answers.length]
    out = {
      for: out.for,
      stages,
      answers,
      picked: following === undefined ? [] : opening(t, prompt, following, entries),
    }
  }
  return out
}

/** What the next question starts with picked, if it answers itself. */
function opening(
  t: Translate,
  prompt: Prompt,
  choice: Choice,
  entries: Map<string, Entry>,
): string[] {
  const next: Prompt = { ...prompt, choice }
  return only(next, choosableOptions(t, next, entries))
}

/**
 * The questions a set of picks opened, in the order the server would pose them.
 *
 * It mirrors `addOpened` in the domain: a branch contributes its own choice,
 * and a bundle contributes whatever is nested inside it. The bundle case is
 * not a nicety -- "a martial weapon and a shield" is a bundle whose first item
 * is the question "which martial weapon?".
 */
function opens(choice: Choice, picks: readonly string[]): Choice[] {
  const out: Choice[] = []
  const walk = (option: Option) => {
    if (option.kind === 'nested' && option.choice !== undefined) out.push(option.choice)
    if (option.kind === 'bundle') (option.items ?? []).forEach(walk)
  }
  // Distinct, because a repeatable prompt can name one option twice and a
  // question does not open twice for being answered with two points.
  for (const key of new Set(picks)) {
    const option = (choice.from.options ?? []).find((o) => o.key === key)
    if (option !== undefined) walk(option)
  }
  return out
}

/**
 * An option's label with its picks added up, or undefined to use the plain one.
 *
 * Only where the same option can be picked twice, and only once it has been:
 * everywhere else the label is what it always was.
 */
function stacked(
  t: Translate,
  stage: Choice,
  key: string,
  count: number,
  entries: Map<string, Entry>,
): string | undefined {
  if (count < 2) return undefined
  const option = (stage.from.options ?? []).find((each) => each.key === key)
  return option === undefined ? undefined : optionLabel(t, option, entries, count)
}

/** What one pick does to the picks in hand. */
function pick(
  current: readonly string[],
  key: string,
  { target, one, repeatable }: { target: number; one: boolean; repeatable: boolean },
): string[] {
  // Where a repeat is meaningful, clicking spends another point rather than
  // taking the first one back -- until the points run out, and then the same
  // click takes them all off this one. So an option cycles 0, 1, 2, 0 and
  // there is always a way back to nothing without reaching for Clear.
  if (repeatable) {
    if (current.length >= target) return current.filter((k) => k !== key)
    return [...current, key]
  }
  if (current.includes(key)) return current.filter((k) => k !== key)
  // Where only one is wanted, picking another *is* changing your mind: the
  // old answer goes. Making somebody unpick before they can pick is asking
  // them to operate the form rather than answer the question.
  if (one) return [key]
  if (current.length >= target) return [...current]
  return [...current, key]
}

/** What an answer given earlier in this card reads as, for the line above. */
function labelsOf(
  t: Translate,
  prompt: Prompt,
  stages: readonly Choice[],
  answer: Answer,
  entries: Map<string, Entry>,
): string {
  const stage = stages.find((choice) => choice.prompt === answer.prompt)
  if (stage === undefined) return answer.picks.join(', ')
  const options = choosableOptions(t, { ...prompt, choice: stage }, entries)
  return answer.picks
    .map((key) => options.find((option) => option.key === key)?.label ?? key)
    .join(', ')
}

/**
 * The answer a question of one option answers itself with.
 *
 * A prompt that wants one answer and offers one that can be given starts with
 * it chosen -- and chosen is not confirmed, which is what keeps this a
 * shortcut rather than a decision made on the player's behalf. Pressing the
 * only button before pressing Confirm is a step that decides nothing.
 */
function only(prompt: Prompt, options: readonly Choosable[]): string[] {
  if (prompt.choice.choose !== 1) return []
  const open = options.filter((option) => !option.disabled)
  return open.length === 1 && open[0] !== undefined ? [open[0].key] : []
}
