import { MemoryRouter } from 'react-router'
import { fireEvent, screen, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import type { Sheet } from '@/lib/api'
import { renderAt as renderBare } from '@/test/render'
import { setupUser } from '@/test/user'

import { SheetBody } from './SheetBody'
import { jsonResponse } from '@/test/api'

// A row's menu links to the item's page, so the sheet is always inside a router.
const renderAt: typeof renderBare = (viewport, ui, ...rest) => renderBare(viewport, <MemoryRouter>{ui}</MemoryRouter>, ...rest)

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
const SECTIONS = ['Overview', 'Actions', 'Equipment', 'Items']

/** What the Equipment tab sorts and slots by. */
const ITEMS: Sheet = {
  ...SHEET,
  catalog: {
    skills: [],
    equipment: [
      { slug: 'leather-armor', name: 'Leather Armor', category: 'armor', slot: 'body', armor: { category: 'light', baseAC: 11 } },
      { slug: 'thieves-tools', name: "Thieves' Tools", category: 'tools' },
      { slug: 'crossbow-bolt', name: 'Crossbow Bolt', category: 'adventuring-gear', gear: { gearCategory: 'ammunition' } },
    ],
  },
}

/** A note the player wrote, beside a custom background that is not one. */
const NOTED: Sheet = {
  ...SHEET,
  customOptions: [
    { id: 'n1', kind: 'note', name: 'Backstory', description: 'Born at sea.\nRaised by gulls.', source: '', selected: true },
    { id: 'criminal', kind: 'background', name: 'Criminal', description: '', source: '', selected: true },
  ],
}

/** A spare armor in the backpack, so that a row has something to wear. */
const PACKED: Sheet = {
  ...ITEMS,
  equipment: { ...ITEMS.equipment, backpack: [...ITEMS.equipment.backpack, { item: 'chain-mail', count: 1 }] },
  catalog: {
    skills: [],
    equipment: [
      ...(ITEMS.catalog?.equipment ?? []),
      { slug: 'chain-mail', name: 'Chain Mail', category: 'armor', slot: 'body', armor: { category: 'heavy', baseAC: 16, strengthMinimum: 13, stealthDisadvantage: true } },
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

    // An item's menu is the one thing that opens; no section does.
    expect.soft([...document.querySelectorAll('[aria-expanded]')].filter((each) => !each.getAttribute('aria-label')?.startsWith('Actions for '))).toHaveLength(0)
    expect.soft(screen.getByTitle('Strength')).toBeInTheDocument()
    expect.soft(screen.getByText('Hit points')).toBeInTheDocument()
    expect.soft(skillRows()).toHaveLength(6)
  })

  // A sheet that is only being read offers one thing besides its tabs: each
  // item's menu, which holds its Details page and nothing that changes it.
  it('offers nothing to press but the tabs and each item\'s details', async () => {
    body('mobile')

    const buttons = screen.queryAllByRole('button')
    expect(buttons.length).toBeGreaterThan(0)
    expect(buttons.every((each) => each.getAttribute('aria-label')?.startsWith('Actions for '))).toBe(true)
    await setupUser().click(buttons[0]!)
    expect(within(await screen.findByRole('menu')).getAllByRole('menuitem').map((each) => each.textContent)).toEqual(['Details'])
  })

  // The same order as a wide screen: a sheet is read name first at every width.
  it('reads who the character is before the ability scores', () => {
    body('mobile')

    expect(leads()).toBe('who')
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

  it('opens a row onto its description, and leaves a row with none as a statement', async () => {
    renderAt('desktop', <SheetBody sheet={{
      ...SHEET,
      catalog: { skills: [], traits: [{ slug: 'darkvision', name: 'Darkvision', desc: ['You see in the dark.'] }] },
    }} />)

    expect.soft(screen.queryByRole('button', { name: 'Fey Ancestry' })).not.toBeInTheDocument()
    expect.soft(screen.queryByText('You see in the dark.')).not.toBeInTheDocument()
    await setupUser().click(screen.getByRole('button', { name: 'Darkvision' }))
    expect(screen.getByText('You see in the dark.')).toBeInTheDocument()
  })

  // The tab is in the URL, so a page opened from one comes back to it.
  it('opens on the tab the URL names', () => {
    renderBare('desktop', <MemoryRouter initialEntries={['/?tab=items']}><SheetBody sheet={ITEMS} /></MemoryRouter>)
    expect(screen.getByRole('tab', { name: 'Items' })).toHaveAttribute('aria-selected', 'true')
  })

  it('draws what is worn in its slot, and what is owned by group', () => {
    renderAt('mobile', <SheetBody sheet={ITEMS} />)

    const slots = screen.getByRole('region', { name: 'Worn and wielded' })
    expect.soft(within(slots).getByText('Leather Armor')).toBeInTheDocument()
    // Items is read top-down, nothing to switch: coins, consumables, gear.
    expect.soft(screen.getByText('Crossbow Bolt')).toBeInTheDocument()
    expect.soft(screen.getByText('×20')).toBeInTheDocument()
    expect.soft(screen.getByText("Thieves' Tools")).toBeInTheDocument()
    // One of a thing is the thing: no "×1".
    expect.soft(screen.queryByText('×1')).not.toBeInTheDocument()
  })

  // Custom is the player's own text: any number of titled items, written on
  // the sheet by its owner and only read by anybody else.
  it('adds, rewrites and deletes a custom item in place, each at the log\'s current revision', async () => {
    const writes: { method: string; path: string; search: string; body: unknown }[] = []
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = new URL(String(input), 'http://test')
      if ((init?.method ?? 'GET') === 'GET') return jsonResponse({ seq: 9, revision: 12, events: [] })
      writes.push({ method: init?.method ?? '', path: url.pathname, search: url.searchParams.get('revision') ?? '', body: JSON.parse(String(init?.body ?? 'null')) })
      return jsonResponse({ seq: 10, sheet: NOTED })
    }))
    const onChanged = vi.fn()
    const user = setupUser()
    renderAt('desktop', <SheetBody sheet={NOTED} characterId="chr_1" onChanged={onChanged} />)
    await user.click(screen.getByRole('tab', { name: 'Custom' }))
    const tab = within(screen.getByRole('tabpanel', { name: 'Custom' }))

    // What is there is read as it was typed, line breaks and all.
    expect(tab.getByRole('article', { name: 'Backstory' })).toHaveTextContent('Born at sea. Raised by gulls.')
    // A background the import kept is a custom entry too, and is not a note.
    expect(tab.queryByText('Criminal')).not.toBeInTheDocument()

    // Add turns into the form where it stood; no dialog opens.
    await user.click(tab.getByRole('button', { name: 'Add' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    await user.type(tab.getByRole('textbox', { name: 'Title' }), 'Debts')
    await user.type(tab.getByRole('textbox', { name: 'Text' }), '30 gp to the harbourmaster')
    await user.click(tab.getByRole('button', { name: 'Save' }))
    await vi.waitFor(() => expect(writes).toHaveLength(1))
    expect(writes[0]).toMatchObject({
      method: 'POST', path: '/v1/characters/chr_1/custom-options',
      body: { revision: 12, option: { kind: 'note', name: 'Debts', description: '30 gp to the harbourmaster', selected: true } },
    })
    await vi.waitFor(() => expect(onChanged).toHaveBeenCalledTimes(1))

    // Edit replaces the item with its form, and the write names the item.
    await user.click(tab.getByRole('button', { name: 'Edit Backstory' }))
    const title = tab.getByRole('textbox', { name: 'Title' })
    await user.clear(title)
    await user.type(title, 'Origins')
    await user.click(tab.getByRole('button', { name: 'Save' }))
    await vi.waitFor(() => expect(writes).toHaveLength(2))
    expect(writes[1]?.body).toMatchObject({ option: { id: 'n1', kind: 'note', name: 'Origins', description: 'Born at sea.\nRaised by gulls.' } })

    // Delete asks first, then names the item and the revision in the URL.
    await user.click(tab.getByRole('button', { name: 'Delete Backstory' }))
    const confirm = within(await screen.findByRole('dialog', { name: 'Delete Backstory?' }))
    expect(writes).toHaveLength(2)
    await user.click(confirm.getByRole('button', { name: 'Delete' }))
    await vi.waitFor(() => expect(writes).toHaveLength(3))
    expect(writes[2]).toMatchObject({ method: 'DELETE', path: '/v1/characters/chr_1/custom-options/n1', search: '12' })

    vi.unstubAllGlobals()
  })

  it('shows a reader the custom items and nothing to change them with, and no Custom tab when there are none', async () => {
    const user = setupUser()
    const { unmount } = renderAt('desktop', <SheetBody sheet={NOTED} />)
    await user.click(screen.getByRole('tab', { name: 'Custom' }))
    const tab = within(screen.getByRole('tabpanel', { name: 'Custom' }))
    expect(tab.getByText('Backstory')).toBeInTheDocument()
    expect(tab.queryAllByRole('button')).toHaveLength(0)
    unmount()

    renderAt('desktop', <SheetBody sheet={SHEET} />)
    expect(screen.queryByRole('tab', { name: 'Custom' })).not.toBeInTheDocument()
    // The owner has the tab even when it is empty: it is where the first one is written.
    renderAt('desktop', <SheetBody sheet={SHEET} characterId="chr_1" />)
    expect(screen.getByRole('tab', { name: 'Custom' })).toBeInTheDocument()
  })

  // Each tab adds from its own half of the catalogue: the search opens in
  // place under the list, and the server filters and pages it.
  it('adds from the catalogue: each tab asks for its half in place, a row opens to its description, and Add is one more in the backpack', async () => {
    const searches: URLSearchParams[] = []
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
      const url = new URL(String(input), 'http://test')
      const collection = url.pathname.split('/').at(-1)
      const offset = Number(url.searchParams.get('offset'))
      let body: unknown = []
      if (collection === 'items') {
        searches.push(url.searchParams)
        body = offset === 0
          ? {
              items: [
                { slug: 'crossbow-bolt', name: 'Crossbow Bolt', category: 'adventuring-gear', categoryName: 'Adventuring Gear', cost: { amount: 1, unit: 'gp' }, weight: 1.5 },
                { slug: 'potion-of-healing', name: 'Potion of Healing', category: 'potion', categoryName: 'Potion', magic: true },
              ],
              total: 3,
              categories: [{ slug: 'adventuring-gear', name: 'Adventuring Gear' }, { slug: 'potion', name: 'Potion' }],
            }
          : { items: [{ slug: 'rope', name: 'Rope' }], total: 3, categories: [] }
      } else if (collection === 'magic-items') {
        body = [{ slug: 'potion-of-healing', name: 'Potion of Healing', desc: ['You regain hit points when you drink this potion.'] }]
      }
      return jsonResponse(body)
    }))
    const onEquipment = vi.fn()
    const user = setupUser()
    renderAt('desktop', <SheetBody sheet={ITEMS} onEquipment={onEquipment} />)
    const last = () => searches.at(-1)

    // Nothing is fetched until a search is opened, and nothing opens a dialog.
    expect(searches).toHaveLength(0)
    await user.click(screen.getByRole('tab', { name: 'Equipment' }))
    await user.click(screen.getByRole('button', { name: 'Add equipment' }))
    await within(screen.getByRole('region', { name: 'Add equipment' })).findByText('3 items')
    expect(last()?.get('wearable')).toBe('true')
    expect(last()?.get('limit')).toBe('20')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()

    await user.click(screen.getByRole('tab', { name: 'Items' }))
    await user.click(screen.getByRole('button', { name: 'Add item' }))
    const search = within(screen.getByRole('region', { name: 'Add item' }))
    await search.findByText('Potion of Healing')
    expect(last()?.get('wearable')).toBe('false')
    // What a row shows came resolved: the category's name, a price, a weight,
    // and how many the character already has.
    expect.soft(search.getByText('Adventuring Gear · 1 gp · 1.5 lb.')).toBeInTheDocument()
    expect.soft(search.getByText('×20')).toBeInTheDocument()

    // Pressing a row opens the item's own description, asked for then.
    await user.click(search.getByRole('button', { name: 'Potion of Healing' }))
    expect(await search.findByText('You regain hit points when you drink this potion.')).toBeInTheDocument()

    // Owned twenty already, so one more is twenty-one; a new thing is one.
    await user.click(search.getByRole('button', { name: 'Add Crossbow Bolt' }))
    expect(onEquipment).toHaveBeenLastCalledWith([
      { path: 'equipment.backpack.crossbow-bolt', op: 'set', value: { kind: 'int', int: 21 } },
    ])
    await user.click(search.getByRole('button', { name: 'Add Potion of Healing' }))
    expect(onEquipment).toHaveBeenLastCalledWith([
      { path: 'equipment.backpack.potion-of-healing', op: 'set', value: { kind: 'int', int: 1 } },
    ])

    await user.click(search.getByRole('button', { name: 'Load more' }))
    await search.findByText('Rope')
    expect(last()?.get('offset')).toBe('2')
    expect(search.getByText('Crossbow Bolt')).toBeInTheDocument()

    await user.click(search.getByRole('combobox', { name: 'Mundane or magic' }))
    await user.click(await screen.findByRole('option', { name: 'Magic' }))
    await vi.waitFor(() => expect(last()?.get('magic')).toBe('true'))
    expect(last()?.get('offset')).toBe('0')

    // Close puts the button back where the search was.
    await user.click(search.getByRole('button', { name: 'Close' }))
    expect(screen.getByRole('button', { name: 'Add item' })).toBeInTheDocument()

    vi.unstubAllGlobals()
  })

  // Only the owner's screen passes `onEquipment`; without it the test above
  // -- and "offers nothing to press but the tabs" -- hold.
  it('takes off what a slot holds, from the menu on its card', async () => {
    const onEquipment = vi.fn()
    const user = setupUser()
    renderAt('mobile', <SheetBody sheet={ITEMS} onEquipment={onEquipment} />)

    const slots = within(screen.getByRole('region', { name: 'Worn and wielded' }))
    await user.click(slots.getByRole('button', { name: 'Actions for Leather Armor' }))
    await user.click(within(await screen.findByRole('menu')).getByRole('menuitem', { name: 'Take off Leather Armor' }))

    expect(onEquipment).toHaveBeenCalledWith([
      { path: 'equipment.equipped', op: 'set', value: { kind: 'slugs', slugs: [] } },
      { path: 'equipment.equipped.leather-armor', op: 'set', value: { kind: 'int', int: 0 } },
      { path: 'equipment.backpack.leather-armor', op: 'set', value: { kind: 'int', int: 1 } },
    ])
  })

  // A card is not a button: what is pressed on the doll is a menu, a worn
  // item's or an empty slot's. An empty slot's names what is carried that
  // fits it -- here the spare armor, which the taken body slot cannot have and
  // Custom can -- and always ends with the way to write a custom item into
  // it. What is worn is on its card only, so it has no row below. No row has
  // a stepper.
  it('presses nothing on the doll but menus, and lists only what is carried', async () => {
    const onEquipment = vi.fn()
    const user = setupUser()
    renderAt('mobile', <SheetBody sheet={PACKED} onEquipment={onEquipment} />)

    const slots = within(screen.getByRole('region', { name: 'Worn and wielded' }))
    // Nothing carried fits the head, and its menu still opens: onto the one entry.
    await user.click(slots.getByRole('button', { name: 'Actions for Head' }))
    const bare = within(await screen.findByRole('menu')).getAllByRole('menuitem')
    expect.soft(bare.map((each) => each.textContent)).toEqual(['Add custom item'])
    expect.soft(bare[0]).toHaveAttribute('href', '/custom-item?tab=equipment&slot=head')
    await user.keyboard('{Escape}')

    // Names, and pressing one puts it on.
    await user.click(slots.getByRole('button', { name: 'Actions for Custom' }))
    const offered = within(await screen.findByRole('menu')).getAllByRole('menuitem')
    expect.soft(offered.map((each) => each.textContent)).toEqual(['Chain Mail', 'Add custom item'])
    await user.click(offered[0] as HTMLElement)
    expect.soft(onEquipment).toHaveBeenCalledWith(expect.arrayContaining([
      { path: 'equipment.backpack.chain-mail', op: 'set', value: { kind: 'int', int: 0 } },
    ]))

    expect.soft(slots.getAllByText('Empty')).toHaveLength(11)
    expect.soft(screen.getAllByRole('button', { name: 'Actions for Leather Armor' })).toHaveLength(1)
    expect.soft(screen.getByRole('button', { name: 'Actions for Chain Mail' })).toBeInTheDocument()
    expect.soft(screen.queryByRole('button', { name: 'One more Crossbow Bolt' })).not.toBeInTheDocument()
    expect.soft(screen.getByRole('button', { name: 'Actions for Crossbow Bolt' })).toBeInTheDocument()
  })

  // A consumable is used one at a time and dropped one or all at a time; gear
  // is only dropped, and one of a thing has one Drop.
  it('uses and drops from the Items tab\'s menus', async () => {
    const onEquipment = vi.fn()
    const user = setupUser()
    renderAt('mobile', <SheetBody sheet={ITEMS} onEquipment={onEquipment} />)

    await user.click(screen.getByRole('button', { name: 'Actions for Crossbow Bolt' }))
    await user.click(within(await screen.findByRole('menu')).getByRole('menuitem', { name: 'Use' }))
    expect(onEquipment).toHaveBeenLastCalledWith([
      { path: 'equipment.backpack.crossbow-bolt', op: 'set', value: { kind: 'int', int: 19 } },
    ])
    await user.click(screen.getByRole('button', { name: 'Actions for Crossbow Bolt' }))
    await user.click(within(await screen.findByRole('menu')).getByRole('menuitem', { name: 'Drop all' }))
    expect(onEquipment).toHaveBeenLastCalledWith([
      { path: 'equipment.backpack.crossbow-bolt', op: 'set', value: { kind: 'int', int: 0 } },
    ])

    await user.click(screen.getByRole('button', { name: "Actions for Thieves' Tools" }))
    const menu = within(await screen.findByRole('menu'))
    expect(menu.queryByRole('menuitem', { name: 'Use' })).not.toBeInTheDocument()
    expect(menu.queryByRole('menuitem', { name: 'Drop one' })).not.toBeInTheDocument()
    expect(menu.getByRole('menuitem', { name: 'Drop' })).toBeInTheDocument()
  })

  it('wears from a row\'s menu, one entry per slot, and drops from the card\'s', async () => {
    const onEquipment = vi.fn()
    const user = setupUser()
    renderAt('mobile', <SheetBody sheet={PACKED} onEquipment={onEquipment} />)

    await user.click(screen.getByRole('button', { name: 'Actions for Chain Mail' }))
    const row = within(await screen.findByRole('menu'))
    expect(row.getAllByRole('menuitem').map((each) => each.textContent)).toEqual(['Details', 'Equip: Body', 'Equip: Custom', 'Drop'])
    await user.click(row.getByRole('menuitem', { name: 'Equip: Body' }))
    expect(onEquipment).toHaveBeenLastCalledWith([
      { path: 'equipment.equipped', op: 'set', value: { kind: 'slugs', slugs: ['chain-mail'] } },
      { path: 'equipment.equipped.leather-armor', op: 'set', value: { kind: 'int', int: 0 } },
      { path: 'equipment.equipped.chain-mail', op: 'set', value: { kind: 'int', int: 1 } },
      { path: 'equipment.backpack.chain-mail', op: 'set', value: { kind: 'int', int: 0 } },
      { path: 'equipment.backpack.leather-armor', op: 'set', value: { kind: 'int', int: 1 } },
    ])

    // Dropping what is worn takes it off without putting it in the backpack.
    await user.click(screen.getByRole('button', { name: 'Actions for Leather Armor' }))
    await user.click(within(await screen.findByRole('menu')).getByRole('menuitem', { name: 'Drop' }))
    expect(onEquipment).toHaveBeenLastCalledWith([
      { path: 'equipment.equipped', op: 'set', value: { kind: 'slugs', slugs: [] } },
      { path: 'equipment.equipped.leather-armor', op: 'set', value: { kind: 'int', int: 0 } },
    ])
  })

  it('puts a wearable in Custom from its row\'s menu and records the placement', async () => {
    const onEquipment = vi.fn()
    const user = setupUser()
    renderAt('mobile', <SheetBody sheet={PACKED} onEquipment={onEquipment} />)

    await user.click(screen.getByRole('button', { name: 'Actions for Chain Mail' }))
    await user.click(within(await screen.findByRole('menu')).getByRole('menuitem', { name: 'Equip: Custom' }))

    expect(onEquipment).toHaveBeenCalledWith(expect.arrayContaining([
      { path: 'equipment.custom', op: 'set', value: { kind: 'slugs', slugs: ['chain-mail'] } },
      { path: 'equipment.equipped.chain-mail', op: 'set', value: { kind: 'int', int: 1 } },
    ]))
  })

  // Everything a row has to say is on the row, read-only sheets included.
  it('prints an item\'s numbers on its row, with the catalogue\'s words', () => {
    renderAt('mobile', <SheetBody sheet={PACKED} />)

    // What could be worn is the card a worn thing is: each number over its caption.
    const row = within(screen.getByRole('button', { name: 'Actions for Chain Mail' }).closest('.mantine-Paper-root') as HTMLElement)
    expect.soft(row.getByText('Armor class').previousElementSibling).toHaveTextContent('16')
    expect.soft(row.getByText('Strength').previousElementSibling).toHaveTextContent('13')
    expect.soft(row.getByText('Disadvantage on Stealth checks')).toBeInTheDocument()
    // The row is not itself a control: the only button on it is its menu.
    expect(screen.queryAllByRole('button', { name: /Chain Mail/ }).map((each) => each.getAttribute('aria-label'))).toEqual(['Actions for Chain Mail'])
  })

  it('uses localized catalogue names and a localized class-resource label', () => {
    // A phone mounts every tab, so one render reaches all four.
    renderAt(
      'mobile',
      <SheetBody
        sheet={{
          ...NAMED,
          resources: { parameters: { 'sneak-attack': { name: 'Sneak Attack', number: 1, dice: '1d6' }, 'brutal-critical-dice': { name: 'Brutal Critical Dice', number: 0 }, 'extra-attacks': { name: 'Extra Attacks', number: 2 } } },
        }}
      />,
    )

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
    // Not reached yet is not a thing the character has.
    expect.soft(screen.queryByText(/Brutal Critical/)).not.toBeInTheDocument()
    // A plain number is counted off at the table, so it is marks to spend rather than a line to read.
    expect.soft(screen.getByRole('img', { name: 'Extra Attacks: 2 of 2 left' })).toBeInTheDocument()
    expect.soft(screen.queryByText('Extra Attacks: 2')).not.toBeInTheDocument()
    // A value that is read rather than counted is a feature's own line; there is no box of scaling values.
    expect.soft(under('Features')).toContain('Sneak Attack: 1d6')
    expect.soft(screen.queryByText('Scaling values')).not.toBeInTheDocument()
  })

  it('lists actions in groups that fold, each row opening onto its description', async () => {
    renderAt('desktop', <SheetBody sheet={{
      ...NAMED,
      actions: [
        { source: 'derived', origin: 'item:rapier', kind: 'action', category: 'equipment', name: 'Rapier', toHit: 5, damage: '1d8+3', range: 5 },
        { source: 'derived', origin: 'feature:second-wind', kind: 'bonus-action', category: 'feature', name: 'Second Wind', uses: 'second-wind' },
        { source: 'derived', origin: 'action:dash', kind: 'action', category: 'basic', name: 'Dash' },
      ],
      resources: { pools: { 'second-wind': { id: 'second-wind', name: 'Second Wind Uses', group: 'class', max: 1, used: 0 } } },
      catalog: { skills: [], actions: [{ slug: 'action:dash', name: 'Dash', desc: ['You gain extra movement.'] }] },
    }} />)
    fireEvent.click(screen.getByRole('tab', { name: 'Actions' }))

    // No search and no filters: the groups are the only control over the list.
    expect.soft(screen.queryByRole('textbox')).not.toBeInTheDocument()
    expect.soft(screen.queryByRole('button', { name: 'Bonus action' })).not.toBeInTheDocument()
    const group = (name: RegExp) => screen.getByRole('button', { name })

    // Every group starts folded: the tab opens as headings with counts.
    for (const name of [/^Equipment/, /^Class and race/, /^Basic/]) expect.soft(group(name)).toHaveAttribute('aria-expanded', 'false')
    expect.soft(screen.queryByText('Rapier')).not.toBeInTheDocument()
    expect.soft(screen.queryByText('Dash')).not.toBeInTheDocument()

    fireEvent.click(group(/^Equipment/))
    fireEvent.click(group(/^Class and race/))
    // A weapon's numbers are captioned columns in one order -- damage, hit,
    // range -- not a sentence.
    const rapier = (await screen.findByText('Rapier')).closest('.mantine-Accordion-item') as HTMLElement
    expect.soft(within(rapier).getByText('Damage').previousElementSibling).toHaveTextContent('1d8+3')
    expect.soft(within(rapier).getByText('Hit').previousElementSibling).toHaveTextContent('+5')
    expect.soft(within(rapier).getByText('Range').previousElementSibling).toHaveTextContent('5 ft.')
    expect.soft(within(rapier).getAllByText(/^(Damage|Hit|Range)$/).map((each) => each.textContent)).toEqual(['Damage', 'Hit', 'Range'])
    expect.soft(screen.getByText('Second Wind Uses: 1')).toBeInTheDocument()
    // A weapon with no prose is a fact, not a control that opens onto nothing.
    expect.soft(screen.queryByRole('button', { name: /Rapier/ })).not.toBeInTheDocument()

    fireEvent.click(group(/^Basic/))
    // The panel unfolds over a transition, and is hidden from roles until it has.
    fireEvent.click(await screen.findByRole('button', { name: /Dash/ }))
    expect.soft(screen.getByText('You gain extra movement.')).toBeInTheDocument()

    // Each group folds on its own.
    fireEvent.click(group(/^Class and race/))
    expect.soft(screen.queryByText('Second Wind')).not.toBeInTheDocument()
    expect.soft(screen.getByText('Rapier')).toBeInTheDocument()
    expect.soft(screen.getByText('Dash')).toBeInTheDocument()
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
    // Their own tab, not the foot of the action list.
    expect.soft(screen.getByRole('tab', { name: 'Resources' })).toBeInTheDocument()
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
