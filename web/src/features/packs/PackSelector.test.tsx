import { fireEvent, screen, waitFor } from '@testing-library/react'
import { renderAt } from '@/test/render'
import { PackSelector } from './PackSelector'
const base = { id: 'srd-2014', version: '1.0.0', digest: 'base' }
const extra = { id: 'extra', version: '1.0.0', digest: 'extra' }
it('starts with SRD and resolves a selected addon before applying', async () => {
  const lock = { edition: '2014', semantics: '1', packs: [extra, base] }
  const available = {
    defaultRules: { ...lock, packs: [base] },
    packs: [
      {
        id: base.id,
        title: 'SRD 5.1',
        builtin: true,
        owned: false,
        archived: false,
        revision: 0,
        releases: [base],
      },
      {
        id: extra.id,
        title: 'My spells',
        builtin: false,
        owned: true,
        archived: false,
        revision: 1,
        releases: [extra],
      },
    ],
  }
  const resolutions: unknown[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, init?: RequestInit) => {
      const resolving = url.includes('/resolve')
      if (resolving) resolutions.push(JSON.parse(String(init?.body)).packs)
      return new Response(JSON.stringify(resolving ? lock : available), {
        headers: { 'Content-Type': 'application/json' },
      })
    }),
  )
  const changed = vi.fn()
  const dirty = vi.fn()
  renderAt('desktop', <PackSelector onChange={changed} onDirtyChange={dirty} />)
  expect(await screen.findByRole('button', { name: /SRD 5.1/ })).toHaveAttribute('aria-pressed', 'true')
  fireEvent.click(screen.getByRole('button', { name: /My spells/ }))
  expect(screen.queryByRole('combobox')).not.toBeInTheDocument()
  expect(dirty).toHaveBeenCalledWith(true)
  expect(changed).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('button', { name: 'Confirm' }))
  await waitFor(() => expect(changed).toHaveBeenCalledWith(lock))
  expect(resolutions).toEqual([[base, extra]])
  expect(dirty).toHaveBeenLastCalledWith(false)
})

it('clears a draft selection and prevents confirming an empty pack set', async () => {
  vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({
    defaultRules: { edition: '2014', semantics: '1', packs: [base] },
    packs: [{ id: base.id, title: 'SRD 5.1', releases: [base] }],
  }), { headers: { 'Content-Type': 'application/json' } })))
  const changed = vi.fn()
  renderAt('desktop', <PackSelector onChange={changed} />)
  expect(await screen.findByRole('button', { name: /SRD 5.1/ })).toHaveAttribute('aria-pressed', 'true')
  fireEvent.click(screen.getByRole('button', { name: 'Clear' }))
  expect(screen.getByRole('button', { name: /SRD 5.1/ })).toHaveAttribute('aria-pressed', 'false')
  expect(screen.getByRole('button', { name: 'Choose 1 more' })).toBeDisabled()
  expect(changed).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('button', { name: /SRD 5.1/ }))
  expect(screen.getByRole('button', { name: 'Confirm' })).toBeEnabled()
})

it('shows finalized packs without any editing controls or attention border', async () => {
  vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({
    packs: [{ id: base.id, title: 'SRD 5.1', releases: [base] }],
  }), { headers: { 'Content-Type': 'application/json' } })))
  const changed = vi.fn()
  renderAt('desktop', <PackSelector finalized value={{ edition: '2014', semantics: '1', packs: [base] }} onChange={changed} disclosure={{ open: true, onOpen: vi.fn() }} />)
  expect(await screen.findByText('SRD 5.1')).toBeInTheDocument()
  expect(screen.getByText('This choice is final.')).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: 'Confirm' })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: 'Clear' })).not.toBeInTheDocument()
  expect(screen.queryByRole('combobox')).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Rule packs' }).closest('[data-highlighted="true"]')).toBeNull()
  expect(changed).not.toHaveBeenCalled()
})
