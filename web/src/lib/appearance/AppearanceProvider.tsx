import { useEffect, useRef, useState, type ReactNode } from 'react'

import { useAuth } from '@/lib/auth'
import { getAppearance, saveAppearance } from '@/lib/api'
import type { SessionUser } from '@/lib/api'
import {
  accountAppearanceKey, BROWSER_APPEARANCE_KEY, isAppearance,
  LAST_ACCOUNT_KEY, readAppearance, startupAppearance, writeAppearance, type Appearance,
} from './preferences'
import { AppearanceContext } from './state'

export interface AppearanceClient {
  get: (signal?: AbortSignal) => Promise<Appearance>
  put: (appearance: Appearance, signal?: AbortSignal) => Promise<Appearance>
}
const defaultClient: AppearanceClient = { get: getAppearance, put: saveAppearance }

export function AppearanceProvider({ children, client = defaultClient }: { children: ReactNode; client?: AppearanceClient }) {
  const { user, status } = useAuth()
  const scope = user && !user.anonymous ? `account:${user.id}` : status === 'loading' || status === 'offline' ? 'startup' : 'browser'
  return <AppearanceScope key={scope} user={user} startup={scope === 'startup'} client={client}>{children}</AppearanceScope>
}

function AppearanceScope({ user, startup, client, children }: {
  user: SessionUser | null; startup: boolean; client: AppearanceClient; children: ReactNode
}) {
  const accountId = user && !user.anonymous ? user.id : null
  const key = accountId ? accountAppearanceKey(accountId) : BROWSER_APPEARANCE_KEY
  const [appearance, setAppearance] = useState<Appearance>(() => startup ? startupAppearance() : readAppearance(key))
  const [loading, setLoading] = useState(Boolean(accountId))
  const [loadFailed, setLoadFailed] = useState(false)
  const [saving, setSaving] = useState(false)
  const [failed, setFailed] = useState(false)
  const [reloadCount, setReloadCount] = useState(0)
  const pending = useRef<AbortController | null>(null)

  useEffect(() => {
    // Login and session refresh trigger an independent GET of the resource.
    pending.current?.abort()
    pending.current = null
    // oxlint-disable-next-line react/set-state-in-effect
    setSaving(false)
    // oxlint-disable-next-line react/set-state-in-effect
    setFailed(false)
    // oxlint-disable-next-line react/set-state-in-effect
    setLoadFailed(false)
    const controller = new AbortController()
    if (accountId) {
      try { localStorage.setItem(LAST_ACCOUNT_KEY, accountId) } catch { /* Optional cache. */ }
      // oxlint-disable-next-line react/set-state-in-effect
      setLoading(true)
      void client.get(controller.signal).then((next) => {
        if (controller.signal.aborted) return
        if (!isAppearance(next)) throw new Error('Invalid appearance response')
        setAppearance(next)
        writeAppearance(key, next)
      }).catch(() => {
        if (!controller.signal.aborted) setLoadFailed(true)
      }).finally(() => {
        if (!controller.signal.aborted) setLoading(false)
      })
    } else if (!startup) {
      try { localStorage.removeItem(LAST_ACCOUNT_KEY) } catch { /* Optional cache. */ }
    }
    return () => { controller.abort(); pending.current?.abort(); pending.current = null }
  }, [user, accountId, key, startup, client, reloadCount])

  async function change(next: Appearance) {
    if (loading || pending.current || !isAppearance(next)) return
    const previous = appearance
    setAppearance(next)
    setFailed(false)
    if (!accountId) { writeAppearance(key, next); return }
    const controller = new AbortController()
    pending.current = controller
    setSaving(true)
    try {
      const saved = await client.put(next, controller.signal)
      if (controller.signal.aborted) return
      if (!isAppearance(saved)) throw new Error('Invalid appearance response')
      setAppearance(saved)
      writeAppearance(key, saved)
      setLoadFailed(false)
    } catch {
      if (controller.signal.aborted) return
      setAppearance(previous)
      setFailed(true)
    } finally {
      if (!controller.signal.aborted) { pending.current = null; setSaving(false) }
    }
  }

  return <AppearanceContext value={{ appearance, loading, loadFailed, saving, failed, change,
    reload: () => setReloadCount((count) => count + 1),
  }}>{children}</AppearanceContext>
}
