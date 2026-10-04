import { useState } from 'react'
import { screen, fireEvent } from '@testing-library/react'
import { renderAt } from '@/test/render'
import type { PackSchema, PackValue } from '@/lib/api/packs'
import { PackForm } from './PackForm'

const schema: PackSchema = {
  root: {
    type: 'object',
    properties: {
      version: { type: 'string' },
      rules: { type: 'array', items: { type: 'object', ref: 'Rule' } },
      locales: { type: 'map', items: { type: 'string' } },
    },
  },
  definitions: {
    Rule: {
      type: 'object',
      properties: { manual: { type: 'boolean' }, minimumLevel: { type: 'number' } },
    },
  },
}
function Harness() {
  const [value, setValue] = useState<PackValue>({
    version: '1.0.0',
    icons: { spells: { 'guiding-mark': 'base64-artwork' } },
    rules: [{ manual: false, minimumLevel: 1 }],
    locales: { en: 'Sample' },
  })
  return (
    <>
      <PackForm
        field={schema.root}
        schema={schema}
        name="document"
        references={[]}
        value={value}
        onChange={(v) => setValue(v!)}
      />
      <output data-testid="result">{JSON.stringify(value)}</output>
    </>
  )
}
it('edits nested typed values without losing sibling content', () => {
  renderAt('desktop', <Harness />)
  fireEvent.change(screen.getByLabelText('Version'), { target: { value: '1.1.0' } })
  const details = screen.getByText('1').parentElement as HTMLDetailsElement
  details.open = true
  fireEvent(details, new Event('toggle'))
  fireEvent.click(screen.getByLabelText('Manual'))
  const result = JSON.parse(screen.getByTestId('result').textContent!)
  expect(result).toEqual({
    version: '1.1.0',
    icons: { spells: { 'guiding-mark': 'base64-artwork' } },
    rules: [{ manual: true, minimumLevel: 1 }],
    locales: { en: 'Sample' },
  })
})
it('adds and removes optional fields and map entries', () => {
  renderAt('desktop', <Harness />)
  fireEvent.click(screen.getByRole('button', { name: 'Remove Version' }))
  expect(JSON.parse(screen.getByTestId('result').textContent!).version).toBeUndefined()
  fireEvent.click(screen.getByRole('button', { name: 'Add Version' }))
  expect(JSON.parse(screen.getByTestId('result').textContent!).version).toBe('')
  fireEvent.change(screen.getByLabelText('Key'), { target: { value: 'ru' } })
  const add = screen.getAllByRole('button', { name: 'Add' })
  fireEvent.click(add.at(-1)!)
  expect(JSON.parse(screen.getByTestId('result').textContent!).locales).toEqual({
    en: 'Sample',
    ru: '',
  })
})
