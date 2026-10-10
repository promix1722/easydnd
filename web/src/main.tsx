import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider } from 'react-router'

import { AppearanceProvider } from '@/lib/appearance'
import { startAnalytics } from '@/lib/analytics'
import { AuthProvider } from '@/lib/auth'
import { LocaleProvider } from '@/lib/i18n'
import { router } from '@/routes'
import { AppTheme, UpdateGate } from '@/ui'

const container = document.getElementById('root')
if (!container) throw new Error('#root is missing from index.html')

void startAnalytics(router)

createRoot(container).render(
  <StrictMode>
    {/* Session and language wrap appearance so preferences follow the account;
        the design system and update dialog stay available on every route. */}
    <LocaleProvider>
      <AuthProvider>
        <AppearanceProvider>
          <AppTheme>
            <UpdateGate />
            <RouterProvider router={router} />
          </AppTheme>
        </AppearanceProvider>
      </AuthProvider>
    </LocaleProvider>
  </StrictMode>,
)
