import { fireEvent, screen, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import type { Change } from '@/lib/api'
import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'

import { AbilityScoresForm } from './AbilityScoresForm'


function form(over: Partial<Parameters<typeof AbilityScoresForm>[0]> = {}) {
  return (
    <AbilityScoresForm pending={false} submitLabel="Confirm" onSubmit={vi.fn()} {...over} />
  )
}

/** What one ability was set to, out of the changes the form emits. */
function scored(changes: readonly Change[], ability: string): number | undefined {
  return changes.find((change) => change.path === `abilities.${ability}`)?.value.int
}

function method(changes: readonly Change[]): string | undefined {
  return changes.find((change) => change.path === 'abilities.method')?.value.slug
}

const chip = (value: string) => screen.getByRole('button', { name: value })
const slot = (ability: RegExp) => screen.getByRole('button', { name: ability })

async function place(user: ReturnType<typeof setupUser>, value: string, ability: RegExp) {
  await user.click(chip(value))
  await user.click(slot(ability))
}

/**
 * One viewport, not two. Only `Columns`, `DataList`, `ModalSheet`,
 * `SectionDeck`, `TabDeck`, `SheetBody` and `RootShell` branch on width, and the suite runs without CSS, so a responsive
 * prop cannot move the DOM either -- nothing in this tree reaches any of them,
 * so a test at one width is a test of both. See docs/web.md.
 */
describe('the standard array', () => {
  const viewport = 'desktop'

  it('deals out the printed numbers and will not be confirmed until all six are placed', async () => {
    const user = setupUser()
    const onSubmit = vi.fn()
    renderAt(viewport, form({ onSubmit }))

    // The array is printed, so none of these is a number to type: there is no
    // field for a score at all, only somewhere to put one.
    expect(screen.queryByLabelText('Strength')).not.toBeInTheDocument()
    for (const value of ['15', '14', '13', '12', '10', '8']) {
      expect(chip(value)).toBeInTheDocument()
    }
    expect(screen.getByRole('button', { name: 'Place all six' })).toBeDisabled()

    await place(user, '15', /Strength/)
    await place(user, '14', /Dexterity/)
    await place(user, '13', /Constitution/)
    await place(user, '12', /Intelligence/)
    await place(user, '10', /Wisdom/)
    await place(user, '8', /Charisma/)

    await user.click(screen.getByRole('button', { name: 'Confirm' }))

    const changes = onSubmit.mock.calls[0]?.[0] as Change[]
    expect(method(changes)).toBe('standard-array')
    expect(scored(changes, 'str')).toBe(15)
    expect(scored(changes, 'cha')).toBe(8)
  })

  it('swaps when a number is put where another one already is', async () => {
    const user = setupUser()
    renderAt(viewport, form())

    await place(user, '15', /Strength/)
    await place(user, '8', /Dexterity/)

    // Strength's 15 onto Dexterity: the two trade rather than the drop being
    // refused or Dexterity's 8 falling back into the pool.
    await user.click(slot(/Strength/))
    await user.click(slot(/Dexterity/))

    expect(slot(/Dexterity/)).toHaveAccessibleName(/15/)
    expect(slot(/Strength/)).toHaveAccessibleName(/8/)
  })

  /*
   * The drag, driven by hand.
   *
   * It is pointer events now rather than HTML5 drag-and-drop, which is what
   * makes it a gesture a finger can make -- see `ScoreAssignment`. Two things
   * jsdom lacks have to be stood in for: `elementFromPoint`, which needs
   * layout, and `setPointerCapture`, which the component already calls
   * optionally for this reason.
   *
   * The stub answers with whatever the drag is meant to be over, which is the
   * one fact the browser would have supplied. Everything else -- the threshold,
   * the swap, the click that must not undo the drop -- is the real code.
   */
  function dragTo(value: string, ability: RegExp) {
    const target = slot(ability)
    const from = chip(value)
    document.elementFromPoint = () => target

    fireEvent.pointerDown(from, { clientX: 0, clientY: 0, pointerId: 1 })
    fireEvent.pointerMove(from, { clientX: 40, clientY: 40, pointerId: 1 })
    fireEvent.pointerUp(from, { clientX: 40, clientY: 40, pointerId: 1 })
    // A real gesture ends in a click on whatever it started on; the component
    // has to swallow exactly this one, or the number is picked straight back up.
    fireEvent.click(from)
  }

  it('takes a number by dragging it as well as by tapping it', () => {
    renderAt(viewport, form())

    dragTo('15', /Wisdom/)

    expect(slot(/Wisdom/)).toHaveAccessibleName(/15/)
    expect(screen.queryByRole('button', { name: '15' })).not.toBeInTheDocument()
  })

  it('puts a number back in the pool when it is dragged off the grid', () => {
    renderAt(viewport, form())

    dragTo('15', /Strength/)
    expect(slot(/Strength/)).toHaveAccessibleName(/15/)

    // Let go over nothing: the hint text, the gap between two cards, the page.
    const from = slot(/Strength/)
    document.elementFromPoint = () => null
    fireEvent.pointerDown(from, { clientX: 0, clientY: 0, pointerId: 1 })
    fireEvent.pointerMove(from, { clientX: 0, clientY: 300, pointerId: 1 })
    fireEvent.pointerUp(from, { clientX: 0, clientY: 300, pointerId: 1 })
    fireEvent.click(from)

    expect(slot(/Strength/)).toHaveAccessibleName(/--/)
    expect(chip('15')).toBeInTheDocument()
  })

  it('leaves a press that does not travel to the tap gesture', () => {
    renderAt(viewport, form())

    const target = slot(/Wisdom/)
    document.elementFromPoint = () => target
    const from = chip('15')

    // Two pixels is a thumb on a button, not a drag. The drop is not taken,
    // and the click that follows is not swallowed -- it picks the number up.
    fireEvent.pointerDown(from, { clientX: 0, clientY: 0, pointerId: 1 })
    fireEvent.pointerMove(from, { clientX: 2, clientY: 0, pointerId: 1 })
    fireEvent.pointerUp(from, { clientX: 2, clientY: 0, pointerId: 1 })
    fireEvent.click(from)

    expect(slot(/Wisdom/)).toHaveAccessibleName(/--/)
    fireEvent.click(slot(/Wisdom/))
    expect(slot(/Wisdom/)).toHaveAccessibleName(/15/)
  })

  it('starts from the scores it is changing, all six already placed', async () => {
    const user = setupUser()
    const onSubmit = vi.fn()
    renderAt(
      viewport,
      form({
        scores: { str: 8, dex: 15, con: 14, int: 13, wis: 12, cha: 10 },
        method: 'standard-array',
        submitLabel: 'Change it',
        onSubmit,
      }),
    )

    expect(screen.getByText('All six placed.')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Change it' }))

    const changes = onSubmit.mock.calls[0]?.[0] as Change[]
    expect(scored(changes, 'dex')).toBe(15)
  })
})

/**
 * One viewport, not two. Only `Columns`, `DataList`, `ModalSheet`,
 * `SectionDeck`, `TabDeck`, `SheetBody` and `RootShell` branch on width, and the suite runs without CSS, so a responsive
 * prop cannot move the DOM either -- nothing in this tree reaches any of them,
 * so a test at one width is a test of both. See docs/web.md.
 */
describe('rolling', () => {
  const viewport = 'desktop'

  it('rolls the numbers rather than taking them, and rolls again on request', async () => {
    const user = setupUser()
    renderAt(viewport, form())

    await user.click(screen.getByRole('combobox', { name: /How were the scores generated/ }))
    await user.click(screen.getByRole('option', { name: 'Rolled' }))

    // Nothing here can be typed over: the dice decided the numbers, and what
    // is left to decide is where they go.
    expect(screen.queryByLabelText('Strength')).not.toBeInTheDocument()
    const before = pool()
    expect(before).toHaveLength(6)
    for (const value of before) {
      expect(value).toBeGreaterThanOrEqual(3)
      expect(value).toBeLessThanOrEqual(18)
    }

    await user.click(screen.getByRole('button', { name: 'Roll again' }))
    expect(pool()).toHaveLength(6)
  })
})

/** The numbers waiting to be placed, as the pool is drawing them. */
function pool(): number[] {
  const strip = screen.getByText('to place').parentElement
  if (strip === null) throw new Error('no pool on screen')
  return within(strip)
    .getAllByRole('button')
    .map((each) => Number(each.textContent))
    .filter((value) => !Number.isNaN(value))
}

/**
 * One viewport, not two. Only `Columns`, `DataList`, `ModalSheet`,
 * `SectionDeck`, `TabDeck`, `SheetBody` and `RootShell` branch on width, and the suite runs without CSS, so a responsive
 * prop cannot move the DOM either -- nothing in this tree reaches any of them,
 * so a test at one width is a test of both. See docs/web.md.
 */
describe('point buy', () => {
  const viewport = 'desktop'

  /**
   * Drives the method `Select` over to point buy.
   *
   * Only the first test does this. The rest pass `method="point-buy"` instead,
   * because the two are the same state and the `Select` is not what they are
   * about: `AbilityScoresForm` seeds `how` from the prop and its budget from
   * `boughtFrom(method, scores)`, which with no scores is the same six 8s
   * `change('point-buy')` sets. Two clicks and a `Combobox` dropdown per test
   * is a real cost across a file that already drives seventy interactions.
   */
  async function openPointBuy(user: ReturnType<typeof setupUser>) {
    await user.click(screen.getByRole('combobox', { name: /How were the scores generated/ }))
    await user.click(screen.getByRole('option', { name: 'Point buy' }))
  }

  it('starts every score at 8 with the whole budget unspent', async () => {
    const user = setupUser()
    renderAt(viewport, form())
    await openPointBuy(user)

    expect(screen.getByText('27 points left of 27')).toBeInTheDocument()
    // Nothing starts below 8, so nothing can be lowered from it either.
    expect(screen.getByRole('button', { name: 'Lower Strength' })).toBeDisabled()
  })

  it('charges two points for the fourteenth and two more for the fifteenth', async () => {
    const user = setupUser()
    renderAt(viewport, form({ method: 'point-buy' }))

    const raise = screen.getByRole('button', { name: 'Raise Strength' })
    for (let i = 0; i < 5; i += 1) await user.click(raise)
    expect(screen.getByText('22 points left of 27')).toBeInTheDocument()

    await user.click(raise)
    expect(screen.getByText('20 points left of 27')).toBeInTheDocument()

    await user.click(raise)
    expect(screen.getByText('18 points left of 27')).toBeInTheDocument()

    // 15 is where point buy stops, whatever is left in the budget.
    expect(raise).toBeDisabled()
  })

  it('will not sell what the budget cannot afford', async () => {
    // Handed the spread rather than clicking twenty-one times to reach it:
    // every one of those scores is buyable, so `boughtFrom` keeps them and the
    // budget is derived from them exactly as it would have been. What spending
    // costs is the test above; what it is like to have spent it all is this
    // one.
    renderAt(
      viewport,
      form({
        method: 'point-buy',
        scores: { str: 15, dex: 15, con: 15, int: 8, wis: 8, cha: 8 },
      }),
    )

    // Three 15s is 27 points exactly, and the fourth ability cannot move.
    expect(screen.getByText('0 points left of 27')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Raise Intelligence' })).toBeDisabled()
  })

  it('lets a spread that leaves points over be confirmed', async () => {
    const user = setupUser()
    const onSubmit = vi.fn()
    renderAt(viewport, form({ onSubmit, method: 'point-buy' }))

    await user.click(screen.getByRole('button', { name: 'Raise Wisdom' }))
    await user.click(screen.getByRole('button', { name: 'Confirm' }))

    const changes = onSubmit.mock.calls[0]?.[0] as Change[]
    expect(method(changes)).toBe('point-buy')
    expect(scored(changes, 'wis')).toBe(9)
    expect(scored(changes, 'str')).toBe(8)
  })
})

describe('manual', () => {
  async function openManual(user: ReturnType<typeof setupUser>) {
    await user.click(screen.getByRole('combobox', { name: /How were the scores generated/ }))
    await user.click(screen.getByRole('option', { name: 'Manual' }))
  }

  it('is the one method where a number is typed', async () => {
    const user = setupUser()
    const onSubmit = vi.fn()
    renderAt('desktop', form({ onSubmit }))
    await openManual(user)

    const strength = screen.getByLabelText('Strength')
    await user.clear(strength)
    await user.type(strength, '17')
    expect(strength).toHaveValue('17')
    await user.click(screen.getByRole('button', { name: 'Confirm' }))

    const changes = onSubmit.mock.calls[0]?.[0] as Change[]
    expect(method(changes)).toBe('manual')
    expect(scored(changes, 'str')).toBe(17)
  })

  it('starts at ten rather than at whatever the last method had not decided', async () => {
    const user = setupUser()
    const onSubmit = vi.fn()
    renderAt('desktop', form({ onSubmit }))

    // Straight from the standard array with nothing placed, which is where
    // this used to show six zeros -- under its own minimum -- and then save
    // six 10s, so the log and the screen disagreed about what was entered.
    await openManual(user)

    for (const ability of ['Strength', 'Dexterity', 'Constitution', 'Intelligence', 'Wisdom', 'Charisma']) {
      expect.soft(screen.getByLabelText(ability)).toHaveValue('10')
    }

    await user.click(screen.getByRole('button', { name: 'Confirm' }))
    const changes = onSubmit.mock.calls[0]?.[0] as Change[]
    expect(scored(changes, 'str')).toBe(10)
  })

  it('steps a score with the same buttons point buy uses, within 1 and 30', async () => {
    const user = setupUser()
    const onSubmit = vi.fn()
    renderAt('desktop', form({ onSubmit, method: 'manual', scores: allSixAt(30) }))

    // 30 is the top, so there is nothing to raise and one to lower.
    expect(screen.getByRole('button', { name: 'Raise Strength' })).toBeDisabled()
    await user.click(screen.getByRole('button', { name: 'Lower Strength' }))
    expect(screen.getByLabelText('Strength')).toHaveValue('29')

    await user.click(screen.getByRole('button', { name: 'Confirm' }))
    expect(scored(onSubmit.mock.calls[0]?.[0] as Change[], 'str')).toBe(29)
  })

  it('will not step below one', async () => {
    renderAt('desktop', form({ method: 'manual', scores: allSixAt(1) }))
    expect(screen.getByRole('button', { name: 'Lower Wisdom' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Raise Wisdom' })).toBeEnabled()
  })
})

function allSixAt(score: number): Record<string, number> {
  return { str: score, dex: score, con: score, int: score, wis: score, cha: score }
}
