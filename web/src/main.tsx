import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider } from 'react-router'

import { AuthProvider } from '@/lib/auth'
import { LocaleProvider } from '@/lib/i18n'
import { router } from '@/routes'
import { AppTheme } from '@/ui'

const container = document.getElementById('root')
if (!container) throw new Error('#root is missing from index.html')

createRoot(container).render(
  <StrictMode>
    <AppTheme>
      {/* Both outside the router, and for the same reason: neither the session
          nor the language is a fact about a route. The session decides which
          application the router renders, so it has to be known before any route
          is matched; the language decides what every one of them says, and a
          screen that waited for it would flash English first. */}
      <LocaleProvider>
        <AuthProvider>
          <RouterProvider router={router} />
        </AuthProvider>
      </LocaleProvider>
    </AppTheme>
  </StrictMode>,
)
