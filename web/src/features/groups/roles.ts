import type { GroupRole } from '@/lib/api'
import type { MessageKey, Translate } from '@/lib/i18n'

/**
 * What each rank is called on screen -- as a key, not a word.
 *
 * The three ranks are the group's whole vocabulary and they are read in four
 * places: the list's badge, the member table, the invite sheet's picker and
 * the join prompt. One table so those four cannot come to disagree, and keys
 * rather than nouns so the table can stay a constant while the language is
 * React state.
 */
const ROLE_KEYS: Record<GroupRole, MessageKey> = {
  owner: 'role.owner',
  dm: 'role.dm',
  player: 'role.player',
}

/** What a rank is called, in the language on screen. */
export function roleLabel(t: Translate, role: GroupRole): string {
  return t(ROLE_KEYS[role])
}

/**
 * The same three ranks as they read inside a sentence: "invited you to join as
 * an owner".
 *
 * A second table rather than `roleLabel(...).toLowerCase()`, which is what this
 * was and which is wrong in two ways at once. English needs "an owner" beside
 * "a DM", so the article has to travel with the noun -- and lowercasing is not
 * a transformation every language agrees on anyway. Both problems belong to
 * whoever is writing the sentence, which is to say the catalogue.
 */
const ROLE_INLINE_KEYS: Record<GroupRole, MessageKey> = {
  owner: 'role.asOwner',
  dm: 'role.asDm',
  player: 'role.asPlayer',
}

/** A rank as it reads mid-sentence, article and all. */
export function roleInline(t: Translate, role: GroupRole): string {
  return t(ROLE_INLINE_KEYS[role])
}

/** Ranks high to low, mirroring domain.Role.Rank in the Go model. */
const RANK: Record<GroupRole, number> = { owner: 3, dm: 2, player: 1 }

/** Whether a rank is at least another one. */
export function atLeast(role: GroupRole, want: GroupRole): boolean {
  return RANK[role] >= RANK[want]
}
