import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'

import { renderAt } from '@/test/render'

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

  it('opens the sections named in defaultOpen', () => {
    renderAt('mobile', <Columns sections={sections} defaultOpen={['skills']} />)

    const controls = screen.getAllByRole('button')
    expect(controls[0]).toHaveAttribute('aria-expanded', 'false')
    expect(controls[1]).toHaveAttribute('aria-expanded', 'true')
  })
})
