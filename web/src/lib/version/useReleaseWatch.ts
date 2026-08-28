import { useEffect, useSyncExternalStore } from 'react'

import { getVersion } from '@/lib/api'
import { WEB_VERSION } from '@/lib/buildinfo'

import { isStale, noteRelease, subscribeToRelease } from './state'

/**
 * Reports whether this tab is behind the deployed release.
 *
 * Two signals feed it, because one is not enough for the three ways this app
 * gets used.
 *
 * The first costs nothing and covers active use: every API response carries
 * X-App-Version, and api/client.ts compares it at the single point every
 * request passes through. Any request the app was going to make anyway is the
 * check, so there is no interval to tune and no traffic that exists only to
 * ask.
 *
 * The second is this hook's own, and it exists because the first can never
 * fire for a tab nobody is touching. A desktop tab left open overnight, a
 * mobile tab the OS froze and thawed a day later, an installed app resumed from
 * the switcher -- none of them make a request until someone does something, by
 * which point they may have been running deleted code for a long time. So the
 * app asks once when it becomes visible again.
 *
 * Visibility is the only trigger. An `online` listener was the obvious second
 * one and buys nothing: coming back from a blip without the tab ever being
 * hidden leaves the next request to carry the header anyway.
 *
 * Failures are swallowed. Being unable to ask is not the same as being told
 * yes, and the dialog this drives is a blocking one -- opening it because the
 * wifi dropped would be the worst version of this feature.
 */
export function useReleaseWatch(own: string = WEB_VERSION): boolean {
  const stale = useSyncExternalStore(subscribeToRelease, isStale, isStale)

  useEffect(() => {
    // Nothing left to learn once it has latched, and nothing to compare
    // against in a dev-server bundle. `own` is a parameter for the same reason
    // it is one on noteRelease: a test has to be able to state both sides,
    // because a test bundle always reports "dev" and "dev" is the one value
    // this is required to ignore.
    if (stale || own === 'dev') return

    const check = (): void => {
      if (document.visibilityState !== 'visible') return
      // The response's own header would latch this through api/client.ts on
      // its way past. Reading the body too costs nothing and keeps the check
      // meaningful on its own terms rather than as a side effect of another
      // mechanism.
      void getVersion()
        .then((body) => {
          noteRelease(body.version, own)
        })
        .catch(() => {
          // Offline, or the API is down. Neither says anything about which
          // release is deployed.
        })
    }

    document.addEventListener('visibilitychange', check)
    return () => {
      document.removeEventListener('visibilitychange', check)
    }
  }, [stale, own])

  return stale
}
