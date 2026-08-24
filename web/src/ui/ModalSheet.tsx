import { Drawer, Modal } from '@mantine/core'
import type { ReactNode } from 'react'

import { useIsDesktop } from './useIsDesktop'

export interface ModalSheetProps {
  opened: boolean
  onClose: () => void
  title?: ReactNode
  children: ReactNode
  /** Desktop modal width. Ignored on mobile, where the sheet is full-width. */
  size?: string | number
}

/**
 * A dialog that is a centred modal on desktop and a bottom sheet on mobile.
 *
 * Callers never branch on viewport: a centred modal on a phone puts its close
 * button out of thumb reach and fights the on-screen keyboard, so the two
 * renderings are genuinely different components -- which is exactly why this
 * wrapper exists rather than a pile of `visibleFrom` props at each call site.
 */
export function ModalSheet({ opened, onClose, title, children, size = 'md' }: ModalSheetProps) {
  const isDesktop = useIsDesktop()

  if (isDesktop) {
    return (
      <Modal opened={opened} onClose={onClose} title={title} size={size} centered>
        {children}
      </Modal>
    )
  }

  return (
    <Drawer
      opened={opened}
      onClose={onClose}
      title={title}
      position="bottom"
      size="auto"
      // Anything taller than this hides the sheet's own header behind the
      // browser chrome on a short phone viewport.
      styles={{ content: { maxHeight: '85dvh' } }}
    >
      {children}
    </Drawer>
  )
}
