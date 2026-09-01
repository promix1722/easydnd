import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

const html = readFileSync(resolve(process.cwd(), 'index.html'), 'utf8')

function occurrences(needle: string): number {
  return html.split(needle).length - 1
}

describe('the initial search document', () => {
  it('has one canonical title and description', () => {
    expect(occurrences('<title>')).toBe(1)
    expect(occurrences('name="description"')).toBe(1)
    expect(occurrences('rel="canonical"')).toBe(1)
    expect(html).toContain('href="https://easydnd.org/"')
    expect(html).toContain('D&amp;D 5e Character Builder &amp; Session Manager')
  })

  it('contains meaningful content before JavaScript runs', () => {
    expect(occurrences('<h1>')).toBe(1)
    expect(html).toContain('<h1>Build a D&amp;D character</h1>')
    expect(html).toContain('<h2>Join a group</h2>')
    expect(html).toContain('<h2>Run sessions</h2>')
    expect(html.indexOf('<main>')).toBeLessThan(html.indexOf('<script type="module"'))
  })

  it('publishes valid WebApplication structured data', () => {
    const match = html.match(/<script type="application\/ld\+json">([\s\S]*?)<\/script>/)
    expect(match).not.toBeNull()

    const data = JSON.parse(match?.[1] ?? '') as Record<string, unknown>
    expect(data['@context']).toBe('https://schema.org')
    expect(data['@type']).toBe('WebApplication')
    expect(data.url).toBe('https://easydnd.org/')
    expect(data.featureList).toEqual(expect.arrayContaining(['D&D 5e character creation']))
  })
})
