import { screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { renderAt } from '@/test/render'
import { setupUser } from '@/test/user'

import { ImportCharacterScreen } from './ImportCharacterScreen'

/**
 * The import screen, against a response in the shape the real route sends.
 *
 * The payload below is the report the committed reference export actually
 * produces -- Urchin unresolved, the purse and the class resource skipped, the
 * half-elf's ability bonuses still open. Inventing a tidier one would not test
 * the case the screen exists for.
 */
const IMPORTED = {
  id: 'chr_000001',
  seq: 6,
  sheet: { identity: { name: 'Сахарок' } },
  report: {
    unresolved: [
      { field: 'character.background', detail: '"Urchin" is not in SRD 5.1' },
      { field: 'character.languages', detail: '"One language of your choice" is not an SRD language' },
    ],
    skipped: [
      { field: 'character.currency', detail: '0 cp, 0 sp, 0 ep, 10 gp, 0 pp -- a purse is not imported' },
      { field: 'character.classResources', detail: '"Sneak Attack" -- class resources are derived' },
    ],
    open: ['half-elf/ability-bonus/0', 'rogue/proficiency/0'],
  },
}

let posted: { url: string; body: string; method: string }[] = []

function mockApi(response: unknown = IMPORTED, status = 201) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      posted.push({
        url: String(input),
        method: init?.method ?? 'GET',
        body: String(init?.body ?? ''),
      })
      return new Response(JSON.stringify(response), {
        status,
        headers: { 'Content-Type': 'application/json' },
      })
    }),
  )
}

function renderImport(viewport: 'mobile' | 'desktop') {
  return renderAt(
    viewport,
    <MemoryRouter initialEntries={['/characters/import']}>
      <Routes>
        <Route path="/characters/import" element={<ImportCharacterScreen />} />
        <Route path="/characters/:id/build" element={<div>build screen</div>} />
        <Route path="/characters" element={<div>the party</div>} />
      </Routes>
    </MemoryRouter>,
  )
}

/**
 * Mantine's FileInput renders a button plus a hidden <input type="file">, so
 * the label points at the button and user.upload has to be given the input.
 */
function fileInputOf(container: HTMLElement): HTMLInputElement {
  const input = container.querySelector('input[type="file"]')
  if (input === null) throw new Error('the screen has no file input')
  return input as HTMLInputElement
}

function exportFile(): File {
  return new File([JSON.stringify({ exportedFrom: 'HexSheet' })], 'rogue.json', {
    type: 'application/json',
  })
}

beforeEach(() => {
  posted = []
  mockApi()
})

afterEach(() => {
  vi.unstubAllGlobals()
})

/**
 * One viewport, not two. Only `Columns`, `DataList`, `ModalSheet`,
 * `SectionDeck`, `TabDeck`, `SheetBody` and `RootShell` branch on width, and the suite runs without CSS, so a responsive
 * prop cannot move the DOM either -- nothing in this tree reaches any of them,
 * so a test at one width is a test of both. See docs/web.md.
 */
describe('ImportCharacterScreen', () => {
  const viewport = 'desktop'

  it('will not import until a file is chosen', () => {
    renderImport(viewport)
    expect(screen.getByRole('button', { name: 'Import' })).toBeDisabled()
  })

  it('posts the file to the import route as raw bytes', async () => {
    const user = setupUser()
    const { container } = renderImport(viewport)

    await user.upload(fileInputOf(container), exportFile())
    await user.click(screen.getByRole('button', { name: 'Import' }))

    await waitFor(() => expect(posted).toHaveLength(1))
    const [sent] = posted
    expect(sent).toBeDefined()
    expect(sent?.method).toBe('POST')
    expect(sent?.url).toContain('/v1/characters/import')
    // The export travels verbatim, not wrapped in an envelope.
    expect(JSON.parse(sent?.body ?? '{}')).toEqual({ exportedFrom: 'HexSheet' })
  })

  it('shows what did not come across, rather than navigating straight on', async () => {
    const user = setupUser()
    const { container } = renderImport(viewport)

    await user.upload(fileInputOf(container), exportFile())
    await user.click(screen.getByRole('button', { name: 'Import' }))

    expect(await screen.findByText(/Urchin/)).toBeInTheDocument()
    expect(screen.getByText(/a purse is not imported/)).toBeInTheDocument()
    expect(screen.getByText(/Sneak Attack/)).toBeInTheDocument()
    // Still on the import screen: the report has to be seen.
    expect(screen.queryByText('build screen')).not.toBeInTheDocument()
  })

  it('says how many choices are still open', async () => {
    const user = setupUser()
    const { container } = renderImport(viewport)

    await user.upload(fileInputOf(container), exportFile())
    await user.click(screen.getByRole('button', { name: 'Import' }))

    expect(await screen.findByText(/Still to decide/)).toBeInTheDocument()
    expect(screen.getByText(/these 2 choices are still open/)).toBeInTheDocument()
  })

  it('goes to the build screen, because an import answers nothing', async () => {
    const user = setupUser()
    const { container } = renderImport(viewport)

    await user.upload(fileInputOf(container), exportFile())
    await user.click(screen.getByRole('button', { name: 'Import' }))

    await user.click(await screen.findByRole('button', { name: /finish this character/i }))
    expect(await screen.findByText('build screen')).toBeInTheDocument()
  })

  it('reports a rejected file and stays put', async () => {
    mockApi(
      { error: { code: 'validation_error', message: 'this file is not a HexSheet export' } },
      400,
    )
    const user = setupUser()
    const { container } = renderImport(viewport)

    await user.upload(fileInputOf(container), exportFile())
    await user.click(screen.getByRole('button', { name: 'Import' }))

    expect(await screen.findByText(/could not be imported/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Import' })).toBeInTheDocument()
  })
})
