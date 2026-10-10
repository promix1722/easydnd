import { useEffect, useRef, useState } from 'react'

import {
  getAgentSession,
  listAgentSessions,
  openAgentSession,
  pollAgentSession,
  polling,
} from '@/lib/api/agent'
import type { AgentEvent, AgentView } from '@/lib/api/agent'
import { ApiError } from '@/lib/api/errors'
import { useAction } from '@/lib/useAction'

// What the tab shows after an answer arrives. A whole session replaces the one
// held unless it is older -- a slow read must not undo a newer write -- and a
// tail of events is appended past the ones already here. Event ids are 1..n.
function merge(
  old: AgentView | null,
  next: AgentView | { events: AgentEvent[] },
): AgentView | null {
  if ('session' in next)
    return old &&
      old.session.id === next.session.id &&
      (old.session.revision > next.session.revision ||
        (old.session.revision === next.session.revision &&
          old.session.events.length > next.session.events.length))
      ? old
      : next
  if (!old) return old
  const added = next.events.filter((e) => e.id > old.session.events.length)
  return added.length
    ? { session: { ...old.session, events: [...old.session.events, ...added] } }
    : old
}

/**
 * The chat a wizard page is on: which one, what this tab holds of it, the
 * poll that keeps it current, and the action every write to it goes through.
 * `named` is the chat the address names, if it names one.
 */
export function useAgentSession(named: string | null, folder: string | undefined) {
  // The chat left unfinished, found when the wizard is opened without one
  // named. Held here rather than navigated to: the page is the wizard either
  // way, and there is no second step that can fail to happen.
  const [resumed, setResumed] = useState<string | null>(null)
  const id = named ?? resumed
  const [view, setView] = useState<AgentView | null>(null)
  const [connected, setConnected] = useState(true)
  const boot = useRef<{ folder: string | undefined; found: Promise<string | undefined> }>(undefined)
  // What the poll loop holds, which is the view one render early: its cursor
  // has to move with an answer, or the next request would ask for the same
  // thing again before React has drawn it.
  const latest = useRef(view)
  latest.current = view
  const action = useAction(async (work: () => Promise<AgentView>) => {
    const result = await work()
    latest.current = merge(latest.current, result)
    setView((old) => merge(old, result))
    return result
  })
  useEffect(() => {
    setView((old) => (old?.session.id === id ? old : null))
    if (!id) {
      // The wizard opens on the chat that was left unfinished, the latest if
      // there are several; a finished one is reached from its character. With
      // none, it opens a new one -- so there is a session, and a first event
      // to draw, before anything is asked.
      //
      // Asked once per arrival, however often this effect runs: a second run
      // that listed before the first had opened would open a second chat.
      let gone = false
      if (boot.current?.folder !== folder || !boot.current)
        boot.current = {
          folder,
          found: listAgentSessions().then((s) => {
            const last = s
              .filter((v) => !v.finished && (!folder || v.folder === folder))
              .sort((a, b) => (b.created ?? '').localeCompare(a.created ?? ''))[0]
            return last?.id ?? action.run(() => openAgentSession(folder)).then((made) => made?.session.id)
          }),
        }
      void boot.current.found
        .then((found) => {
          if (found && !gone) setResumed(found)
        })
        .catch(() => {})
      return () => {
        gone = true
      }
    }
    // One poll a second, each answered at once with what this tab lacks.
    // Reads never disable the message composer. See docs/polling.md.
    const stop = new AbortController()
    const pause = (ms: number) => new Promise((done) => window.setTimeout(done, ms))
    const visible = () =>
      new Promise<void>((done) => {
        if (!document.hidden) return done()
        const shown = () => {
          if (document.hidden && !stop.signal.aborted) return
          document.removeEventListener('visibilitychange', shown)
          stop.signal.removeEventListener('abort', shown)
          done()
        }
        document.addEventListener('visibilitychange', shown)
        stop.signal.addEventListener('abort', shown)
      })
    void (async () => {
      await action.run(() => getAgentSession(id)).catch(() => {})
      while (!stop.signal.aborted) {
        await visible()
        const held = latest.current?.session.id === id ? latest.current.session : undefined
        // A finished chat is a record: nothing will ever be added to it.
        if (held?.finished) return
        try {
          const next = held
            ? await pollAgentSession(id, held.revision, held.events.length, stop.signal)
            : await getAgentSession(id)
          if (stop.signal.aborted) return
          setConnected(true)
          if (next) {
            latest.current = merge(latest.current, next)
            setView((old) => merge(old, next))
          }
          if (held) await pause(polling.every)
        } catch (cause) {
          if (stop.signal.aborted) return
          // Discarded, or never this account's: nothing left to wait for.
          if (cause instanceof ApiError && cause.status === 404) return
          setConnected(false)
          await pause(1000)
        }
      }
    })()
    return () => stop.abort()
    // action.run is stable; switching session is the only subscription boundary.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, folder])
  /** Lets go of the chat found on arrival, so the next arrival looks again. */
  const forget = () => {
    boot.current = undefined
    setResumed(null)
  }
  return { id, view, action, connected, forget }
}

export type AgentSessionAction = ReturnType<typeof useAgentSession>['action']
