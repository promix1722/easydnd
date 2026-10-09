import { screen, within } from '@testing-library/react'
import { vi } from 'vitest'
import type { Equipment, Item } from '@/lib/api'
import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'
import { SheetEquipment, SheetItems } from './SheetEquipment'

const icon = 'data:image/webp;base64,YXJ0'
const items = new Map<string, Item>([
  ['sword', { slug: 'sword', name: 'Sword', icon, slot: 'main-hand', desc: ['A **well-balanced** blade.'], weapon: { damage: { dice: '1d8', type: 'slashing' } } }],
  ['potion-of-healing', { slug: 'potion-of-healing', name: 'Healing potion', icon, category: 'potion' }],
])
const equipment: Equipment = {
  equipped: [{ item: 'sword', count: 1 }],
  backpack: [{ item: 'potion-of-healing', count: 2 }, { count: 1, custom: { name: 'Keepsake' } }],
  loot: [],
}
const name = (slug: string) => items.get(slug)?.name ?? slug

it('shows 88px inventory artwork, keeps custom names, and still consumes an item', async () => {
  const user = setupUser()
  const onChange = vi.fn()
  renderAt('mobile', <SheetItems equipment={equipment} items={items} name={name} lookup={(_, slug) => slug} onChange={onChange} />)
  expect(screen.getByAltText('')).toHaveAttribute('width', '88')
  expect(screen.getByText('Keepsake')).toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: /Healing potion/ }))
  await user.click(screen.getByRole('menuitem', { name: /Use/ }))
  expect(onChange).toHaveBeenCalledOnce()
})

it('shows equipped item descriptions and facts and still opens slot controls', async () => {
  const user = setupUser()
  renderAt('desktop', <SheetEquipment equipment={equipment} items={items} name={name} lookup={(_, slug) => slug} onChange={vi.fn()} />)
  const slot = screen.getByRole('button', { name: /Main hand/i })
  expect(within(slot).getByText('Sword')).toBeInTheDocument()
  expect(within(slot).getByText('well-balanced').tagName).toBe('STRONG')
  expect(slot).toHaveTextContent('1d8 slashing')
  expect(within(slot).getByAltText('')).toHaveAttribute('height', '88')
  await user.click(slot)
  expect(screen.getByRole('dialog')).toHaveTextContent('Sword')
})
