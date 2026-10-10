import { useState } from 'react'
import type { ReactNode } from 'react'
import { useSearchParams } from 'react-router'

import { bySlug } from '@/lib/api'
import type { Change, Item, Sheet } from '@/lib/api'
import {
  BlockList,
  Card,
  Divider,
  Group,
  Markdown,
  Panel,
  ProficiencyMark,
  SimpleGrid,
  Stack,
  TabDeck,
  Text,
  Title,
  joinProse,
} from '@/ui'
import type { DeckPanel } from '@/ui'

import { CustomNotes } from './CustomNotes'
import { IdentityTable } from './IdentityTable'
import { ProficienciesPanel } from './ProficienciesPanel'
import { ResourcePools } from './ResourcePools'
import { SheetActions } from './SheetActions'
import { SheetEquipment, SheetItems } from './SheetEquipment'
import { SkillsPanel } from './SkillsPanel'
import { Vitals } from './Vitals'
import { SheetSpells } from './SheetSpells'

import { abilitiesInOrder, signed, titleCase } from '@/domain'
import { useT } from '@/lib/i18n'

import { abilityAbbr, abilityName } from './labels'


/**
 * Everything a sheet says about a character, without anything about who is
 * looking at it.
 *
 * Its own component because two screens draw it: its owner's, and the one a
 * group member opens for a character shared with their table. The table sees
 * the same sheet the owner does, drawn by the same code, so the two cannot
 * drift into disagreeing about what the character is. The one difference is
 * `onEquipment`: only the owner's screen passes it, and without it nothing on
 * the sheet can be pressed.
 *
 * The sheet is a handful of tabs at every width -- Overview, Actions, Spells,
 * Resources, Equipment, Items, Custom -- handed to `ui/TabDeck`, which draws a tab row on a wide screen
 * and the same row over a swiped deck on a phone. A handful rather than a tab per
 * section: a sheet is read by what you are doing (looking someone up, taking a
 * turn, casting, gearing up), not by which table of the rulebook a number is in.
 */
export function SheetBody({
  sheet: s,
  onEquipment,
  characterId,
  onChanged,
  pending = false,
}: {
  sheet: Sheet
  /** Posts an inventory edit. Absent on a sheet that is only being read. */
  onEquipment?: (changes: Change[]) => void
  /** The character being edited; lets its owner prepare spells. Absent on a sheet only being read. */
  characterId?: string
  /** Called after the Spells tab wrote something, so the screen reloads the sheet. */
  onChanged?: () => void
  pending?: boolean
}) {
  // The sheet arrives with what its slugs mean: names in `catalogNames`, and
  // in `catalog` the entries a panel reads more of. Nothing here asks the
  // compendium for anything, and a sheet without them -- one a write echoed
  // back -- draws title-cased slugs.
  const catalog = s.catalog
  const skills = catalog ? bySlug(catalog.skills) : null
  const proficiencies = catalog ? bySlug(catalog.proficiencies ?? []) : null
  const items = bySlug<Item>([...(catalog?.magicItems ?? []), ...(catalog?.equipment ?? [])])
  const names = new Map(Object.entries(s.catalogNames ?? {}))
  const identity = s.identity
  // A plain-number scaling value -- Extra Attacks: 1, Maneuvers: 3 -- is counted
  // off at the table like any pool, so it is drawn as one: that many marks,
  // spent in a game. A die, a fraction or a word has no number of uses and
  // stays a line under Scaling values. The id is the one the game tracker
  // spends it by.
  const scaling = Object.entries(s.resources.parameters ?? {})
  const countable = (value: (typeof scaling)[number][1]) =>
    (value.number ?? 0) > 0 && !value.dice && !value.text && !value.rational && value.boolean === undefined
  // Hit Dice are a vital, drawn there; everything else spendable is on Resources.
  const pools = [
    ...Object.values(s.resources.pools ?? {}).filter((pool) => pool.max > 0 && pool.group !== 'hit-dice')
      .sort((a, b) => a.name.localeCompare(b.name)),
    ...scaling.filter(([, value]) => countable(value))
      .map(([slug, value]) => ({ id: `scaling/${slug}`, name: value.name, group: 'scaling', max: value.number ?? 0, used: 0 }))
      .sort((a, b) => a.name.localeCompare(b.name)),
  ]
  // What is left -- a die, a fraction, a word -- is not counted off, it is read:
  // "Sneak Attack: 1d6". It is the size of a feature, so it is drawn with the
  // features rather than in a box of its own.
  const valued = new Map(scaling.map(([, value]) => value)
    // A zero is a value the class has not reached yet -- Brutal Critical before
    // ninth level -- and "0" on a sheet reads as a thing the character has.
    .filter((value) => !countable(value) && value.boolean !== false && (value.number !== 0 || !!value.dice || !!value.text || !!value.rational || value.boolean === true))
    .map((value) => {
      const amount = value.dice || value.text || (value.rational ? `${value.rational.numerator}/${value.rational.denominator}` : value.boolean ? '' : String(value.number))
      return [value.name, amount ? `${value.name}: ${amount}` : value.name] as const
    }))

  const t = useT()
  // The tab is in the URL, so the way back from an item's page or from
  // writing a custom one lands on the tab it was opened from.
  const [params, setParams] = useSearchParams()
  const tab = params.get('tab') ?? 'overview'
  const setTab = (next: string) => setParams((was) => { was.set('tab', next); return was }, { replace: true })
  const who = <IdentityTable identity={identity} names={names} />
  const abilities = <AbilityCards sheet={s} />
  const named = (collection: string, slug: string) =>
    names.get(`${collection}:${slug}`) ?? items.get(slug)?.name ?? titleCase(slug)
  // A feature that has a value says it on its own line -- "Sneak Attack: 1d6"
  // in place of "Sneak Attack" -- and a value no feature is named for follows them.
  const [opened, setOpened] = useState<string | null>(null)
  const rows = (collection: 'traits' | 'features' | 'languages', slugs: readonly string[]): ListRow[] => {
    const prose = bySlug(catalog?.[collection] ?? [])
    return slugs.map((slug) => ({ key: `${collection}:${slug}`, label: named(collection, slug), desc: prose.get(slug)?.desc }))
  }
  const features = rows('features', s.features ?? [])
  const featureLines: ListRow[] = [
    ...features.map((row) => ({ ...row, label: valued.get(row.label) ?? row.label })),
    ...[...valued].filter(([name]) => !features.some((row) => row.label === name)).map(([, line]) => line).sort()
      .map((line) => ({ key: `value:${line}`, label: line })),
  ]
  const headed = (title: string, content: ReactNode) => (
    <Panel>
      <Stack gap="sm">
        <Title order={4}>{title}</Title>
        {content}
      </Stack>
    </Panel>
  )

  const panels: DeckPanel[] = [
    {
      value: 'overview',
      label: t('sheet.overview'),
      content: (
        <Stack gap="lg">
          {/* Who the character is, then what everything about them is derived from: the order a sheet is written in, at every width. */}
          {who}
          {abilities}
          <Vitals sheet={s} />
          <SimpleGrid cols={{ base: 1, md: 2 }} spacing="lg">
            {headed(t('sheet.skills'), <SkillsPanel skills={s.skills} catalog={skills} />)}
            {headed(t('sheet.proficiencies'), (
              <ProficienciesPanel
                proficiencies={s.proficiencies}
                catalog={proficiencies}
                proficiencyBonus={s.status.proficiencyBonus}
              />
            ))}
            {headed(t('sheet.traitsAndFeatures'), (
              <Stack gap="sm">
                <ItemList
                  label={t('sheet.traits')}
                  items={rows('traits', s.traits ?? [])}
                  empty={t('sheet.noTraits')}
                  open={opened}
                  onOpen={setOpened}
                />
                <ItemList
                  label={t('sheet.features')}
                  items={featureLines}
                  empty={t('sheet.noFeatures')}
                  open={opened}
                  onOpen={setOpened}
                />
                <ItemList
                  label={t('sheet.languages')}
                  items={rows('languages', s.base.languages ?? [])}
                  empty={t('sheet.none')}
                  open={opened}
                  onOpen={setOpened}
                />
              </Stack>
            ))}
            {/* In the player's own words, so each answer is a line of prose rather than a list entry. */}
            {headed(t('stage.personality'), (
              <Stack gap="sm">
                {([
                  [t('written.personalityTrait'), identity.personalityTraits],
                  [t('written.ideal'), identity.ideals],
                  [t('written.bond'), identity.bonds],
                  [t('written.flaw'), identity.flaws],
                ] as const).map(([label, lines]) => (
                  <Stack key={label} gap={4}>
                    <Text size="xs" c="dimmed" tt="uppercase">{label}</Text>
                    {lines?.length
                      ? lines.map((line, at) => <Text key={at} size="sm" style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{line}</Text>)
                      : <Text size="sm" c="dimmed">{t('sheet.none')}</Text>}
                  </Stack>
                ))}
              </Stack>
            ))}
          </SimpleGrid>
        </Stack>
      ),
    },
    {
      value: 'actions',
      label: t('sheet.actions'),
      content: (
        <Stack gap="md">
          <Panel>
            <Stack gap="md">
              <SheetActions
                actions={s.actions ?? []}
                entries={bySlug(catalog?.actions ?? [])}
                pools={s.resources.pools ?? {}}
              />
            </Stack>
          </Panel>
        </Stack>
      ),
    },
  ]
  if (s.spells.sources?.length) panels.push({
    value: 'spells', label: t('sheet.spells'),
    content: <SheetSpells sheet={s} characterId={characterId} onChanged={onChanged} />,
  })
  /*
   * What a class hands out to spend, on a tab of its own so the action list is
   * only actions: every pool, and every scaling value that is a plain number
   * and so can be counted off. Drawn only for a character with any, because a
   * tab about somebody else's class is not a fact at all.
   */
  if (pools.length > 0) panels.push({
    value: 'resources',
    label: t('sheet.resources'),
    content: (
      <Stack gap="md">
        {headed(t('sheet.consumables'), <ResourcePools pools={pools} />)}
      </Stack>
    ),
  })
  panels.push({
    value: 'equipment',
    label: t('sheet.equipment'),
    content: (
      <SheetEquipment
        equipment={s.equipment}
        items={items}
        actions={s.actions ?? []}
        name={(slug) => named('equipment', slug)}
        lookup={named}
        disabled={pending}
        {...(onEquipment ? { onChange: onEquipment } : {})}
      />
    ),
  }, {
    value: 'items',
    label: t('sheet.items'),
    content: (
      <SheetItems
        equipment={s.equipment}
        items={items}
        name={(slug) => named('equipment', slug)}
        lookup={named}
        disabled={pending}
        {...(onEquipment ? { onChange: onEquipment } : {})}
      />
    ),
  })
  // Last, and only where there is something to read or somebody to write it:
  // a sheet shared with a table has no empty tab to open.
  if (characterId !== undefined || s.customOptions?.some((option) => option.kind === 'note')) {
    panels.push({
      value: 'custom',
      label: t('sheet.custom'),
      content: <Panel>
        <CustomNotes options={s.customOptions} disabled={pending}
          {...(characterId !== undefined ? { characterId } : {})} {...(onChanged ? { onChanged } : {})} />
      </Panel>,
    })
  }

  // "Character sheet" rather than the character's name: the name is already the
  // heading above this, and a landmark whose name changed per character would
  // give a screen-reader user a different table of contents on every sheet.
  const shown = panels.some((panel) => panel.value === tab) ? tab : 'overview'
  return <TabDeck bar label={t('sheet.label')} panels={panels} value={shown} onChange={setTab} />
}


/**
 * The six abilities, in the order a sheet prints them, each with its saving
 * throw under a rule.
 *
 * A save *is* an ability check the character may be trained in, and printing
 * the two a screen apart made the reader carry a modifier between them -- while
 * a separate six-row panel repeated the six labels the cards had already given.
 * Merged, nothing can drift out of alignment, because there is no second list
 * to align.
 */
function AbilityCards({ sheet: s }: { sheet: Sheet }) {
  const t = useT()
  return (
    <SimpleGrid cols={{ base: 3, sm: 6 }} spacing={{ base: 'xs', sm: 'sm' }}>
      {abilitiesInOrder(abilitiesOnSheet(s)).map(([ability]) => {
        const score = s.abilities.scores[ability]
        const modifier = s.abilities.modifiers[ability]
        const save = s.savingThrows[ability]
        return (
          <Card key={ability} withBorder padding="xs" radius="md">
            <Stack gap={0} align="center">
              <Text size="xs" c="dimmed" title={abilityName(t, ability)}>
                {abilityAbbr(t, ability)}
              </Text>
              <Text fw={700} size="xl">
                {modifier === undefined ? '--' : signed(modifier)}
              </Text>
              <Text size="xs" c="dimmed">
                {score ?? ' '}
              </Text>
            </Stack>
            {save !== undefined && (
              <>
                <Divider my={8} />
                <Group gap={6} justify="center" wrap="nowrap">
                  <Text size="xs" c="dimmed">
                    {t('sheet.save')}
                  </Text>
                  <ProficiencyMark level={save.proficient ? 'proficient' : 'none'} size={10} />
                  <Text size="sm" fw={500}>
                    {signed(save.bonus)}
                  </Text>
                </Group>
              </>
            )}
          </Card>
        )
      })}
    </SimpleGrid>
  )
}


/**
 * Every ability the projection mentions at all, as a set to be ordered.
 *
 * The union of two projections rather than just the scores, because the card
 * is now the only place either one is drawn. Scores and saving throws arrive
 * as separate objects and neither is guaranteed to hold all six: dropping an
 * ability that has a save but no score would silently swallow a number the
 * server sent, which is the failure the missing-score rule was never about.
 * A card with no score prints no score; it still prints the save.
 */
function abilitiesOnSheet(sheet: Sheet): Record<string, true> {
  const present: Record<string, true> = {}
  for (const ability of Object.keys(sheet.abilities.scores)) present[ability] = true
  for (const ability of Object.keys(sheet.savingThrows)) present[ability] = true
  return present
}

interface ListRow {
  key: string
  label: string
  /** The entry's prose, where the catalogue has any. */
  desc?: string[] | undefined
}

/**
 * One labelled group of a panel: a heading, then a row per entry.
 *
 * These used to be comma-joined sentences -- "Darkvision, Fey Ancestry, Skill
 * Versatility" on one line under a label -- which is a thing to read rather
 * than a thing to search, and which put the twelfth item and the first in the
 * same visual object.
 *
 * One column at every width. It was two from `lg`, and a grid row is as tall
 * as its tallest cell: one name long enough to wrap pushed its short neighbour
 * apart from the rows around it. And a row now opens where it stands onto what
 * the entry says, which needs the width of the panel to be read in.
 *
 * A row the catalogue has no prose for is a statement, as `BlockList` draws
 * one: a name with nothing to open is not a control.
 *
 * `open` is the caller's, so the three groups of a panel share it and one
 * description is open in the panel rather than one in each group.
 */
function ItemList({
  label,
  items,
  empty,
  open,
  onOpen,
}: {
  label: string
  items: ListRow[]
  empty: string
  open: string | null
  onOpen: (key: string | null) => void
}) {
  return (
    <Stack gap={4}>
      <Text size="xs" c="dimmed" tt="uppercase">
        {label}
      </Text>
      {items.length === 0 ? (
        <Text size="sm" c="dimmed">
          {empty}
        </Text>
      ) : (
        <BlockList
          outlined
          open={open}
          onOpen={onOpen}
          items={items.map((item) => ({
            key: item.key,
            header: <Text size="sm">{item.label}</Text>,
            body: item.desc?.length ? <Markdown size="sm">{joinProse(item.desc)}</Markdown> : undefined,
          }))}
        />
      )}
    </Stack>
  )
}
