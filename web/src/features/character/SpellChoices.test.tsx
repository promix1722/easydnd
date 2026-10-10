import { screen, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { Entry, Prompt, Spell } from '@/lib/api'
import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'

import { PromptCard } from './PromptCard'

const spells: Spell[] = [
  { slug: 'light', icon: 'data:image/webp;base64,bGlnaHQ=', name: 'Light', level: 0, school: 'evocation', classes: ['wizard'], castingTime: { kind: 'action' }, components: { verbal: true, material: true } },
  { slug: 'detect-magic', icon: 'data:image/webp;base64,ZGV0ZWN0', name: 'Detect Magic', level: 1, school: 'divination', classes: ['wizard', 'cleric'], castingTime: { kind: 'action' }, components: { verbal: true, somatic: true }, concentration: true, ritual: true, desc: ['***Sense magic.*** Sense nearby magic.\n\n- A visible aura\n- A magical school'] },
  { slug: 'shield', icon: 'data:image/webp;base64,c2hpZWxk', name: 'Shield', level: 1, school: 'abjuration', classes: ['wizard'], castingTime: { kind: 'reaction' } },
]
const entries = new Map<string, Entry>([
  ...spells.map((spell): [string, Entry] => [spell.slug, spell]),
  ['wizard', { slug: 'wizard', name: 'Wizard' }],
  ['cleric', { slug: 'cleric', name: 'Cleric' }],
  ['divination', { slug: 'divination', name: 'Divination' }],
  ['evocation', { slug: 'evocation', name: 'Evocation' }],
  ['abjuration', { slug: 'abjuration', name: 'Abjuration' }],
  ['fireball', { slug: 'fireball', name: 'Fireball', level: 3 } as Spell],
])
const prompt: Prompt = {
  source: 'class:bard', group: 'class', event: { type: 'level' }, optional: false, heldOnly: false,
  choice: { prompt: 'magical-secrets/spell/known', kind: 'spell', choose: 2,
    from: { kind: 'explicit', options: spells.map((spell) => ({ kind: 'ref', ref: `spell:${spell.slug}`, key: spell.slug })) } },
}

for (const viewport of ['desktop', 'mobile'] as const) {
  describe(`spell choices on ${viewport}`, () => {
    it('previews before adding, keeps selected spells above filters, and enforces the limit', async () => {
      const user = setupUser()
      const onAnswer = vi.fn()
      const { container } = renderAt(viewport, <PromptCard prompt={prompt} entries={entries} pending={false} onAnswer={onAnswer} />)
      const selected = screen.getByRole('region', { name: 'Selected spells' })
      const available = screen.getByRole('region', { name: 'Available spells' })
      const detect = within(available).getByRole('button', { name: 'Detect Magic' })
      expect(within(detect).getByText('Level 1')).toBeInTheDocument()
      expect(within(detect).getByLabelText('Concentration')).toBeInTheDocument()
      expect(within(detect).getByLabelText('Ritual')).toBeInTheDocument()
      expect(within(detect).getByText('Divination · 1 action · V, S')).toBeInTheDocument()
      expect(container.querySelector('img[src="data:image/webp;base64,ZGV0ZWN0"]')).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'Fireball' })).not.toBeInTheDocument()
      expect(within(available).queryByRole('heading')).not.toBeInTheDocument()
      expect(screen.getByRole('textbox', { name: 'Search spells' }).compareDocumentPosition(available) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
      expect(within(available).getByRole('article', { name: 'Light' })).toContainElement(within(available).getByRole('button', { name: 'Add Light' }))
      const finish = screen.getByRole('button', { name: 'Finish selection' })
      expect(finish).toBeDisabled()
      expect(selected.compareDocumentPosition(finish) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
      expect(finish.compareDocumentPosition(screen.getByRole('textbox', { name: 'Search spells' })) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()

      await user.click(within(available).getByRole('button', { name: 'Light' }))
      expect(onAnswer).not.toHaveBeenCalled()
      expect(within(selected).queryByRole('button', { name: 'Light' })).not.toBeInTheDocument()
      if (viewport === 'mobile') {
        expect(screen.getByRole('dialog', { name: 'Light' })).toBeInTheDocument()
        const add = within(available).getByRole('button', { name: 'Add Light', hidden: true })
        expect(add.textContent).toBe('')
        expect(add.querySelector('svg')).not.toBeNull()
        await user.click(screen.getByRole('button', { name: 'Back' }))
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
        expect(within(selected).queryByRole('button', { name: 'Light' })).not.toBeInTheDocument()
        await user.click(within(available).getByRole('button', { name: 'Light' }))
      } else {
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
        expect(within(available).getByRole('article', { name: 'Light' })).toContainElement(screen.getByRole('region', { name: 'Light' }))
        expect(within(screen.getByRole('region', { name: 'Light' })).getByRole('separator')).toBeInTheDocument()
        const article = within(available).getByRole('article', { name: 'Light' })
        expect(article.querySelectorAll('img')).toHaveLength(1)
        expect(within(article).getAllByText('Cantrip')).toHaveLength(1)
        expect(within(article).getAllByRole('button', { name: /Add/ })).toHaveLength(1)
      }
      await user.click(screen.getByRole('button', { name: viewport === 'mobile' ? 'Add' : 'Add Light' }))
      expect(within(selected).getByRole('button', { name: 'Light' })).toBeInTheDocument()
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
      await user.click(screen.getByRole('checkbox', { name: 'Ritual' }))
      expect(within(selected).getByRole('button', { name: 'Light' })).toBeInTheDocument()
      expect(within(available).queryByRole('button', { name: 'Light' })).not.toBeInTheDocument()
      expect(within(available).queryByRole('button', { name: 'Shield' })).not.toBeInTheDocument()
      await user.click(within(available).getByRole('button', { name: 'Detect Magic' }))
      expect(screen.getByText('Sense magic.').closest('strong')).not.toBeNull()
      expect(screen.getByText('A visible aura').closest('li')).not.toBeNull()
      await user.click(screen.getByRole('button', { name: viewport === 'mobile' ? 'Add' : 'Add Detect Magic' }))
      expect(screen.getByRole('checkbox', { name: 'Ritual' })).toBeChecked()
      expect(finish).toBeEnabled()
      await user.click(screen.getByRole('button', { name: 'Reset filters' }))
      expect(within(selected).getAllByRole('button').filter((button) => button.hasAttribute('aria-expanded')).map((button) => button.getAttribute('aria-label'))).toEqual(['Light', 'Detect Magic'])
      await user.click(within(available).getByRole('button', { name: 'Shield' }))
      expect(screen.getByRole('button', { name: viewport === 'mobile' ? 'Add' : 'Add Shield' })).toBeDisabled()
      if (viewport === 'mobile') await user.click(screen.getByRole('button', { name: 'Back' }))
      else await user.click(within(available).getByRole('button', { name: 'Shield' }))
      await user.click(within(selected).getByRole('button', { name: 'Remove Light' }))
      expect(finish).toBeDisabled()
      expect(within(available).getAllByRole('button').filter((button) => button.hasAttribute('aria-expanded')).map((button) => button.getAttribute('aria-label'))).toEqual(['Light', 'Shield'])
      await user.click(within(available).getByRole('button', { name: 'Add Light' }))
      await user.click(finish)
      expect(onAnswer).toHaveBeenCalledWith([{ prompt: prompt.choice.prompt, picks: ['detect-magic', 'light'] }])
    })

    it('combines dropdown filters, reports empty results and resets without widening eligibility', async () => {
      const user = setupUser()
      renderAt(viewport, <PromptCard prompt={prompt} entries={entries} pending={false} onAnswer={vi.fn()} />)
      await user.click(screen.getByRole('combobox', { name: 'Level' }))
      await user.click(screen.getByRole('option', { name: 'Cantrip' }))
      expect(screen.getByRole('button', { name: 'Light' })).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'Detect Magic' })).not.toBeInTheDocument()
      await user.click(screen.getByRole('checkbox', { name: 'No material component' }))
      expect(screen.getByText('No spell matches those filters.')).toBeInTheDocument()
      await user.click(screen.getByRole('button', { name: 'Reset filters' }))
      await user.click(screen.getByRole('combobox', { name: 'School' }))
      await user.click(screen.getByRole('option', { name: 'Divination' }))
      await user.click(screen.getByRole('combobox', { name: 'Class' }))
      await user.click(screen.getByRole('option', { name: 'Cleric' }))
      await user.click(screen.getByRole('combobox', { name: 'Casting time' }))
      await user.click(screen.getByRole('option', { name: '1 action' }))
      expect(screen.getByRole('button', { name: 'Detect Magic' })).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'Fireball' })).not.toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'Light' })).not.toBeInTheDocument()
    })
  })
}
