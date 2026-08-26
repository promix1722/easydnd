import { screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import type { Change, Prompt } from '@/lib/api'
import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'

import { blocksFor } from './blocks'
import type { Asking } from './blocks'
import type { SettledRow } from './settled'
import { StagePanel } from './StagePanel'

/**
 * The tab's list, without the screen around it.
 *
 * Every block here is a name, a form or a fact, and none of them is a prompt
 * with options -- which is deliberate: those fetch their catalogue entries on
 * mount, and this file has nothing to answer them with. What the options do
 * once they are drawn is `PromptCard`'s test, and what the screen posts is
 * `BuildScreen`'s.
 */

const CLASS_ROW: SettledRow = {
  seq: 4,
  stage: 'class',
  label: 'Class chosen',
  value: 'Rogue',
  level: 1,
  event: { seq: 4, type: 'class', ref: 'class:rogue', level: 1 },
}

const LEVEL_ROW: SettledRow = {
  seq: 5,
  stage: 'class',
  label: 'Level gained',
  value: 'Rogue',
  level: 2,
  event: { seq: 5, type: 'level', ref: 'class:rogue', level: 2 },
}

const NESTED_ROW: SettledRow = {
  seq: 3,
  stage: 'race',
  label: 'Half Elf · Ability Bonus',
  value: 'Dex, Con',
  event: {
    seq: 3,
    type: 'race',
    ref: 'race:half-elf',
    choices: [{ prompt: 'half-elf/ability-bonus/0', picks: ['dex', 'con'] }],
  },
}

const SKILLS: Prompt = {
  choice: {
    prompt: 'rogue/proficiency/0',
    choose: 2,
    kind: 'proficiency',
    from: { kind: 'explicit', options: [] },
  },
  source: 'class:rogue',
  group: 'class',
  level: 1,
  optional: false,
  advances: false,
  event: { type: 'class', ref: 'class:rogue', level: 1 },
  heldOnly: false,
}

const SCORES: Prompt = {
  choice: {
    prompt: 'character/abilities',
    choose: 6,
    kind: 'ability-scores',
    from: { kind: 'explicit' },
  },
  group: 'abilities',
  optional: false,
  advances: false,
  event: { type: 'change' },
  heldOnly: false,
}

/**
 * A written question, and nothing to pick between.
 *
 * The empty option set is the whole of what tells this apart from a menu, and
 * it is the server's own statement that there is nothing on offer.
 */
const TRAITS: Prompt = {
  choice: {
    prompt: 'character/personality-trait',
    choose: 1,
    kind: 'personality',
    from: { kind: 'explicit' },
  },
  group: 'personality',
  optional: true,
  advances: false,
  event: { type: 'change' },
  heldOnly: false,
}

const NAMES = new Map([['class:rogue', 'Rogue']])

function panel(
  settled: readonly SettledRow[],
  prompts: readonly Prompt[],
  over: {
    openKey?: string
    asking?: Asking | null
    onOpen?: (key: string | null) => void
    onNext?: () => void
    onAnswerChanges?: (asking: Asking, changes: Change[]) => void
    lines?: readonly string[]
  } = {},
) {
  return (
    <StagePanel
      blocks={blocksFor(settled, prompts)}
      openKey={over.openKey ?? null}
      onOpen={over.onOpen ?? vi.fn()}
      asking={over.asking ?? null}
      names={NAMES}
      onAnswerPicks={vi.fn()}
      onNameChange={vi.fn()}
      onAnswerName={vi.fn()}
      onAnswerChanges={over.onAnswerChanges ?? vi.fn()}
      pending={false}
      fields={[]}
      {...(over.onNext ? { onNext: over.onNext } : {})}
      {...(over.lines ? { lines: over.lines } : {})}
    />
  )
}


/**
 * One viewport, not two. Only `Columns`, `DataList`, `ModalSheet`,
 * `SectionDeck`, `TabDeck`, `SheetBody` and `RootShell` branch on width, and the suite runs without CSS, so a responsive
 * prop cannot move the DOM either -- nothing in this tree reaches any of them,
 * so a test at one width is a test of both. See docs/web.md.
 */
describe('StagePanel', () => {
  const viewport = 'desktop'

  it('draws what was decided and what is still asked as one list, in level order', () => {
    renderAt(viewport, panel([LEVEL_ROW, CLASS_ROW], [SKILLS]))

    // A decided choice and an open one are the same object at two moments, so
    // they are one list -- and it reads up the levels: took rogue at 1, still
    // owes two skills at 1, gained a level at 2.
    const blocks = screen.getAllByText(/Class chosen|Two to be proficient in|Level gained/)
    expect(blocks.map((each) => each.textContent)).toEqual([
      'Class chosen',
      'Two to be proficient in · from Rogue',
      'Level gained',
    ])
  })

  it('marks what is still wanted, and nothing that is settled', () => {
    renderAt(viewport, panel([CLASS_ROW], [SKILLS]))

    const wanted = screen.getByRole('button', { name: /Two to be proficient in/ })
    expect(wanted.closest('[data-highlighted="true"]')).not.toBeNull()
    const decided = screen.getByRole('button', { name: /Class chosen/ })
    expect(decided.closest('[data-highlighted="true"]')).toBeNull()
  })

  it('opens nothing of its own accord, and reports what was pressed', async () => {
    const user = setupUser()
    const onOpen = vi.fn()
    renderAt(viewport, panel([], [SCORES], { onOpen }))

    // The screen does not pick a question: it has no way of knowing which of
    // the open choices anybody came here to make.
    expect(screen.queryByLabelText('Strength')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /Six ability scores/ }))
    expect(onOpen).toHaveBeenCalledWith('open:character/abilities')
  })

  it('answers a question inside the block that asked it', async () => {
    renderAt(
      viewport,
      panel([], [SCORES], {
        openKey: 'open:character/abilities',
        asking: { prompt: SCORES, replaces: null },
      }),
    )

    // The form is in the block, not in a card somewhere below it, and the
    // block's own header is the only place the question is put.
    const block = screen
      .getByRole('button', { name: /Six ability scores/ })
      .closest('[data-highlighted]')
    expect(block).not.toBeNull()
    const strength = await screen.findByRole('button', { name: /Strength/ })
    expect(block?.contains(strength)).toBe(true)
  })

  it('says a question is being put again where it cannot be re-posed', async () => {
    renderAt(viewport, panel([NESTED_ROW], [], { openKey: 'settled:3' }))

    // No button and no explanation to read past: opening a decided block puts
    // its question again, and this one takes a moment longer to do it.
    expect(await screen.findByText(/Putting that question again/)).toBeInTheDocument()
    expect(screen.queryAllByRole('button', { name: /Drop it/ })).toHaveLength(0)
  })

  it('leaves a level already taken as a fact rather than a block that opens', () => {
    renderAt(viewport, panel([LEVEL_ROW], []))

    expect(screen.getByText('Level gained')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Level gained/ })).not.toBeInTheDocument()
  })

  it('offers the way on only once the tab has nothing left to answer', async () => {
    const user = setupUser()
    const onNext = vi.fn()
    const { rerender } = renderAt(viewport, panel([CLASS_ROW], [SKILLS], { onNext }))

    // Not while there is still something here to do: the tabs are always
    // there for going somewhere else, and this is the end of a piece of work
    // rather than another way to navigate.
    expect(screen.queryByRole('button', { name: 'Next' })).not.toBeInTheDocument()

    rerender(panel([CLASS_ROW], [], { onNext }))
    await user.click(screen.getByRole('button', { name: 'Next' }))

    // It names no category: that word belongs to the tab it is on.
    expect(onNext).toHaveBeenCalled()
  })

  it('names no category when there is nothing left, or nothing yet', () => {
    const { rerender } = renderAt(viewport, panel([CLASS_ROW], []))

    // Never "nothing left in class": the category's word lives in its tab and
    // appears exactly once on the page.
    expect(screen.getByText('Nothing left here.')).toBeInTheDocument()

    rerender(panel([], []))
    expect(screen.getByText('Nothing to answer yet.')).toBeInTheDocument()
  })
})

/**
 * A trait is written, not picked.
 *
 * The SRD prints eight of each and the compendium carries them, but a trait is
 * the one line on a sheet that is nobody's but the player's -- so the prompt
 * arrives with nothing to choose between and the surface is a field.
 */
describe('the questions answered in words', () => {
  const viewport = 'desktop'

  const written = () =>
    panel([], [TRAITS], {
      openKey: 'open:character/personality-trait',
      asking: { prompt: TRAITS, replaces: null },
    })

  it('offers a field rather than options, and writes what the sheet stores', async () => {
    const user = setupUser()
    const onAnswerChanges = vi.fn()
    renderAt(
      viewport,
      panel([], [TRAITS], {
        openKey: 'open:character/personality-trait',
        asking: { prompt: TRAITS, replaces: null },
        onAnswerChanges,
      }),
    )

    // A field, and no menu: the SRD's eight suggestions are in the compendium
    // to read, not to pick from.
    expect(screen.queryByRole('button', { name: /sacred texts/ })).not.toBeInTheDocument()
    const field = screen.getByLabelText('Personality trait')
    // More than one line, because these are sentences.
    expect(field.tagName).toBe('TEXTAREA')

    await user.type(field, 'I quote sacred texts.')
    await user.click(screen.getByRole('button', { name: 'Confirm' }))

    expect(onAnswerChanges.mock.calls[0]?.[1]).toEqual([
      {
        path: 'identity.personalityTraits',
        op: 'set',
        value: { kind: 'string', string: 'I quote sacred texts.' },
      },
    ])
  })

  it('will not answer with nothing', async () => {
    const user = setupUser()
    renderAt(viewport, written())

    // Nothing written is the same as not answering, and this is optional --
    // so there is nothing to confirm.
    expect(screen.getByRole('button', { name: 'Confirm' })).toBeDisabled()
    await user.type(screen.getByLabelText('Personality trait'), '   ')
    expect(screen.getByRole('button', { name: 'Confirm' })).toBeDisabled()
  })

  it('starts from what is already written when the answer is being changed', () => {
    renderAt(
      viewport,
      panel([], [TRAITS], {
        openKey: 'open:character/personality-trait',
        asking: { prompt: TRAITS, replaces: null },
        lines: ['I quote sacred texts.'],
      }),
    )

    expect(screen.getByLabelText('Personality trait')).toHaveValue('I quote sacred texts.')
  })
})
