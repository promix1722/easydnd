import { useState } from 'react'
import { Link, useNavigate } from 'react-router'

import type { GroupSummary } from '@/lib/api'
import { fieldMessage, createGroup, deleteGroup, listGroups, removeMember, renameGroup } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import { useAction } from '@/lib/useAction'
import { useResource } from '@/lib/useResource'
import { useT } from '@/lib/i18n'
import {
  ACTION_ICON_SIZE,
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
  ModalSheet,
  Page,
  pageState,
  Stack,
  Text,
  TextInput,
} from '@/ui'

import { atLeast, roleLabel } from './roles'

/** The tables this account sits at. */
export function GroupListScreen() {
  const t = useT()
  const navigate = useNavigate()
  const { data, error, loading, reload, refresh } = useResource('groups', (signal) => listGroups(signal))

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
    refresh()
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
    // No trail below the section, so `Page` draws "Groups" as the heading with
    // the section's glyph and no breadcrumb above it -- which is exactly what
    // this screen rendered before, plus the glyph.
    <Page
      trail={[]}
      state={pageState(
        { data, error, loading },
        {
          title: t('groups.list.loadFailed'),
          fallback: t('error.unknown'),
          onRetry: reload,
        },
      )}
    >
      <Stack gap="md">
        <DataList
          items={groups}
          getKey={(group) => group.id}
          actions={(group) => [
            // Leaving comes first: it is not an edit of the group, and putting
            // it between Rename and Delete reads as though it were one of them.
            // The owner cannot walk away from their own table, and nobody else
            // may delete it, so each rank gets exactly one of these two.
            ...(group.role !== 'owner'
              ? [
                  {
                    key: 'leave',
                    label: t('groups.leave'),
                    icon: <IconLogout size={ACTION_ICON_SIZE} />,
                    onClick: () => void act(leave.run(group.id, me)),
                  },
                ]
              : []),
            ...(atLeast(group.role, 'dm')
              ? [
                  {
                    key: 'rename',
                    label: t('common.rename'),
                    icon: <IconPencil size={ACTION_ICON_SIZE} />,
                    onClick: () => {
                      setNewName(group.name)
                      setRenaming(group)
                    },
                  },
                ]
              : []),
            ...(group.role === 'owner'
              ? [
                  {
                    key: 'delete',
                    label: t('common.delete'),
                    color: 'red' as const,
                    icon: <IconTrash size={ACTION_ICON_SIZE} />,
                    onClick: () => setDeleting(group),
                  },
                ]
              : []),
          ]}
          columns={[
            {
              key: 'name',
              header: t('common.name'),
              primary: true,
              text: (group) => group.name,
              to: (group) => `/groups/${group.id}`,
              render: (group) => (
                <Anchor component={Link} to={`/groups/${group.id}`}>
                  {group.name}
                </Anchor>
              ),
            },
            {
              key: 'role',
              header: t('groups.yourRole'),
              // A badge, so it rides beside the name rather than joining the
              // dimmed run of facts, where it would read as neither.
              slot: 'badge',
              render: (group) => <Badge variant="light">{roleLabel(t, group.role)}</Badge>,
            },
          ]}
          empty={t('groups.list.empty')}
        />

        {/* Under the table and to the left: adding a row belongs beneath the
            rows, not in the heading, which is about the section rather than
            about what you can put in it. */}
        <Group>
          <Button
            variant="light"
            leftSection={<IconPlus size={ACTION_ICON_SIZE} />}
            onClick={() => setCreating(true)}
          >
            {t('groups.new')}
          </Button>
        </Group>

        <ModalSheet
          opened={renaming !== null}
          onClose={() => setRenaming(null)}
          title={t('groups.renameTitle')}
          onSubmit={() => {
            const target = renaming
            setRenaming(null)
            if (target !== null) void act(rename.run(target.id, newName))
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
            {rename.error !== null && (
              <Alert color="red" title={t('groups.renameFailed')}>
                {rename.error}
              </Alert>
            )}
            <Group justify="flex-end">
              <Button variant="default" onClick={() => setRenaming(null)}>
                {t('common.cancel')}
              </Button>
              <Button type="submit" loading={rename.pending}>
                {t('common.rename')}
              </Button>
            </Group>
          </Stack>
        </ModalSheet>

        <ModalSheet
          opened={deleting !== null}
          onClose={() => setDeleting(null)}
          title={t('groups.deleteTitle')}
        >
          <Stack gap="sm">
            {/* Named in full: it takes the whole table with it, and nobody at it
                can undo that. */}
            <Text size="sm">{t('groups.deleteWarning', { name: deleting?.name ?? '' })}</Text>
            <Group justify="flex-end">
              <Button variant="default" onClick={() => setDeleting(null)}>
                {t('common.cancel')}
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
                {t('common.delete')}
              </Button>
            </Group>
          </Stack>
        </ModalSheet>

        <ModalSheet
          opened={creating}
          onClose={() => setCreating(false)}
          title={t('groups.new')}
          onSubmit={() => void submit()}
        >
          <Stack gap="sm">
            <TextInput
              label={t('common.name')}
              placeholder={t('groups.namePlaceholder')}
              value={name}
              onChange={(event) => setName(event.currentTarget.value)}
              error={fieldMessage(t, create.fields, 'name')}
              data-autofocus
            />
            {create.error !== null && (
              <Alert color="red" title={t('groups.createFailed')}>
                {create.error}
              </Alert>
            )}
            <Group justify="flex-end">
              <Button variant="default" onClick={() => setCreating(false)}>
                {t('common.cancel')}
              </Button>
              <Button type="submit" loading={create.pending}>
                {t('common.create')}
              </Button>
            </Group>
          </Stack>
        </ModalSheet>
    </Stack>
    </Page>
  )
}
