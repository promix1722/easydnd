import type { ReactElement } from 'react'

import { useAuth } from '@/lib/auth'

import { NotFoundPage } from './NotFoundPage'

/**
 * Renders a screen only for a superadmin, and the not-found page for
 * everybody else -- the same answer the server gives their requests, so the
 * client does not confirm what the API declines to.
 *
 * This is a courtesy and not the guard: `admin` is a flag on the session the
 * server sent, and every admin route asks the question again.
 */
export function AdminOnly({ children }: { children: ReactElement }): ReactElement {
  const { user } = useAuth()
  return user?.admin ? children : <NotFoundPage />
}
