import { useState } from 'react'

import { STAGES, stageOf } from '@/domain'
import type { Stage } from '@/domain'
import { useT } from '@/lib/i18n'
import type { Resource } from '@/lib/useResource'

import { blockOrder, blocksFor } from './blocks'
import type { Block, BlockOrder } from './blocks'
import { NEW_INITIAL_PROMPTS, NEW_NAME_KEY, firstUnfinished, landingStage } from './buildModel'
import type { BuildView } from './buildModel'
import { creationPrompt } from './creationPrompts'
import { settledByStage } from './settled'
import type { BuildDrafts } from './useBuildDrafts'

/**
 * Which tab a build screen is on and which block is open on it, with the
 * blocks every tab draws.
 *
 * `landing` is the route's state: see `landingStage`.
 */
export function useBuildStages({ id, isNew, landing, build, view, drafts }: {
  id: string
  isNew: boolean
  landing: unknown
  build: Resource<BuildView>
  view: BuildView
  drafts: BuildDrafts
}) {
  const t = useT()
  const { setPackDirty, setPackError, setImageDraft } = drafts

  const [chosenStage, setChosenStage] = useState<Stage | null>(landingStage(landing))
  const [openKey, setOpenKey] = useState<string | null>(null)
  const [seeded, setSeeded] = useState(false)
  const [askedOn, setAskedOn] = useState<Stage | null>(null)
  const [kitSeen, setKitSeen] = useState(false)
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
    setChosenStage(landingStage(landing))
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
  //
  // Equipment is stricter: only for a visit that was asked about a starting
  // kit. The kit is asked once, by the first class at level 1, and it seeds
  // the inventory; what the character carries after that is edited on the
  // sheet, so Edit and a level-up never see the tab. Answering the kit does
  // not take the tab away mid-creation, though: the answer stays changeable
  // until the player leaves.
  // ponytail: "creating" is "this visit saw a kit prompt" -- coming back to a
  // half-built character with the kit answered hides the tab. A stored
  // finished flag is the upgrade.
  const asked = (each: Stage) => open.some((prompt) => stageOf(prompt.group, prompt.choice.kind, prompt.choice.prompt, prompt.purpose) === each)
  if (!kitSeen && asked('equipment')) setKitSeen(true)
  // Custom holds what the player writes unasked, on a character that exists.
  const visibleStages = STAGES.filter((each) => each === 'custom' ? !isNew : each === 'equipment' ? kitSeen || asked(each) : each !== 'cantrips' && each !== 'spells' ||
    asked(each) ||
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

  return {
    open, visibleStages, stage, blocksByStage, blocks, firstChoice, opened, posingName, orderFor,
    openKey, setOpenKey,
    creating, setCreating,
    focusNextStage, setFocusNextStage,
    setChosenStage, setAskedOn, setAdvanceAfter, setOpenAfterCreation,
  }
}

export type BuildStages = ReturnType<typeof useBuildStages>
