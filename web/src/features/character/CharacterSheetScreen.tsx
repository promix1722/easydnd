import { Link, useParams } from 'react-router'

import { getPrompts, getSheet } from '@/lib/api'
import type { Prompt, Sheet } from '@/lib/api'
import { useResource } from '@/lib/useResource'
import { Alert, Anchor, Button, Page, pageState, Stack } from '@/ui'

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

  const state = pageState(sheet, {
    title: 'Could not load this character',
    fallback: 'Unknown error',
    onRetry: sheet.reload,
  })

  if (state.kind !== 'ready' || sheet.data === null) {
    // "Projecting the sheet" rather than "Loading": the sheet is derived from
    // the event log on request, and the word says so. It is the one screen
    // whose loading line says something the generic one does not, which is why
    // `Page` takes an override rather than imposing a single word everywhere.
    return (
      <Page
        trail={[{ label: null }]}
        state={state.kind === 'loading' ? { ...state, what: 'Projecting the sheet...' } : state}
      />
    )
  }

  const s = sheet.data.sheet
  const identity = s.identity
  const outstanding = sheet.data.prompts ?? []

  return (
    <Page
      trail={[{ label: identity.name || 'Unnamed' }]}
      actions={
        <Anchor component={Link} to={`/characters/${id}/log`}>
          <Button variant="subtle">Event log</Button>
        </Anchor>
      }
    >
      <Stack gap="lg">
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
    </Page>
  )
}
