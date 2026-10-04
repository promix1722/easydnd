import { request } from './client'
import type { SpellSearch } from './catalog'

/**
 * The development server's spell-icon generator -- a development-only route
 * on the Go service, so a production build neither calls nor ships it.
 *
 * One job is running at a time for the whole process, and the state below
 * is that process's state rather than this screen's: a second tab sees the
 * same queue, which is why the screen polls and restores rather than owning.
 */
export interface IconGenerationJob {
  state: 'queued' | 'generating' | 'done' | 'skipped' | 'failed'
  /** A reason slug (icon_generation_*), not prose; map it like an error. */
  reason?: string
  /** Set once artwork landed: a cache-buster for the row's icon URL. */
  revision?: string
}

export interface IconGenerationState {
  configured: boolean
  running: boolean
  total: number
  /** Done + skipped + failed -- the count that reaches `total`. */
  completed: number
  skipped: number
  failed: number
  /** Per-spell outcome, keyed by slug. */
  items: Record<string, IconGenerationJob>
}

/**
 * What a POST asks for. `scope` is the catalogue scope string the screen
 * already builds ('browse' or '/packs/catalog?packs=...'); the server
 * allowlists it and follows `search` across every page itself, so `limit` and
 * `offset` in `search` are ignored. `slug` narrows the job to one spell and
 * `slugs` to an explicit list -- the resume path sends one for the icons a
 * previous run left unfinished; the two keys never travel together.
 * `replace` regenerates icons that already exist rather than skipping them.
 */
export interface IconGenerationRequest {
  scope: string
  search: SpellSearch
  slug?: string
  slugs?: string[]
  replace: boolean
}

export function getSpellIconGeneration(): Promise<IconGenerationState> {
  return request<IconGenerationState>('/dev/spell-icons')
}

/**
 * Enqueues a generation run and returns the queue as it was accepted -- the
 * paid work itself is still in flight, so callers keep polling rather than
 * reading this as finished. The server answers 409 when a run is already
 * active, which `useSpellIconGeneration` reports rather than hiding.
 */
export function startSpellIconGeneration(
  body: IconGenerationRequest,
): Promise<IconGenerationState> {
  const { slug, ...rest } = body
  return request<IconGenerationState>('/dev/spell-icons', {
    method: 'POST',
    body: { ...rest, ...(slug === undefined ? {} : { slug }) },
  })
}
