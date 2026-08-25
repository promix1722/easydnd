import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { captureInviteToken, clearInviteToken, readInviteToken } from './inviteToken'

function setHash(token: string) {
  window.location.hash = token === '' ? '' : `#${token}`
}

beforeEach(() => {
  window.sessionStorage.clear()
  setHash('')
})

afterEach(() => {
  window.sessionStorage.clear()
  setHash('')
})

describe('captureInviteToken', () => {
  it('returns the token in the fragment and saves it', () => {
    setHash('a-token')
    expect(captureInviteToken()).toBe('a-token')

    // The whole point: it is still available once the fragment is gone, which
    // is what a trip through Google or /login does to it.
    setHash('')
    expect(readInviteToken()).toBe('a-token')
  })

  it('falls back to what was saved when there is no fragment', () => {
    setHash('a-token')
    captureInviteToken()
    setHash('')

    expect(captureInviteToken()).toBe('a-token')
  })

  it('is empty when nothing was ever offered', () => {
    expect(captureInviteToken()).toBe('')
    expect(readInviteToken()).toBe('')
  })

  // A fresh link must win over a stale one, or somebody forwarded a second
  // invitation would keep joining the first group.
  it('prefers the fragment over anything saved earlier', () => {
    setHash('older')
    captureInviteToken()
    setHash('newer')

    expect(captureInviteToken()).toBe('newer')
    setHash('')
    expect(readInviteToken()).toBe('newer')
  })

  it('forgets on request', () => {
    setHash('a-token')
    captureInviteToken()
    setHash('')

    clearInviteToken()
    expect(readInviteToken()).toBe('')
  })
})
