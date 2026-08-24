import { Link, useNavigate } from 'react-router'

import { classLine } from '@/domain'
import { listCharacters } from '@/lib/api'
import { useResource } from '@/lib/useResource'
import { Alert, Anchor, Button, DataList, Group, Loader, Stack, Text, Title } from '@/ui'

/** The party. */
export function CharacterListScreen() {
  const navigate = useNavigate()
  const { data, error, loading, reload } = useResource('characters', (signal) =>
    listCharacters(signal),
  )

  const characters = data?.characters ?? []

  return (
    <Stack gap="md">
      <Group justify="space-between" align="flex-start">
        <div>
          <Title order={2}>Characters</Title>
          <Text c="dimmed" size="sm">
            Your party.
          </Text>
        </div>
        <Group gap="xs">
          <Button variant="default" onClick={() => void navigate('/characters/import')}>
            Import
          </Button>
          <Button onClick={() => void navigate('/characters/new')}>New character</Button>
        </Group>
      </Group>

      {error !== null && (
        <Alert color="red" title="Could not load your characters">
          <Stack gap="xs" align="flex-start">
            <Text size="sm">{error}</Text>
            <Button variant="light" onClick={reload}>
              Try again
            </Button>
          </Stack>
        </Alert>
      )}

      {loading ? (
        <Group gap="xs">
          <Loader size="sm" />
          <Text size="sm" c="dimmed">
            Loading...
          </Text>
        </Group>
      ) : (
        <DataList
          items={characters}
          getKey={(character) => character.id}
          columns={[
            {
              key: 'name',
              header: 'Name',
              primary: true,
              render: (character) => (
                <Anchor component={Link} to={`/characters/${character.id}`}>
                  {character.name || 'Unnamed'}
                </Anchor>
              ),
            },
            { key: 'level', header: 'Level', render: (character) => character.level || '--' },
            { key: 'classes', header: 'Classes', render: (character) => classLine(character.classes) },
          ]}
          empty="No characters yet. Make one."
        />
      )}
    </Stack>
  )
}
