import { useEffect, useRef, useState } from 'react'

import { useResource } from '@/lib/useResource'

export interface PagedRows<T> {
  rows: T[]
  total: number
}

/**
 * One filtered listing with "Load more" -- the pattern `SpellsScreen` has,
 * written once for the two admin tables.
 *
 * `key` names the filter set. A new one refetches from the top, and the rows
 * already on screen stay (as `page`, with `stale` true) until the answer
 * lands, so a table dims rather than emptying on every keystroke pause.
 */
export function usePaged<T>(
  key: string,
  fetchPage: (offset: number, signal?: AbortSignal) => Promise<PagedRows<T>>,
) {
  const found = useResource(key, (signal) => fetchPage(0, signal))

  const [last, setLast] = useState<PagedRows<T> | null>(null)
  if (found.data !== null && found.data !== last) setLast(found.data)

  // Appended pages, reset during render when the filters change: an effect
  // would paint the old extra rows once under the new first page.
  const [extra, setExtra] = useState<T[]>([])
  const [shownKey, setShownKey] = useState(key)
  const [loadingMore, setLoadingMore] = useState(false)
  if (shownKey !== key) {
    setShownKey(key)
    setExtra([])
  }
  const activeKey = useRef(key)
  useEffect(() => { activeKey.current = key }, [key])

  const page = found.data ?? last
  const rows = page === null ? [] : [...page.rows, ...extra]

  async function loadMore() {
    setLoadingMore(true)
    try {
      const next = await fetchPage(rows.length)
      // A page that answers after the filters moved belongs to nobody.
      if (activeKey.current === key) setExtra((previous) => [...previous, ...next.rows])
    } catch {
      if (activeKey.current === key) found.reload()
    } finally {
      setLoadingMore(false)
    }
  }

  return {
    page,
    rows,
    stale: found.loading,
    loading: found.loading && page === null,
    error: found.error,
    reload: found.reload,
    loadMore,
    loadingMore,
  }
}
