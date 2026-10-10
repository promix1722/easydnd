import { useState } from 'react'
import type { NavigateFunction } from 'react-router'

import type { Stage } from '@/domain'
import {
  appendEvents,
  reviseEvents,
  createCharacter,
  deleteEvent,
  getPrompts,
  replaceEvent,
} from '@/lib/api'
import type { ApiFieldError, CharacterEvent } from '@/lib/api'
import { characterPath } from '@/lib/api/characters'
import { useT } from '@/lib/i18n'
import { useAction } from '@/lib/useAction'
import type { Resource } from '@/lib/useResource'

import { eventFor, reaskedKey } from './answers'
import { inheritPlace, keyForRow, reclaimPlace, settledKey } from './blocks'
import type { Asking } from './blocks'
import { NEW_NAME_KEY } from './buildModel'
import type { BuildView, Preview, Reposed } from './buildModel'
import { resolveRefNames } from './refNames'
import type { SettledRow } from './settled'
import type { SpellSubmission } from './SpellStagePanel'
import type { BuildDrafts } from './useBuildDrafts'
import type { BuildStages } from './useBuildStages'

/**
 * Every write a build screen makes -- creating the character, answering a
 * question, changing or dropping an answer -- with what each is waiting on
 * and what the server said about the last one.
 */
export function useBuildActions({ id, isNew, folder, build, view, drafts, stages, navigate }: {
  id: string
  isNew: boolean
  folder: string | undefined
  build: Resource<BuildView>
  view: BuildView
  drafts: BuildDrafts
  stages: BuildStages
  navigate: NavigateFunction
}) {
  const t = useT()
  const { packDirty, setPackError, nameDraft, setNameError, imageDraft, setImageDraft, selectedRules, draftRules } = drafts
  const { stage, openKey, orderFor, setOpenKey, setFocusNextStage, setAdvanceAfter, setChosenStage, setCreating, setOpenAfterCreation } = stages

  const create = useAction(createCharacter)
  const answer = useAction(appendEvents)
  const revise = useAction(replaceEvent)
  const spellSave = useAction(reviseEvents)
  const remove = useAction(deleteEvent)
  const [preview, setPreview] = useState<Preview | null>(null)
  const [reposed, setReposed] = useState<Reposed | null>(null)

  /**
   * Closing the question, and opening whatever takes its place.
   *
   * `open` is a key that does not exist yet: the block it names arrives with
   * the reread this starts. Answering a nested option and dropping an entry to
   * put its question again both leave a *new* question in the same place, and
   * a question that arrives shut is one the player has to press a second time
   * for no reason they could name.
   */
  const done = (open: string | null = null) => {
    setOpenKey(open)
    setFocusNextStage(null)
    setAdvanceAfter(open === null ? { view, stage } : null)
    setPreview(null)
    // Pinned to wherever the answer was given. The screen opens on the first
    // category with something to do, and that is the whole of the help it
    // offers: answering one question is not a request to be moved on to
    // another, and a player who has just chosen a class is usually looking at
    // what the class brought with it.
    setChosenStage(stage)
    // Behind the list rather than instead of it: the write is already
    // confirmed, and what is left is to find out what else it changed.
    build.refresh()
  }

  /**
   * Creates the character the Personal tab is describing.
   *
   * `landOn` rides across the navigation in the route's state, because the
   * navigation is what loses it: a different URL is a different mount, and the
   * new one has no memory of which tab the gesture was aimed at.
   */
  const createCharacterFromDraft = async (landOn: Stage) => {
    if (create.pending) return
    if (packDirty) { setPackError(t('packs.applyFirst')); return }
    if (nameDraft.trim() === '') {
      setNameError(t('build.nameRequired'))
      return
    }
    const created = await create.run({
      name: nameDraft.trim(),
      ...(imageDraft ? { image: imageDraft } : {}),
      ...(selectedRules ? { rules: selectedRules } : {}),
      ...(folder ? { folder } : {}),
    })
    // replace: true, because the URL of a character that does not exist is
    // not a place the Back button should return anyone to.
    if (created) {
      // Rules were chosen before there was a character to write them to.
      // Save that choice against the new character before showing its prompts.
      const rulesSaved = draftRules === null || await answer.run(created.id, created.seq, [
        { type: 'change', changes: draftRules },
      ], created.seq) !== null
      // Preserve the name question's position when its saved entry arrives.
      inheritPlace(orderFor('personal'), NEW_NAME_KEY, settledKey(created.seq))
      // Set before the navigation, because the navigation is what changes the
      // resource key -- and the render that reads the new key is the one that
      // would otherwise blank the page.
      setCreating(true)
      setOpenAfterCreation(rulesSaved ? landOn : 'rules')
      await navigate(`/characters/${created.id}/build`, {
        replace: true,
        state: { stage: rulesSaved ? landOn : 'rules' },
      })
    }
  }

  /** Sends one appended entry, then rereads everything. */
  const append = async (event: CharacterEvent, open: string | null) => {
    const answered = openKey
    const written = await answer.run(id, view.prompts.seq, [event], view.prompts.revision ?? view.prompts.seq)
    if (written === null) return
    // The entry that just answered a question takes that question's place.
    // A single appended event is the log's new head, so the response's seq
    // names it -- and without this the answer would appear at the bottom of
    // the list while the question it answered vanished from the middle.
    inheritPlace(orderFor(stage), answered, settledKey(written.seq))
    done(open)
  }

  /**
   * Prices a change, and either makes it or asks.
   *
   * The dry run is the same call the commit makes with `dryRun` set: a price
   * quoted by a second code path is a price that can disagree with what
   * happens, and one that disagrees is worse than none.
   *
   * What the price decides is whether there is anything to ask about. A change
   * that costs nothing else is simply made -- confirming every change taught
   * players to confirm without reading, which is exactly the habit the one
   * change that *does* cost something needs them not to have.
   */
  const price = async (row: SettledRow, event: CharacterEvent | null, open: string | null) => {
    const result =
      event === null
        ? await remove.run(id, row.seq, view.prompts.seq, true, view.prompts.revision ?? view.prompts.seq)
        : await revise.run(id, row.seq, view.prompts.seq, event, true, view.prompts.revision ?? view.prompts.seq)
    if (result === null) return
    const dropped = result.dropped ?? []
    if (dropped.length === 0) {
      await write(row, event, open)
      return
    }
    setPreview({ row, event, dropped, names: await resolveRefNames(dropped, `${characterPath(id)}/catalog`), open })
  }

  /** Makes the change, whether it was asked about or not. */
  const write = async (row: SettledRow, event: CharacterEvent | null, open: string | null) => {
    // expectedSeq is sent again, so a log that moved between the price and
    // this write is the existing sequence conflict rather than a silent
    // commit of a price that was quoted against a different log.
    const written =
      event === null
        ? await remove.run(id, row.seq, view.prompts.seq, false, view.prompts.revision ?? view.prompts.seq)
        : await revise.run(id, row.seq, view.prompts.seq, event, false, view.prompts.revision ?? view.prompts.seq)
    if (written === null) return
    // A removal is how a question that cannot be re-posed gets asked again, so
    // the question that comes back takes the answer's place in the list -- and
    // opens there, because being asked again is what the press meant.
    if (event === null) reclaimPlace(orderFor(row.stage), keyForRow(row))
    done(open)
  }

  const saveSpells = async (submissions: SpellSubmission[], next: Stage, approved = false) => {
    const replacements = submissions.flatMap((item) => {
      if (item.replaces === undefined) return []
      const choices = (item.replaces.event.choices ?? []).map((answer) => answer.prompt === item.prompt.choice.prompt ? { ...answer, picks: item.picks } : answer)
      return [{ seq: item.replaces.seq, event: eventFor(item.prompt, choices) }]
    })
    const events = submissions.filter((item) => item.replaces === undefined).map((item) => item.changes === undefined ? eventFor(item.prompt, [{ prompt: item.prompt.choice.prompt, picks: item.picks }]) : { type: 'change', changes: item.changes })
    const run = (dryRun: boolean) => replacements.length === 1 && events.length === 0
      ? revise.run(id, replacements[0]!.seq, view.prompts.seq, replacements[0]!.event, dryRun, view.prompts.revision ?? view.prompts.seq)
      : spellSave.run(id, view.prompts.seq, view.prompts.revision ?? view.prompts.seq, replacements, events, dryRun)
    if (replacements.length > 0 && !approved) {
      const preview = await run(true)
      if (preview === null) return
      if ((preview.dropped ?? []).length > 0) {
        const row = submissions.find((item) => item.replaces !== undefined)!.replaces!
        setPreview({ row, event: replacements[0]!.event, dropped: preview.dropped ?? [], names: await resolveRefNames(preview.dropped ?? [], `${characterPath(id)}/catalog`), open: null, spellBatch: { submissions, next } })
        return
      }
    }
    const written = replacements.length === 0
      ? await answer.run(id, view.prompts.seq, events, view.prompts.revision ?? view.prompts.seq)
      : await run(false)
    if (written !== null) { done(); setChosenStage(next) }
  }

  const commit = async () => {
    if (preview === null) return
    if (preview.spellBatch !== undefined) await saveSpells(preview.spellBatch.submissions, preview.spellBatch.next, true)
    else await write(preview.row, preview.event, preview.open)
  }

  /**
   * Backing out of the question, and out of the block that asked it.
   *
   * A block whose question was being put again the long way round has nothing
   * to show once the drop is refused -- it was never re-posed -- so it closes
   * with the dialog. One that is being answered stays open, because the answer
   * being reconsidered is still on screen.
   */
  const cancelPreview = () => {
    if (preview?.event === null) setOpenKey(null)
    setPreview(null)
  }

  /**
   * Puts an answered choice's question again without touching the answer.
   *
   * The server stops sending a question once it is answered, but it will say
   * what was being asked at an entry's position. The card then opens on the
   * saved answer and a change replaces the entry in place. This used to drop
   * the entry and wait for the question to come back -- so a player who opened
   * a card to look at what they had picked saw nothing picked, and had in fact
   * just unpicked it. The drop remains only for what cannot be found again: an
   * entry bundling several questions, or a prompt the server no longer poses.
   */
  const repose = async (row: SettledRow) => {
    const answers = row.event.choices ?? []
    const first = answers[0]?.prompt
    const whole = first !== undefined && answers.every((answer) => answer.prompt === first || answer.prompt.startsWith(`${first}/`))
    const posed = whole ? await getPrompts(id, undefined, row.seq).then((response) => response.prompts, () => []) : []
    const prompt = posed.find((each) => each.choice.prompt === first)
    if (prompt === undefined) await price(row, null, reaskedKey(row))
    else setReposed({ seq: row.seq, prompt })
  }

  const savePortrait = async (image: string) => {
    setImageDraft(image)
    if (isNew) return
    const written = await revise.run(id, 1, view.prompts.seq, {
      type: 'init', changes: [{ path: 'identity.image', op: 'set', value: { kind: 'string', string: image } }],
    }, false, view.prompts.revision ?? view.prompts.seq)
    if (written) { setImageDraft(undefined); build.refresh() }
  }

  const submitEvent = (asked: Asking, event: CharacterEvent) => {
    // After saving, the refreshed prompts open the next choice on this tab.
    if (asked.replaces === null) void append(event, null)
    else void price(asked.replaces, event, null)
  }

  const failure = create.error ?? answer.error ?? revise.error ?? remove.error ?? spellSave.error
  const fields: readonly ApiFieldError[] =
    create.fields.length > 0
      ? create.fields
      : answer.fields.length > 0
        ? answer.fields
        : revise.fields

  return {
    create, answer, revise, spellSave, remove,
    preview, reposed, failure, fields,
    createCharacterFromDraft, saveSpells, commit, cancelPreview, repose, savePortrait, submitEvent,
  }
}
