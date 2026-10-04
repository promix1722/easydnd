import { useState } from 'react'

/** Seeded artwork is served by the API; the browser never reads the seed pack. */
export function SpellIcon({ slug, size, revision }: { slug: string; size: number; revision?: string }) {
  const path = slug.split('/').map(encodeURIComponent).join('/')
  const src = `/v1/spell-icons/${path}.webp${revision ? `?v=${encodeURIComponent(revision)}` : ''}`
  const [failedSource, setFailedSource] = useState<string | null>(null)

  return (
    <img
      key={src}
      src={src}
      alt=""
      loading="lazy"
      decoding="async"
      width={size}
      height={size}
      onError={() => setFailedSource(src)}
      style={{
        borderRadius: 4,
        flexShrink: 0,
        display: 'block',
        visibility: failedSource === src ? 'hidden' : 'visible',
      }}
    />
  )
}
