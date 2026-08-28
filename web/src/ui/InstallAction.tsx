import { useState } from 'react'

import { Button, List, Stack, Text } from '@mantine/core'
import { IconDownload } from '@tabler/icons-react'

import { useT } from '@/lib/i18n'

import { ModalSheet } from './ModalSheet'

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
 * Nothing is drawn unless there is something to offer. Already installed, or a
 * browser that cannot, and the header is exactly as it was.
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
      <Button
        variant="subtle"
        leftSection={<IconDownload size={16} />}
        onClick={offer === 'ios' ? () => setShowing(true) : onInstall}
      >
        {t('install.action')}
      </Button>

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
