import { request } from './client'
import { isAppearance, type Appearance } from '@/lib/appearance/preferences'

/** The current account's appearance resource, independent of authentication. */
export async function getAppearance(signal?: AbortSignal): Promise<Appearance> {
  const value = await request<Appearance>('/appearance', signal ? { signal } : {})
  if (!isAppearance(value)) throw new Error('Invalid appearance response')
  return value
}

/** PUT replaces both fields and returns the persisted representation. */
export async function saveAppearance(appearance: Appearance, signal?: AbortSignal): Promise<Appearance> {
  const value = await request<Appearance>('/appearance', {
    method: 'PUT', body: appearance, ...(signal ? { signal } : {}),
  })
  if (!isAppearance(value)) throw new Error('Invalid appearance response')
  return value
}
