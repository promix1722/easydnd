/**
 * The spell icons, one webp per slug.
 *
 * Generated offline by `make spell-icons` and committed under
 * `src/assets/spells/` -- imported rather than dropped in `public/` so Vite
 * content-hashes them and nginx's immutable cache rule applies, the same
 * trade the landing images made (docs/web.md). The glob makes a new icon a
 * new file, not a new import line.
 *
 * A plain `<img>` rather than a `@/ui` re-export, the way `shell/Wordmark`
 * draws the brand: decorative, sized in the markup so a slow network cannot
 * shift the layout, and `alt=""` because the spell's name is always the very
 * next thing on the line.
 */

const icons = import.meta.glob('../../assets/spells/*.webp', {
  eager: true,
  query: '?url',
  import: 'default',
}) as Record<string, string>

const bySlug = new Map(
  Object.entries(icons).map(([path, url]) => [path.replace(/^.*\/|\.webp$/g, ''), url]),
)

/** Nothing for a slug with no icon: a spell without one looks as it did before icons. */
export function SpellIcon({ slug, size }: { slug: string; size: number }) {
  const url = bySlug.get(slug)
  if (url === undefined) return null
  return (
    <img
      src={url}
      alt=""
      width={size}
      height={size}
      style={{ borderRadius: 4, flexShrink: 0, display: 'block' }}
    />
  )
}
