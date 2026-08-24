import { useCallback, useEffect, useRef, useState } from 'react'

import { describeError } from './api/errors'

/**
 * A request outcome. Modelled as a union rather than three loose booleans so
 * that "loading with stale data still on screen" and "failed but data set"
 * are simply unrepresentable.
 */
type Outcome<T> =
  | { kind: 'loading' }
  | { kind: 'ready'; data: T }
  | { kind: 'failed'; error: string }

export interface Resource<T> {
  data: T | null
  error: string | null
  loading: boolean
  reload: () => void
}

/**
 * Fetches a value and tracks the request.
 *
 * There is deliberately no server-state library behind this. The reason is
 * structural rather than principled: every endpoint that mutates a character
 * returns the new sheet in its response, so there is no cache to invalidate --
 * the response *is* the invalidation. A query library earns its keep when many
 * components read overlapping queries with independent lifetimes, and here one
 * screen owns one character. Revisit if a party view ever renders six.
 *
 * `key` identifies what is being fetched, e.g. `sheet:chr_000001`. Changing it
 * aborts the request in flight and starts a new one. It is a string rather
 * than a dependency array so that the effect's dependencies stay statically
 * checkable, and because every caller here is fetching one addressable thing.
 *
 * `fetcher` is held in a ref, so a caller may pass an inline arrow without
 * pinning it in a useCallback at every call site.
 */
export function useResource<T>(key: string, fetcher: (signal: AbortSignal) => Promise<T>): Resource<T> {
  const [outcome, setOutcome] = useState<Outcome<T>>({ kind: 'loading' })
  const [attempt, setAttempt] = useState(0)

  // The fetcher is read only from the effect, so it is parked in a ref there
  // rather than during render.
  const latest = useRef(fetcher)
  useEffect(() => {
    latest.current = fetcher
  })

  // Resetting to loading when the key changes is done during render, not in
  // an effect: an effect would paint the previous resource's data once under
  // the new key before correcting itself.
  const [shown, setShown] = useState(key)
  if (shown !== key) {
    setShown(key)
    setOutcome({ kind: 'loading' })
  }

  // The reset belongs to the click, not to the effect, for the same reason.
  const reload = useCallback(() => {
    setOutcome({ kind: 'loading' })
    setAttempt((n) => n + 1)
  }, [])

  useEffect(() => {
    const controller = new AbortController()

    latest
      .current(controller.signal)
      .then((data) => {
        if (controller.signal.aborted) return
        setOutcome({ kind: 'ready', data })
      })
      .catch((cause: unknown) => {
        if (controller.signal.aborted) return
        setOutcome({ kind: 'failed', error: describeError(cause) })
      })

    return () => {
      controller.abort()
    }
  }, [key, attempt])

  return {
    data: outcome.kind === 'ready' ? outcome.data : null,
    error: outcome.kind === 'failed' ? outcome.error : null,
    loading: outcome.kind === 'loading',
    reload,
  }
}
