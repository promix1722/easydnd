import { request } from './client'
import { developmentSession, newDevelopmentSession, saveDevelopmentSession } from './devSession'

export type DevelopmentAccount = 'master' | 'player1' | 'player2'

/** Development-only API; the server does not register this route in production. */
export async function loginDevelopmentAccount(account: DevelopmentAccount): Promise<{ game_ids: string[] }> {
  const previous = developmentSession()
  const scope = newDevelopmentSession()
  // Verify storage before signing in: a reload must retain the selector.
  saveDevelopmentSession(previous ?? '')
  const result = await request<{ game_ids: string[] }>('/dev/login', {
    method: 'POST', body: { account }, developmentSession: scope,
  })
  // Rotate even in a duplicated tab, which may have inherited its opener's
  // sessionStorage. Switching accounts must never overwrite that tab's cookie.
  saveDevelopmentSession(scope)
  return result
}
