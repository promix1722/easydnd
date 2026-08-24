import { useAuth } from '@/lib/auth'
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

  if (status === 'loading') {
    return (
      <Center h="100dvh">
        <Loader aria-label="Checking your session" />
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
          <Alert color="yellow" title="Cannot reach easydnd">
            <Text size="sm">
              You appear to be offline, so we could not check whether you are signed in.
            </Text>
          </Alert>
          <Button onClick={() => void refresh()}>Try again</Button>
        </Stack>
      </Center>
    )
  }

  return status === 'authenticated' ? <RootShell /> : <LandingShell />
}
