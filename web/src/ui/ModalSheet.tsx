import { Drawer, Modal } from '@mantine/core'

import { useT } from '@/lib/i18n'
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
  const t = useT()
  const isDesktop = useIsDesktop()

  // Mantine's close button ships no accessible name of its own, so without
  // this it is a control a screen reader announces as "button" -- and the one
  // control every sheet has. Named here rather than at each call site: there
  // are two renderings of one component and they must not drift.
  const close = { 'aria-label': t('common.close') }

  if (isDesktop) {
    return (
      <Modal
        opened={opened}
        onClose={onClose}
        title={title}
        size={size}
        centered
        closeButtonProps={close}
      >
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
      closeButtonProps={close}
      /*
       * The height is set here rather than with `size`, and that is a bug fix
       * rather than a preference.
       *
       * `size="auto"` reads like it should hug the content and does nothing of
       * the sort: Mantine's `getSize` turns any non-numeric size into
       * `var(--drawer-size-<name>)`, and there is no `--drawer-size-auto` in
       * its stylesheet. An undefined custom property is *guaranteed-invalid*,
       * which is precisely the case where `var()` takes its fallback -- so
       * `height: var(--drawer-height, calc(100% - ...))` resolved to 100% and
       * every sheet on a phone opened as tall as the cap below allowed,
       * whatever was in it. A three-line "New folder" form filled the screen.
       *
       * `height: auto` is an inline style, so it beats the class rule outright
       * and needs no `!important`. The cap stays: anything taller than this
       * hides the sheet's own header behind the browser chrome on a short
       * phone viewport.
       */
      styles={{ content: { height: 'auto', maxHeight: '85dvh' } }}
    >
      {children}
    </Drawer>
  )
}
