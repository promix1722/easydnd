import type { Icon } from '@tabler/icons-react'
import { IconDice5, IconShield, IconUsers, IconWand } from '@tabler/icons-react'

import type { MessageKey } from '@/lib/i18n'

/**
 * The three sections of the app, declared once.
 *
 * This used to be `shell/nav.ts`, and it moved for a reason worth knowing: a
 * breadcrumb starts at the section it is in, so every *screen* now needs the
 * same label and glyph the navbar draws -- and `features/` may not import
 * `@/shell`. `lib/` could not hold it either, being denied the icon package.
 * `ui/` is the one layer both the chrome and the screens can see, so the table
 * came here whole rather than being copied, which is the outcome the old
 * file's own comment existed to prevent.
 */
export interface Section {
  /** Where the section links: the navbar's entry, and the first crumb's. */
  to: string
  /**
   * What to call it -- a message key, not a word.
   *
   * The table is a constant and the language is React state, so this cannot
   * hold the noun itself: it is read by the navbar, the tab bar and the first
   * crumb of every trail, and each of those translates it where it draws it.
   * That the three go on agreeing is the whole reason the table is here rather
   * than copied into each of them.
   */
  label: MessageKey
  /** Drawn wherever the section is named, and nowhere else. */
  icon: Icon
  /**
   * Path prefixes the section owns but does not link to.
   *
   * Characters is why this exists rather than a prefix match on `to`: its list
   * is `/`, and `/` as a prefix matches the entire app. Splitting where a
   * section *links* from what it *owns* is what lets a character sheet light
   * Characters in the navbar and start its trail there, without `/` swallowing
   * `/account`, `/login` and every 404 along with it.
   *
   * Before the split there was nothing to split: `activeNavPath` matched on
   * `to` alone, so the price of `/` not swallowing everything was that a
   * character sheet lit nothing at all and the phone's dropdown fell back to
   * the word "Menu". That was defensible while the navbar was the only thing
   * claiming to say where you were. It stopped being defensible the moment a
   * trail on the same page said "Characters".
   */
  owns: readonly string[]
}

/**
 * The order is the order they are drawn in, and the glyphs are deliberate
 * rather than obvious -- note that the people are *not* on Groups:
 *
 * - **Characters** is literally where the people are. Each row is somebody you
 *   play, so the people glyph is theirs.
 * - **Groups** is a table, a banner, an allegiance -- the heraldic reading,
 *   which is a shield. It is a group of *players*, and drawing it with people
 *   would say the same thing as the section above it.
 * - **Games** is one sitting. Dice.
 * - **Spells** is the compendium's browsable half. A wand: the thing a spell
 *   is cast with, and the one glyph in the row that is nobody's person, table
 *   or sitting.
 */
export const SECTIONS: readonly Section[] = [
  { to: '/', label: 'section.characters', icon: IconUsers, owns: ['/characters'] },
  { to: '/groups', label: 'section.groups', icon: IconShield, owns: ['/groups'] },
  { to: '/games', label: 'section.games', icon: IconDice5, owns: ['/games'] },
  { to: '/spells', label: 'section.spells', icon: IconWand, owns: ['/spells'] },
]

/**
 * Which section a path belongs to, or null.
 *
 * A nested route keeps its parent section -- exact matching alone blanks the
 * navigation as soon as anybody opens a detail page. `/` is matched exactly and
 * never as a prefix; everything else matches itself or a path beneath it. The
 * trailing slash in the prefix test is load-bearing: without it `/groupsfoo`
 * would be answered with Groups.
 */
export function sectionFor(pathname: string): Section | null {
  return (
    SECTIONS.find(
      (section) =>
        pathname === section.to ||
        section.owns.some((owned) => pathname === owned || pathname.startsWith(`${owned}/`)),
    ) ?? null
  )
}
