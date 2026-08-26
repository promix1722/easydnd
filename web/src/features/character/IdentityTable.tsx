import type { Identity } from '@/lib/api'
import { Card, SimpleGrid, Stack, Text } from '@/ui'

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

  /**
   * Four columns, each a field and the one that qualifies it.
   *
   * The pairing is the layout rather than a consequence of it, which is why
   * these are nested rather than eight entries in one flat grid. A subrace only
   * means anything as a qualification of a race, a subclass of a class, a level
   * of the character it belongs to and experience of the background it was
   * earned past -- so each sits under its own, at every width. Flat, the
   * pairing held at four columns and broke at two: "Class" landed under "Name"
   * and "Subrace" under "Class", which reads as a claim about the character.
   */
  const columns: { key: string; fields: [string, string][] }[] = [
    {
      key: 'name',
      fields: [
        ['Name', identity.name === '' ? UNSET : identity.name],
        ['Level', String(identity.level)],
      ],
    },
    {
      key: 'race',
      fields: [
        ['Race', named('races', identity.race)],
        ['Subrace', named('subraces', identity.subrace)],
      ],
    },
    {
      key: 'class',
      fields: [
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
      ],
    },
    {
      key: 'background',
      fields: [
        ['Background', named('backgrounds', identity.background)],
        ['Experience', String(identity.experience)],
      ],
    },
  ]

  // Two columns on a phone rather than one. Eight fields down a single column
  // is most of a screen for what is the shortest thing on the sheet, and these
  // values are a word each.
  //
  // Drawn in a card, and the same card the ability scores and the vitals use --
  // `withBorder padding="xs"`. That is an alignment fix rather than decoration.
  // A bordered card insets what is inside it by its border and its padding, so
  // a bare table sitting above a row of cards puts its labels that much to the
  // left of theirs: two columns of small dimmed labels down one page, not
  // lining up. Matching the container is what lines them up exactly, where
  // padding the table by hand would be the same measurement written down as a
  // number -- and that number has already changed once, when the cards went
  // from `sm` to `xs` to give a phone back a few pixels a screen.
  return (
    <Card withBorder padding="xs" radius="md">
      <SimpleGrid
        cols={{ base: 2, lg: 4 }}
        spacing={{ base: 'sm', lg: 'md' }}
        verticalSpacing={{ base: 'xs', lg: 'sm' }}
      >
        {columns.map((column) => (
          <Stack key={column.key} gap={4}>
            {column.fields.map(([label, value]) => (
              <Stack key={label} gap={0}>
                <Text size="xs" c="dimmed">
                  {label}
                </Text>
                <Text size="sm" fw={500}>
                  {value}
                </Text>
              </Stack>
            ))}
          </Stack>
        ))}
      </SimpleGrid>
    </Card>
  )
}
