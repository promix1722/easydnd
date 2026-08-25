import { screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import type { CatalogProficiency } from '@/lib/api'
import { bySlug } from '@/lib/api'
import { renderAt } from '@/test/render'

import { ProficienciesPanel } from './ProficienciesPanel'

/**
 * What these defend is the one judgement the panel makes: which rows get a
 * number. Everything else it draws is handed to it.
 */
const CATALOG: CatalogProficiency[] = [
  { slug: 'thieves-tools', name: "Thieves' Tools", type: 'artisans-tools' },
  { slug: 'disguise-kit', name: 'Disguise Kit', type: 'artisans-tools' },
  { slug: 'lute', name: 'Lute', type: 'musical-instruments' },
  { slug: 'land-vehicles', name: 'Land Vehicles', type: 'vehicles' },
  { slug: 'rapiers', name: 'Rapiers', type: 'weapons' },
  { slug: 'light-armor', name: 'Light Armor', type: 'armor' },
  { slug: 'something-new', name: 'Something New', type: 'wands' },
]

const HELD = [
  'light-armor',
  'rapiers',
  'thieves-tools',
  'disguise-kit',
  'lute',
  'land-vehicles',
  'something-new',
]

function render(
  held: string[] = HELD,
  catalog: Map<string, CatalogProficiency> | null = bySlug(CATALOG),
) {
  renderAt(
    'desktop',
    <ProficienciesPanel proficiencies={held} catalog={catalog} proficiencyBonus={2} />,
  )
}

/** One row's text: the name, and the bonus when the row carries one. */
function row(name: string): string {
  const label = screen.getByText(name)
  return label.closest('div')?.parentElement?.textContent ?? ''
}

describe('the proficiencies panel', () => {
  it('prints the proficiency bonus against a tool', () => {
    render()

    // A tool check's ability depends on what is being attempted, so the
    // proficiency bonus is the only part of the number fixed in advance --
    // which is exactly the part worth printing.
    expect(row("Thieves' Tools")).toBe("Thieves' Tools+2")
    expect(row('Lute')).toBe('Lute+2')
    expect(row('Land Vehicles')).toBe('Land Vehicles+2')
  })

  it('prints no number against a weapon or a suit of armor', () => {
    render()

    // A weapon's attack roll has a fixed ability, so a bare +2 would be the
    // less useful half of a number this panel is not showing; armor adds
    // nothing to any roll at all.
    expect(row('Rapiers')).toBe('Rapiers')
    expect(row('Light Armor')).toBe('Light Armor')
  })

  it('groups them, so a list is searchable rather than a sentence to read', () => {
    render()

    expect(
      screen.getAllByText(/^(?:Tools|Weapons|Armor|Other)$/).map((h) => h.textContent),
    ).toEqual(['Tools', 'Weapons', 'Armor', 'Other'])
  })

  it('keeps a type it does not recognise rather than dropping it', () => {
    render()

    // An unrecognised proficiency means the server and this client disagree
    // about the game, which is a thing to see rather than a thing to hide.
    expect(screen.getByText('Something New')).toBeInTheDocument()
  })

  it('draws only the groups the character has something in', () => {
    render(['rapiers'])

    expect(screen.getByText('Weapons')).toBeInTheDocument()
    expect(screen.queryByText('Tools')).not.toBeInTheDocument()
    expect(screen.queryByText('Armor')).not.toBeInTheDocument()
  })

  it('still lists them when the compendium could not be fetched', () => {
    render(HELD, null)

    // Without types there is no grouping and no bonus, but the character's
    // proficiencies are still theirs and the panel still draws them.
    expect(screen.getByText('Thieves Tools')).toBeInTheDocument()
    expect(row('Thieves Tools')).toBe('Thieves Tools')
    expect(screen.getByText('Other')).toBeInTheDocument()
  })

  it('says so when there are none', () => {
    render([])

    expect(screen.getByText('None yet.')).toBeInTheDocument()
  })
})
