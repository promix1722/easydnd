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
