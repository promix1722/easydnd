# Developing, caching and shipping

Part of the [web client documentation](../web.md).

## One dev server per worktree

The ports above are what an unclaimed worktree uses. Once one claims a slot
(see [backend.md](../backend.md#running-more-than-one-worktree)), `make web/dev`
passes three variables that `vite.config.ts` reads, all defaulting to the
values quoted above:

| | |
| --- | --- |
| `EASYDND_WEB_PORT` | what Vite binds, on `127.0.0.1` |
| `EASYDND_API_ORIGIN` | where `/v1` is proxied -- this worktree's API, not `:8080` |
| `EASYDND_WEB_PUBLIC_URL` | where a *browser* reaches this dev server, when a proxy is in front |

The last one is the interesting one, and it exists because a proxy makes "the
port Vite binds" and "the port the browser dials" two different numbers. Two
settings have to be told:

- **`server.allowedHosts`.** Vite refuses a `Host` header it does not
  recognise, and a proxy that preserves the browser's `Host` -- as it must, or
  the origin the API sees would be wrong -- forwards a name Vite has never
  heard of. Without the public hostname listed, every request comes back
  *"Blocked request. This host is not allowed."*
- **`server.hmr.clientPort`.** The HMR WebSocket is dialled *by the browser*,
  so it would otherwise be pointed at a port only this machine can reach and
  the console would loop `[vite] failed to connect to websocket`. The app is
  unaffected when that happens -- module loads and `/v1` are ordinary HTTP --
  but edits stop appearing without a reload.

`EASYDND_WEB_PUBLIC_URL` must also match an entry in the API's
`auth.rp_origins` byte for byte, trailing slash and port included, or
`middleware.SameOrigin` rejects every POST. `make dev` derives both from the
same place, which is the point.

`server.strictPort` is on: a busy port fails instead of sliding to the next
one. Three other things name this port -- the proxy in front, `auth.rp_origins`
and the neighbouring worktree that must not be handed it -- so a silent drift
would not surface as "port busy" but as "request origin is not allowed" on
every write.

One consequence of reaching the client over plain HTTP on a name that is not
`localhost`: it is not a secure context, so `window.PublicKeyCredential` is
undefined, `lib/webauthn/support.ts` reports passkeys unavailable and the
sign-in screen draws no passkey card. That is the designed degradation, not a
bug -- the guest session is the way in, and `lib/api/client.ts` already falls
back off `crypto.randomUUID` so `X-Request-Id` is still sent.

### `make preview` is the only secure origin

The dev server cannot show you the PWA at all, for two independent reasons.
Its origin is not secure, as above; and it has no service worker, because
`devOptions.enabled` is false in `vite.config.ts` -- a worker there would
shadow the module graph and serve stale chunks after every edit. So
`beforeinstallprompt` never fires, `ui/InstallAction` never draws, the update
dialog never triggers and passkeys are unavailable.

`make preview` answers all of it by being a different thing rather than a
better dev server:

- It serves **the built bundle**, not a development approximation of it --
  `make web/build` first, then the real `web/dist` with its real service
  worker and precache. No Vite, and so **no HMR**: restart to rebuild.
- **The Go binary serves both halves.** `-web web/dist` puts the bundle behind
  the same process that answers `/v1` (`internal/api/http/static.go`), so there
  is one origin and no proxy between them. The flag is development-only and
  unset in production, where nginx serves the bundle and owns the caching
  policy this deliberately does not reproduce -- except for one rule it cannot
  do without. Served with a `Last-Modified` and no `Cache-Control`, `sw.js` is
  open to *heuristic* freshness, and a browser reusing it means
  `registration.update()` installs nothing, the update dialog's reload has no
  new worker to wait for, and the old worker answers the navigation from its own
  precache: the same page, and the dialog a second time. So `staticSite` says
  `no-cache` on everything and `immutable` on the content-hashed names, which is
  the shape of the nginx blocks for `/sw.js` and `/assets/`. Its SPA fallback
  stops at
  `/assets/`, which nginx answers `=404`: those names carry a content hash, so a
  request for one that is gone is a page built against an older bundle, and
  answering it with `index.html` gives a module script an HTML body and a MIME
  type error that names nothing that went wrong. Restarting a preview rebuilds
  the bundle under any tab still holding the last one, so this is the failure a
  preview hits most.
- It listens on a **fixed 8090**, reached at `https://hton.cloud:8890`, where
  nginx terminates TLS with the real certificate the host already has. That is
  what makes it a secure context.

Both of the update path's real bugs were found here rather than in production,
which is the argument for having it: the browser reusing a cached `sw.js`, and
the reload firing before the new worker had installed. See
[Why registerType is 'prompt'](#why-registertype-is-prompt).

The nginx side is not in this repo -- it is the machine's, shared with whatever
else runs on that host -- and it is one server block in
`/etc/nginx/conf.d/z-dev-ports.conf` proxying `8890` to `127.0.0.1:8090`.

**One port means one preview at a time**, across every worktree. That is the
trade for not spending ten certificates and ten ports on something used to
check a release rather than to work in.

## Offering the install, and clearing the notch

An installable app that says so to nobody is not offering much. On
HTTPS Chrome offers its own omnibox install icon regardless; on iOS nothing
appears at all, because iOS has no install API and never has. `ui/InstallAction`
is the offer, and `lib/install` is the one bit of state behind it.

**A button, not a banner**, and that is [web.dev's guidance][promote] rather
than taste: *"Don't show banners on initial page load or out of context"*, and
*"keep promotions outside of the flow of your user journeys"*. It also settles a
question this client would otherwise have had to answer -- a button has nothing
to dismiss, so nothing has to be remembered.

It renders `null` unless there is something to offer, so an installed app and a
browser that cannot install both get the chrome exactly as it was.

**It is a glyph in the header, left of the language**, not a control floating
`position: fixed` in a corner of the page. That would sit on top of the page's
own content at every width -- over the foot of a table, over the landing
footer -- which is worse than being one more glyph in the corner the rest of
the chrome already shares. `shell/AccountActions` draws it for the two
signed-in chromes and `shell/SignInActions` for the landing one, in both cases
immediately before the language.

**Icon only**, for the same reason the language and the way out are icons: on a
390px row the word "Install" is width this offer has not earned. It survives as
the control's `aria-label` and as its tooltip, so nothing is lost to a screen
reader.

Three answers, kept as a string because `useSyncExternalStore` compares
snapshots by identity and an object rebuilt per call re-renders for ever:

| | |
| --- | --- |
| `'none'` | already standalone, or nothing on offer |
| `'prompt'` | Chrome fired `beforeinstallprompt` and we kept the event |
| `'ios'` | an iOS device, where there is no event to keep |

`lib/install/state.ts` registers its listeners **at import time**, unlike
`lib/version`, whose store is only ever written by an explicit call.
`beforeinstallprompt` fires early and is never replayed, so a listener attached
after React mounts has already missed it. It is also `preventDefault`ed, or the
viewport carries two offers of the same thing. The event is single use:
`prompt()` must come from a user gesture and cannot be called twice, so the
button calls `install()` directly and the offer drops to `'none'` afterwards
either way.

**iOS gets the same button and a different thing behind it.** There is nothing
to call, so the button opens a sheet naming the two taps -- Share, then Add to
Home Screen. Not a Safari check: since iOS 16.4 those same taps install from
Chrome, Edge and Firefox, so "is this iOS" is the whole question, and the
iPadOS-13-and-later case that reports itself as a Macintosh is the only wrinkle.

### The notch is the other half of viewport-fit=cover

`index.html` says `viewport-fit=cover`, which tells iOS to hand the page the
whole display -- including the strip under the status bar and the one under the
home indicator. Without the matching half, `env(safe-area-inset-*)`, the header
simply sits beneath the notch. In a browser tab Safari's own chrome covers it
and nothing looks wrong; installed, it is the first thing anybody sees.

`shell/chrome.ts` carries the tokens and the three shells apply them, the way
`HEADER_HEIGHT` already solves the same class of problem. `HEADER_BOX` grows the bar
upward so it paints behind the status bar, while `paddingTop: SAFE_TOP` on the
same element keeps the row of controls where it was. The `0px` fallbacks in
those `env()` calls are load-bearing: an `env()` a browser does not know is
invalid, and an invalid value inside `calc()` poisons the declaration, so the
header would lose its height rather than gain nothing.

**None of this can be exercised in `make web/dev`.** `beforeinstallprompt` needs
a registered service worker and a secure context, and the dev server has neither
-- see [`make preview` is the only secure origin](#make-preview-is-the-only-secure-origin).
`make preview`, or a production build reached over `localhost`, is the only way
to see the button; the notch needs a real device, because Chrome's
device emulation does not simulate the insets.

[promote]: https://web.dev/articles/promote-install

## Two caches decide what a returning visitor sees

Neither of them is the browser's, and the whole of this section is about not
being surprised by that.

A build produces a service worker. `vite-plugin-pwa` generates it from the
config in `web/vite.config.ts`; there is no `sw.js` in the repo and no
`manifest.webmanifest` either. It precaches the whole bundle, `index.html`
included, and answers every navigation out of that precache through a Workbox
`NavigationRoute`. So for a returning visitor -- and for every installed app --
**nginx's `no-cache` on `/index.html` decides nothing.** It applies to a first
visit and to the worker's own update fetches, and that is all.

That has one consequence worth stating on its own, because it is the difference
between this working and appearing to work: **`location.reload()` does not
reload onto a new release.** It is answered from the precache with the page it
was already showing. Getting past that is what `src/lib/version/reload.ts` is
for.

### The dialog is blocking, and that is the design

`src/lib/version` watches for a deploy in production builds. Vite development
mode (`import.meta.env.DEV`) disables both API-header detection and visibility
checks, regardless of `VITE_APP_VERSION`. `make web/dev` supplies a commit hash
at startup; rebuilding the API at a later commit does not change that value,
and reloading the tab cannot reconcile the two. Comparing hashes in development
would therefore cause an endless blocking reload prompt. HMR handles development
updates; commit hashes remain visible for diagnostics. Tests exercise mismatched
hashes in both modes so production checks remain active.

Two signals feed the production watch:

- **Every API response carries `X-App-Version`.** `lib/api/client.ts` compares
  it against `WEB_VERSION` at the one point every request passes through, so any
  request the app was going to make anyway is the check. No interval, no traffic
  that exists only to ask. It is read before the ok/not-ok branch, because a
  client running against a newer API is exactly the one whose requests start
  failing.
- **An explicit check when the tab becomes visible, or the network returns.**
  The first signal can never fire for a tab nobody is touching, and that is not
  an edge case: a desktop tab left open overnight, a mobile tab the OS froze and
  thawed a day later, an installed app resumed from the switcher. These are the
  app's only lifecycle listeners.

The answer latches. Once a newer release is known, this tab stays stale until it
reloads -- a response held in an HTTP cache can name a release that stopped
being deployed some time ago, and unlatching on one of those would dismiss a
dialog somebody was reading.

`ui/UpdateRequired` then blocks the app until it is reloaded. No dismiss, no
"later". A dismissible banner leaves someone talking to a newer API with older
code, which is the failure the whole mechanism exists to prevent, and the
failure is quiet: requests succeed until one does not, and the one that does not
is usually a save. Between interrupting someone and losing their character
sheet, this interrupts.

There is no exemption for any route, and the cost is worth naming. If nginx ever
serves a stale `index.html`, reloading does not fix it and the dialog has no way
out -- the app is unusable until the server is. There is no page left to reach
that shows the two versions disagreeing; diagnose that case with `curl https://easydnd.org/version.json` against
`/v1/version`, which is what the deploy pipeline does anyway.

### Why registerType is 'prompt'

`'autoUpdate'` is half a mechanism rather than a choice. It sets `skipWaiting`
and `clientsClaim` in the generated worker, but nothing here imports
`virtual:pwa-register`, so `injectRegister` falls back to `'script'` and the
emitted `registerSW.js` is a bare `register()` call with no update listener in
it. A deploy then does this: the new worker installs, skips waiting, claims
tabs that are still running the previous release's JavaScript, and
`cleanupOutdatedCaches()` deletes the precache holding the chunks those tabs
will ask for next. Nothing reloads them.

`'prompt'` leaves `skipWaiting` off, so a new worker waits instead of seizing
live tabs, and Workbox's template emits a `message` listener for
`{type: 'SKIP_WAITING'}`. `reload.ts` calls `registration.update()`, sends that
message to the waiting worker, and reloads on `controllerchange` -- with a
five-second fallback, because the button has to do something even if the worker
is wedged.

**`update()` resolving is not the install finishing.** The promise settles once
the script has been fetched and the install job is running: `registration.waiting`
is empty and `registration.installing` is the worker that matters. Reading only
`waiting` falls through to the plain reload milliseconds after the new worker
began precaching, and the old worker, still in control, answers the navigation
from its own precache with the same page the dialog was complaining about --
a button that needs pressing twice. So `reload.ts` waits for an installing
worker to reach `installed` before sending `SKIP_WAITING`, and treats
`redundant`, `activating` and `activated` as "nothing to skip" -- a plain reload,
which is right for each of them. `reload.test.ts` pins the ordering.

The preview server needs one thing of its own here: both `http.FileServer` and
`http.ServeFile` answer `/index.html` with a 301 to `./`, so every install would
spend a redirect on the worker's precache fetch and store a response marked
`redirected`, which a browser may refuse to hand to a navigation. nginx serves
the file, so `internal/api/http/static.go` opens it and hands it to
`ServeContent` instead.

`reload.ts` is written against `navigator.serviceWorker` rather than
`virtual:pwa-register` so that it stays ordinary TypeScript: the virtual module
resolves only through the plugin, which would make it a build-time dependency of
every test that touches the file.

The worker is disabled in `make web/dev` (`devOptions.enabled: false`) -- it
would otherwise shadow the dev server's module graph and serve stale chunks
after every edit. So **none of this runs in the dev server**, and the watch
ignores a development build outright -- `import.meta.env.DEV`, and a bundle
built with no version, which reports `WEB_VERSION === 'dev'`.

### One caching rule, applied twice

Stated once here and enforced in `deploy/nginx/easydnd.conf`:

> A URL whose contents can change is never cached without revalidation. A URL
> whose contents can never change is cached forever. There is no middle.

| Served | Policy | Why |
|---|---|---|
| `/assets/*` | `immutable`, one year | Vite content-hashes them; the URL cannot change meaning |
| `/workbox-<hash>.js` and its map | `immutable`, one year | Hashed too, but emitted at the bundle root, so the rule above does not reach it |
| `/icons/*`, `/favicon.svg` | `no-cache` | Fixed filenames, bytes generated from the palette |
| `index.html`, `version.json`, `sw.js`, `registerSW.js`, `manifest.webmanifest` | `no-cache` | Stable URLs whose contents change every release |
| `/v1/version` | `no-store`, set by Go | Answers "which release is live"; an answer that can be held is not one |

The icons are in the `no-cache` row because their filenames never change: on a
long `max-age` a palette change is invisible to a returning browser for as long
as it lasts. Installed clients never show the problem, because Workbox refetches
every revisioned precache entry with `cache: 'reload'` -- which is precisely how
it goes unnoticed.

`manifest.webmanifest` has a rule of its own because stock nginx `mime.types`
has no `webmanifest` entry and would serve it as `application/octet-stream`.

## Screens are fetched on first visit

`routes/index.tsx` reaches every screen behind sign-in through `React.lazy`,
so the entry chunk is the shell, the landing page, the sign-in screen and the
design system, and each screen is a chunk of its own. Otherwise a signed-out
visitor downloads the builder, the tracker, the pack editor and the admin
tables to read a carousel: about 545 kB gzipped up front, against about 300 kB.

Three things hold it together:

- **Each import names the screen's file, not its feature's barrel.** A barrel
  re-exports every screen of its feature, and a module that is imported both
  statically and dynamically stays where the static import puts it. `HomeRoute`
  fetches the character list the same way for the same reason: it is the one
  route the landing page shares.
- **The `Suspense` boundary is `shell/RouteOutlet.tsx`**, inside the chrome, so
  a deep link shows the header and a loader rather than a blank window. A
  navigation between screens never shows the fallback: the router makes it a
  transition and the page being left stays up.
- **A chunk that fails to load reloads the tab, once.** The usual cause is a
  release that went out while the tab was open, and `main.tsx` answers Vite's
  `vite:preloadError` with the same reload the update dialog performs. Installed
  clients rarely get that far: the service worker precaches every chunk but the
  die's.

Both locale catalogues still ship in the entry chunk. Fetching the inactive
one on switch would save about 25 kB gzipped and would make the first render
of a Russian visit wait on a request; it has not been worth that.

## How it ships

The frontend is not deployed on its own: a tag push builds a `web.tar.gz` that
travels in the same release directory as the binary and the SRD data, and nginx
serves `/opt/easydnd/current/web` behind an SPA fallback. Because all three sit
behind one symlink they swap together, so a rollback reverts the UI, the API
and its data as a unit. The full pipeline is in
[backend.md](../backend.md#deployment).

Two things about that are the frontend's to keep working:

1. **The build must be given `VITE_APP_VERSION`.** It writes
   `dist/version.json`, which the deploy workflow reads through the public URL
   to prove nginx is serving this release rather than a cached `index.html`. An
   unset variable would be a silent no-op, so `web/vite.config.ts` refuses to
   build without it. The value is a tag on a release and a short commit SHA
   anywhere else, decided by `deploy/release-version.sh` and passed in by
   `make web/build` -- and by `make web/dev`, so a dev session reports its
   commit rather than the word "dev". Production bundles must match the binary;
   development mode allows the two processes to restart independently without
   a release dialog. See
   [backend.md](../backend.md#what-a-release-is-called-and-where-it-lives).
2. **A bad bundle goes live silently.** The API-side health gate cannot see the
   frontend, so `deploy.sh` checks the bundle exists *before* the symlink swap.
   Without that, a bundle that unpacked badly would go live as a blank site and
   would not roll back.

[mantine]: https://mantine.dev

## Analytics

PostHog is optional. In its installation screen, expand **Need to set up
manually?**, choose React, and copy the browser **project token** (`phc_`).
Enter it yourself in the existing env files, outside the repository:

| Environment | File | Loaded by |
|---|---|---|
| Development | `~/config/easydnd/dev.env` | Make; shared across worktrees |
| Production | `/etc/easydnd/prod.env` | Supervisor |

```sh
EASYDND_POSTHOG_TOKEN="phc_your_project_token"
```

Do not commit the token. The browser receives it at runtime, so this keeps it
out of Git rather than making it a browser secret. Never use a personal API key.
An unset or empty variable leaves analytics disabled with the committed configs.
The HTTPS ingestion host stays in `analytics.host` in `config.dev.yaml` and
`config.prod.yaml`; both currently use `https://eu.i.posthog.com`.

No `web/.env.local`, `VITE_POSTHOG_*`, or GitHub build variables are needed.
The development path is `~/config`, not `~/.config`. Keep `dev.env` mode 600
and `prod.env` mode 640 with owner `root:easydnd`, as for the existing secrets.
After editing `prod.env`, run `sudo supervisorctl restart easydnd`; for local
development, restart the API through its Make target so it reloads `dev.env`.

Restart the development API after changing its YAML and reload the browser.
Production configuration ships through the normal release process. The browser
fetches `/v1/analytics-config` once, then loads `posthog-js` asynchronously. The
page stays usable if the API, SDK, or tracker is blocked. There is no startup
event queue or retry loop; actions before initialization may be missed.

Every event carries `environment` from the API's `env`, `app_version` from
`WEB_VERSION`, and `account_type` (`visitor`, `guest`, or `registered`). A built
bundle served by `make preview` therefore stays tagged `development`. One
PostHog project receives both environments: filter production reports with
`environment = production`, and use a development-filtered view for testing.

Tracked events:

| Event | Trigger |
|---|---|
| `$pageview` | Initial page after auth resolves, and navigation to a different path |
| `signed_in` | Successful passkey/guest sign-in or a pending Google/development sign-in confirmed after redirect |
| `character_created` | Successful manual character creation API call, before finishing the builder |
| `group_joined` | Successful invite acceptance |
| `game_created` | Successful game creation |

Restoring an existing session does not generate another sign-in. Identified
users use `<environment>:<account-id>`; logout, session expiry, and account
switches reset identity. SDK persistence is scoped to the browser tab to match
development's independent tab accounts. Registered users reconnect to their
stable identity on subsequent visits; anonymous/guest retention across closed
tabs is not measured.

Automatic click capture, session replay, surveys, exceptions, and performance
capture are off. Outbound event properties are allowlisted, including SDK-added
properties: raw URLs, query strings, fragments, referrers, person traits, names,
emails, character content, and AI conversations are excluded. Page URLs use
router templates, such as `/characters/:id`; unknown paths become `/*`.

To verify, use an ordinary browser to open the development site, sign in, create
a character, and navigate. PostHog's live events should show `development`, the
app version, and the actions above, without private URL values. Its installation
screen should then detect events. Automated tests use a mocked SDK and never
send ingestion requests. Keep the free plan and billing disabled; development
traffic shares the project's monthly event allowance.

Suggested production reports are daily distinct active users, a
`$pageview → signed_in → character_created` activation funnel, and weekly
retention from `character_created` to `$pageview`. The activation funnel covers
manual creation; AI imports and character copies are not creation events in
this first integration.
