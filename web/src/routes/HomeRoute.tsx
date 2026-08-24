import { PasskeyNotice } from '@/features/auth'
import { CharacterListScreen } from '@/features/characters'
import { useAuth } from '@/lib/auth'
import { Stack } from '@/ui'

import { LandingPage } from './LandingPage'

/**
 * `/` for everybody.
 *
 * Signed out it is the landing page; signed in it is the party list. Branching
 * here rather than redirecting is what keeps the address bar honest: there is
 * one home page, and it shows you what you are entitled to see.
 *
 * System status is deliberately not on it. "Which release am I talking to" is
 * a deploy question rather than something either audience came here to read,
 * so it lives on `/status` alone.
 */
export function HomeRoute() {
  const { status } = useAuth()

  return status === 'authenticated' ? (
    <Stack gap="md">
      <PasskeyNotice />
      <CharacterListScreen />
    </Stack>
  ) : (
    <LandingPage />
  )
}
