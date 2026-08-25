import { useState } from 'react'
import { Link, useParams } from 'react-router'

import { bySlug, getCollection, getPrompts, getSheet } from '@/lib/api'
import type { CatalogProficiency, CatalogSkill, Entry, Prompt, Sheet } from '@/lib/api'
import { useResource } from '@/lib/useResource'
import {
  Alert,
  Anchor,
  Button,
  Card,
  Columns,
  Divider,
  Group,
  Loader,
  ProficiencyMark,
  SimpleGrid,
  Stack,
  Text,
  Title,
} from '@/ui'

import { OutstandingChoices } from './OutstandingChoices'
import { IdentityTable } from './IdentityTable'
import { ProficienciesPanel } from './ProficienciesPanel'
import { SkillsPanel, SkillsToggle } from './SkillsPanel'
import { Vitals } from './Vitals'

import { abilitiesInOrder, abilityName, answerable, signed, titleCase } from '@/domain'

/** The sheet, and what the character has not decided yet. */
interface SheetView {
  sheet: Sheet
  /**
   * Null when `/prompts` failed. The list of what is left is worth having and
   * is not worth losing the sheet over -- a sheet that refuses to draw because
   * a second request failed is a page that fails for a reason it is not about.
   */
  prompts: Prompt[] | null
  /**
   * The compendium's skills, for the panel's names and governing abilities.
   * Null when that request failed -- same bargain as `prompts` above: the
   * sheet is worth drawing with title-cased slugs and no ability tags, and is
   * not worth losing to a second request.
   */
  skills: Map<string, CatalogSkill> | null
  /** The compendium's proficiencies, for the panel's names and types. Null on
   * the same terms as `skills` above. */
  proficiencies: Map<string, CatalogProficiency> | null
  /** Compendium names for the identity table, keyed "<collection>:<slug>". */
  names: Map<string, string> | null
}

/**
 * The collections the identity table names things out of.
 *
 * Fetched together and flattened into one map keyed by collection *and* slug:
 * two collections may use the same slug, and a bare slug map would let a
 * background quietly rename a class. Each is session-cached, and the build
 * screen has usually warmed all five before a sheet is ever opened.
 */
const NAMED = ['races', 'subraces', 'classes', 'subclasses', 'backgrounds'] as const

async function namesOf(): Promise<Map<string, string> | null> {
  try {
    const collections = await Promise.all(
      NAMED.map((collection) => getCollection<Entry>(collection)),
    )
    const names = new Map<string, string>()
    collections.forEach((entries, at) => {
      for (const entry of entries) names.set(`${NAMED[at]}:${entry.slug}`, entry.name)
    })
    return names
  } catch {
    // The sheet is worth drawing with title-cased slugs; it is not worth
    // losing to a compendium request, as with the prompts above.
    return null
  }
}

/** The character sheet. */
export function CharacterSheetScreen() {
  const { id = '' } = useParams()
  // Held here rather than inside SkillsPanel because the control that flips it
  // is the section's `aside`, drawn on the panel's title line by Columns --
  // which puts the button and the block it hides in two different subtrees.
  // Declared above the early returns below, as a hook has to be.
  const [showUntrained, setShowUntrained] = useState(true)
  const sheet = useResource<SheetView>(`sheet:${id}`, async (signal) => {
    const [projected, prompts, skills, proficiencies, names] = await Promise.all([
      getSheet(id, signal),
      getPrompts(id, signal).then(
        // Advancement is not offered anywhere in this client while level-up
        // does not work; see domain/stages.ts.
        (response) => (response.prompts ?? []).filter((prompt) => answerable(prompt.group)),
        () => null,
      ),
      // Session-cached, so this is one request for the whole visit however
      // many sheets are opened.
      getCollection<CatalogSkill>('skills').then(bySlug, () => null),
      getCollection<CatalogProficiency>('proficiencies').then(bySlug, () => null),
      namesOf(),
    ])
    return { sheet: projected, prompts, skills, proficiencies, names }
  })

  if (sheet.loading) {
    return (
      <Group gap="xs">
        <Loader size="sm" />
        <Text size="sm" c="dimmed">
          Projecting the sheet...
        </Text>
      </Group>
    )
  }
  if (sheet.error !== null || sheet.data === null) {
    return (
      <Alert color="red" title="Could not load this character">
        <Stack gap="xs" align="flex-start">
          <Text size="sm">{sheet.error ?? 'Unknown error'}</Text>
          <Button variant="light" onClick={sheet.reload}>
            Try again
          </Button>
        </Stack>
      </Alert>
    )
  }

  const s = sheet.data.sheet
  const identity = s.identity
  const outstanding = sheet.data.prompts ?? []

  return (
    <Stack gap="lg">
      <Group justify="space-between" align="flex-start">
        <Title order={2}>{identity.name || 'Unnamed'}</Title>
        <Anchor component={Link} to={`/characters/${id}/log`}>
          <Button variant="subtle">Event log</Button>
        </Anchor>
      </Group>

      <IdentityTable identity={identity} names={sheet.data.names} />

      {/*
        An unfinished character says so on the page it is looked at most.
        The same `/prompts` response the build screen's tabs draw, named by the
        same `choiceName` -- there is no second notion anywhere in this client
        of what is still outstanding, and so no way for the sheet and the build
        screen to disagree about it. Here it is a statement of what is left and
        the way in is the link below; there each choice is a block that opens.
      */}
      {outstanding.length > 0 && (
        <Alert color="blue" title="Still to choose">
          <Stack gap="xs" align="flex-start">
            <OutstandingChoices prompts={outstanding} />
            <Anchor component={Link} to={`/characters/${id}/build`}>
              <Button variant="light">Answer these</Button>
            </Anchor>
          </Stack>
        </Alert>
      )}

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
                catalog={sheet.data.skills}
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
                catalog={sheet.data.proficiencies}
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
