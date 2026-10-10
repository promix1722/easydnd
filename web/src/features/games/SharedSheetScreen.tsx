import { Navigate, useParams } from 'react-router'

import type { Sheet } from '@/lib/api'
import { getSharedOwner, getSharedSheet } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import { CatalogScope } from '@/lib/api/catalogScope'
import { useResource } from '@/lib/useResource'
import { useLocale, useT } from '@/lib/i18n'
import { Avatar, characterAvatar, Badge, Page, pageState } from '@/ui'


import { SheetBody } from '../character/SheetBody'

/**
 * A character shared with a group, read by somebody who does not own it.
 *
 * It draws the same `SheetBody` its owner sees, because the server renders both
 * with one converter -- the table is looking at the character, not at a summary
 * of it. What is missing is everything about changing it: no build link, and so
 * no way in to whatever the character has still to decide. That is not hidden;
 * there is simply no route behind it for anybody but the owner.
 */
export function SharedSheetScreen() {
  const t = useT()
  const locale = useLocale()
  const { id: groupId = '', character = '' } = useParams()
  // The sheet names its own slugs, exactly as the owner's does: one resolver
  // on the server writes both, so the two cannot name things differently.
  const { data, error, loading, reload } = useResource<{
    sheet: Sheet
    /**
     * Whose character it is, for the crumb before its name, or null when the
     * lookup failed. A sheet does not say who owns it, so this is asked beside
     * it -- and tolerated failing, on the bargain the owner's sheet already
     * makes for `prompts`: a shared sheet is worth drawing, and is not worth
     * losing to a second request for one name.
     */
    owner: { id: string; name: string } | null
  }>(`shared:${locale}:${character}`, async (signal) => {
    // Opened by link there is no group, and so nobody to ask whose it is.
    const [sheet, owner] = await Promise.all([getSharedSheet(character, signal), groupId ? getSharedOwner(groupId, character, signal) : Promise.resolve(null)])
    return { sheet, owner }
  })
  const { user } = useAuth()

  const state = pageState(
    { data, error, loading },
    {
      title: t('sharedSheet.loadFailed'),
      fallback: t('sharedSheet.missing'),
      onRetry: reload,
    },
  )

  // The trail is the player and then the character: a character is somebody's,
  // and that is the fact a reader opening it from a game or a group wants
  // first. It used to be the group, which is how the read was granted rather
  // than whose sheet this is. The player has no page yet, so the crumb is a
  // name and not a link.
  const player = groupId === '' ? [] : [{ label: data === null ? null : (data.owner?.name || t('common.unnamed')) }]

  // Your own character is not a thing to read at arm's length: it opens as
  // your own sheet, with everything that can be done to it.
  if (data?.owner != null && user != null && data.owner.id === user.id) {
    return <Navigate replace to={`/characters/${encodeURIComponent(character)}`} />
  }

  if (state.kind !== 'ready' || data === null) {
    return (
      <Page
        trail={[...player, { label: null }]}
        state={state.kind === 'loading' ? { ...state, what: t('sharedSheet.loading') } : state}
      />
    )
  }

  const identity = data.sheet.identity
  return (
    <Page
      mark={<Avatar image={identity.image} fallback={characterAvatar(identity.classes)} size={48} />}
      trail={[...player, { label: identity.name || 'Unnamed' }]}
      badge={<Badge variant="light">{t('sharedSheet.readOnly')}</Badge>}
    >
      {/* The way back is the trail now. The "Back to the group" button that
          used to sit here said the same thing in a second place. */}
      <CatalogScope.Provider value={`/shared/${encodeURIComponent(character)}/catalog`}>
        <SheetBody sheet={data.sheet} />
      </CatalogScope.Provider>
    </Page>
  )
}
