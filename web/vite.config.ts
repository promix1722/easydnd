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

/**
 * Where this dev server listens, and where a browser reaches it.
 *
 * They are the same thing until something proxies between them. Each worktree
 * claims a slot (`make ports`), so Vite binds one port while the browser dials
 * another on the proxy in front. Two settings have to be told: `allowedHosts`,
 * because Vite refuses a Host header it does not recognise, and
 * `hmr.clientPort`, because the HMR socket is dialled by the browser and would
 * otherwise be sent to a port nothing outside can reach.
 *
 * EASYDND_WEB_PUBLIC_URL must appear byte for byte in this worktree's
 * `auth.rp_origins`, or middleware.SameOrigin rejects every POST -- including
 * the guest sign-in, which over plain HTTP is the only way in at all.
 *
 * The defaults reproduce what an unclaimed worktree has always done, so
 * `make web/dev` in a fresh clone is unchanged.
 */
const devPort = Number(process.env.EASYDND_WEB_PORT ?? 5173)
const apiTarget = process.env.EASYDND_API_ORIGIN ?? 'http://127.0.0.1:8080'
const publicUrl = process.env.EASYDND_WEB_PUBLIC_URL
  ? new URL(process.env.EASYDND_WEB_PUBLIC_URL)
  : null
const publicPort = publicUrl
  ? Number(publicUrl.port) || (publicUrl.protocol === 'https:' ? 443 : 80)
  : devPort

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
    // Loopback, like the API and for the same reason: this may be a shared
    // machine, and whatever proxies in front connects to 127.0.0.1. Named
    // rather than left default because Vite's `localhost` can resolve to ::1
    // while the proxy dials the v4 literal.
    host: '127.0.0.1',
    port: devPort,
    // A fixed port, not "the next one free". Three other things name this port
    // -- the proxy in front, auth.rp_origins, and the neighbouring worktree
    // that must not be handed it -- so drifting to 5174 would not surface as
    // "port busy" but as "request origin is not allowed" on every POST, which
    // is a much longer afternoon.
    strictPort: true,
    ...(publicUrl
      ? {
          // Vite rejects an unrecognised Host header and the proxy forwards
          // the browser's, so the public name has to be listed. localhost and
          // bare IPs are always allowed and need no entry.
          allowedHosts: [publicUrl.hostname],
          // hmr.host stays unset: the client falls back to location.hostname,
          // which is already right. Only the port is wrong without this.
          hmr: {
            clientPort: publicPort,
            protocol: publicUrl.protocol === 'https:' ? 'wss' : 'ws',
          },
        }
      : {}),
    // The API binds 127.0.0.1 and has no CORS middleware by design; proxying
    // keeps dev same-origin so the browser never needs one.
    proxy: {
      '/v1': {
        target: apiTarget,
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
