import type { CatalogProficiency } from '@/lib/api'
import { Group, ProficiencyMark, SimpleGrid, Stack, Text } from '@/ui'

import { signed, titleCase } from '@/domain'

import { useT } from '@/lib/i18n'
import type { MessageKey } from '@/lib/i18n'

/**
 * Everything the character is trained with that is not a skill or a save:
 * tools, weapons, armor.
 *
 * These used to be one comma-joined paragraph at the foot of "Traits and
 * features" -- "Light Armor, Simple Weapons, Longswords, Rapiers, Shortswords,
 * Hand Crossbows, Thieves Tools" -- which is a sentence to be read rather than
 * a list to be searched, and which put a tool a player rolls with beside a
 * racial trait they never touch again.
 *
 * **The bonus is printed on tools and on nothing else**, and the reason is
 * worth stating because it looks arbitrary. A tool check is an ability check,
 * but *which* ability depends on what is being attempted -- picking a lock
 * with thieves' tools is Dexterity, spotting a forgery with a forgery kit is
 * Intelligence -- so the only part of the number that is fixed in advance is
 * the proficiency bonus, and that is exactly what a sheet can usefully print.
 * A weapon's attack roll has a fixed ability, so the proficiency bonus alone
 * would be the *less* useful half of a number the sheet is not otherwise
 * showing; and armor proficiency adds nothing to any roll at all -- it only
 * stops the penalties. Nothing here is computed: the number is
 * `status.proficiencyBonus`, exactly as the server derived it.
 */

/** The types that are a tool in the sense the rules use the word. */
const TOOLS = new Set([
  'artisans-tools',
  'gaming-sets',
  'musical-instruments',
  'other-tools',
  'vehicles',
])

interface Section {
  key: string
  /** A message key: the table is a constant, the language is React state. */
  title: MessageKey
  /** Tool checks are the ones whose fixed part is the proficiency bonus. */
  bonus: boolean
  holds: (type: string | undefined) => boolean
}

/**
 * Tools lead. They are the ones with a number to read and the ones a player
 * reaches for mid-session; armor is settled when it is put on. "Other" catches
 * a type this client does not know, drawn rather than dropped -- an
 * unrecognised proficiency means the server and this client disagree about the
 * game, which is a thing to see rather than a thing to hide.
 */
const SECTIONS: Section[] = [
  { key: 'tools', title: 'proficiency.tools', bonus: true, holds: (type) => type !== undefined && TOOLS.has(type) },
  { key: 'weapons', title: 'proficiency.weapons', bonus: false, holds: (type) => type === 'weapons' },
  { key: 'armor', title: 'proficiency.armor', bonus: false, holds: (type) => type === 'armor' },
  {
    key: 'other',
    title: 'proficiency.other',
    bonus: false,
    holds: (type) => type === undefined || !TOOLS.has(type) && type !== 'weapons' && type !== 'armor',
  },
]

export interface ProficienciesPanelProps {
  /** The sheet's other proficiencies, as slugs. Skills and saves are not here. */
  proficiencies: string[] | undefined
  /**
   * The compendium's proficiencies, for names and types. Null when that
   * request failed, which costs the grouping and falls the names back to the
   * slug -- not a reason to refuse to draw the panel.
   */
  catalog: Map<string, CatalogProficiency> | null
  /** Printed against each tool. Derived by the server, never here. */
  proficiencyBonus: number
}

export function ProficienciesPanel({
  proficiencies,
  catalog,
  proficiencyBonus,
}: ProficienciesPanelProps) {
  const t = useT()
  const rows = (proficiencies ?? [])
    .map((slug) => ({
      slug,
      name: catalog?.get(slug)?.name ?? titleCase(slug),
      type: catalog?.get(slug)?.type,
    }))
    .sort((a, b) => a.name.localeCompare(b.name))

  if (rows.length === 0) {
    return (
      <Text size="sm" c="dimmed">
        {t('panel.noneYet')}
      </Text>
    )
  }

  return (
    <Stack gap="sm">
      {SECTIONS.map((section) => {
        const held = rows.filter((row) => section.holds(row.type))
        if (held.length === 0) return null

        return (
          <Stack key={section.key} gap={4}>
            <Text size="xs" c="dimmed" tt="uppercase">
              {t(section.title)}
            </Text>
            <SimpleGrid cols={{ base: 1, lg: 2 }} spacing="md" verticalSpacing={4}>
              {held.map((row) => (
                <Group key={row.slug} justify="space-between" gap="xs" wrap="nowrap">
                  <Group gap={8} wrap="nowrap" style={{ minWidth: 0 }}>
                    <ProficiencyMark level="proficient" />
                    <Text size="sm" truncate>
                      {row.name}
                    </Text>
                  </Group>
                  {section.bonus && (
                    <Text size="sm" fw={500}>
                      {signed(proficiencyBonus)}
                    </Text>
                  )}
                </Group>
              ))}
            </SimpleGrid>
          </Stack>
        )
      })}
    </Stack>
  )
}
