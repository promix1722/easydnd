import { describe, expect, it, vi } from 'vitest'
import { act, render, screen, waitFor } from '@testing-library/react'

import { useReleaseWatch } from './useReleaseWatch'

/**
 * The signal this covers is the one the response header cannot carry: a tab
 * nobody has touched. A desktop tab left open overnight, a mobile tab the OS
 * froze and thawed a day later, an installed app resumed from the switcher --
 * none of them make a request until someone does something, so without this
 * they would sit on deleted code indefinitely.
 *
 * `own` is passed explicitly throughout. A test bundle reports "dev", and "dev"
 * is the one value the watch must ignore, so leaning on the default would make
 * every assertion below pass for the wrong reason.
 */
function Probe({ own }: { own: string }) {
  return <span>{useReleaseWatch(own) ? 'stale' : 'current'}</span>
}

/** Answers /v1/version with a release, the way the Go handler does. */
function serving(version: string): void {
  vi.stubGlobal(
    'fetch',
    vi.fn(
      () =>
        new Promise<Response>((resolve) => {
          resolve(
            new Response(JSON.stringify({ version }), {
              status: 200,
              headers: { 'Content-Type': 'application/json' },
            }),
          )
        }),
    ),
  )
}

async function becomeVisible(): Promise<void> {
  await act(async () => {
    document.dispatchEvent(new Event('visibilitychange'))
  })
}

describe('useReleaseWatch', () => {
  it('asks when the tab becomes visible, and reports a release it is behind', async () => {
    serving('v1.0.5')
    render(<Probe own="v1.0.4" />)
    expect(screen.getByText('current')).toBeInTheDocument()

    await becomeVisible()

    await waitFor(() => {
      expect(screen.getByText('stale')).toBeInTheDocument()
    })
  })

  it('stays quiet when the server is serving the release this tab is on', async () => {
    serving('v1.0.4')
    render(<Probe own="v1.0.4" />)

    await becomeVisible()

    expect(screen.getByText('current')).toBeInTheDocument()
  })

  it('does not ask at all from a dev bundle', async () => {
    serving('v1.0.5')
    render(<Probe own="dev" />)

    await becomeVisible()

    // Not merely "ignored the answer": a dev session should not be making the
    // request in the first place.
    expect(fetch).not.toHaveBeenCalled()
    expect(screen.getByText('current')).toBeInTheDocument()
  })

  it('says nothing when it cannot ask', async () => {
    // Being unable to reach the API is not evidence about what is deployed,
    // and the dialog this drives is a blocking one -- opening it because the
    // wifi dropped would be the worst version of this feature.
    vi.stubGlobal('fetch', vi.fn(() => Promise.reject(new Error('offline'))))
    render(<Probe own="v1.0.4" />)

    await becomeVisible()

    expect(screen.getByText('current')).toBeInTheDocument()
  })

  it('stops listening once unmounted', async () => {
    serving('v1.0.5')
    const { unmount } = render(<Probe own="v1.0.4" />)
    unmount()

    await becomeVisible()

    expect(fetch).not.toHaveBeenCalled()
  })
})
