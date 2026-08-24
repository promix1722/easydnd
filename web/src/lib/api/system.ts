import { request } from './client'

/**
 * Shapes come from internal/api/http/v1/system. `version` is the raw commit
 * SHA: deploy/deploy.sh health-gates a release by grepping for it, so it is a
 * deploy contract, not just a display string.
 */

export interface VersionResponse {
  version: string
}

export interface HealthResponse {
  status: string
}

export function getVersion(signal?: AbortSignal): Promise<VersionResponse> {
  return request<VersionResponse>('/version', signal ? { signal } : {})
}

export function getHealth(signal?: AbortSignal): Promise<HealthResponse> {
  return request<HealthResponse>('/health', signal ? { signal } : {})
}
