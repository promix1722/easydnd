import { afterEach, expect, it, vi } from 'vitest'

import { getCollection, getEntries } from './catalog'

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

it('keeps private selections separate and rechecks access for every collection read', async () => {
  const fetcher = vi.fn().mockResolvedValue(new Response(JSON.stringify([]), { headers: { 'Content-Type': 'application/json' } }))
  // Each response body is consumed once, including consecutive reads of one scope.
  fetcher.mockImplementation(async () => new Response(JSON.stringify([]), { headers: { 'Content-Type': 'application/json' } }))
  vi.stubGlobal('fetch', fetcher)
  await getCollection('feats', '/packs/catalog?packs=private%401.0.0')
  await getCollection('feats', '/packs/catalog?packs=private%401.0.0')
  await getCollection('feats', '/characters/one/catalog')
  expect(fetcher).toHaveBeenCalledTimes(3)
  expect(String(fetcher.mock.calls[0]?.[0])).toContain('/packs/catalog/feats?packs=private%401.0.0')
  expect(String(fetcher.mock.calls[2]?.[0])).toContain('/characters/one/catalog/feats')
})
