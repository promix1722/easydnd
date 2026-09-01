import { useParams } from 'react-router'

import type { Sheet } from '@/lib/api'
import { getGroup, getSharedSheet } from '@/lib/api'
import { useResource } from '@/lib/useResource'
import { useLocale, useT } from '@/lib/i18n'
import { Badge, Page, pageState } from '@/ui'

import { titleCase } from '@/domain'

import type { Compendium } from '../character/compendium'
import { loadCompendium } from '../character/compendium'
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
  // The same compendium the owner's own sheet loads, so the two name things
  // out of one set. Both requests are session-cached.
  const { data, error, loading, reload } = useResource<{
    sheet: Sheet
    compendium: Compendium
    /**
     * The group's name, for the middle crumb, or null when the lookup failed.
     *
     * Fetched here rather than threaded through the route because the trail is
     * `Groups / <group> / <character>` and a crumb that says a group id is no
     * better than one that says nothing. The failure is tolerated on the same
     * bargain the owner's sheet already makes for `prompts` and the compendium:
     * a shared sheet is worth drawing, and is not worth losing to a second
     * request for one word. A null renders as the crumb's placeholder.
     */
    groupName: string | null
  }>(`shared:${locale}:${character}`, async (signal) => {
    const [sheet, compendium, groupName] = await Promise.all([
      getSharedSheet(character, signal),
      loadCompendium(),
      getGroup(groupId, signal).then(
        (group) => group.name,
        () => null,
      ),
    ])
    return { sheet, compendium, groupName }
  })

  const state = pageState(
    { data, error, loading },
    {
      title: t('sharedSheet.loadFailed'),
      fallback: t('sharedSheet.missing'),
      onRetry: reload,
    },
  )

  // The group is a crumb here, unlike on a game: a shared sheet really does
  // hang off the group, which is what grants the read. See
  // docs/web.md#sharing-is-reading.
  const group = { label: data?.groupName ?? null, to: `/groups/${groupId}` }

  if (state.kind !== 'ready' || data === null) {
    return (
      <Page
        trail={[group, { label: null }]}
        state={state.kind === 'loading' ? { ...state, what: t('sharedSheet.loading') } : state}
      />
    )
  }

  const identity = data.sheet.identity
  const named = (collection: string, slug: string | undefined) =>
    slug === undefined
      ? null
      : (data.compendium.names?.get(`${collection}:${slug}`) ?? titleCase(slug))
  const classes = (identity.classes ?? [])
    .map(({ class: slug, level }) => `${named('classes', slug) ?? slug} ${level}`)
    .join(' / ')

  return (
    <Page
      trail={[group, { label: identity.name || 'Unnamed' }]}
      badge={<Badge variant="light">{t('sharedSheet.readOnly')}</Badge>}
      subtitle={
        <>
          {[
            named('races', identity.race),
            named('backgrounds', identity.background),
            classes,
          ]
            .filter((part) => part !== null && part !== '--')
            .join(' · ')}
        </>
      }
    >
      {/* The way back is the trail now. The "Back to the group" button that
          used to sit here said the same thing in a second place. */}
      <SheetBody sheet={data.sheet} compendium={data.compendium} />
    </Page>
  )
}
