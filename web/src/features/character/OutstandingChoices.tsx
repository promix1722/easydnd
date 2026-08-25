import type { Prompt } from '@/lib/api'
import { Badge, Button, Group, Stack, Text } from '@/ui'

import { offersOptions } from './options'
import { refName } from './refNames'

import { titleCase } from '@/domain'

export interface OutstandingChoicesProps {
  prompts: readonly Prompt[]
  /** Compendium names for the entries the prompts hang off. */
  names?: ReadonlyMap<string, string>
  /** The prompt currently being answered, drawn as the one in hand. */
  selected?: string
  /**
   * Present where the list is a way in. Without it the list is a statement of
   * what is left -- which is what the sheet wants, and what a party list would
   * want if it ever grew one.
   */
  onOpen?: (prompt: Prompt) => void
  /**
   * What to say when there is nothing outstanding.
   *
   * Never names a category: "Nothing left here", not "nothing left in race".
   * The category's word belongs to its tab and appears exactly once.
   */
  empty?: string
}

/**
 * What a character still has to decide.
 *
 * One component, three callers: a build tab filtered to its own category, the
 * character sheet showing all of them, and -- when it exists -- the level-up
 * page. There is deliberately no second notion of "outstanding" anywhere in
 * this client: the server answers `/prompts` with exactly the questions the
 * character is being asked, and a prompt that is not in that list is not a
 * question, whatever the sheet looks like.
 *
 * Nothing here can be answered before it is asked, because there is nothing
 * here that was not asked.
 */
export function OutstandingChoices({
  prompts,
  names,
  selected,
  onOpen,
  empty = 'Nothing left here.',
}: OutstandingChoicesProps) {
  if (prompts.length === 0) {
    return (
      <Text size="sm" c="dimmed">
        {empty}
      </Text>
    )
  }

  return (
    <Stack gap={4} align="stretch">
      {prompts.map((prompt) => {
        const line = (
          <Group gap={8} wrap="nowrap" justify="space-between" w="100%">
            <Text size="sm" style={{ whiteSpace: 'normal', textAlign: 'left' }}>
              {nameOf(prompt)}
              {prompt.source !== undefined && (
                <Text span size="xs" c="dimmed">
                  {' '}
                  · from {refName(prompt.source, names ?? new Map())}
                </Text>
              )}
            </Text>
            {prompt.optional && (
              <Badge size="xs" variant="light" color="gray">
                optional
              </Badge>
            )}
          </Group>
        )

        if (onOpen === undefined) {
          return <div key={prompt.choice.prompt}>{line}</div>
        }
        return (
          <Button
            key={prompt.choice.prompt}
            variant={selected === prompt.choice.prompt ? 'light' : 'subtle'}
            justify="flex-start"
            onClick={() => onOpen(prompt)}
          >
            {line}
          </Button>
        )
      })}
    </Stack>
  )
}

/**
 * A choice named as a thing rather than asked as a question.
 *
 * Deliberately a different register from `PromptCard`, which asks -- "Choose a
 * race" -- because this is a list of what is left, and a list of imperatives
 * reads as a list of orders. It also keeps the two vocabularies apart: nothing
 * here is a heading over an answering surface, so nothing here has to agree
 * with one.
 *
 * A kind this client has not learned yet still gets a line. The server may
 * grow a kind before the browser does, and a choice you cannot see is worse
 * than a choice named plainly.
 */
function nameOf(prompt: Prompt): string {
  const { choose, kind } = prompt.choice
  switch (kind) {
    case 'race':
      return 'A race'
    case 'subrace':
      return 'A subrace'
    case 'background':
      return 'A background'
    case 'class':
      return 'A class'
    case 'subclass':
      return 'An archetype'
    case 'level':
      return 'Another level, in one of your classes'
    case 'alignment':
      return 'An alignment'
    case 'text':
      return 'A name'
    case 'proficiency':
      return prompt.heldOnly
        ? `${count(choose)} to double your proficiency in`
        : `${count(choose)} to be proficient in`
    case 'ability-bonus':
      return `${count(choose)} ability ${choose === 1 ? 'score' : 'scores'} to raise`
    // Two questions share this kind. The one that offers options is the
    // improvement a level grants; the one that offers none is the six numbers
    // a character starts with, which is a form rather than a choice of N.
    case 'ability-scores':
      return offersOptions(prompt)
        ? 'An improvement to your scores, or a feat'
        : `${count(choose)} ability scores`
    case 'language':
      return `${count(choose)} more ${choose === 1 ? 'language' : 'languages'}`
    case 'equipment':
      return 'Starting equipment'
    case 'personality':
      return `${count(choose)} personality ${choose === 1 ? 'trait' : 'traits'}`
    case 'ideal':
      return 'An ideal'
    case 'bond':
      return 'A bond'
    case 'flaw':
      return 'A flaw'
    case 'spell':
      return `${count(choose)} ${choose === 1 ? 'spell' : 'spells'}`
    default:
      return `${count(choose)} of ${titleCase(kind)}`
  }
}

/** Small counts read as words; large ones read as numbers. */
function count(n: number): string {
  return ['Zero', 'One', 'Two', 'Three', 'Four', 'Five', 'Six'][n] ?? String(n)
}
