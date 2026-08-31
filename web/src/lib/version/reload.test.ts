import { afterEach, describe, expect, it, vi } from 'vitest'

import { reloadOntoDeployedRelease } from './reload'

/**
 * The half of the update that has no UI and used to be wrong in a way only a
 * network trace showed: the dialog's button appeared to do nothing, because the
 * reload it fired beat the new worker's install and the old worker answered the
 * navigation from its own precache.
 *
 * jsdom has no service workers, so `navigator.serviceWorker` is stubbed. That
 * is a stub of the *platform*, not of this module -- the thing under test is
 * still the real ordering, and the assertions are about which of the two
 * possible endings happened and when.
 */
type Listener = () => void

function fakeWorker(state: ServiceWorker['state']) {
  const listeners: Listener[] = []
  return {
    state,
    postMessage: vi.fn(),
    addEventListener: (_: string, listener: Listener) => listeners.push(listener),
    /** Moves the worker on and tells whoever is waiting. */
    become(next: ServiceWorker['state']) {
      this.state = next
      for (const listener of [...listeners]) listener()
    },
  }
}

function stubServiceWorker(registration: unknown) {
  const controllerChange: Listener[] = []
  Object.defineProperty(navigator, 'serviceWorker', {
    configurable: true,
    value: {
      getRegistration: () => Promise.resolve(registration),
      addEventListener: (event: string, listener: Listener) => {
        if (event === 'controllerchange') controllerChange.push(listener)
      },
    },
  })
  return { takeControl: () => controllerChange.forEach((listener) => listener()) }
}

afterEach(() => {
  Reflect.deleteProperty(navigator, 'serviceWorker')
})

describe('reloadOntoDeployedRelease', () => {
  it('waits for an installing worker before telling it to skip waiting', async () => {
    const worker = fakeWorker('installing')
    const registration = {
      waiting: null,
      installing: worker,
      update: () => {
        // What a browser actually does: resolve with the install still running.
        // Reading `waiting` at this point finds nothing, which is what used to
        // send this straight to a plain reload.
        queueMicrotask(() => {
          worker.become('installed')
        })
        return Promise.resolve()
      },
    }
    const { takeControl } = stubServiceWorker(registration)
    const reload = vi.fn()

    await reloadOntoDeployedRelease(reload)

    expect(worker.postMessage).toHaveBeenCalledWith({ type: 'SKIP_WAITING' })
    // Not yet: the new worker has been told to take over and the reload is the
    // controllerchange's job. Reloading here is the bug this test exists for.
    expect(reload).not.toHaveBeenCalled()

    takeControl()
    expect(reload).toHaveBeenCalledTimes(1)
  })

  it('reloads plainly when there is no new worker to wait for', async () => {
    stubServiceWorker({ waiting: null, installing: null, update: () => Promise.resolve() })
    const reload = vi.fn()

    await reloadOntoDeployedRelease(reload)

    expect(reload).toHaveBeenCalledTimes(1)
  })
})
