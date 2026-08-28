import { WEB_VERSION } from '@/lib/buildinfo'

/**
 * Whether this tab is still running the release that is deployed.
 *
 * The question has one answer for the whole tab, so it lives in module state
 * rather than in a context: the API client learns it as a side effect of a
 * request somebody else asked for, and has nowhere to return it to. That makes
 * this the second piece of module-level mutable state in src/, after the
 * catalogue's request cache -- and, like that one, it has to be reset between
 * test files, because the suite shares one module registry.
 *
 * Nothing here imports @/lib/api, and that is deliberate rather than
 * incidental: api/client.ts imports this file, so an import back the other way
 * would close a cycle. The half that does need the API -- the check on regaining
 * focus -- lives in ./useReleaseWatch.ts, which nothing here imports.
 */

/** Header name and semantics come from internal/api/http/middleware/version.go. */
const HEADER_APP_VERSION = 'X-App-Version'

let stale = false
const listeners = new Set<() => void>()

/** Whether a newer release is known to be deployed. */
export function isStale(): boolean {
  return stale
}

/** Subscribes to the answer changing. Shaped for useSyncExternalStore. */
export function subscribeToRelease(listener: () => void): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

/**
 * Records which release the server just said it was.
 *
 * Latching, on purpose: once a newer release is known to exist, this tab stays
 * stale until it reloads. Nothing that arrives later is evidence to the
 * contrary -- a response held in an HTTP cache can name an older release long
 * after it stopped being deployed, and unlatching on one of those would dismiss
 * a dialog the user was reading.
 *
 * `own` is a parameter rather than a straight read of WEB_VERSION so that a
 * test can state both sides of the comparison. Production has exactly one
 * caller and it takes the default.
 */
export function noteRelease(deployed: string | null | undefined, own: string = WEB_VERSION): void {
  // In order: already latched; a dev bundle, which has no release to be behind
  // and would otherwise open the dialog on every `make dev` session's first
  // request; no header at all, meaning something other than our Go handler
  // answered -- an nginx error page, a proxy timeout, a captive portal -- which
  // is not evidence about anything; and the ordinary case of agreeing.
  if (stale || own === 'dev' || !deployed || deployed === own) return

  stale = true
  for (const listener of listeners) listener()
}

/** Reads the release header off a response the API client already has. */
export function noteReleaseHeader(response: Response): void {
  noteRelease(response.headers.get(HEADER_APP_VERSION))
}

/** Clears the watch. For tests; see src/test/setup.ts. */
export function resetReleaseWatch(): void {
  stale = false
  listeners.clear()
}
