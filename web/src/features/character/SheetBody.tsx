import { useState } from 'react'

import type { Sheet } from '@/lib/api'
import {
  Card,
  Columns,
  Divider,
  Group,
  ProficiencyMark,
  SimpleGrid,
  Stack,
  Text,
} from '@/ui'

import type { Compendium } from './compendium'
import { IdentityTable } from './IdentityTable'
import { ProficienciesPanel } from './ProficienciesPanel'
import { SkillsPanel, SkillsToggle } from './SkillsPanel'
import { Vitals } from './Vitals'

import { abilitiesInOrder, abilityName, signed, titleCase } from '@/domain'


/**
 * Everything a sheet says about a character, without anything about who is
 * looking at it.
 *
 * Its own component because two screens draw it: its owner's, and the one a
 * group member opens for a character shared with their table. The difference
 * between those two pages is entirely in what surrounds this -- a link to the
 * event log, the list of what is still to choose -- and none of it is inside.
 * That is the point: the table sees the same sheet the owner does, drawn by the
 * same code, so the two cannot drift into disagreeing about what the character
 * is.
 *
 * `showUntrained` lives here rather than in SkillsPanel because the control
 * that flips it is the section's `aside`, drawn on the panel's title line by
 * Columns -- which puts the button and the block it hides in two subtrees.
 */
export function SheetBody({
  sheet: s,
  compendium,
}: {
  sheet: Sheet
  compendium: Compendium
}) {
  const { names, skills, proficiencies } = compendium
  const [showUntrained, setShowUntrained] = useState(true)
  const identity = s.identity

  return (
    <Stack gap="lg">
      <IdentityTable identity={identity} names={names} />

      {/*
        The saving throw lives in the ability's own card, under a rule.
        A save *is* an ability check the character may be trained in, and
        printing the two a screen apart made the reader carry a modifier
        between them -- while a separate six-row panel repeated the six labels
        the cards had already given. Merged, nothing can drift out of
        alignment, because there is no second list to align.
      */}
      <SimpleGrid cols={{ base: 3, sm: 6 }} spacing="sm">
        {abilitiesInOrder(abilitiesOnSheet(s)).map(([ability]) => {
          const score = s.abilities.scores[ability]
          const modifier = s.abilities.modifiers[ability]
          const save = s.savingThrows[ability]
          return (
            <Card key={ability} withBorder padding="sm" radius="md">
              <Stack gap={0} align="center">
                <Text size="xs" c="dimmed" tt="uppercase" title={abilityName(ability)}>
                  {ability}
                </Text>
                <Text fw={700} size="xl">
                  {modifier === undefined ? '--' : signed(modifier)}
                </Text>
                <Text size="xs" c="dimmed">
                  {score ?? '\u00a0'}
                </Text>
              </Stack>
              {save !== undefined && (
                <>
                  <Divider my={8} />
                  <Group gap={6} justify="center" wrap="nowrap">
                    <Text size="xs" c="dimmed">
                      Save
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

      <Vitals sheet={s} />

      <Columns
        cols={2}
        sections={[
          {
            key: 'skills',
            title: 'Skills',
            aside: (
              <SkillsToggle
                skills={s.skills}
                showing={showUntrained}
                onToggle={() => setShowUntrained((shown) => !shown)}
              />
            ),
            content: (
              <SkillsPanel
                skills={s.skills}
                catalog={skills}
                showUntrained={showUntrained}
              />
            ),
          },
          {
            key: 'proficiencies',
            title: 'Proficiencies',
            content: (
              <ProficienciesPanel
                proficiencies={s.proficiencies}
                catalog={proficiencies}
                proficiencyBonus={s.status.proficiencyBonus}
              />
            ),
          },
        ]}
      />

      <Columns
        cols={2}
        sections={[
          {
            key: 'traits',
            title: 'Traits and features',
            content: (
              <Stack gap="xs">
                <SlugList label="Traits" slugs={s.traits} empty="No racial traits." />
                <SlugList label="Features" slugs={s.features} empty="No class features." />
                <SlugList label="Languages" slugs={s.base.languages} empty="None." />
              </Stack>
            ),
          },
          {
            key: 'resources',
            title: 'Resources and gear',
            content: (
              <Stack gap="xs">
                {s.resources.class !== undefined &&
                  s.resources.class.map((pool) => (
                    <Text key={pool.key} size="sm">
                      {titleCase(pool.key ?? '')}: {pool.dice ?? pool.max}
                    </Text>
                  ))}
                {s.resources.spellSlots !== undefined && (
                  <Text size="sm">
                    Spell slots:{' '}
                    {Object.entries(s.resources.spellSlots)
                      .map(([level, pool]) => `${level}: ${pool.max}`)
                      .join(', ')}
                  </Text>
                )}
                <SlugList
                  label="Equipped"
                  slugs={s.equipment.equipped.map((stack) => stack.item ?? '')}
                  empty="Nothing worn or wielded."
                />
                <SlugList
                  label="Carried"
                  slugs={s.equipment.backpack.map((stack) =>
                    stack.count > 1 ? `${stack.item ?? ''} ×${stack.count}` : (stack.item ?? ''),
                  )}
                  empty="Empty."
                />
              </Stack>
            ),
          },
        ]}
      />
    </Stack>
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

function SlugList({
  label,
  slugs,
  empty,
}: {
  label: string
  slugs: string[] | undefined
  empty: string
}) {
  return (
    <div>
      <Text size="xs" c="dimmed" tt="uppercase">
        {label}
      </Text>
      <Text size="sm">
        {slugs !== undefined && slugs.length > 0
          ? slugs.map((slug) => titleCase(slug)).join(', ')
          : empty}
      </Text>
    </div>
  )
}
