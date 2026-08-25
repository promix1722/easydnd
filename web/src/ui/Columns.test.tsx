import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'

import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'

import { Columns, type ColumnsSection } from './Columns'

const sections: ColumnsSection[] = [
  { key: 'abilities', title: 'Abilities', content: <p>STR 16</p> },
  { key: 'skills', title: 'Skills', content: <p>Stealth +7</p> },
]

describe('Columns', () => {
  it('shows every section at once on desktop', () => {
    renderAt('desktop', <Columns sections={sections} />)

    expect(screen.getByText('STR 16')).toBeInTheDocument()
    expect(screen.getByText('Stealth +7')).toBeInTheDocument()
    expect(screen.queryAllByRole('button')).toHaveLength(0)
  })

  it('collapses sections behind accordion controls on mobile', () => {
    renderAt('mobile', <Columns sections={sections} />)

    // One control per section, and only the first open by default. Mantine
    // keeps collapsed panels mounted, so the assertion has to be about the
    // accessibility tree -- which is also what actually decides whether a
    // screen reader announces the content -- rather than about presence.
    const controls = screen.getAllByRole('button')
    expect(controls).toHaveLength(sections.length)
    expect(controls[0]).toHaveAttribute('aria-expanded', 'true')
    expect(controls[1]).toHaveAttribute('aria-expanded', 'false')
  })

  // The aside is drawn beside the title, and on mobile the title *is* a
  // button. Nesting one button in another is invalid markup and the outer
  // control swallows the press, so what matters is that it stays clickable.
  describe('a section aside', () => {
    const withAside: ColumnsSection[] = [
      { ...sections[0]!, aside: <button type="button">Filter</button> },
      sections[1]!,
    ]

    it.each(['desktop', 'mobile'] as const)('is reachable at %s', async (viewport) => {
      const clicked: string[] = []
      renderAt(
        viewport,
        <Columns
          sections={[
            { ...sections[0]!, aside: <button type="button" onClick={() => clicked.push('aside')}>Filter</button> },
            sections[1]!,
          ]}
        />,
      )

      await setupUser().click(screen.getByRole('button', { name: 'Filter' }))
      expect(clicked).toEqual(['aside'])
    })

    it('does not swallow the accordion control it sits beside', async () => {
      renderAt('mobile', <Columns sections={withAside} />)

      // The section starts open; pressing its control closes it. If the aside
      // had been rendered inside the control, this would be a nested button.
      const control = screen.getByRole('button', { name: 'Abilities' })
      expect(control).toHaveAttribute('aria-expanded', 'true')
      await setupUser().click(control)
      expect(control).toHaveAttribute('aria-expanded', 'false')
    })

    it('is absent from a section that did not ask for one', () => {
      renderAt('desktop', <Columns sections={withAside} />)

      expect(screen.getAllByRole('button', { name: 'Filter' })).toHaveLength(1)
    })
  })

  it('opens the sections named in defaultOpen', () => {
    renderAt('mobile', <Columns sections={sections} defaultOpen={['skills']} />)

    const controls = screen.getAllByRole('button')
    expect(controls[0]).toHaveAttribute('aria-expanded', 'false')
    expect(controls[1]).toHaveAttribute('aria-expanded', 'true')
  })
})
