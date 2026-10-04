import { afterEach, expect, it, vi } from 'vitest'
import { loginDevelopmentAccount } from './development'
import { request } from './client'
import { developmentSession, saveDevelopmentSession } from './devSession'
import { signOut } from './auth'

afterEach(() => {
  window.sessionStorage.clear()
  vi.unstubAllGlobals()
})

it('keeps each tab identity through reloads and rotates copied selectors when switching', async () => {
  const fetch = vi.fn(async (_url: unknown, _options?: RequestInit) => new Response(JSON.stringify({ game_ids: ['training'] })))
  vi.stubGlobal('fetch', fetch)
  await loginDevelopmentAccount('master')
  const master = developmentSession()!
  expect(master).toMatch(/^[a-f0-9]{32}$/)
  expect(fetch.mock.calls[0]![1]!.headers).toMatchObject({ 'X-EasyDnD-Dev-Session': master })
  await loginDevelopmentAccount('player1')
  const player = developmentSession()!
  expect(player).not.toBe(master)
  await request('/auth/me')
  const lastHeaders = fetch.mock.calls.at(-1)![1]!.headers
  expect(lastHeaders).toMatchObject({ 'X-EasyDnD-Dev-Session': player })
  // Restore the other tab's storage, as a reload of that tab would do.
  saveDevelopmentSession(master)
  await request('/auth/me')
  expect(fetch.mock.calls.at(-1)![1]!.headers)
    .toMatchObject({ 'X-EasyDnD-Dev-Session': master })
})

it('preserves the current tab when a switch fails', async () => {
  saveDevelopmentSession('a'.repeat(32))
  vi.stubGlobal('fetch', vi.fn(async () => new Response('{}', { status: 403 })))
  await expect(loginDevelopmentAccount('player2')).rejects.toThrow()
  expect(developmentSession()).toBe('a'.repeat(32))
})

it('logs out the selected session without falling back to an older browser session', async () => {
  saveDevelopmentSession('b'.repeat(32))
  const fetch = vi.fn(async (_url: unknown, _options?: RequestInit) => new Response('{"signed_out":true}'))
  vi.stubGlobal('fetch', fetch)
  await signOut()
  expect(fetch.mock.calls[0]![1]!.headers).toMatchObject({ 'X-EasyDnD-Dev-Session': 'b'.repeat(32) })
  expect(developmentSession()).toBe('b'.repeat(32))
  await request('/auth/me')
  expect(fetch.mock.calls.at(-1)![1]!.headers).toMatchObject({ 'X-EasyDnD-Dev-Session': 'b'.repeat(32) })
})

it('does not establish a session that cannot survive navigation when storage is blocked', async () => {
  const fetch = vi.fn()
  vi.stubGlobal('fetch', fetch)
  const blocked = vi.spyOn(Object.getPrototypeOf(window.sessionStorage) as Storage, 'setItem').mockImplementation(() => { throw new Error('blocked') })
  try {
    await expect(loginDevelopmentAccount('master')).rejects.toThrow('blocked')
    expect(fetch).not.toHaveBeenCalled()
  } finally { blocked.mockRestore() }
})
