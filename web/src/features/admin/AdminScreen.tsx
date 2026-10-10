import { Link, useSearchParams } from 'react-router'

import { classLine } from '@/domain'
import { listAdminCharacters, listAdminPlayers } from '@/lib/api'
import { formatDate, formatDateTime, useLocale, useT } from '@/lib/i18n'
import { Anchor, Badge, Page, Panel, TabRow, Text } from '@/ui'

import { AdminTable, FilterBox, FilterSelect } from './AdminTable'
import { usePaged } from './usePaged'

import type { AdminCharacter, AdminPlayer } from '@/lib/api'

type SetFilter = (key: string, value: string | null) => void

/**
 * Every account and every character, for a superadmin.
 *
 * The tab and every filter live in the URL, so a filtered table is a link and
 * Back undoes a filter. The server guards the data -- `AdminOnly` on the route
 * only spares everybody else a screen of failed requests.
 */
export function AdminScreen() {
  const t = useT()
  const [params, setParams] = useSearchParams()
  const tab = params.get('tab') === 'characters' ? 'characters' : 'players'

  const setFilter: SetFilter = (key, value) => {
    // The functional form: two filter boxes each commit from their own timer,
    // and one must not write back the other's stale value.
    setParams((previous) => {
      const next = new URLSearchParams(previous)
      if (value === null || value === '') next.delete(key)
      else next.set(key, value)
      return next
    }, { replace: true })
  }

  return (
    <Page trail={[]}>
      <Panel>
        <TabRow
          tabs={[
            { value: 'players', label: t('admin.players') },
            { value: 'characters', label: t('section.characters') },
          ]}
          value={tab}
          // A tab's filters are its own: switching drops them.
          onChange={(value) => setParams(value === 'players' ? {} : { tab: value })}
        >
          {tab === 'players'
            ? <PlayersTab params={params} setFilter={setFilter} />
            : <CharactersTab params={params} setFilter={setFilter} />}
        </TabRow>
      </Panel>
    </Page>
  )
}

function PlayersTab({ params, setFilter }: { params: URLSearchParams; setFilter: SetFilter }) {
  const t = useT()
  const locale = useLocale()
  const filters = { q: params.get('q'), kind: params.get('kind') }
  const paged = usePaged<AdminPlayer>(`admin:players:${JSON.stringify(filters)}`, async (offset, signal) => {
    const page = await listAdminPlayers(filters, offset, signal)
    return { rows: page.players, total: page.total }
  })
  const charactersOf = (player: AdminPlayer) => `/admin?tab=characters&owner=${encodeURIComponent(player.id)}`

  return (
    <AdminTable
      paged={paged}
      loadFailed={t('admin.players.loadFailed')}
      count={(count) => t('admin.players.count', { count })}
      empty={t('admin.players.empty')}
      filters={
        <>
          <FilterBox label={t('admin.players.search')} value={filters.q ?? ''} onCommit={(value) => setFilter('q', value)} />
          <FilterSelect
            label={t('admin.players.kind')}
            placeholder={t('admin.players.allKinds')}
            options={[
              { value: 'account', label: t('admin.players.accounts') },
              { value: 'guest', label: t('admin.players.guests') },
            ]}
            value={filters.kind}
            onChange={(value) => setFilter('kind', value)}
          />
        </>
      }
      getKey={(player) => player.id}
      badges={(player) => player.anonymous && <Badge size="sm" variant="default">{t('group.guest')}</Badge>}
      columns={[
        {
          key: 'name',
          header: t('common.name'),
          primary: true,
          text: (player) => player.display_name || t('common.unnamed'),
          to: charactersOf,
          render: (player) => (
            <Anchor component={Link} to={charactersOf(player)}>
              <Text size="sm">{player.display_name || t('common.unnamed')}</Text>
            </Anchor>
          ),
        },
        { key: 'email', header: t('admin.players.email'), render: (player) => player.email ?? '' },
        { key: 'id', header: t('admin.id'), render: (player) => player.id },
        { key: 'created', header: t('admin.players.created'), render: (player) => formatDate(player.created_at, locale) },
        {
          key: 'lastSignIn',
          header: t('admin.players.lastSignIn'),
          render: (player) => (player.last_used_at ? formatDateTime(player.last_used_at, locale) : ''),
        },
        { key: 'passkeys', header: t('account.passkeys'), render: (player) => player.passkeys },
      ]}
    />
  )
}

function CharactersTab({ params, setFilter }: { params: URLSearchParams; setFilter: SetFilter }) {
  const t = useT()
  const filters = { owner: params.get('owner'), id: params.get('id'), public: params.get('public') }
  const paged = usePaged<AdminCharacter>(`admin:characters:${JSON.stringify(filters)}`, async (offset, signal) => {
    const page = await listAdminCharacters(filters, offset, signal)
    return { rows: page.characters, total: page.total }
  })
  // The shared-sheet route: a superadmin may read any sheet and change none.
  const sheetOf = (character: AdminCharacter) => `/shared/${encodeURIComponent(character.id)}`

  return (
    <AdminTable
      paged={paged}
      loadFailed={t('admin.characters.loadFailed')}
      count={(count) => t('admin.characters.count', { count })}
      empty={t('admin.characters.empty')}
      filters={
        <>
          <FilterBox label={t('admin.characters.owner')} value={filters.owner ?? ''} onCommit={(value) => setFilter('owner', value)} />
          <FilterBox label={t('admin.characters.id')} value={filters.id ?? ''} onCommit={(value) => setFilter('id', value)} />
          <FilterSelect
            label={t('admin.characters.visibility')}
            placeholder={t('admin.characters.anyVisibility')}
            options={[
              { value: 'true', label: t('admin.characters.public') },
              { value: 'false', label: t('admin.characters.private') },
            ]}
            value={filters.public}
            onChange={(value) => setFilter('public', value)}
          />
        </>
      }
      getKey={(character) => character.id}
      badges={(character) => character.public && <Badge size="sm" variant="default">{t('admin.characters.public')}</Badge>}
      columns={[
        {
          key: 'name',
          header: t('common.name'),
          primary: true,
          text: (character) => character.name || t('common.unnamed'),
          to: sheetOf,
          render: (character) => (
            <Anchor component={Link} to={sheetOf(character)}>
              <Text size="sm">{character.name || t('common.unnamed')}</Text>
            </Anchor>
          ),
        },
        { key: 'classes', header: t('characters.classes'), render: (character) => classLine(character.classes) },
        { key: 'owner', header: t('admin.characters.ownerColumn'), render: (character) => character.owner_name || character.owner },
        { key: 'id', header: t('admin.id'), render: (character) => character.id },
        { key: 'revision', header: t('admin.characters.revision'), render: (character) => character.revision },
      ]}
    />
  )
}
