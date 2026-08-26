import { useState } from 'react'
import { useNavigate, useParams } from 'react-router'

import type { GroupDetail, GroupMember } from '@/lib/api'
import { deleteGroup, getGroup, removeMember, renameGroup, setMemberRole } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import { useAction } from '@/lib/useAction'
import { useResource } from '@/lib/useResource'
import {
  ACTION_ICON_SIZE,
  ACTION_SIZE,
  Alert,
  Badge,
  Button,
  DataList,
  Group,
  IconLogout,
  IconPencil,
  IconTrash,
  IconUserPlus,
  Menu,
  ModalSheet,
  Page,
  pageState,
  Stack,
  TabRow,
  Text,
  TextInput,
  Tooltip,
} from '@/ui'

import { TablePanel } from '../games'

import { InviteSheet } from './InviteSheet'
import { atLeast, ROLE_LABELS } from './roles'

/** One table: who is at it, and what this account may do about that. */
export function GroupScreen() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const { user } = useAuth()
  const { data, error, loading, reload } = useResource(`group:${id}`, (signal) =>
    getGroup(id, signal),
  )

  const [tab, setTab] = useState('members')
  const [renaming, setRenaming] = useState(false)
  const [newName, setNewName] = useState('')
  const rename = useAction(renameGroup)
  const [inviting, setInviting] = useState(false)
  const [handover, setHandover] = useState<GroupMember | null>(null)
  const change = useAction(setMemberRole)
  const remove = useAction(removeMember)
  const destroy = useAction(deleteGroup)

  const state = pageState(
    { data, error, loading },
    { title: 'Could not load this group', fallback: 'That group is not there.', onRetry: reload },
  )

  // The header renders either way, which is the change: this used to replace
  // the whole screen with a spinner or an alert, so a group that would not load
  // left you on a page that did not say where you were. The trail is still
  // drawn; only the body is stood in for. The name is null until it arrives, and
  // `Page` draws a placeholder that keeps its accessible name.
  if (state.kind !== 'ready' || data === null) {
    return <Page trail={[{ label: data?.name ?? null }]} state={state} />
  }

  const group: GroupDetail = data
  const me = user?.id ?? ''
  const isOwner = group.role === 'owner'
  const canManage = atLeast(group.role, 'dm')

  async function act(work: Promise<unknown | null>) {
    if ((await work) === null) return
    reload()
  }

  async function leave() {
    if ((await remove.run(group.id, me)) === null) return
    await navigate('/groups', { replace: true })
  }

  async function close() {
    if ((await destroy.run(group.id)) === null) return
    await navigate('/groups', { replace: true })
  }

  const failure = change.error ?? remove.error ?? destroy.error ?? rename.error

  return (
    <Page
      trail={[{ label: group.name }]}
      badge={<Badge variant="light">{ROLE_LABELS[group.role]}</Badge>}
      actions={
        <>
          {isOwner ? (
            // Rendered and disabled rather than hidden. A control that is not
            // there teaches nothing; one that is there with a reason teaches
            // the rule the first time somebody reaches for it.
            <Tooltip label="Make somebody else the owner first, or delete the group">
              <Button
                size={ACTION_SIZE}
                variant="subtle"
                leftSection={<IconLogout size={ACTION_ICON_SIZE} />}
                data-disabled
                onClick={(event) => event.preventDefault()}
              >
                Leave
              </Button>
            </Tooltip>
          ) : (
            <Button
              size={ACTION_SIZE}
              variant="subtle"
              leftSection={<IconLogout size={ACTION_ICON_SIZE} />}
              loading={remove.pending}
              onClick={() => void leave()}
            >
              Leave
            </Button>
          )}
          {canManage && (
            <Button
              size={ACTION_SIZE}
              variant="subtle"
              leftSection={<IconPencil size={ACTION_ICON_SIZE} />}
              onClick={() => {
                setNewName(group.name)
                setRenaming(true)
              }}
            >
              Rename
            </Button>
          )}
          {isOwner && (
            <Button
              size={ACTION_SIZE}
              color="red"
              variant="subtle"
              leftSection={<IconTrash size={ACTION_ICON_SIZE} />}
              loading={destroy.pending}
              onClick={() => void close()}
            >
              Delete
            </Button>
          )}
        </>
      }
    >
      <Stack gap="md">
        {failure !== null && (
          <Alert color="red" title="That did not work">
            {failure}
          </Alert>
        )}

        <TabRow
          tabs={[
            { value: 'members', label: 'Members' },
            { value: 'characters', label: 'Characters' },
          ]}
          value={tab}
          onChange={setTab}
        >
          {tab === 'characters' && <TablePanel groupId={group.id} role={group.role} />}
          {tab === 'members' && (
        <DataList
          items={group.members}
          getKey={(member) => member.user_id}
          columns={[
            {
              key: 'name',
              header: 'Member',
              primary: true,
              render: (member) => (
                <Group gap="xs">
                  <Text size="sm">{member.display_name || 'Unnamed'}</Text>
                  {member.user_id === me && (
                    <Badge size="xs" variant="light">
                      You
                    </Badge>
                  )}
                  {member.anonymous && (
                    <Tooltip label="A guest. Their session expires, and they cannot return to this seat.">
                      <Badge size="xs" color="gray" variant="outline">
                        Guest
                      </Badge>
                    </Tooltip>
                  )}
                </Group>
              ),
            },
            {
              key: 'role',
              header: 'Role',
              render: (member) => ROLE_LABELS[member.role],
            },
            {
              key: 'actions',
              header: '',
              render: (member) => {
                // The owner is nobody's to unseat, including their own, and a
                // player manages no one. The server enforces all of this; the
                // point of repeating it here is to not offer a button that is
                // going to come back 403.
                if (member.role === 'owner' || member.user_id === me || !canManage) return null
                return (
                  <Menu position="bottom-end" withinPortal>
                    <Menu.Target>
                      <Button size={ACTION_SIZE} variant="subtle">
                        Manage
                      </Button>
                    </Menu.Target>
                    <Menu.Dropdown>
                      {isOwner && member.role === 'player' && (
                        <Menu.Item onClick={() => void act(change.run(group.id, member.user_id, 'dm'))}>
                          Make DM
                        </Menu.Item>
                      )}
                      {isOwner && member.role === 'dm' && (
                        <Menu.Item
                          onClick={() => void act(change.run(group.id, member.user_id, 'player'))}
                        >
                          Make player
                        </Menu.Item>
                      )}
                      {isOwner && <Menu.Item onClick={() => setHandover(member)}>Make owner</Menu.Item>}
                      <Menu.Item
                        color="red"
                        onClick={() => void act(remove.run(group.id, member.user_id))}
                      >
                        Remove from group
                      </Menu.Item>
                    </Menu.Dropdown>
                  </Menu>
                )
              },
            },
          ]}
          empty="Nobody here yet."
        />
          )}
          {tab === 'members' && canManage && (
            <Group mt="md">
              <Button
                size={ACTION_SIZE}
                variant="light"
                leftSection={<IconUserPlus size={ACTION_ICON_SIZE} />}
                onClick={() => setInviting(true)}
              >
                Invite
              </Button>
            </Group>
          )}
        </TabRow>

        <ModalSheet
          opened={renaming}
          onClose={() => setRenaming(false)}
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
            <Group justify="flex-end">
              <Button variant="default" onClick={() => setRenaming(false)}>
                Cancel
              </Button>
              <Button
                loading={rename.pending}
                onClick={() => {
                  setRenaming(false)
                  void act(rename.run(group.id, newName))
                }}
              >
                Rename
              </Button>
            </Group>
          </Stack>
        </ModalSheet>

        <InviteSheet
          groupId={group.id}
          opened={inviting}
          onClose={() => {
            setInviting(false)
            reload()
          }}
        />

        <ModalSheet
          opened={handover !== null}
          onClose={() => setHandover(null)}
          title="Hand over this group"
        >
          <Stack gap="sm">
            {/* Named in full, because it is the one action here that cannot be
                undone by the person taking it. */}
            <Text size="sm">
              {handover?.display_name} becomes the owner. You become a DM, and can then leave the
              group. Only they will be able to hand it back.
            </Text>
            <Group justify="flex-end">
              <Button variant="default" onClick={() => setHandover(null)}>
                Cancel
              </Button>
              <Button
                loading={change.pending}
                onClick={() => {
                  const target = handover
                  setHandover(null)
                  if (target !== null) void act(change.run(group.id, target.user_id, 'owner'))
                }}
              >
                Make owner
              </Button>
            </Group>
          </Stack>
        </ModalSheet>
      </Stack>
    </Page>
  )
}
