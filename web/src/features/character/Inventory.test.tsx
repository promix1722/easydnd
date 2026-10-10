import { MemoryRouter } from 'react-router'
import { screen, within } from '@testing-library/react'
import { vi } from 'vitest'
import type { Equipment, Item } from '@/lib/api'
import { renderAt as renderBare } from '@/test/render'
import { setupUser } from '@/test/user'
import { SheetEquipment, SheetItems } from './SheetEquipment'

// A row's menu links to the item's page, so the sheet is always inside a router.
const renderAt: typeof renderBare = (viewport, ui, ...rest) => renderBare(viewport, <MemoryRouter>{ui}</MemoryRouter>, ...rest)

const icon = 'data:image/webp;base64,YXJ0'
const items = new Map<string, Item>([
  ['sword', { slug: 'sword', name: 'Sword', icon, slot: 'main-hand', desc: ['A **well-balanced** blade.'], weapon: { damage: { dice: '1d8', type: 'slashing' } } }],
  ['custom-keepsake', { slug: 'custom-keepsake', name: 'Keepsake' }],
  ['potion-of-healing', { slug: 'potion-of-healing', name: 'Healing potion', icon, category: 'potion' }],
])
const equipment: Equipment = {
  equipped: [{ item: 'sword', count: 1 }],
  backpack: [{ item: 'potion-of-healing', count: 2 }, { item: 'custom-keepsake', count: 1 }],
  loot: [],
}
const name = (slug: string) => items.get(slug)?.name ?? slug

it('shows 66px inventory artwork, keeps custom names, and still consumes an item', async () => {
  const user = setupUser()
  const onChange = vi.fn()
  renderAt('mobile', <SheetItems equipment={equipment} items={items} name={name} lookup={(_, slug) => slug} onChange={onChange} />)
  expect(screen.getByAltText('')).toHaveAttribute('width', '66')
  expect(screen.getByText('Keepsake')).toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: /Healing potion/ }))
  await user.click(screen.getByRole('menuitem', { name: /Use/ }))
  expect(onChange).toHaveBeenCalledOnce()
})

it('shows equipped item descriptions and facts beside a menu, on a card that is not a button', async () => {
  const user = setupUser()
  renderAt('desktop', <SheetEquipment equipment={equipment} items={items} name={name} lookup={(_, slug) => slug} onChange={vi.fn()} />)
  const slots = screen.getByRole('region', { name: 'Worn and wielded' })
  expect(within(slots).getByText('Sword')).toBeInTheDocument()
  expect(within(slots).getByText('well-balanced').tagName).toBe('STRONG')
  expect(slots).toHaveTextContent('1d8 slashing')
  expect(within(slots).getByAltText('')).toHaveAttribute('height', '66')
  expect(screen.queryByRole('button', { name: /Main hand/i })).not.toBeInTheDocument()
  await user.click(within(slots).getByRole('button', { name: 'Actions for Sword' }))
  expect(await screen.findByRole('menuitem', { name: 'Take off Sword' })).toBeInTheDocument()
})
