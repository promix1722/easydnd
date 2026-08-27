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
  /**
   * What the soft keyboard's Go key does, and what Enter in a field does.
   *
   * A dialog whose contents are a field and a confirm button is a form, and a
   * phone keyboard offers a Go key on the strength of that. Every dialog here
   * was a bare `TextInput` beside a `Button onClick`, so Go did nothing at all
   * -- you type a name, press the obvious key, and the app ignores you.
   *
   * It lives on the wrapper rather than at each call site because there are
   * eleven of these and they had already drifted three ways: two folder dialogs
   * wrapped their own `<form>`, `NameForm` listened for Enter on the input, and
   * the other eight did nothing. Passing it here also makes the confirm button
   * a `type="submit"`, which is what puts Go on the keyboard in the first place.
   *
   * Omit it where the dialog is not a form -- a confirmation with no field, the
   * invite sheet's read-only link.
   */
  onSubmit?: () => void
}

/**
 * A dialog that is a centred modal on desktop and a bottom sheet on mobile.
 *
 * Callers never branch on viewport: a centred modal on a phone puts its close
 * button out of thumb reach and fights the on-screen keyboard, so the two
 * renderings are genuinely different components -- which is exactly why this
 * wrapper exists rather than a pile of `visibleFrom` props at each call site.
 */
export function ModalSheet({
  opened,
  onClose,
  title,
  children,
  size = 'md',
  onSubmit,
}: ModalSheetProps) {
  const t = useT()
  const isDesktop = useIsDesktop()

  // Mantine's close button ships no accessible name of its own, so without
  // this it is a control a screen reader announces as "button" -- and the one
  // control every sheet has. Named here rather than at each call site: there
  // are two renderings of one component and they must not drift.
  const close = { 'aria-label': t('common.close') }

  // A real `<form>`, not a keydown handler: the Go key exists because the
  // browser can see a form with a submit button in it, and no amount of
  // listening for Enter conjures one.
  const body =
    onSubmit === undefined ? (
      children
    ) : (
      <form
        onSubmit={(event) => {
          event.preventDefault()
          onSubmit()
        }}
      >
        {children}
      </form>
    )

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
        {body}
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
       *
       * **`svh`, not `dvh`, and that is a bug fix.** `dvh` is *defined* to
       * change as dynamic browser UI appears and disappears -- and with
       * `interactive-widget=resizes-content` in index.html, the soft keyboard
       * resizes the layout viewport too. So a sheet capped in `dvh` had its
       * height chase every one of those resizes, and anything that resized in
       * response -- a dropdown repositioning inside it -- fed the next one.
       * Opening the invite sheet's role picker on a phone flashed continuously.
       * `svh` is the small viewport: fixed for the life of the page, so the cap
       * holds still while the content above the keyboard does the moving.
       */
      styles={{
        content: { height: 'auto', maxHeight: '85svh' },
        // The body scrolls rather than the sheet growing past its cap. With
        // `interactive-widget=resizes-content` in index.html the cap is measured
        // against the space left above the keyboard, so a form taller than that
        // -- the folder dialog, which carries a paragraph above its field --
        // keeps its field reachable instead of pushing it under the keys.
        body: { overflowY: 'auto' },
      }}
    >
      {body}
    </Drawer>
  )
}
