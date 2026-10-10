/**
 * Where a character on a table opens.
 *
 * Your own opens as your own sheet -- the one you can change -- and anybody
 * else's as the read the group grants. One function because two lists link to
 * a character, the group's table and a game's roster, and they must agree.
 */
export function sheetPath(group: string, character: { id: string; owner_id?: string }, me: string): string {
  return character.owner_id !== undefined && character.owner_id === me
    ? `/characters/${encodeURIComponent(character.id)}`
    : `/groups/${group}/characters/${character.id}`
}
