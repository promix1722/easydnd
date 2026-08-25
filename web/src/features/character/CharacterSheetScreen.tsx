import { Link, useParams } from 'react-router'

import { getPrompts, getSheet } from '@/lib/api'
import type { Prompt, Sheet } from '@/lib/api'
import { useResource } from '@/lib/useResource'
import {
  Alert,
  Anchor,
  Button,
  Group,
  Loader,
  Stack,
  Text,
  Title,
} from '@/ui'

import { OutstandingChoices } from './OutstandingChoices'
import type { Compendium } from './compendium'
import { loadCompendium } from './compendium'
import { SheetBody } from './SheetBody'

import { answerable } from '@/domain'

/** The sheet, and what the character has not decided yet. */
interface SheetView {
  sheet: Sheet
  /**
   * Null when `/prompts` failed. The list of what is left is worth having and
   * is not worth losing the sheet over -- a sheet that refuses to draw because
   * a second request failed is a page that fails for a reason it is not about.
   */
  prompts: Prompt[] | null
  /**
   * The compendium collections the body names things out of. Each is null when
   * its request failed -- the same bargain as `prompts` above: the sheet is
   * worth drawing with title-cased slugs, and is not worth losing to a second
   * request.
   */
  compendium: Compendium
}


/** The character sheet. */
export function CharacterSheetScreen() {
  const { id = '' } = useParams()
  const sheet = useResource<SheetView>(`sheet:${id}`, async (signal) => {
    const [projected, prompts, compendium] = await Promise.all([
      getSheet(id, signal),
      getPrompts(id, signal).then(
        // Advancement is not offered anywhere in this client while level-up
        // does not work; see domain/stages.ts.
        (response) => (response.prompts ?? []).filter((prompt) => answerable(prompt.group)),
        () => null,
      ),
      // Session-cached, so this is one request for the whole visit however
      // many sheets are opened.
      loadCompendium(),
    ])
    return { sheet: projected, prompts, compendium }
  })

  if (sheet.loading) {
    return (
      <Group gap="xs">
        <Loader size="sm" />
        <Text size="sm" c="dimmed">
          Projecting the sheet...
        </Text>
      </Group>
    )
  }
  if (sheet.error !== null || sheet.data === null) {
    return (
      <Alert color="red" title="Could not load this character">
        <Stack gap="xs" align="flex-start">
          <Text size="sm">{sheet.error ?? 'Unknown error'}</Text>
          <Button variant="light" onClick={sheet.reload}>
            Try again
          </Button>
        </Stack>
      </Alert>
    )
  }

  const s = sheet.data.sheet
  const identity = s.identity
  const outstanding = sheet.data.prompts ?? []

  return (
    <Stack gap="lg">
      <Group justify="space-between" align="flex-start">
        <Title order={2}>{identity.name || 'Unnamed'}</Title>
        <Anchor component={Link} to={`/characters/${id}/log`}>
          <Button variant="subtle">Event log</Button>
        </Anchor>
      </Group>


      {/*
        An unfinished character says so on the page it is looked at most.
        The same `/prompts` response the build screen's tabs draw, named by the
        same `choiceName` -- there is no second notion anywhere in this client
        of what is still outstanding, and so no way for the sheet and the build
        screen to disagree about it. Here it is a statement of what is left and
        the way in is the link below; there each choice is a block that opens.
      */}
      {outstanding.length > 0 && (
        <Alert color="blue" title="Still to choose">
          <Stack gap="xs" align="flex-start">
            <OutstandingChoices prompts={outstanding} />
            <Anchor component={Link} to={`/characters/${id}/build`}>
              <Button variant="light">Answer these</Button>
            </Anchor>
          </Stack>
        </Alert>
      )}
      <SheetBody sheet={s} compendium={sheet.data.compendium} />
    </Stack>
  )
}
