import { Link, useParams } from 'react-router'

import type { Sheet } from '@/lib/api'
import { getSharedSheet } from '@/lib/api'
import { useResource } from '@/lib/useResource'
import { Alert, Anchor, Badge, Button, Group, Loader, Stack, Text, Title } from '@/ui'

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
  }>(`shared:${character}`, async (signal) => {
    const [sheet, compendium] = await Promise.all([
      getSharedSheet(character, signal),
      loadCompendium(),
    ])
    return { sheet, compendium }
  })

  if (loading) {
    return (
      <Group gap="xs">
        <Loader size="sm" />
        <Text size="sm" c="dimmed">
          Projecting the sheet...
        </Text>
      </Group>
    )
  }
  if (error !== null || data === null) {
    return (
      <Alert color="red" title="Could not load this character">
        <Stack gap="xs" align="flex-start">
          <Text size="sm">{error ?? 'That character is not on this table.'}</Text>
          <Button variant="light" onClick={reload}>
            Try again
          </Button>
        </Stack>
      </Alert>
    )
  }

  const identity = data.sheet.identity

  return (
    <Stack gap="lg">
      <Group justify="space-between" align="flex-start">
        <div>
          <Group gap="xs">
            <Title order={2}>{identity.name || 'Unnamed'}</Title>
            <Badge variant="light">Read only</Badge>
          </Group>
          <Text c="dimmed" size="sm">
            {[
              identity.race !== undefined ? titleCase(identity.race) : null,
              identity.background !== undefined ? titleCase(identity.background) : null,
              classLine(identity.classes),
            ]
              .filter((part) => part !== null && part !== '--')
              .join(' · ')}
          </Text>
        </div>
        <Anchor component={Link} to={`/groups/${groupId}`}>
          <Button variant="subtle">Back to the group</Button>
        </Anchor>
      </Group>

      <SheetBody sheet={data.sheet} compendium={data.compendium} />
    </Stack>
  )
}
