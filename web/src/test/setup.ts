import '@testing-library/jest-dom/vitest'
import { afterEach } from 'vitest'
import { cleanup } from '@testing-library/react'

import { resetViewport } from './viewport'

afterEach(() => {
  cleanup()
  resetViewport()
})
