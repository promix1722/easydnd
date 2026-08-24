import { useState } from 'react'

import type { Entry, Prompt } from '@/lib/api'
import { Badge, Button, Card, Group, Stack, Text } from '@/ui'

import { choosableOptions } from './options'

import { titleCase } from '@/domain'

export interface PromptCardProps {
  prompt: Prompt
  /** Resolved catalogue entries, for options that name one. */
  entries: Map<string, Entry>
  pending: boolean
  onAnswer: (picks: string[]) => void
}

/**
 * Renders one prompt and collects an answer.
 *
 * There is deliberately one component for every kind of prompt rather than one
 * per kind. Every prompt in this application is the same shape -- choose N of
 * these, where each option carries the key that names it -- because the server
 * synthesises "which race?" into the compendium's own grammar rather than
 * inventing a second one. A per-kind component set would be several files that
 * differ only in their labels.
 *
 * What the kinds do change is presentation, and that is `describe` below.
 */
export function PromptCard({ prompt, entries, pending, onAnswer }: PromptCardProps) {
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

  const options = choosableOptions(prompt, entries)
  const target = prompt.choice.choose
  const ready = picked.length === target

  const toggle = (key: string) => {
    setPicked((current) => {
      if (current.includes(key)) return current.filter((k) => k !== key)
      // Ability bonuses are the one prompt where picking the same option
      // twice is meaningful: two points into Dexterity is "+2 to one
      // ability". Everywhere else a second click on a chosen option is a
      // deselect, handled above.
      if (current.length >= target) return current
      return [...current, key]
    })
  }

  return (
    <Card withBorder padding="lg" radius="md">
      <Stack gap="md">
        <Group justify="space-between" align="flex-start" wrap="nowrap">
          <div>
            <Text fw={600}>{describe(prompt)}</Text>
            {prompt.source !== undefined && (
              <Text size="xs" c="dimmed">
                from {titleCase(prompt.source.split(':')[1] ?? prompt.source)}
              </Text>
            )}
          </div>
          {prompt.optional && (
            <Badge variant="light" color="gray">
              optional
            </Badge>
          )}
        </Group>

        <Stack gap="xs">
          {options.map((option) => {
            const count = picked.filter((k) => k === option.key).length
            return (
              <Button
                key={option.key}
                variant={count > 0 ? 'filled' : 'default'}
                justify="space-between"
                disabled={option.disabled}
                onClick={() => toggle(option.key)}
                rightSection={
                  count > 1 ? <Badge size="sm">×{count}</Badge> : option.disabled ? (
                    <Text size="xs" c="dimmed">
                      {option.reason}
                    </Text>
                  ) : null
                }
              >
                <Text size="sm" style={{ whiteSpace: 'normal', textAlign: 'left' }}>
                  {option.label}
                  {option.detail !== undefined && (
                    <Text span size="xs" c="dimmed">
                      {' '}
                      -- {option.detail}
                    </Text>
                  )}
                </Text>
              </Button>
            )
          })}
          {options.length === 0 && (
            <Text size="sm" c="dimmed">
              The compendium offers nothing here. SRD 5.1 is a starter set -- one background, one
              feat -- so this is a gap in the data rather than in the rules.
            </Text>
          )}
        </Stack>

        <Group>
          <Button onClick={() => onAnswer(picked)} disabled={!ready} loading={pending}>
            {ready ? 'Confirm' : `Choose ${target - picked.length} more`}
          </Button>
          {picked.length > 0 && (
            <Button variant="subtle" onClick={() => setPicked([])}>
              Clear
            </Button>
          )}
        </Group>
      </Stack>
    </Card>
  )
}

/** The question, phrased for a person. */
function describe(prompt: Prompt): string {
  const { choose, kind } = prompt.choice
  switch (kind) {
    case 'race':
      return 'Choose a race'
    case 'subrace':
      return 'Choose a subrace'
    case 'background':
      return 'Choose a background'
    case 'class':
      return 'Choose a class'
    case 'subclass':
      return 'Choose an archetype'
    case 'level':
      return 'Gain a level in which class?'
    case 'alignment':
      return 'Choose an alignment'
    case 'proficiency':
      return prompt.heldOnly
        ? `Double your proficiency in ${choose} of these`
        : `Choose ${choose} to be proficient in`
    case 'ability-bonus':
      return `Raise ${choose} ability ${choose === 1 ? 'score' : 'scores'}`
    case 'ability-scores':
      return 'Improve your ability scores, or take a feat'
    case 'language':
      return `Choose ${choose} more ${choose === 1 ? 'language' : 'languages'}`
    case 'equipment':
      return 'Choose your starting equipment'
    case 'personality':
      return `Choose ${choose} personality ${choose === 1 ? 'trait' : 'traits'}`
    case 'ideal':
      return 'Choose an ideal'
    case 'bond':
      return 'Choose a bond'
    case 'flaw':
      return 'Choose a flaw'
    case 'spell':
      return `Choose ${choose} ${choose === 1 ? 'spell' : 'spells'}`
    default:
      return `Choose ${choose}`
  }
}
