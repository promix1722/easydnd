import { Avatar as MantineAvatar } from '@mantine/core'
import { IconUserCircle } from '@tabler/icons-react'

/** Portrait tile matching the spell artwork's background and corner radius. */
export function Avatar({ image, fallback, size = 48 }: { image?: string | undefined; fallback?: string | undefined; size?: number }) {
  return (
    <MantineAvatar src={image || fallback || null} alt="" size={size} radius={8}
      style={{ flexShrink: 0, border: '1px solid black', background: '#252c3b' }}
      styles={{ image: { objectFit: image ? 'cover' : 'contain', padding: image ? 0 : 3 }, placeholder: { display: 'grid', placeItems: 'center', background: 'transparent', color: '#cbd5e1', border: 0, lineHeight: 0 } }}>
      <IconUserCircle size={Math.round(size * 2 / 3)} aria-hidden="true" />
    </MantineAvatar>
  )
}
