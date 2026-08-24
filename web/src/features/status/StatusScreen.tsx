import { Stack, Text, Title } from '@/ui'

import { StatusPanel } from './StatusPanel'

/**
 * The deploy-diagnostics screen.
 *
 * It lives here rather than in routes/ because routes/ is the table and a
 * screen in it is drift -- the same reason every other screen moved when the
 * character features landed.
 */
export function StatusScreen() {
  return (
    <Stack gap="md">
      <div>
        <Title order={2}>System status</Title>
        <Text c="dimmed" size="sm">
          Which release this browser is talking to, on both sides of nginx.
        </Text>
      </div>
      <StatusPanel />
    </Stack>
  )
}
