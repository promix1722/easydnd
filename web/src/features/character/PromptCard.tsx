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
  const [picked, setPicked] = useState<string[]>([])

  // A new prompt is a new question; carrying the previous answer over would
  // silently pre-select something the player never chose. Reset during render
  // rather than in an effect, so the new prompt is never painted once with
  // the old prompt's answer showing.
  const [shown, setShown] = useState(prompt.choice.prompt)
  if (shown !== prompt.choice.prompt) {
    setShown(prompt.choice.prompt)
    setPicked([])
  }

  const options = choosableOptions(t, prompt, entries)
  const target = prompt.choice.choose
  const ready = picked.length === target
  // "Choose one" is a different question from "choose two", and the
  // difference is what a click on a third option means.
  const one = target === 1

  const toggle = (key: string) => {
    setPicked((current) => {
      if (current.includes(key)) return current.filter((k) => k !== key)
      // Where only one is wanted, picking another *is* changing your mind:
      // the old answer goes. Making somebody unpick before they can pick is
      // asking them to operate the form rather than answer the question.
      if (one) return [key]
      // Ability bonuses are the one prompt where picking the same option
      // twice is meaningful: two points into Dexterity is "+2 to one
      // ability". Everywhere else a second click on a chosen option is a
      // deselect, handled above.
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

