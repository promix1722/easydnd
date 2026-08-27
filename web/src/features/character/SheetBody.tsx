import type { Sheet } from '@/lib/api'
import {
  Bullet,
  Card,
  Divider,
  Group,
  ProficiencyMark,
  SectionDeck,
  SimpleGrid,
  Stack,
  Text,
  useIsDesktop,
} from '@/ui'
import type { DeckSection } from '@/ui'

import type { Compendium } from './compendium'
import { IdentityTable } from './IdentityTable'
import { ProficienciesPanel } from './ProficienciesPanel'
import { SkillsPanel } from './SkillsPanel'
import { Vitals } from './Vitals'

import { abilitiesInOrder, signed, titleCase } from '@/domain'
import { useT } from '@/lib/i18n'

import { abilityAbbr, abilityName } from './labels'


/**
 * Everything a sheet says about a character, without anything about who is
 * looking at it.
 *
 * Its own component because two screens draw it: its owner's, and the one a
 * group member opens for a character shared with their table. The difference
 * between those two pages is entirely in what surrounds this -- the owner's
 * way in to the questions still open -- and none of it is inside. That is the
 * point: the table sees the same sheet the owner does, drawn by the same code,
 * so the two cannot drift into disagreeing about what the character is.
 *
 * The whole sheet is one list of sections now, handed to `ui/SectionDeck`. On a
 * wide screen that draws what it always drew: identity, the ability cards and
 * the vitals across the page, then the four panels two abreast. On a phone the
 * same seven become a deck -- a row of tabs under the character's name and one
 * section on screen, swiped between. Nothing there opens or closes any more.
 * The accordion it replaces asked a player to open a panel to read it, and let
 * them leave four open at once; a sheet is seven things you leaf between, not
 * one thing with six footnotes.
 */
export function SheetBody({
  sheet: s,
  compendium,
}: {
  sheet: Sheet
  compendium: Compendium
}) {
  const { names, skills, proficiencies } = compendium
  const identity = s.identity

  /*
   * The one viewport question this file asks, and it is a question about
   * reading order rather than about layout.
   *
   * On a wide screen the sheet opens with who the character is and then what
   * everything about them is derived from, because there is room for both at
   * once and that is the order a sheet is written in. On a phone the section is
   * a slide you land on, and the first thing on it should be the thing reached
   * for mid-turn: the six modifiers, not the background.
   *
   * Swapped in the document rather than with `column-reverse`, which would do
   * it in CSS and leave the page saying one order and the screen showing
   * another. Two static blocks would survive that; a habit of it does not.
   */
  const t = useT()
  const isDesktop = useIsDesktop()
  const who = <IdentityTable identity={identity} names={names} />
  const abilities = <AbilityCards sheet={s} />

  const sections: DeckSection[] = [
    {
      key: 'identity',
      // One section rather than two, and the merge costs the wide screen
      // nothing: a `full` section is drawn bare, so two of them stacked and one
      // holding both are the same page. What it buys is the phone, where they
      // were the two thinnest slides on the sheet -- eight one-word fields, and
      // six cards -- and neither filled a screen on its own.
      //
      // Headed by nothing on a wide screen: the character's name is directly
      // above it and a second heading would say it twice. The title is what
      // names the tab on a phone, and `Main` is the one label here that names a
      // place rather than its contents -- deliberately, because the section
      // holds two things and a tab that named either would send a reader
      // looking for the other one somewhere else.
      title: t('sheet.main'),
      desktop: 'full',
      content: isDesktop ? (
        <>
          {who}
          {abilities}
        </>
      ) : (
        <>
          {abilities}
          {who}
        </>
      ),
    },
    {
      key: 'vitals',
      title: t('sheet.vitals'),
      desktop: 'full',
      content: <Vitals sheet={s} />,
    },
    {
      key: 'skills',
      title: t('sheet.skills'),
      desktop: 'panel',
      content: <SkillsPanel skills={s.skills} catalog={skills} />,
    },
    {
      key: 'proficiencies',
      title: t('sheet.proficiencies'),
      desktop: 'panel',
      content: (
        <ProficienciesPanel
          proficiencies={s.proficiencies}
          catalog={proficiencies}
          proficiencyBonus={s.status.proficiencyBonus}
        />
      ),
    },
    {
      key: 'traits',
      title: t('sheet.traitsAndFeatures'),
      desktop: 'panel',
      content: (
        <Stack gap="sm">
          <ItemList
            label={t('sheet.traits')}
            items={(s.traits ?? []).map(titleCase)}
            empty={t('sheet.noTraits')}
          />
          <ItemList
            label={t('sheet.features')}
            items={(s.features ?? []).map(titleCase)}
            empty={t('sheet.noFeatures')}
          />
          <ItemList
            label={t('sheet.languages')}
            items={(s.base.languages ?? []).map(titleCase)}
            empty={t('sheet.none')}
          />
        </Stack>
      ),
    },
    {
      key: 'resources',
      title: t('sheet.resourcesAndGear'),
      desktop: 'panel',
      content: (
        <Stack gap="sm">
          {/*
            The two pools are drawn only when the character has any, unlike the
            two below them. A backpack with nothing in it is a fact about the
            character; a rage counter on a character who cannot rage is not a
            fact at all, it is a row about somebody else.
          */}
          {s.resources.class !== undefined && s.resources.class.length > 0 && (
            <ItemList
              label={t('sheet.resources')}
              items={s.resources.class.map(
                (pool) => `${titleCase(pool.key ?? '')}: ${pool.dice ?? pool.max}`,
              )}
            />
          )}
          {Object.entries(s.resources.spellSlots ?? {}).length > 0 && (
            <ItemList
              label={t('sheet.spellSlots')}
              items={Object.entries(s.resources.spellSlots ?? {}).map(
                ([level, pool]) => t('sheet.slotLevel', { level, max: pool.max }),
              )}
            />
          )}
          <ItemList
            label={t('sheet.equipped')}
            items={s.equipment.equipped.map((stack) => titleCase(stack.item ?? ''))}
            empty={t('sheet.nothingWorn')}
          />
          <ItemList
            label={t('sheet.carried')}
            items={s.equipment.backpack.map((stack) =>
              stack.count > 1
                ? `${titleCase(stack.item ?? '')} ×${stack.count}`
                : titleCase(stack.item ?? ''),
            )}
            empty={t('sheet.empty')}
          />
        </Stack>
      ),
    },
  ]

  // "Character sheet" rather than the character's name: the name is already the
  // heading above this, and a landmark whose name changed per character would
  // give a screen-reader user a different table of contents on every sheet.
  return <SectionDeck label={t('sheet.label')} cols={2} sections={sections} />
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
}: {
  label: string
  items: string[]
  empty?: string
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
              <Bullet />
              <Text size="sm">{item}</Text>
            </Group>
          ))}
        </SimpleGrid>
      )}
    </Stack>
  )
}
