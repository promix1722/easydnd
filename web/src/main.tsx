import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider } from 'react-router'

import { AuthProvider } from '@/lib/auth'
import { router } from '@/routes'
import { AppTheme } from '@/ui'

const container = document.getElementById('root')
if (!container) throw new Error('#root is missing from index.html')

createRoot(container).render(
  <StrictMode>
    <AppTheme>
      {/* Outside the router: the session decides which application the router
          renders, so it has to be known before any route is matched. */}
      <AuthProvider>
        <RouterProvider router={router} />
      </AuthProvider>
    </AppTheme>
  </StrictMode>,
)
