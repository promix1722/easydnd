import { Link, useParams } from 'react-router'

import { getSheet } from '@/lib/api'
import type { Sheet } from '@/lib/api'
import { useResource } from '@/lib/useResource'
import {
  Alert,
  Anchor,
  Badge,
  Button,
  Card,
  Columns,
  Group,
  Loader,
  SimpleGrid,
  Stack,
  Text,
  Title,
} from '@/ui'

import { abilityName, classLine, signed, titleCase } from '@/domain'

/** The character sheet. */
export function CharacterSheetScreen() {
  const { id = '' } = useParams()
  const sheet = useResource<Sheet>(`sheet:${id}`, (signal) => getSheet(id, signal))

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

  const s = sheet.data
  const identity = s.identity

  return (
    <Stack gap="lg">
      <Group justify="space-between" align="flex-start">
        <div>
          <Title order={2}>{identity.name || 'Unnamed'}</Title>
          <Text c="dimmed" size="sm">
            {[
              identity.race !== undefined ? titleCase(identity.race) : null,
              identity.background !== undefined ? titleCase(identity.background) : null,
              classLine(identity.classes),
            ]
              .filter((part) => part !== null && part !== '--')
              .join(' · ')}
          </Text>
        </div>
        <Group gap="xs">
          <Anchor component={Link} to={`/characters/${id}/log`}>
            <Button variant="subtle">Event log</Button>
          </Anchor>
          <Anchor component={Link} to={`/characters/${id}/build`}>
            <Button>Level up</Button>
          </Anchor>
        </Group>
      </Group>

      <SimpleGrid cols={{ base: 2, sm: 4 }} spacing="sm">
        <Stat label="Armor class" value={s.status.armorClass} />
        <Stat label="Hit points" value={`${s.base.hitPoints.current} / ${s.base.hitPoints.max}`} />
        <Stat label="Initiative" value={signed(s.status.initiative)} />
        <Stat label="Proficiency" value={signed(s.status.proficiencyBonus)} />
      </SimpleGrid>

      <SimpleGrid cols={{ base: 3, sm: 6 }} spacing="sm">
        {Object.entries(s.abilities.scores).map(([ability, score]) => (
          <Card key={ability} withBorder padding="sm" radius="md">
            <Stack gap={0} align="center">
              <Text size="xs" c="dimmed" tt="uppercase" title={abilityName(ability)}>
                {ability}
              </Text>
              <Text fw={700} size="xl">
                {signed(s.abilities.modifiers[ability] ?? 0)}
              </Text>
              <Text size="xs" c="dimmed">
                {score}
              </Text>
            </Stack>
          </Card>
        ))}
      </SimpleGrid>

      <Columns
        cols={2}
        sections={[
          {
            key: 'skills',
            title: 'Skills',
            content: (
              <Stack gap={4}>
                {Object.entries(s.skills)
                  .sort(([a], [b]) => a.localeCompare(b))
                  .map(([skill, state]) => (
                    <Group key={skill} justify="space-between" gap="xs">
                      <Group gap={6}>
                        <Text size="sm">{titleCase(skill)}</Text>
                        {state.proficiency === 'expertise' && (
                          <Badge size="xs" variant="light">
                            expertise
                          </Badge>
                        )}
                      </Group>
                      <Text size="sm" fw={500}>
                        {signed(state.bonus)}
                      </Text>
                    </Group>
                  ))}
                {Object.keys(s.skills).length === 0 && (
                  <Text size="sm" c="dimmed">
                    None yet.
                  </Text>
                )}
              </Stack>
            ),
          },
          {
            key: 'saves',
            title: 'Saving throws',
            content: (
              <Stack gap={4}>
                {Object.entries(s.savingThrows).map(([ability, state]) => (
                  <Group key={ability} justify="space-between" gap="xs">
                    <Group gap={6}>
                      <Text size="sm" tt="uppercase">
                        {ability}
                      </Text>
                      {state.proficient && (
                        <Badge size="xs" variant="light">
                          proficient
                        </Badge>
                      )}
                    </Group>
                    <Text size="sm" fw={500}>
                      {signed(state.bonus)}
                    </Text>
                  </Group>
                ))}
              </Stack>
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
                <SlugList label="Other proficiencies" slugs={s.proficiencies} empty="None." />
                <SlugList label="Languages" slugs={s.base.languages} empty="None." />
              </Stack>
            ),
          },
          {
            key: 'resources',
            title: 'Resources and gear',
            content: (
              <Stack gap="xs">
                {s.resources.hitDice !== undefined && s.resources.hitDice.length > 0 && (
                  <Text size="sm">
                    Hit Dice: {s.resources.hitDice.map((pool) => pool.dice).join(', ')}
                  </Text>
                )}
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

function Stat({ label, value }: { label: string; value: string | number }) {
  return (
    <Card withBorder padding="sm" radius="md">
      <Stack gap={0}>
        <Text size="xs" c="dimmed">
          {label}
        </Text>
        <Text fw={700} size="lg">
          {value}
        </Text>
      </Stack>
    </Card>
  )
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
