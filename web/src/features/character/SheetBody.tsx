import { useState } from 'react'
import type { ReactNode } from 'react'

import { SpellIcon } from '@/features/spells/spellIcon'
import { bySlug } from '@/lib/api'
import type { Change, Item, Sheet } from '@/lib/api'
import {
  Bullet,
  Card,
  Divider,
  Group,
  Panel,
  ProficiencyMark,
  SimpleGrid,
  Stack,
  TabDeck,
  Text,
  Title,
  useIsDesktop,
} from '@/ui'
import type { DeckPanel } from '@/ui'

import { IdentityTable } from './IdentityTable'
import { ProficienciesPanel } from './ProficienciesPanel'
import { ResourcePools } from './ResourcePools'
import { SheetEquipment } from './SheetEquipment'
import { SkillsPanel } from './SkillsPanel'
import { Vitals } from './Vitals'
import { spellChoiceName } from './promptNames'
import { collectionOfKind, kindOf, slugOf } from '@/domain'

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
 * The sheet is four tabs at every width -- Overview, Actions, Spells,
 * Equipment -- handed to `ui/TabDeck`, which draws a tab row on a wide screen
 * and the same row over a swiped deck on a phone. Four rather than a tab per
 * section: a sheet is read by what you are doing (looking someone up, taking a
 * turn, casting, gearing up), not by which table of the rulebook a number is in.
 */
export function SheetBody({
  sheet: s,
  onEquipment,
  pending = false,
}: {
  sheet: Sheet
  /** Posts an inventory edit. Absent on a sheet that is only being read. */
  onEquipment?: (changes: Change[]) => void
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
  const spells = bySlug(catalog?.spells ?? [])
  const names = new Map(Object.entries(s.catalogNames ?? {}))
  const identity = s.identity
  // Hit Dice are a vital, drawn there; everything else spendable is on Actions.
  const pools = Object.values(s.resources.pools ?? {}).filter((pool) => pool.max > 0 && pool.group !== 'hit-dice')
    .sort((a, b) => a.name.localeCompare(b.name))
  const parameters = Object.values(s.resources.parameters ?? {})
    .filter((value) => value.boolean !== false)
    .map((value) => {
      const amount = value.dice || value.text || (value.rational ? `${value.rational.numerator}/${value.rational.denominator}` : value.boolean ? '' : String(value.number))
      return amount ? `${value.name}: ${amount}` : value.name
    })
    .sort()

  /*
   * The one viewport question this file asks, and it is a question about
   * reading order rather than about layout.
   *
   * On a wide screen the sheet opens with who the character is and then what
   * everything about them is derived from, because there is room for both at
   * once and that is the order a sheet is written in. On a phone the tab is
   * a slide you land on, and the first thing on it should be the thing reached
   * for mid-turn: the six modifiers, not the background.
   *
   * Swapped in the document rather than with `column-reverse`, which would do
   * it in CSS and leave the page saying one order and the screen showing
   * another.
   */
  const t = useT()
  const isDesktop = useIsDesktop()
  const [tab, setTab] = useState('overview')
  const who = <IdentityTable identity={identity} names={names} />
  const abilities = <AbilityCards sheet={s} />
  const named = (collection: string, slug: string) =>
    names.get(`${collection}:${slug}`) ?? (collection === 'spells' ? spells : items).get(slug)?.name ?? titleCase(slug)
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
          {isDesktop ? <>{who}{abilities}</> : <>{abilities}{who}</>}
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
                  items={(s.traits ?? []).map((slug) => named('traits', slug))}
                  empty={t('sheet.noTraits')}
                />
                <ItemList
                  label={t('sheet.features')}
                  items={(s.features ?? []).map((slug) => named('features', slug))}
                  empty={t('sheet.noFeatures')}
                />
                <ItemList
                  label={t('sheet.languages')}
                  items={(s.base.languages ?? []).map((slug) => named('languages', slug))}
                  empty={t('sheet.none')}
                />
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
          {/*
            What the server derived, as it derived it. It does not yet turn an
            equipped weapon into an attack, so a sheet with none says so.
          */}
          <Panel>
            <Stack gap="md">
              <ItemList
                label={t('sheet.actions')}
                items={(s.actions ?? []).map((action) =>
                  [action.name, action.toHit === undefined ? '' : signed(action.toHit), action.damage, action.uses, action.notes]
                    .filter(Boolean).join(' · '))}
                empty={t('sheet.noActions')}
              />
              {/*
                Scaling values only -- a Sneak Attack die, an aura's range -- drawn
                only when there is one, because a row about somebody else's class
                is not a fact at all.
              */}
              {parameters.length > 0 && <ItemList label={t('sheet.resources')} items={parameters} />}
            </Stack>
          </Panel>
          {pools.length > 0 && headed(t('sheet.consumables'), <ResourcePools pools={pools} />)}
        </Stack>
      ),
    },
  ]
  if (s.spells.sources?.length) panels.push({
    value: 'spells', label: t('sheet.spells'), content: <Stack gap="md">
      {s.spells.sources.map((source) => <Panel key={source.source}><Stack gap="xs">
        <Text fw={600}>{source.source.startsWith('rule:custom-spells') ? t('spellRules.custom') : named(collectionOfKind(kindOf(source.source)) ?? 'classes', slugOf(source.source))}</Text>
        {(['cantrips', 'known', 'spellbook', 'prepared', 'arcanum', 'mastery'] as const).map((mode) => {
          const list = source[mode] ?? []
          return list.length === 0 ? null : <ItemList key={mode} label={spellChoiceName(t, mode === 'cantrips' ? 'cantrip' : mode, mode === 'prepared' ? source.preparationLimit ?? list.length : list.length)} items={list.map((slug) => named('spells', slug))} mark={(at) => <SpellIcon icon={spells.get(list[at] ?? '')?.icon} />} />
        })}
      </Stack></Panel>)}
    </Stack>,
  })
  panels.push({
    value: 'equipment',
    label: t('sheet.equipment'),
    content: (
      <SheetEquipment
        equipment={s.equipment}
        items={items}
        name={(slug) => named('equipment', slug)}
        disabled={pending}
        {...(onEquipment ? { onChange: onEquipment } : {})}
      />
    ),
  })

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

/**
 * One labelled group of a panel: a heading, then a row per entry.
 *
 * These used to be comma-joined sentences -- "Darkvision, Fey Ancestry, Skill
 * Versatility" on one line under a label -- which is a thing to read rather
 * than a thing to search, and which put the twelfth item and the first in the
 * same visual object. It is the same argument that took the proficiencies out
 * of the foot of this panel and gave them one of their own, so it is drawn the
 * same way: `ProficienciesPanel`'s grid, one column on a phone and two from
 * `lg`, where a panel is half the page and a name is short.
 *
 * `empty` is optional because the two absences are different. A backpack with
 * nothing in it is worth a row saying so -- "Empty." is the answer to the
 * question. A group that does not apply to this character at all is not asked
 * about, and its caller leaves it out rather than passing a message here.
 *
 * Every row is marked by a `Bullet`, which is the empty ring `ProficiencyMark`
 * draws for an untrained skill. That is what makes the four lists on this sheet
 * one thing seen four times: the same glyph, the same gap, the same indent, and
 * the mark carrying a training level where there is one to carry.
 */
function ItemList({
  label,
  items,
  empty,
  mark,
}: {
  label: string
  items: string[]
  empty?: string
  /** Drawn in place of the bullet when it yields something: a spell's artwork. */
  mark?: (at: number) => ReactNode
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
        <SimpleGrid cols={{ base: 1, lg: 2 }} spacing="md" verticalSpacing={4}>
          {items.map((item, at) => (
            <Group key={`${item}-${at}`} gap={8} wrap="nowrap" style={{ minWidth: 0 }}>
              {mark?.(at) ?? <Bullet />}
              <Text size="sm">{item}</Text>
            </Group>
          ))}
        </SimpleGrid>
      )}
    </Stack>
  )
}
