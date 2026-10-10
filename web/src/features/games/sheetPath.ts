/**
 * Where a character on a table opens, if it opens for you at all.
 *
 * Your own opens as your own sheet -- the one you can change -- and anybody
 * else's as the read the group grants: to whoever runs the table always, to a
 * player only once its owner opened it. One function because two lists link to
 * a character, the group's table and a game's roster, and they must agree.
 */
export function sheetPath(
  group: string, character: { id: string; owner_id?: string; public?: boolean }, me: string, master: boolean,
): string | undefined {
  if (character.owner_id !== undefined && character.owner_id === me) return `/characters/${encodeURIComponent(character.id)}`
  return master || character.public ? `/groups/${group}/characters/${character.id}` : undefined
}
