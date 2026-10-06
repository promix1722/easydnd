import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'

import { appendEvents, getEvents, getPrompts, getSheet, replaceEvent } from '@/lib/api'
import type { Change, Prompt, Sheet } from '@/lib/api'
import { useAction } from '@/lib/useAction'
import { useResource } from '@/lib/useResource'
import { Avatar, characterAvatar, Badge, Button, Group, ModalSheet, NumberInput, Page, Stack, Text, pageState } from '@/ui'

import { desiredLevelChange } from './desiredLevel'
import { SheetBody } from './SheetBody'

import { MAX_LEVEL } from '@/domain'

import { useLocale, useT } from '@/lib/i18n'

/** The sheet, and what the character has not decided yet. */
interface SheetView {
  maxLevel: number
  sheet: Sheet
  /**
   * Null when `/prompts` failed. The list of what is left is worth having and
   * is not worth losing the sheet over -- a sheet that refuses to draw because
   * a second request failed is a page that fails for a reason it is not about.
   */
  prompts: Prompt[] | null
}


/** The character sheet. */
export function CharacterSheetScreen() {
  const t = useT()
  const locale = useLocale()
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const [pickingLevel, setPickingLevel] = useState<number | null>(null)

  /**
   * Declares the new desired level, then goes where the choices it opened are.
   *
   * The declaration lives in one entry, so raising it again *replaces* that
   * entry -- the same revision the build screen's identity tab performs --
   * rather than appending a second declaration for the log to read past. Only
   * a character that never declared one gets an append. Raising it can drop
   * nothing, so there is nothing to preview.
   */
  const levelUp = useAction(async (target: number) => {
    const log = await getEvents(id)
    const entry = log.events.find((event) =>
      (event.changes ?? []).some((change) => change.path === 'identity.desiredLevel'),
    )
    if (entry?.seq !== undefined) {
      const changes = (entry.changes ?? []).map((change) =>
        change.path === 'identity.desiredLevel' ? desiredLevelChange(target) : change,
      )
      return replaceEvent(id, entry.seq, log.seq, { type: entry.type, changes })
    }
    return appendEvents(id, log.seq, [{ type: 'change', changes: [desiredLevelChange(target)] }], log.revision ?? log.seq)
  })

  const confirmLevelUp = async (target: number) => {
    const written = await levelUp.run(target)
    if (written === null) return
    setPickingLevel(null)
    await navigate(`/characters/${id}/build`, { state: { stage: 'class' } })
  }
  // The sheet carries the localized names of its own slugs, so changing
  // language must reload it: that is what the locale is doing in the key.
  const sheet = useResource<SheetView>(`sheet:${locale}:${id}`, async (signal) => {
    const [projected, prompts] = await Promise.all([
      getSheet(id, signal),
      getPrompts(id, signal).then((response) => response, () => null),
    ])
    return { sheet: projected, prompts: prompts?.prompts ?? null, maxLevel: prompts?.buildPolicy?.maxLevel ?? MAX_LEVEL }
  })

  // Read the log's head at the moment of writing: the sheet does not carry a
  // sequence, and an edit made in another tab should conflict rather than vanish.
  const editEquipment = useAction(async (changes: Change[]) => {
    if (changes.length === 0) return
    const log = await getEvents(id)
    await appendEvents(id, log.seq, [{ type: 'change', changes }], log.revision ?? log.seq)
    sheet.refresh()
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
      mark={<Avatar image={identity.image} fallback={characterAvatar(identity.classes)} size={48} />}
      trail={[{ label: identity.name || 'Unnamed' }]}
      /*
       * A mark, and only while the character is unfinished.
       *
       * The button used to carry that message by appearing and disappearing,
       * which made one control mean two things: a way in to the build screen,
       * and the news that there is work left. They are separate facts, so the
       * news is a badge on the name -- where a rank or "Read only" already
       * goes -- and the way in is a button that is always there.
       *
       * A `/prompts` that failed is `null`, deliberately survivable, and draws
       * no badge: silence is the right answer to a question that could not be
       * asked, where "unfinished" would be a guess.
       */
      {...(outstanding.length > 0
        ? { badge: <Badge variant="light">{t('sheet.unfinished')}</Badge> }
        : {})}
      /*
       * What was here before was a link to the event log, drawn on every sheet
       * whether or not anybody wanted the record -- and beneath it, an alert
       * listing every choice still open above the sheet the page is for. Both
       * are gone: the log is not offered anywhere in this client for now (the
       * route still serves it, and `/characters/:id/log` is still the
       * unabridged record), and the list of what is left belongs to the screen
       * that answers it rather than to the one that reads the character.
       */
      actions={
        <Group gap="xs" wrap="nowrap">
          {/* The chat this character was made in, while the server still has it. */}
          {s.importSession && (
            <Button component={Link} variant="light" to={`/ai-wizard/${encodeURIComponent(s.importSession)}`}>
              {t('agent.history')}
            </Button>
          )}
          {/*
            Only for a character with a level to raise: one that has not taken
            its first class yet is still being created, and the build screen is
            already the whole of that. Gone at 20, where the rules stop.
          */}
          {identity.level >= 1 && identity.level < sheet.data.maxLevel && (
            <Button
              variant="light"
              onClick={() =>
                setPickingLevel(
                  Math.min(
                    Math.max(identity.desiredLevel ?? 0, identity.level + 1),
                    MAX_LEVEL,
                  ),
                )
              }
            >
              {t('sheet.levelUp')}
            </Button>
          )}
          <Button component={Link} to={`/characters/${id}/build`} variant="light">
            {t('common.edit')}
          </Button>
        </Group>
      }
    >
      {editEquipment.error !== null && (
        <Text size="sm" c="red">
          {editEquipment.error}
        </Text>
      )}
      <SheetBody
        sheet={s}
        pending={editEquipment.pending}
        onEquipment={(changes) => void editEquipment.run(changes)}
      />

      <ModalSheet
        opened={pickingLevel !== null}
        onClose={() => setPickingLevel(null)}
        title={t('sheet.levelUp')}
      >
        {pickingLevel !== null && (
          <Stack gap="md">
            <NumberInput
              aria-label={t('choice.desiredLevel')}
              min={identity.level + 1}
              max={MAX_LEVEL}
              clampBehavior="strict"
              allowDecimal={false}
              value={pickingLevel}
              onChange={(value) => {
                if (typeof value === 'number') setPickingLevel(value)
              }}
            />
            {levelUp.error !== null && (
              <Text size="sm" c="red">
                {levelUp.error}
              </Text>
            )}
            <Group>
              <Button loading={levelUp.pending} onClick={() => void confirmLevelUp(pickingLevel)}>
                {t('answer.confirm')}
              </Button>
              <Button variant="subtle" onClick={() => setPickingLevel(null)}>
                {t('common.cancel')}
              </Button>
            </Group>
          </Stack>
        )}
      </ModalSheet>
    </Page>
  )
}
