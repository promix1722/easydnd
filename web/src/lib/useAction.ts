import { useCallback, useEffect, useRef, useState } from 'react'

import { ApiError, describeError } from './api/errors'
import type { ApiFieldError } from './api/errors'

export interface Action<TArgs extends unknown[], TResult> {
  run: (...args: TArgs) => Promise<TResult | null>
  pending: boolean

  /** A message worth showing a person, or null. */
  error: string | null

  /**
   * The server's field-level complaints, so a screen can point at the control
   * that produced one rather than showing a banner. The API's envelope names
   * a prompt in `field` -- "events[0].choices.half-elf/ability-bonus/0" --
   * which is exactly what a build screen needs to highlight.
   */
  fields: readonly ApiFieldError[]
  reset: () => void
}

/**
 * Wraps a mutating call: pending, error and the server's field errors.
 *
 * The counterpart to useResource, and hand-rolled for the same reason. It
 * returns the result rather than storing it, because every write in this API
 * answers with the new sheet and the caller almost always wants to do
 * something with it immediately.
 *
 * `run` resolves to null on failure instead of throwing, so a submit handler
 * reads as a straight line rather than a try/catch. The action is held in a
 * ref for the same reason useResource holds its fetcher in one: so a caller
 * may pass an inline arrow.
 */
export function useAction<TArgs extends unknown[], TResult>(
  perform: (...args: TArgs) => Promise<TResult>,
): Action<TArgs, TResult> {
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [fields, setFields] = useState<readonly ApiFieldError[]>([])

  // Read only from run(), so it is parked in a ref from an effect rather
  // than during render.
  const latest = useRef(perform)
  useEffect(() => {
    latest.current = perform
  })

  const reset = useCallback(() => {
    setError(null)
    setFields([])
  }, [])

  const run = useCallback(async (...args: TArgs): Promise<TResult | null> => {
    setPending(true)
    setError(null)
    setFields([])
    try {
      return await latest.current(...args)
    } catch (cause: unknown) {
      setError(describeError(cause))
      if (cause instanceof ApiError) setFields(cause.fields)
      return null
    } finally {
      setPending(false)
    }
  }, [])

  return { run, pending, error, fields, reset }
}
