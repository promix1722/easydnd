import { useState } from 'react'
import { useNavigate, useParams } from 'react-router'

import type { GroupDetail, GroupMember } from '@/lib/api'
import { deleteGroup, getGroup, removeMember, setMemberRole } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import { useAction } from '@/lib/useAction'
import { useResource } from '@/lib/useResource'
import {
  Alert,
  Badge,
  Button,
  DataList,
  Group,
  Loader,
  Menu,
  ModalSheet,
  Stack,
  Text,
  Title,
  Tooltip,
} from '@/ui'

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

  const [inviting, setInviting] = useState(false)
  const [handover, setHandover] = useState<GroupMember | null>(null)
  const change = useAction(setMemberRole)
  const remove = useAction(removeMember)
  const destroy = useAction(deleteGroup)

  if (loading) {
    return (
      <Group gap="xs">
        <Loader size="sm" />
        <Text size="sm" c="dimmed">
          Loading...
        </Text>
      </Group>
    )
  }
  if (error !== null || data === null) {
    return (
      <Alert color="red" title="Could not load this group">
        <Stack gap="xs" align="flex-start">
          <Text size="sm">{error ?? 'That group is not there.'}</Text>
          <Button variant="light" onClick={reload}>
            Try again
          </Button>
        </Stack>
      </Alert>
    )
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

  const failure = change.error ?? remove.error ?? destroy.error

  return (
    <Stack gap="md">
      <Group justify="space-between" align="flex-start">
        <div>
          <Title order={2}>{group.name}</Title>
          <Group gap="xs" mt={4}>
            <Badge variant="light">{ROLE_LABELS[group.role]}</Badge>
            <Text c="dimmed" size="sm">
              {group.members.length} {group.members.length === 1 ? 'member' : 'members'}
            </Text>
          </Group>
        </div>
        <Group gap="xs">
          {canManage && <Button onClick={() => setInviting(true)}>Invite</Button>}
          {isOwner ? (
            // Rendered and disabled rather than hidden. A control that is not
            // there teaches nothing; one that is there with a reason teaches
            // the rule the first time somebody reaches for it.
            <Tooltip label="Make somebody else the owner first, or delete the group">
              <Button variant="default" data-disabled onClick={(event) => event.preventDefault()}>
                Leave
              </Button>
            </Tooltip>
          ) : (
            <Button variant="default" loading={remove.pending} onClick={() => void leave()}>
              Leave
            </Button>
          )}
          {isOwner && (
            <Button color="red" variant="light" loading={destroy.pending} onClick={() => void close()}>
              Delete group
            </Button>
          )}
        </Group>
      </Group>

      {failure !== null && (
        <Alert color="red" title="That did not work">
          {failure}
        </Alert>
      )}

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
                    <Button size="compact-xs" variant="subtle">
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
  )
}
