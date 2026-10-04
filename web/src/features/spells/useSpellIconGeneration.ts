import { useCallback, useEffect, useRef, useState } from 'react'

import { describeError } from '@/lib/api'
import {
  getSpellIconGeneration,
  startSpellIconGeneration,
} from '@/lib/api/spellIcons'
import type { IconGenerationRequest, IconGenerationState } from '@/lib/api/spellIcons'
import { useT } from '@/lib/i18n'

/** How often the queue is re-asked while it says it is working. */
const POLL_MS = 1500

export interface SpellIconGeneration {
  /** The dev process's queue, or null until the first answer arrives. */
  state: IconGenerationState | null
  /** The last GET or POST's rejection, in words; null when all is well. */
  error: string | null
  /** A POST is in flight -- the window a second click could double-charge. */
  submitting: boolean
  /** Ask for the queue now; the "check again" control for a failed poll. */
  refresh: () => void
  /** Enqueue one spell or a whole filtered batch. */
  generate: (request: IconGenerationRequest) => Promise<boolean>
}

/**
 * The dev spell-icon generator's queue, shared by the screen that starts it.
 *
 * The queue belongs to the service process, not this hook: another tab, or
 * this one reloaded, sees the same run. So the hook GETs once at mount to
 * restore whatever is already in flight and then polls only while the server
 * says `running` -- a finished queue holds still and asks for nothing.
 *
 * All of it is inert in a production bundle: the route it talks to is
 * registered only in the development server, so `import.meta.env.DEV` gates
 * the effects here, and the one JSX caller is gated at its call site the way
 * StubButton is.
 */
export function useSpellIconGeneration(): SpellIconGeneration {
  const t = useT()
  const [state, setState] = useState<IconGenerationState | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  // The poll ticks at an interval, so a request still in flight must not be
  // asked again before it answered -- overlap would pile up stale answers.
  const fetching = useRef(false)
  const posting = useRef(false)
  const epoch = useRef(0)
  const refresh = useCallback(() => {
    if (!import.meta.env.DEV || fetching.current || posting.current) return
    const requestEpoch = epoch.current
    fetching.current = true
    void getSpellIconGeneration()
      .then((next) => {
        if (epoch.current !== requestEpoch) return
        setState(next)
        setError(null)
      })
      .catch((cause: unknown) => {
        if (epoch.current === requestEpoch) setError(describeError(t, cause))
      })
      .finally(() => {
        fetching.current = false
      })
  }, [t])

  // Restore at mount: a run started before this screen loaded (or in another
  // tab) announces itself here and picks the poller up below.
  useEffect(() => refresh(), [refresh])

  const running = state?.running === true
  useEffect(() => {
    if (!running) return undefined
    const timer = setInterval(refresh, POLL_MS)
    return () => clearInterval(timer)
  }, [running, refresh])

  const generate = useCallback(
    async (request: IconGenerationRequest): Promise<boolean> => {
      if (!import.meta.env.DEV || posting.current) return false
      posting.current = true
      epoch.current++
      setSubmitting(true)
      setError(null)
      try {
        setState(await startSpellIconGeneration(request))
        return true
      } catch (cause: unknown) {
        // The 409 for a run already in flight lands here, as words --
        // the paid second submission never happened, so it is a message,
        // not a retry.
        setError(describeError(t, cause))
        return false
      } finally {
        posting.current = false
        setSubmitting(false)
      }
    },
    [t],
  )

  return { state, error, submitting, refresh, generate }
}
