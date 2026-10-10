import type { RulesLock } from './packs'
import { request } from './client'

export interface AgentEvent {
  actions?: ('view' | 'edit')[]
  files?: { name: string; mime: string }[]
  options?: string[]
  source?: string
  assumption?: string
  id: number
  kind: string
  text?: string
  tool?: string
  data?: unknown
}
export interface AgentSession {
  id: string
  folder: string
  status:
    | 'opening'
    | 'queued'
    | 'running'
    | 'waiting'
    | 'paused'
    | 'failed'
    | 'review'
  revision: number
  characterId?: string
  created: string
  finished: boolean
  events: AgentEvent[]
  files: { name: string; mime: string }[]
  manual: {
    id: string
    kind: string
    name: string
    description: string
    source: string
  }[]
  assumptions: string[]
}
export interface AgentView {
  session: AgentSession
}
export const agentCapabilities = () =>
  request<{ enabled: boolean }>('/agent-capabilities')
export const listAgentSessions = () =>
  request<AgentSession[]>('/agent-sessions')
export const getAgentSession = (id: string) =>
  request<AgentView>(`/agent-sessions/${encodeURIComponent(id)}`)
/** How often an open chat asks the server what is new, in milliseconds. A
 * value and not a constant so that tests need not wait for it. */
export const polling = { every: 1000 }
// One poll: what this tab holds goes up, and the answer -- at once -- is the
// whole session if its revision moved, the events past `after` if only those
// did, or nothing (204). See docs/polling.md.
export const pollAgentSession = (
  id: string,
  revision: number,
  after: number,
  signal: AbortSignal,
) =>
  request<AgentView | { events: AgentEvent[] } | undefined>(
    `/agent-sessions/${encodeURIComponent(id)}?revision=${revision}&after=${after}`,
    { signal },
  )
/** Opens a chat: one question in it, no rules and no character yet. */
export const openAgentSession = (folder?: string) =>
  request<AgentView>(`/agent-sessions${folder ? `?folder=${encodeURIComponent(folder)}` : ''}`, {
    method: 'POST',
  })
/** Answers an opened chat's first question. */
export const chooseAgentRules = (id: string, revision: number, rules: RulesLock) =>
  request<AgentView>(`/agent-sessions/${encodeURIComponent(id)}/control`, {
    method: 'POST',
    body: { revision, action: 'rules', rules },
  })
/** An opened chat's first message, with the sheet if there is one. It is
 * where the character begins. */
export function startAgentSession(id: string, revision: number, files: File[], instructions: string) {
  const formData = new FormData()
  for (const file of files) formData.append('files', file)
  formData.append('instructions', instructions)
  formData.append('revision', String(revision))
  return request<AgentView>(`/agent-sessions/${encodeURIComponent(id)}/files`, {
    method: 'POST',
    formData,
  })
}
export const controlAgent = (
  id: string,
  revision: number,
  action: string,
  text = '',
) =>
  request<AgentView>(`/agent-sessions/${encodeURIComponent(id)}/control`, {
    method: 'POST',
    body: { revision, action, text },
  })
