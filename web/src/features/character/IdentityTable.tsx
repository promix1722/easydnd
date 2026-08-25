import type { Identity } from '@/lib/api'
import { SimpleGrid, Stack, Text } from '@/ui'

import { titleCase } from '@/domain'

/**
 * Who the character is, as labelled pairs rather than a sentence.
 *
 * The sheet used to say this in one dimmed line under the name -- "Elf ·
 * Wizard 1" -- which reads well and answers badly. A line has no room for the
 * subrace or the subclass, and a reader looking for one of them has to know
 * the order the line was written in. Pairs are searchable: the label is where
 * the eye goes, and a field with nothing in it says so instead of silently
 * closing the gap.
 *
 * Every row is drawn even when empty, for that reason. A character with no
 * background has a Background row reading `--`, because "not chosen yet" is
 * the answer to the question and a missing row is not.
 */

/** A field that has not been answered yet, as against one that is nought. */
const UNSET = '--'

export interface IdentityTableProps {
  identity: Identity
  /**
   * The compendium's names, keyed `"<collection>:<slug>"` -- so that the table
   * prints "Half-Elf" rather than title-casing the slug into "Half Elf", and
   * prints it in the negotiated locale. Keyed by collection as well as slug
   * because two collections may use one slug and a bare slug map would let a
   * background rename a class.
   *
   * Null when those requests failed, which falls every name back to the slug.
   */
  names: Map<string, string> | null
}

export function IdentityTable({ identity, names }: IdentityTableProps) {
  /** The compendium's name for a slug, or the slug title-cased. */
  const named = (collection: string, slug: string | undefined): string =>
    slug === undefined || slug === ''
      ? UNSET
      : (names?.get(`${collection}:${slug}`) ?? titleCase(slug))

  // The first class is the one the character started as, and its subclass is
  // the one worth naming beside it; a multiclassed character gets the whole
  // line rather than a truncation of it.
  const classes = identity.classes ?? []
  const subclasses = classes.filter((entry) => entry.subclass !== undefined)

  const rows: [string, string][] = [
    ['Name', identity.name === '' ? UNSET : identity.name],
    ['Race', named('races', identity.race)],
    ['Subrace', named('subraces', identity.subrace)],
    ['Level', String(identity.level)],
    [
      'Class',
      classes.length === 0
        ? UNSET
        : classes.map((entry) => `${named('classes', entry.class)} ${entry.level}`).join(' · '),
    ],
    [
      'Subclass',
      subclasses.length === 0
        ? UNSET
        : subclasses.map((entry) => named('subclasses', entry.subclass)).join(' · '),
    ],
    ['Background', named('backgrounds', identity.background)],
    ['Experience', String(identity.experience)],
  ]

  return (
    <SimpleGrid cols={{ base: 1, sm: 2, lg: 4 }} spacing="md" verticalSpacing={4}>
      {rows.map(([label, value]) => (
        <Stack key={label} gap={0}>
          <Text size="xs" c="dimmed">
            {label}
          </Text>
          <Text size="sm" fw={500}>
            {value}
          </Text>
        </Stack>
      ))}
    </SimpleGrid>
  )
}
