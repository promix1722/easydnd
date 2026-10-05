import { useEffect, useState } from 'react'
import type { AgentEvent } from '@/lib/api/agent'

/** The pause between two bubbles, the step of the typing, and the longest any
 * one message takes to type, in milliseconds. */
const BEAT = 300
const TICK = 30
const LONGEST = 2400

/** Off, everything is shown as it arrives. For tests of anything but the
 * pacing, which would otherwise each wait for it. */
export const pacing = { on: true }

const spoken = (event: AgentEvent) =>
  ['assistant', 'question', 'response'].includes(event.kind) && !!event.text

/** Whether an event is shown the moment it is reached: everything that draws
 * nothing, the player's own words, text that was already streamed chunk by
 * chunk, and all of it in a tab nobody is looking at. */
function instant(events: AgentEvent[], at: number) {
  const event = events[at]!
  if (document.hidden || !pacing.on) return true
  if (event.kind === 'progress') return false
  if (!spoken(event)) return true
  return event.kind === 'response' && events[at - 1]?.kind === 'delta'
}

/**
 * Paces a chat the way a messenger does. The server answers in bursts -- a
 * whole response's writes and its question in one poll -- and drawing them as
 * they land is a wall of bubbles at once. This hands the transcript on one
 * bubble at a time, and types the assistant's words out.
 *
 * What was already there when the chat was opened is history and is shown at
 * once. Reduced motion does not switch this off: a pause is not motion, and
 * what moves is CSS, which answers that setting itself. `settled`
 * is false while something is still to come: the turn is not the player's
 * until they have been shown all of it.
 */
export function useReveal(id: string | undefined, events: AgentEvent[]) {
  let [at, setAt] = useState({ id, shown: events.length, typed: 0 })
  if (at.id !== id || at.shown > events.length) {
    at = { id, shown: events.length, typed: 0 }
    setAt(at)
  }
  let { shown } = at
  while (shown < events.length && instant(events, shown)) shown++
  const typed = shown === at.shown ? at.typed : 0
  const next = events[shown]
  const waiting = events.length - shown

  useEffect(() => {
    if (!next) return
    const text = next.text ?? ''
    const typing = spoken(next)
    const timer = window.setTimeout(
      () => {
        if (!typing) return setAt({ id, shown: shown + 1, typed: 0 })
        // A word at a time, and never so slowly that a long message drags.
        const space = text.indexOf(' ', typed + 1)
        const to = Math.max(
          space < 0 ? text.length : space + 1,
          typed + Math.ceil((text.length * TICK) / LONGEST),
        )
        setAt(to >= text.length ? { id, shown: shown + 1, typed: 0 } : { id, shown, typed: to })
      },
      // Only a very long backlog is hurried, and never into a blur.
      typing ? TICK : waiting > 25 ? BEAT / 2 : BEAT,
    )
    return () => window.clearTimeout(timer)
    // `next` is events[shown]; the count covers the events arriving after it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, shown, typed, events.length])

  if (!next) return { events, settled: true }
  const partial = { ...next, text: (next.text ?? '').slice(0, typed) }
  // What a message offers is offered once it has been said.
  delete partial.options
  return {
    events: typed > 0 ? [...events.slice(0, shown), partial] : events.slice(0, shown),
    settled: false,
  }
}
