import { fileURLToPath } from 'node:url'
import { defineConfig, type Plugin } from 'vitest/config'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'

import { PALETTE } from './src/theme/tokens.ts'

/**
 * Fills index.html's `theme-color` from the palette, at dev and at build.
 *
 * The browser paints its own chrome from that meta tag before a line of React
 * runs, so it cannot read a token the way a component does -- which is how it
 * came to hold `#7a1f2b`, a colour that matched neither the mark nor anything
 * else in the product. A rewrite at serve time is what lets the tag stay a
 * literal in a hand-edited file while still having exactly one source.
 *
 * It throws rather than passing the HTML through untouched: a rewrite that
 * quietly finds nothing to rewrite is not a gate, it is a hope.
 */
function themeColour(): Plugin {
  return {
    name: 'easydnd:theme-colour',
    transformIndexHtml(html) {
      // Presence, not difference. `String.replace` hands back an identical
      // string when the value it wrote is the value already there, so asking
      // "did anything change?" reports the correct case as the missing one --
      // which is exactly how this failed the first time it ran.
      const tag = /(<meta name="theme-color" content=")[^"]*(")/
      if (!tag.test(html)) {
        throw new Error('index.html has no theme-color meta tag for the palette to fill')
      }
      return html.replace(tag, `$1${PALETTE.brand}$2`)
    },
  }
}

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
    themeColour(),
    versionManifest(),
    VitePWA({
      /**
       * 'prompt', not 'autoUpdate', and the difference is not a preference.
       *
       * 'autoUpdate' sets skipWaiting and clientsClaim in the generated worker.
       * That was half a mechanism: nothing here imports `virtual:pwa-register`,
       * so injectRegister fell back to 'script' and the emitted registerSW.js
       * was a bare register() call with no update listener in it. A deploy then
       * did this -- the new worker installed, skipped waiting, claimed tabs
       * that were still running the previous release's JavaScript, and
       * cleanupOutdatedCaches deleted the precache holding the chunks those
       * tabs would ask for next. Nothing reloaded them.
       *
       * 'prompt' leaves skipWaiting off, so a new worker waits instead of
       * seizing live tabs, and workbox's template then emits a `message`
       * listener for {type: 'SKIP_WAITING'}. src/lib/version/reload.ts sends that
       * message when someone presses the button in the update dialog, which is
       * what makes the reload that follows land on the new release. See
       * docs/web.md, "Two caches decide what a returning visitor sees".
       */
      registerType: 'prompt',
      // The service worker would otherwise shadow the dev server's module
      // graph and serve stale chunks after every edit.
      devOptions: { enabled: false },
      includeAssets: ['favicon.svg', 'icons/apple-touch-icon.png'],
      workbox: {
        globPatterns: ['**/*.{js,css,html,svg,png,woff2}'],
        // Navigations to /v1/ are the API's, not the router's. This is load
        // bearing rather than tidy: the Google sign-in return is a top-level
        // navigation to /v1/auth/sso/:provider/callback, and without this the
        // worker would answer it with index.html and sign-in would break for
        // every installed client.
        //
        // version.json needs no rule here. It is not precached -- `json` is
        // absent from globPatterns above -- and no runtime caching route
        // exists, so it always reaches the network.
        navigateFallbackDenylist: [/^\/v1\//],
      },
      manifest: {
        // i18n-exempt (whole block): the manifest is built once and read by the
        // OS before a line of JS runs, so it cannot be swapped per locale by
        // anything in src/. `easydnd.org` is the product's name rather than a
        // word, and is never translated -- see docs/web.md, "Text that is
        // never translated".
        name: 'easydnd.org - D&D character and battle tracker',
        // Android truncates the home-screen label at roughly 12 characters,
        // which this just fits.
        short_name: 'easydnd.org',
        description: 'Create characters, level them up, and run encounters.',
        theme_color: PALETTE.brand,
        background_color: PALETTE.dark.background,
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
      // The message catalogues, deliberately outside src/. They are content,
      // not code: the whole point of keeping every caption in them is that
      // translating this app never means opening a component, and a directory
      // sitting among the layers would say the opposite.
      '@locales': fileURLToPath(new URL('./locales', import.meta.url)),
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
  /**
   * The test suite, in one project that isolates nothing.
   *
   * `isolate: false` is what makes it fast, and it is worth a lot. With
   * isolation on, vitest forks a process per test file and each one rebuilds
   * the whole Mantine + embla + React module graph and its own jsdom -- across
   * 48 files that was 36s spent on imports and 78s on constructing jsdoms, out
   * of 229s total. Sharing both took the run to 64s without changing a single
   * assertion. The pool is left at the default `forks` deliberately: `threads`
   * measured no better, and the worker count is left alone too -- vitest takes
   * `availableParallelism - 1`, which is the right answer on every machine this
   * runs on.
   *
   * What it costs is the guarantee that a file starts from nothing, and two
   * things follow from that. Both are load-bearing:
   *
   *   - src/test/setup.ts must reset anything a file can leave in module state.
   *     Its afterEach is that list.
   *   - **`vi.mock` cannot be used, by anybody.** One shared registry means
   *     whichever file loads a module first decides what every later file gets,
   *     so a mock registered by the second file arrives too late. It does not
   *     fail loudly: the test gets the real module and something else breaks,
   *     in whatever order the files happened to run. A component that needs a
   *     dependency swapped takes it as a prop instead -- InviteSheet's
   *     `copyLink` is the worked example -- and `npm run lint:layers` fails on
   *     any vi.mock in src/.
   *
   * There used to be a second, isolated project for the one file that mocked.
   * BaseSequencer runs projects in name order and isolated ones first, so that
   * single file was a serial prefix on every run: 52 files took 13.8s and all
   * 53 took 16.3s. One optional prop bought that 2.4s back.
   */
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    isolate: false,
    // No `css: true`: nothing in the suite reads a cascaded style. The only
    // style assertions are on inline `element.style` -- DragonMark's width and
    // the carousel's custom properties, both written by JS -- and Mantine emits
    // its class names whether or not a stylesheet was ever parsed. Running
    // @mantine/core's CSS through PostCSS and into every jsdom bought nothing.
  },
})
