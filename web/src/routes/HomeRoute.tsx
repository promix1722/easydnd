import { lazy } from 'react'

import { useAuth } from '@/lib/auth'

import { LandingPage } from './LandingPage'

// Fetched on first use, like every screen behind sign-in: a static import
// here would put the list, and every feature barrel it reaches, in the chunk
// a signed-out visitor downloads to read the landing page. See ./index.tsx.
const CharacterListScreen = lazy(() =>
  import('@/features/characters/CharacterListScreen').then((m) => ({ default: m.CharacterListScreen })),
)

/**
 * `/` for everybody.
 *
 * Signed out it is the landing page; signed in it is the character list. Branching
 * here rather than redirecting is what keeps the address bar honest: there is
 * one home page, and it shows you what you are entitled to see.
 *
 * System status is deliberately not on it. "Which release am I talking to" is
 * a deploy question rather than something either audience came here to read,
 * so it is not shown here.
 *
 * Nor is the guest notice. What a guest session costs belongs where a guest
 * goes to find out about their account -- `/account` says it, and the header
 * names the session beside the button that ends it -- not standing over the
 * character list every time they open the app.
 */
export function HomeRoute() {
  const { status } = useAuth()

  return status === 'authenticated' ? <CharacterListScreen /> : <LandingPage />
}
