export const AVATAR_CLASSES = ['barbarian', 'bard', 'cleric', 'druid', 'fighter', 'monk', 'paladin', 'ranger', 'rogue', 'sorcerer', 'warlock', 'wizard'] as const
// Keep this pool and its order fixed: IDs select a stable member of this artwork pool.
export const AVATAR_RANDOM = ['wolf', 'fox', 'owl', 'raven', 'bear', 'lion', 'dragon', 'stag', 'cat', 'serpent', 'turtle', 'bat', 'phoenix', 'griffin', 'kraken', 'skull', 'helmet', 'crown', 'mask', 'crystal', 'key', 'chalice', 'compass', 'moon'] as const
const pathFor = (name: string) => `/avatars/${name}.webp`

/** Starting class determines the emblem, including on multiclass characters. */
export function characterAvatar(classes?: readonly { class: string }[]): string | undefined {
  const name = classes?.[0]?.class.split('/').at(-1) ?? ''
  const matched = AVATAR_CLASSES.find((className) => className === name)
  return matched ? pathFor(matched) : undefined
}

/** Stable selections for accounts and classless NPCs share the random artwork pool. */
export function playerAvatar(id: string): string {
  let hash = 0
  for (const char of id) hash = (Math.imul(hash, 31) + char.codePointAt(0)!) >>> 0
  return `/avatars/random/${AVATAR_RANDOM[hash % AVATAR_RANDOM.length]!}.webp`
}
