import { Tooltip } from '@mantine/core'

/**
 * How trained a character is in one skill or save, as a single glyph.
 *
 * A skill row has to answer two questions at once -- what do I add, and is
 * that number trained -- and the bonus alone answers only the first. A rogue
 * with Dexterity 16 reads +3 for Acrobatics whether they are proficient or
 * not; only at level 5 do the numbers separate. So the training level needs a
 * mark of its own, and it needs to be the thing the eye lands on first when
 * eighteen rows are on screen and six of them matter.
 *
 * The four levels are drawn as one shape filling in: an empty ring, a half
 * disc, a full disc, and a disc inside a second ring. That last one is the
 * only one that is not "more of the same", which is right -- Expertise is not
 * more training, it is the bonus counted twice, and a reader should be able to
 * spot the doubled rows without reading a word.
 *
 * Colour is deliberately *not* what carries this. The glyphs differ in shape,
 * so the panel survives a monochrome print and a red/green colour blindness
 * alike; the row's own dimming is a second, redundant channel.
 */
export type ProficiencyLevel = 'none' | 'half' | 'proficient' | 'expertise'

/** What each level is called, and what it does to the bonus. */
const DESCRIPTIONS: Record<ProficiencyLevel, string> = {
  none: 'Not proficient',
  half: 'Half proficiency -- half the bonus, rounded down',
  proficient: 'Proficient -- your proficiency bonus applies',
  // Worded as PromptCard words the choice that grants it ("Double your
  // proficiency in ..."), so the sheet and the question that filled it in
  // describe the same thing the same way.
  expertise: 'Expertise -- your proficiency bonus, doubled',
}

export interface ProficiencyMarkProps {
  level: ProficiencyLevel
  /** Rendered edge length, in pixels. */
  size?: number
}

/**
 * Drawn in `currentColor`, which is the opposite of what `DragonMark` does and
 * for the opposite reason. That is a two-tone badge carrying its own field so
 * it survives either colour scheme; this sits inline in a row of text under
 * `defaultColorScheme="auto"` and has to take the row's colour with it --
 * dimmed when the row is dimmed, inked when it is not. A literal here would
 * be the one thing on the row that ignored the theme.
 */
export function ProficiencyMark({ level, size = 12 }: ProficiencyMarkProps) {
  const description = DESCRIPTIONS[level]

  return (
    <Tooltip label={description} withArrow>
      {/*
        Named with `aria-label` rather than a <title> child, which is the one
        place this departs from DragonMark. A <title> is a text node, and
        eighteen of these share a panel where each row's text is read as a
        unit -- "Stealth DEX +7" would come back with a sentence about
        proficiency bonuses wedged into the middle of it. The accessible name
        is identical either way.
      */}
      <svg
        viewBox="0 0 16 16"
        role="img"
        aria-label={description}
        style={{ width: size, height: size, flex: 'none', display: 'block' }}
      >

        {/* The ring every level shares, so the glyphs sit on one baseline and
            one diameter however full they are. */}
        <circle
          cx="8"
          cy="8"
          r={level === 'expertise' ? 4.5 : 5.5}
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
          opacity={level === 'none' ? 0.55 : 1}
        />

        {/* Proficient and Expertise fill it; half fills the left half only.
            A half disc rather than a smaller dot: "half" is about the bonus
            being halved, and a shape cut in two says that where a size
            difference would only read as a lighter version of proficient. */}
        {level === 'proficient' && <circle cx="8" cy="8" r="3" fill="currentColor" />}
        {level === 'expertise' && <circle cx="8" cy="8" r="2.25" fill="currentColor" />}
        {level === 'half' && <path d="M 8 3 A 5 5 0 0 0 8 13 Z" fill="currentColor" />}

        {/* The outer ring, Expertise only: the bonus counted a second time. */}
        {level === 'expertise' && (
          <circle cx="8" cy="8" r="7.25" fill="none" stroke="currentColor" strokeWidth="1.5" />
        )}
      </svg>
    </Tooltip>
  )
}
