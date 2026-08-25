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
 * One viewport, not two. Only `Columns`, `DataList`, `ModalSheet` and
 * `RootShell` branch on width, and the suite runs without CSS, so a responsive
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

  it('takes a number by dragging it as well as by tapping it', async () => {
    renderAt(viewport, form())

    // The mouse gesture and the tap gesture are the same operation: this is
    // the one a phone cannot make.
    fireEvent.drop(slot(/Wisdom/), { dataTransfer: { getData: () => '0' } })

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
 * One viewport, not two. Only `Columns`, `DataList`, `ModalSheet` and
 * `RootShell` branch on width, and the suite runs without CSS, so a responsive
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
 * One viewport, not two. Only `Columns`, `DataList`, `ModalSheet` and
 * `RootShell` branch on width, and the suite runs without CSS, so a responsive
 * prop cannot move the DOM either -- nothing in this tree reaches any of them,
 * so a test at one width is a test of both. See docs/web.md.
 */
describe('point buy', () => {
  const viewport = 'desktop'

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
    renderAt(viewport, form())
    await openPointBuy(user)

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
    const user = setupUser()
    renderAt(viewport, form())
    await openPointBuy(user)

    for (const ability of ['Strength', 'Dexterity', 'Constitution']) {
      const raise = screen.getByRole('button', { name: `Raise ${ability}` })
      for (let i = 0; i < 7; i += 1) await user.click(raise)
    }

    // Three 15s is 27 points exactly, and the fourth ability cannot move.
    expect(screen.getByText('0 points left of 27')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Raise Intelligence' })).toBeDisabled()
  })

  it('lets a spread that leaves points over be confirmed', async () => {
    const user = setupUser()
    const onSubmit = vi.fn()
    renderAt(viewport, form({ onSubmit }))
    await openPointBuy(user)

    await user.click(screen.getByRole('button', { name: 'Raise Wisdom' }))
    await user.click(screen.getByRole('button', { name: 'Confirm' }))

    const changes = onSubmit.mock.calls[0]?.[0] as Change[]
    expect(method(changes)).toBe('point-buy')
    expect(scored(changes, 'wis')).toBe(9)
    expect(scored(changes, 'str')).toBe(8)
  })
})

describe('manual', () => {
  it('is the one method where a number is typed', async () => {
    const user = setupUser()
    const onSubmit = vi.fn()
    renderAt('desktop', form({ onSubmit }))

    await user.click(screen.getByRole('combobox', { name: /How were the scores generated/ }))
    await user.click(screen.getByRole('option', { name: 'Manual' }))

    const strength = screen.getByLabelText('Strength')
    await user.clear(strength)
    await user.type(strength, '17')
    expect(strength).toHaveValue('17')
    await user.click(screen.getByRole('button', { name: 'Confirm' }))

    const changes = onSubmit.mock.calls[0]?.[0] as Change[]
    expect(method(changes)).toBe('manual')
    expect(scored(changes, 'str')).toBe(17)
  })
})
