import { afterEach, expect, it, vi } from 'vitest'

import { getEntries } from './catalog'

afterEach(() => vi.unstubAllGlobals())

it('loads a whole spell-choice pool within the API slug limit', async () => {
  const requested: string[][] = []
  vi.stubGlobal('fetch', vi.fn(async (address: string) => {
    const slugs = new URL(address, 'http://localhost').searchParams.get('slugs')!.split(',')
    requested.push(slugs)
    return new Response(JSON.stringify(slugs.map((slug) => ({ slug, name: slug }))), { status: slugs.length <= 200 ? 200 : 400, headers: { 'Content-Type': 'application/json' } })
  }))
  const slugs = Array.from({ length: 319 }, (_, index) => `spell-${index}`)
  const entries = await getEntries('spells', slugs)
  expect(entries.map((entry) => entry.slug)).toEqual(slugs)
  expect(requested.every((chunk) => chunk.length <= 200)).toBe(true)
})
