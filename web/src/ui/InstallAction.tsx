import { useState } from 'react'

import { Box, Button, List, Stack, Text } from '@mantine/core'
import { IconDownload } from '@tabler/icons-react'

import { useT } from '@/lib/i18n'

import { ModalSheet } from './ModalSheet'

import { CHROME_INSET } from '@/theme/tokens'

import type { InstallOffer } from '@/lib/install'

export interface InstallActionProps {
  offer: InstallOffer
  /** Opens the browser's install dialog. Injected so a test can watch for it. */
  onInstall: () => void
}

/**
 * Offers to put easydnd on the home screen, where the browser allows it.
 *
 * A button rather than a banner, and that is web.dev's own guidance rather than
 * taste: "Don't show banners on initial page load or out of context", and "keep
 * promotions outside of the flow of your user journeys". It also means there is
 * nothing to dismiss, so nothing has to be remembered -- which is what keeps
 * this out of the argument about there being no localStorage in this client.
 *
 * **It owns its own place: the bottom left corner, at every width.** It sat in
 * the header, which is the row that says where you are and how to leave, and
 * this is neither -- it is an offer, and an offer belongs out of the way of the
 * work. The corner is also the only spot that is free in all three chromes: the
 * phone's header is one row of four controls at 390px, the desktop navbar keeps
 * its list at the top, and the landing footer already has three things in it.
 *
 * `position: fixed`, so where it is mounted does not matter and no shell has to
 * arrange it -- and low enough in the stack to sit under a dialog, since an
 * offer that floats over the sheet it opened is worse than no offer.
 *
 * Nothing is drawn unless there is something to offer. Already installed, or a
 * browser that cannot, and the corner is empty.
 *
 * iOS gets the same button and a different answer behind it, because iOS has no
 * install API at all -- no event to wait for, nothing to call. Every install
 * there is someone tapping Share and then Add to Home Screen, so the honest
 * thing is to say so. Not Safari-only: since iOS 16.4 the same two taps work in
 * Chrome, Edge and Firefox.
 */
export function InstallAction({ offer, onInstall }: InstallActionProps) {
  const t = useT()
  const [showing, setShowing] = useState(false)

  if (offer === 'none') return null

  return (
    <>
      <Box
        style={{
          position: 'fixed',
          // The strip a home indicator covers, spelled out rather than
          // imported: `shell/chrome.ts` owns the pair for the two bars, and
          // `@/ui` may not import `@/shell`. The fallback inside `env()` is
          // load-bearing -- see the note there.
          bottom: `calc(${CHROME_INSET}px + env(safe-area-inset-bottom, 0px))`,
          left: `calc(${CHROME_INSET}px + env(safe-area-inset-left, 0px))`,
          zIndex: 100,
        }}
      >
        <Button
          // Drawn rather than dissolved: this floats over whatever is behind
          // it, and `subtle` over a table is a word with no edges.
          variant="default"
          leftSection={<IconDownload size={16} />}
          onClick={offer === 'ios' ? () => setShowing(true) : onInstall}
        >
          {t('install.action')}
        </Button>
      </Box>

      <ModalSheet
        opened={showing}
        onClose={() => setShowing(false)}
        title={t('install.title')}
        size="sm"
      >
        <Stack gap="md">
          <Text size="sm">{t('install.lead')}</Text>
          <List size="sm" type="ordered">
            <List.Item>{t('install.share')}</List.Item>
            <List.Item>{t('install.addToHome')}</List.Item>
          </List>
          <Text size="sm" c="dimmed">
            {t('install.after')}
          </Text>
        </Stack>
      </ModalSheet>
    </>
  )
}
