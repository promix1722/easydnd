import { useEffect, useId, useRef, useState } from 'react'
import { Button, FileButton, Group, Stack, Text } from '@mantine/core'

import { useT } from '@/lib/i18n'
import { Avatar } from './Avatar'
import { ModalSheet } from './ModalSheet'
import { cropPortrait, portraitCrop, validatePortrait } from './portrait'

export function AvatarEditor({ image, fallback, pending = false, error, onSave }: {
  image?: string | undefined
  fallback?: string | undefined
  pending?: boolean
  error?: string | null | undefined
  onSave: (image: string) => Promise<unknown> | void
}) {
  const t = useT()
  const [file, setFile] = useState<File | null>(null)
  const [invalid, setInvalid] = useState(false)
  const reset = useRef<() => void>(null)
  return (
    <Stack gap="sm">
      <Group>
        <Avatar image={image} fallback={fallback} />
        <FileButton accept="image/jpeg,image/png,image/webp" resetRef={reset} onChange={(next) => {
          reset.current?.()
          if (!next) return
          setInvalid(false)
          try { validatePortrait(next); setFile(next) } catch { setInvalid(true) }
        }}>
          {(props) => <Button {...props} variant="light" disabled={pending}>{t('character.image.upload')}</Button>}
        </FileButton>
        {image && <Button variant="subtle" disabled={pending} onClick={() => { setInvalid(false); void onSave('') }}>{t('character.image.remove')}</Button>}
      </Group>
      <Text size="xs" c="dimmed">{t('character.image.hint')}</Text>
      {(invalid || error) && <Text size="sm" c="red" role="alert">{invalid ? t('error.field.character.image.invalid') : error}</Text>}
      {file && <CropEditor file={file} onClose={() => setFile(null)} onSave={(value) => {
        setFile(null)
        void onSave(value)
      }} />}
    </Stack>
  )
}

function CropEditor({ file, onClose, onSave }: { file: File; onClose: () => void; onSave: (image: string) => void }) {
  const t = useT()
  const id = useId()
  const [image, setImage] = useState<HTMLImageElement | null>(null)
  const [invalid, setInvalid] = useState(false)
  const [x, setX] = useState(50)
  const [y, setY] = useState(50)
  const [zoom, setZoom] = useState(1)
  const gesture = useRef<{ x: number; y: number; originX: number; originY: number } | null>(null)
  useEffect(() => {
    const url = URL.createObjectURL(file)
    const source = new Image()
    let active = true
    source.src = url
    source.decode().then(() => {
      if (active) {
        if (source.naturalWidth && source.naturalHeight) setImage(source)
        else setInvalid(true)
      }
    }).catch(() => { if (active) setInvalid(true) })
    return () => { active = false; URL.revokeObjectURL(url) }
  }, [file])
  const crop = image ? portraitCrop(image, x, y, zoom) : null
  const scale = crop ? 256 / crop.side : 1
  const clamp = (value: number) => Math.max(0, Math.min(100, value))

  return (
    <ModalSheet opened onClose={onClose} title={t('avatar.crop')} size="sm">
      <Stack gap="sm">
        <Text size="sm">{t('avatar.cropHint')}</Text>
        <div style={{ width: 256, height: 256, alignSelf: 'center', position: 'relative', overflow: 'hidden', borderRadius: 8, outline: '1px solid black', background: '#252c3b', touchAction: 'none', cursor: 'grab' }}
          onPointerDown={(event) => {
            if (!image) return
            event.currentTarget.setPointerCapture(event.pointerId)
            gesture.current = { x: event.clientX, y: event.clientY, originX: x, originY: y }
          }}
          onPointerMove={(event) => {
            const held = gesture.current
            if (!held || !image || !crop) return
            const width = (image.naturalWidth - crop.side) * scale
            const height = (image.naturalHeight - crop.side) * scale
            if (width) setX(clamp(held.originX - (event.clientX - held.x) / width * 100))
            if (height) setY(clamp(held.originY - (event.clientY - held.y) / height * 100))
          }}
          onPointerUp={() => { gesture.current = null }}
          onPointerCancel={() => { gesture.current = null }}>
          {image && crop && <img src={image.src} alt={t('avatar.preview')} draggable={false} style={{ position: 'absolute', maxWidth: 'none', width: image.naturalWidth * scale, height: image.naturalHeight * scale, left: -crop.left * scale, top: -crop.top * scale, pointerEvents: 'none' }} />}
        </div>
        {[
          { key: 'zoom', label: t('avatar.zoom'), value: zoom, max: 3, min: 1, step: 0.01, set: setZoom },
          { key: 'horizontal', label: t('avatar.horizontal'), value: x, max: 100, min: 0, step: 1, set: setX },
          { key: 'vertical', label: t('avatar.vertical'), value: y, max: 100, min: 0, step: 1, set: setY },
        ].map((control) => <Stack key={control.key} gap={0}>
          <Text component="label" htmlFor={`${id}-${control.key}`} size="sm">{control.label}</Text>
          <input id={`${id}-${control.key}`} type="range" min={control.min} max={control.max} step={control.step} value={control.value} disabled={!image} onChange={(event) => control.set(Number(event.currentTarget.value))} />
        </Stack>)}
        {invalid && <Text size="sm" c="red" role="alert">{t('error.field.character.image.invalid')}</Text>}
        <Group justify="flex-end">
          <Button variant="default" onClick={onClose}>{t('common.cancel')}</Button>
          <Button disabled={!image || invalid} onClick={() => {
            if (!image) return
            try { onSave(cropPortrait(image, x, y, zoom)) } catch { setInvalid(true) }
          }}>{t('avatar.useCrop')}</Button>
        </Group>
      </Stack>
    </ModalSheet>
  )
}
