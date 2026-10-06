import { fireEvent, screen, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import type { Sheet } from '@/lib/api'
import { renderAt } from '@/test/render'

import { SheetBody } from './SheetBody'

/**
 * The sheet body's own tests, rendered from props.
 *
 * The panel rather than the page: `SheetBody` takes a projection and a
 * compendium, so there is no fetch, no router and nothing to wait for, and the
 * six sections it lays out are what is under test.
 * `CharacterSheetScreen.test.tsx` keeps the seam -- that the screen fetches all
 * three things and hands them here -- and is desktop-only for that reason.
 *
 * Both viewports here, because this is the one screen-level tree whose two
 * renderings differ, and it differs twice over: `ui/TabDeck` mounts only the
 * showing tab on a wide screen and every tab as a swiped deck on a phone, and
 * `SheetBody` itself orders the two halves of the overview's head by width.
 */

/** Enough of a character to fill all six sections, and no more. */
const SHEET: Sheet = {
  identity: {
    name: 'Zephyr',
    race: 'half-elf',
    classes: [{ class: 'rogue', level: 1 }],
    level: 1,
    background: 'acolyte',
    experience: 900,
  },
  base: {
    hitPoints: { current: 9, max: 9 },
    languages: ['common'],
    deathSaves: { successes: 0, failures: 0 },
  },
  abilities: {
    scores: { cha: 8, con: 14, dex: 16, int: 12, str: 18, wis: 10 },
    modifiers: { cha: -1, con: 2, dex: 3, int: 1, str: 4, wis: 0 },
  },
  // Three trained and three not, so a panel that dropped either kind would
  // come out at a number the other could not produce.
  skills: {
    acrobatics: { proficiency: 'none', bonus: 3 },
    arcana: { proficiency: 'none', bonus: 1 },
    deception: { proficiency: 'proficient', bonus: 1 },
    perception: { proficiency: 'proficient', bonus: 2 },
    stealth: { proficiency: 'expertise', bonus: 7 },
    survival: { proficiency: 'none', bonus: 0 },
  },
  savingThrows: {
    dex: { proficient: true, bonus: 5 },
    int: { proficient: true, bonus: 3 },
  },
  status: { armorClass: 15, initiative: 3, proficiencyBonus: 2, passivePerception: 12 },
  proficiencies: ['thieves-tools', 'light-armor'],
  traits: ['darkvision', 'fey-ancestry'],
  features: ['sneak-attack'],
  equipment: {
    equipped: [{ item: 'leather-armor', count: 1 }],
    backpack: [
      { item: 'thieves-tools', count: 1 },
      { item: 'crossbow-bolt', count: 20 },
    ],
    loot: [],
  },
  resources: {},
  spells: {},
  actions: [],
}

/**
 * SHEET itself resolves nothing, which falls every name back to a title-cased
 * slug. Distinct names here prove these sections read what the sheet resolved,
 * not titleCase.
 */
const NAMED: Sheet = {
  ...SHEET,
  catalogNames: {
    'traits:darkvision': 'Night Sight',
    'features:sneak-attack': 'Surprise Strike',
    'languages:common': 'Trade Tongue',
    'equipment:leather-armor': 'Hide Jerkin',
    'equipment:thieves-tools': 'Locksmith Kit',
    'equipment:crossbow-bolt': 'Quarrel',
  },
}

/**
 * The six sections, in the order the sheet decides things come in: who the
 * character is and what everything else is derived from, the body's state, then
 * what they are trained in and what they carry.
 */
// No Spells tab: this character casts nothing, and a tab with nothing under
// it is a question the sheet should not ask.
const SECTIONS = ['Overview', 'Actions', 'Equipment']

/** What the Equipment tab sorts and slots by. */
const ITEMS: Sheet = {
  ...SHEET,
  catalog: {
    skills: [],
    equipment: [
      { slug: 'leather-armor', name: 'Leather Armor', category: 'armor', armor: { category: 'light', baseAC: 11 } },
      { slug: 'thieves-tools', name: "Thieves' Tools", category: 'tools' },
      { slug: 'crossbow-bolt', name: 'Crossbow Bolt', category: 'adventuring-gear', gear: { gearCategory: 'ammunition' } },
    ],
  },
}

function body(viewport: 'mobile' | 'desktop') {
  return renderAt(viewport, <SheetBody sheet={SHEET} />)
}

/**
 * The skill rows, scoped to the skills section.
 *
 * The ability cards draw the same mark for their saving throws, so an unscoped
 * query would count those too.
 */
/**
 * Which of the two halves of the main section comes first in the document:
 * the identity table, or the ability cards.
 */
function leads(): 'who' | 'abilities' {
  const name = screen.getByText('Name')
  const strength = screen.getByTitle('Strength')
  const order = name.compareDocumentPosition(strength)
  return (order & Node.DOCUMENT_POSITION_FOLLOWING) !== 0 ? 'who' : 'abilities'
}

function skillRows(): HTMLElement[] {
  const panel = screen.getByText(/proficient|Nothing trained/).parentElement
  if (panel === null) throw new Error('the skills panel is not on the page')
  return within(panel).getAllByRole('img', { name: /proficien|Expertise/i })
}

describe('the sheet body on a phone', () => {
  it('is a deck of named tabs in the order the sheet reads', () => {
    body('mobile')

    // The first tab list is the sheet's; the Equipment tab holds one of its own.
    expect
      .soft(within(screen.getAllByRole('tablist')[0]!).getAllByRole('tab').map((tab) => tab.textContent))
      .toEqual(SECTIONS)
    for (const name of SECTIONS) {
      expect.soft(screen.getByRole('group', { name })).toBeInTheDocument()
    }
  })

  /**
   * The replacement for the accordion test this file inherited. Nothing on the
   * phone opens or closes any more: every section is drawn, and which one is on
   * screen is the carousel's business rather than a state a section is in.
   */
  it('leaves every section open', () => {
    body('mobile')

    expect.soft(document.querySelectorAll('[aria-expanded]')).toHaveLength(0)
    expect.soft(screen.getByTitle('Strength')).toBeInTheDocument()
    expect.soft(screen.getByText('Hit points')).toBeInTheDocument()
    expect.soft(skillRows()).toHaveLength(6)
  })

  // Pressing a tab is the only thing there is to press: the deck has no
  // controls of its own on the slides, and the panels no longer carry a filter.
  it('offers nothing to press but the tabs', () => {
    body('mobile')

    expect(screen.queryAllByRole('button')).toHaveLength(0)
  })

  /**
   * The one thing on this sheet whose *order* differs by width. A phone lands
   * on a slide, so it leads with the numbers reached for mid-turn; a wide
   * screen shows both at once and reads in the order a sheet is written in.
   *
   * Asserted on document position rather than on a style, because the swap is
   * a swap in the document -- doing it with `column-reverse` would leave this
   * assertion passing while the screen showed the other order.
   */
  it('leads with the ability scores, not with who the character is', () => {
    body('mobile')

    expect(leads()).toBe('abilities')
  })
})

/**
 * Both panels used to say their contents as a comma-joined sentence under a
 * label -- "Darkvision, Fey Ancestry" on one line -- which is a thing to read
 * rather than a thing to search. They are rows now, at both widths, which is
 * the shape `ProficienciesPanel` was given for the same reason.
 *
 * Asserted at one viewport, because the rows are the same markup either way:
 * what differs between the two is which container the panel sits in, and that
 * is `TabDeck`'s business and tested there.
 */
describe('the panels that were sentences', () => {
  /** The rows drawn under one uppercase group label. */
  function under(label: string): string[] {
    const heading = screen.getByText(label)
    const group = heading.parentElement
    if (group === null) throw new Error(`no group under ${label}`)
    return within(group)
      .getAllByText(/./)
      .map((node) => node.textContent ?? '')
      .filter((text) => text !== label)
  }

  it('draws traits, features and languages as rows', () => {
    body('desktop')

    expect.soft(under('Traits')).toEqual(['Darkvision', 'Fey Ancestry'])
    expect.soft(under('Features')).toEqual(['Sneak Attack'])
    expect.soft(under('Languages')).toEqual(['Common'])
  })

  it('draws what is worn in its slot, and what is owned by group', () => {
    renderAt('mobile', <SheetBody sheet={ITEMS} />)

    const slots = screen.getByRole('region', { name: 'Worn and wielded' })
    expect.soft(within(slots).getByText('Leather Armor')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('tab', { name: 'Consumables' }))
    expect.soft(screen.getByText('Crossbow Bolt')).toBeInTheDocument()
    expect.soft(screen.getByText('×20')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('tab', { name: 'Other gear' }))
    expect.soft(screen.getByText("Thieves' Tools")).toBeInTheDocument()
    // One of a thing is the thing: no "×1".
    expect.soft(screen.queryByText('×1')).not.toBeInTheDocument()
  })

  // Only the owner's screen passes `onEquipment`; without it the test above
  // -- and "offers nothing to press but the tabs" -- hold.
  it('takes off what a slot holds', () => {
    const onEquipment = vi.fn()
    renderAt('mobile', <SheetBody sheet={ITEMS} onEquipment={onEquipment} />)

    fireEvent.click(screen.getByRole('button', { name: 'Armor' }))
    fireEvent.click(screen.getByRole('button', { name: 'Take off Leather Armor' }))

    expect(onEquipment).toHaveBeenCalledWith([
      { path: 'equipment.equipped', op: 'set', value: { kind: 'slugs', slugs: [] } },
      { path: 'equipment.equipped.leather-armor', op: 'set', value: { kind: 'int', int: 0 } },
      { path: 'equipment.backpack.leather-armor', op: 'set', value: { kind: 'int', int: 1 } },
    ])
  })

  it('uses localized catalogue names and a localized class-resource label', () => {
    // A phone mounts every tab, so one render reaches all four.
    renderAt(
      'mobile',
      <SheetBody
        sheet={{
          ...NAMED,
          resources: { parameters: { 'sneak-attack': { name: 'Sneak Attack', number: 1, dice: '1d6' } } },
        }}
      />,
    )

    // No item details in this compendium, so nothing is known to be wearable.
    fireEvent.click(screen.getByRole('tab', { name: 'Other gear' }))
    for (const name of [
      'Night Sight',
      'Surprise Strike',
      'Trade Tongue',
      'Hide Jerkin',
      'Locksmith Kit',
      'Quarrel',
      'Sneak Attack: 1d6',
    ]) {
      expect.soft(screen.getAllByText(name).length).toBeGreaterThan(0)
    }
  })

  // Capacity only: what a sheet has spent is a fact about one game, not the character.
  it('draws consumables as marks, slots by level, and leaves Hit Dice to the vitals', () => {
    renderAt('mobile', <SheetBody sheet={{ ...NAMED, resources: { pools: {
      'spell-slots/2': { id: 'spell-slots/2', name: '', group: 'spell-slots', max: 2, used: 0, slotLevel: 2 },
      'spell-slots/1': { id: 'spell-slots/1', name: '', group: 'spell-slots', max: 4, used: 0, slotLevel: 1 },
      'channel-divinity': { id: 'channel-divinity', name: 'Channel Divinity Uses', group: 'class', max: 1, used: 0 },
      'hit-dice/cleric': { id: 'hit-dice/cleric', name: '', group: 'hit-dice', max: 3, used: 0, dice: '1d8' },
    } } }} />)

    expect.soft(screen.getByRole('heading', { name: 'Consumable slots' })).toBeInTheDocument()
    expect.soft(screen.getAllByRole('img', { name: /left$/ }).map((row) => row.getAttribute('aria-label'))).toEqual([
      'Spell slots, level 1: 4 of 4 left', 'Spell slots, level 2: 2 of 2 left', 'Channel Divinity Uses: 1 of 1 left',
    ])
    expect.soft(screen.queryByRole('button', { name: /Spend one/ })).not.toBeInTheDocument()
  })

  // A group with nothing in it still says so, because "nothing worn" is the
  // answer to the question and a missing group is not.
  it('says so when a group is empty', () => {
    renderAt('desktop', <SheetBody sheet={{ ...SHEET, traits: [] }} />)

    expect(screen.getByText('No racial traits.')).toBeInTheDocument()
  })
})

describe('the sheet body on a wide screen', () => {
  it('reads in the order a sheet is written in, who first', () => {
    body('desktop')

    expect(leads()).toBe('who')
  })

  it('draws the same tabs, and the whole overview under the first', () => {
    body('desktop')

    expect
      .soft(screen.getAllByRole('tab').map((tab) => tab.textContent))
      .toEqual(SECTIONS)
    expect.soft(screen.getByTitle('Strength')).toBeInTheDocument()
    expect.soft(screen.getByText('Hit points')).toBeInTheDocument()
    expect.soft(skillRows()).toHaveLength(6)
  })

  /**
   * The identity table, the ability cards and the vitals are drawn bare above
   * the panels; only the four panels are headed. A heading over the ability
   * cards would be the phone's tab label leaking onto a page that never had
   * one.
   */
  it('heads the panels and nothing above them', () => {
    body('desktop')

    for (const named of ['Skills', 'Proficiencies', 'Traits and features']) {
      expect.soft(screen.getByRole('heading', { name: named })).toBeInTheDocument()
    }
    for (const bare of ['Overview', 'Vitals']) {
      expect.soft(screen.queryByRole('heading', { name: bare })).not.toBeInTheDocument()
    }
  })
})
