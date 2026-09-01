import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider } from 'react-router'

import { AuthProvider } from '@/lib/auth'
import { LocaleProvider } from '@/lib/i18n'
import { router } from '@/routes'
import { AppTheme, UpdateGate } from '@/ui'

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
        {/* Above the session and outside the router: a tab running a release
            that is no longer deployed has to be told so on every page,
            including the ones that render without an account -- the landing
            chrome and /legal both sit outside RootGate.

            Inside `LocaleProvider` though, and it did not used to be. The
            comment here once read "it says nothing, so it needs no language",
            which stopped being true when the dialog got a heading, a sentence
            and a button: `UpdateRequired` calls `useT`. Mounted above the
            provider it ran before any i18next instance existed, and
            react-i18next said so on every cold start -- "you will need to pass
            in an i18next instance". It then found one anyway, because
            `createI18n` registers the global as a side effect of
            `initReactI18next`, so the dialog was quietly reading its words off
            a module-level singleton instead of out of context. That works
            until an instance is passed explicitly, which is exactly what
            `src/test/render.tsx` does. */}
        <UpdateGate />
        <AuthProvider>
          <RouterProvider router={router} />
        </AuthProvider>
      </LocaleProvider>
    </AppTheme>
  </StrictMode>,
)
