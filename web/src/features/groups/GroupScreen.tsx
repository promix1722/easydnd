import { useState } from 'react'
import { useNavigate, useParams } from 'react-router'

import type { GroupDetail, GroupMember } from '@/lib/api'
import { fieldMessage, deleteGroup, getGroup, removeMember, renameGroup, setMemberRole } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import { useT } from '@/lib/i18n'
import { useAction } from '@/lib/useAction'
import { useResource } from '@/lib/useResource'
import {
  ACTION_ICON_SIZE,
  Alert,
  Badge,
  Button,
  DataList,
  Group,
  IconLogout,
  IconPencil,
  IconTrash,
  IconUserPlus,
  ModalSheet,
  Page,
  Panel,
  Stack,
  TabRow,
  Text,
  TextInput,
  Tooltip,
  pageState,
} from '@/ui'

import { TablePanel } from '../games'

import { InviteSheet } from './InviteSheet'
import { atLeast, roleLabel } from './roles'

/** One table: who is at it, and what this account may do about that. */
export function GroupScreen() {
  const t = useT()
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const { user } = useAuth()
  const { data, error, loading, reload, refresh } = useResource(`group:${id}`, (signal) =>
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
    { title: t('group.loadFailed'), fallback: t('group.missing'), onRetry: reload },
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
    refresh()
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
      badge={<Badge variant="light">{roleLabel(t, group.role)}</Badge>}
      // Beside the trail rather than in the actions cluster: leaving is a fact
      // about your standing here, not something done to the group.
      lead={
        isOwner ? (
          // Rendered and disabled rather than hidden. A control that is not
          // there teaches nothing; one that is there with a reason teaches
          // the rule the first time somebody reaches for it.
          <Tooltip label={t('group.ownerCannotLeave')}>
            <Button
              variant="subtle"
              leftSection={<IconLogout size={ACTION_ICON_SIZE} />}
              data-disabled
              onClick={(event) => event.preventDefault()}
            >
              {t('groups.leave')}
            </Button>
          </Tooltip>
        ) : (
          <Button
            variant="subtle"
            leftSection={<IconLogout size={ACTION_ICON_SIZE} />}
            loading={remove.pending}
            onClick={() => void leave()}
          >
            {t('groups.leave')}
          </Button>
        )
      }
      actions={
        <>
          {canManage && (
            <Button
              variant="subtle"
              leftSection={<IconPencil size={ACTION_ICON_SIZE} />}
              onClick={() => {
                setNewName(group.name)
                setRenaming(true)
              }}
            >
              {t('common.rename')}
            </Button>
          )}
          {isOwner && (
            <Button
              color="red"
              variant="subtle"
              leftSection={<IconTrash size={ACTION_ICON_SIZE} />}
              loading={destroy.pending}
              onClick={() => void close()}
            >
              {t('common.delete')}
            </Button>
          )}
        </>
      }
    >
      <Panel>
        <Stack gap="md">
          {failure !== null && (
            <Alert color="red" title={t('group.actionFailed')}>
              {failure}
            </Alert>
          )}

          <TabRow
            tabs={[
              { value: 'members', label: t('group.members') },
              { value: 'characters', label: t('section.characters') },
            ]}
            value={tab}
            onChange={setTab}
          >
            {tab === 'characters' && <TablePanel groupId={group.id} role={group.role} />}
            {tab === 'members' && (
          <DataList
            items={group.members}
            getKey={(member) => member.user_id}
            badges={(member: GroupMember) => (
              <>
                {member.user_id === me && (
                  <Badge size="xs" variant="light">
                    {t('group.you')}
                  </Badge>
                )}
                {member.anonymous && (
                  <Tooltip label={t('group.guestExplained')}>
                    <Badge size="xs" color="gray" variant="outline">
                      {t('group.guest')}
                    </Badge>
                  </Tooltip>
                )}
              </>
            )}
            actions={(member: GroupMember) => {
              // The owner is nobody's to unseat, including their own, and a
              // player manages no one. The server enforces all of this; the point
              // of repeating it here is to not offer a button that is going to
              // come back 403.
              if (member.role === 'owner' || member.user_id === me || !canManage) return []
              return [
                ...(isOwner && member.role === 'player'
                  ? [
                      {
                        key: 'promote',
                        label: t('group.makeDm'),
                        onClick: () => void act(change.run(group.id, member.user_id, 'dm')),
                      },
                    ]
                  : []),
                ...(isOwner && member.role === 'dm'
                  ? [
                      {
                        key: 'demote',
                        label: t('group.makePlayer'),
                        onClick: () => void act(change.run(group.id, member.user_id, 'player')),
                      },
                    ]
                  : []),
                ...(isOwner
                  ? [{ key: 'handover', label: t('group.makeOwner'), onClick: () => setHandover(member) }]
                  : []),
                {
                  key: 'remove',
                  label: t('group.removeMember'),
                  color: 'red' as const,
                  onClick: () => void act(remove.run(group.id, member.user_id)),
                },
              ]
            }}
            columns={[
              {
                key: 'name',
                header: t('group.member'),
                primary: true,
                // No `to`: a member is not a page. The name is the row's identity
                // and the label every action here is announced with, which is why
                // it is a string rather than only markup.
                text: (member: GroupMember) => member.display_name || t('common.unnamed'),
                render: (member: GroupMember) => (
                  <Text size="sm">{member.display_name || t('common.unnamed')}</Text>
                ),
              },
              {
                key: 'role',
                header: t('group.role'),
                render: (member: GroupMember) => roleLabel(t, member.role),
              },
            ]}
            empty={t('group.noMembers')}
          />
            )}
            {tab === 'members' && canManage && (
              <Group mt="md">
                <Button
                  variant="light"
                  leftSection={<IconUserPlus size={ACTION_ICON_SIZE} />}
                  onClick={() => setInviting(true)}
                >
                  {t('group.invite')}
                </Button>
              </Group>
            )}
          </TabRow>

          <ModalSheet
            opened={renaming}
            onClose={() => setRenaming(false)}
            title={t('groups.renameTitle')}
            onSubmit={() => {
              setRenaming(false)
              void act(rename.run(group.id, newName))
            }}
          >
            <Stack gap="sm">
              <TextInput
                label={t('common.name')}
                value={newName}
                error={fieldMessage(t, rename.fields, 'name')}
                onChange={(event) => setNewName(event.currentTarget.value)}
                data-autofocus
              />
              <Group justify="flex-end">
                <Button variant="default" onClick={() => setRenaming(false)}>
                  {t('common.cancel')}
                </Button>
                <Button type="submit" loading={rename.pending}>
                  {t('common.rename')}
                </Button>
              </Group>
            </Stack>
          </ModalSheet>

          <InviteSheet
            groupId={group.id}
            opened={inviting}
            onClose={() => {
              setInviting(false)
              refresh()
            }}
          />

          <ModalSheet
            opened={handover !== null}
            onClose={() => setHandover(null)}
            title={t('group.handoverTitle')}
          >
            <Stack gap="sm">
              {/* Named in full, because it is the one action here that cannot be
                  undone by the person taking it. */}
              <Text size="sm">
                {t('group.handoverWarning', { name: handover?.display_name ?? '' })}
              </Text>
              <Group justify="flex-end">
                <Button variant="default" onClick={() => setHandover(null)}>
                  {t('common.cancel')}
                </Button>
                <Button
                  loading={change.pending}
                  onClick={() => {
                    const target = handover
                    setHandover(null)
                    if (target !== null) void act(change.run(group.id, target.user_id, 'owner'))
                  }}
                >
                  {t('group.makeOwner')}
                </Button>
              </Group>
            </Stack>
          </ModalSheet>
        </Stack>
      </Panel>
    </Page>
  )
}
