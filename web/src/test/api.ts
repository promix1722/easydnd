/**
 * The path half of an API call, without the query.
 *
 * Tests route a stubbed `fetch` by URL, and every request now carries
 * `?locale=` -- the client appends it so the server can answer in the language
 * the visitor chose (src/lib/api/locale.ts). Matching on the whole string
 * makes every such test hostage to the next query parameter anybody adds, so
 * they match on the path and assert the parameter where it is the subject.
 */
export function apiPath(url: string): string {
  return url.split('?')[0] ?? url
}
