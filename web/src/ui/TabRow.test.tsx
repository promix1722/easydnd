import { screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { renderAt } from '@/test/render'
import type { Viewport } from '@/test/viewport'
import { setupUser } from '@/test/user'

import { TabRow } from './TabRow'

const TABS = [
  { value: 'identity', label: 'identity' },
  { value: 'class', label: 'class' },
  { value: 'race', label: 'race' },
]

function renderRow(viewport: Viewport, value = 'class', onChange = vi.fn()) {
  const result = renderAt(
    viewport,
    <TabRow tabs={TABS} value={value} onChange={onChange} actions={<button>Next</button>}>
      <p>panel</p>
    </TabRow>,
  )
  return { ...result, onChange }
}

/**
 * The ids Mantine mints differ between two renders of the same tree, so they
 * are stripped before the two viewports' markup is compared. Everything else
 * -- elements, classes, data attributes -- has to match exactly.
 */
function structure(html: string): string {
  return html
    .replace(/(id|aria-controls|aria-labelledby)="[^"]*"/g, '$1=""')
    .replace(/mantine-[a-z0-9]+-/gi, 'mantine-')
}

/**
 * One viewport, because this file proves it is enough: the last test in it
 * compares the two renderings byte for byte. `ModalSheet`, `Columns` and
 * `SectionDeck` swap components at the breakpoint and so need testing twice
 * over; this is one
 * rendering with a ScrollArea that is inert at a width the tabs fit in.
 */
describe('TabRow', () => {
  const viewport = 'desktop'

  it('draws every tab in the order given, and marks the active one', () => {
    renderRow(viewport)

    expect(screen.getAllByRole('tab').map((tab) => tab.textContent)).toEqual([
      'identity',
      'class',
      'race',
    ])
    expect(screen.getByRole('tab', { name: 'class' })).toHaveAttribute('aria-selected', 'true')
  })

  it('reports the tab that was pressed', async () => {
    const user = setupUser()
    const { onChange } = renderRow(viewport)

    await user.click(screen.getByRole('tab', { name: 'race' }))

    expect(onChange).toHaveBeenCalledWith('race')
  })

  it('keeps the actions out of the strip', () => {
    renderRow(viewport)

    // Next belongs to the row rather than to a tab, so it is a button beside
    // the strip and never one of the things being scrolled past.
    expect(screen.getByRole('button', { name: 'Next' })).toBeInTheDocument()
    expect(screen.queryByRole('tab', { name: 'Next' })).not.toBeInTheDocument()
  })

  it('draws the panel of whichever tab is active', () => {
    renderRow(viewport)

    expect(screen.getByText('panel')).toBeInTheDocument()
  })
})

describe('TabRow', () => {
  /**
   * The claim the primitive is built on. `ModalSheet`, `Columns` and
   * `SectionDeck` swap components at the breakpoint and so need testing twice
   * over; this one is one rendering with a ScrollArea that is inert at a width the tabs fit in,
   * which is what makes a test at either width a test of both.
   */
  it('renders the same markup at both viewports', () => {
    const mobile = renderRow('mobile')
    const mobileHtml = structure(mobile.container.innerHTML)
    mobile.unmount()

    const desktop = renderRow('desktop')

    expect(structure(desktop.container.innerHTML)).toBe(mobileHtml)
  })
})
