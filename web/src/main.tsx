import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider } from 'react-router'

import { AppearanceProvider } from '@/lib/appearance'
import { startAnalytics } from '@/lib/analytics'
import { AuthProvider } from '@/lib/auth'
import { LocaleProvider } from '@/lib/i18n'
import { reloadOntoDeployedRelease } from '@/lib/version'
import { router } from '@/routes'
import { AppTheme, UpdateGate } from '@/ui'

const container = document.getElementById('root')
if (!container) throw new Error('#root is missing from index.html')

void startAnalytics(router)

// A screen's chunk that will not load almost always means a release went out
// while this tab was open: the file it is asking for belonged to the previous
// one. Vite reports that as an event, and the answer is the one the update
// dialog gives -- reload onto what is deployed.
// ponytail: once per visit, so a chunk that is genuinely missing cannot loop;
// after that the update dialog and the router's error page are what is left.
window.addEventListener('vite:preloadError', () => {
  try {
    if (window.sessionStorage.getItem('easydnd.chunkReload') !== null) return
    window.sessionStorage.setItem('easydnd.chunkReload', '1')
  } catch {
    return
  }
  void reloadOntoDeployedRelease()
})

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
