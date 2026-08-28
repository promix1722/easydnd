import { describe, expect, it, vi } from 'vitest'

import { isStale, noteRelease, noteReleaseHeader, resetReleaseWatch, subscribeToRelease } from './state'

/**
 * Both sides of the comparison are stated explicitly. The bundle under test
 * reports "dev" -- vitest never sets VITE_APP_VERSION -- and "dev" is the one
 * value the watch is required to ignore, so a test that leaned on the default
 * would pass no matter what the code did.
 */
describe('the release watch', () => {
  it('stays current while the server reports the release this bundle is', () => {
    noteRelease('v1.0.4', 'v1.0.4')
    expect(isStale()).toBe(false)
  })

  it('goes stale when the server reports a different release', () => {
    noteRelease('v1.0.5', 'v1.0.4')
    expect(isStale()).toBe(true)
  })

  it('ignores a dev bundle, which has no release to be behind', () => {
    noteRelease('v1.0.5', 'dev')
    expect(isStale()).toBe(false)
  })

  it('ignores a response that named no release at all', () => {
    // An nginx error page, a proxy timeout, a captive portal: something other
    // than our handler answered, which says nothing about what is deployed.
    noteRelease(null, 'v1.0.4')
    noteRelease(undefined, 'v1.0.4')
    noteRelease('', 'v1.0.4')
    expect(isStale()).toBe(false)
  })

  it('latches, so a response naming the old release cannot dismiss the dialog', () => {
    noteRelease('v1.0.5', 'v1.0.4')
    // A response held in an HTTP cache can name a release that stopped being
    // deployed some time ago; arriving late is not evidence of anything.
    noteRelease('v1.0.4', 'v1.0.4')
    expect(isStale()).toBe(true)
  })

  it('tells subscribers once, when the answer changes', () => {
    const listener = vi.fn()
    subscribeToRelease(listener)

    noteRelease('v1.0.4', 'v1.0.4')
    expect(listener).not.toHaveBeenCalled()

    noteRelease('v1.0.5', 'v1.0.4')
    expect(listener).toHaveBeenCalledTimes(1)

    // Already latched: there is nothing further to say.
    noteRelease('v1.0.6', 'v1.0.4')
    expect(listener).toHaveBeenCalledTimes(1)
  })

  it('stops telling a subscriber that unsubscribed', () => {
    const listener = vi.fn()
    subscribeToRelease(listener)()

    noteRelease('v1.0.5', 'v1.0.4')
    expect(listener).not.toHaveBeenCalled()
  })

  it('reads the release off a response header', () => {
    // The header name is the contract with
    // internal/api/http/middleware/version.go.
    const response = new Response(null, { headers: { 'X-App-Version': 'v1.0.5' } })
    noteReleaseHeader(response)
    // The bundle under test reports "dev", so this must not latch -- which is
    // also the guarantee that `make dev` does not open the dialog on its first
    // request against a real API.
    expect(isStale()).toBe(false)
  })

  it('resets, because the suite shares one module registry', () => {
    noteRelease('v1.0.5', 'v1.0.4')
    expect(isStale()).toBe(true)
    resetReleaseWatch()
    expect(isStale()).toBe(false)
  })
})
