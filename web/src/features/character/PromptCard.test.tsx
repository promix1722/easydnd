import { screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import type { Entry, Prompt } from '@/lib/api'
import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'

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
    heldOnly: false,
    event: { type: 'class', ref: 'class:rogue', level: 1 },
    ...overrides,
  }
}


/**
 * One viewport, not two. Only `Columns`, `DataList`, `ModalSheet`,
 * `SectionDeck`, `TabDeck`, `SheetBody` and `RootShell` branch on width, and the suite runs without CSS, so a responsive
 * prop cannot move the DOM either -- nothing in this tree reaches any of them,
 * so a test at one width is a test of both. See docs/web.md.
 */
describe('PromptCard', () => {
  const viewport = 'desktop'

  it('will not confirm until the right number is picked', async () => {
    const user = setupUser()
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
    expect(onAnswer).toHaveBeenCalledWith([
      { prompt: 'rogue/proficiency/0', picks: ['acrobatics', 'stealth'] },
    ])
  })

  it('answers with the server option keys, not with labels', async () => {
    const user = setupUser()
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
              key: 'shortbow+arrow',
              kind: 'bundle',
              items: [{ key: 'stealth', kind: 'ref', ref: 'item:stealth', count: 1 }],
            },
          ],
        },
      },
    })
    renderAt(viewport, <PromptCard prompt={bundle} entries={entries} pending={false} onAnswer={onAnswer} />)

    // The one option a question of one option can be answered with is already
    // chosen -- see `only` -- so what is left is to confirm it.
    await user.click(screen.getByRole('button', { name: /^confirm$/i }))
    // The bundle has no slug of its own; the server named it by its contents.
    expect(onAnswer).toHaveBeenCalledWith([
      { prompt: 'rogue/starting-equipment/1', picks: ['shortbow+arrow'] },
    ])
  })

  // Taking a level asks which class it goes into, and while multiclassing is
  // not offered there is one answer: pressing it before pressing Confirm is a
  // step that decides nothing.
  it('starts with the only answer chosen, where there is only one', () => {
    const single = skillPrompt({
      choice: {
        prompt: 'character/level',
        choose: 1,
        kind: 'level',
        from: {
          kind: 'explicit',
          options: [{ key: 'bard', kind: 'ref', ref: 'class:bard', count: 1 }],
        },
      },
    })
    renderAt(viewport, <PromptCard prompt={single} entries={entries} pending={false} onAnswer={vi.fn()} />)

    // Chosen, and not confirmed: the level is still the player's to take.
    expect(screen.getByRole('button', { name: /^confirm$/i })).toBeEnabled()
    expect(screen.queryByRole('button', { name: /choose 1 more/i })).not.toBeInTheDocument()
  })

  // Two options is a real question, and answering it for the player would be
  // choosing their character's next level for them.
  it('chooses nothing where there is a choice to make', () => {
    renderAt(viewport, <PromptCard prompt={skillPrompt()} entries={entries} pending={false} onAnswer={vi.fn()} />)

    expect(screen.getByRole('button', { name: /choose 2 more/i })).toBeDisabled()
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
    const user = setupUser()
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

  it('swaps the answer when only one is wanted', async () => {
    const user = setupUser()
    const one = skillPrompt({ choice: { ...skillPrompt().choice, choose: 1 } })
    renderAt(viewport, <PromptCard prompt={one} entries={entries} pending={false} onAnswer={vi.fn()} />)

    await user.click(screen.getByRole('button', { name: /Acrobatics/ }))
    // Picking another is how you change your mind. Making somebody unpick
    // first would be asking them to operate the form rather than answer it.
    const stealth = screen.getByRole('button', { name: /Stealth/ })
    expect(stealth).toBeEnabled()
    await user.click(stealth)

    expect(stealth).toHaveAttribute('data-variant', 'filled')
    expect(screen.getByRole('button', { name: /Acrobatics/ })).toHaveAttribute(
      'data-variant',
      'default',
    )
  })

  it('greys out what is left once as many are picked as are wanted', async () => {
    const user = setupUser()
    renderAt(
      viewport,
      <PromptCard prompt={skillPrompt()} entries={entries} pending={false} onAnswer={vi.fn()} />,
    )

    await user.click(screen.getByRole('button', { name: /Acrobatics/ }))
    expect(screen.getByRole('button', { name: /Deception/ })).toBeEnabled()

    await user.click(screen.getByRole('button', { name: /Stealth/ }))

    // Two of two are picked, so the third does nothing -- and an option that
    // still looks pressable but does nothing reads as a broken button.
    expect(screen.getByRole('button', { name: /Deception/ })).toBeDisabled()
    // The two that were picked stay live, because unpicking is how you undo.
    expect(screen.getByRole('button', { name: /Acrobatics/ })).toBeEnabled()
  })

  it('does not ask the question the block around it is already asking', () => {
    renderAt(
      viewport,
      <PromptCard prompt={skillPrompt()} entries={entries} pending={false} onAnswer={vi.fn()} />,
    )

    // The block is headed "2 to be proficient in · from Rogue". A card that
    // said "Choose 2 to be proficient in" under it would be asking twice.
    expect(screen.queryByText('Choose 2 to be proficient in')).not.toBeInTheDocument()
    expect(screen.queryByText('from Rogue')).not.toBeInTheDocument()
  })
})

/**
 * What an option says about itself, and when.
 *
 * The compendium's description used to sit on the same line as the name, cut
 * to 120 characters -- six options each trailing half a sentence, none of them
 * finishing it. It is shown under the option that was picked instead, whole:
 * that is the one being decided about, and it is the only one with room.
 */
describe('an option description', () => {
  const viewport = 'desktop'

  const DESC =
    'You have draconic ancestry. Choose one type of dragon from the Draconic Ancestry table. ' +
    'Your breath weapon and damage resistance are determined by the type.'

  const dragons = new Map<string, Entry>([
    ['draconic-ancestry-black', { slug: 'draconic-ancestry-black', name: 'Black', desc: [DESC] }],
    ['draconic-ancestry-blue', { slug: 'draconic-ancestry-blue', name: 'Blue', desc: [DESC] }],
  ])

  const ancestry: Prompt = {
    choice: {
      prompt: 'draconic-ancestry/trait/0',
      choose: 1,
      kind: 'trait',
      from: {
        kind: 'explicit',
        options: [
          { key: 'draconic-ancestry-black', kind: 'ref', ref: 'trait:draconic-ancestry-black' },
          { key: 'draconic-ancestry-blue', kind: 'ref', ref: 'trait:draconic-ancestry-blue' },
        ],
      },
    },
    group: 'race',
    optional: false,
    heldOnly: false,
    event: { type: 'race', ref: 'race:dragonborn' },
  }

  it('stays out of the list until its option is picked, and then says all of it', async () => {
    const user = setupUser()
    renderAt(
      viewport,
      <PromptCard prompt={ancestry} entries={dragons} pending={false} onAnswer={vi.fn()} />,
    )

    // Nothing describes itself while the list is still a list.
    expect(screen.queryByText(/draconic ancestry table/i)).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /^Black/ }))

    // Whole, not cut mid-word: it is under the name now, so it has the room.
    expect(screen.getByText(DESC)).toBeInTheDocument()
    // And only the picked one carries it.
    expect(screen.getAllByText(DESC)).toHaveLength(1)
  })
})

/**
 * A choice inside a choice, answered in the card that offered it.
 *
 * The improvement a level grants is the shape: "raise your scores, or take a
 * feat", where the first branch is itself "choose 2 of these six". Picking a
 * branch used to confirm it, post it, and wait for the server to pose the
 * branch as a second block further down the tab.
 */
describe('PromptCard with a branch', () => {
  const viewport = 'desktop'

  const scores = {
    prompt: 'rogue/ability-score-improvement/4/0',
    choose: 2,
    kind: 'ability-bonus',
    repeatable: true,
    from: {
      kind: 'explicit' as const,
      options: [
        { key: 'str', kind: 'ability-bonus' as const, ability: 'str', bonus: 1 },
        { key: 'dex', kind: 'ability-bonus' as const, ability: 'dex', bonus: 1 },
      ],
    },
  }

  const improvement: Prompt = {
    choice: {
      prompt: 'rogue/ability-score-improvement/4',
      choose: 1,
      kind: 'ability-scores',
      from: {
        kind: 'explicit',
        options: [
          { key: 'rogue/ability-score-improvement/4/0', kind: 'nested', choice: scores },
        ],
      },
    },
    source: 'class:rogue',
    group: 'class',
    level: 4,
    optional: false,
    heldOnly: false,
    event: { type: 'level', ref: 'class:rogue', level: 4 },
  }

  it('draws the branch in the same card and answers both together', async () => {
    const user = setupUser()
    const onAnswer = vi.fn()
    renderAt(
      viewport,
      <PromptCard prompt={improvement} entries={new Map()} pending={false} onAnswer={onAnswer} />,
    )

    // The branch is the only option, so `only` has it chosen -- and choosing
    // it is what draws what it offers, here, without a round trip.
    const strength = await screen.findByRole('button', { name: /Strength/ })
    expect(strength).toBeInTheDocument()

    await user.click(strength)
    await user.click(screen.getByRole('button', { name: /Dexterity/ }))
    await user.click(screen.getByRole('button', { name: /^confirm$/i }))

    // Parent first, because the server validates a batch answer by answer
    // against a log that grows: the second is legal because the first landed.
    expect(onAnswer).toHaveBeenCalledWith([
      {
        prompt: 'rogue/ability-score-improvement/4',
        picks: ['rogue/ability-score-improvement/4/0'],
      },
      { prompt: 'rogue/ability-score-improvement/4/0', picks: ['str', 'dex'] },
    ])
  })

  // "+2 to one ability, or +1 to two" is the rule, and two points into one
  // score is the first half of it.
  it('spends both points on one score where the prompt says it may', async () => {
    const user = setupUser()
    const onAnswer = vi.fn()
    renderAt(
      viewport,
      <PromptCard prompt={improvement} entries={new Map()} pending={false} onAnswer={onAnswer} />,
    )

    await user.click(await screen.findByRole('button', { name: /Dexterity \+1/ }))
    await user.click(screen.getByRole('button', { name: /Dexterity \+1/ }))
    // The second point reads as what the score gains, not as a multiplier
    // beside the number it multiplies.
    expect(await screen.findByRole('button', { name: /Dexterity \+2/ })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /^confirm$/i }))
    expect(onAnswer.mock.calls[0]?.[0]?.[1]).toEqual({
      prompt: 'rogue/ability-score-improvement/4/0',
      picks: ['dex', 'dex'],
    })
  })

  // A half-elf's two bonuses are the same kind over the same options and are
  // "+1 to two *different* scores". Only the prompt can tell them apart.
  it('will not repeat where the prompt does not say it may', async () => {
    const user = setupUser()
    const strict: Prompt = {
      ...improvement,
      choice: { ...scores, prompt: 'half-elf/ability-bonus/0', repeatable: false },
    }
    renderAt(
      viewport,
      <PromptCard prompt={strict} entries={new Map()} pending={false} onAnswer={vi.fn()} />,
    )

    await user.click(await screen.findByRole('button', { name: /Dexterity/ }))
    // A second click takes the first one back rather than adding a point.
    await user.click(screen.getByRole('button', { name: /Dexterity/ }))
    expect(screen.getByRole('button', { name: /choose 2 more/i })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Dexterity \+2/ })).not.toBeInTheDocument()
  })

  // Spent points have to be recoverable without clearing the whole answer:
  // with both of them on one score, every other option is greyed out, so that
  // score is the only thing left to click and it had better do something.
  it('takes the points back off an option once they are all on it', async () => {
    const user = setupUser()
    renderAt(
      viewport,
      <PromptCard prompt={improvement} entries={new Map()} pending={false} onAnswer={vi.fn()} />,
    )

    await user.click(await screen.findByRole('button', { name: /Dexterity \+1/ }))
    await user.click(screen.getByRole('button', { name: /Dexterity \+1/ }))
    await user.click(screen.getByRole('button', { name: /Dexterity \+2/ }))

    // Back to nothing spent, and both options live again.
    expect(screen.getByRole('button', { name: /choose 2 more/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Strength \+1/ })).toBeEnabled()
    expect(screen.getByRole('button', { name: /Dexterity \+1/ })).toBeEnabled()
  })
})
