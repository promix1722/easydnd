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
  expect(await screen.findByRole('checkbox', { name: 'SRD 5.1' })).toBeChecked()
  fireEvent.click(screen.getByRole('checkbox', { name: 'My spells' }))
  expect(dirty).toHaveBeenCalledWith(true)
  expect(changed).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('button', { name: 'Apply selection' }))
  await waitFor(() => expect(changed).toHaveBeenCalledWith(lock))
  expect(resolutions).toEqual([[base, extra]])
  expect(dirty).toHaveBeenLastCalledWith(false)
})
