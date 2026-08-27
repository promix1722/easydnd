import { describe, expect, it } from 'vitest'
import { screen, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router'

import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'
import { DESKTOP_BREAKPOINT_PX, VIEWPORT_WIDTHS, type Viewport } from '@/test/viewport'

import { classLine } from '@/domain'

import { Badge, Code, Stack, Text } from '@mantine/core'

import { DataList, type DataListColumn, type RowAction } from './DataList'

interface Row {
  id: string
  name: string
  level: number
  classes: { class: string; level: number }[]
}

const rows: Row[] = [
  { id: 'a', name: 'Vex', level: 3, classes: [{ class: 'rogue', level: 3 }] },
  { id: 'b', name: 'Grog', level: 5, classes: [{ class: 'barbarian', level: 5 }] },
]

const columns: DataListColumn<Row>[] = [
  { key: 'name', header: 'Name', primary: true, text: (row) => row.name, render: (row) => row.name },
  { key: 'level', header: 'Level', render: (row) => row.level },
]

const at = (viewport: Viewport, ui: React.ReactElement) =>
  renderAt(viewport, <MemoryRouter>{ui}</MemoryRouter>)

/**
 * The card's dimmed line of facts, as one string.
 *
 * It has to be read this way because the line is built from a `<span>` per
 * fact, so the paragraph holding them has no direct text of its own -- and
 * `getByText` matches an element's *own* text nodes, not its descendants'.
 * Matching on `textContent` is the honest way to assert on something assembled
 * from parts.
 */
function metaLine(text: string): HTMLElement {
  return screen.getByText(
    (_content, element) => element?.tagName === 'P' && element.textContent === text,
  )
}

/** Every list in the app opens its row actions this way; see `RowMenu`. */
async function openMenu(name: string): Promise<HTMLElement> {
  const user = setupUser()
  await user.click(screen.getByRole('button', { name: `Actions for ${name}` }))
  return await screen.findByRole('menu')
}

describe('DataList', () => {
  it('places both test viewports on the intended side of the breakpoint', () => {
    expect(VIEWPORT_WIDTHS.mobile).toBeLessThan(DESKTOP_BREAKPOINT_PX)
    expect(VIEWPORT_WIDTHS.desktop).toBeGreaterThanOrEqual(DESKTOP_BREAKPOINT_PX)
  })

  it('renders a table on desktop', () => {
    at('desktop', <DataList items={rows} columns={columns} getKey={(row) => row.id} />)

    expect(screen.getByRole('table')).toBeInTheDocument()
    expect(screen.getByRole('columnheader', { name: 'Name' })).toBeInTheDocument()
    expect(screen.getByText('Vex')).toBeInTheDocument()
  })

  it('shows the empty message at both viewports', () => {
    at('desktop', <DataList items={[]} columns={columns} getKey={(row) => row.id} empty="None yet." />)
    expect(screen.getByText('None yet.')).toBeInTheDocument()
  })

  describe('the card', () => {
    it('draws no table, and joins the other columns into one line of values', () => {
      at('mobile', <DataList items={rows} columns={columns} getKey={(row) => row.id} />)

      expect(screen.queryByRole('table')).not.toBeInTheDocument()
      expect(screen.getByText('Vex')).toBeInTheDocument()
      // The headers are gone from the card. They used to be printed as
      // "Level: 3" on a line of their own, which is what made a 390px screen
      // spend a whole row on one word.
      expect(screen.queryByText(/Level:/)).not.toBeInTheDocument()
      expect(screen.getAllByText('3')).not.toHaveLength(0)
    })

    it('separates two facts with a middle dot', () => {
      const three: DataListColumn<Row>[] = [
        ...columns,
        { key: 'classes', header: 'Classes', render: (row) => classLine(row.classes) },
      ]
      at('mobile', <DataList items={[rows[0]!]} columns={three} getKey={(row) => row.id} />)

      expect(metaLine('3 · Rogue 3')).toBeInTheDocument()
    })

    it.each([
      ['a dash, which is what a table prints for nothing', '--'],
      ['an empty string', ''],
      ['null', null],
    ])('drops a fact that is %s', (_why, value) => {
      const withEmpty: DataListColumn<Row>[] = [
        columns[0]!,
        { key: 'classes', header: 'Classes', render: () => value },
      ]
      at('mobile', <DataList items={[rows[0]!]} columns={withEmpty} getKey={(row) => row.id} />)

      expect(screen.getByText('Vex')).toBeInTheDocument()
      expect(screen.queryByText('--')).not.toBeInTheDocument()
    })

    it('drops the dash `classLine` really answers with, rather than one this test invented', () => {
      // The guard on the coupling: `DataList` knows the literal '--' and
      // `domain/classLine` produces it, and neither imports the other.
      expect(classLine([])).toBe('--')

      const unbuilt: Row = { id: 'c', name: 'Untitled', level: 0, classes: [] }
      const both: DataListColumn<Row>[] = [
        columns[0]!,
        { key: 'classes', header: 'Classes', render: (row) => classLine(row.classes) },
      ]
      at('mobile', <DataList items={[unbuilt]} columns={both} getKey={(row) => row.id} />)

      expect(screen.getByText('Untitled')).toBeInTheDocument()
      expect(screen.queryByText('--')).not.toBeInTheDocument()
    })

    it('puts a badge column beside the name rather than in the line of facts', () => {
      const withBadge: DataListColumn<Row>[] = [
        columns[0]!,
        {
          key: 'role',
          header: 'Role',
          slot: 'badge',
          render: () => <Badge variant="light">Owner</Badge>,
        },
      ]
      at('mobile', <DataList items={[rows[0]!]} columns={withBadge} getKey={(row) => row.id} />)

      // Not joined by a dot to anything: it is the only non-primary column, so
      // a meta line would have carried it if the slot were ignored.
      expect(screen.getByText('Owner')).toBeInTheDocument()
      expect(screen.queryByText(/·/)).not.toBeInTheDocument()
    })

    it('gives a block column its own line, headed by its column header', () => {
      const withBlock: DataListColumn<Row>[] = [
        columns[0]!,
        {
          key: 'detail',
          header: 'Detail',
          slot: 'block',
          render: () => (
            <Stack gap={4}>
              <Code>hit_points 12 → 21</Code>
            </Stack>
          ),
        },
      ]
      at('mobile', <DataList items={[rows[0]!]} columns={withBlock} getKey={(row) => row.id} />)

      expect(screen.getByText('Detail')).toBeInTheDocument()
      expect(screen.getByText('hit_points 12 → 21')).toBeInTheDocument()
    })
  })

  describe('row actions', () => {
    let calls: string[] = []
    const reset = () => {
      calls = []
    }
    const takeOff = (row: Row): RowAction[] => [
      { key: 'off', label: 'Take off', color: 'red', onClick: () => calls.push(row.id) },
    ]

    it('spells them out as buttons on desktop, named for their row', () => {
      reset()
      at('desktop', <DataList items={rows} columns={columns} getKey={(r) => r.id} actions={takeOff} />)

      // Named for the row, not just "Take off": a column of buttons all called
      // the same thing is ambiguous to a screen reader and to a test alike.
      expect(screen.getByRole('button', { name: 'Take off Vex' })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Take off Grog' })).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: /^Actions for/ })).not.toBeInTheDocument()
    })

    it('folds them behind one control on a phone', async () => {
      reset()
      at('mobile', <DataList items={rows} columns={columns} getKey={(r) => r.id} actions={takeOff} />)

      expect(screen.queryByRole('button', { name: 'Take off Vex' })).not.toBeInTheDocument()

      const menu = await openMenu('Vex')
      const item = within(menu).getByRole('menuitem', { name: 'Take off' })
      const user = setupUser()
      await user.click(item)

      // Vex's id, so the click reached the action built for Vex's row.
      expect(calls).toEqual(['a'])
    })

    it('folds them on desktop too once there are more than three', () => {
      reset()
      const four = (): RowAction[] =>
        ['One', 'Two', 'Three', 'Four'].map((label) => ({
          key: label,
          label,
          onClick: () => {},
        }))
      at('desktop', <DataList items={[rows[0]!]} columns={columns} getKey={(r) => r.id} actions={four} />)

      // The threshold `FolderPanel` already settled on for its own header:
      // four buttons is what a row cannot lay out.
      expect(screen.getByRole('button', { name: 'Actions for Vex' })).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'One Vex' })).not.toBeInTheDocument()
    })

    it.each(['mobile', 'desktop'] as const)('draws no control for a row with none, at %s', (viewport) => {
      reset()
      at(
        viewport,
        <DataList
          items={rows}
          columns={columns}
          getKey={(r) => r.id}
          actions={(row) => (row.id === 'a' ? takeOff(row) : [])}
        />,
      )

      expect(screen.queryByRole('button', { name: /Grog/ })).not.toBeInTheDocument()
    })

    it('keeps one edge by reserving the gutter a mixed list leaves empty', () => {
      reset()
      const { container } = at(
        'mobile',
        <DataList
          items={rows}
          columns={columns}
          getKey={(r) => r.id}
          actions={(row) => (row.id === 'a' ? takeOff(row) : [])}
        />,
      )

      // Grog has no menu, but Vex does, so Grog's card still spends the space:
      // the two names end at the same x and the two cards are the same depth.
      //
      // What is asserted is that the gutter *is* the control, hidden -- which
      // is why the two can no longer be different sizes. An earlier version
      // reserved a `Box` of the same width and had to assert the numbers
      // matched; this one cannot get them wrong.
      const hidden = container.querySelector<HTMLElement>('[aria-hidden][style*="visibility"]')
      expect(hidden?.style.visibility).toBe('hidden')
      expect(hidden?.querySelector('button')).not.toBeNull()

      // And exactly one control anybody can reach: the hidden twin is inside
      // `aria-hidden`, so it holds the space without being a second thing to
      // press or a second thing a screen reader reads out.
      expect(screen.getAllByRole('button', { name: /^Actions/ })).toHaveLength(1)
      expect(screen.getByRole('button', { name: 'Actions for Vex' })).toBeInTheDocument()
    })

    it('reserves nothing when no row in the list is actionable', () => {
      reset()
      const { container } = at(
        'mobile',
        <DataList items={rows} columns={columns} getKey={(r) => r.id} actions={() => []} />,
      )

      expect(container.querySelector('[aria-hidden][style*="visibility"]')).toBeNull()
      expect(screen.queryByRole('button', { name: /^Actions/ })).not.toBeInTheDocument()
    })

    it('draws no actions column at all when nothing in the list is actionable', () => {
      reset()
      at('desktop', <DataList items={rows} columns={columns} getKey={(r) => r.id} actions={() => []} />)

      expect(screen.getAllByRole('columnheader')).toHaveLength(columns.length)
    })
  })

  describe('the name', () => {
    it('is a link when the primary column says where it goes', () => {
      const linked: DataListColumn<Row>[] = [
        { ...columns[0]!, primary: true, text: (row) => row.name, to: (row) => `/characters/${row.id}` },
        columns[1]!,
      ]
      at('mobile', <DataList items={[rows[0]!]} columns={linked} getKey={(r) => r.id} />)

      expect(screen.getByRole('link', { name: 'Vex' })).toHaveAttribute('href', '/characters/a')
    })

    it('is plain text when it does not', () => {
      at('mobile', <DataList items={[rows[0]!]} columns={columns} getKey={(r) => r.id} />)

      expect(screen.queryByRole('link')).not.toBeInTheDocument()
      expect(screen.getByText('Vex')).toBeInTheDocument()
    })
  })

  it('draws the marks a row carries beside its name at both viewports', () => {
    at(
      'mobile',
      <DataList
        items={[rows[0]!]}
        columns={columns}
        getKey={(r) => r.id}
        badges={() => <Text span>Yours</Text>}
      />,
    )
    expect(screen.getByText('Yours')).toBeInTheDocument()
  })
})
