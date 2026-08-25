import { useState } from 'react'
import { Link, useNavigate } from 'react-router'

import type { GroupSummary } from '@/lib/api'
import { createGroup, deleteGroup, listGroups, removeMember, renameGroup } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import { useAction } from '@/lib/useAction'
import { useResource } from '@/lib/useResource'
import {
  ACTION_ICON_SIZE,
  ACTION_SIZE,
  Alert,
  Anchor,
  Badge,
  Button,
  DataList,
  Group,
  IconLogout,
  IconPencil,
  IconPlus,
  IconTrash,
  Loader,
  ModalSheet,
  Stack,
  Text,
  TextInput,
  Title,
} from '@/ui'

import { atLeast, ROLE_LABELS } from './roles'

/** The tables this account sits at. */
export function GroupListScreen() {
  const navigate = useNavigate()
  const { data, error, loading, reload } = useResource('groups', (signal) => listGroups(signal))

  const { user } = useAuth()
  const me = user?.id ?? ''

  const [creating, setCreating] = useState(false)
  const [name, setName] = useState('')
  const [renaming, setRenaming] = useState<GroupSummary | null>(null)
  const [deleting, setDeleting] = useState<GroupSummary | null>(null)
  const [newName, setNewName] = useState('')
  const create = useAction(createGroup)
  const rename = useAction(renameGroup)
  const destroy = useAction(deleteGroup)
  const leave = useAction(removeMember)

  async function act(work: Promise<unknown | null>) {
    if ((await work) === null) return
    reload()
  }

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
      <Title order={2}>Groups</Title>

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
            {
              key: 'actions',
              header: '',
              render: (group) => (
                <Group gap="xs" justify="flex-end" wrap="nowrap">
                  {/* Leaving comes first: it is not an edit of the group, and
                      putting it between Rename and Delete reads as though it
                      were one of them. The owner cannot walk away from their
                      own table, and nobody else may delete it, so each rank
                      gets exactly one of these two. */}
                  {group.role !== 'owner' && (
                    <Button
                      size={ACTION_SIZE}
                      variant="subtle"
                      leftSection={<IconLogout size={ACTION_ICON_SIZE} />}
                      onClick={() => void act(leave.run(group.id, me))}
                    >
                      Leave
                    </Button>
                  )}
                  {atLeast(group.role, 'dm') && (
                    <Button
                      size={ACTION_SIZE}
                      variant="subtle"
                      leftSection={<IconPencil size={ACTION_ICON_SIZE} />}
                      onClick={() => {
                        setNewName(group.name)
                        setRenaming(group)
                      }}
                    >
                      Rename
                    </Button>
                  )}
                  {group.role === 'owner' && (
                    <Button
                      size={ACTION_SIZE}
                      variant="subtle"
                      color="red"
                      leftSection={<IconTrash size={ACTION_ICON_SIZE} />}
                      onClick={() => setDeleting(group)}
                    >
                      Delete
                    </Button>
                  )}
                </Group>
              ),
            },
          ]}
          empty="No groups yet. Make one, or open an invitation link somebody sent you."
        />
      )}

      {/* Under the table and to the left: adding a row belongs beneath the
          rows, not in the heading, which is about the section rather than
          about what you can put in it. */}
      <Group>
        <Button
          size={ACTION_SIZE}
          variant="light"
          leftSection={<IconPlus size={ACTION_ICON_SIZE} />}
          onClick={() => setCreating(true)}
        >
          New group
        </Button>
      </Group>

      <ModalSheet
        opened={renaming !== null}
        onClose={() => setRenaming(null)}
        title="Rename this group"
      >
        <Stack gap="sm">
          <TextInput
            label="Name"
            value={newName}
            error={rename.fields.find((field) => field.field === 'name')?.message}
            onChange={(event) => setNewName(event.currentTarget.value)}
            data-autofocus
          />
          {rename.error !== null && (
            <Alert color="red" title="Could not rename it">
              {rename.error}
            </Alert>
          )}
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setRenaming(null)}>
              Cancel
            </Button>
            <Button
              loading={rename.pending}
              onClick={() => {
                const target = renaming
                setRenaming(null)
                if (target !== null) void act(rename.run(target.id, newName))
              }}
            >
              Rename
            </Button>
          </Group>
        </Stack>
      </ModalSheet>

      <ModalSheet
        opened={deleting !== null}
        onClose={() => setDeleting(null)}
        title="Delete this group"
      >
        <Stack gap="sm">
          {/* Named in full: it takes the whole table with it, and nobody at it
              can undo that. */}
          <Text size="sm">
            {deleting?.name} goes, along with everything shared with it and every game played
            at it. The characters themselves stay yours.
          </Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setDeleting(null)}>
              Cancel
            </Button>
            <Button
              color="red"
              loading={destroy.pending}
              onClick={() => {
                const target = deleting
                setDeleting(null)
                if (target !== null) void act(destroy.run(target.id))
              }}
            >
              Delete
            </Button>
          </Group>
        </Stack>
      </ModalSheet>

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
