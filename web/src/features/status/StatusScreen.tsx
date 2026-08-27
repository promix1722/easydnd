import { Stack, Text, Title } from '@/ui'

import { StatusPanel } from './StatusPanel'

import { useT } from '@/lib/i18n'

/**
 * The deploy-diagnostics screen.
 *
 * It lives here rather than in routes/ because routes/ is the table and a
 * screen in it is drift -- the same reason every other screen moved when the
 * character features landed.
 */
export function StatusScreen() {
  const t = useT()

  return (
    <Stack gap="md">
      <div>
        <Title order={2}>{t('status.title')}</Title>
        <Text c="dimmed" size="sm">
          {t('status.subtitle')}
        </Text>
      </div>
      <StatusPanel />
    </Stack>
  )
}
