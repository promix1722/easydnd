import { screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import type { Prompt } from '@/lib/api'
import { renderAt } from '@/test/render'

import { OutstandingChoices } from './OutstandingChoices'

function prompt(over: Partial<Prompt> & { choice: Prompt['choice'] }): Prompt {
  return {
    group: 'race',
    optional: false,
    advances: false,
    event: { type: 'race' },
    heldOnly: false,
    ...over,
  }
}

const SKILLS = prompt({
  choice: {
    prompt: 'skill-versatility/proficiency/0',
    choose: 2,
    kind: 'proficiency',
    from: { kind: 'explicit', options: [] },
  },
  source: 'race:half-elf',
})

const LANGUAGE = prompt({
  choice: {
    prompt: 'half-elf/language/0',
    choose: 1,
    kind: 'language',
    from: { kind: 'collection', collection: 'language' },
  },
  source: 'race:half-elf',
})

const LEVEL = prompt({
  choice: {
    prompt: 'character/level',
    choose: 1,
    kind: 'level',
    from: { kind: 'collection', collection: 'class' },
  },
  group: 'advance',
  optional: true,
  advances: true,
})

describe('OutstandingChoices', () => {
  it('names each choice and what posed it', () => {
    renderAt(
      'desktop',
      <OutstandingChoices
        prompts={[SKILLS, LANGUAGE]}
        names={new Map([['race:half-elf', 'Half-Elf']])}
      />,
    )

    expect(screen.getByText(/Two to be proficient in/)).toBeInTheDocument()
    expect(screen.getByText(/One more language/)).toBeInTheDocument()
    // "from Half-Elf" is what makes a loose list of questions legible: two
    // skills from a race and two from a class are different questions.
    expect(screen.getAllByText(/from Half-Elf/)).toHaveLength(2)
  })

  it('says an optional choice is optional', () => {
    renderAt('desktop', <OutstandingChoices prompts={[LEVEL]} />)

    expect(screen.getByText('optional')).toBeInTheDocument()
  })

  it('is a statement rather than a way in', () => {
    renderAt('desktop', <OutstandingChoices prompts={[SKILLS]} />)

    // The sheet renders it read-only, and that is the only way it renders: it
    // says what is left, and the way to answer is the link beside it. The
    // build screen draws the same choices as blocks that open, because there
    // they are ways in.
    expect(screen.queryAllByRole('button')).toHaveLength(0)
  })
})
