/**
 * Reloading onto the deployed release, which is not the same as reloading.
 *
 * The service worker precaches index.html and answers every navigation from
 * that precache, so on a controlled client -- which is every returning visitor
 * and every installed app -- location.reload() is served the same stale page it
 * was already showing. The update dialog would reopen on top of it, forever.
 *
 * So the worker has to be updated first. The generated sw.js listens for
 * {type: 'SKIP_WAITING'} (workbox emits that listener because registerType is
 * 'prompt' rather than 'autoUpdate'; see vite.config.ts), and the reload waits
 * for the new worker to take control.
 *
 * Written against navigator.serviceWorker rather than vite-plugin-pwa's
 * `virtual:pwa-register`, so that this file is ordinary TypeScript: the virtual
 * module resolves only through the plugin, which would make it a build-time
 * dependency of anything importing it, tests included.
 */

/**
 * How long to wait for the new worker to take control before reloading anyway.
 *
 * The button must always do something. If the worker is wedged, a plain reload
 * is still better than a dialog that swallows the press -- at worst the page
 * comes back stale and says so again, which is where we already were.
 */
const TAKEOVER_TIMEOUT_MS = 5_000

/**
 * Reloads this tab onto the currently deployed release.
 *
 * `reload` is injected so that a test can watch for the reload it would
 * otherwise have to survive; production has one caller and it takes the
 * default.
 */
export async function reloadOntoDeployedRelease(
  reload: () => void = () => {
    window.location.reload()
  },
): Promise<void> {
  const registration = 'serviceWorker' in navigator
    ? await navigator.serviceWorker.getRegistration()
    : undefined
  if (!registration) {
    // No worker to get past: an unsupported browser, a first visit, or the dev
    // server, where the worker is disabled outright.
    reload()
    return
  }

  // Armed before the update is asked for, not after. `update()` can resolve
  // with the new worker already installed and claiming, and a listener added
  // afterwards would have missed the event it exists to catch.
  let reloaded = false
  const once = (): void => {
    if (reloaded) return
    reloaded = true
    reload()
  }
  navigator.serviceWorker.addEventListener('controllerchange', once, { once: true })
  window.setTimeout(once, TAKEOVER_TIMEOUT_MS)

  // Fetches sw.js and installs it if the bytes differ. nginx serves that file
  // no-cache, so the check is a real one rather than a cache hit. The spec has
  // this resolve once the install job is done, so `waiting` is populated by the
  // time we read it -- and if a browser ever disagrees, the timeout above is
  // the answer rather than a second wait for `statechange`.
  try {
    await registration.update()
  } catch {
    // Offline, or the update fetch failed. Fall through: there may still be a
    // worker waiting from an earlier check, and the timeout covers the rest.
  }

  if (registration.waiting) {
    registration.waiting.postMessage({ type: 'SKIP_WAITING' })
    return
  }

  // The worker is already the deployed one -- only the API had moved on, or
  // this tab was told by a response that arrived before the bundle changed.
  // A plain reload is the whole remedy.
  once()
}
