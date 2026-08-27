import { Link, useParams } from 'react-router'

import { getPrompts, getSheet } from '@/lib/api'
import type { Prompt, Sheet } from '@/lib/api'
import { useResource } from '@/lib/useResource'
import { Button, Page, pageState } from '@/ui'

import type { Compendium } from './compendium'
import { loadCompendium } from './compendium'
import { SheetBody } from './SheetBody'

import { answerable } from '@/domain'

import { useT } from '@/lib/i18n'

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
  const t = useT()
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
    title: t('build.loadFailed'),
    fallback: t('error.unknown'),
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
        state={state.kind === 'loading' ? { ...state, what: t('sharedSheet.loading') } : state}
      />
    )
  }

  const s = sheet.data.sheet
  const identity = s.identity
  const outstanding = sheet.data.prompts ?? []

  return (
    <Page
      trail={[{ label: identity.name || 'Unnamed' }]}
      /*
       * One action, and only while there is something to do with it.
       *
       * What was here before was a link to the event log, drawn on every sheet
       * whether or not anybody wanted the record -- and beneath it, an alert
       * listing every choice still open above the sheet the page is for. Both
       * are gone: the log is not offered anywhere in this client for now (the
       * route still serves it, and `/characters/:id/log` is still the
       * unabridged record), and the list of what is left belongs to the screen
       * that answers it rather than to the one that reads the character.
       *
       * So the header says the one thing the sheet cannot: that this character
       * is not finished, and where finishing it happens. Its presence is the
       * whole message, which is why there is nothing here when `/prompts` comes
       * back empty -- and why a `/prompts` that failed (`null`, deliberately
       * survivable) draws no button rather than a wrong one.
       */
      {...(outstanding.length > 0
        ? {
            actions: (
              <Button component={Link} to={`/characters/${id}/build`} variant="light">
                {t('sheet.answerWhatIsLeft')}
              </Button>
            ),
          }
        : {})}
    >
      <SheetBody sheet={s} compendium={sheet.data.compendium} />
    </Page>
  )
}
