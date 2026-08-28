import { describe, expect, it, vi } from 'vitest'

import { getInstallOffer, install, subscribeToInstall } from './state'

/**
 * The events are dispatched for real rather than mocked. The suite shares one
 * module registry so `vi.mock` is banned, and this module listens on `window`
 * at import time anyway -- a synthetic event is both the honest way in and the
 * only one.
 */
function chromeOffersInstall(prompt = vi.fn(() => Promise.resolve())): Event {
  const event = new Event('beforeinstallprompt', { cancelable: true })
  Object.assign(event, { prompt, userChoice: Promise.resolve({ outcome: 'accepted' }) })
  window.dispatchEvent(event)
  return event
}

describe('the install offer', () => {
  it('offers nothing until the browser says it can', () => {
    // jsdom is not iOS and fires no beforeinstallprompt of its own.
    expect(getInstallOffer()).toBe('none')
  })

  it('offers the browser prompt once the event arrives', () => {
    chromeOffersInstall()
    expect(getInstallOffer()).toBe('prompt')
  })

  it('takes the event out of Chrome hands, so only one offer is shown', () => {
    const event = chromeOffersInstall()
    // preventDefault is what suppresses Chrome's own promotion; without it the
    // viewport carries two offers of the same thing.
    expect(event.defaultPrevented).toBe(true)
  })

  it('tells subscribers when the answer changes', () => {
    const listener = vi.fn()
    subscribeToInstall(listener)

    chromeOffersInstall()
    expect(listener).toHaveBeenCalledTimes(1)
  })

  it('stops telling a subscriber that unsubscribed', () => {
    const listener = vi.fn()
    subscribeToInstall(listener)()

    chromeOffersInstall()
    expect(listener).not.toHaveBeenCalled()
  })

  it('prompts the browser, and spends the event doing it', async () => {
    const prompt = vi.fn(() => Promise.resolve())
    chromeOffersInstall(prompt)

    await install()

    expect(prompt).toHaveBeenCalledTimes(1)
    // The event is single use: Chrome will not accept a second prompt() on it,
    // so the button must stop offering rather than offer a dialog that refuses
    // to open.
    expect(getInstallOffer()).toBe('none')
  })

  it('does nothing when there is no event to prompt with', async () => {
    await expect(install()).resolves.toBeUndefined()
  })

  it('stops offering once the app has been installed', () => {
    chromeOffersInstall()
    // Fired however the install happened -- our button, or Chrome's own
    // omnibox icon, which still works and which we do not hear about otherwise.
    window.dispatchEvent(new Event('appinstalled'))

    expect(getInstallOffer()).toBe('none')
  })
})
