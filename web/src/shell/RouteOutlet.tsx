import { Suspense } from 'react'
import { Outlet } from 'react-router'

import { Center, Loader } from '@/ui'

/**
 * Where a shell draws the matched route.
 *
 * The boundary is here, inside the chrome, because the screens are fetched on
 * first visit (see routes/index.tsx): a deep link then shows the header and a
 * loader while its chunk arrives, rather than a blank window. A navigation
 * between screens never reaches the fallback -- the router makes it a
 * transition, so the page being left stays up until the next one is ready.
 */
export function RouteOutlet() {
  return (
    <Suspense fallback={<Center py="xl"><Loader /></Center>}>
      <Outlet />
    </Suspense>
  )
}
