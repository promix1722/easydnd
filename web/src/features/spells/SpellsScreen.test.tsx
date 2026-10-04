import { screen, waitFor } from '@testing-library/react'
import { MemoryRouter, useLocation } from 'react-router'
import { expect, it, vi } from 'vitest'

import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'
import { SpellsScreen } from './SpellsScreen'

function Location() {
  return <output data-testid="location">{useLocation().search}</output>
}

it('keeps shared filters in the browse URL and sends them to catalogue search', async () => {
  const searches: URLSearchParams[] = []
  vi.stubGlobal('fetch', vi.fn(async (input: string | URL | Request) => {
    const url = new URL(String(input), 'http://localhost')
    let data: unknown = []
    if (url.pathname === '/v1/packs/spell-filters') data = {
      packs: [{ id: 'library', title: 'My library', version: '1.0.0', versions: ['1.0.0'] }],
      sources: [{ id: 'library:phb', name: "Player's Handbook", packId: 'library' }],
      unavailable: [], schools: [{ slug: 'divination', name: 'Divination' }], classes: [{ slug: 'wizard', name: 'Wizard' }],
    }
    if (url.pathname.endsWith('/magic-schools')) data = [{ slug: 'divination', name: 'Divination' }]
    if (url.pathname.endsWith('/classes')) data = [{ slug: 'wizard', name: 'Wizard' }]
    if (url.pathname.endsWith('/spells')) {
      searches.push(url.searchParams)
      data = { total: 1, spells: [{ slug: 'detect-magic', name: 'Detect Magic', level: 1, school: 'divination', ritual: true }] }
    }
    return new Response(JSON.stringify(data), { headers: { 'Content-Type': 'application/json' } })
  }))
  const user = setupUser()
  renderAt('desktop', <MemoryRouter initialEntries={['/spells?school=divination&level=1']}><SpellsScreen /><Location /></MemoryRouter>)
  const search = await screen.findByRole('textbox', { name: 'Search spells' })
  expect(screen.getByRole('combobox', { name: 'School' })).toHaveValue('Divination')
  expect(screen.queryByRole('button', { name: 'Confirm' })).not.toBeInTheDocument()
  await user.click(screen.getByRole('combobox', { name: 'Pack' }))
  await user.click(screen.getByRole('option', { name: 'My library' }))
  await user.click(screen.getByRole('combobox', { name: 'Book / source' }))
  await user.click(screen.getByRole('option', { name: "PH / My library" }))
  await user.keyboard('{Escape}')
  await user.click(screen.getByRole('checkbox', { name: 'Ritual' }))
  await user.type(search, 'detect')
  await waitFor(() => {
    const last = searches.at(-1)
    expect(last?.get('q')).toBe('detect')
    expect(last?.get('ritual')).toBe('true')
    expect(last?.get('school')).toBe('divination')
    expect(last?.get('level')).toBe('1')
    expect(last?.get('pack')).toBe('library')
    expect(last?.get('source')).toBe('library:phb')
  })
  expect(screen.getByTestId('location')).toHaveTextContent('ritual=1')
  expect(screen.getByTestId('location')).toHaveTextContent('q=detect')
  expect(screen.getByTestId('location')).toHaveTextContent('pack=library')
  expect(screen.getByTestId('location')).toHaveTextContent('source=library%3Aphb')
  expect(screen.getByRole('textbox', { name: 'Search spells' })).toBe(search)
  expect(screen.getByRole('link', { name: 'Detect Magic' })).toBeInTheDocument()
})
