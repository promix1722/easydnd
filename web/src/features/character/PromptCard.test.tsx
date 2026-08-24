import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import type { Entry, Prompt } from '@/lib/api'
import { renderAt } from '@/test/render'
import type { Viewport } from '@/test/viewport'

import { PromptCard } from './PromptCard'

const entries = new Map<string, Entry>([
  ['acrobatics', { slug: 'acrobatics', name: 'Acrobatics' }],
  ['stealth', { slug: 'stealth', name: 'Stealth' }],
  ['deception', { slug: 'deception', name: 'Deception' }],
])

function skillPrompt(overrides: Partial<Prompt> = {}): Prompt {
  return {
    choice: {
      prompt: 'rogue/proficiency/0',
      choose: 2,
      kind: 'proficiency',
      from: {
        kind: 'explicit',
        options: [
          { key: 'acrobatics', kind: 'ref', ref: 'proficiency:acrobatics' },
          { key: 'stealth', kind: 'ref', ref: 'proficiency:stealth' },
          { key: 'deception', kind: 'ref', ref: 'proficiency:deception' },
        ],
      },
    },
    source: 'class:rogue',
    group: 'class',
    optional: false,
    advances: false,
    heldOnly: false,
    event: { type: 'class', ref: 'class:rogue', level: 1 },
    ...overrides,
  }
}

const viewports: Viewport[] = ['mobile', 'desktop']

describe.each(viewports)('PromptCard at %s', (viewport) => {
  it('will not confirm until the right number is picked', async () => {
    const user = userEvent.setup()
    const onAnswer = vi.fn()
    renderAt(viewport, <PromptCard prompt={skillPrompt()} entries={entries} pending={false} onAnswer={onAnswer} />)

    // The button says how many are still needed rather than sitting inert.
    expect(screen.getByRole('button', { name: /choose 2 more/i })).toBeDisabled()

    await user.click(screen.getByRole('button', { name: /Acrobatics/ }))
    expect(screen.getByRole('button', { name: /choose 1 more/i })).toBeDisabled()

    await user.click(screen.getByRole('button', { name: /Stealth/ }))
    const confirm = screen.getByRole('button', { name: /^confirm$/i })
    expect(confirm).toBeEnabled()

    await user.click(confirm)
    expect(onAnswer).toHaveBeenCalledWith(['acrobatics', 'stealth'])
  })

  it('answers with the server option keys, not with labels', async () => {
    const user = userEvent.setup()
    const onAnswer = vi.fn()
    const bundle = skillPrompt({
      choice: {
        prompt: 'rogue/starting-equipment/1',
        choose: 1,
        kind: 'equipment',
        from: {
          kind: 'explicit',
          options: [
            {
              key: '#0',
              kind: 'bundle',
              items: [{ key: 'stealth', kind: 'ref', ref: 'item:stealth', count: 1 }],
            },
          ],
        },
      },
    })
    renderAt(viewport, <PromptCard prompt={bundle} entries={entries} pending={false} onAnswer={onAnswer} />)

    await user.click(screen.getByRole('button', { name: /Stealth/ }))
    await user.click(screen.getByRole('button', { name: /^confirm$/i }))
    // The bundle has no slug of its own; the server named it by position.
    expect(onAnswer).toHaveBeenCalledWith(['#0'])
  })

  it('disables an option the character already has', () => {
    renderAt(
      viewport,
      <PromptCard
        prompt={skillPrompt({ held: ['stealth'] })}
        entries={entries}
        pending={false}
        onAnswer={vi.fn()}
      />,
    )
    expect(screen.getByRole('button', { name: /Stealth/ })).toBeDisabled()
    expect(screen.getByRole('button', { name: /Acrobatics/ })).toBeEnabled()
  })

  // Expertise doubles a proficiency you have, so there the held options are
  // the only ones on offer.
  it('inverts that for a heldOnly prompt', () => {
    renderAt(
      viewport,
      <PromptCard
        prompt={skillPrompt({ heldOnly: true, held: ['stealth'] })}
        entries={entries}
        pending={false}
        onAnswer={vi.fn()}
      />,
    )
    expect(screen.getByRole('button', { name: /Stealth/ })).toBeEnabled()
    expect(screen.getByRole('button', { name: /Acrobatics/ })).toBeDisabled()
  })

  it('clears the answer when the prompt changes', async () => {
    const user = userEvent.setup()
    const { rerender } = renderAt(
      viewport,
      <PromptCard prompt={skillPrompt()} entries={entries} pending={false} onAnswer={vi.fn()} />,
    )
    await user.click(screen.getByRole('button', { name: /Acrobatics/ }))
    expect(screen.getByRole('button', { name: /choose 1 more/i })).toBeInTheDocument()

    const next = skillPrompt({
      choice: { ...skillPrompt().choice, prompt: 'skill-versatility/proficiency/0' },
    })
    rerender(<PromptCard prompt={next} entries={entries} pending={false} onAnswer={vi.fn()} />)

    // Carrying the previous answer over would pre-select something the player
    // never chose for this question.
    expect(screen.getByRole('button', { name: /choose 2 more/i })).toBeInTheDocument()
  })

  it('says so when the compendium offers nothing', () => {
    renderAt(
      viewport,
      <PromptCard
        prompt={skillPrompt({
          choice: { prompt: 'p', choose: 1, kind: 'proficiency', from: { kind: 'explicit', options: [] } },
        })}
        entries={entries}
        pending={false}
        onAnswer={vi.fn()}
      />,
    )
    expect(screen.getByText(/SRD 5\.1 is a starter set/)).toBeInTheDocument()
  })
})
