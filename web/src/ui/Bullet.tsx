export interface BulletProps {
  /** Rendered edge length, in pixels. Matches `ProficiencyMark`'s default. */
  size?: number
}

/**
 * An empty point, marking one item on a list and saying nothing else.
 *
 * It is deliberately the same ring `ProficiencyMark` draws for `level="none"`,
 * at the same diameter and the same weight, because the character sheet has
 * four lists on it and two of them are marked by that component. A list whose
 * items began at a different indent from the list beside it would read as a
 * different kind of thing, when what they are is the same kind of thing with
 * and without a training level to report. `Bullet.test.tsx` pins that the two
 * rings match, so they cannot drift apart in a later edit.
 *
 * It is **not** `ProficiencyMark level="none"`, and the difference is the whole
 * reason this exists: that component names itself "Not proficient" and carries
 * a tooltip explaining proficiency bonuses. Drawn beside "Darkvision" it would
 * be telling a screen-reader user something false about a racial trait. This
 * one is `aria-hidden`, like every other glyph here that sits beside a text
 * label already saying what the row is, and it is drawn in `currentColor` so it
 * dims with the row it belongs to.
 */
export function Bullet({ size = 12 }: BulletProps) {
  return (
    <svg
      viewBox="0 0 16 16"
      aria-hidden
      style={{ width: size, height: size, flex: 'none', display: 'block' }}
    >
      <circle
        cx="8"
        cy="8"
        r="5.5"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        opacity={0.55}
      />
    </svg>
  )
}
