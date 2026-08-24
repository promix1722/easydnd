import type { GroupRole } from '@/lib/api'

/** What each rank is called on screen. */
export const ROLE_LABELS: Record<GroupRole, string> = {
  owner: 'Owner',
  dm: 'DM',
  player: 'Player',
}

/** Ranks high to low, mirroring domain.Role.Rank in the Go model. */
const RANK: Record<GroupRole, number> = { owner: 3, dm: 2, player: 1 }

/** Whether a rank is at least another one. */
export function atLeast(role: GroupRole, want: GroupRole): boolean {
  return RANK[role] >= RANK[want]
}
