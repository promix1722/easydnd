import { characterPath } from '@/lib/api/characters'
import type { CustomOption } from '@/lib/api/characters'
import { DEFAULT_BUILD_POLICY } from '@/lib/api/packPolicy'
import { CatalogScope, RulesEdition, CharacterPolicy } from '@/lib/api/catalogScope'
import { PackSelector } from '@/features/packs'
import { type RulesLock } from '@/lib/api/packs'
import { useState } from 'react'
import { useLocation, useNavigate, useParams, useSearchParams } from 'react-router'

import {
  appendEvents,
  reviseEvents,
  createCharacter,
  deleteEvent,
  getEvents,
  getPrompts,
  getSheet,
  replaceEvent,
  describeField,
} from '@/lib/api'
import type {
  Answer,
  ApiFieldError,
  Change,
  CharacterEvent,
  Dropped,
  Prompt,
  PromptsResponse,
  Sheet,
} from '@/lib/api'
import { useT } from '@/lib/i18n'
import type { Translate } from '@/lib/i18n'
import { useAction } from '@/lib/useAction'
import { useResource } from '@/lib/useResource'
import { Avatar, characterAvatar, AvatarEditor, BlockList, Alert, Badge, Button, Group, ModalSheet, Page, Panel, Stack, TabDeck, Text } from '@/ui'
import type { Crumb } from '@/ui'

import {
  blockOrder,
  blocksFor,
  inheritPlace,
  keyFor,
  keyForRow,
  promptKey,
  reclaimPlace,
  settledKey,
} from './blocks'
import type { Asking, Block, BlockOrder } from './blocks'
import { creationPrompt } from './creationPrompts'
import { eventLabel, stageLabel } from './labels'
import { resolveRefNames } from './refNames'
import { settledByStage, settledPickName } from './settled'
import type { SettledRow } from './settled'
import { CustomOptionsPanel } from './CustomOptionsPanel'
import { StagePanel } from './StagePanel'
import { SpellStagePanel } from './SpellStagePanel'
import type { SpellSubmission } from './SpellStagePanel'
import type { Scores } from './AbilityScoresForm'

import {
  STAGES,
  kindOf,
  promptLabel,
  stageOf,
} from '@/domain'
import type { Stage } from '@/domain'

/** Everything one build screen reads, in one round of requests. */
interface BuildView {
 rules?: RulesLock | undefined
  prompts: PromptsResponse
  events: CharacterEvent[]
  sheet: Sheet | null
  names: Map<string, string>
}

const EMPTY_VIEW: BuildView = {
  prompts: { seq: 0, complete: false, prompts: [] },
  events: [],
  sheet: null,
  names: new Map(),
}

/** A change, priced before it is paid for. */
interface Preview {
  spellBatch?: { submissions: SpellSubmission[]; next: Stage }
  row: SettledRow
  /** Null for a removal: there is nothing to put back. */
  event: CharacterEvent | null
  dropped: Dropped[]
  names: Map<string, string>
  /** The block to open once this is made: see `done`. */
  open: string | null
}

/**
 * Creating a character and changing one, which are the same screen because
 * they are the same question to the API.
 *
 * It is still a loop rather than a wizard. Prompts nest -- answering the "two
 * skills" branch of a rogue's Expertise is what brings the two-skill prompt
 * into existence -- so the number of steps is not knowable until the last one
 * is answered, and there is nothing to enumerate. What the tabs enumerate is
 * the server's own prompt *groups*, which are fixed: five categories a
 * question can belong to, not five steps to walk through. The UI splits these
 * groups into tabs for personal details, rules, classes and their choices.
 * Every tab is reachable, and a completed choice opens the next one there.
 *
 * Three requests, deliberately, because they answer three different questions.
 * `/prompts` says what is still open, `/events` says what was decided and in
 * which entry, and `/sheet` says what all that adds up to -- and the sheet
 * cannot be folded from the log in the browser, because an ability score
 * improvement's increments are derived at projection time.
 *
 * Nothing here decides what an answer means. The server says which event
 * carries it and this copies that verbatim, and the *group* an answer belongs
 * to is written by the server too -- so a dropped answer reappears under its
 * own tab without this screen routing it anywhere.
 */
export function BuildScreen() {
  const t = useT()
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const location = useLocation()
  // The folder the character list was filtered to when New character was pressed.
  // Only /characters/new carries it; absent means the account's default, which
  // the server resolves.
  const [search] = useSearchParams()
  const folder = search.get('folder') ?? undefined
  const isNew = id === ''

  // useResource refreshes on translator changes without unmounting the active draft.
  const build = useResource<BuildView>(`build:${id}`, async (signal) => {
    if (id === '') return EMPTY_VIEW
    const [prompts, log, sheet] = await Promise.all([
      getPrompts(id, signal),
      getEvents(id, signal),
      getSheet(id, signal),
    ])
    return {
      prompts,
      rules: log.rules,
      events: log.events,
      sheet,
      names: await resolveRefNames([
        ...log.events, ...prompts.prompts,
        ...(prompts.spellRules ?? []).flatMap((rule) => [{ source: rule.source }, ...(rule.automatic ?? []).map((slug) => ({ ref: `spell:${slug}` })), ...(rule.listClasses ?? []).map((slug) => ({ ref: `class:${slug}` }))]),
        // An equipment card is titled by what it offers, so those items are
        // named before any card is opened.
        ...prompts.prompts.filter((prompt) => prompt.choice.kind === 'equipment')
          .flatMap((prompt) => (prompt.choice.from.options ?? []).flatMap((option) => [option, ...(option.items ?? [])]))
          .flatMap((option) => option.ref === undefined ? [] : [{ ref: option.ref }]),
        ...[...sheet.equipment.equipped, ...sheet.equipment.backpack, ...sheet.equipment.loot]
          .flatMap((stack) => stack.item === undefined ? [] : [{ ref: `item:${stack.item}` }]),
      ], `${characterPath(id)}/catalog`),
    }
  })

  const [selectedRules, setSelectedRules] = useState<RulesLock | undefined>(undefined)
  const [packError, setPackError] = useState('')
  const [packDirty, setPackDirty] = useState(false)
  const create = useAction(createCharacter)
  const answer = useAction(appendEvents)
  const revise = useAction(replaceEvent)
  const spellSave = useAction(reviseEvents)
  const remove = useAction(deleteEvent)

  const [chosenStage, setChosenStage] = useState<Stage | null>(landingStage(location.state))
  const [openKey, setOpenKey] = useState<string | null>(null)
  const [seeded, setSeeded] = useState(false)
  const [askedOn, setAskedOn] = useState<Stage | null>(null)
  const [nameDraft, setNameDraft] = useState('')
  const [imageDraft, setImageDraft] = useState<string | undefined>(undefined)
  const [draftRules, setDraftRules] = useState<Change[] | null>(null)
  // The custom entry being written, opened from a picker's last option or
  // from the entry's own block.
  const [customDraft, setCustomDraft] = useState<CustomOption | null>(null)
  const [nameError, setNameError] = useState<string | undefined>(undefined)
  const [preview, setPreview] = useState<Preview | null>(null)
  const [creating, setCreating] = useState(false)
  const [advanceAfter, setAdvanceAfter] = useState<{ view: BuildView; stage: Stage } | null>(null)
  const [openAfterCreation, setOpenAfterCreation] = useState<Stage | null>(null)
  const [focusNextStage, setFocusNextStage] = useState<Stage | null>(null)

  /*
   * One order per tab, held once and mutated rather than replaced: see
   * BlockOrder. The list is redrawn by new data, never by the memory of where
   * the old data sat.
   *
   * It was a single order while a single tab was rendered. Now that every tab
   * is a slide and all five are drawn at once, one shared order would let the
   * place a dropped answer vacated on the class tab be claimed by whatever
   * arrived next under background -- and where a block sits is a fact about
   * the tab it sits on, so there is one per tab.
   */
  const [orders] = useState(() => new Map<Stage, BlockOrder>())
  const orderFor = (each: Stage): BlockOrder => {
    const held = orders.get(each)
    if (held !== undefined) return held
    const made = blockOrder()
    orders.set(each, made)
    return made
  }

  /*
    Creation changes the URL under a screen that is not replaced.

    Both routes render this component, so React reuses the instance rather than
    mounting a second one -- which is why creating used to move the player to
    another tab without anything in the code saying "advance". No tab had been
    chosen, the reread brought the first real prompts back, and
    `firstUnfinished` below answered the only question it is asked: class.
    Typing a name is not a request to be asked about a class.

    So the tab the gesture aimed at rides across in the route's state, and is
    taken up here, when the character the screen is looking at changes. The
    initialiser above covers the other way in -- a deep link, or a router that
    does remount -- and neither path can be relied on alone.
  */
  const [shownId, setShownId] = useState(id)
  const arriving = shownId !== id
  if (arriving) {
    setPackDirty(false)
    setPackError('')
    if (!creating) orders.clear()
    if (!creating) setImageDraft(undefined)
    setShownId(id)
    setChosenStage(landingStage(location.state))
    setOpenKey(creating ? NEW_NAME_KEY : null)
  }
  /*
    The character has arrived, so this is an ordinary build screen again.

    Both guards are load-bearing, and both were learned the hard way. On the
    render that first sees the new id, `useResource` has not yet reset itself
    -- it does that during its own render, and what it returns *this* time is
    still the previous key's answer, which for a character that did not exist
    is an empty view that reads as `ready`. So "there is data" is true on
    exactly the render where it means nothing, and clearing there put the
    spinner back a render later. And on `/characters/new` there is no read to
    be in the middle of at all, so "not loading" is true from the moment the
    flag is set. What has to be true is that the id has settled *and* the read
    it started has finished.
  */
  if (creating && !arriving && !isNew && !build.loading) setCreating(false)

  const view = build.data ?? EMPTY_VIEW
  const open = view.prompts.prompts.flatMap((prompt) => {
    const supported = creationPrompt(prompt)
    return supported === null ? [] : [supported]
  })
  // Until a tab is clicked the screen opens on the first thing left to do,
  // and moves on as things are answered. That is the loop, kept: a player who
  // never touches a tab is walked through the questions in order, and one who
  // does is pinned where they put themselves.
  const settled = settledByStage(t, view)
  // Spell tabs only have work when the server offers a choice or the player
  // has a saved selection to edit. A non-caster should never have to pass
  // through empty Cantrips and Spells tabs.
  const visibleStages = STAGES.filter((each) => each !== 'cantrips' && each !== 'spells' ||
    open.some((prompt) => stageOf(prompt.group, prompt.choice.kind, prompt.choice.prompt, prompt.purpose) === each) ||
    (settled.get(each)?.length ?? 0) > 0 || view.sheet?.customOptions?.some((option) => option.kind === (each === 'cantrips' ? 'cantrip' : 'spell')))
  const preferredStage = chosenStage ?? (isNew ? 'rules' : firstUnfinished(open))
  const stage = visibleStages.includes(preferredStage) ? preferredStage : firstUnfinished(open)

  // A stage change driven by fresh prompts can leave a key from the previous
  // tab. Explicit tab navigation sets its own first choice below.
  if (askedOn !== stage) {
    setAskedOn(stage)
    setOpenKey(null)
  }
  // While the character is being created there is no log to read yet, so the
  // question it is being created by is still the thing on screen.
  const posingName = isNew || creating
  /*
   * Every tab's blocks, not just the one on screen, because every tab is a
   * slide and the slide you are swiping towards has to be drawn before you
   * arrive at it.
   *
   * Before there is a character there is no `/prompts` response, so the
   * Personal poses the name itself: see NEW_NAME_PROMPT. Rules and Class
   * show the other initial questions on their own tabs.
   *
   * The order is the screen's memory of where things are, and `blocksFor` both
   * reads it and writes to it: whatever is new keeps the place the level
   * ordering just gave it, so the next answer moves nothing already on screen.
   */
  const blocksByStage = new Map<Stage, Block[]>(
    STAGES.map((each) => [
      each,
      blocksFor(
        settled.get(each) ?? [],
        (posingName ? NEW_INITIAL_PROMPTS : open).filter((prompt) =>
          stageOf(prompt.group, prompt.choice.kind, prompt.choice.prompt, prompt.purpose) === each),
        orderFor(each),
      ),
    ]),
  )
  const blocks = blocksByStage.get(stage) ?? []
  const firstChoice = (on: Stage) => blocksByStage.get(on)?.find((block) => block.kind === 'open')?.key ?? null
  // The first tab on a loaded build is entered just like a clicked tab.
  if (!seeded && !build.loading) {
    setSeeded(true)
    setOpenKey(firstChoice(stage))
  }
  if (openAfterCreation === stage && !creating && !build.loading) {
    setOpenAfterCreation(null)
    const next = firstChoice(stage)
    setOpenKey(next)
    if (next === null) setFocusNextStage(stage)
  }
  // Wait for the refreshed prompts: an answer can create new choices on this tab.
  if (advanceAfter !== null && (advanceAfter.stage !== stage || advanceAfter.view !== view)) {
    setAdvanceAfter(null)
    if (advanceAfter.stage === stage) {
      const next = firstChoice(stage)
      setOpenKey(next)
      if (next === null) setFocusNextStage(stage)
    }
  }
  const activeKey = openKey
  const opened = blocks.find((block) => block.key === activeKey) ?? null
  // What the open block is asking, which is a fact about the block rather than
  // a second piece of state. A settled block whose question cannot be put
  // again has none, and says so where its surface would have been.
  const asking: Asking | null =
    opened === null
      ? null
      : opened.kind === 'open'
        ? { prompt: opened.prompt, replaces: null }
        : askingFor(opened.row)

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

  const goToStage = (next: Stage) => {
    if (next === stage) return
    setFocusNextStage(null)
    // Rules and name can be visited before creation. Other tabs still need a
    // character, so entering one of those creates it from the name draft.
    if (isNew) {
      if (next === 'rules' || next === 'personal') {
        setChosenStage(next)
        setAskedOn(next)
        setOpenKey(next === 'rules' ? NEW_RULES_KEY : NEW_NAME_KEY)
        return
      }
      void createCharacterFromDraft(next)
      return
    }
    setAdvanceAfter(null)
    setChosenStage(next)
    setAskedOn(next)
    const first = firstChoice(next)
    setOpenKey(first)
    if (first === null) setFocusNextStage(next)
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
   * Opening a block, which for a settled one is putting its question again.
   *
   * Every decided block opens onto the question that decided it, and there is
   * no second gesture anywhere on this screen. Where the question cannot be
   * re-posed -- a nested prompt, whose options came with a prompt the server
   * stopped emitting -- putting it again means dropping the entry, which is
   * the same thing reached from the other side: the question comes back
   * outstanding, in the block's own place. That costs nothing else in the
   * usual case, and where it does, `price` asks first.
   */
  const openBlock = (key: string | null) => {
    setOpenKey(key)
    setFocusNextStage(null)
    const block = key === null ? null : blocks.find((each) => each.key === key)
    if (block?.kind !== 'settled') return
    if (isSpellChoice(block.row)) return
    const question = reask(block.row)
    if (question === null) {
      void price(block.row, null, reaskedKey(block.row))
      return
    }
    // A rename starts from the name it is changing rather than from nothing.
    if (question.choice.kind === 'text') {
      setNameDraft(block.row.value)
    }
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

  /*
    The whole page comes down for a first load, and only for a first load.

    `useResource` blanks when its key changes, which is right -- a different
    character is a different screen. But creation changes the key from
    `build:` to `build:chr_1` under a screen that is already up, and taking it
    down meant that typing a name tore the page off and rebuilt it, spinner
    and all, for a write that had already succeeded. It read as a reload
    because that is exactly what it looked like.

    So the creating transition keeps the page: the same chrome, the same tab,
    and the name still in the block it was typed into with its button turning.
    What replaces it a moment later is that block with an answer in it.
  */
  if (build.loading && !creating) {
    return (
      <Page
        mark={<Avatar image={isNew ? imageDraft : view.sheet?.identity.image} fallback={characterAvatar(isNew ? undefined : view.sheet?.identity.classes)} size={48} />}
        trail={buildTrail(t, isNew, null)}
        state={{ kind: 'loading', what: t('build.loading') }}
      />
    )
  }
  if (build.error !== null) {
    return (
      <Page
        mark={<Avatar image={isNew ? imageDraft : view.sheet?.identity.image} fallback={characterAvatar(isNew ? undefined : view.sheet?.identity.classes)} size={48} />}
        trail={buildTrail(t, isNew, null)}
        state={{
          kind: 'failed',
          title: t('build.loadFailed'),
          detail: build.error,
          onRetry: build.reload,
        }}
      />
    )
  }

  const failure = create.error ?? answer.error ?? revise.error ?? remove.error ?? spellSave.error
  const fields: readonly ApiFieldError[] =
    create.fields.length > 0
      ? create.fields
      : answer.fields.length > 0
        ? answer.fields
        : revise.fields

  return (
    <CharacterPolicy.Provider value={view.prompts.buildPolicy ?? DEFAULT_BUILD_POLICY}><RulesEdition.Provider value={(isNew ? selectedRules : view.rules)?.edition ?? '2014'}><CatalogScope.Provider value={id ? `${characterPath(id)}/catalog` : ''}><Page
      // The draft, while the character it names is being created: the sheet
      // that would say so is the thing still in flight, and a trail that read
      // "Unnamed" for a moment would be naming the one fact just supplied.
      mark={<Avatar image={isNew ? imageDraft : view.sheet?.identity.image} fallback={characterAvatar(isNew ? undefined : view.sheet?.identity.classes)} size={48} />}
      trail={buildTrail(t, isNew, creating ? nameDraft.trim() : title(view))}
      /*
       * On the heading line, against the right edge, and only once there is a
       * character to finish.
       *
       * It sat against the last tab for a while, which is where `TabRow` had a
       * slot for it. Two things were wrong with that. It is not a control on
       * the tabs -- it acts on the character, which is exactly what `Page`'s
       * `actions` slot is for and what the sheet's own button already uses --
       * and the tabs plus a button do not fit across 390px, so the strip that
       * is supposed to scroll was competing with it for the width.
       */
      {...(posingName
        ? {}
        : {
            actions: (
              <Button
                variant={view.prompts.complete ? 'filled' : 'light'}
                onClick={() => void navigate(`/characters/${id}`)}
              >
                {t('build.finish')}
              </Button>
            ),
          })}
      {...(!posingName && view.prompts.complete
        ? { subtitle: t('build.subtitleComplete') }
        : {})}
    >
      <Panel>
        <Stack gap="lg">

          {failure !== null && (
            <Alert color="red" title={t('build.rejected')}>
              <Stack gap={4}>
                <Text size="sm">{failure}</Text>
                {fields.map((field) => (
                  <Text key={field.field} size="xs" c="dimmed">
                    {describeField(t, field)}
                  </Text>
                ))}
              </Stack>
            </Alert>
          )}

          <TabDeck
            // "Character build" rather than the character's name, which is
            // already the crumb above this. A landmark renamed per character
            // would give a screen-reader user a different table of contents on
            // every build.
            label={t('build.deckLabel')}
            value={stage}
            onChange={(next) => goToStage(next as Stage)}
            /*
              Not until there is a character. While one is being posed for, a tab
              press *creates* it rather than moving anywhere, so a swipe would be
              a gesture the screen answers by refusing to move -- and a deck that
              snaps back is worse than one that never gives.
            */
            swipeable={!posingName}
            panels={visibleStages.map((each) => {
              // Where Next goes from *this* tab, which is a fact about the tab
              // and not about the one on screen -- every panel is mounted, so
              // every panel's button has to be its own.
              // Custom entries are blocks of the tab they belong to, above its
              // Next, not a section of their own under the deck.
              const customs = (on: Stage) => !isNew && view.sheet !== null
                ? <CustomOptionsPanel id={id} stage={on} modal={on === stage} sheet={view.sheet} revision={view.prompts.revision ?? view.prompts.seq} onSaved={() => build.refresh()} draft={customDraft} onDraft={setCustomDraft} />
                : null
              const after = each === 'rules' && isNew && draftRules !== null ? 'personal' : stageAfter(each, open)
              return {
                value: each,
                label: stageLabel(t, each),
                content: each === 'spells' || each === 'cantrips' ? (
                  <SpellStagePanel
                    cantripsOnly={each === 'cantrips'}
                    rules={view.prompts.spellRules ?? []}
                    blocks={blocksByStage.get(each) ?? []}
                    active={each === stage}
                    names={view.names}
                    loadSavedPrompt={async (row) => {
                      const response = await getPrompts(id, undefined, row.seq)
                      const prompt = response.prompts.find((item) => item.choice.prompt === row.event.choices?.[0]?.prompt)
                      if (prompt === undefined) throw new Error(t('prompt.nothingOffered'))
                      return prompt
                    }}
                    onAnswers={(submissions) => void saveSpells(submissions, each === 'cantrips' && visibleStages.includes('spells') ? 'spells' : 'equipment')}
                    pending={answer.pending || revise.pending || spellSave.pending || remove.pending || build.loading}
                    revision={view.prompts.revision ?? view.prompts.seq}
                    onNext={() => goToStage(each === 'cantrips' && visibleStages.includes('spells') ? 'spells' : 'equipment')}
                  >{customs(each)}</SpellStagePanel>
                ) : (
                  <Stack>
                  <StagePanel
                    blocks={blocksByStage.get(each) ?? []}
                    openKey={openKey}
                    onOpen={openBlock}
                    asking={asking}
                    names={view.names}
                    onAnswerPicks={(asked, answers) =>
                      submitEvent(asked, eventFor(asked.prompt, answers))
                    }
                    onNameChange={(next) => {
                      setNameDraft(next)
                      setNameError(undefined)
                    }}
                    onAnswerName={(asked, next) => {
                      if (isNew) void createCharacterFromDraft('personal')
                      else submitEvent(asked, initEventFor(next))
                    }}
                    onAnswerChanges={(asked, changes) => {
                      if (isNew && asked.prompt.choice.prompt === 'character/ruleset') {
                        setDraftRules(changes)
                        setOpenKey(PACKS_KEY)
                        setFocusNextStage(null)
                      } else submitEvent(asked, { type: asked.prompt.event.type, changes })
                    }}
                    pending={
                      creating || create.pending || answer.pending || revise.pending || remove.pending
                    }
                    fields={fields}
                    {...(after === null ? {} : { onNext: () => goToStage(after) })}
                    {...(posingName || asking?.prompt.choice.kind === 'text'
                      ? { name: nameDraft }
                      : {})}
                    {...maybeScores(asking?.replaces ?? null)}
                    {...maybeLines(asking?.replaces ?? null)}
                    {...(view.sheet !== null ? { level: view.sheet.identity.level } : {})}
                    posing={posingName}
                    rulesSelected={draftRules !== null}
                    focusNext={focusNextStage === each}
                  >
                  {each === 'rules' && <PackSelector key={id} disclosure={{ open: openKey === PACKS_KEY, onOpen: (open) => openBlock(open ? PACKS_KEY : null) }} onDirtyChange={setPackDirty} value={isNew ? selectedRules : view.rules} finalized={!isNew || selectedRules !== undefined} onChange={(rules) => {
                    if (isNew) {
                      setSelectedRules(rules)
                      if (draftRules?.[0]?.value.slug !== rules.edition) { setDraftRules(null); setOpenKey(NEW_RULES_KEY) }
                      else { setOpenKey(null); setFocusNextStage('rules') }
                      return
                    }
                  }}>
                  {packError && <Alert color="red">{packError}</Alert>}
                  </PackSelector>}
                  {each === 'personal' && <BlockList open={openKey} onOpen={openBlock} items={[{
                    key: 'portrait',
                    header: <Text size="sm">{t('avatar.portrait')}</Text>,
                    body: <AvatarEditor fallback={characterAvatar(isNew ? undefined : view.sheet?.identity.classes)} image={imageDraft ?? view.sheet?.identity.image} pending={revise.pending || creating} error={revise.error} onSave={savePortrait} />,
                  }]} />}
                  {customs(each)}
                  </StagePanel>
                  </Stack>
                ),
              }
            })}
          />

          {nameError !== undefined && (
            <Text size="sm" c="red">
              {nameError}
            </Text>
          )}

          {/*
            Only ever open because something would be lost. A change that costs
            nothing else is simply made -- see `price` -- so this dialog means one
            thing and never cries wolf.
          */}
          <ModalSheet
            opened={preview !== null}
            onClose={() => cancelPreview()}
            title={preview?.event === null ? t('build.askAgainTitle') : t('build.changeTitle')}
          >
            {preview !== null && (
              <Stack gap="md">
                <Text size="sm">
                  {preview.event === null
                    ? t('build.dropWarningAskAgain', { count: preview.dropped.length })
                    : t('build.dropWarningChange', { count: preview.dropped.length })}
                </Text>

                {preview.dropped.length > 0 && (
                  <Stack gap="xs">
                    {preview.dropped.map((entry) => (
                      <div key={entry.seq}>
                        <Group gap={6}>
                          <Text size="sm" fw={500}>
                            {eventLabel(t, entry.type)}
                            {entry.ref !== undefined && `: ${preview.names.get(entry.ref) ?? entry.ref}`}
                          </Text>
                          <Badge size="xs" variant="light" color="gray">
                            {reasonLabel(t, entry.reason)}
                          </Badge>
                        </Group>
                        {(entry.lost ?? []).map((lost) => (
                          <Text key={lost.prompt} size="xs" c="dimmed">
                            {promptLabel(lost.prompt)}
                            {lost.picks !== undefined &&
                              `: ${lost.picks
                                .map((pick) => settledPickName(t, pick, preview.names))
                                .join(', ')}`}
                          </Text>
                        ))}
                      </div>
                    ))}
                    <Text size="xs" c="dimmed">
                      {t('build.dropReassurance')}
                    </Text>
                  </Stack>
                )}

                <Group>
                  <Button
                    color="red"
                    loading={revise.pending || remove.pending}
                    onClick={() => void commit()}
                  >
                    {preview.event === null ? t('build.askAgain') : t('answer.changeIt')}
                  </Button>
                  <Button variant="subtle" onClick={() => cancelPreview()}>
                    {t('common.cancel')}
                  </Button>
                </Group>
              </Stack>
            )}
            </ModalSheet>
        </Stack>
      </Panel>
    </Page></CatalogScope.Provider></RulesEdition.Provider></CharacterPolicy.Provider>
  )
}

/**
 * The trail for a build page.
 *
 * Two crumbs either way: the character's name, or "New character" while there
 * is nothing yet to name. It used to end in a third reading "Creation", which
 * said what the screen does -- but the heading of a screen full of questions
 * is not asked what it is, and the word cost a line on a 390px trail to say
 * something the questions underneath it were already saying.
 */
function buildTrail(t: Translate, isNew: boolean, name: string | null): Crumb[] {
  if (isNew) return [{ label: t('characters.newCharacter') }]
  return [{ label: name }]
}

/**
 * The tab creation asked to land on, out of the route state it rode in on.
 *
 * Unknown or absent is null rather than a guess: a link somebody typed carries
 * no state, and a state naming a tab this build does not have is a client that
 * has moved on. Both mean "open where you would have opened anyway".
 */
function landingStage(state: unknown): Stage | null {
  const named = (state as { stage?: unknown } | null)?.stage
  return typeof named === 'string' && STAGES.includes(named as Stage) ? (named as Stage) : null
}

/**
 * The question a dropped entry puts back, named from the entry itself.
 *
 * An answer to a nested prompt cannot be re-posed directly, so opening it drops
 * it and the server emits that prompt again -- under the slug the entry
 * answered, which is the one thing the entry does say.
 */
function reaskedKey(row: SettledRow): string | null {
  const answered = (row.event.choices ?? [])[0]?.prompt
  return answered === undefined ? null : promptKey(answered)
}

/** How a drop reason reads, without borrowing a category's word. */
const REASONS = {
  'not-offered': 'build.reason.notOffered',
  'answers-dropped': 'build.reason.answersDropped',
  empty: 'build.reason.empty',
} as const

/** Why an entry was dropped, or the server's own word if this client is behind. */
function reasonLabel(t: Translate, reason: string): string {
  const key = REASONS[reason as keyof typeof REASONS]
  return key === undefined ? reason : t(key)
}

/**
 * The prompt a character with no log at all is asked.
 *
 * The server emits the real one -- `character/init` -- as soon as a character
 * exists to have an empty log. Before that there is no character to ask about
 * and no request to make, so Personal poses the same question itself
 * and the answer is a creation rather than an append.
 */
const NEW_NAME_PROMPT: Prompt = {
  choice: { prompt: 'character/init', choose: 1, kind: 'text', from: { kind: 'explicit' } },
  group: 'identity',
  optional: false,
  event: { type: 'init' },
  heldOnly: false,
}

/**
 * The initial questions, shown on Rules, Personal and Class before creation.
 *
 * Rules can be chosen before creation and saved with the first character
 * write. Level still needs an existing character to be answered.
 */
const NEW_RULES_PROMPT: Prompt = {
  choice: { prompt: 'character/ruleset', choose: 1, kind: 'text', from: { kind: 'explicit' } },
  group: 'identity',
  optional: false,
  event: { type: 'change' },
  heldOnly: false,
}

const NEW_INITIAL_PROMPTS: Prompt[] = [
  NEW_RULES_PROMPT,
  NEW_NAME_PROMPT,
  {
    choice: {
      prompt: 'character/desired-level',
      choose: 1,
      kind: 'level',
      from: { kind: 'explicit' },
    },
    group: 'identity',
    optional: false,
    event: { type: 'change' },
    heldOnly: false,
  },
]

const NEW_NAME_KEY = keyFor({ prompt: NEW_NAME_PROMPT, replaces: null })
const PACKS_KEY = 'rule-packs'
const NEW_RULES_KEY = keyFor({ prompt: NEW_RULES_PROMPT, replaces: null })

/**
 * The question behind a settled block, where there is one to put again.
 *
 * Null is an answer rather than a failure: a nested prompt cannot be re-posed
 * from here, and the block says so and offers the drop instead.
 */
function isSpellChoice(row: SettledRow): boolean {
  return row.stage === 'spells' || row.stage === 'cantrips'
}

function askingFor(row: SettledRow): Asking | null {
  const prompt = reask(row)
  return prompt === null ? null : { prompt, replaces: row }
}

/** The header's subject: what the character is called, or that it is not. */
function title(view: BuildView): string {
  const name = view.sheet?.identity.name ?? ''
  return name === '' ? 'Unnamed' : name
}

/**
 * The first category with something required still open.
 *
 * Optional prompts do not count. A finished character always has one -- the
 * offer of another level -- so counting them would mean no character was ever
 * anywhere but wherever that offer lives.
 */
function firstUnfinished(prompts: readonly Prompt[]): Stage {
  const required = new Set(
    prompts.filter((p) => !p.optional).flatMap((p) => [stageOf(p.group, p.choice.kind, p.choice.prompt, p.purpose)].filter(isStage)),
  )
  const any = new Set(prompts.flatMap((p) => [stageOf(p.group, p.choice.kind, p.choice.prompt, p.purpose)].filter(isStage)))
  return STAGES.find((s) => required.has(s)) ?? STAGES.find((s) => any.has(s)) ?? 'personal'
}

/**
 * The next category with something still open, wrapping round.
 *
 * Follow display order, including optional questions such as Personality.
 * Only wrap to an earlier tab for required work that is still outstanding.
 */
function stageAfter(stage: Stage, prompts: readonly Prompt[]): Stage | null {
  const all = new Set(prompts.flatMap((p) => [stageOf(p.group, p.choice.kind, p.choice.prompt, p.purpose)].filter(isStage)))
  const required = new Set(prompts.filter((p) => !p.optional)
    .flatMap((p) => [stageOf(p.group, p.choice.kind, p.choice.prompt, p.purpose)].filter(isStage)))
  // Someone who entered a name before choosing rules should see Rules next.
  if (stage === 'personal' && required.has('rules')) return 'rules'
  const from = STAGES.indexOf(stage)
  return STAGES.slice(from + 1).find((s) => all.has(s))
    ?? STAGES.slice(0, from).find((s) => required.has(s))
    ?? null
}

function isStage(stage: Stage | null): stage is Stage {
  return stage !== null
}

/**
 * The questions whose answer is a value on the sheet rather than a pick.
 *
 * The server's projector calls these the character's *inputs* -- a name, the
 * six ability scores, an alignment -- and applies them before anything derives
 * from them. They are the one family of answer that arrives as an addressed
 * change, because there is nothing for them to hang off: no catalogue entry is
 * being named, and no grant posed them. The prompt says the entry is a
 * `change`; what it cannot say is which path, so the path lives here.
 *
 * A name and the scores have a form each and never reach `eventFor`. An
 * alignment is an ordinary choose-one-of-these, which is exactly why it was
 * wrong for so long: the prompt is namespaced `character/`, like a race and a
 * class, and this screen took the namespace for the shape and posted a
 * `change` event that named an alignment and changed nothing. The server
 * accepted it, attributed it to no prompt, and the alignment stayed unset --
 * the worst kind of failure, which is the silent one.
 */
const INPUTS: readonly {
  prompt: string
  path: string
  kind: string
  /** Absent where the answer is written rather than picked from a set. */
  collection?: string
}[] = [
  {
    prompt: 'character/alignment',
    path: 'identity.alignment',
    kind: 'alignment',
    collection: 'alignment',
  },
  // The desired level and the ruleset are the character's own questions about
  // itself. Both are answered as changes by a form of their own -- see
  // AnswerSurface, which routes them by slug -- so `kind` here only has to
  // survive the round trip through `inputPrompt`.
  { prompt: 'character/desired-level', path: 'identity.desiredLevel', kind: 'level' },
  { prompt: 'character/ruleset', path: 'identity.ruleset', kind: 'text' },
  // The four the player answers in their own words. They are inputs for the
  // same reason the alignment is -- they settle a value and name nothing in
  // the compendium -- and they have no collection because there is nothing to
  // choose between. See features/character/promptNames for what each is called.
  { prompt: 'character/personality-trait', path: 'identity.personalityTraits', kind: 'personality' },
  { prompt: 'character/ideal', path: 'identity.ideals', kind: 'ideal' },
  { prompt: 'character/bond', path: 'identity.bonds', kind: 'bond' },
  { prompt: 'character/flaw', path: 'identity.flaws', kind: 'flaw' },
]

/** The prompt an entry's changes settle, where they settle one. */
function inputOf(event: CharacterEvent): (typeof INPUTS)[number] | undefined {
  const paths = new Set((event.changes ?? []).map((change) => change.path))
  return INPUTS.find((input) => paths.has(input.path))
}

/** The question an input poses, so that answering it again is one mechanism. */
function inputPrompt(input: (typeof INPUTS)[number], stage: Stage): Prompt {
  return {
    choice: {
      prompt: input.prompt,
      choose: 1,
      kind: input.kind,
      from:
        input.collection === undefined
          ? { kind: 'explicit' }
          : { kind: 'collection', collection: input.collection },
    },
    group: stage === 'personal' || stage === 'rules' ? 'identity' : stage,
    optional: false,
      event: { type: 'change' },
    heldOnly: false,
  }
}

/**
 * The question behind a settled entry, posed again -- or null where it cannot
 * be.
 *
 * An entry that names a catalogue reference can be re-asked from the entry
 * alone: the kind of the reference says which collection the answers come
 * from, which is a compendium question rather than a rule. So can the two
 * forms, which pose themselves. What cannot is an answer to a nested prompt --
 * a rogue's Expertise, a half-elf's ability bonuses -- because the options
 * that made it up arrived with a prompt the server stopped emitting the moment
 * it was answered, and rebuilding them here would be this client deciding what
 * an answer means.
 */
/**
 * The entry kinds whose options are narrowed by another entry the character
 * holds, and which this client therefore cannot re-pose.
 *
 * Race, class and background draw on their whole collection, so the question
 * is rebuildable from the answer alone. These two are not.
 */
const NARROWED = ['subrace', 'subclass']

function reask(row: SettledRow): Prompt | null {
  const event = row.event
  if (event.type === 'init') {
    return { ...NEW_NAME_PROMPT, group: row.stage }
  }
  // Checked before the reference, because an entry with both is a follow-up:
  // the ref names what posed the question, and re-asking *that* would change
  // the wrong selection.
  if ((event.choices ?? []).length > 0) return null
  if (event.ref !== undefined) {
    const kind = kindOf(event.ref)
    // A subrace and a subclass are narrowed by the entry above them: the
    // server offers a half-elf's subraces and a rogue's archetypes, not every
    // one in the compendium. Rebuilding the question here would offer all of
    // them -- Berserker, Champion, Devotion to a rogue -- so it is not
    // rebuilt. Opening the block drops the entry instead, which reaches the
    // same place from the other side: the server poses the question again,
    // with the right list, and the drop is priced like any other.
    if (NARROWED.includes(kind)) return null
    return {
      choice: {
        prompt: `character/${kind}`,
        choose: 1,
        kind,
        from: { kind: 'collection', collection: kind },
      },
      group: row.stage,
      optional: false,
      event: {
        type: event.type,
        ...(event.level !== undefined ? { level: event.level } : {}),
      },
      heldOnly: false,
    }
  }
  const input = inputOf(event)
  if (input !== undefined) {
    return inputPrompt(input, row.stage)
  }
  if (isScores(event)) {
    return {
      choice: {
        prompt: 'character/abilities',
        choose: 6,
        kind: 'ability-scores',
        from: { kind: 'explicit' },
      },
      group: row.stage,
      optional: false,
          event: { type: event.type },
      heldOnly: false,
    }
  }
  return null
}

function isScores(event: CharacterEvent): boolean {
  return (event.changes ?? []).some((change) => change.path.startsWith('abilities.'))
}

/**
 * The scores as they were *stored*, for a form that is changing them.
 *
 * Deliberately not the projected ones on the sheet. Those have the racial
 * bonuses added, and seeding a form from them would add the bonuses a second
 * time the moment it was saved.
 */
function maybeScores(row: SettledRow | null): { scores?: Scores; method?: string } {
  if (row === null || !isScores(row.event)) return {}
  const scores: Scores = {}
  let method: string | undefined
  for (const change of row.event.changes ?? []) {
    const [head, tail] = change.path.split('.')
    if (head !== 'abilities' || tail === undefined) continue
    if (tail === 'method') method = change.value.slug ?? change.value.string
    else if (change.value.int !== undefined) scores[tail] = change.value.int
  }
  return { scores, ...(method !== undefined ? { method } : {}) }
}

/**
 * What an entry wrote to one path, in the order it wrote it.
 *
 * The list is stored as a `set` followed by `add`s -- see `WrittenForm` -- so
 * reading it back is reading the values in order, and nothing here has to know
 * which op was which.
 */
function linesOf(event: CharacterEvent, path: string): string[] {
  return (event.changes ?? [])
    .filter((change) => change.path === path)
    .flatMap((change) => (change.value.string === undefined ? [] : [change.value.string]))
}

/**
 * What a settled written answer says, for the form that is changing it.
 *
 * Nothing at all for every other kind of entry, which is what keeps the prop
 * off surfaces that would have no use for it.
 */
function maybeLines(row: SettledRow | null): { lines?: readonly string[] } {
  if (row === null) return {}
  const input = inputOf(row.event)
  if (input === undefined || input.collection !== undefined) return {}
  return { lines: linesOf(row.event, input.path) }
}

/** A name, as the entry that carries one. */
function initEventFor(name: string): CharacterEvent {
  return {
    type: 'init',
    changes: [
      { path: 'identity.name', op: 'set', value: { kind: 'string', string: name } },
    ],
  }
}

/** The entry a prompt said its answer travels in, filled in with the answer. */
function eventFor(prompt: Prompt, answers: readonly Answer[]): CharacterEvent {
  // The question itself is the first answer; anything after it answers a
  // branch the first one opened, resolved in the same card.
  const picks = answers[0]?.picks ?? []

  // An input settles a value on the sheet, so its answer is the change that
  // settles it rather than a pick attached to an entry that means nothing.
  const input = INPUTS.find((each) => each.prompt === prompt.choice.prompt)
  if (input !== undefined) {
    return {
      type: prompt.event.type,
      changes: [
        { path: input.path, op: 'set', value: { kind: 'slug', slug: picks[0] ?? '' } },
      ],
    }
  }

  const event: CharacterEvent = {
    type: prompt.event.type,
    ...(prompt.event.ref !== undefined ? { ref: prompt.event.ref } : {}),
    ...(prompt.event.level !== undefined ? { level: prompt.event.level } : {}),
  }
  // A prompt that selects a catalogue entry carries its answer in the event's
  // ref; every other prompt carries it in the choices.
  //
  // All of them, in the order they were given. The server validates a batch
  // answer by answer against a log that grows as each lands, so a branch and
  // what the branch opened travel together: the second is legal because the
  // first one arrived.
  if (selectsTheEventItself(prompt)) {
    event.ref = `${refKindFor(prompt)}:${picks[0] ?? ''}`
  } else {
    event.choices = answers.map((answer) => ({ prompt: answer.prompt, picks: answer.picks }))
  }
  return event
}

/**
 * Whether the answer goes in the event's ref rather than its choices.
 *
 * "Choose a race" is answered by *being* a race event that names one, not by
 * an answer attached to a race event that names nothing.
 *
 * The `character/` namespace is not the test, though it reads like one: the
 * character poses its own inputs under it too, and those are neither. Those
 * are taken by `eventFor` before this is asked.
 */
function selectsTheEventItself(prompt: Prompt): boolean {
  return prompt.choice.prompt.startsWith('character/') || prompt.choice.prompt.endsWith('/subclass')
}

function refKindFor(prompt: Prompt): string {
  const set = prompt.choice.from
  if (set.kind === 'collection' && set.collection !== undefined) return set.collection
  const first = set.options?.[0]
  return first?.ref?.split(':')[0] ?? 'race'
}
