/**
 * The spell icons, one webp per slug.
 *
 * Generated offline by `make spell-icons` and committed under
 * `public/spells/`, which is the whole trick: the file name *is* the slug, so
 * the URL can be computed rather than looked up, and nothing about these 319
 * images enters the module graph at all. The browser asks for one when the
 * `<img>` for it scrolls into view, and never asks for the rest.
 *
 * They used to be imported through an eager `import.meta.glob`, for Vite's
 * content hashing and the `immutable` cache rule that follows it. That paid
 * for itself twice over in the wrong direction. Vite inlines assets under 4 kB
 * as base64, so 66 icons were baked bodily into the entry chunk -- 319 kB of a
 * 1.2 MB bundle, sent to every visitor who never opens this page. The other
 * 253 became a map of URL strings the bundle carried whether or not anything
 * rendered them. And in the dev server every entry in an eager glob is served
 * as its own module, so a cold load spent 319 round trips before the app
 * appeared.
 *
 * What that buys back is a hash in the file name. An unhashed file cannot be
 * `immutable` -- nginx's catch-all gives `/spells/` `no-cache`, so a repeat
 * visit revalidates and gets a 304 -- which is the correct policy for a name
 * that can be rerolled in place, and a few hundred bytes on a warm cache
 * against 319 kB on every cold one.
 *
 * A plain `<img>` rather than a `@/ui` re-export, the way `shell/Wordmark`
 * draws the brand: decorative, sized in the markup so a slow network cannot
 * shift the layout, and `alt=""` because the spell's name is always the very
 * next thing on the line.
 */

/**
 * Every SRD spell has an icon today -- 319 slugs, 319 files, checked by nothing
 * because `make spell-icons` draws from the same list. `onError` is the guard
 * for the day that stops being true: a missing file is answered by the SPA
 * fallback with `index.html`, which fails to decode as an image, and the icon
 * removes itself rather than leaving a broken-image glyph in the row.
 */
export function SpellIcon({ slug, size }: { slug: string; size: number }) {
  return (
    <img
      src={`/spells/${slug}.webp`}
      alt=""
      // The list is 319 rows and roughly 300 of them start below the fold.
      // This attribute is the entire feature: without it the page asks for
      // every icon at once.
      loading="lazy"
      // Decorative, so the row's text should not wait behind its decode.
      decoding="async"
      width={size}
      height={size}
      onError={(event) => {
        event.currentTarget.style.visibility = 'hidden'
      }}
      style={{ borderRadius: 4, flexShrink: 0, display: 'block' }}
    />
  )
}
