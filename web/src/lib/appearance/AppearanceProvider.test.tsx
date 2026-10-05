import { fireEvent, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { MemoryRouter } from 'react-router'
import type { ReactElement } from 'react'

import type { AuthState } from '@/lib/auth/state'
import { testAccount, testGuest, withAuth } from '@/test/auth'
import { renderAt } from '@/test/render'
import { AppTheme } from '@/ui'
import { PALETTES } from '@/theme/palettes'
import { AccountScreen } from '@/features/account'
import { AppearanceProvider, type AppearanceClient } from './AppearanceProvider'
import { useAppearance } from './state'
import { accountAppearanceKey, BROWSER_APPEARANCE_KEY, DEFAULT_APPEARANCE, LAST_ACCOUNT_KEY, readAppearance, startupAppearance, type Appearance } from './preferences'

function withAppearance(state: Partial<AuthState>, ui: ReactElement, client?: AppearanceClient) {
  return withAuth(state, <AppearanceProvider {...(client ? { client } : {})}>{ui}</AppearanceProvider>)
}
function Probe() {
  const { appearance, change, loading, loadFailed, reload, saving, failed } = useAppearance()
  return <>
    <output aria-label="Preference">{appearance.palette}/{appearance.color_scheme}</output>
    <button disabled={loading || saving} onClick={() => { void change({ palette: 'moss', color_scheme: 'dark' }) }}>Change</button>
    {failed ? <span>Failed</span> : null}
    {loadFailed ? <button onClick={reload}>Retry read</button> : null}
  </>
}
function deferred() {
  let resolve!: (value: Response) => void
  const promise = new Promise<Response>((done) => { resolve = done })
  return { promise, resolve }
}
const response = (a: Appearance = DEFAULT_APPEARANCE) => new Response(JSON.stringify(a))
const ready = () => waitFor(() => expect(screen.getByRole('button', { name: 'Change' })).toBeEnabled())

describe('personal appearance resource', () => {
  it('remembers guest choices across mounts without an API request', () => {
    const fetch = vi.fn()
    vi.stubGlobal('fetch', fetch)
    const result = renderAt('desktop', withAppearance({ user: testGuest }, <Probe />))
    fireEvent.click(screen.getByRole('button', { name: 'Change' }))
    expect(screen.getByLabelText('Preference')).toHaveTextContent('moss/dark')
    expect(readAppearance(BROWSER_APPEARANCE_KEY)).toEqual({ palette: 'moss', color_scheme: 'dark' })
    expect(fetch).not.toHaveBeenCalled()
    result.unmount()
    renderAt('desktop', withAppearance({ user: testGuest }, <Probe />))
    expect(screen.getByLabelText('Preference')).toHaveTextContent('moss/dark')
  })

  it('GETs server settings, previews immediately and PUTs the whole resource', async () => {
    localStorage.setItem(BROWSER_APPEARANCE_KEY, JSON.stringify({ palette: 'moss', color_scheme: 'dark' }))
    const pending = deferred()
    const fetch = vi.fn<typeof globalThis.fetch>((_url, init) => init?.method === 'PUT'
      ? pending.promise : Promise.resolve(response({ palette: 'parchment', color_scheme: 'light' })))
    vi.stubGlobal('fetch', fetch)
    renderAt('desktop', withAppearance({}, <Probe />))
    expect(screen.getByRole('button', { name: 'Change' })).toBeDisabled()
    await ready()
    expect(screen.getByLabelText('Preference')).toHaveTextContent('parchment/light')
    fireEvent.click(screen.getByRole('button', { name: 'Change' }))
    expect(screen.getByLabelText('Preference')).toHaveTextContent('moss/dark')
    expect(screen.getByRole('button', { name: 'Change' })).toBeDisabled()
    expect(readAppearance(accountAppearanceKey(testAccount.id)).palette).toBe('parchment')
    pending.resolve(response({ palette: 'moss', color_scheme: 'dark' }))
    await ready()
    expect(readAppearance(accountAppearanceKey(testAccount.id)).palette).toBe('moss')
    expect(fetch.mock.calls.map(([url]) => url)).toEqual(['/v1/appearance?locale=en', '/v1/appearance?locale=en'])
    expect(fetch.mock.calls[0]?.[1]?.method).toBe('GET')
    expect(fetch.mock.calls[1]?.[1]?.method).toBe('PUT')
    expect(JSON.parse(String(fetch.mock.calls[1]?.[1]?.body))).toEqual({ palette: 'moss', color_scheme: 'dark' })
  })

  it('restores the previous theme on a failed PUT and allows retry', async () => {
    vi.stubGlobal('fetch', vi.fn<typeof globalThis.fetch>((_url, init) => Promise.resolve(init?.method === 'PUT'
      ? new Response('{}', { status: 500 }) : response())))
    renderAt('desktop', withAppearance({}, <Probe />))
    await ready()
    fireEvent.click(screen.getByRole('button', { name: 'Change' }))
    await screen.findByText('Failed')
    expect(screen.getByLabelText('Preference')).toHaveTextContent('dragon/auto')
    await ready()
  })

  it('keeps the confirmed cache when GET fails and supports retry', async () => {
    localStorage.setItem(accountAppearanceKey(testAccount.id), JSON.stringify({ palette: 'parchment', color_scheme: 'light' }))
    const fetch = vi.fn().mockResolvedValueOnce(new Response('{}', { status: 500 }))
      .mockResolvedValueOnce(response({ palette: 'midnight', color_scheme: 'dark' }))
    vi.stubGlobal('fetch', fetch)
    renderAt('desktop', withAppearance({}, <Probe />))
    fireEvent.click(await screen.findByRole('button', { name: 'Retry read' }))
    expect(screen.getByLabelText('Preference')).toHaveTextContent('parchment/light')
    await ready()
    expect(screen.getByLabelText('Preference')).toHaveTextContent('midnight/dark')
    expect(fetch).toHaveBeenCalledTimes(2)
  })

  it('discards a PUT after switching accounts or signing out', async () => {
    const pending = deferred()
    const reads = vi.fn().mockResolvedValueOnce(response()).mockResolvedValueOnce(response({ palette: 'midnight', color_scheme: 'light' }))
    vi.stubGlobal('fetch', vi.fn<typeof globalThis.fetch>((_url, init) => init?.method === 'PUT' ? pending.promise : reads()))
    const result = renderAt('desktop', withAppearance({}, <Probe />))
    await ready()
    fireEvent.click(screen.getByRole('button', { name: 'Change' }))
    result.rerender(withAppearance({ user: { ...testAccount, id: 'bob' } }, <Probe />))
    pending.resolve(response({ palette: 'moss', color_scheme: 'dark' }))
    await ready()
    expect(screen.getByLabelText('Preference')).toHaveTextContent('midnight/light')
    expect(readAppearance(accountAppearanceKey(testAccount.id)).palette).toBe('dragon')
    result.rerender(withAppearance({ user: null, status: 'anonymous' }, <Probe />))
    expect(screen.getByLabelText('Preference')).toHaveTextContent('dragon/auto')
    expect(localStorage.getItem(LAST_ACCOUNT_KEY)).toBeNull()
  })

  it('discards a late GET from a previous account', async () => {
    const old = deferred()
    vi.stubGlobal('fetch', vi.fn().mockReturnValueOnce(old.promise).mockResolvedValueOnce(response({ palette: 'moss', color_scheme: 'light' })))
    const result = renderAt('desktop', withAppearance({}, <Probe />))
    result.rerender(withAppearance({ user: { ...testAccount, id: 'bob' } }, <Probe />))
    await ready()
    old.resolve(response({ palette: 'midnight', color_scheme: 'dark' }))
    await waitFor(() => expect(screen.getByLabelText('Preference')).toHaveTextContent('moss/light'))
    expect(localStorage.getItem(accountAppearanceKey(testAccount.id))).toBeNull()
  })

  it('restores confirmed account appearance during startup and offline, then browser appearance for a guest', () => {
    localStorage.setItem(LAST_ACCOUNT_KEY, testAccount.id)
    localStorage.setItem(accountAppearanceKey(testAccount.id), JSON.stringify({ palette: 'midnight', color_scheme: 'dark' }))
    localStorage.setItem(BROWSER_APPEARANCE_KEY, JSON.stringify({ palette: 'parchment', color_scheme: 'light' }))
    const result = renderAt('desktop', withAppearance({ status: 'loading', user: null }, <Probe />))
    expect(screen.getByLabelText('Preference')).toHaveTextContent('midnight/dark')
    result.rerender(withAppearance({ status: 'offline', user: null }, <Probe />))
    expect(screen.getByLabelText('Preference')).toHaveTextContent('midnight/dark')
    result.rerender(withAppearance({ user: testGuest }, <Probe />))
    expect(screen.getByLabelText('Preference')).toHaveTextContent('parchment/light')
  })

  it('GETs updated preferences when the session is refreshed', async () => {
    const fetch = vi.fn().mockResolvedValueOnce(response()).mockResolvedValueOnce(response({ palette: 'midnight', color_scheme: 'dark' }))
    vi.stubGlobal('fetch', fetch)
    const result = renderAt('desktop', withAppearance({}, <Probe />))
    await ready()
    result.rerender(withAppearance({ user: { ...testAccount } }, <Probe />))
    await ready()
    expect(screen.getByLabelText('Preference')).toHaveTextContent('midnight/dark')
    expect(fetch).toHaveBeenCalledTimes(2)
  })

  it('validates corrupt caches and tolerates unavailable storage', () => {
    localStorage.setItem(BROWSER_APPEARANCE_KEY, '{')
    expect(readAppearance(BROWSER_APPEARANCE_KEY).palette).toBe('dragon')
    localStorage.setItem(BROWSER_APPEARANCE_KEY, JSON.stringify({ palette: 'toString', color_scheme: 'dark' }))
    expect(readAppearance(BROWSER_APPEARANCE_KEY).palette).toBe('dragon')
    vi.stubGlobal('localStorage', { getItem: () => { throw new Error('unavailable') }, setItem: () => { throw new Error('unavailable') }, removeItem: () => {} })
    expect(startupAppearance().color_scheme).toBe('auto')
    renderAt('desktop', withAppearance({ user: testGuest }, <Probe />))
    fireEvent.click(screen.getByRole('button', { name: 'Change' }))
    expect(screen.getByLabelText('Preference')).toHaveTextContent('moss/dark')
  })

  for (const viewport of ['mobile', 'desktop'] as const) {
    it(`offers labeled palette and mode controls on ${viewport}`, async () => {
      renderAt(viewport, <MemoryRouter initialEntries={['/account']}>{withAppearance({ user: testGuest }, <AccountScreen />)}</MemoryRouter>)
      fireEvent.click(screen.getByRole('combobox', { name: 'Color theme' }))
      fireEvent.click(await screen.findByRole('option', { name: 'Midnight' }))
      expect(screen.getByRole('combobox', { name: 'Color theme' })).toHaveValue('Midnight')
      fireEvent.click(screen.getByRole('combobox', { name: 'Display mode' }))
      fireEvent.click(await screen.findByRole('option', { name: 'Dark' }))
      expect(screen.getByRole('combobox', { name: 'Display mode' })).toHaveValue('Dark')
    })
  }

  it('applies each palette and mode without remounting content', async () => {
    const meta = document.createElement('meta')
    meta.name = 'theme-color'
    document.head.append(meta)
    let server = DEFAULT_APPEARANCE
    const client: AppearanceClient = { get: async () => server, put: async (next) => next }
    const tree = () => withAppearance({ user: { ...testAccount } }, <AppTheme env="test"><input aria-label="Draft" /></AppTheme>, client)
    const result = renderAt('desktop', tree())
    fireEvent.change(screen.getByLabelText('Draft'), { target: { value: 'Keep this' } })
    for (const palette of ['dragon', 'parchment', 'midnight', 'moss'] as const) {
      for (const scheme of ['auto', 'light', 'dark'] as const) {
        server = { palette, color_scheme: scheme }
        result.rerender(tree())
        await waitFor(() => {
          expect(document.documentElement.dataset.mantineColorScheme).toBe(scheme === 'auto' ? 'light' : scheme)
          expect(meta.content).toBe(PALETTES[palette].brand)
        })
        expect(screen.getByLabelText('Draft')).toHaveValue('Keep this')
      }
    }
    meta.remove()
  })
})
