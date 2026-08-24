import { request } from './client'

/**
 * Groups.
 *
 * Shapes mirror internal/api/http/v1/group, field names included -- the
 * envelope is snake_case there, so it is snake_case here rather than being
 * rewritten in transit.
 *
 * Nothing in here is called `Group`. `@/ui` already exports Mantine's layout
 * primitive under that name and every screen in this codebase uses it, so a
 * type of the same name would shadow it in exactly the files that need both.
 */

export type GroupRole = 'owner' | 'dm' | 'player'

/** A rank an invitation may offer. The owner is never one of them. */
export type InvitableRole = Extract<GroupRole, 'dm' | 'player'>

export interface GroupMember {
  user_id: string
  display_name: string
  role: GroupRole
  joined_at: string
  /** A guest: their session expires and they cannot come back to this seat. */
  anonymous: boolean
}

export interface GroupDetail {
  id: string
  name: string
  created_at: string
  /** The caller's own rank, which is what decides the controls on screen. */
  role: GroupRole
  members: GroupMember[]
}

export interface GroupSummary {
  id: string
  name: string
  created_at: string
  role: GroupRole
}

export interface InviteLink {
  token: string
  role: GroupRole
  expires_at: string
}

export interface InvitePreview {
  group_id: string
  group_name: string
  role: GroupRole
  invited_by?: string
  expires_at: string
  already_member: boolean
}

export function listGroups(signal?: AbortSignal): Promise<{ groups: GroupSummary[] }> {
  return request<{ groups: GroupSummary[] }>('/groups', signal ? { signal } : {})
}

export function getGroup(id: string, signal?: AbortSignal): Promise<GroupDetail> {
  return request<GroupDetail>(`/groups/${encodeURIComponent(id)}`, signal ? { signal } : {})
}

export function createGroup(name: string): Promise<GroupDetail> {
  return request<GroupDetail>('/groups', { method: 'POST', body: { name } })
}

export function renameGroup(id: string, name: string): Promise<GroupDetail> {
  return request<GroupDetail>(`/groups/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: { name },
  })
}

export function deleteGroup(id: string): Promise<void> {
  return request<void>(`/groups/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

export function createInvite(id: string, role: InvitableRole): Promise<InviteLink> {
  return request<InviteLink>(`/groups/${encodeURIComponent(id)}/invites`, {
    method: 'POST',
    body: { role },
  })
}

/**
 * Changes a member's rank. Asking for 'owner' hands the group over, and the
 * caller becomes a DM in the same step.
 */
export function setMemberRole(id: string, user: string, role: GroupRole): Promise<GroupDetail> {
  return request<GroupDetail>(
    `/groups/${encodeURIComponent(id)}/members?user=${encodeURIComponent(user)}`,
    { method: 'PATCH', body: { role } },
  )
}

/** Removes somebody. Passing your own id is how you leave. */
export function removeMember(id: string, user: string): Promise<void> {
  return request<void>(
    `/groups/${encodeURIComponent(id)}/members?user=${encodeURIComponent(user)}`,
    { method: 'DELETE' },
  )
}

/**
 * Reads an invitation without acting on it.
 *
 * The token goes in the body, never in the URL: nginx logs the whole request
 * line, and this one is usable for a day. The client keeps it in a URL
 * fragment, which no browser sends to any server.
 */
export function previewInvite(token: string, signal?: AbortSignal): Promise<InvitePreview> {
  return request<InvitePreview>('/invites/preview', {
    method: 'POST',
    body: { token },
    ...(signal ? { signal } : {}),
  })
}

export function acceptInvite(token: string): Promise<GroupDetail> {
  return request<GroupDetail>('/invites/accept', { method: 'POST', body: { token } })
}
