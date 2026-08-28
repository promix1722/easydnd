import { request } from './client'

/**
 * Shapes come from internal/api/http/v1/system. `version` is the release
 * identifier -- a tag on a release, a short commit SHA anywhere else.
 * deploy/deploy.sh health-gates a release by matching it in this body, so it is
 * a deploy contract, not just a display string.
 *
 * It is also what @/lib/version compares against WEB_VERSION, though the usual
 * route for that is the X-App-Version header rather than this endpoint.
 */

export interface VersionResponse {
  version: string
}

export function getVersion(signal?: AbortSignal): Promise<VersionResponse> {
  return request<VersionResponse>('/version', signal ? { signal } : {})
}

