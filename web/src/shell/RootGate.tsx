import { useAuth } from '@/lib/auth'
import { useT } from '@/lib/i18n'
import { Alert, Button, Center, Loader, Stack, Text } from '@/ui'

import { LandingShell } from './LandingShell'
import { RootShell } from './RootShell'

/**
 * Picks which application the visitor sees.
 *
 * This is the whole of the "two experiences, one build" mechanism: one
 * hostname, one bundle, one route table, and a single branch here on whether
 * the server recognises the session cookie. The URL never changes, so a deep
 * link survives signing in -- the same route simply renders its real content
 * once the state flips.
 */
export function RootGate() {
  const { status, refresh } = useAuth()
  const t = useT()

  if (status === 'loading') {
    return (
      <Center h="100dvh">
        <Loader aria-label={t('auth.checkingSession')} />
      </Center>
    )
  }

  if (status === 'offline') {
    // Deliberately not the landing page. We do not know that this person is
    // signed out -- only that we could not ask -- and guessing "signed out"
    // would eject them every time the network dropped.
    return (
      <Center h="100dvh" p="md">
        <Stack align="center" gap="md" maw={420}>
          <Alert color="yellow" title={t('offline.title')}>
            <Text size="sm">{t('offline.detail')}</Text>
          </Alert>
          <Button onClick={() => void refresh()}>{t('page.retry')}</Button>
        </Stack>
      </Center>
    )
  }

  return status === 'authenticated' ? <RootShell /> : <LandingShell />
}
