import { screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import type { Identity } from '@/lib/api'
import { renderAt } from '@/test/render'

import { IdentityTable } from './IdentityTable'

const NAMES = new Map<string, string>([
  ['races:half-elf', 'Half-Elf'],
  ['subraces:high-elf', 'High Elf'],
  ['classes:rogue', 'Rogue'],
  ['subclasses:thief', 'Thief'],
  ['backgrounds:acolyte', 'Acolyte'],
])

const ZEPHYR: Identity = {
  name: 'Zephyr',
  race: 'half-elf',
  background: 'acolyte',
  classes: [{ class: 'rogue', level: 3, subclass: 'thief' }],
  level: 3,
  experience: 900,
}

function render(identity: Identity = ZEPHYR, names: Map<string, string> | null = NAMES) {
  renderAt('desktop', <IdentityTable identity={identity} names={names} />)
}

/** The value printed under one label. */
function value(label: string): string {
  return screen.getByText(label).parentElement?.textContent?.replace(label, '') ?? ''
}

/** The column one field sits in: its own box, and the box holding that. */
function column(label: string): Element | null {
  return screen.getByText(label).parentElement?.parentElement ?? null
}

describe('the identity table', () => {
  it('answers each field under its own label', () => {
    render()

    expect(value('Name')).toBe('Zephyr')
    expect(value('Race')).toBe('Half-Elf')
    expect(value('Level')).toBe('3')
    expect(value('Class')).toBe('Rogue 3')
    expect(value('Subclass')).toBe('Thief')
    expect(value('Background')).toBe('Acolyte')
    expect(value('Experience')).toBe('900')
  })

  /**
   * The pairing is the layout. A subrace is a qualification of a race and
   * nothing on its own, so it sits under it -- at four columns and at two,
   * which is what a flat grid of eight could not promise: there, two columns
   * put "Class" under "Name" and "Subrace" under "Class".
   */
  it('puts each qualifier under the field it qualifies', () => {
    render()

    const pairs: [string, string][] = [
      ['Name', 'Level'],
      ['Race', 'Subrace'],
      ['Class', 'Subclass'],
      ['Background', 'Experience'],
    ]
    for (const [head, qualifier] of pairs) {
      expect.soft(column(head)).toBe(column(qualifier))
    }

    // And four columns rather than one long one.
    expect.soft(new Set(pairs.map(([head]) => column(head))).size).toBe(4)
  })

  /**
   * The card is an alignment fix, not decoration: it is what puts the table's
   * labels on the same left edge as the ability cards' and the vitals'. Pinned
   * because the alternative -- padding the table by hand -- looks identical
   * until the card's padding changes, and then only this one has drifted.
   */
  it('sits in the same card the sheet draws its numbers in', () => {
    render()

    const card = screen.getByText('Name').closest('.mantine-Card-root')
    expect.soft(card).not.toBeNull()
    expect.soft(card).toContainElement(screen.getByText('Experience'))
  })

  it('draws a field the character has not answered, rather than closing the gap', () => {
    render()

    // "Not chosen yet" is the answer to "what subrace?", and a missing row is
    // not. A reader must be able to see what is still blank.
    expect(value('Subrace')).toBe('--')
  })

  it('names things the way the compendium does', () => {
    render()

    // titleCase would render the slug "half-elf" as "Half Elf".
    expect(screen.getByText('Half-Elf')).toBeInTheDocument()
    expect(screen.queryByText('Half Elf')).not.toBeInTheDocument()
  })

  it('falls back to the slug when the compendium could not be fetched', () => {
    render(ZEPHYR, null)

    // Worse names, still a table. A second request failing is not a reason to
    // refuse to say who the character is.
    expect(value('Race')).toBe('Half Elf')
    expect(value('Class')).toBe('Rogue 3')
    expect(value('Name')).toBe('Zephyr')
  })

  it('spells out a multiclassed character rather than truncating it', () => {
    render({
      ...ZEPHYR,
      level: 5,
      classes: [
        { class: 'rogue', level: 3, subclass: 'thief' },
        { class: 'wizard', level: 2 },
      ],
    })

    expect(value('Class')).toBe('Rogue 3 · Wizard 2')
    // Only one of the two has an archetype yet, and the row says so by
    // naming the one that does rather than padding the other with a dash.
    expect(value('Subclass')).toBe('Thief')
    expect(value('Level')).toBe('5')
  })

  it('says a character with nothing chosen has nothing chosen', () => {
    render({ name: '', level: 0, experience: 0 })

    expect(value('Name')).toBe('--')
    expect(value('Race')).toBe('--')
    expect(value('Class')).toBe('--')
    expect(value('Background')).toBe('--')
    // Nought experience is a real answer, not an absence.
    expect(value('Experience')).toBe('0')
  })
})
