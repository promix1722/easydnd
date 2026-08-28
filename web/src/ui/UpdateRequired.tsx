import { useState } from 'react'

import { Button, Group, Stack, Text } from '@mantine/core'

import { useT } from '@/lib/i18n'

import { ModalSheet } from './ModalSheet'

export interface UpdateRequiredProps {
  opened: boolean
  /** Reloads onto the deployed release. Injected so a test can watch for it. */
  onReload: () => void
}

/**
 * Says that this tab is running a release the server has stopped serving, and
 * does not take no for an answer.
 *
 * The blocking is the design rather than an oversight. A dismissible banner
 * leaves someone talking to a newer API with older code, which is the exact
 * failure this whole mechanism exists to prevent -- and the failure is quiet:
 * requests succeed until one does not, and the one that does not is usually a
 * save. Between interrupting someone and losing their character sheet, this
 * interrupts.
 *
 * There is deliberately no "later". The only control is the one that fixes it.
 */
export function UpdateRequired({ opened, onReload }: UpdateRequiredProps) {
  const t = useT()
  const [reloading, setReloading] = useState(false)

  const reload = (): void => {
    // The reload can take a moment -- it waits for the new service worker to
    // take control before the page changes under anyone. Without this the
    // button looks broken for that second and gets pressed again.
    setReloading(true)
    void onReload()
  }

  return (
    <ModalSheet
      opened={opened}
      // Required by the primitive, unreachable here: nothing can dismiss this.
      onClose={() => {}}
      dismissible={false}
      title={t('update.title')}
      size="sm"
    >
      <Stack gap="md">
        <Text size="sm">{t('update.detail')}</Text>
        <Group justify="flex-end">
          <Button onClick={reload} loading={reloading}>
            {t('update.action')}
          </Button>
        </Group>
      </Stack>
    </ModalSheet>
  )
}
