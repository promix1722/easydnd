import { characterPath } from '@/lib/api/characters'
import { DEFAULT_BUILD_POLICY } from '@/lib/api/packPolicy'
import { CatalogScope, RulesEdition, CharacterPolicy } from '@/lib/api/catalogScope'
import { PackSelector } from '@/features/packs'
import { useLocation, useNavigate, useParams, useSearchParams } from 'react-router'

import {
  autoEquip,
  getPrompts,
  describeField,
} from '@/lib/api'
import { useT } from '@/lib/i18n'
import { Avatar, characterAvatar, AvatarEditor, BlockList, Alert, Button, Group, Page, Panel, Stack, TabDeck, Text } from '@/ui'

import type { Asking } from './blocks'
import { askingFor, eventFor, initEventFor, isSpellChoice, maybeLines, maybeScores, reask } from './answers'
import { EMPTY_VIEW, NEW_NAME_KEY, NEW_RULES_KEY, PACKS_KEY, buildTrail, stageAfter, title } from './buildModel'
import { stageLabel } from './labels'
import { CustomNotes } from './CustomNotes'
import { CustomOptionsPanel } from './CustomOptionsPanel'
import { DropPreviewSheet } from './DropPreviewSheet'
import { StagePanel } from './StagePanel'
import { SpellStagePanel } from './SpellStagePanel'
import { useBuildActions } from './useBuildActions'
import { useBuildDrafts } from './useBuildDrafts'
import { useBuildStages } from './useBuildStages'
import { useBuildView } from './useBuildView'

import type { Stage } from '@/domain'

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

  const build = useBuildView(id)
  const view = build.data ?? EMPTY_VIEW
  const drafts = useBuildDrafts()
  const stages = useBuildStages({ id, isNew, landing: location.state, build, view, drafts })
  const actions = useBuildActions({ id, isNew, folder, build, view, drafts, stages, navigate })

  const {
    selectedRules, setSelectedRules, packError, setPackDirty, nameDraft, setNameDraft, imageDraft,
    draftRules, setDraftRules, customDraft, setCustomDraft, nameError, setNameError,
  } = drafts
  const {
    open, visibleStages, stage, blocksByStage, blocks, firstChoice, opened, posingName,
    openKey, setOpenKey, creating, focusNextStage, setFocusNextStage,
    setChosenStage, setAskedOn, setAdvanceAfter,
  } = stages
  const {
    create, answer, revise, spellSave, remove, preview, reposed, failure, fields,
    createCharacterFromDraft, saveSpells, commit, cancelPreview, repose, savePortrait, submitEvent,
  } = actions

  // What the open block is asking, which is a fact about the block rather than
  // a second piece of state. A settled block whose question cannot be put
  // again has none, and says so where its surface would have been.
  const asking: Asking | null =
    opened === null
      ? null
      : opened.kind === 'open'
        ? { prompt: opened.prompt, replaces: null }
        : askingFor(opened.row, reposed)

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
      void repose(block.row)
      return
    }
    // A rename starts from the name it is changing rather than from nothing.
    if (question.choice.kind === 'text') {
      setNameDraft(block.row.value)
    }
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

  // Both ways out of the builder -- the header's Finish, and Next on the last
  // tab once nothing required is open -- go through here. A build equips
  // nothing, so this is where the character is dressed: wired to only one of
  // the two, a character finished by the other arrived with a full backpack
  // and nothing on. The sheet opens either way; a failure here costs an
  // empty paperdoll, not the way out.
  const finish = () => void autoEquip(id).catch(() => undefined).then(() => navigate(`/characters/${id}`))

  return (
    <CharacterPolicy.Provider value={view.prompts.buildPolicy ?? DEFAULT_BUILD_POLICY}><RulesEdition.Provider value={(isNew ? selectedRules : view.rules)?.edition ?? '2014'}><CatalogScope.Provider value={id ? `${characterPath(id)}/catalog` : ''}><Page
      // The draft, while the character it names is being created: the sheet
      // that would say so is the thing still in flight, and a trail that read
      // "Unnamed" for a moment would be naming the one fact just supplied.
      mark={<Avatar image={isNew ? imageDraft : view.sheet?.identity.image} fallback={characterAvatar(isNew ? undefined : view.sheet?.identity.classes)} size={48} />}
      trail={buildTrail(t, isNew, creating ? nameDraft.trim() : title(view) || t('common.unnamed'))}
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
                onClick={finish}
              >
                {t('build.finish')}
              </Button>
            ),
          })}
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
              // Next with nowhere left to go is Finish, once nothing required
              // is open; until then there is no Next on the last tab.
              const next = after !== null ? () => goToStage(after)
                : !posingName && view.prompts.complete ? finish
                : undefined
              return {
                value: each,
                label: stageLabel(t, each),
                // Not a StagePanel: nothing is asked here, and an empty one
                // says "Nothing to answer yet" about a tab that never asks.
                content: each === 'custom' ? (
                  <Stack>
                    <CustomNotes options={view.sheet?.customOptions} characterId={id} onChanged={() => build.refresh()} />
                    {next !== undefined && <Group><Button variant="light" onClick={next}>{t('stagePanel.next')}</Button></Group>}
                  </Stack>
                ) : each === 'spells' || each === 'cantrips' ? (
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
                    // A save lands on the next tab with work, or stays put when
                    // this was the last: Equipment is only there while a kit is chosen.
                    onAnswers={(submissions) => void saveSpells(submissions, each === 'cantrips' && visibleStages.includes('spells') ? 'spells' : after ?? each)}
                    pending={answer.pending || revise.pending || spellSave.pending || remove.pending || build.loading}
                    revision={view.prompts.revision ?? view.prompts.seq}
                    {...(each === 'cantrips' && visibleStages.includes('spells') ? { onNext: () => goToStage('spells') } : next === undefined ? {} : { onNext: next })}
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
                    {...(next === undefined ? {} : { onNext: next })}
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

          <DropPreviewSheet
            preview={preview}
            pending={revise.pending || remove.pending}
            onCommit={() => void commit()}
            onCancel={cancelPreview}
          />
        </Stack>
      </Panel>
    </Page></CatalogScope.Provider></RulesEdition.Provider></CharacterPolicy.Provider>
  )
}
