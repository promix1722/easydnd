import { screen, waitFor, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'

import { DevAccounts } from './DevAccounts'

afterEach(() => vi.unstubAllGlobals())

describe.each(['mobile', 'desktop'] as const)('development accounts (%s)', (viewport) => {
  it('offers three local sign-ins and retains the chooser after a failed request', async () => {
    const fetch = vi.fn(async (_url: unknown, _options?: RequestInit) =>
      new Response(JSON.stringify({ error: { code: 'access_denied' } }), { status: 403 }))
    vi.stubGlobal('fetch', fetch)
    renderAt(viewport, <DevAccounts />)
    for (const name of ['master', 'player1', 'player2']) {
      expect(screen.getByRole('button', { name })).toBeInTheDocument()
    }
    await setupUser().click(screen.getByRole('button', { name: 'player2' }))
    await screen.findByRole('alert')
    expect(String(fetch.mock.calls[0]![0])).toContain('/dev/login')
    expect(JSON.parse(fetch.mock.calls[0]![1]!.body as string)).toEqual({ account: 'player2' })
    expect(screen.getByRole('button', { name: 'master' })).toBeEnabled()
  })

  it('names the current identity and keeps a failed switch available in the header menu', async () => {
    const fetch = vi.fn(async () => new Response(JSON.stringify({ error: { code: 'access_denied' } }), { status: 403 }))
    vi.stubGlobal('fetch', fetch)
    renderAt(viewport, <DevAccounts compact currentName="player1" />)
    const user = setupUser()
    await user.click(screen.getByRole('button', { name: 'Switch development account (player1)' }))
    const menu = within(await screen.findByRole('menu'))
    expect(menu.getAllByRole('menuitem')).toHaveLength(3)
    await user.click(menu.getByRole('menuitem', { name: 'master' }))
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(menu.getByRole('menuitem', { name: 'master' })).toBeEnabled())
    expect(screen.getByRole('menu')).toBeInTheDocument()
  })
})
