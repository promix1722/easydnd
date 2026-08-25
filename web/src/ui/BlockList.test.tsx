import { screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { renderAt } from '@/test/render'
import type { Viewport } from '@/test/viewport'
import { setupUser } from '@/test/user'

import { BlockList } from './BlockList'

const ITEMS = [
  { key: 'one', header: 'The first thing', body: <p>what the first thing says</p> },
  { key: 'two', header: 'The second thing', body: <p>what the second thing says</p> },
  { key: 'three', header: 'A settled fact' },
]

const viewports: Viewport[] = ['mobile', 'desktop']

describe.each(viewports)('BlockList at %s', (viewport) => {
  it('shows every header and no body until one is opened', () => {
    renderAt(viewport, <BlockList items={ITEMS} open={null} onOpen={vi.fn()} />)

    expect(screen.getByText('The first thing')).toBeInTheDocument()
    expect(screen.getByText('The second thing')).toBeInTheDocument()
    expect(screen.queryByText('what the first thing says')).not.toBeInTheDocument()
  })

  it('draws only the open block body', () => {
    renderAt(viewport, <BlockList items={ITEMS} open="two" onOpen={vi.fn()} />)

    // Bodies are mounted, not merely hidden, one at a time: a body may fetch
    // what it needs on mount, and nine collapsed blocks should not pay for it.
    expect(screen.getByText('what the second thing says')).toBeInTheDocument()
    expect(screen.queryByText('what the first thing says')).not.toBeInTheDocument()
  })

  it('opens the block that was pressed, and closes the one that was open', async () => {
    const user = setupUser()
    const onOpen = vi.fn()
    const { rerender } = renderAt(viewport, <BlockList items={ITEMS} open={null} onOpen={onOpen} />)

    await user.click(screen.getByRole('button', { name: 'The first thing' }))
    expect(onOpen).toHaveBeenCalledWith('one')

    rerender(<BlockList items={ITEMS} open="one" onOpen={onOpen} />)
    await user.click(screen.getByRole('button', { name: 'The first thing' }))
    // Pressing the open block shuts it, which arrives as nothing being open.
    expect(onOpen).toHaveBeenLastCalledWith(null)
  })

  it('says which block is open, and which region belongs to it', async () => {
    const user = setupUser()
    renderAt(viewport, <BlockList items={ITEMS} open="one" onOpen={vi.fn()} />)

    const control = screen.getByRole('button', { name: 'The first thing' })
    expect(control).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByRole('button', { name: 'The second thing' })).toHaveAttribute(
      'aria-expanded',
      'false',
    )

    // Reachable and operable from the keyboard, because it is a real button.
    await user.tab()
    expect(control).toHaveFocus()
  })

  it('leaves a block with no body as a statement rather than a dead control', () => {
    renderAt(viewport, <BlockList items={ITEMS} open={null} onOpen={vi.fn()} />)

    expect(screen.getByText('A settled fact')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'A settled fact' })).not.toBeInTheDocument()
  })

  it('marks the blocks that want something', () => {
    renderAt(
      viewport,
      <BlockList
        items={[{ key: 'one', header: 'Still wanted', body: <p>pick one</p>, highlighted: true }]}
        open={null}
        onOpen={vi.fn()}
      />,
    )

    const control = screen.getByRole('button', { name: 'Still wanted' })
    expect(control.closest('[data-highlighted="true"]')).not.toBeNull()
  })
})
