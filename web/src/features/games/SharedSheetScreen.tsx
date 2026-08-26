import { useParams } from 'react-router'

import type { Sheet } from '@/lib/api'
import { getGroup, getSharedSheet } from '@/lib/api'
import { useResource } from '@/lib/useResource'
import { Badge, Page, pageState } from '@/ui'

import type { Compendium } from '../character/compendium'
import { loadCompendium } from '../character/compendium'
import { SheetBody } from '../character/SheetBody'

import { classLine, titleCase } from '@/domain'

/**
 * A character shared with a group, read by somebody who does not own it.
 *
 * It draws the same `SheetBody` its owner sees, because the server renders both
 * with one converter -- the table is looking at the character, not at a summary
 * of it. What is missing is everything about changing it: no build link, no
 * event log, no outstanding choices. None of that is hidden; there is simply no
 * route behind any of it for anybody but the owner.
 */
export function SharedSheetScreen() {
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
  }>(`shared:${character}`, async (signal) => {
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
      title: 'Could not load this character',
      fallback: 'That character is not on this table.',
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
        state={state.kind === 'loading' ? { ...state, what: 'Projecting the sheet...' } : state}
      />
    )
  }

  const identity = data.sheet.identity

  return (
    <Page
      trail={[group, { label: identity.name || 'Unnamed' }]}
      badge={<Badge variant="light">Read only</Badge>}
      subtitle={
        <>
          {[
            identity.race !== undefined ? titleCase(identity.race) : null,
            identity.background !== undefined ? titleCase(identity.background) : null,
            classLine(identity.classes),
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
