import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'

import { renderAt } from '@/test/render'
import { DESKTOP_BREAKPOINT_PX, VIEWPORT_WIDTHS } from '@/test/viewport'

import { DataList, type DataListColumn } from './DataList'

interface Row {
  id: string
  name: string
  level: number
}

const rows: Row[] = [
  { id: 'a', name: 'Vex', level: 3 },
  { id: 'b', name: 'Grog', level: 5 },
]

const columns: DataListColumn<Row>[] = [
  { key: 'name', header: 'Name', render: (row) => row.name, primary: true },
  { key: 'level', header: 'Level', render: (row) => row.level },
]

describe('DataList', () => {
  it('places both test viewports on the intended side of the breakpoint', () => {
    expect(VIEWPORT_WIDTHS.mobile).toBeLessThan(DESKTOP_BREAKPOINT_PX)
    expect(VIEWPORT_WIDTHS.desktop).toBeGreaterThanOrEqual(DESKTOP_BREAKPOINT_PX)
  })

  it('renders a table on desktop', () => {
    renderAt('desktop', <DataList items={rows} columns={columns} getKey={(row) => row.id} />)

    expect(screen.getByRole('table')).toBeInTheDocument()
    expect(screen.getByRole('columnheader', { name: 'Name' })).toBeInTheDocument()
    expect(screen.getByText('Vex')).toBeInTheDocument()
  })

  it('renders labelled cards instead of a table on mobile', () => {
    renderAt('mobile', <DataList items={rows} columns={columns} getKey={(row) => row.id} />)

    expect(screen.queryByRole('table')).not.toBeInTheDocument()
    expect(screen.getByText('Vex')).toBeInTheDocument()
    // The header becomes an inline label per card, so a phone reader still
    // knows what "5" means.
    expect(screen.getAllByText(/Level:/)).toHaveLength(rows.length)
  })

  it('shows the empty message at both viewports', () => {
    renderAt('desktop', <DataList items={[]} columns={columns} getKey={(row) => row.id} empty="None yet." />)
    expect(screen.getByText('None yet.')).toBeInTheDocument()
  })
})
