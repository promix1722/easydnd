import { useState } from 'react'
import { Link, useNavigate } from 'react-router'

import { createGroup, listGroups } from '@/lib/api'
import { useAction } from '@/lib/useAction'
import { useResource } from '@/lib/useResource'
import {
  Alert,
  Anchor,
  Badge,
  Button,
  DataList,
  Group,
  Loader,
  ModalSheet,
  Stack,
  Text,
  TextInput,
  Title,
} from '@/ui'

import { ROLE_LABELS } from './roles'

/** The tables this account sits at. */
export function GroupListScreen() {
  const navigate = useNavigate()
  const { data, error, loading, reload } = useResource('groups', (signal) => listGroups(signal))

  const [creating, setCreating] = useState(false)
  const [name, setName] = useState('')
  const create = useAction(createGroup)

  const groups = data?.groups ?? []

  // A group is one text field, so it opens a sheet rather than earning a route
  // of its own the way a six-ability character form does.
  async function submit() {
    const created = await create.run(name)
    if (created === null) return
    setCreating(false)
    setName('')
    await navigate(`/groups/${created.id}`)
  }

  return (
    <Stack gap="md">
      <Group justify="space-between" align="flex-start">
        <div>
          <Title order={2}>Groups</Title>
          <Text c="dimmed" size="sm">
            The tables you play at.
          </Text>
        </div>
        <Button onClick={() => setCreating(true)}>New group</Button>
      </Group>

      {error !== null && (
        <Alert color="red" title="Could not load your groups">
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
          items={groups}
          getKey={(group) => group.id}
          columns={[
            {
              key: 'name',
              header: 'Name',
              primary: true,
              render: (group) => (
                <Anchor component={Link} to={`/groups/${group.id}`}>
                  {group.name}
                </Anchor>
              ),
            },
            {
              key: 'role',
              header: 'Your role',
              render: (group) => <Badge variant="light">{ROLE_LABELS[group.role]}</Badge>,
            },
          ]}
          empty="No groups yet. Make one, or open an invitation link somebody sent you."
        />
      )}

      <ModalSheet opened={creating} onClose={() => setCreating(false)} title="New group">
        <Stack gap="sm">
          <TextInput
            label="Name"
            placeholder="Wednesday Night"
            value={name}
            onChange={(event) => setName(event.currentTarget.value)}
            error={create.fields.find((field) => field.field === 'name')?.message}
            data-autofocus
          />
          {create.error !== null && (
            <Alert color="red" title="Could not create the group">
              {create.error}
            </Alert>
          )}
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setCreating(false)}>
              Cancel
            </Button>
            <Button loading={create.pending} onClick={() => void submit()}>
              Create
            </Button>
          </Group>
        </Stack>
      </ModalSheet>
    </Stack>
  )
}
