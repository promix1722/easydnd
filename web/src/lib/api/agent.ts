import type { RulesLock } from './packs'
import type { CharacterEvent as Event, Sheet } from './characters'
import { request } from './client'

export interface AgentEvent {
  actions?: ('view' | 'edit' | 'save')[]
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
    | 'saved'
  revision: number
  characterId?: string
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
  sheet: Sheet
}
export const agentCapabilities = () =>
  request<{ enabled: boolean }>('/agent-capabilities')
export const listAgentSessions = () =>
  request<AgentSession[]>('/agent-sessions')
export const getAgentSession = (id: string) =>
  request<AgentView>(`/agent-sessions/${encodeURIComponent(id)}`)
export function createAgentSession(
  files: File[],
  instructions: string,
  folder?: string,
  rules?: RulesLock,
) {
  const formData = new FormData()
  for (const file of files) formData.append('files', file)
  formData.append('instructions', instructions)
  if (rules) formData.append('rules', JSON.stringify(rules))
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
export const editAgent = (id: string, revision: number, events: Event[]) =>
  request<AgentView>(`/agent-sessions/${encodeURIComponent(id)}/edit`, {
    method: 'POST',
    body: { revision, events },
  })
export const finalizeAgent = (id: string, revision: number) =>
  request<AgentView>(`/agent-sessions/${encodeURIComponent(id)}/finalize`, {
    method: 'POST',
    body: { revision },
  })

export function addAgentFiles(
  id: string,
  revision: number,
  files: File[],
  instructions: string,
) {
  const formData = new FormData()
  for (const file of files) formData.append('files', file)
  formData.append('revision', String(revision))
  formData.append('instructions', instructions)
  return request<AgentView>(`/agent-sessions/${encodeURIComponent(id)}/files`, {
    method: 'POST',
    formData,
  })
}
