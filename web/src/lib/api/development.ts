import { request } from './client'

export type DevelopmentAccount = 'master' | 'player1' | 'player2'

/** Development-only API; the server does not register this route in production. */
export function loginDevelopmentAccount(account: DevelopmentAccount): Promise<{ game_ids: string[] }> {
  return request('/dev/login', { method: 'POST', body: { account } })
}
