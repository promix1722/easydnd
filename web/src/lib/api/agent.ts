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
// One long-poll request: what this tab holds goes up, and the answer is the
// whole session if its revision moved, the events past `after` if only those
// did, or nothing (204) once the server has waited. See docs/long-polling.md.
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
export function createAgentSession(
  files: File[],
  instructions: string,
  folder?: string,
  rules?: RulesLock,
  unattended = false,
) {
  const formData = new FormData()
  for (const file of files) formData.append('files', file)
  formData.append('instructions', instructions)
  if (rules) formData.append('rules', JSON.stringify(rules))
  if (unattended) formData.append('unattended', 'true')
  return request<AgentView>(
    `/agent-sessions${folder ? `?folder=${encodeURIComponent(folder)}` : ''}`,
    { method: 'POST', formData },
  )
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
