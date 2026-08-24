import { getHealth, getVersion } from '@/lib/api'
import { useResource } from '@/lib/useResource'
import type { Resource } from '@/lib/useResource'

export interface ApiStatus {
  version: string
  health: string
}

export type ApiStatusState = Resource<ApiStatus>

/**
 * Fetches the API's version and liveness.
 *
 * The outcome union, the AbortController and the reload counter that used to
 * live here now live in useResource, which every screen uses. Keeping this
 * hook as a thin wrapper rather than deleting it is deliberate: it is the
 * existing call site that proves the generalization, and StatusPanel did not
 * have to change.
 */
export function useApiStatus(): ApiStatusState {
  return useResource<ApiStatus>('api-status', async (signal) => {
    const [version, health] = await Promise.all([getVersion(signal), getHealth(signal)])
    return { version: version.version, health: health.status }
  })
}
