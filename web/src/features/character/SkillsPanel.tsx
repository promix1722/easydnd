import type { CatalogSkill, Skill } from '@/lib/api'
import {
  Box,
  Button,
  Divider,
  Group,
  ProficiencyMark,
  SimpleGrid,
  Stack,
  Text,
} from '@/ui'

import { signed, titleCase } from '@/domain'

/**
 * The skills panel: every skill in the game, trained ones first.
 *
 * Two decisions worth stating, because both are about drawing eighteen rows
 * where there used to be six.
 *
 * The list is **all** of them. A sheet listing only what something trained is
 * the wrong sheet to read at a table: the question asked most often is what to
 * roll for a skill the character has no training in, and that is exactly the
 * row such a sheet leaves out. The untrained rows arrive from the server with
 * their bonuses already computed -- this component adds nothing up, in keeping
 * with `domain/format.ts`, because a browser deriving its own bonus would be a
 * second implementation of the rules to disagree with the first.
 *
 * They are ordered by **training rather than alphabetically**, which reverses
 * what this panel used to do. With six rows the alphabet was the only sequence
 * a reader could search; with eighteen, six of which matter, the useful
 * question is "what am I good at", and the answer should not be scattered down
 * a list of things nothing trained. Alphabetical still decides ties, so within
 * each block the order is stable and searchable.
 */

/** Highest training first. Ties fall through to the name. */
const RANK: Record<Skill['proficiency'], number> = {
  expertise: 3,
  proficient: 2,
  half: 1,
  none: 0,
}

interface Row {
  slug: string
  state: Skill
  name: string
  ability: string | undefined
}

/** How many skills nothing has trained. */
function untrainedIn(skills: Record<string, Skill>): number {
  return Object.values(skills).filter((state) => state.proficiency === 'none').length
}

export interface SkillsToggleProps {
  skills: Record<string, Skill>
  showing: boolean
  onToggle: () => void
}

/**
 * The panel's filter, drawn on the title's line rather than inside the body.
 *
 * Split from the panel because it belongs to the section rather than to the
 * content -- see `ColumnsSection.aside`. The two share `skills` rather than a
 * count passed between them, so there is no way for the button to promise
 * "Show all 18" over a panel holding some other number.
 *
 * Nothing to hide means no button. A character trained in all eighteen is not
 * a case worth a control that would do nothing.
 */
export function SkillsToggle({ skills, showing, onToggle }: SkillsToggleProps) {
  const untrained = untrainedIn(skills)
  if (untrained === 0) return null

  return (
    <Button variant="subtle" onClick={onToggle}>
      {showing ? 'Hide untrained' : `Show all ${Object.keys(skills).length}`}
    </Button>
  )
}

export interface SkillsPanelProps {
  /** The sheet's skills, keyed by slug. Drawn exactly as they arrive. */
  skills: Record<string, Skill>
  /**
   * The compendium's skills, for names and governing abilities. Null when
   * that request failed, which costs the ability tags and falls the names back
   * to the slug -- not a reason to refuse to draw the panel.
   */
  catalog: Map<string, CatalogSkill> | null
  /** Whether the untrained block is drawn. Owned by the screen, because the
   * control that flips it is drawn in the panel's header rather than here. */
  showUntrained: boolean
}

export function SkillsPanel({ skills, catalog, showUntrained }: SkillsPanelProps) {
  const rows: Row[] = Object.entries(skills)
    .map(([slug, state]) => ({
      slug,
      state,
      // The catalogue's name is in the negotiated locale and is the only one
      // that gets "Sleight of Hand" right; title-casing the slug capitalises
      // the "Of".
      name: catalog?.get(slug)?.name ?? titleCase(slug),
      ability: catalog?.get(slug)?.ability,
    }))
    .sort(
      (a, b) =>
        RANK[b.state.proficiency] - RANK[a.state.proficiency] || a.name.localeCompare(b.name),
    )

  const trained = rows.filter((row) => row.state.proficiency !== 'none')
  const untrained = rows.filter((row) => row.state.proficiency === 'none')
  const doubled = rows.filter((row) => row.state.proficiency === 'expertise').length

  if (rows.length === 0) {
    return (
      <Text size="sm" c="dimmed">
        None yet.
      </Text>
    )
  }

  return (
    <Stack gap="xs">
      <Text size="xs" c="dimmed">
        {trained.length === 0
          ? 'Nothing trained yet'
          : `${trained.length} proficient${doubled > 0 ? ` · ${doubled} with expertise` : ''}`}
      </Text>

      {trained.length > 0 && <SkillGrid rows={trained} />}

      {showUntrained && untrained.length > 0 && (
        <>
          {trained.length > 0 && <Divider />}
          <SkillGrid rows={untrained} />
        </>
      )}
    </Stack>
  )
}

/**
 * Two columns only from `lg` up. This panel is half the page beside the saving
 * throws, so splitting it at `md` would leave each column around 220px and put
 * "Animal Handling WIS +1" on two lines.
 */
function SkillGrid({ rows }: { rows: Row[] }) {
  return (
    <SimpleGrid cols={{ base: 1, lg: 2 }} spacing="md" verticalSpacing={4}>
      {rows.map((row) => (
        <SkillRow key={row.slug} row={row} />
      ))}
    </SimpleGrid>
  )
}

function SkillRow({ row }: { row: Row }) {
  const untrained = row.state.proficiency === 'none'
  // Dimming is the *second* channel, not the first: the mark's shape already
  // separates the four levels, so the panel still reads on a monochrome print
  // and to a reader who cannot tell the two greys apart. The mark takes this
  // colour with it, being drawn in currentColor.
  // Named explicitly rather than left undefined: `exactOptionalPropertyTypes`
  // rejects an optional prop passed as undefined, and the trained rows do want
  // a colour rather than no opinion -- the mark inherits it.
  const color = untrained ? 'dimmed' : 'var(--mantine-color-text)'

  return (
    <Group justify="space-between" gap="xs" wrap="nowrap">
      <Group gap={8} wrap="nowrap" style={{ minWidth: 0 }}>
        <Box c={color} style={{ display: 'flex' }}>
          <ProficiencyMark level={row.state.proficiency} />
        </Box>
        <Text size="sm" c={color} truncate>
          {row.name}
        </Text>
      </Group>
      <Group gap={8} wrap="nowrap">
        {row.ability !== undefined && (
          // Uppercased in the text rather than by `tt`, unlike the saving
          // throws. Those are the only other place printing a bare ability
          // slug, and leaving both lowercase would make eighteen skill rows
          // indistinguishable from the six saves to anything reading the
          // document rather than looking at it.
          <Text size="xs" c="dimmed">
            {row.ability.toUpperCase()}
          </Text>
        )}
        <Text size="sm" fw={500} c={color}>
          {signed(row.state.bonus)}
        </Text>
      </Group>
    </Group>
  )
}
