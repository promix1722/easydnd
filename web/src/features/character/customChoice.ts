import { createContext } from 'react'

/**
 * How a picker offers "something the rules do not have": the builder supplies
 * it, and a picker of one of these kinds ends its list with it. Null outside a
 * builder, where there is no character to keep a custom entry on.
 */
export const CustomChoice = createContext<((kind: string) => void) | null>(null)

/**
 * The choices whose answer may be an entry the player writes themselves, and
 * the kind of entry each one makes: a question's kind on the left, a custom
 * option's on the right. Cantrips and spells are offered by the spell tabs'
 * own list. That is the whole set -- a custom entry is always an answer to a
 * question the builder already asks, never a control of its own.
 */
export const CUSTOM_KINDS: Readonly<Record<string, string>> = {
  race: 'race',
  class: 'class',
  background: 'background',
  equipment: 'item',
}
