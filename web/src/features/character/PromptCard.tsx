import { useState } from 'react'

import type { Entry, Prompt } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { Badge, Button, Group, Stack, Text } from '@/ui'

import { choosableOptions } from './options'

export interface PromptCardProps {
  prompt: Prompt
  /** Resolved catalogue entries, for options that name one. */
  entries: Map<string, Entry>
  pending: boolean
  onAnswer: (picks: string[]) => void
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
 */
export function PromptCard({ prompt, entries, pending, onAnswer }: PromptCardProps) {
  const t = useT()
  const options = choosableOptions(t, prompt, entries)
  const [picked, setPicked] = useState<string[]>(() => only(prompt, options))

  // A new prompt is a new question; carrying the previous answer over would
  // silently pre-select something the player never chose. Reset during render
  // rather than in an effect, so the new prompt is never painted once with
  // the old prompt's answer showing.
  const [shown, setShown] = useState(prompt.choice.prompt)
  if (shown !== prompt.choice.prompt) {
    setShown(prompt.choice.prompt)
    setPicked(only(prompt, options))
  }
  const target = prompt.choice.choose
  const ready = picked.length === target
  // "Choose one" is a different question from "choose two", and the
  // difference is what a click on a third option means.
  const one = target === 1

  // Ability bonuses are the one prompt where picking the same option twice is
  // meaningful: two points into Strength is how "+2 to one ability" is
  // written, and the server accepts a repeat for this kind alone. Everywhere
  // else a second click on a chosen option means "not that one after all".
  //
  // The deselect used to run first and unconditionally, which made the repeat
  // unreachable: the second click on Strength took the first one back, so the
  // improvement could only ever be spread across two abilities. Clear is how
  // a repeat is undone.
  const repeatable = prompt.choice.kind === 'ability-bonus'

  const toggle = (key: string) => {
    setPicked((current) => {
      if (current.includes(key) && !repeatable) return current.filter((k) => k !== key)
      // Where only one is wanted, picking another *is* changing your mind:
      // the old answer goes. Making somebody unpick before they can pick is
      // asking them to operate the form rather than answer the question.
      if (one) return [key]
      if (current.length >= target) return current
      return [...current, key]
    })
  }

  return (
    <Stack gap="md">
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
                count > 1 ? <Badge size="sm">×{count}</Badge> : option.disabled ? (
                  <Text size="xs" c="dimmed">
                    {option.reason}
                  </Text>
                ) : null
              }
            >
              <Stack gap={2} style={{ textAlign: 'left' }}>
                <Text size="sm" style={{ whiteSpace: 'normal' }}>
                  {option.label}
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

      <Group>
        <Button onClick={() => onAnswer(picked)} disabled={!ready} loading={pending}>
          {ready ? t('answer.confirm') : t('prompt.chooseMore', { count: target - picked.length })}
        </Button>
        {picked.length > 0 && (
          <Button variant="subtle" onClick={() => setPicked([])}>
            {t('prompt.clear')}
          </Button>
        )}
      </Group>
    </Stack>
  )
}


/**
 * The answer a question of one option answers itself with.
 *
 * Taking a level asks which class it goes into, and while multiclassing is not
 * offered there is exactly one: pressing the only button before pressing
 * Confirm is a step that decides nothing. So a prompt that wants one answer
 * and offers one that can be given starts with it chosen -- and chosen is not
 * confirmed, which is what keeps this a shortcut rather than a decision made
 * on the player's behalf. When multiclassing returns the list grows, this
 * stops firing, and the question is a real one again.
 */
function only(prompt: Prompt, options: readonly { key: string; disabled: boolean }[]): string[] {
  if (prompt.choice.choose !== 1) return []
  const open = options.filter((option) => !option.disabled)
  return open.length === 1 && open[0] !== undefined ? [open[0].key] : []
}
