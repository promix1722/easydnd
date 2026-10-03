import { fireEvent, screen } from '@testing-library/react'
import { useState } from 'react'
import { CharacterPolicy } from '@/lib/api/catalogScope'
import { DEFAULT_BUILD_POLICY } from '@/lib/api/packPolicy'
import { renderAt } from '@/test/render'
import { PointBuy } from './PointBuy'

it('uses a selected core pack’s costs and budget', () => {
  function Form() {
    const [scores, setScores] = useState({ str: 10, dex: 10, con: 10, int: 10, wis: 10, cha: 10 })
    return <CharacterPolicy.Provider value={{ ...DEFAULT_BUILD_POLICY, pointBuyBudget: 2, pointCosts: { 10: 0, 11: 2, 12: 4 } }}><PointBuy scores={scores} onChange={(next) => setScores(next as typeof scores)} /></CharacterPolicy.Provider>
  }
  renderAt('desktop', <Form />)
  fireEvent.click(screen.getByRole('button', { name: 'Raise Strength' }))
  expect(screen.getByRole('button', { name: 'Raise Dexterity' })).toBeDisabled()
  expect(screen.getByRole('button', { name: 'Raise Strength' })).toBeDisabled()
  expect(screen.getByRole('button', { name: 'Lower Strength' })).toBeEnabled()
})
