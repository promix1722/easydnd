import type { ReactNode } from 'react'
import { Button, Group, Divider, Modal, Stack } from '@mantine/core'
import { useT } from '@/lib/i18n'

import { useIsDesktop } from './useIsDesktop'

/** Inline reading on desktop; a full-screen detail page on phones. */
export function ChoiceDetails({ title, onBack, actions, mobileSummary, children }: {
  title: string
  onBack: () => void
  actions: ReactNode
  mobileSummary?: ReactNode
  children: ReactNode
}) {
  const t = useT()
  const isDesktop = useIsDesktop()
  if (isDesktop) {
    return <Stack role="region" aria-label={title} gap="md" p="sm">
      <Divider />
      {children}
    </Stack>
  }
  return (
    <Modal opened onClose={onBack} title={title} fullScreen withCloseButton={false}>
      <Stack gap="md">
        <Group justify="space-between">
          <Button variant="default" onClick={onBack}>{t('common.back')}</Button>
          {actions}
        </Group>
        {mobileSummary}
        {children}
      </Stack>
    </Modal>
  )
}
