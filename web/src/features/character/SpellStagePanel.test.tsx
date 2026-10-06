import { screen, waitFor, within } from '@testing-library/react'
import { beforeEach, expect, it, vi } from 'vitest'

import type { Prompt, Spell } from '@/lib/api'
import { renderAt } from '@/test/render'
import { spellFetch } from '@/test/spells'
import { setupUser } from '@/test/user'
import type { Block } from './blocks'
import { SpellStagePanel } from './SpellStagePanel'

const spells: Spell[] = [
  { slug: 'fireball', name: 'Fireball', level: 3 },
  { slug: 'shield', name: 'Shield', level: 1 },
  { slug: 'alarm', name: 'Alarm', level: 1, ritual: true },
]


const question = (owner: string, choose = 1): Prompt => ({
  choice: { prompt: `${owner}/spell/known/5`, kind: 'spell', choose,
    from: { kind: 'explicit', options: spells.map((spell) => ({ key: spell.slug, kind: 'ref', ref: `spell:${spell.slug}` })) } },
  purpose: 'known', source: `class:${owner}`, group: 'class', level: 5,
  event: { type: 'level', ref: `class:${owner}`, level: 5 }, heldOnly: false, optional: false,
})
const blocks: Block[] = [
  { key: 'wizard', kind: 'settled', changeable: true, row: { seq: 2, stage: 'spells', label: 'Wizard · Spells known', value: 'Fireball',
    event: { type: 'level', choices: [{ prompt: 'wizard/spell/known/5', picks: ['fireball'] }] } } },
  { key: 'sorcerer', kind: 'settled', changeable: true, row: { seq: 3, stage: 'spells', label: 'Sorcerer · Spells known', value: 'Shield',
    event: { type: 'level', choices: [{ prompt: 'sorcerer/spell/known/5', picks: ['shield'] }] } } },
]
const answer = vi.fn()
function Panel() {
  return <SpellStagePanel active blocks={blocks} loadSavedPrompt={async (row) => question(row.seq === 2 ? 'wizard' : 'sorcerer')}
    names={new Map()} onAnswers={answer} pending={false} revision={3} />
}

beforeEach(() => {
  answer.mockClear()
  vi.stubGlobal('fetch', vi.fn(spellFetch(spells)))
})
for (const viewport of ['desktop', 'mobile'] as const) {
  it(`shows all saved selections immediately and edits their own source on ${viewport}`, async () => {
    const user = setupUser()
    renderAt(viewport, <Panel />)
    await waitFor(() => expect(screen.getByRole('button', { name: 'Remove Shield' })).toBeEnabled())
    const selected = screen.getByRole('region', { name: 'Selected spells' })
    expect(within(selected).getAllByRole('heading').map((heading) => heading.textContent)).toEqual(['Level 1', 'Level 3'])
    expect(within(selected).getByRole('button', { name: 'Fireball' })).toBeInTheDocument()
    expect(within(selected).getByRole('button', { name: 'Shield' })).toBeInTheDocument()
    const filter = screen.getByRole('textbox', { name: 'Search spells' })
    const available = screen.getByRole('region', { name: 'Available spells' })
    expect(selected.compareDocumentPosition(filter) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(filter.compareDocumentPosition(available) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    await user.click(within(selected).getByRole('button', { name: 'Remove Shield' }))
    expect(answer).not.toHaveBeenCalled()
    expect(within(selected).queryByRole('button', { name: 'Shield' })).not.toBeInTheDocument()
    expect(within(selected).getByRole('button', { name: 'Fireball' })).toBeInTheDocument()
    expect(screen.queryByRole('combobox', { name: 'Spell selection' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled()
    await user.type(filter, 'Alarm')
    expect(within(selected).getByRole('button', { name: 'Fireball' })).toBeInTheDocument()
    const alarm = within(available).getByRole('article', { name: 'Alarm' })
    await waitFor(() => expect(within(alarm).getByRole('button', { name: 'Add Alarm' })).toBeEnabled())
    await user.click(within(alarm).getByRole('button', { name: 'Add Alarm' }))
    expect(within(selected).getByRole('article', { name: 'Alarm' })).toContainElement(within(selected).getByRole('button', { name: 'Remove Alarm' }))
    await user.click(screen.getByRole('button', { name: 'Next' }))
    expect(answer).toHaveBeenCalledWith([{ replaces: blocks[1]?.kind === 'settled' ? blocks[1].row : null, prompt: question('sorcerer'), picks: ['alarm'] }])
  })
}

it('unions all level allowances in one picker and keeps available spells ungrouped', async () => {
  const user = setupUser()
  const submissions = vi.fn()
  const first: Prompt = { ...question('warlock', 2), level: 1,
    choice: { ...question('warlock', 2).choice, prompt: 'warlock/spell/known/1', from: { kind: 'explicit', options: question('warlock').choice.from.options?.filter((option) => option.key !== 'fireball') ?? [] } } }
  const later: Prompt = { ...question('warlock'), level: 5 }
  const forget: Prompt = { ...question('warlock'), purpose: 'forget', optional: true }
  renderAt('desktop', <SpellStagePanel active blocks={[
    { kind: 'open', key: 'first', prompt: first }, { kind: 'open', key: 'later', prompt: later }, { kind: 'open', key: 'forget', prompt: forget },
  ]} names={new Map()} loadSavedPrompt={vi.fn()} onAnswers={submissions} pending={false} revision={1} />)
  await waitFor(() => expect(screen.getByRole('button', { name: 'Add Fireball' })).toBeEnabled())
  expect(screen.getByText('Selected: 0 / 3')).toBeInTheDocument()
  expect(screen.queryByRole('combobox', { name: 'Spell selection' })).not.toBeInTheDocument()
  const available = screen.getByRole('region', { name: 'Available spells' })
  expect(within(available).queryByRole('heading')).not.toBeInTheDocument()
  expect(within(available).getAllByRole('article').map((row) => row.getAttribute('aria-label'))).toEqual(['Alarm', 'Shield', 'Fireball'])
  await user.click(screen.getByRole('button', { name: 'Add Fireball' }))
  await user.click(screen.getByRole('button', { name: 'Add Shield' }))
  expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled()
  await user.click(screen.getByRole('button', { name: 'Add Alarm' }))
  expect(screen.getByText('Selected: 3 / 3')).toBeInTheDocument()
  const selected = screen.getByRole('region', { name: 'Selected spells' })
  expect(within(selected).getAllByRole('heading').map((heading) => heading.textContent)).toEqual(['Level 1', 'Level 3'])
  await user.click(screen.getByRole('button', { name: 'Next' }))
  expect(submissions).toHaveBeenCalledWith([
    { prompt: first, picks: ['shield', 'alarm'] }, { prompt: later, picks: ['fireball'] },
  ])
})

it('keeps saved cantrips and allows the one additional cantrip gained on level-up', async () => {
  const user = setupUser()
  const cantrips = ['chill-touch', 'eldritch-blast', 'mage-hand'].map((slug) => ({ slug, name: slug, level: 0 }))
  vi.stubGlobal('fetch', vi.fn(spellFetch(cantrips)))
  const old: Block = { key: 'old', kind: 'settled', changeable: true, row: { seq: 2, stage: 'cantrips', label: '', value: '',
    event: { type: 'level', choices: [{ prompt: 'warlock/spell/cantrip/1', picks: ['chill-touch', 'eldritch-blast'] }] } } }
  const extra: Prompt = { ...question('warlock'), purpose: 'cantrip', level: 4,
    held: ['chill-touch', 'eldritch-blast'], choice: { prompt: 'warlock/spell/cantrip/4', choose: 1, kind: 'spell',
      from: { kind: 'explicit', options: cantrips.map((spell) => ({ key: spell.slug, ref: `spell:${spell.slug}`, kind: 'ref' })) } } }
  const submit = vi.fn()
  renderAt('mobile', <SpellStagePanel active cantripsOnly blocks={[old, { key: 'new', kind: 'open', prompt: extra }]}
    names={new Map()} loadSavedPrompt={vi.fn()} onAnswers={submit} pending={false} revision={2} />)
  await waitFor(() => expect(screen.getByRole('button', { name: 'Add mage-hand' })).toBeEnabled())
  expect(screen.getByText('Selected: 2 / 3')).toBeInTheDocument()
  const selected = screen.getByRole('region', { name: 'Cantrips' })
  expect(within(selected).getByText('Cantrips')).toBeInTheDocument()
  expect(within(selected).queryByText('Selected spells')).not.toBeInTheDocument()
  expect(within(selected).queryByRole('heading', { name: 'Cantrip' })).not.toBeInTheDocument()
  const available = screen.getByRole('region', { name: 'Available spells' })
  expect(within(available).queryByRole('button', { name: 'chill-touch' })).not.toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: 'Add mage-hand' }))
  expect(screen.getByText('Selected: 3 / 3')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Remove chill-touch' })).toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: 'Next' }))
  expect(submit).toHaveBeenCalledWith([{ prompt: extra, picks: ['mage-hand'] }])
})

it('still allows preparing a spell already present in the spellbook', async () => {
  const user = setupUser()
  const book: Block = { key: 'book', kind: 'settled', changeable: true, row: { seq: 2, stage: 'spells', label: '', value: '',
    event: { type: 'level', purpose: 'spellbook', choices: [{ prompt: 'wizard/spell/spellbook/1', picks: ['shield'] }] } } }
  const prepare: Prompt = { ...question('wizard'), purpose: 'prepared', optional: true, upTo: true }
  const submit = vi.fn()
  renderAt('desktop', <SpellStagePanel active blocks={[book, { key: 'prepare', kind: 'open', prompt: prepare }]}
    names={new Map()} loadSavedPrompt={vi.fn()} onAnswers={submit} pending={false} revision={2} />)
  await waitFor(() => expect(screen.getByRole('button', { name: 'Add Shield' })).toBeEnabled())
  await user.click(screen.getByRole('button', { name: 'Add Shield' }))
  await user.click(screen.getByRole('button', { name: 'Next' }))
  expect(submit).toHaveBeenCalledWith([{ prompt: prepare, picks: ['shield'] }])
})

it('keeps every saved spell editable alongside new draft picks and submits all edits together', async () => {
  const user = setupUser()
  const next = vi.fn()
  const submit = vi.fn()
  const extra: Prompt = { ...question('bard'), choice: { ...question('bard').choice, prompt: 'bard/spell/known/6' } }
  renderAt('desktop', <SpellStagePanel active blocks={[...blocks, { kind: 'open', key: 'extra', prompt: extra }]}
    names={new Map()} loadSavedPrompt={async (row) => question(row.seq === 2 ? 'wizard' : 'sorcerer')}
    onAnswers={submit} onNext={next} pending={false} revision={3} />)
  await waitFor(() => expect(screen.getByRole('button', { name: 'Add Alarm' })).toBeEnabled())
  await user.click(screen.getByRole('button', { name: 'Add Alarm' }))
  expect(screen.getByRole('button', { name: 'Remove Fireball' })).toBeEnabled()
  expect(screen.getByRole('button', { name: 'Remove Shield' })).toBeEnabled()
  await user.click(screen.getByRole('button', { name: 'Remove Fireball' }))
  await user.click(screen.getByRole('button', { name: 'Remove Shield' }))
  expect(screen.getByRole('button', { name: 'Remove Alarm' })).toBeInTheDocument()
  await waitFor(() => expect(screen.getByRole('button', { name: 'Add Shield' })).toBeEnabled())
  await user.click(screen.getByRole('button', { name: 'Add Shield' }))
  await user.click(screen.getByRole('button', { name: 'Add Fireball' }))
  expect(screen.getAllByRole('button', { name: 'Next' })).toHaveLength(1)
  const selected = screen.getByRole('region', { name: 'Selected spells' })
  const button = screen.getByRole('button', { name: 'Next' })
  expect(selected.compareDocumentPosition(button) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  expect(button.compareDocumentPosition(screen.getByRole('textbox', { name: 'Search spells' })) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  await user.click(button)
  expect(submit).toHaveBeenCalledOnce()
  const changes = submit.mock.calls[0]![0] as { replaces?: { seq: number }; picks: string[] }[]
  expect(changes.filter((change) => change.replaces !== undefined).map((change) => change.replaces!.seq)).toEqual([2, 3])
  expect(changes.flatMap((change) => change.picks).sort()).toEqual(['alarm', 'fireball', 'shield'])
  expect(next).not.toHaveBeenCalled()
})

it('uses the same Next button to advance when there is no unsaved draft', async () => {
  const next = vi.fn()
  renderAt('desktop', <SpellStagePanel active blocks={blocks} names={new Map()} loadSavedPrompt={vi.fn()} onAnswers={vi.fn()} onNext={next} pending={false} revision={3} />)
  await waitFor(() => expect(screen.getByRole('button', { name: 'Next' })).toBeEnabled())
  await setupUser().click(screen.getByRole('button', { name: 'Next' }))
  expect(next).toHaveBeenCalledOnce()
})

for (const cantripsOnly of [false, true]) {
  it(`browses beyond eligibility without changing selections or allowing illegal additions (${cantripsOnly ? 'cantrips' : 'spells'})`, async () => {
    const user = setupUser()
    const offered: Spell[] = cantripsOnly
      ? [{ slug: 'light', name: 'Light', level: 0 }, { slug: 'mage-hand', name: 'Mage Hand', level: 0 }]
      : spells.filter((spell) => spell.level === 1)
    const other: Spell = { slug: 'unavailable', name: 'Unavailable', level: cantripsOnly ? 0 : 2, desc: ['Read the full description.'] }
    const wrongTab: Spell = { slug: 'other-tab', name: 'Other Tab', level: cantripsOnly ? 1 : 0 }
    vi.stubGlobal('fetch', vi.fn(spellFetch([...offered, other, wrongTab])))
    const prompt: Prompt = { ...question('wizard', 2), choice: { ...question('wizard', 2).choice,
      from: { kind: 'explicit', options: offered.map((spell) => ({ key: spell.slug, kind: 'ref', ref: `spell:${spell.slug}` })) } } }
    renderAt('desktop', <SpellStagePanel active cantripsOnly={cantripsOnly} blocks={[{ key: 'open', kind: 'open', prompt }]}
      names={new Map()} loadSavedPrompt={vi.fn()} onAnswers={vi.fn()} pending={false} revision={1} />)
    const first = offered[0]!
    await waitFor(() => expect(screen.getByRole('button', { name: `Add ${first.name}` })).toBeEnabled())
    const eligibility = screen.getByRole('checkbox', { name: 'Available for your character' })
    expect(eligibility).toBeChecked()
    expect(screen.queryByRole('button', { name: 'Unavailable' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: `Add ${first.name}` }))
    await user.click(eligibility)
    await waitFor(() => expect(screen.getByRole('button', { name: 'Unavailable' })).toBeInTheDocument())
    expect(screen.getByRole('button', { name: 'Add Unavailable' })).toBeDisabled()
    expect(screen.queryByRole('button', { name: 'Other Tab' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: `Remove ${first.name}` })).toBeEnabled()
    await user.click(screen.getByRole('button', { name: 'Unavailable' }))
    expect(screen.getByText('Read the full description.')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Reset filters' }))
    expect(eligibility).toBeChecked()
    expect(screen.queryByRole('button', { name: 'Unavailable' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: `Remove ${first.name}` })).toBeEnabled()
    expect(screen.getByText('Selected: 1 / 2')).toBeInTheDocument()
  })
}

it('adds, saves, and removes custom spells beyond normal level and count limits', async () => {
  const user = setupUser()
  const custom: Prompt = { ...question('custom'), purpose: 'custom', source: 'rule:custom-spells', optional: true, upTo: true,
    event: { type: 'change' }, choice: { ...question('custom').choice, prompt: 'custom/spell/known', choose: 319 } }
  const standard: Prompt = { ...question('wizard'), choice: { ...question('wizard').choice,
    from: { kind: 'explicit', options: [{ key: 'shield', kind: 'ref', ref: 'spell:shield' }] } } }
  const submit = vi.fn()
  renderAt('desktop', <SpellStagePanel active blocks={[{ key: 'normal', kind: 'open', prompt: standard }, { key: 'custom', kind: 'open', prompt: custom }]}
    names={new Map()} loadSavedPrompt={vi.fn()} onAnswers={submit} pending={false} revision={1} />)
  await waitFor(() => expect(screen.getByRole('button', { name: 'Add Shield' })).toBeEnabled())
  await user.click(screen.getByRole('button', { name: 'Add Shield' }))
  await user.click(screen.getByRole('checkbox', { name: 'Available for your character' }))
  await waitFor(() => expect(screen.getByRole('button', { name: 'Add Fireball' })).toBeEnabled())
  await user.click(screen.getByRole('button', { name: 'Add Fireball' }))
  await waitFor(() => expect(screen.getByRole('button', { name: 'Next' })).toBeEnabled())
  const selected = screen.getByRole('region', { name: 'Selected spells' })
  expect(within(selected).getByText('Custom choice')).toBeInTheDocument()
  expect(screen.getByText('Selected: 2 / 1')).toBeInTheDocument()
  await user.click(screen.getByRole('checkbox', { name: 'Available for your character' }))
  expect(within(selected).getByRole('button', { name: 'Remove Fireball' })).toBeEnabled()
  await user.click(screen.getByRole('button', { name: 'Next' }))
  expect(submit).toHaveBeenCalledWith([{ prompt: standard, picks: ['shield'] }, { prompt: custom, picks: ['fireball'] }])
  await user.click(within(selected).getByRole('button', { name: 'Remove Fireball' }))
  expect(within(selected).queryByRole('button', { name: 'Fireball' })).not.toBeInTheDocument()
})

it('saves the current-level total without a replacement step', async () => {
  const user = setupUser()
  const submit = vi.fn()
  const current = { ...question('sorcerer', 3), level: 5 }
  renderAt('desktop', <SpellStagePanel active blocks={[{ key: 'spells', kind: 'open', prompt: current }]}
    names={new Map()} loadSavedPrompt={vi.fn()} onAnswers={submit} pending={false} revision={1} />)
  await waitFor(() => expect(screen.getByRole('button', { name: 'Add Fireball' })).toBeEnabled())
  expect(screen.queryByRole('region', { name: 'Optional spell replacements' })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: 'Save and review replacements' })).not.toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: 'Add Fireball' }))
  await user.click(screen.getByRole('button', { name: 'Add Shield' }))
  await user.click(screen.getByRole('button', { name: 'Add Alarm' }))
  expect(screen.getByText('Selected: 3 / 3')).toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: 'Next' }))
  expect(submit).toHaveBeenCalledWith([{ prompt: current, picks: ['fireball', 'shield', 'alarm'] }])
})

it('caps highest-level picks across saved and new choices, with an explicit custom tag', async () => {
  const user = setupUser()
  const submit = vi.fn()
  const saved: Block = { key: 'first', kind: 'settled', changeable: true, row: { seq: 2, stage: 'spells', label: 'Sorcerer', value: 'Fireball',
    event: { type: 'level', choiceSource: 'class:sorcerer', purpose: 'known', choices: [{ prompt: 'sorcerer/spell/known/1', picks: ['fireball'] }] } } }
  const third: Spell = { slug: 'haste', name: 'Haste', level: 3 }
  const all = [...spells, third]
  vi.stubGlobal('fetch', vi.fn(spellFetch(all)))
  const normal = question('sorcerer', 2)
  normal.choice.from.options!.push({ key: 'haste', kind: 'ref', ref: 'spell:haste' })
  const custom = { ...question('custom'), purpose: 'custom', source: 'rule:custom-spells', optional: true, upTo: true, event: { type: 'change' }, choice: { ...question('custom').choice, prompt: 'custom/spell/known', choose: 319 } }
  renderAt('desktop', <SpellStagePanel active blocks={[saved, { kind: 'open', key: 'normal', prompt: normal }, { kind: 'open', key: 'custom', prompt: custom }]}
    names={new Map()} loadSavedPrompt={vi.fn()} onAnswers={submit} pending={false} revision={1}
    rules={[{ id: 'known', source: 'class:sorcerer', class: 'sorcerer', classLevel: 5, count: 3, minLevel: 1, maxLevel: 3, maxLevelCount: 1, purpose: 'known', optional: false }]} />)
  await waitFor(() => expect(screen.getByRole('button', { name: 'Add Haste' })).toBeDisabled())
  const summary = screen.getByRole('region', { name: 'Your character’s spell allowances' })
  const selected = screen.getByRole('region', { name: 'Selected spells' })
  expect(summary.compareDocumentPosition(selected) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  expect(within(summary).getByText('Selected: 1 / 3')).toBeInTheDocument()
  expect(summary).toHaveTextContent('Level 3: at most 1')
  await user.click(screen.getByRole('checkbox', { name: 'Available for your character' }))
  await user.click(screen.getByRole('button', { name: 'Add Haste' }))
  expect(within(selected).getByText('Custom choice')).toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: 'Add Shield' }))
  await user.click(screen.getByRole('button', { name: 'Add Alarm' }))
  await user.click(screen.getByRole('button', { name: 'Next' }))
  expect(submit).toHaveBeenCalledWith([{ prompt: normal, picks: ['shield', 'alarm'] }, { prompt: custom, picks: ['haste'] }])
})

it('keeps an increased limit in the save batch and appends available spells', async () => {
  const user = setupUser()
  const all = Array.from({ length: 45 }, (_, i) => ({ slug: `spell-${i}`, name: `Spell ${String(i).padStart(2, '0')}`, level: 1, classes: ['sorcerer'] }))
  vi.stubGlobal('fetch', vi.fn(spellFetch(all)))
  const custom = { ...question('custom'), purpose: 'custom', source: 'rule:custom-spells', optional: true, upTo: true, event: { type: 'change' }, choice: { ...question('custom').choice, prompt: 'custom/spell/known', choose: 319 } }
  const submit = vi.fn()
  renderAt('mobile', <SpellStagePanel active blocks={[{ key: 'custom', kind: 'open', prompt: custom }]} names={new Map()} loadSavedPrompt={vi.fn()} onAnswers={submit} pending={false} revision={1} />)
  await user.click(screen.getByRole('button', { name: 'Increase spell limit' }))
  await user.click(screen.getByRole('checkbox', { name: 'Available for your character' }))
  const available = screen.getByRole('region', { name: 'Available spells' })
  await waitFor(() => expect(within(available).getAllByRole('article')).toHaveLength(20))
  await user.click(screen.getByRole('button', { name: 'Load more' }))
  await waitFor(() => expect(within(available).getAllByRole('article')).toHaveLength(40))
  expect(screen.getByRole('button', { name: 'Add Spell 00' })).toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: 'Add Spell 20' }))
  expect(screen.getByText('Selected: 1 / 1')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Add Spell 39' })).toBeInTheDocument()
  await user.type(screen.getByRole('textbox', { name: 'Search spells' }), 'Spell 44')
  expect(within(available).getAllByRole('article')).toHaveLength(1)
  expect(screen.queryByRole('button', { name: 'Load more' })).not.toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: 'Next' }))
  expect(submit).toHaveBeenCalledWith([
    { prompt: custom, picks: ['spell-20'] },
    expect.objectContaining({ changes: [{ path: 'spellLimits.known', op: 'increment', value: { kind: 'int', int: 1 } }] }),
  ])
})

for (const cantripsOnly of [false, true]) {
  it(`decreases the saved ${cantripsOnly ? 'cantrip' : 'spell'} allowance to zero without going negative`, async () => {
    const user = setupUser()
    const submit = vi.fn()
    renderAt('desktop', <SpellStagePanel active cantripsOnly={cantripsOnly} blocks={[]} names={new Map()} loadSavedPrompt={vi.fn()}
      onAnswers={submit} pending={false} revision={1} rules={[
        { id: 'custom/limit', source: 'rule:custom-spells', purpose: 'custom-limit', count: 2, minLevel: cantripsOnly ? 0 : 9, maxLevel: cantripsOnly ? 0 : 9, optional: true },
      ]} />)
    const minus = screen.getByRole('button', { name: 'Decrease spell limit' })
    expect(screen.getByText('Additional allowance')).toBeInTheDocument()
    expect(screen.getByText('Additional choices: 2')).toBeInTheDocument()
    await user.click(minus)
    expect(screen.getByText('Selected: 0 / 1')).toBeInTheDocument()
    await user.click(minus)
    expect(screen.getByText('Selected: 0 / 0')).toBeInTheDocument()
    expect(minus).toBeDisabled()
    await user.click(minus)
    await waitFor(() => expect(screen.getByRole('button', { name: 'Next' })).toBeEnabled())
    await user.click(screen.getByRole('button', { name: 'Next' }))
    expect(submit).toHaveBeenCalledWith([expect.objectContaining({ changes: [
      { path: `spellLimits.${cantripsOnly ? 'cantrip' : 'known'}`, op: 'increment', value: { kind: 'int', int: -2 } },
    ] })])
  })
}
