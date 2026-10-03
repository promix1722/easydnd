import { screen, within } from '@testing-library/react'
import { expect, it } from 'vitest'
import { renderAt } from '@/test/render'
import { SpellRulesSummary } from './SpellRulesSummary'

it('compacts historical allowances into one current-level message per source', () => {
  renderAt('mobile', <SpellRulesSummary cantripsOnly={false} count={7} total={6} names={new Map([['class:sorcerer', 'Sorcerer'], ['trait:gift', 'Racial Gift']])}
    rules={[
      { id: 'one', source: 'class:sorcerer', class: 'sorcerer', classLevel: 2, count: 3, minLevel: 1, maxLevel: 1, purpose: 'known', optional: false, listClasses: ['sorcerer'] },
      { id: 'two', source: 'class:sorcerer', class: 'sorcerer', classLevel: 4, count: 2, minLevel: 1, maxLevel: 2, purpose: 'known', optional: false, listClasses: ['sorcerer'] },
      { id: 'three', source: 'class:sorcerer', class: 'sorcerer', classLevel: 5, count: 1, minLevel: 1, maxLevel: 3, maxLevelCount: 2, purpose: 'known', optional: false, listClasses: ['sorcerer'] },
      { id: 'replace', source: 'class:sorcerer', class: 'sorcerer', classLevel: 5, count: 1, minLevel: 1, maxLevel: 3, purpose: 'replace', optional: true },
      { id: 'race', source: 'trait:gift', count: 1, minLevel: 1, maxLevel: 1, purpose: 'known', optional: false },
    ]} />)
  const summary = within(screen.getByRole('region', { name: 'Your character’s spell allowances' }))
  const sorcerer = summary.getByText('Sorcerer · Level 5').parentElement!
  expect(sorcerer).toHaveTextContent('Sorcerer · Level 5Spells known: 6 · Spell levels 1–3')
  expect(summary.queryByText(/replace one known spell/)).not.toBeInTheDocument()
  expect(summary.getByText('Racial Gift').parentElement).toHaveTextContent('One-time racial or feature grant')
  expect(summary.getByText('Selected: 7 / 6')).toBeInTheDocument()
  expect(summary.queryByText(/Casting slots are separate/)).not.toBeInTheDocument()
})
