import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryRouter } from 'react-router'
import type { CaptureResult, PostHogConfig } from 'posthog-js'

let analytics: typeof import('./analytics')
let options: Partial<PostHogConfig>
let events: CaptureResult[]
let distinctID: string
let identifiedID: string | undefined

beforeEach(async () => {
  vi.resetModules()
  sessionStorage.clear()
  analytics = await import('./analytics')
  const { default: posthog } = await import('posthog-js')
  events = []
  distinctID = 'anonymous-browser'
  identifiedID = undefined
  vi.spyOn(posthog, 'init').mockImplementation((_token, supplied) => {
    options = supplied!
    supplied!.loaded!(posthog)
    return posthog
  })
  vi.spyOn(posthog, 'get_distinct_id').mockImplementation(() => distinctID)
  vi.spyOn(posthog, 'get_property').mockImplementation(() => identifiedID)
  vi.spyOn(posthog, 'identify').mockImplementation((id) => { distinctID = id!; identifiedID = id })
  vi.spyOn(posthog, 'reset').mockImplementation(() => { distinctID = 'fresh-anonymous'; identifiedID = undefined })
  vi.spyOn(posthog, 'capture').mockImplementation((event) => {
    const payload: CaptureResult = {
      uuid: 'test', event,
      properties: {
        distinct_id: distinctID, $browser: 'Firefox',
        $current_url: 'https://example.com/groups/join?token=secret',
        $referrer: 'https://example.com/private?token=secret',
        $initial_current_url: 'https://example.com/private',
        email: 'private@example.com',
        $set: { name: 'Private' },
      },
      $set_once: { $initial_current_url: 'https://example.com/?token=secret' },
    }
    const scrub = options.before_send
    const safe = typeof scrub === 'function' ? scrub(payload) : payload
    if (safe) events.push(safe)
    return safe ?? undefined
  })
  vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({
    environment: 'development', token: 'phc_test', host: 'https://eu.i.posthog.com',
  }))))
})

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  sessionStorage.clear()
})

function routerAt(path = '/characters/private-id?secret=hidden') {
  return createMemoryRouter([
    { path: '/characters/:id' }, { path: '/groups/join' }, { path: '*' },
  ], { initialEntries: [path] })
}

describe('analytics', () => {
  it('initializes once, tags preview traffic from the API, and sanitizes enriched SDK payloads', async () => {
    const router = routerAt()
    analytics.setAnalyticsUser({ id: 'account', anonymous: false })
    await Promise.all([analytics.startAnalytics(router), analytics.startAnalytics(router)])
    expect(fetch).toHaveBeenCalledTimes(1)
    expect(events).toHaveLength(1)
    expect(events[0]?.properties).toEqual({
      distinct_id: 'development:account', $browser: 'Firefox',
      environment: 'development', app_version: 'dev', account_type: 'registered',
      $current_url: window.location.origin + '/characters/:id', $pathname: '/characters/:id',
    })
    expect(events[0]?.$set_once).toBeUndefined()
    expect(JSON.stringify(events)).not.toMatch(/private|secret|email/)
    expect(options).toMatchObject({ autocapture: false, disable_session_recording: true, capture_pageview: false })
    analytics.setAnalyticsUser({ id: 'account', anonymous: false })
    expect(events).toHaveLength(1)
    await router.navigate('/characters/private-id?filter=changed')
    expect(events).toHaveLength(1)
    await router.navigate('/groups/join?token=private')
    expect(events).toHaveLength(2)
    expect(events[1]?.properties.$pathname).toBe('/groups/join')
    await router.navigate('/unknown-private-path')
    expect(events[2]?.properties.$pathname).toBe('/*')
    router.dispose()
  })

  it('waits for auth, resets accounts and guests, and retains tags after logout', async () => {
    const router = routerAt()
    await analytics.startAnalytics(router)
    expect(events).toHaveLength(0)
    analytics.setAnalyticsUser({ id: 'one', anonymous: false })
    expect(distinctID).toBe('development:one')
    analytics.setAnalyticsUser({ id: 'two', anonymous: false })
    expect(distinctID).toBe('development:two')
    analytics.setAnalyticsUser({ id: 'guest-private', anonymous: true })
    expect(distinctID).toBe('fresh-anonymous')
    analytics.track('character_created')
    expect(events.at(-1)?.properties.account_type).toBe('guest')
    analytics.setAnalyticsUser(null)
    analytics.track('game_created')
    expect(events.at(-1)?.properties).toMatchObject({ environment: 'development', app_version: 'dev', account_type: 'visitor' })
    expect(JSON.stringify(events)).not.toContain('guest-private')
    router.dispose()
  })

  it('records a successful SSO return once even when config loads after auth', async () => {
    const router = routerAt()
    analytics.beginAnalyticsRedirectSignIn()
    analytics.setAnalyticsUser({ id: 'one', anonymous: false })
    analytics.finishAnalyticsRedirectSignIn(true)
    await analytics.startAnalytics(router)
    analytics.finishAnalyticsRedirectSignIn(true)
    expect(events.filter(({ event }) => event === 'signed_in')).toHaveLength(1)
    analytics.beginAnalyticsRedirectSignIn()
    analytics.finishAnalyticsRedirectSignIn(false)
    analytics.finishAnalyticsRedirectSignIn(true)
    expect(events.filter(({ event }) => event === 'signed_in')).toHaveLength(1)
    router.dispose()
  })

  it.each(['disabled', 'offline', 'bad response'])('leaves the app usable when analytics is %s', async (kind) => {
    vi.stubGlobal('fetch', vi.fn(async () => {
      if (kind === 'offline') throw new Error('offline')
      return new Response(JSON.stringify({ environment: 'development', token: '', host: '' }), { status: kind === 'bad response' ? 500 : 200 })
    }))
    const router = routerAt()
    await expect(analytics.startAnalytics(router)).resolves.toBeUndefined()
    analytics.setAnalyticsUser(null)
    analytics.track('game_created')
    expect(events).toHaveLength(0)
    router.dispose()
  })

  it('captures actions only after successful API responses, without content', async () => {
    const router = routerAt()
    analytics.setAnalyticsUser(null)
    await analytics.startAnalytics(router)
    const { createCharacter } = await import('./api/characters')
    const { createGame } = await import('./api/games')
    const { acceptInvite } = await import('./api/groups')
    vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({ id: 'private', name: 'Private name' }))))
    await createCharacter({ name: 'Private name' })
    await createGame('private', 'Private name')
    await acceptInvite('private-token')
    expect(events.map(({ event }) => event)).toEqual(['$pageview', 'character_created', 'game_created', 'group_joined'])
    expect(JSON.stringify(events)).not.toContain('Private name')
    vi.stubGlobal('fetch', vi.fn(async () => new Response('{}', { status: 500 })))
    await expect(createCharacter({ name: 'Failed' })).rejects.toThrow()
    await expect(createGame('private', 'Failed')).rejects.toThrow()
    await expect(acceptInvite('failed')).rejects.toThrow()
    expect(events).toHaveLength(4)
    router.dispose()
  })
})
