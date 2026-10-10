import { screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, expect, it, vi } from 'vitest'

import { withAuth } from '@/test/auth'
import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'

import { CustomItemScreen } from './CustomItemScreen'

afterEach(() => vi.unstubAllGlobals())

interface Write { path: string; body: { option?: { id: string }; events?: unknown[] } & Record<string, unknown> }

/** The screen at `at`, over an API that answers every read and records every write. */
function open(at: string) {
  const writes: Write[] = []
  const json = (body: unknown, status = 200) => new Response(status === 204 ? null : JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = new URL(String(input), 'http://test').pathname
    if ((init?.method ?? 'GET') === 'GET') {
      if (path.includes('/catalog/')) return json([])
      if (path.endsWith('/sheet')) return json({ identity: { name: 'Ada' }, equipment: { equipped: [], backpack: [], loot: [] } })
      if (path.endsWith('/events')) return json({ seq: 9, revision: 12, events: [] })
      return json({ id: 'gam_1', name: 'Thursday night' })
    }
    const body = JSON.parse(String(init?.body ?? 'null')) as Write['body']
    writes.push({ path, body })
    if (path.endsWith('/custom-items')) return json(null, 204)
    return json({ seq: 10, revision: 13, sheet: { equipment: { equipped: [], backpack: [{ item: `custom-${body.option?.id ?? ''}`, count: 1 }], loot: [] } } })
  }))
  renderAt('desktop', withAuth({}, <MemoryRouter initialEntries={[at]}>
    <Routes>
      <Route path="/characters/:id/custom-item/:option?" element={<CustomItemScreen />} />
      <Route path="/games/:id/characters/:character/custom-item" element={<CustomItemScreen />} />
      <Route path="/characters/:id" element={<div>the sheet</div>} />
      <Route path="/games/:id" element={<div>the game</div>} />
    </Routes>
  </MemoryRouter>))
  return writes
}

it('writes the item on the owner\'s sheet, puts it in the slot it was opened from, and goes back', async () => {
  const user = setupUser()
  const writes = open('/characters/chr_1/custom-item?tab=equipment&slot=body')

  await user.type(await screen.findByRole('textbox', { name: /Name/ }), 'Glass Plate')
  await user.click(screen.getByRole('button', { name: 'Add custom item' }))

  expect(await screen.findByText('the sheet')).toBeInTheDocument()
  expect(writes).toHaveLength(2)
  expect.soft(writes[0]).toMatchObject({
    path: '/v1/characters/chr_1/custom-options',
    body: { revision: 12, option: { kind: 'item', name: 'Glass Plate', selected: true, placement: 'backpack', item: { slot: 'body' } } },
  })
  const slug = `custom-${writes[0]?.body.option?.id ?? ''}`
  expect.soft(writes[1]).toMatchObject({
    path: '/v1/characters/chr_1/events',
    body: { expectedSeq: 10, expectedRevision: 13, events: [{ type: 'change', changes: [
      { path: 'equipment.equipped', value: { slugs: [slug] } },
      { path: `equipment.equipped.${slug}` },
      { path: `equipment.backpack.${slug}` },
    ] }] },
  })
})

it('gives the item through the game, to the character the game names', async () => {
  const user = setupUser()
  const writes = open('/games/gam_1/characters/chr_1/custom-item?entry=pc_chr_1')

  await user.type(await screen.findByRole('textbox', { name: /Name/ }), 'Moon Shield')
  await user.click(screen.getByRole('button', { name: 'Give item' }))

  expect(await screen.findByText('the game')).toBeInTheDocument()
  expect(writes).toEqual([{ path: '/v1/games/gam_1/entries/pc_chr_1/custom-items', body: { name: 'Moon Shield', description: '', item: {} } }])
})
