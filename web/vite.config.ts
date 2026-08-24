/// <reference types="vitest/config" />
import { fileURLToPath } from 'node:url'
import { defineConfig, type Plugin } from 'vite'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'

/**
 * Emits dist/version.json containing the build's commit SHA.
 *
 * This is the frontend half of the deploy contract. The Go build already
 * asserts that `-X …buildinfo.Version` actually landed, because a wrong
 * package path is a silent no-op that ships a binary reporting "dev". The same
 * trap exists here: an unset VITE_APP_VERSION would ship a bundle claiming
 * "dev", the pipeline's public check would never match the SHA, and the deploy
 * would fail minutes later with no obvious cause. Fail at build time instead.
 */
function versionManifest(): Plugin {
  let version = ''
  return {
    name: 'easydnd:version-manifest',
    apply: 'build',
    configResolved() {
      version = process.env.VITE_APP_VERSION ?? ''
      if (!version) {
        throw new Error(
          'VITE_APP_VERSION is not set. Build via `make web/build`, or set it ' +
            'explicitly: VITE_APP_VERSION=$(git rev-parse HEAD) npm run build',
        )
      }
    },
    generateBundle() {
      // Shape mirrors GET /v1/version so both halves of a release answer the
      // same question the same way.
      this.emitFile({
        type: 'asset',
        fileName: 'version.json',
        source: JSON.stringify({ version }) + '\n',
      })
    },
  }
}

export default defineConfig({
  plugins: [
    react(),
    versionManifest(),
    VitePWA({
      registerType: 'autoUpdate',
      // The service worker would otherwise shadow the dev server's module
      // graph and serve stale chunks after every edit.
      devOptions: { enabled: false },
      includeAssets: ['favicon.svg', 'icons/apple-touch-icon.png'],
      workbox: {
        globPatterns: ['**/*.{js,css,html,svg,png,woff2}'],
        // version.json must never be answered from cache: the deploy pipeline
        // reads it through the public URL to prove which release is live.
        navigateFallbackDenylist: [/^\/v1\//],
      },
      manifest: {
        name: 'easydnd - D&D character and battle tracker',
        short_name: 'easydnd',
        description: 'Create characters, level them up, and run encounters.',
        theme_color: '#7a1f2b',
        background_color: '#1a1b1e',
        display: 'standalone',
        orientation: 'portrait',
        start_url: '/',
        scope: '/',
        icons: [
          { src: 'icons/icon-192.png', sizes: '192x192', type: 'image/png' },
          { src: 'icons/icon-512.png', sizes: '512x512', type: 'image/png' },
          {
            src: 'icons/icon-maskable-512.png',
            sizes: '512x512',
            type: 'image/png',
            purpose: 'maskable',
          },
        ],
      },
    }),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    // The API binds 127.0.0.1 and has no CORS middleware by design; proxying
    // keeps dev same-origin so the browser never needs one.
    proxy: {
      '/v1': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: false,
      },
    },
  },
  build: {
    // Worth the ~2 MB per release: without maps, a production stack trace is
    // minified chunk offsets, and the release that produced them is pruned
    // after five deploys.
    sourcemap: true,
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    css: true,
  },
})
