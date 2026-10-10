# Frontend

The engineering doc for the [easydnd.org](https://easydnd.org) browser client:
layout, layer rules, and how it ships. For the Go API it talks to, see
[backend.md](backend.md); for the game model behind both, see [dnd.md](dnd.md).

Status: **real**. Sign-in (passkeys and Google), the character list with its
folders, character creation, the build loop, level-up, the account screen and
the sheet are all built and tested -- see
[Level-up is the desired level](#level-up-is-the-desired-level). The battle
tracker is not; `/games` is the roster a game is run from -- see
[Games are a section, not a corner of a group](#games-are-a-section-not-a-corner-of-a-group).

Character creation asks for a class's starting kit slot by slot -- body, main
hand, off hand, a backup weapon, the pack -- including category menus and
complete bundles. The server expands category membership;
the client preserves its option keys and order. See
[Builder choice behavior](#builder-choice-behavior) for the interaction contract.
Ability scores remain the six standard characteristics, including with addons.

## Pack transition

Build edits, deletes and level-up requests carry the server's `revision`
as `expectedRevision`, alongside their positional `expectedSeq`. This closes
the same-length-edit concurrency gap. Legacy test/API fixtures without a
revision fall back to their sequence.

The ability editor still uses the base-game presentation. Resources do not: the sheet and the game page read
the generic `resources.pools`/`parameters` and take their names from the pack,
so the client holds no table of resource labels and a homebrew pool shows up
without a client change. `packActions` is still unread. See
[packs.md](packs.md) for the read/write contract.

## Quick start

```sh
make web/deps                       # once, per worktree -- node_modules is not shared
make web/dev                        # http://127.0.0.1:5173, proxies /v1 to :8080
make web/check                      # typecheck, lint, layer-check, tests -- mirrors CI
make web/icons                      # after changing PALETTE_NAME or the mark
```

`make web/dev` proxies `/v1` to the API, so run `make run/server` alongside it
-- or `make dev` at the repo root, which starts both and a Postgres. `make
verify` at the repo root runs the frontend checks and the Go ones together, and
runs them **at the same time**: `web/test` is by far the longest thing in it, so
it is started first and the Go side happens inside its shadow. See
[backend.md](backend.md#tests).

## The test suite does not isolate test files

`vite.config.ts` runs the suite with `isolate: false`, so the test files a
worker picks up share **one** module registry and **one** DOM rather than
forking a fresh pair per file. Rebuilding the Mantine, embla and React module
graph 48 times over cost 36s of imports and 78s of DOM construction out of
229s total; sharing both took the run to 64s without a single assertion
changing. Passing `delay: null` to user-event (see `src/test/user.ts`) took it
to 38s from there, and the two rules below took it to 24s.

The DOM is **happy-dom**, not jsdom. At a hundred files the suite had grown to
186s of test time -- nearly all of it React rendering into the DOM -- which on
the three workers vitest forks on a four-core machine is 66s of wall. happy-dom
renders the same trees in half the time: 92s of test time, about 40s of wall.
It cost one test-side change and no production code: a storage spy targets
the `sessionStorage` instance rather than `Storage.prototype`, because
happy-dom's storage is a proxy a call never routes through the prototype of.
Neither engine computes layout
or evaluates `@media`, so everything below that says the DOM cannot measure
something is true of both.

Nothing sets the worker count. vitest's own default is the right answer on
every machine this runs on, and a number written down here would be wrong on
the next one.

What it costs is the guarantee that a test file starts from nothing, and two
things follow from that.

**`src/test/setup.ts` is what keeps the suite honest.** Its `afterEach`
unmounts the tree, resets the viewport, clears stubbed globals and empties the
catalogue request cache -- the only module-level mutable state in `src/`. If
you add another piece, reset it there in the same change. A test that passes
because of what ran before it is worse than one that fails. It is also the only
place those resets belong: a file that repeats one in its own hook is not safer,
only harder to read.

**`vi.mock` is not available, to anybody.** A shared registry means whichever
file loads a module first decides what every later file sees, so a mock
registered by the second file arrives too late. It does not fail loudly: the
test gets the real module, and the assertion breaks somewhere else, in whatever
order the files happened to run in.

**A component that needs a dependency swapped takes it as a prop.**
`InviteSheet` is the worked example. It must show an error when the clipboard
cannot be reached -- the bug it exists for -- so it accepts an optional
`copyLink` that defaults to the real `copyText`, and its test hands over a
`vi.fn()` that resolves `false`. No global is touched and nothing leaks into the
next file in the worker. `npm run lint:layers` fails on any `vi.mock` under
`src/`, and says this.

## Three rules about writing a test here

**A test runs at one viewport unless the tree branches on width.** The
components that call `useIsDesktop` do: in `ui/`, `DataList`, `ModalSheet`,
`TabDeck`, `ChoiceDetails` and -- for one style property, see below -- `TabRow`;
`RootShell`; and a handful of feature components (`Inventory`,
`SpellChoiceRow`, `SpellTags`, `GameTracker`, the landing carousel).

"Make the theme responsive" is the obvious wrong turn. Anything that must
differ by width and is not a *layout* belongs in `ui/app.css`, so nothing
branches in JavaScript and nothing re-renders at the breakpoint. It also means
no test here can assert a rendered size: the suite parses no CSS and the DOM
evaluates no `@media`.

`ui/Page` is deliberately **not** one of them, and its own test proves it
rather than asserting it in prose: the last case there compares the two
renderings byte for byte. The cheapest way to "fix" a future layout problem in
a shared page component would be to reach for `useIsDesktop`, and that test is
what goes red when somebody does. Nothing else can, because the suite runs
without CSS -- a `SimpleGrid cols={{ base: 2, sm: 3 }}` renders one DOM
whatever the width is -- so `describe.each(['mobile','desktop'])` around a tree
that reaches none of those components is the same assertion run twice against
byte-identical markup. `src/ui/TabRow.test.tsx` makes the same comparison: its
last test shows the two renderings equal but for the list's `flex-wrap`.

Where a block runs at one width, the comment above it names this rule, so the
next reader knows it was a decision. Where a block still runs at both -- the
group screens, `ModalSheet` itself, the rows of `CharacterListScreen`, and
`SheetBody`, whose deck draws a different tree at each width -- it is because
the swap is what the test is about.

The criterion is what the test *presses*, not what screen it is on.
`CharacterListScreen`'s row actions live inside `DataList`, so a test that
presses one belongs at both widths; the tests that press a folder's heading,
its action menu or **New folder** do not -- what they assert is a request body
or where a press navigated to, which is the same markup either way. They run at
one width. The folder dialogs are `ModalSheet`s and that swap is asserted on
its own terms in `src/ui/ModalSheet.test.tsx`, once, rather than several more
times here.

**Render the panel, not the page, when the panel is what is under test.**
`ProficienciesPanel.test.tsx`, `Vitals.test.tsx`, `SheetBody.test.tsx` and the
skills-panel block of `CharacterSheetScreen.test.tsx` mount their component
directly rather than the whole sheet, and keep one full-sheet test to hold the
seam. `SheetBody.test.tsx` is the newest of them and the clearest case: it takes
a projection and a compendium as props, so the phone's deck is tested without a
mocked fetch, a router or a single `findBy` -- and what is left in
`CharacterSheetScreen.test.tsx` is the seam, which is that the screen fetches
those two things and hands them on. A sheet test costs
about twice a panel test, because most of the sheet's weight is the eighteen
`ProficiencyMark`s -- each a Mantine `Tooltip`, each a floating-ui hook stack --
and mounting the page to read one panel pays for the other seventeen sections
too.

Where several read-only assertions share one fixture, one test that renders
once and asserts many times beats several that each re-mount it. Use
`expect.soft` when merging, so the merged test still reports every failure
instead of stopping at the first; a shared `beforeAll` render is not available,
because `src/test/setup.ts`'s global `afterEach` unmounts between tests and
exempting a file from that would give up the isolation rule above.

**Hand a component the state, rather than driving it there, when the driving
is not what is being tested.** `AbilityScoresForm.test.tsx` is the case: four
point-buy tests each spent two clicks and a Mantine `Combobox` dropdown moving
the method `Select` to a state the component takes as a prop, and one of them
clicked `Raise` twenty-one times to reach a spread it also takes as a prop.
`AbilityScoresForm` seeds its budget from `boughtFrom(method, scores)`, which is
the same value `change('point-buy')` sets, so the two are the same state. One
test still drives the `Select`, because that transition is what *it* is about;
the rest are handed `method="point-buy"`. The rule is not "avoid interactions"
-- it is that an interaction should be either the subject of the test or absent
from it.

The suite also does not process CSS (`css: true` is off). Nothing asserts on a
cascaded style -- the only style assertions read inline `element.style`, which
components and Mantine write from JS -- and the class names the tests query on
are emitted whether or not a stylesheet was ever parsed.

## One dev server per worktree

The ports above are what an unclaimed worktree uses. Once one claims a slot
(see [backend.md](backend.md#running-more-than-one-worktree)), `make web/dev`
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

## Layout

A single responsive SPA in `web/`, served by nginx from the same release
directory as the binary. React 19 + TypeScript + Vite, with
[Mantine][mantine] as the component library and a PWA manifest so it installs
to a phone home screen. It also ships a service worker, which matters more than
it sounds -- see
[Two caches decide what a returning visitor sees](#two-caches-decide-what-a-returning-visitor-sees).

```
web/locales/  en.json and ru.json -- every user-facing word in the client
web/src/
  theme/      framework-free design tokens (breakpoints, the palette, the content cap)
  ui/         the design system -- the only place Mantine is imported, and the section table
  lib/        API client, data hooks, WebAuthn plumbing, the auth context and i18n; no UI
  shell/      the chrome: RootGate picks it, RootShell picks the viewport
  features/   screens -- one directory per aggregate (characters/, groups/, ...)
  routes/     the route table, one tree for both viewports
  domain/     pure display helpers; the Go model in dnd.md owns the rules
```

`locales/` sits outside `src/` deliberately. It is content, not code -- see
[Localization](#localization).

Net new dependencies for Google sign-in: **zero**. The whole flow is a
server-side redirect, so the button is a link and there is no Google
JavaScript on the page.

`domain/` holds nothing that computes a rule. Ability modifiers, proficiency
bonuses and armor class all arrive derived, because a second implementation in
the browser is a second implementation to disagree.

## Data, without a query library

`lib/useResource` fetches and `lib/useAction` mutates; there is no server-state
library behind either, and the reason is structural rather than a preference.
Every endpoint that writes to a character returns the freshly projected sheet,
so there is nothing to invalidate -- the response *is* the invalidation. A
query library earns its keep when many components read overlapping queries with
independent lifetimes, and here one screen owns one character.

The character list is the one screen that holds two resources -- its folders
and its characters. It still does not need a library. The two reads are independent and unkeyed: the
characters read fetches *every* character once and the screen groups them by
the folder each one carries, so a folder appearing or disappearing costs one
reload rather than a request per shelf. Every mutation on that screen is
followed by an explicit reload of the lists that changed, and there is no cache
to go stale in between. Revisit it when something needs a *third* component's
copy of the same data to update by itself.

The compendium is immutable for the life of the server process, so
`lib/api/catalog.ts` memoises each collection's *promise* for the session: two
components mounting in the same tick make one request, not two.

Net new dependencies for the whole character feature: zero.

The landing carousel is the exception that finally broke that run, and it is
recorded rather than slipped in: `@mantine/carousel`, `embla-carousel` and
`embla-carousel-react`, three packages bought for one page. Hand-rolling was the
default here and is wrong for this one thing -- a drag-and-snap carousel is
momentum physics, pointer capture, RTL and keyboard semantics, and "it is just a
flexbox with `scroll-snap`" stops being true the moment a thumb is involved. The
price is small and measured rather than assumed: +27 kB of JavaScript and +3 kB
of CSS minified, about +10 kB gzipped. The cost that outlives the decision is
the peer range -- `@mantine/carousel` pins `@mantine/core` to an *exact* version
rather than a range, so the four Mantine packages now move as a unit. A
lockfile diff that bumps `@mantine/core` is a UI-wide upgrade wearing a
carousel's clothes, and should be reviewed as one.

They serve two surfaces: the landing page, and `ui/TabDeck` -- which is
what the character sheet and the build screen become on a phone. That does not re-open the decision,
it settles it. The +10 kB is amortised over the page a visitor meets the app on
and the page they spend the most time on, and the second use is the one that
justifies a real carousel rather than a `scroll-snap` flexbox, because it is the
one a thumb is actually on at a table. The peer-range warning above is
unchanged, and is still the thing to look for in a lockfile diff.

## The die is real 3D, and it is paid for in one chunk

`ui/D20Scene.tsx` draws a genuine icosahedron with three.js and throws it
around a cannon-es physics world. It is by a wide margin the largest thing this
app depends on -- **158 kB gzipped**, against the 10 kB the landing carousel
cost, which had been the whole of this project's dependency budget.
That is a sixteen-fold jump and it was made deliberately, so the reasoning is
here rather than in a commit message.

**What was tried first, and why it was not enough.** The die began as twenty
`clip-path` triangles placed with `matrix3d` and tumbled with the Web
Animations API -- about half a kilobyte, no GPU context, and geometrically
correct: an icosahedron is convex, so back-face culling alone gives exact
occlusion with no depth sorting. It read as flat. Static per-face shading
cannot fake a specular highlight travelling across a facet, and that highlight
is most of what tells an eye it is looking at a solid rather than a hexagon
with lines on it. The approach was sound and the result was not, which is worth
recording because the arithmetic still says the cheap version should have
worked.

**What the size actually buys**, beyond looking right: real lighting and a
contact shadow, and -- the part that changed the interaction -- a die you throw
rather than a die you trigger. See below.

**The cost is contained rather than accepted.** Three separate mechanisms keep
three.js out of the main bundle, and all three are load-bearing:

1. `ui/D20.tsx` -- the half that always ships -- reaches the scene through a
   `lazy(() => import('./D20Scene'))` and mounts it on an **intersection**, not
   on mount. The die is the fourth panel of a carousel; embla mounts all four,
   but only the panel you have swiped to is on screen. Swipe to the die and it
   fetches. Never swipe and it never does.
2. `vite.config.ts` names that chunk in workbox's `globIgnores`. Without this
   the service worker would have precached it on first visit -- it globs
   `**/*.js` -- and every visitor would have downloaded three.js in the
   background regardless of step 1. This is the step that is easy to miss and
   silent when wrong. The chunk is simply the one the dynamic `import()`
   produces, `D20Scene-*.js`; no `manualChunks` rule names it, because under
   Rolldown a manual chunk also takes its modules' dependencies -- React moves
   into it and the page preloads three.js for everybody while the die goes on
   working. The `lazyScene` build plugin fails the build unless there is
   exactly one such chunk and nothing the entry loads reaches it.
3. `scripts/check-layers.mjs` lists `three` and `cannon-es` as `ui/`-only
   vendors, so a feature cannot import either directly and quietly undo the
   split.

**What that costs the visitor is a wait, so the wait is drawn.** The chunk is
158 kB over whatever connection a phone has, on the panel somebody swiped to
*because* they wanted the die, and an empty panel there reads as a panel with
nothing in it rather than as one still arriving -- a moment a development
machine, where the chunk comes from memory, never shows. So the `Suspense`
fallback is a centred `Loader`, named for a screen reader.

It sits under `Suspense` only, and deliberately not before the intersection: an
unswiped panel has not asked for anything, so a spinner there would claim work
that is not happening -- and would animate off screen for as long as the landing
page is open. Loading is announced by the spinner's own name rather than through
the polite live region below it, which carries results; a spinner interrupting
the number somebody just rolled is the wrong trade.

The measured result: `D20Scene-*.js` is 154 kB gzipped, is not in the precache
manifest, and is not among the chunks `index.html` loads. The build checks the
last of those; the precache is worth a look after a dependency bump.

The trade this makes is that a die thrown for the first time *offline* does not
work. That is the right way round: a visitor who never opens the die should not
have paid for it.

### The throw decides the number

Nothing is predetermined. The die is given the velocity your thumb gave it, the
simulation runs, and whichever face is up when it stops is the answer --
read back by `faceUp` in `ui/d20Geometry.ts`. A die that landed on a number
chosen before it left your hand is theatre, and a gentle flick that still spins
wildly to reach its target is theatre you can feel.

The honest consequence is that a practised thumb has some influence over the
result, exactly as it does with a real die on a real table. That is charming
for a toy and disqualifying for a roll that matters. **If this ever becomes the
app's actual roller** -- a d20 against a DC with a character sheet behind it --
the number has to come from `domain/dice.ts` and the physics be demoted to an
animation of a result already decided. The reduced-motion path is already that
shape, so the change has somewhere to start.

Two things the physics needs that are not obvious: the die is thrown into a
closed box of six invisible planes, because an enthusiastic thumb will
otherwise fling it out of frame; and a throw that has not come to rest within
four and a half seconds is placed flat and read, because a die wedged against a
wall would otherwise report a number no face is actually showing.

### The die is a page, not a dialog

`/roll` is a route, rendered by `features/dice/DiceScreen.tsx` inside the
ordinary phone chrome, and deliberately not a full-screen dialog. A dialog
covers the header, so the menu the die was opened from is unreachable until it
is dismissed; cut back to sit *below* the header it is half-drawn over the page
beneath and still carries a close button -- a second way out of a place you can
already leave through the menu.

A page needs none of it. The chrome is simply there, every section is one press
away, the back gesture takes you off the die rather than out of the app, and
leaving is navigating instead of dismissing.

It is a page but **not a section**. `/roll` is absent from `ui/sections.ts`, so
it owns no other paths, lights nothing in the navbar and starts no breadcrumb
-- and the desktop rail never offers it, because the section table is what both
shells map over and the die is a phone's. `shell/MobileShell.tsx` links to it
directly, below a divider, as the one entry in that menu that is not a section.

Being outside the table does not mean being nameless, though. The phone's
trigger is the only thing on screen saying where you are, and it takes its
label from `sectionFor` -- which answers nothing for `/roll`, so the control
would fall through to the word "Menu" on a page that this very menu links to
*by name*. So `MobileShell` holds a `DIE` constant shaped like a `Section`
(and an `ACCOUNT` beside it, for the same reason), and the trigger, the glyph
and the tick all read one `current`, with no second code path. The fallback
stays where it belongs: a 404 has genuinely no name to give.

The screen uses no `ui/Page`. That primitive exists to put a breadcrumb and a
title above a screen, and this screen wants neither -- see below. It sizes
itself to what the header leaves with the same `AppShell` custom properties
`routes/LandingPage.tsx` uses, including the same `calc(...)` wrapper, because
Mantine's `rem()` mangles a bare `max(`.

### You read the die, not a caption

The camera looks **straight down**, and there is no visible text anywhere in
the component -- no instruction, no printed result. Those two facts are one
decision: from directly above, the number that landed is simply the one facing
you, exactly as it is on a table, and a caption printing it again would be the
app reading the dice for you.

Three things follow, and each is easy to undo by accident:

- **The camera needs `camera.up`.** Looking straight down makes the up vector
  parallel to the view direction, and `lookAt` then has no way to resolve the
  roll -- the scene arrives empty rather than wrong, which is a confusing way
  to fail.
- **The key light is well off the camera axis.** Directly overhead it would
  put the die's shadow exactly underneath the die, where a top-down camera
  cannot see it -- and the shadow is most of what gives the die a floor to sit
  on rather than a hole to hover over.
- **The scene has no size of its own.** It measures its container with a
  `ResizeObserver` and builds the camera from the result, because both of its
  homes are a box somebody else decided the shape of. A fixed square floated
  small and off-centre in a tall carousel slide with the panel empty around it.
  The camera height is derived so that `VISIBLE_HALF` die radii fit across the
  panel's *narrow* side, which keeps the die the same apparent size whatever
  shape the panel is -- and the physics walls are then placed at exactly the
  edges of what that camera can see, inset by one radius, so the die bounces
  off the sides of the picture instead of half-vanishing behind them.
- **The live region is the die's whole accessible surface.** The number is
  painted into a WebGL canvas, which is opaque to assistive technology by
  construction, so the `VisuallyHidden` announcement in `ui/D20.tsx` is not a
  duplicate of something on screen -- it is the only channel carrying the
  result. For the same reason the scene's container is `role="button"` and
  focusable with an Enter/Space handler: every other way of throwing the die is
  pointer input, and a die you can only throw with a thumb is a die some people
  cannot throw at all.

### The die you can pick up

Pressing does not throw. It picks the die up: while a finger is down the body
goes **kinematic** -- gravity stops applying and the walls stop pushing back --
and its position is set from the pointer every frame, so the die simply follows
your hand. Letting go returns it to dynamic and hands it the velocity your hand
had, from the last few samples of the drag rather than the whole of it: what
the hand was doing at the moment of release is the fling, and averaging in the
slow start of a long drag flattens every throw towards nothing.

The die moves by the drag's **delta**, not to the finger. Snapping it under the
pointer is simpler, but pressing anywhere in a tall
panel then teleports the die across the screen before you have moved at all.
Tracking the offset means a press picks the die up wherever it happens to be.

Mapping screen to world is similar triangles rather than a raycast, because the
camera looks straight down: the picture is a known width per unit of distance,
and the held plane is a known distance away. Screen up is world -z -- that is
what `camera.up` was set to -- so the vertical axis is negated on the way in.

A release with no speed behind it still throws, from a random direction. A die
that could be picked up and put down again would be a control that did nothing.

### Killing the tail of a throw

The last phase of a throw is the part nobody watches. The die has visibly
finished, and then spends another second nudging itself a few degrees at a time
until the rest test agrees -- which reads as lag, not as physics.

Raising gravity does not fix it, and that is worth knowing before trying:
a nearly-stationary die is barely falling, so the tail is governed by damping
and restitution rather than by weight.

What works is a threshold, and it is what lets the two halves of the throw want
opposite things. The die is *meant* to be bouncy -- restitution is `0.62` so it
visibly caroms off the walls -- and bounciness is exactly what produces a long
tail of diminishing hops. So above `SETTLING_SPEED_SQ` the die is still being
thrown and damping is almost nothing; the moment it drops below, damping goes
up twentyfold and the die commits. Tuning either half is a matter of moving
that one number rather than trading the bounce against the wait.

One CSS property is doing more work than it looks: the canvas carries
`touch-action: none`. Without it the landing carousel reads a drag across the
die as a swipe between panels, so throwing the die turns the page instead.

### The geometry is derived, shared, and the only part that is unit-tested

`ui/d20Geometry.ts` holds the solid and imports nothing -- no three, no cannon,
no React. Three consumers have to agree about it exactly: the rendered mesh,
the physics collider standing in for it, and the reader that decides which face
landed up. A collider that disagreed with its mesh by one flipped winding is a
die that visibly bounces off nothing.

Twelve golden-ratio vertices; a face is any three of them one edge apart; the
scan finds exactly twenty, which is its own proof. Two properties are fixed
afterwards and both are the kind of bug that does not look like one:

- **Winding.** Each triple is reordered to be counter-clockwise seen from
  outside. Neither consumer checks: three would cull the face as a backface,
  and cannon would compute an inward normal and let the die fall through the
  floor it is standing on.
- **Numbering.** Opposite faces sum to 21, as on a real die, and 6 and 9 are
  underlined in the texture, because on a solid that lands in any orientation
  there is nothing else to tell them apart.

`d20Geometry.test.ts` is where the real coverage is, and it is all pure
arithmetic: every vertex in exactly five faces, every face wound outward, every
value used once with antipodes summing to 21, and -- the one that matters most
-- all twenty faces rotated upward in turn and read back, which is what stands
between the player and a reader that is out by one face. None of it needs a
GPU. `D20.test.tsx` pins only that the heavy chunk stays unloaded, because a
WebGL die is untestable in happy-dom and asserting on a mock of one would be
asserting on the mock.

The numerals are drawn to a canvas at load rather than shipped as an image:
no binary asset to generate, commit and keep in step with the palette -- the
argument `scripts/gen-icons.mjs` exists to make, minus the file.

That canvas is a 5x4 atlas, and each face maps onto one cell through `atlasUV`
-- a mapping with a second winding to get right, quite apart from the one the
mesh and the collider share. A face is wound counter-clockwise seen from
outside, so its three corners have to be listed counter-clockwise *as a human
sees the image*; `v` runs downward, so that order is apex, bottom-left,
bottom-right. Listed the other way round the map reverses orientation and every
numeral is drawn mirrored -- right place, right size, backwards, and invisible
to a suite that can only check where triangles are. `d20Geometry.test.ts` pins
the sign of that area, which is the only thing standing between the die and
a backwards 7.

`embla-carousel` itself is in `check-layers.mjs`'s `ui/`-only package list.
`@mantine/carousel` is guarded by the `@mantine/` entry, but the engine
underneath it ships its own types and nothing else would stop a feature importing
`EmblaCarouselType` directly -- the same hole `@tabler/` came through, one
package wider.

It costs something in the test suite too. embla constructs a `ResizeObserver`
and an `IntersectionObserver` unconditionally, and the test DOM implements neither, so
`test/setup.ts` installs inert stubs. They deliberately never fire: happy-dom has no
layout, so anything they reported would be fiction, and a test that leaned on one
would be testing the stub. A carousel is therefore asserted on its structure --
its panels named, in order, in a named region, and whatever the call site does
about height -- and never on which panel is scrolled into view. That is why
`LandingPage.test.tsx` pins a height expression that still mentions both shell
offsets, while a `TabDeck` never sets `--carousel-height` at all.

It is also why the deck's tab strip reads React state rather than embla's
`selectedScrollSnap()`. Pressing a tab is therefore observable in happy-dom and
swiping is not, which is the right way round: the press is the thing a test can
honestly make a claim about.

## Folders are filing, not sharing

A folder is a named place one account files its characters, and nothing else:
one owner, nothing shared with anybody, which is why the screen has no owner
column and no permissions on it. It is deliberately **not** called a group: the
Groups section next to it is the shared one, with people and ranks in it, and
the two words name genuinely different things. A folder lives inside the
Characters section and never appears in Groups.

### A folder is the structure of the page, not a filter on it

Every folder is a bordered `FolderPanel` -- a heading you can collapse, its
own table under it, and its own **New character** beneath that. (**Import** is
withheld for now; `AgentImportScreen` and its route are
untouched, so putting the button back is one line in `FolderAdditions`.)
Four things follow:

- **There is nothing to filter.** A folder you can see the edges of is not a
  narrowing of a list; it *is* the list. So there is no folder `Select` above
  it, and `ui/Page` has no `filters` slot.
- **There is nothing for the Folder column to say.** Inside a folder every row
  would carry the same word as the heading two lines above it.
- **`?folder=` is a fact about where you pressed** rather than about what a
  control was set to. Every add button carries its own folder, which is also
  why each one is named for it -- three buttons reading "New character" is the
  same ambiguity as a column of "Delete"s.
- **Both resources drive the page's state.** A character carries a folder *id*,
  so a folder listing that will not load leaves nothing to head the panels
  with. So there is one alert for the page, not a second, inline one for the
  folders request: without the folders there are no rows to draw.

### The order is the account's, and reordering says so whole

Folders are drawn in the order their owner put them in. The default folder
leads regardless -- it is the one folder an account cannot lose, and a list
whose first entry wanders is a list nobody can point at -- so it has no grip
and no **Move up**/**Move down**.

Reordering is `PUT /v1/folders/order` carrying **every movable folder in the
wanted order**, not a single move. A "move this one up" applied to a listing
that has changed since it was drawn moves the wrong folder; a complete order
either matches what the account has or is refused, and sending it twice leaves
the same result. That is what makes a drag safe to re-send. See
[docs/backend.md](backend.md#folders).

The drag itself is hand-rolled over the native events, the way
`features/character/ScoreAssignment` is, and for the same two reasons: there is
no drag library below `@/ui`, and a native drag fires on neither a touchscreen
nor happy-dom. So it is never the only way to do something -- **Move up** and
**Move down** in each folder's menu are the real path, and the one the tests
press. Four folder actions is also the case `@/ui` blesses a `Menu` for, rather
than the spelled-out buttons a table row gets.

## Where a control lives says what it acts on

Three rules, applied to every list screen, because a control's position is the
only thing on screen that says what it will change.

**The heading line acts on the entity the page is about**, and on nothing else:
rename, delete -- and, in `Page`'s **`lead`** slot beside the trail rather than
against the right edge, **Leave**. That one is not a thing done to the group so
much as a statement about your standing in it, and it reads as part of where you
are; `lead` exists for that and has one caller. It is not where you add to it.
The two character screens are the rule applied to something that is not a list:
the sheet's **Edit** and the build screen's **Finish** both act on the
character, and both sit on the trail's line, against the right edge, drawn by
`ui/Page`'s `actions`.

**A row acts on that row.** Rename and delete sit in the row whose entity they
edit, with an icon each -- and whether they are drawn is the caller's rank at
*that* row's table, not at whichever one happens to be first. A DM at one group
and a player at another must not get a Delete on the second because they have
one on the first.

**A row's actions are data, not markup.** A screen hands `ui/DataList` a
`rowActions` list -- a label, an icon, a colour, a handler -- and `DataList`
draws it **behind one `⋮` menu at every width**, named for its row ("Actions for
Ada"). Spelled out as buttons on a wide table, a three-action row -- Make DM,
Make owner, Remove from group -- is a control panel wider than the data beside
it. One rendering, no switch. The items inside the menu need no row name of
their own: they are inside a menu that said whose row it is.

They are data rather than a `ReactNode` slot because a cluster of `<Button>`s
cannot become menu items; a list of descriptions can.

**Leaving is not editing.** It comes before rename and delete rather than
between them, because sitting in the middle of that pair reads as though it
were one of them.

Every one of these controls draws an `ACTION_ICON_SIZE` glyph from `@/ui`. It is
a constant rather than a literal because the sizes had already drifted once: the
icons were a mix of 14, 16 and the icon package's own 24, so the same three
actions were drawn three different sizes depending on which screen you were
looking at.

There is no size constant beside it: a button's size is one theme default for
the whole app, so a row's buttons cannot drift from a heading's by being
written down separately.

The heading's controls are capped to `CONTENT_MAX_WIDTH` too, so they land on the
table's right edge rather than the window's -- otherwise Rename and Delete drift
away from the rows they act on as the monitor gets wider.

**Adding goes under the table, on the left.** New group, New game, Add a
character, Invite -- all of them add a row, so all of them sit beneath the rows.
Invite is the one that reads oddly until you see it that way: it is not a thing
you do *to* a group, it is how a person gets added to one.

The icons come from `@tabler/icons-react`, re-exported one glyph at a time from
`@/ui/index.ts` -- and that re-export list *is* the app's icon inventory. They
sit beside a text label that already names the action, so each is `aria-hidden`
by omission: a tabler `<svg>` carries no `<title>` and adds nothing to an
accessible name, which is also why `getByRole('button', { name })` is unaffected
by adding one.

A **section's glyph** is drawn wherever the section is *named* -- the desktop
navbar, the phone dropdown (its items and its trigger), and the first crumb of
every trail -- and nowhere else. Not on buttons, not in table cells, not on the
mobile cards.

Every page is capped at `CONTENT_MAX_WIDTH` (1024px) on desktop, applied once by
`ui/Page`. It is a fact about the page, not about tables -- the character
sheet is not a table and is capped all the same -- so it is a layout token in
`theme/tokens.ts`.

## Every page is the same page

Characters, Groups and Games do the same job -- a list of things you own or sit
at, each row opening onto a detail page -- so they are built the same way:
`ui/Page` is the one shape they share, loading and failure states, content
width and heading included.

**The last crumb is the heading, and the whole trail is one line at one size.**
There is no separate title line: the trail runs `Groups / Wednesday Night` with
the page's own name last, and every part of it is drawn the same -- not a
heading on the list screen and a small crumb on everything below it, the same
word in two sizes.

The heading element holds the page's name *and nothing else*, so its accessible
name is that name rather than the whole trail; the parents beside it are a real
`<nav aria-label="Breadcrumb">` of links. A page still has exactly one
`role="heading"` at level 2.

**An action is centred on the trail, not pinned to the top of its row.** The
heading line is `ROW_HEIGHT` tall and the trail sits in the middle of it, so an
action aligned to `flex-start` hung a few pixels above the words it belongs to
-- little enough to look like two rows that had failed to line up rather than
like a decision.

**The heading lines up with the navbar entry naming the same section.** A
section is named twice on screen at once -- in the navbar, and again as the
heading of the page it opened -- and the two sat 4px apart with glyphs of 18 and
20px, which is close enough to read as a mistake rather than a choice. They
share `ROW_HEIGHT` and `CHROME_INSET` from `theme/tokens.ts` and draw the same
glyph size, so they land on one line by construction rather than by either side
nudging itself into place. (`AppShell.Main`'s top padding has to carry
`--app-shell-header-offset` explicitly when it is overridden: Mantine builds it
as offset plus shell padding, and a bare number drops the offset and slides the
page under the header.)

That line is **smaller on a phone than on desktop** (`h4` against `h3`): a
trail runs two or three names deep, and at a larger size that wraps a 390px
line twice. It is a responsive value, not a branch.

**A section root renders no `<nav>` at all.** With an empty `trail` there is one
crumb, it is the heading, and there is nothing above it to navigate to -- which
is what lets "the trail replaces the title" need no special case for the three
list screens.

**Below `md`, the section's *word* is dropped -- not its crumb.** The phone's
one row of chrome already carries a control naming the section you are in and
opening the others, with that section's glyph beside it. So a crumb spelling the
word out again spends a 390px line restating what sits an inch above it, and on
a section root the heading *is* that word.

The way back is not a restatement, though: without it a group's page has no
route to Groups but the browser's own Back button. So on a subpage the section crumb stays as **its
glyph alone, carrying the link**, and only the word and the `/` after it are
hidden. On a section root there is no crumb at all -- there is nothing above it
to go back to -- which is what "not for the main menu item" means here.

Two things to know about that. It is done with `visibleFrom` rather than a
branch, so `Page` still renders one tree at every width and stays off the list
of components that swap markup. And the link carries an `aria-label`: below
`md` the only thing left inside it is an `aria-hidden` glyph, and a link with no
accessible name is a link nobody can follow. Deeper crumbs keep their words at
both widths, because a group on a shared character's sheet is a real parent
rather than a restatement of the chrome.

**A page the chrome names drops its heading below `md`**, and a section root is
the usual case: `Page` knows the section from the URL and the phone's selector
is showing that very word. `/account` is the exception that needed saying out
loud -- it belongs to no section, and the selector names it anyway because the
account is a row in the menu it opens -- so it passes **`namedByChrome`** and
gets the same treatment. A desktop is untouched by either: same `h2`, same
1024px cap, same starting height as every other page.

**The row goes with the word, and the block goes with the row.** Hiding the
heading alone left what it sat in: `ROW_HEIGHT` of nothing plus the stack's
gap, above every list in the app, which on a 390px screen is the most expensive
blank space there is. So `Page` also drops the heading row when the phone would
find nothing on it, and the whole header block when the subtitle has gone too.
Both are decisions about the *props* -- is there a badge, an action, a subtitle
-- and the breakpoint stays inside `visibleFrom`, so this is still one tree.
A section root with an action on its line keeps the row at both widths and
loses only the duplicated word.

**The current page is not repeated inside the trail.** A breadcrumb ending in a
non-link copy of the heading directly beneath it says the same name twice to a
screen reader. The nav is the path *to* here; the heading is here.

**The section crumb is never passed by a screen.** `Page` derives it from the
URL, so a screen cannot start its trail somewhere the navbar disagrees with.

**A crumb whose name has not arrived is a `Skeleton` with a hidden "Loading".**
An `<h2>` with no accessible name is a hole in the page, and a heading that
appears a beat late moves everything under it. The skeleton is a `span`, because
a `div` inside a heading is invalid markup.

**`loading` and `failed` replace the body, never the header.** There are no
early returns: you still know where you are when the thing you came
for will not load, and "Try again" sits beneath a trail rather than alone on a
blank page. `pageState` adapts a `useResource` in one call. The loading line
takes an override for the two screens whose word says something the generic one
does not -- "Projecting the sheet...", "Reading the log..." -- rather than
imposing one word everywhere or letting four drift apart again.

**`PageBody` is the same two blocks around a smaller region**, exported from
`ui/Page.tsx` for the screen whose *controls* have loaded and whose *results*
have not. A page is the right unit when there is nothing at all to show; it is
the wrong one when replacing the body would also replace the filters you just
touched. `features/spells/SpellsScreen.tsx` is the worked example and the
reason it exists -- see below. Reach for `Page`'s own `state` prop first; this is the
escape hatch.

**`Page` does not branch on viewport, and must not.** The actions wrap under the
heading on a narrow screen because the row is allowed to wrap, and the cap is
inert below 1024px. `Page.test.tsx` pins that by comparing the two renderings
byte for byte. What may branch on the width is the handful of `ui/` components
that genuinely have to -- `DataList`, `ModalSheet`, `TabDeck`, `ChoiceDetails`
and, for its wrapping alone, `TabRow`.

### What stayed different, and why

Unifying is not flattening. Two things survived because each does real work:

| Kept | Why |
| --- | --- |
| Groups' `TabRow` | Members and Characters are two views of one table. |
| Games' gate on **New game** | You can only open a game at a table you run. |

Out of scope, deliberately: `/login`, `/legal`, the 404 and the join
flow. None of them is in a section and none is behind the signed-in chrome.

`/account` **is** in scope, and wears `Page`. Being in no section is not the
same as having no shape: `sectionFor` answers null for `/account`, which is
exactly the case `Page` already draws as a heading with no breadcrumb above it.

## Sharing is reading, and it is one component

A group screen has two tabs -- Members and Characters -- because both are the
same table seen two ways and neither is a page of its own; somebody who can
grant private packs gets a third. `TabRow` is what draws them. **Games are
deliberately not a tab**: see below.

The **Characters** tab is what the group's members have shared with each other.
Sharing grants a read and only a read, and the panel says so by what it does not
draw: there is no edit control anywhere on it, and no build link. That
is not the client hiding something it could offer -- there is no route behind it
for anybody but the owner, so a button would come back 404. The only action
on somebody else's row is **Take off**, and only for a DM, because a guest's
session ends and their character would otherwise be stuck on the table.

A shared character opens at `/groups/:id/characters/:character`, and it draws
`SheetBody` -- **the same component its owner's own sheet draws**. That is the
point of the split: the server renders both with one converter, so the table is
looking at the character rather than at a summary of it, and the two cannot
drift into disagreeing about what it is. What surrounds the body differs, and
that is the whole difference between the two pages.

A **game** is one sitting, and it is never called a session -- in this client
that word means being signed in, right down to `SessionUser` and
`startGuestSession` in the same flat `@/lib/api` barrel.

## Spells are the compendium, browsable

A `SECTIONS` entry, at `/spells` and `/spells/:slug`. The section is a
reader over the catalogue rather than anybody's data: the rows belong to no
account, there is nothing to create and no row action, and a guest sees exactly
what an account does. It is still behind `Private` like every section, because
the catalog endpoints require a session -- the Go router marks the two lines to
move if a truly public browser is ever wanted.

It is also a **search-and-filter surface**, and the server
does the work: `searchSpells` in `lib/api/catalog.ts` sends the filters as
query parameters and gets back a `{spells, total}` page, filtered, sorted and
paged by `search.go` -- so the screen only says what it wants and appends
pages behind a **Load more** button, shown while fewer rows are loaded than
`total` says exist. The search box debounces 300ms before writing the URL,
because the URL is the request and firing one per letter would race four
requests to answer the word. The filter state lives in `useSearchParams`, not
component state, so a filtered list survives a reload and can be handed to
somebody as a URL, the same way the character list carries `?folder=`.

The detail page asks the same endpoint for the whole entry with `?slugs=`,
which is where the prose and the remaining rule values live.

**Item artwork uses shared pack labels.** Equipment and magic items reference
one pack-local `icons.items` library. Resolved API entries carry an optional
`icon` data URL. `ui/ItemIcon` renders the 128×128 assets at 66×66, one and
a half times the spell icon, with pixelated scaling beside the visible item name.
An equipment slot reserves that height whether or not anything is worn, and
cuts the worn item's facts and description to it, so equipping never moves the
cards below. Inventory rows, equipped slots
and starting-equipment choices use the same component; equipment
columns stack on narrow screens to make room. Missing images leave the name
and controls usable. Item artwork is decorative to screen readers. See
`docs/packs.md` for the authored mapping and conversion workflow.

**Spell artwork belongs to its pack release.** One WebP per spell lives in
`data/pack/srd-5.1/spell-icons/`, the non-SRD spells included, and the loader
picks them up by name. Existing catalog summaries and details expose an
optional `icon` WebP data URL. The same 44px slate rounded square is used in
spell lists, spell details and character spell choices. A small inset keeps
transparent artwork inside the tile, and the dark background makes glowing
runes readable on both light and dark pages. Packs without artwork show no
image. Changing packs or releases changes the artwork with the catalog.

**Adjusting a filter must not take the filters down.** `useResource` blanks
itself when its key changes, which is correct for a key naming a different
*thing* -- a different character, a different group -- and wrong for the one
key in this app that carries adjustable filter state. Were the search results
and the schools and classes filling the Selects a single resource keyed on the
search, every ticked checkbox would throw away the controls that ticked it, the
page-level state would take the whole screen down to a spinner and rebuild it
-- search box, filters, count and table -- to change which rows were in the
table, and typing would lose the caret on every pause.

So it is two resources. `spells:options` is keyed on nothing, loads once --
one small request for the packs, books, schools and classes there are to filter
by, `…/catalog/spell-filters`, never the spells themselves -- and is the only
one that gates the page. The search is keyed on its filters but gates the
results region alone through `PageBody`. A third piece finishes it: the screen
holds the last page that arrived, so a search in flight *dims* the rows already
on screen instead of emptying the table. They are the previous answer, not a
wrong one.

Artwork travels through the existing catalog response rather than Vite assets
or separate image routes. It is not included in the application bundle or PWA
precache. This supports packs imported after deployment without rebuilding the
website. The tradeoff is larger catalog JSON and no independent image cache;
the SRD artwork is about 8 MB before base64 encoding. Private and shared pack
artwork follows the existing catalog authorization and cache policy.

That tradeoff is why **the spell list is never downloaded**. Inline artwork
makes every spell about 30 KB, so "all the spells" is 11 MB, and the server
refuses the bare collection outright (see
[backend.md](backend.md#spells-are-never-served-whole)). A screen gets spells
three ways and no others: the ones it names (`getEntries('spells', slugs)`),
a page of a search (`searchSpells`, `searchSpellOffer`), or resolved inside a
sheet. `getCollection('spells')` is a request the API answers with a 400.

**`features/spells/spellText.ts` is the piece the structured rule values were
waiting for**: the compendium stores casting time, range and duration as
`{kind, amount, unit}` precisely so a client can render "90 feet" per locale,
and this module is that rendering. It sits in `features/spells/` and *not* in
`src/domain/`, deliberately: `domain/` may not import `@/lib`, and these
functions want the typed `Translate` so every key they name is checked against
`en.json` at compile time -- the same reasoning that moved
`features/character/labels.ts` out of `domain/`. Other features import it from
here, which is precedented (`features/games` imports `../groups/roles`).

## Games are a section, not a corner of a group

The header enforces it. A game's trail is `Games / Thursday night`, and the
group it is played at is a **subtitle, never a crumb** -- a trail reading
`Groups / Wednesday Night / Thursday night` would say the opposite of this
section and would disagree with the navbar, which lights Games.

**A character's trail runs through whose it is.** Somebody else's character,
opened from a game or from a group's table, reads `Groups / <player> /
<character>` -- the player first, because a character is somebody's and that
is the fact a reader wants before its name -- not the group, which is how the
read was *granted* rather than whose sheet it is. A player has no page yet, so that crumb is a name and
not a link; `Page` draws a crumb with no `to` at the trail's size, dimmed.
`getSharedOwner` in `lib/api/games.ts` finds the name from two requests the
group's screens already make, since a sheet does not say who owns it.

**Your own character opens as your own sheet.** `features/games/sheetPath`
is the one place the group's table and a game's roster decide where a
character links to: yours to `/characters/:id`, anybody else's to the group's
read -- and to nothing, a plain name, for a player looking at a character its
owner has not opened. A stale or hand-typed shared URL for your own character redirects the
same way, so there is one view of a character you can change.

**The owner opens or hides a character from its sheet.** The header's
**Access** button (`features/character/VisibilityAction`) opens a sheet with
one switch -- open to anyone with the link -- and, once it is on, the link
itself as a field and a Copy button. The sheet says what "open" means before
the switch is thrown: anybody *signed in* who has the link reads it, only the
owner changes it, and in the groups it is shared with the DMs and the group's
owner read it either way while players read it only once it is open. The
link is `/shared/:character`, a route with no group in it because no group
grants that read; it draws the same read-only sheet, without a player crumb,
since with no group there is nobody to ask whose it is. Its items are at
`/shared/:character/items/:slug`.

**An item is a sub-page of its character**: `<character> / <item>` on your
own sheet, `<player> / <character> / <item>` on a shared one, the character
crumb leading back to the sheet the item was opened from.

The trail is smaller on a phone than on a wide screen (`h4` against `h3`):
two or three names at `h2` wrap a 390px line twice.

One wrinkle, recorded because it is a gap rather than a decision: that subtitle
points at the group without naming it, because `GameDetail` carries `group_id`
but no `group_name` -- only `GameSummary`, which the list screen uses, does.
Closing it means either the field on the detail response or an extra `getGroup`
on every game page for one word.

Games get their own `SECTIONS` entry and live at `/games` and `/games/:id`,
beside Characters and Groups rather than inside one.

The reason is the same one that keeps folders out of Groups: **a game belongs
to a group the way a character belongs to a folder** -- the group is a fact
about the game, not the route to it. Somebody who plays at three tables wants
one list of their games, and making them open a group first is asking them to
remember where Thursday's game lives in order to find it. `GET /v1/games`
answers that in one request, and each row carries `group_name` so the list can
say which table without a request per row.

A game screen seats shared player characters from its group and creates
private NPCs from owned characters or editable stubs.

**Add character from group** is a flat list. A game is played at exactly one
group, so there is one set of shared characters and nothing to branch on.

**Add NPC from my characters** is a tree of your folders, collapsed. A folder is how its
owner already thinks about their characters -- somebody with three campaigns'
worth of them knows which shelf tonight's is on -- and a flat list would make
them read every name to find it. It fetches everything up front rather than per
branch, because a character listing carries its own folder: one request covers
every shelf, and a request per shelf would be slower for no benefit.

The group picker offers only characters that are not already seated. The NPC
picker permits repeated private copies of any owned character.

Development builds expose the seeded accounts on `/login` and through the
account icon in the signed-in header. The choices are master, player1 and
player2; each establishes a normal session through `/v1/dev/login`, then
reloads into the current seeded game or the training game. Reloading clears
resources and unsaved drafts belonging to the previous identity. Open separate
tabs and choose a different account in each to test the master and players
together. Each shortcut login selects its own HttpOnly cookie using a random
value in tab-local sessionStorage; switching one tab preserves the others.
Development API servers also namespace their cookies by listen address so
worktrees on different ports cannot overwrite each other. Both controls
are gated by `import.meta.env.DEV`, so production bundles offer none of them.
See [the seeded party](backend.md#seeded-development-party) for the sample
characters, games, locks and private NPCs.

The roster is an active tracker. `GameTracker` renders a name heading, a line
of HP/current maximum, temporary HP, AC, spell save DCs, movement, vision and
rolled initiative, then six ability scores with modifiers, then free-text tags.
Labels sit above values, using the sheet identity table's dimmed captions and
plain bordered cards. Vitals and abilities use the same responsive column
widths (seven on desktop), so both rows align. On mobile each row starts
collapsed, showing HP, temporary HP, AC, initiative and spell DC in five
columns -- the five numbers asked for mid-turn. HP there is the current value
alone, without the maximum, and initiative is captioned with one letter, "I"
or «И», because five columns at 390px leave no room for the word. Russian
roster captions use «Вр. ОЗ» and «СЗ» to fit the compact grid. The mobile
header is the name alone. Unset initiative reads "—", and private NPC values are omitted. A
chevron in the header expands movement, vision and abilities independently
for that entry. **Initiative** has its own entry in the "…" menu and its own
one-field dialog (`InitiativeSheet`), beside **HP**, rather than a third box
in the hit points dialog: it is set once, when a fight starts, and hit points
change all through it, so the two were the wrong things to open together.
Emptying the field clears it -- the entry has not rolled yet. **Items and coins** reach the page through the same "…" menu
and never through the roster payload, which carries no inventory. They are
three entries and three dialogs, because each does one thing and a player in a
fight wants the one they pressed. **Coins** (`CoinsSheet`) is the purse and
nothing else, and is the same dialog for its owner and for a DM: five rows, one
coin under another, edited freely and written once by Save. A write per field
would be a log entry per digit, and two experiences of one purse were one too
many. Behind the one button the two differ: an owner sets their own totals
(`writeChanges`), a DM sends each coin's difference (`POST
.../entries/{entry}/coins`), so coins the player spent in the same moment are
not put back. **Use item** is on a player's own card and lists what is used up,
with a Use button on each row, written as the owner. **Transfer item** is on
the card of whoever *receives* -- any other seated character, offered to
anybody with a character of their own at the game to give from -- so who it
goes to is said by where it was pressed and the dialog needs no recipient
field. It always names its source in a "From" select -- never the receiver
itself, and a real choice with two characters of your own seated. It lists everything carried that the catalogue knows, with a Transfer
button on each row (`POST .../entries/{entry}/give`). Both are one component
(`ItemsSheet`), and in both one press is the whole errand: the dialog closes
and the tracker's notice says what happened ("Used", "Transferred"). Neither
draws the sheet's Items tab: that tab is the purse, wearing, dropping and
adding too, which is the sheet's job. A `Select` in any of these passes
`SHEET_COMBOBOX` like every select in a `ModalSheet`: without it the dropdown
is portalled outside the phone's drawer, whose focus trap takes the focus back,
and the two reopen each other for ever. At the
foot of the roster a DM has "Give an item": the sheet's own `AddItems` search,
which takes `onAdd(hit)` rather than writing the backpack itself, with a
"Give to" select above it and the search scoped to the receiver's catalogue
(`/shared/{character}/catalog`), because their rule packs decide what exists
for them. An Add closes the search -- a DM hands out one thing and goes back to
the table -- and, because nothing on the roster shows an inventory, says so in
the same notice ("Added"), which sits in the corner and leaves after four
seconds. An item's
description is not opened in place here, as it is on a sheet: a found row, and
an item's name in either list, are links to
`/games/{game}/characters/{character}/items/{slug}`, the same `ItemScreen`
read from that character's catalogue with a trail that leads back to the game.
In place it would be a layer over the game page with no address, and the
browser's Back would leave the game altogether. See
[What a table hands over](backend.md#what-a-table-hands-over). An entry's **consumables** are a dialog -- for its owner and a DM only; the
server sends nobody else the pools, so another player's card has no such entry
-- opened from "Consumable slots" in the row's
"…" menu and drawn by `features/character/ResourcePools`: one
row per pool -- spell slots by level, then named pools, then Hit Dice -- with
`ui/Pips` marking a disc for a use still available and a ring for one spent.
They are not in the row itself: a paladin has five pools and a table has six
paladins, and inline the roster stops being a list. Buttons, names and marks
are three columns of one grid, so each starts on the same line in every row,
and a slot row says "Spell slots, level 1" in full: under a shared heading
every row beneath it read as a slot. Where `can_edit` is true a row leads with
a minus and a plus, spend one and give one back, sent as `PATCH
.../entries/{entry}` with `{used: {<pool>: n}}`. The marks are not buttons: a
target that small is hit by accident. A press is drawn at once from a local
count and saved behind it through the dialog's own `useAction`; the roster's
shared one would disable every control on the page for the length of each
request, which read as the whole page redrawing. The local count is dropped
when the server reports the same number, so a long rest called meanwhile still
shows. The plus is the whole undo; there is no per-player rest. Masters have
**Long rest** and **Short rest** in the toolbar, each behind a confirmation
because it cannot be undone. A long rest returns everybody's spent uses; a
short rest returns only what the catalogue says a short rest refills, which
the server decides (docs/backend.md).
A pool of more than twenty (Lay on Hands, high-level
ki) is a number rather than a wall of marks, and a capacity
of 9999 -- how a pack spells "no limit" -- reads "Unlimited". Counts are the
game's, not the character's: see
[backend.md](backend.md#active-game-entries). Tags stay visible in the collapsed row; empty tag lists have no
placeholder or blank row, while editable entries retain the inline add control. The six abilities occupy the first six desktop columns. Row actions use the same “…” menu
at every width. Owners edit their own unlocked game values; masters can lock
player entries and move or sort the shared list. On a wide screen masters can drag an entry by
the dedicated grip with a mouse or stylus, using the folder list's
reserved drop indicator. The pointer is captured until release; a press must
travel eight pixels before it becomes a drag. Canceled gestures and releases
outside the roster leave ordering intact. Moving
down lands after the hovered row; moving up lands before it. Move up/down in
the menu provide keyboard ordering -- and on a phone they are the only ordering:
no grip is drawn there, since it cost a narrow row a column and a drag under a
thumb fights the page's scroll. Only masters can reorder. Sorting remains explicit.
An edit sheet keeps its draft through background updates and disables saving
if a refreshed permission says the entry is locked. The full-width Damage
field previews how much temporary HP absorbs, then subtracts the remainder
from HP, with neither going below zero. Revising or clearing damage recalculates
from the draft's starting values, so typing and failed retries never subtract
twice. Editing HP or temporary HP directly accepts the current preview as the
new draft and clears damage. Apply confirms only changed fields; the close
cross discards the draft.

The master can add private NPC copies from the folder tree, including
multiple copies, or create an editable stub. Stub defaults are name NPC, HP 10/10, AC 10,
walking 30 feet, normal vision, abilities 10, zero temporary HP, no spell DC,
no initiative, and no tags. The master edits name, HP, AC, ability scores, walking speed, and a single
spell save DC. Other copied movement modes and senses remain intact while
editing walking; they are not exposed as extra form controls. A copied caster's
highest DC initializes the field, and an explicit edit applies the new DC to
its casting profiles. Players see only names and ordering. The private flow never calls the
player seating/share action. The toolbar is one row of six buttons in one
style -- Add from group, Add NPC stub, Add prepared NPC, Long rest, Short rest,
Order -- with no icons and no quieter variant, because all six are the same
kind of act on the table. Player seating uses only the group picker. The NPC name is the first field in its edit sheet.
Tags are managed directly in each row: adding or removing a tag immediately
patches only tags, outside the stat editor. Read-only entries show badges;
editable entries show removable tags and a small inline add field. Failed
writes retain the draft, and refreshed locks disable its controls.
For an older running API that still creates 1/1 stubs, the client records the
existing roster and initializes only a uniquely identified new stub to 10/10.
On older APIs, a uniquely identified new stub named Monster also receives the
default NPC name. Character copies and existing NPCs keep their names and HP.

Dragging first uses the atomic `before_id` operation. If an older running API
rejects that operation with a validation error, the client falls back to its
existing adjacent moves, using each confirmed response to check stable IDs and
the destination. Only the dragged entry moves; retries are bounded, missing
entries abort, and permission failures do not trigger fallback. This permits
frontend updates to run against an older API.

The game resource polls every three seconds while visible and refreshes on
focus, visibility return, and successful writes. `useResource` aborts superseded
requests and optionally retains its last result after a background error;
the game displays a retry status until recovery. Initial failures still use
normal page error handling. PATCH requests contain only changed fields, so
saving a draft does not overwrite unrelated updates from another participant.

`GamesScreen` offers **New game** only to somebody who runs at least one table,
because a player has nowhere to put one and a dialog with an empty picker
teaches nothing. The picker is where the group is chosen, which is why creation
is the one call that names a group at all.

A shared character's sheet stays at `/groups/:id/characters/:character`, and
that asymmetry is deliberate: sharing *is* a group's doing and the group is what
grants the read, so the URL says so.

Two things on the character list are not cosmetic:

- **The default folder's menu has no delete control.** It is the folder an
  account is guaranteed to have, and the API refuses to delete it. Rename is
  offered, because what an account cannot lose is the folder, not its name.
- **The delete-folder confirmation states the character count.** Deleting a
  folder deletes the characters in it, and there is no undo. A dialog that
  only named the folder would be
  describing a smaller action than the one about to happen.

New character carries its own folder through as `?folder=`: there is one under
every folder's table, so where you press is where the next character lands.

### A row's dialogs belong to the screen, not to the row

Move, Copy and Delete are not a component rendered inside a table cell that
owns its own `ModalSheet`s. A row's actions are data, and `ui/DataList` renders
no children, so there is nowhere inside a row to put a dialog.

Nor should there be. A dialog mounted in a table cell disappears when its row
does -- and a successful **Move** is precisely the moment the row leaves for
another folder's table.

`useCharacterActions` is a hook that returns the four actions
to hand to each folder's list and the three sheets to render once at the bottom
of the screen, beside the folder dialogs that were already there. Copy has no
dialog, because it asks nothing. **Send a copy** has one because it has
something to show: the link is minted as the sheet opens and drawn as a
selectable field beside its Copy button, for the reason the invite link is.

## The build screen is a loop, not a wizard

The screen is one component and the parts it is made of, all in
`features/character/`. `BuildScreen.tsx` is the orchestration and the JSX.
`useBuildView` reads prompts, events and the sheet; `useBuildDrafts` holds
unsaved inputs; `useBuildStages` decides which tab and block are open;
`useBuildActions` owns every write and the drop preview. `buildModel.ts` and
`answers.ts` are the view types and the pure functions between an answer and
the event that records it, and `DropPreviewSheet.tsx` is the "this would
drop" dialog.

`features/character/BuildScreen` reads `/prompts`, `/events` and `/sheet`, and
draws up to eleven tabs -- Cantrips and Spells only for a caster, Equipment only
on a visit that was asked about a starting kit, Custom once the character
exists, see [Builder choice
behavior](#builder-choice-behavior). It is a loop rather than an N-step wizard, and it has to
be: prompts nest -- answering the "two skills" branch of a rogue's Expertise is
what brings the two-skill prompt into existence -- so the total number of steps
is not knowable until the last one is answered. The tabs are not steps. They
are display categories in the order `domain/stages.ts` states: rules, personal
(the server's `identity` group), class, abilities, race, background, cantrips, spells, equipment, personality,
custom. Spell and equipment
choices use their kind to select a tab; other choices use the server's group.
Saved answers use the same mapping, with prompt IDs as a fallback for older
events without choice metadata. Rule ownership remains class/race/background.
Class comes first after the name, because it is the choice the most other
choices hang off -- and the scores straight after it, because they are
what the class was picked *for*: a barbarian wants the 15 in Strength, and
deciding that while the class is still the last thing you looked at is the
difference between building a character and filling in a form. Personality is
the last tab that asks anything, and asks nothing about the rules -- see
[below](#who-the-character-is-is-its-own-tab-and-its-own-words). **Custom** is
after it and asks nothing at all -- see
[Custom is what the player writes unasked](#custom-is-what-the-player-writes-unasked).

### The tabs are a deck, so a phone can swipe between them

The tabs are `ui/TabDeck`. **On a phone** it is a strip over a carousel of
all the panels, every one mounted, one on screen: pressing a tab scrolls the
carousel to it and swiping the panel reports the tab it landed on, and neither
can drive the other in a loop -- scrolling to the slide embla already holds does
nothing, and the deck only reports a slide that is not the one the caller asked
for.

**The strip scrolls on a phone and wraps on a wide screen.** `ui/TabRow` keeps
its tabs on one line under `md`, where a swipe finds the ones off the edge and
the strip comes to rest on the active tab. Above `md` there is no swipe, and
that same rest position would hide the *first* tabs: a caster's builder in
Russian is ten or eleven tabs, wider than the page's content column, and
opened on Spells it would show the row from "Race" on with nothing to say
there was more to the left. So on a wide screen the list wraps onto a second line instead. It
costs a row of height and hides nothing; the scroller is still there and has
nothing to scroll, so the edge fade never draws.

**On a wide screen there is no carousel at all**, only the strip and the panel
that is showing. The carousel answers a phone and nothing else: there the panel
is the biggest thing on screen and a swipe across it is the cheapest gesture
available, where reaching back up to a 60px tab is the dearest. With a mouse the
tabs are one click away, a drag across the page is how you select text, and
mounting every panel to show one is many times the work for a gesture nobody
makes. That is the branch, and it is why `TabDeck` is on the list of components
whose two renderings differ rather than the list whose renderings match.

**A press scrolls, a swipe is left alone, and anything else jumps.** Sliding
from one tab to the next answers a press -- it shows which way you went. The
same slide arriving unasked is the page moving while you are reading it, which
is what a cold load of the build screen would otherwise do: it opens on the
first unanswered category, so every load would begin on the first tab and
slide sideways off it.

A swipe is the third case: embla selects the slide the moment the gesture
decides and is still settling on to it, so a deck that "synced" to that
selection would cut its own animation short. The deck checks
`selectedScrollSnap()` and does nothing when the carousel is already where it is
being asked to go.

This is the same object as the character sheet's deck, and that is why it
lives in `ui/`: stage tabs a player leafs between are sheet sections under a
different name, and two copies of a two-way embla sync are two copies of the
one thing in it that goes subtly wrong. The sheet has no desktop half of its
own either: it is the same tabs at every width, and this primitive draws both.

Two things follow from every panel being mounted, and both are worth knowing.
Where a block sits is remembered per tab rather than for the screen as a whole
(see [One block per choice](#one-block-per-choice)): with every tab drawn at
once, one shared memory would let the place a dropped answer vacated under
*class* be claimed by whatever arrived next under *background*. And a test
about what a tab holds has to say which tab -- `BuildScreen.test.tsx` scopes
those queries to a slide, because a query against the whole document sees
every category at once.

The deck does not swipe while the character does not exist. A tab press there
*creates* one rather than moving anywhere, so a swipe would be a gesture the
screen answers by refusing to move, and a deck that snaps back is worse than
one that never gives.

Keyboard focus walks all the panels rather than stopping at the end of
one, and the strip follows it: that is embla's own `watchFocus`, which scrolls
a focused slide into view and emits the same `select` a swipe does. Nothing in
`TabDeck` implements it, which is worth saying because the obvious hand-rolled
version is a focus handler that fights the one already there.

The screen opens on the first category with something required outstanding --
and, for a character with nothing required left, on the first tab -- not the
first tab with *anything* open: a custom spell is on offer to every character
for ever, and Edit would open a finished fighter on Cantrips. That is the whole of the help it offers: **answering does not move you**.
A tab changes when a tab is pressed, and never on the way back from a write --
answering one question is not a request to be asked another, and a player who
has just chosen barbarian is usually looking at what barbarian brought with
it. That has no exception for the first answer either: see
[Creating is answering the first question](#creating-is-answering-the-first-question).
There is no Next in the tab row for the same reason: the order is the player's,
and a control that walks the tabs in the server's order is a wizard's stride in
a screen that is not a wizard. `Finish` is on the trail's line, in `Page`'s
`actions`, because it acts on the character rather than on the tabs.

A `Next` does appear **under the list, once a tab has nothing left to answer**,
and only then. That is not navigation -- the tabs are always there -- it is the
end of a piece of work saying where the next piece is, at the moment when that
is the only thing left to say. It goes to the next later category with anything
still open, optional included -- that is how a player reaches Personality --
and wraps round only for *required* work. The standing offer of a custom
spell (`purpose: custom`) is not followed: it is open on every character, and
Next would walk a fighter through Cantrips and Spells because of it. **With nowhere left to go and
nothing required open, Next is Finish**: it lands on the sheet, the same as
the button on the trail. Until the character is complete the last tab has no
Next. The build screen prints no line saying everything required is
answered; the filled Finish and a Next that leaves say it.

Three requests, because they answer three different questions: `/prompts` says
what is still open, `/events` says what was decided and in which entry, and
`/sheet` says what all of it adds up to. The sheet cannot be folded from the
log in the browser -- an ability score improvement's increments are derived at
projection time -- so it is fetched rather than computed.

**Nothing can be answered before it is asked.** Every tab is freely clickable,
because a tab is a place to look as well as a place to answer, but what can be
answered on one is exactly what `/prompts` returned for it. One surface draws
those questions -- the build tab, as blocks that open -- and it is the only one
in the client. The character sheet does not list them. See [what the sheet says
about an unfinished character](#what-the-sheet-says-about-an-unfinished-character).

**The client routes nothing.** Every stored entry carries the group of the
prompt it answered, written by the server, so a change that invalidates an
answer makes the server re-emit that prompt under its own group -- and the
group is the tab. A client that worked out for itself which category an
orphaned answer belonged to would be a second, unverified copy of a decision
the server has already made.

Nothing in it decides what an answer *means* either. The prompt says which
event carries it and the screen copies that verbatim, so the browser never
learns that a first level is a `class` event and a fourth is a `level` one.
Option keys come from the server for the same reason: a bundle of a shortbow
and twenty arrows has no slug of its own, and the server composes it one --
`shortbow+arrow` -- so a settled row can read it back as "Shortbow, Arrow"
without asking the compendium what the answer meant.

The exception, and its bounds, is the character's **inputs**: a name, an
alignment, the six ability scores, the desired level and the ruleset. They
settle a value on the sheet rather than naming a catalogue entry or answering
a grant, so each is stored as an addressed change -- and the prompt, which
says the entry is a `change`, has nowhere to say to which path. `INPUTS` in `features/character/answers.ts` is that table and is
deliberately the only place a path is written down. It is worth knowing why it
exists: an alignment is namespaced `character/alignment` exactly like
`character/race`, this screen read the namespace as the shape, and the
`change` event it posted -- naming an alignment, changing nothing -- was
accepted by a server that could attribute it to no prompt. The alignment
simply never saved, with a 200 to say so. (The server refuses that shape now,
`field.answer.notAPick`: the AI Wizard made the same mistake later, and two
such entries beside the still-open question are three blocks under one key.)

A settled input reads back as what it is. An alignment is shown by the
compendium's name for it, asked for alongside the entries' references
(`alignment:<slug>`), and not as its slug in title case -- English in every
language, and under a rule pack "Dnd 2014/neutral Good". The four written
lines are printed as written even when they are stored as a list, which is
how the wizard stores them: printed the way a list of slugs is, "кто-нибудь"
comes out "кто Нибудь".

One `PromptCard` renders every kind of prompt rather than one component per
kind, because the server synthesises "which race?" into the compendium's own
grammar instead of a second vocabulary. What the kinds change is the wording --
and one thing they change is what a click means. Where a prompt wants **one**
answer, picking another swaps it, because picking another *is* changing your
mind and making somebody unpick first is asking them to operate the form rather
than answer the question. Where it wants **N**, the options that were not
picked go grey as soon as N are: the question has been answered, and an option
that still looks pressable but does nothing reads as a broken button. What was
picked stays live either way, because unpicking is how you undo.

An option's **description sits under the option that was picked**, and only
there. Run along the same line as the name and cut to 120 characters, it gives
a list of six draconic ancestries six half-sentences and no whole one. Under
the name there is room for what the compendium actually wrote, and
under *only the picked one* the list stays a list -- the description is shown
where it is being decided about.

Some questions are genuinely not that shape and get a form each: a name
(`NameForm`), the six ability scores (`AbilityScoresForm`, below), the four
roleplaying lines (`WrittenForm`), the desired level (`DesiredLevelForm`) and
the ruleset (`RulesetForm`). `StagePanel` chooses between them and `PromptCard`
by the kind of the prompt, and that is the only place in the client mapping a
prompt kind to a control.

None of them says what the question is, and there is no exception. The
block they open inside is headed by the choice's own name, and a surface that
repeated it -- "Two more languages", then "Choose 2 more languages" -- would be
asking twice. `NameForm` is not the exception, though "What are they called?"
is the first line anybody reads in this application: with that heading *and* a
field label under it, the first screen anybody sees would say "A name", "What
are they called?" and "Name", three times over one empty box. The block says
it; the field carries an `aria-label` for whoever
cannot see where it sits.

### One block per choice

A tab is one list, and every choice on it is a block that opens onto its own
answering surface -- not what has been decided in one card, what is left in
another, and the question in hand detached at the bottom.

A decided choice and an open one are the same object at two moments, so they
are one list rather than two sections: the choice of a race *is* the question
"which race?" once it has an answer. `features/character/blocks` merges them,
sorted by level with everything un-levelled first, which reads as the story it
is -- took rogue at 1, still owes two skills at 1, gained a level at 2 -- and
`groupByLevel` then cuts the ordered list into one section per level, so the
panel draws a "Level N" heading over each run instead of a tag on every card.
What tells the two apart is that an open block is drawn to stand out, not
where it sits.

A question a **feat** opens carries the level the feat was taken at, and the
server is what says so (`featLevel` in `domain/character/prompts.go`): Slasher
taken at a fighter's fourth level asks "+1 to Strength or Dexterity" as part of
what fourth level asked. Without a level this list would read it as "belongs
to none" and draw it above Level 1, ahead of the improvement that opened it.

**Nothing opens itself**, with one exception below. The screen does not open
the first open question of the tab: it has no way of knowing which of five a player
came here to make, and a surface that opens itself is one they have to close.
One block is open at a time, and answering closes it rather than advancing to
the next question.

**The list grows; it does not rearrange.** Level order decides where a block
goes the first time it is drawn, and after that it stays there:
`features/character/blocks` keeps a `BlockOrder` of where everything sits, and
a key it has not seen sorts to the end. There is one `BlockOrder` **per tab**,
because where a block sits is a fact about the tab it sits on and every tab is
drawn at once -- see [the tabs are a
deck](#the-tabs-are-a-deck-so-a-phone-can-swipe-between-them). Answering a question therefore adds
what the answer brought with it and moves nothing else -- and the entry that
answers a question takes that question's own place, rather than the question
vanishing from the middle of the list while its answer appears at the bottom.
The screen learns which entry that is from the write's response: a single
appended event is the log's new head, so the `seq` it answers with names it.

Nor does the screen go away while it catches up. A write the server has
already confirmed is followed by `useResource`'s `refresh` rather than its
`reload`: the same request, with the list left standing rather than replaced
by a spinner and rebuilt underneath whoever was reading it. A refresh that
*fails* still takes the screen down to its error, because a list quietly out
of date is worse than one that says it could not check.

The trail reads `Characters / Ada`, and ends there: a third crumb naming what
the screen does -- "Creation" -- answers nothing a screen full of questions
is asked, and costs a line of a 390px trail to say it.
The route stays `/build` either way, because a URL somebody has open is not
worth breaking over a word. The tabs are capitalised for the same reason a heading is -- a tab
is a title, not a sentence -- and the word in each is still the category's own,
which is all the rule below asks of it.

`ui/BlockList` is the primitive underneath, wrapping Mantine's accordion so
that feature code neither assembles one nor re-decides its variant. It mounts a
body only while its block is open, which is not a styling nicety: a prompt's
surface fetches the catalogue entries its options name, and a tab of collapsed
blocks would otherwise pay for a collection apiece on every paint. A block with
no body -- a level already taken -- is a statement, not a disabled control.

### Creating is answering the first question

`/characters/new` renders `BuildScreen` with no `:id`. The identity tab holds
the name, answering it creates the character with that name alone, and the URL
is replaced with the build one. It is also the one block that opens itself:
there is no `/prompts` response yet, so the tab poses the question rather than
reading it, and it is the only thing on the page -- nothing else is being
pre-empted, and a front door whose one row is shut reads as broken. There is no
separate create screen because there was never a second thing being done. Nor
does creation carry the score method and the six numbers: that would be eight
selections in one log entry and nothing a player could point at and change. The
scores are an ordinary open choice, answered on the abilities tab and written
as their own entry.

**Creating does not move you either**, and getting that right took carrying the
intended tab across the navigation. Both `/characters/new` and
`/characters/:id/build` render this same component, so React reuses the
instance rather than mounting a second one -- which is why typing a name used
to leave you looking at the class tab with nothing in the code saying
"advance". No tab had been chosen, the reread brought the first real prompts
back, and `firstUnfinished` answered the only question it is ever asked. So the
tab the gesture aimed at rides across in the route's state and is taken up when
the character the screen is looking at changes: confirming the name stays on
identity, and pressing a tab -- which also creates the character, because
nothing else can be answered until it exists -- goes to *that* tab rather than
discarding it.

**Nor does the page come down to make way for it.** `useResource` blanks when
its key changes, which is right -- a different character is a different screen
-- but creation changes the key from `build:` to `build:chr_1` underneath a
screen that is already up, so typing a name tore the whole page off, spinner
and all, for a write that had already succeeded. It read as a page reload
because that is what it looked like. A `creating` flag holds the page through
that one transition: the same chrome, the same tab, and the name still in the
block it was typed into with its button turning, replaced a moment later by
that block with an answer in it.

Clearing that flag is fiddlier than it looks, and the comment in the code says
why: on the render that first sees the new id, `useResource` has not reset
itself yet and still returns the *previous* key's answer -- which for a
character that did not exist is an empty view reading as `ready`. So "there is
data now" is true on exactly the render where it means nothing. The flag goes
when the id has settled and the read it started has finished.

`/characters/new?folder=` carries the folder whose **New character** was
pressed, so that is where the next character lands. Absent, the server resolves the account's
default.

### Changing anything is one mechanism

Every settled block is exactly one log entry, and opening it replaces that
entry: pick the new value, `PUT …?dryRun=true`, read what would be dropped,
commit the same `PUT` on confirmation. There is no separate `[Change]` button
because there is no second gesture on this screen -- pressing the thing you
want to deal with is the whole of it. There is no
append-a-correction path and no Back button. The dialog names every dropped
entry and says the questions will be waiting outstanding in their own
categories, because they will be -- and it opens **only** when something would
be lost. A change that costs nothing else is simply made, because confirming
every change teaches players to confirm without reading, which is exactly the
habit the one change that *does* cost something needs them not to have.

An answer that carries *picks* -- a rogue's Expertise, a starting weapon --
cannot be rebuilt from the answer alone, because the options that made it up
arrived with a prompt the server stopped emitting the moment it was answered.
So the screen asks for it: `GET /prompts?before=<seq>` returns the questions as
they stood at that entry's position, `repose` finds the one the entry answered,
and the block opens **on its answer** -- as deep as the answer went, so a focus
picked out of "an arcane focus" opens on the list of foci with that one pressed.
Opening writes nothing; a change replaces the entry in place.

Dropping the entry instead and waiting for the question to come back would
reach the same question, blank: a player who opened a card to see what they
had picked would see nothing picked, and would in fact have just unpicked it. The drop
remains only where the question cannot be found again -- an entry that bundles
several questions, or a prompt the server no longer poses there -- and then
`reclaimPlace` holds the block's own place for what returns, and it is asked
about on the same rule as everything else: only if another answer cannot
survive it.

The question that comes back is also **open**. `done` takes the key of a block
that does not exist yet, and the reread is what brings it into being; without
that the row you pressed turned back into a shut question and wanted pressing
again, which is the same gesture twice for one intention.

A **subrace and a subclass** take the same route, for a different reason:
their options are narrowed by the entry above them. The server offers a
half-elf their subraces and a rogue their archetypes, and this screen cannot
know either list -- rebuilding the question from the answer's own kind, which
is right for a race or a class, offered Berserker and Champion to a rogue.
`NARROWED` in `answers.ts` is that list, and `reask` returns nothing for it.

### A choice inside a choice is answered where it was asked

Some options are themselves questions. "A martial weapon and a shield, or two
martial weapons" makes the second branch a choice; so does the improvement a
level grants, whose first branch is "raise two of your scores". Confirming
such a pick, posting it, and drawing the branch the server then poses as a
**second block** further down the tab would turn a question the player is in
the middle of answering into two questions in two places.

`PromptCard` resolves it in place. Two things make that possible: the branch's whole inner choice ships inside the option it is
(`Option.choice`), and the server validates a batch of answers one at a time
against a log that grows as each lands -- so a branch answer and the answer it
opens travel in one event, the second legal because the first arrived. So
picking a branch draws its options in the same card, with nothing fetched and
nothing posted, and Confirm sends all of it. `opens` is the client's mirror of
the domain's `addOpened`, bundles included, because "a martial weapon **and** a
shield" is a bundle whose first item is a question.

A branch that is the only branch is not a question, and `settle` takes it on
arrival rather than making it a click: the improvement opens straight onto the
six abilities, with "or a feat" still there to change to.

`loadEntries` reaches into branches for the same reason -- an inner option's
name has to be there before anything is posted, and a branch drawing on a whole
collection ("or a feat") pulls that collection in the same pass.

One entry carries the branch and its contents, so the settled row would
read "Expertise, Skill Stealth, Skill Acrobatics" -- the choice named once as
itself. `leafAnswers` drops the branch answers: an answer whose id is extended
by another answer's in the same entry (`X` and `X/0`) said only which way the
question went. Every branch id in the compendium extends its parent's, and so
does the improvement's, which the server synthesises. The log screen reads
through the same function.

### A category's word appears exactly once

In its tab. A block is headed by the choice's own name -- "A race" -- or by
what was decided -- "Race chosen" -- and never by the category alone, and the
one line a tab with nothing to ask still prints is "Nothing to answer yet"
rather than "nothing to answer in race". (A tab whose questions are all
*answered* prints nothing: the blocks are still there to read, so a line
announcing their absence was announcing something that is not absent.) It is a
small rule with a large payoff: `getByText('race')` means one thing on this page, and
a test that breaks does so for the reason it says.

### The method decides what there is to do about the scores

`AbilityScoresForm` is four editors behind one `Select`, because in the rules
the method decides what is actually being chosen -- and three of the four do
not let a number be typed at all. That is the point of them rather than an
omission: in none of them is the number yours to pick.

| Method | What it is | What you do |
| --- | --- | --- |
| Standard array | six printed numbers | deal them out |
| Rolled | six numbers, 4d6 drop lowest | deal them out, or roll again |
| Point buy | a 27-point budget | spend it |
| Manual | an escape hatch | step or type anything from 1 to 30 |

**Use recommended** deals the scores out for the player, and is there only
when the server has advice to give: the ability-score prompt carries
`recommended`, the character's class's abilities in order of importance, read
from the class's `abilityPriority` in the pack (docs/packs.md). The advice is
an order rather than six numbers so that it fits every method: the dealt
numbers -- printed or rolled -- go out best-first by it, and point buy and
manual, which have no numbers of their own, get the standard array placed the
same way (point buy only where its budget can buy that array). No class yet,
or a pack with no priority for it, means no button rather than a guess. It
fills the form and stops; Confirm is still the player's.

The two that deal out a set share `ScoreAssignment`: a pool you take from and
six abilities to put numbers on. Two gestures, one operation: drag a number, or
tap it and tap where it goes. The tap is not a fallback -- it is the keyboard's
path and the one a screen reader can follow, since every half of the surface is
a real `<button>`.

**The drag is pointer events, not HTML5 drag-and-drop**, and that is what makes
it a gesture a finger can make. `draggable` + `dragstart` is a mouse protocol
that no mobile browser fires for touch, so on it the instruction "drag a
number onto an ability" does nothing whatsoever on the device the app is used
on at a table -- and a single pointer API covers finger, mouse and stylus with
one implementation.
Three things the native protocol did for free are done here instead: the drop
target is found with `elementFromPoint` over a `data-ability` attribute, the
gesture is held with `setPointerCapture` so events keep arriving once the finger
has left what it started on, and `touch-action: none` on anything you can pick
up stops the browser scrolling the page and cancelling the drag on the first
millimetre. A press that never travels `DRAG_THRESHOLD` is not a drag at all and
falls through to the click, which is what keeps the tap exact.
`features/character/scoreDrag` holds the arithmetic and the lookup, in its own
module because a file that exports a component may export nothing else without
losing fast refresh.

The carousel had to be told to keep its hands off first, which is what
`ui/swipe.ts`'s **`NO_SWIPE`** is for. Embla takes the pointer down on any
element that is not a field, and from a few pixels of sideways drift it both
scrolls the deck and swallows the `click` that was about to land -- so on a
phone a drag towards Strength would swipe to the next tab, and even a tap on a
44px square pressed with a thumb would often do nothing at all. `TabDeck` passes
`watchDrag` a predicate instead of `true`, and a surface marked with the
attribute keeps its own gestures: the deck is still swiped from anywhere else on
the slide. `ScoreAssignment` marks the whole
surface rather than each control, because the drift that has to be tolerated
happens between a press and its release and the release is not always over what
was pressed.

The number follows the pointer as a ghost, and it is drawn in a `Portal` for a
reason worth writing down: `position: fixed` is positioned against the nearest
*transformed* ancestor rather than against the viewport, and the carousel's
track is translated on every frame. Drawn in place it is laid out against that
track and clipped by its overflow -- the drag lands and nothing appears to
move. For the same class of reason the gesture's state is read from the render
its handler was made in rather than through a `setState` updater: an updater
must be pure, placing a number is not, and StrictMode invokes it twice to prove
the point.

The suite drives the drag by hand, with `document.elementFromPoint` stubbed:
happy-dom computes no layout, so the browser's one contribution to the gesture is
the one thing a test has to supply. The threshold, the swap and the click that
must not undo the drop are the real code.

Dropping onto a taken ability swaps the two. **Dropping one anywhere that is not
an ability returns it to the pool** -- undoing a placement is half of dealing six
numbers out, and tapping the number and tapping it again is not what anybody tries on a surface that says "drag". Tapping a
placed number twice still works. Nothing can be confirmed until all six are placed,
because six numbers and five decisions is not an answer. Both halves are drawn
at least 44px square: a badge is a few millimetres of target for the one gesture the whole surface exists for.

Point buy and manual share `ScoreStepper`, and only the middle of the row
differs. Point buy's number cannot be typed over -- a score there is *bought*,
and typing 15 into it would be taking it -- while manual's can, because ten
presses to reach a 20 is not an escape hatch. Point buy is priced by
`domain/abilities`, which is where the rule lives: 8 costs nothing, 9 to 13
cost a point each, 14 costs two and 15 costs two more. The steppers refuse a
raise the budget cannot afford, so the screen enforces the budget instead of
complaining about it afterwards -- and points may be left unspent, because a
player who wants an even spread of 13s has spent 25 and is finished.

Manual starts at **ten**, and takes the outgoing method's numbers only when
that method actually produced six. Otherwise switching to it from an unplaced
array would show six zeros -- under its own stated minimum of 1 -- because an
ability nobody has dealt to reads as a 0.

The dice live in `domain/abilities` too, and take the die as a parameter: a
test that cannot say what was rolled can only assert that six numbers came
back, and "between 3 and 18" is not a test of dropping the lowest. The
algorithm is the SRD's rule verbatim -- roll four d6, total the highest three,
six times -- and SRD 5.1 has no re-roll-if-unplayable clause, so neither does
this.

### Who the character is is its own tab, and its own words

`personality` is the last tab with questions -- after spells and equipment,
since none of it is required and the character can be played without it --
and the only one whose questions are not about the rules: a personality trait, an ideal, a bond, a flaw and an alignment. They are
the *background's* questions -- it is the acolyte entry that suggests what an
acolyte tends to believe -- but under background they would put five
questions nobody has to answer in front of the one required question on that
tab. The group of their own is the server's, not this client's:
`domain/stages.ts` has a line for it, and nothing else here knows the tab
exists.

The four are **written, not picked**. SRD 5.1 prints eight of each and the
compendium carries them, but a menu of eight leaves no way to say anything
else about a character who is yours, and the state behind them is free text
(`State.Identity.PersonalityTraits` is `[]string`). The suggestions are still in the compendium for anybody who wants to
read them.

That makes them the character's **inputs**, like a name and an alignment: they
settle a value on the sheet rather than naming a catalogue entry, so each is
written as the change that settles it -- `identity.personalityTraits set "..."`
followed by an `add` per further line, which is how the list is stored and
therefore how it reads back. `features/character/promptNames` holds the one
table of kind to path to noun, from both ends: the field that writes a trait
and the block that heads a decided one cannot come to call it two things.

`WrittenForm` draws **one** field, and it is a `Textarea` rather than an input.
Acolyte's table suggests two traits and the SRD prints eight of each, but a
count is a fact about a *menu* -- "pick two of these eight" -- and what is
asked is one answer in the player's own words, which is a sentence and
sometimes several. Nothing written is the same as not answering, and these are
optional, so the button simply stays disabled.

It is a fixed three rows rather than an autosizing one. Mantine's autosize is
`react-textarea-autosize`, which measures through a listener happy-dom has no
element to attach -- the field could not even be focused under test -- and
`vi.mock` is not available to paper over it. Three rows and a scrollbar is a
smaller loss than a control the suite cannot drive.

The prompt is told apart from a menu the same way the six starting scores are
told apart from a level-up improvement: **by whether it offers anything to pick
between**. That is the server's own statement of what may be picked here, not a
slug this client has memorised.

### Level-up is the desired level

Levelling up is one gesture: declaring the level the character is built
towards. The class tab asks it as `character/desired-level` -- a number
form, answered as the change event that settles `identity.desiredLevel` -- and
the sheet's **Level up** button raises the same declaration: it *replaces* the
entry that made it (the same revision the class tab performs, via
`features/character/desiredLevel.ts` so the two write the same change) and
lands on the class tab, where the choices the declaration opened are waiting.
The declaration *is* the level: the server raises the character to it and
poses what those levels open -- the archetype, the improvements, a feature's
picks -- as ordinary prompts in the ordinary list. So "level me to 9" is one
number followed by the build loop, not a wizard, and this screen has no code
that knows what a level is.

The rules tab asks `character/ruleset` once -- the rules edition of the
selected core pack, confirmed like any other choice -- and the answer is
final: the settled block is drawn locked (`blocksFor` marks it unchangeable), and the
server refuses any change to a value the compendium does not serve.

**All three identity questions are drawn before the character exists**, in the
order they are asked: the name, the rules, the level. Only the name can be
answered there -- it is what creates the character, and the other two are
answered against one -- so the other two are drawn with no body at all, which
`ui/BlockList` already renders as a statement rather than a control that would
fail. `NEW_INITIAL_PROMPTS` in `buildModel.ts` is the client's copy of what the
server poses a moment later, so confirming a name settles a row rather than
growing two new ones underneath it.

**Nothing here asks which class a level went into**, and nothing fills it in
either. There is no such question to ask: multiclassing is not offered, so a
level has one place to go, and the server does not pose it -- see
[backend.md](backend.md#creation-and-level-up-are-one-flow) for what turning
it on costs.

Levels a character already *has* stay visible as settled blocks on the class
tab, under their own "Level N" headings (`groupByLevel`), and stay read-only:
`blocksFor` marks a bare level entry unchangeable, and `ui/BlockList` draws
one as a statement rather than as a control that refuses. They are facts about
the character. The way back down is the class tab's level, revised like any
other entry -- priced and confirmed, so what those levels bought is shown
before it goes.

What a level *granted* is a different thing wearing the same event type, and
stays changeable: an improvement, an Expertise, a feature's pick all arrive as
`level` events carrying answers, and locking them with the levels would take a
whole class of real decisions off the screen.

## The sheet decides what order things come in

Up to seven tabs, at every width: **Overview**, **Actions**, **Spells**,
**Resources**, **Equipment**, **Items**, **Custom**. Spells and Resources are
there when the character has any; Custom is there for the owner always and
for a reader when there is something in it.
`features/character/SheetBody` builds them and `ui/TabDeck` draws them -- a tab
row over the showing panel on a wide screen, the same row over a swiped deck on
a phone. They are named for what a player is doing rather than for a table of
the rulebook: looking the character up, taking a turn, casting, gearing up,
going through the pack.

- **Overview** is who the character is and the abilities everything else is
  derived from, the body's state, then skills, proficiencies, traits and
  personality as headed panels -- two abreast on a wide screen, stacked on a
  phone. Personality prints the trait, ideal, bond and flaw as the player wrote
  them, and says so when one has not been written.
- **Actions** lists what the server derived (`sheet.actions`) and nothing
  else. The list is
  `features/character/SheetActions.tsx`: one group per source -- **Equipment**,
  **Class and race**, **Basic** -- each a heading with a count that folds on
  its own, over a `BlockList` whose rows open onto their description. Only the
  groups a sheet has are drawn. A row is marked with its part of the turn only
  when that is not a plain action -- Bonus action, Reaction, No action -- since
  "Action" on most rows of a list called Actions says nothing.
  **Every group starts folded**: the tab opens as headings with their counts,
  and the player unfolds the one they came for. There is no
  search and no filter: either is more control than a dozen rows need, and the
  one question a player would ask of them, "hide the basic ones", is what a
  folded group already answers. What is on
  the list is the rule pack's decision
  (see [packs.md](packs.md#action-tags)); the client knows no class. The prose
  arrives in the sheet response as `catalog.actions`, keyed by each action's
  `origin`, so opening a row is not a request. A row with no prose -- a mundane
  weapon -- has no body and is drawn as a fact, not a control. An action whose
  source the client does not know is filed under Class and race. A
  pool is shown by capacity, as everywhere on a sheet; spending is the game
  tracker's.
- **Spells** is drawn only for a character with a spell source.
- **Resources** is what a class hands out to spend, drawn only for a
  character with any: the pools ("Consumable slots"), and among them every
  scaling value that is a plain number -- Extra Attacks: 1, Maneuvers: 3 -- as
  that many marks, which a game spends like a pool (docs/backend.md). It grows
  with level rather than coming back on a rest, but a table still counts it off
  within a turn, and a number printed as "Extra Attacks: 1" offered nothing to
  count with. **There is no box of scaling values.** A value that is read
  rather than counted -- a Sneak Attack die -- is the size of a feature, so it
  is on Overview with the features: "Sneak Attack: 1d6" in place of "Sneak
  Attack", and a value no feature is named for follows them. A scaling value of
  zero is one the class has not reached yet and is not drawn.
- **Equipment** and **Items** are described
  [below](#equipment-is-what-is-worn-items-is-what-is-carried).
- **Custom** is described
  [below](#custom-is-what-the-player-writes-unasked).

The same handful of tabs at both widths is one layout to learn, and the cost
is deliberate: the phone's Overview is a scroll rather than
five slides.

### Equipment is what is worn; Items is what is carried

Both are `features/character/SheetEquipment`, one component each over the same
props, and they split the inventory by whether a thing *can* be worn rather than
by whether it is: a wearable in the backpack is on the Equipment tab, under the
doll it could go on.

**Equipment** is, on top, a **paperdoll** of twelve cards in three columns,
stacking on narrow screens: what is held -- Main hand, Off hand, Arms, Custom -- then what is
worn down the middle -- Head, Body, Belt, Legs -- then what hangs or is slipped
on -- Back, Amulet, Ring 1, Ring 2. Each occupied card shows the item's name,
available stats and Markdown description beside its icon. The labels are
the player's words for the catalogue's slots: Belt is `waist`, Amulet is
`neck`, Legs is `feet`, and the two ring cards are the one `ring` slot with
room for two. Arms is bracers *and* gloves, because the catalogue has one slot
for both (docs/dnd.md). **Custom** is the card that takes any wearable: a
second cloak, a third ring, a helm over a circlet. No portrait between the
columns; it
is in the page header already. Below the doll, the **Wearable** rows:
everything with a slot that is *not* worn, one row per entity however many
lists the server splits it across. A worn item is on its card and nowhere
else -- a row counts only the units still carried and is not drawn when there
are none -- so nothing is listed twice and "where is my armor" has one answer.

**Items** is the rest, read top-down with nothing to switch: the **Coins**
panel first, then **Consumables**, then **Other gear**, each a panel with the
same heading the Wearable rows have.

**Each tab ends with an Add button, and each adds from its own half of the
catalogue.** `AddItems` in `ItemPicker.tsx` is the button and the search it
turns into **in place, under the list it adds to** -- not a dialog, so what
was just added is visible above the search that added it. It is a name
search, a category select and a mundane / magic select over rows with **Add**
on each and **Load more** under them -- the spell selection's shape, because
like spells the items are never pulled whole (`searchItems` against
`…/characters/{id}/catalog/items`, docs/backend.md). **Close** puts the
button back.

- **Add equipment**, after the Wearable rows, lists only what has a slot;
  **Add item**, after Other gear, only what has none. That scope is the tab's
  and is not a filter the player can clear, so a thing added from a tab
  appears on that tab.
- **The category options arrive with the page.** The server names them and
  scopes them to the tab's half; nothing here derives them from data.
- **A row is pressed to read the item.** It opens to the same `ItemBody` the
  item page draws -- numbers, source, prose -- through `ChoiceDetails`, so it
  is inline on desktop and a full screen with Back and Add on a phone, as a
  spell's is. The description is asked for when the row opens
  (`getEntries` by slug): a search hit carries a row's worth and no prose.
- **One Add is one more in the backpack and one write** --
  `setTotal(equipment, slug, owned + 1)` through the same `onChange` every row
  menu uses -- and the search closes, as it does when a DM gives an item at a
  game: one thing, then back to the sheet. Staying open for the next would
  leave a player scrolling back up past a page of results to see whether
  anything had happened. An "Added" notice says so above the button and
  leaves after four seconds. It is shown when the sheet's own count of that
  item goes up, not when the button is pressed, so a write that failed is
  never announced as an addition. A row shows `×n` for what the character
  already has, read from the sheet.
- The buttons exist only with `onChange`, which is how a sheet says it is
  editable: the shared, read-only sheet has neither.

A custom item -- one the catalogue does not hold -- is written on a page of
its own, `features/character/CustomItemScreen`, and both tabs lead to it: an
**Add custom item** button beside each tab's Add, on its line and its equal
(`AddItems` takes it as `beside`), and the last entry of every
empty slot's menu, which is why that menu is never disabled.

**It is one screen for every way in, and a page rather than a dialog**, so its
trail is the way back. `/characters/:id/custom-item` writes on the owner's
sheet; `?slot=` (an empty slot's menu sets it) prefills the slot and, if the
item still goes there when saved, puts it on with the same `equip` changes the
slot's menu writes; `/characters/:id/custom-item/:option` changes an existing
one, reached from **Edit** on a custom item's own page. A game opens the same
screen at `/games/:id/characters/:character/custom-item` -- under `/games` so
the trail reads Games / the game / the character -- from **Add custom item** in a player row's menu,
shown to whoever runs the table, with the entry in `?entry=`; there it posts to
`/games/{id}/entries/{entry}/custom-items` and returns to the game, which says
what was given in the same corner notice the item search under the roster
uses -- the page passes it through the navigation's `state`. The form is the
name and description, an icon chosen from a grid of the packs' item icons
(fetched when the grid is opened, never before), category, slot, weight and
cost, and a kind -- neither, weapon, armor -- that reveals the weapon's or the
armor's fields. There is no delete: dropping the item takes it off the sheet.

**The sheet's tab is in the URL** (`?tab=`, `SheetBody`), which is what makes
the crumb back from that page -- and from an item's own -- land on the tab it
was opened from rather than on Overview.

The slot is the catalogue's: each item carries `slot`, written on the pack's
row or derived by the server from what the item is (see
docs/dnd.md). `domain/equipment.ts` reads it and nothing else, so a cloak is on
the back because the catalogue says so, not because the client knows the word.
The group is still derived here: *wearable* is anything with a slot, and
*consumable* is guessed from the category plus a short slug list, because the
catalogue has no "used up" field yet.

The server keeps one `equipped` list and one placement, `equipment.custom`, so
which item sits in which slot is derived too, bar that one: the Custom
occupant is seated first, then everything else goes by its shape, and a second
held item takes the off hand. Custom is stored rather than derived because
nothing about an item's shape could put it there -- a cloak in Custom while
Back stands empty is a choice, and the only way to keep a choice is to write
it down. Nothing is hidden: a slot worn past its capacity lists every
occupant, and an equipped item with no slot at all that nobody placed -- an
old log, an import, homebrew without the field -- is shown in an
**Elsewhere** card that appears only when something is in it -- it is not a
body part but "everything we could not place", so it is not a permanent slot.

**A weapon's numbers are captioned, not written out**, on the Actions tab and
the Equipment tab alike: `features/character/WeaponStats` draws **Damage**,
**Hit** and **Range**, in that order everywhere, each over its own caption and
the two that are rolled in colour. They are `StatColumns`, a stat block's
shape: the value large, because it is what is read, what it is small beneath,
a rule between one and the next -- and nothing in capitals, on any item
caption or badge. A dimmed sentence -- "+5 to hit · 1d8+3 · 5 ft." -- would be
three things read off mid-turn written as prose. Where a row is wide they are columns at its right, and on a
slot card they are columns beside the icon that wrap rather than clip; on a
phone's inventory row, which is too narrow for three columns beside a name,
they lead the row's list of captioned facts. The numbers come from the sheet's action for the weapon when it is
wielded (`weaponNumbers` in `options.ts`), so both tabs print the server's own
sum -- die plus modifier, and the bonus to hit; a weapon that is only carried
has no action, and shows the catalogue's die, type and range with no Hit --
*every* carried one, a second dagger beside the one in the hand included: the
list under the slots is never given the actions, so a carried dagger cannot
find the wielded one's by slug and print a bonus it does not have.
Properties, weight and thrown range are under
the name, each under its own caption; see the bubble below.

**Every item has a page of its own**, `features/character/ItemScreen`, opened
by **Details** in its menu -- on an inventory row and on a slot card alike, and
on a sheet that is only being read, where Details is the menu's one entry. It
is the spell page's shape: the icon beside the name, captioned facts (armor
class, damage, range, properties, weight, cost, the book it is from), then the
description. It reads the *character's* catalogue rather than the compendium's,
since an item on a sheet is that sheet's pack's item, homebrew included; the
two routes say whose -- `/characters/:id/items/:slug`, and the same under
`/groups/:id/characters/:character` for a shared sheet -- and the menu links
relatively, so it lands under whichever is showing.

That page is also why a **slot card says so little**. A card is a name
line -- the item, the slot it is in as a small badge, its menu -- over one
icon's height of columns: a weapon's three numbers, or armor's class and the
Strength it asks (`armorStats`; a shield's class is written as the `+2` it
adds), with a disadvantage on Stealth as one line beneath. None of the other
facts are there, and nothing worn says what it weighs -- weight is a fact about
carrying a thing. Properties, weight and the rest are one press away. An empty
slot is the same card with nothing in it: the slot's badge and the menu on the
name line, where a worn item's are, over the word. It keeps a full card's
height where cards sit side by side and is as short as its two lines on a
phone, where they do not.

**What could be worn is drawn as what is worn.** The Equipment tab's list
under the slots is the same card (`ItemCard` in `features/character/
Inventory`), three across on a wide screen and one on a phone, with a count
where the slot's badge would be: a dagger looks the same in the hand and in
the pack, and its other facts are on its page either way. As rows of captioned
facts, a Russian phone gives "Дистанция метания" half the row and breaks the
damage it is captioning over two lines. The fixed column widths that line
rows up are a wide screen's only; on a phone three of them are wider than the
screen.

Every inventory row on the Items tab is a **bubble** (`features/character/
Inventory`): the name, then the item's facts, **each under its own caption**
(`itemFactList` in `options.ts`, the list the item's page prints, less what
the row draws as columns and what the thing cost) -- not one grey sentence
("Thrown range: 20/60 ft. · Finesse, Light, Thrown · Weight: 1 lb."), three
kinds of thing run together, which on a phone wraps to three lines.
On a wide screen the properties are a line of their own and the rest are
caption-and-value pairs on the next; on a phone everything, the weapon's
numbers first, is one two-column list, captions down the left. The pack and
book badges are on one line in the bottom right corner on a wide screen, under
the numbers and the menu -- beside them they sat at a different place on every
row -- and **a phone's row has no badges**: which book a dagger is from is on
its page, and two pills wrapped under every row said it louder than the
dagger's damage. The columns of one row sit over the next row's:
in a list each is at least a fixed width (`Stat.width`), so "1d4 Piercing" and
"1d6 Bludgeoning" start at the same place; a slot card stands alone and takes
only what it needs. Nothing opens: everything a row has to say is on the
row. The words the line needs -- damage types, weapon properties -- arrive in
the sheet's `catalogNames`, resolved by the server like everything else on the
sheet (docs/backend.md#the-sheet-arrives-resolved).

Both tabs are **editable only when `SheetBody` is given `onEquipment`**. Only
the owner's `CharacterSheetScreen` passes it; `SharedSheetScreen` does not, so
a sheet shared with a table has nothing to press. With it, every row has a
**menu** on the right, and so does every worn item on its card -- the same
three dots, so there is one way to act on an item wherever it is drawn. A
card itself is never pressed -- that would make an empty rectangle a button.
An empty one has the three dots too, opening the **names of what is carried that fits that slot** --
nothing but names, since each has its card below -- and, last, **Add custom
item**, so the menu opens even when nothing carried fits. Naming an item is the row menu's Equip reached from the other end, through the
same `slotsFor` and `equip`, so the two cannot disagree about what goes where. A wearable's row menu has
**one Equip entry per slot it could go in** -- its own, Custom, and the off
hand for a held thing once the main hand is taken (`slotsFor` in
`domain/equipment.ts`; alone, a held item is seated in the main hand whatever
was asked, so the entry is not offered). A worn item's menu takes it off or
drops it. A row also has **Use** on a
consumable (one fewer, and nothing else yet -- no potion takes effect), and
Drop on anything, offered as *Drop one* and *Drop all* once there is more than
one. No count stepper: a dozen torches is still one row with one menu. The
purse is five fields, labelled with the coins' full names -- Copper, Gold --
because "cp" and "зм" are rulebook shorthand a new player does not read.
A reader of somebody else's sheet gets the same five fields, read-only, rather
than a line of text ("Gold: 15") that would be a second drawing of one fact.
Every edit is one `change` event on `equipment.*` paths, appended to the log.
Equipping writes the equipped list both whole and per slug -- see the comment
on `equippedChanges` for why the server needs both -- and into Custom, the
placement as well.

### What the sheet says about an unfinished character

A badge on the name reading **Unfinished**, and only when `/prompts` comes back
with `complete: false` -- not "with something in it": optional prompts stay
open for ever -- an alignment, a custom spell -- so that badge would never
come off a finished character, and the build screen and the sheet would
disagree about the same response. The badge is the whole message: the sheet
does not enumerate what is open, because enumerating it puts the build screen's work on
the page nobody came to build on -- an alert, a list of five questions, and the
sheet itself pushed below the fold on a phone. The screen that answers a
question is the screen that lists it.

The mark is not the *button*. One that appeared and disappeared with the
prompts would say two things -- here is the way in, and there is work left --
and a finished character would have no way back to the build screen at all.
**Edit** is on every sheet, and the news is a badge, where a rank and "Read
only" already go.

The **Event log** has no link on the page for now -- `/characters/:id/log`
still serves it, and is still the unabridged record, but nothing in the client
links there.

A `/prompts` that *failed* draws no badge. The request is deliberately
survivable -- a sheet is worth drawing with a second request down, which is the
same bargain the compendium lookups make -- and the honest reading of "I do not
know what is open" is to say nothing rather than to guess. Edit does not depend
on that answer and is drawn either way.

### On a phone the sheet is a deck, not an accordion

The same tabs under the character's name, one on screen, and a swipe
between them. **Nothing opens and nothing closes**: the carousel decides what is
visible, so a tab has no shut state to be in.

It is not an accordion, and the two answer different questions. An accordion
is right for a page of two or three panels where the answer is usually in the first and the rest are detail. A character
sheet is not that shape: its tabs are peers a player leafs between at a table.

The strip and the carousel are `ui/TabDeck`, which the build screen's stage tabs
also draw -- see [the tabs are a
deck](#the-tabs-are-a-deck-so-a-phone-can-swipe-between-them). It scrolls away
with the page rather than pinning under the header: one fewer row of chrome on a
screen this app has already spent an argument buying back (see [Two views, one
codebase](#two-views-one-codebase)), and a swipe changes tab from anywhere on
the slide, so the strip is not the only way through.

**The strip sits on a bar of its own, and nothing on a tab sits on the page.**
The sheet's tabs are the page's own rather than something inside a `Panel`, so
drawn plainly their captions lay straight on the backdrop's doodles with a
hairline under them -- and so did the Actions and Spells lists beneath. Build
and Group never showed it, only because their `TabRow` happens to be inside a
`Panel`. So the sheet passes `bar`, which `TabDeck` hands to `TabRow`: the strip
is drawn on the same bordered `Paper` a `Panel` is, and the list's grey rule is
dropped because the bar's border already draws that line. It is a flag rather
than the default, because a bar inside a `Panel` is a box in a box -- which is
why the Items tab's inner row does not pass it and sits in a `Panel` with
its rows and the purse instead. Every tab's content is in panels for the same
reason: two surfaces, the bar and then what it selects.

**The deck is as tall as the slide that is showing.** Every other slide is
given `height: 0` and left to overflow, so the viewport is sized by the one
slide with a height and clips the rest to it -- a neighbour is still drawn
sliding in during a swipe, cut at the foot of the panel being left. Nothing is
measured: the alternative, a `ResizeObserver` sizing the viewport, reads a
layout happy-dom does not compute, so the suite could neither exercise it nor catch
it breaking. Sized to the tallest slide instead, a swipe from the foot of
Overview would land a long way down a mostly empty Actions with the tabs
off-screen above.

**And exactly as wide as the deck.** Each slide carries `min-width: 0`. A flex
item does not shrink below its content, so a single row too wide for a phone
-- a weapon's name beside its three figures -- would widen its whole slide, and
embla, which centres a slide, would rest with both of the panel's edges cut off.
A row that does not fit is that row's problem: it wraps, as the weapon
row and a page's header actions do.

### What a phone spends its height on

Two of the sheet's measurements are chosen for a phone, which is where they
cost most:

- **The card grids gap at `xs` from `base` and `sm` from `sm` up** -- the ability
  row and the vitals rows.
- **Every card on the sheet is `padding="xs"`, at both widths.** This is the one
  that is not responsive, and not for want of trying: Mantine's `Card` takes
  `padding` as a plain spacing value where `SimpleGrid` takes a responsive one,
  so there is no way to say "tighter on a phone" without a viewport branch in
  three components. Four pixels a card is most of what a phone gets back here --
  six vitals rows, an ability row and the identity table -- and on a wide screen
  it is two pixels a side on a page that has room to spare either way. The three
  have to move together whatever the value, because the identity table's
  alignment above depends on all of them agreeing.

None of this is assertable: the suite runs without CSS, so a spacing prop is a
change no test can see. That is the honest reason there is nothing pinning it.

**The main section reads the same at every width: who the character is, then
the ability cards.** A phone that opened on the six modifiers first -- on the
argument that a slide should open on what is reached for mid-turn -- would
make this the one place on the sheet whose order depended on width, and would
open a character's page on six numbers before saying whose they were.

Both sheet screens get this, because there is only one of them to get:
`SheetBody` is drawn by the owner's sheet and by the one a group member opens
for a character shared with their table, and the whole point of that split is
that the two cannot disagree about what a sheet is.

`features/character/SheetBody` prints ability scores and saving
throws as a sheet does -- STR, DEX, CON, INT, WIS, CHA -- and it has to impose
that itself, because the API cannot say it. Both arrive as objects keyed by
slug and a Go map serialises its keys sorted, so a screen that walks the
response as it came prints CHA first. The order lives in `domain/format.ts` as
`ABILITY_ORDER`, and anything drawing more than one ability in sequence walks
it through `abilitiesInOrder` rather than walking the response -- the ability
scores form's six inputs included. Walking a fixed list against a projection
that may not match it also keeps a slug the six do not cover, drawn last rather
than filtered out: an unrecognised ability means the server and this client
disagree about the game, which is a thing to see rather than a thing to hide.

**The saving throw is drawn inside its ability's card**, under a rule, rather
than in a panel of its own further down. A save is an ability check the
character may be trained in, so printing the two a screen apart asked the
reader to carry a modifier between them, and the separate panel spent six rows
repeating the six labels the cards had already given. Merged, the two cannot
fall out of alignment, because there is no second list to align. Training is
the same `ui/ProficiencyMark` the skills use -- one vocabulary for "you are
proficient in this" across the whole sheet.

That decides what a *missing* ability means. Scores and saving throws are two
projections and neither promises all six keys, and the card is the only place
either is drawn, so dropping a card with no score would swallow a save the
server did send. So the grid walks the **union** of
the two (`abilitiesOnSheet`), and a card with no score prints a dash where the
modifier goes and still prints its save. It claims nothing it was not sent, and
hides nothing it was.

Above them, `features/character/IdentityTable` says who the character is as
labelled pairs, in **four columns of two**: name over level, race over subrace,
class over subclass, background over experience. One dimmed line under the
name ("Elf · Wizard 1") reads well and answers badly: a line has no room for the subrace or the subclass, and a reader looking
for one of them has to know the order it was written in.

The pairing is the layout rather than a consequence of it, and the columns are
nested for that reason rather than being eight cells in one flat grid. A subrace
is a qualification of a race and means nothing on its own; so is a subclass of a
class, a level of the character it belongs to, and experience of the background
it was earned past. Flat, that pairing held at four columns and broke at two,
where "Class" landed under "Name" and "Subrace" under "Class" -- an arrangement
that reads as a claim about the character. Nested, it holds at four columns, at
two and at one. **Two columns is what a phone gets**: eight one-word fields down
a single column is most of a screen for the shortest thing on the sheet.

The table is drawn in **the same card the ability scores and the vitals are**,
and that is an alignment fix rather than decoration. A bordered card insets what
is inside it by its border and its padding, so a bare table above a row of cards
puts its labels that much to the left of theirs: two columns of small dimmed
labels down one page, not lining up. The two ways to fix that are not equal.
Padding the table by hand writes the measurement down as a number, and it is a
number nothing keeps true -- the day the card's padding changes, the only block
on the sheet that does not follow is this one. It has already changed once, when
the three of them went from `padding="sm"` to `padding="xs"`. Matching the
container is exact by construction and cannot drift.

**Every field is drawn even when empty**, showing
`--`, because "not chosen yet" is the answer to the question and a missing row
is not -- on a half-built character the blanks are the most useful thing on the
page. Names come with the sheet, in `catalogNames`: one map keyed
`"<collection>:<slug>"`; keyed by collection as well as slug because two
collections may share one, and a bare slug map would let a background rename a
class. Without it the table falls back to title-casing, and "half-elf" becomes
"Half Elf" rather than "Half-Elf".

**The sheet asks the compendium for nothing.** The server has the character's
catalogue in hand when it projects the sheet, so it resolves every slug there
and sends the answer in the same response: `catalogNames` for what the sheet
only names, and `catalog` -- skills, proficiencies, equipment, magic items,
spells -- for the entries a panel reads more of, limited to what this character
references. See [the resolved sheet](backend.md#the-sheet-arrives-resolved).
Opening a sheet is two requests, the sheet and its prompts, and a refresh after
an inventory edit is one.

Doing that lookup in the browser would mean fetching fourteen whole
collections, one of them every spell in the rules with its artwork inlined:
about 11 MB to print a dozen names. **No screen may download a
collection in order to name a few of its entries, and nothing may download the
whole spell list at all** -- a client gets the spells it names or one page of a
search. The sheet's own spells are the first case: they arrive resolved, icon
included, and the Spells tab (`features/character/SheetSpells`) draws them as
the build screen's rows -- icon, level, concentration and ritual tags, school,
casting time, components -- under a heading per spell level. What arrives is a
summary without the spell's text; opening a row fetches that one spell by slug
through the screen's catalogue scope, `/characters/:id/catalog` on the owner's
sheet and `/shared/:id/catalog` on the one a table reads, and shows it the way
the build screen does: inline below the row on a wide screen, as a full-screen
view with a Back button on a phone. The school and class names come from the
same scope's `spell-filters`, one small request per tab.

A class that prepares spells -- cleric, druid, paladin, wizard -- prepares them
on its owner's sheet. The tab finds the class's `prepared` question again the
way the build screen reposes a saved answer (the entry in the log that answers
`<class>/spell/prepared/<level>`, then `/prompts?before=` for the question as
it was asked; or the open prompt when it has never been answered) and draws
two lists: what is prepared, with Remove, and what could be, searched,
filtered and paged by the server from the question's options exactly as the
build screen's available list is, with Add. Spells the rules prepare on their
own (a domain's) are in the first list without a Remove. Every Add or Remove is
one write, as taking off a piece of armour is: an append the first time, a
replacement of the answering entry after that, and a delete when the last
prepared spell goes, because an empty up-to answer is one the server treats as
unanswered. There is no draft and no dry run -- nothing in the log depends on
a preparation answer, so there is never anything a change could drop. A shared
sheet shows the same rows and opens the same text, and nothing on it can be
pressed to change the character.

Under the cards is a second headline row, `features/character/Vitals`: passive
Perception, the spellcasting numbers, speed, vision and Hit Dice. Hit Dice are
here rather than beside the backpack because they are a fact about the body,
not about the kit.

**Three lines of six**, the width of the ability row, so every character's
sheet puts a number in the same place and a reader never hunts for one. The
line breaks are meaning rather than wrapping: abilities, then the body's state
-- hit points, the temporary pool on top of them, the Hit Dice that refill
them, then armor class, initiative and proficiency -- then what the character
can do at range. Hit points, temporary hit points and Hit Dice lead the second
line together because they are one subject, and reading them apart means asking
the same question three times.

**A caster gets three cards, not one** -- attack bonus, save DC and the ability
behind both are three questions asked at three different moments, and a spell
that attacks never wants the DC. A character who casts nothing keeps all three
and reads `n/a`, because a row that changes length between characters costs
more than three quiet cards.

The two absences are deliberately different words. **`n/a` is "this does not
apply to you"** -- a barbarian has no spell save DC. **`--` is "this applies
and is not known here"**, which is what an unset speed is. Neither is a zero,
because `0 ft.` is a claim and temporary hit points of nought is a real answer
that has to stay distinguishable from both. The sense names its own card --
"Darkvision / 60 ft.", because "Vision / 60 ft." says less and the label is the
half with room for the word.

The skills beside them are a different case. `features/character/SkillsPanel`
draws **all eighteen**, ordered by how trained the character is — Expertise,
then proficient, then half, then untrained, alphabetical within each block.
Eighteen rows of which six matter is a
different problem: the question is what the character is good at, and the
answer should not be scattered down a list of things nothing trained. The
alphabet still breaks ties, so each block stays stable and searchable.

It draws eighteen rows because the **server sends eighteen**, not because the
client fills in the gaps. The untrained skills arrive with their bonuses
already computed (see [dnd.md](dnd.md#the-projected-sheet)); this panel adds
nothing up. Unioning the sheet against the compendium and adding an ability
modifier here would be the browser computing a rule, which
[`domain/format.ts`](../web/src/domain/format.ts) exists to forbid — and it
would be wrong the day Jack of All Trades starts halving a bonus.

What the compendium *does* supply is each skill's **name and governing
ability**, which arrive in the sheet's own `catalog.skills` -- all eighteen,
because the panel draws all eighteen. The name matters twice: it is in the
negotiated locale, and it is the only spelling that gets "Sleight of Hand"
right, where title-casing the slug capitalises the "Of". A sheet without them
-- the one a write echoes back carries no `catalog` -- loses the ability tags
and falls the names back to the slug; it does not stop the panel drawing.

Training level is carried by a mark — `ui/ProficiencyMark`, one glyph filling
in across the four levels, with Expertise ringed rather than merely fuller
because it is the bonus counted twice rather than more training. The mark is
what separates the rows, and the dimming of untrained ones is a **second**,
redundant channel: a panel distinguishing eighteen rows by a shade of grey
reads to nobody on a monochrome print and to nobody who cannot tell the two
greys apart. **There is no filter over the top of it.** A "Hide untrained"
toggle would answer the question the eighteen rows exist to answer -- what do
I roll for a skill nothing trained -- by taking those rows away. The two
channels above already separate the trained from the
untrained without anything to press first.

Skills stays at half width rather than spreading: its rows are name, ability and bonus, and
a full page width would set the bonus so far from the name that the eye travels
back along an empty line to pair them. **`features/character/ProficienciesPanel`
takes the half beside it** -- everything the character is trained with that is
not a skill or a save, grouped into Tools, Weapons and Armor. A comma-joined
paragraph at the foot of "Traits and features" would be a sentence to be read
rather than a list to be searched, and would file a tool a player rolls with
beside a racial trait they never touch again.

**Consumables have their own panel.** Everything a character spends -- spell
slots by level, Pact Magic, Channel Divinity, ki -- is drawn by
`ResourcePools` as rows of marks in a "Consumable slots" section, present only when
the character has a pool. It is the same component the game page unfolds, here
without `onChange`, so it is read-only and always full: what has been spent is
a fact about one game.

**"Traits and features" is a list of rows that open**, one column at every
width, drawn by `ui/BlockList` as the Actions tab is. Not a grid: a grid row is
as tall as its tallest cell, and one name long enough to wrap -- "Wild Shape
(CR 1/4 or below, no flying or swim speed)" -- pushes its short neighbour away
from the rows around it, so the list reads as uneven.

**A row opens where it stands onto what the entry says.** A player who reads
"Fey Ancestry" on their sheet should not have to leave it to learn what that
is. The prose arrives in the sheet response as `catalog.traits`,
`catalog.features` and `catalog.languages` -- only the entries that have any --
so opening a row is not a request. A row with no prose is a statement, not a
control: a 2014 entry outside the SRD on a server without the private overlay
pack, or a scaling value no feature is named for. One description is open in
the panel at a time, not one per group. A statement and a closed row are the
same height, which is `BlockList`'s doing and holds on the Actions tab too.

**A group that does not apply is left out
rather than drawn empty** -- a rage counter on a character who cannot rage is
not a fact about them -- where a group that applies and is empty says so, which
is why an empty backpack still draws "Empty." That is the same distinction the
vitals row draws between `n/a` and `--`.

**The bonus is printed on tools and on nothing else**, and the reason is worth
stating because it looks arbitrary. A tool check is an ability check, but
*which* ability depends on what is being attempted -- picking a lock with
thieves' tools is Dexterity, spotting a forgery with a forgery kit is
Intelligence -- so the only part of the number fixed in advance is the
proficiency bonus, which is exactly what a sheet can usefully print. A weapon's
attack roll has a fixed ability, so a bare proficiency bonus would be the less
useful half of a number this panel is not showing; armor proficiency adds
nothing to any roll at all, and only stops the penalties. Nothing is computed
here either: the number is `status.proficiencyBonus` as the server derived it,
and the *type* that decides which rows get it comes from the sheet's
`catalog.proficiencies`, resolved by the server beside the skill names.

The stat row above leads with hit points, then armor class, initiative and
proficiency. Hit points are the one number that moves between one glance and
the next and the one reached for mid-turn, where armor class is settled at the
start of a fight; at two columns on a phone, first position is the only one
visible without the eye travelling.

## The log has its own page, and it never asks for the sheet

**Nothing links to it at the moment** (see [what the sheet says about an
unfinished character](#what-the-sheet-says-about-an-unfinished-character)), so
the route is reached by typing it. The screen and its route are kept whole:
it is the one page that can answer "why do I have this proficiency?" when the
projection has gone wrong.

Its breadcrumb trail is `Characters / Event log`, where every other detail
page names the thing it is about. That is the same rule, applied one step
further: the character's name lives on the sheet, and fetching it for a crumb
would reintroduce exactly the dependency this page exists without. The trail
says less instead. `/characters/:id/build` is the asymmetry worth noting -- it
already holds the sheet, so it gets `Characters / Ada`.

`/characters/:id/log` draws the character's event log: one row per stored
event, in sequence, with the kind read back as prose, the time the server
stamped and the payload -- references, answers, changes -- as the log holds
them. It is the page [dnd.md](dnd.md) implies by justifying event sourcing on
the grounds that it makes "why do I have this proficiency?" answerable; until
something drew the log, that claim could not be checked from a browser.

`features/character/CharacterLogScreen` deliberately does *not* fetch the
sheet, not even for the character's name in its header. The sheet is a
projection of this log, and a projection that has gone wrong -- a slug the
compendium no longer has, an event that should not have been appended -- is the
circumstance in which somebody opens the log in the first place. A page that
fails whenever the thing it exists to diagnose fails is no use. The compendium
lookup that turns `race:half-elf` into "Half-Elf" is held to the same rule: it
is one request per collection, and a failure yields slugs rather than an error.

Nothing on the page writes. Changing a decision happens on the build screen,
where the thing being changed is in front of you: one entry is replaced and
everything after it revalidated. The log is where you come afterwards to see
what that cost.

## A session that ends while the page is open signs the page out

`AuthProvider` asks who is signed in once, on mount. A session can end after
that -- it expires, or the server restarts with a new signing key, which a
development server does on every restart -- and the first anybody hears of it
is a 401 on whatever a screen asked for next. Left to each screen, that is
reported as its own failure ("Could not load your folders -- That is not
valid. Start again.") with the player on a private page that can no longer
work.

So the API client says it once: `onUnauthorized` in `lib/api/client.ts` calls
its listeners whenever any request is answered 401, and `AuthProvider`, while
somebody is signed in, answers by becoming anonymous with the session-expired
message. Every private route then does what it does for an anonymous visitor
and shows the landing page with its Log in button. The screen's own request
still fails, but nobody is left looking at it.

## Private routes branch, they do not redirect

`routes/Private.tsx` renders the landing page to a signed-out visitor instead
of navigating away, which is the same rule `HomeRoute` follows: the URL never
changes on account of who is looking, so a shared deep link to a character
survives being opened by someone who has not signed in yet.

`routes/AdminOnly.tsx` is the same shape with a different question and a
different answer: a superadmin gets the screen, everybody else -- signed out
included -- gets the not-found page, which is what the server says to their
requests too. It reads `user.admin` from the session and is a courtesy, not
the guard.

### An invitation link is the deep link that arrives at a stranger

Every other private route is followed by somebody who already has an account.
An invitation is the opposite: it is *sent to people who do not*, which makes
it the one link that must survive a whole sign-up before it can be used.

The token rides in the **URL fragment** -- `/groups/join#<token>` -- because a
fragment is never sent to any server, so it stays out of nginx's access log,
out of `Referer`, and out of any link unfurler that fetches the URL. It is then
posted in a request body, never in a query string. That is the safest place to
carry it and the least durable one, and three things would lose it:

1. **`<Private>` branches, so the screen never mounts.** Whatever `JoinScreen`
   did to save the token would run only for visitors who are already signed in
   -- precisely the ones who do not need it. `/groups/join` is therefore *not*
   wrapped in `Private`: `routes/InvitationRoute.tsx` captures the token first and
   branches afterwards, the way `HomeRoute` does for `/`.
2. **`/login` is a different URL.** Returning to `from.pathname` drops the
   search and the fragment, so an invitation link comes back as a bare
   `/groups/join`. `LoginScreen` rebuilds `pathname + search + hash`, and
   ignores a `from` that is not a path of ours -- history is attacker-reachable.
   Its "already signed in" branch goes to that same `from`, not to `/`: that
   render and the post-sign-in `navigate` both fire when a sign-in succeeds,
   and if they disagree the root wins, leaving the invitation behind.
3. **Google leaves the origin entirely.** Nothing in a URL survives that, and
   it cannot: `currentPath()` sends `pathname + search`, and the server refuses
   any `return_to` containing `#` outright (`SafeReturnTo`). So the token is
   also copied into `sessionStorage` by `features/groups/inviteToken.ts` --
   session, not local, because an invitation is one visit's business and one
   left behind in a shared browser is somebody else's group.

Passkey and guest sign-in are `fetch` calls that never leave the page, so they
would need none of the above; Google is the one that does. One mechanism
covering all three is less to get wrong than two.

A signed-out visitor gets `InvitePrompt` rather than the bare landing page:
they followed a link somebody sent them on purpose, and a carousel about the
app does not tell them the thing they came for is one button away. It cannot name
the group -- previewing needs a session, and opening that up so a stranger
could read a group's name off a link is not a trade worth one sentence.

**Every invitation is accepted on one page, `/invitations`**, which is a
section in the menu at both widths -- drawn under the menu's rule, directly
above the account (`belowRule` in `ui/sections.ts`): it has a trail of its own
like any section, but it is an errand rather than a place. The two addresses links are sent as,
`/groups/join#<token>` and `/characters/receive#<token>`, are
`InvitationLink`: it saves the token under its kind's key and replaces itself
with `/invitations`, carrying the fragment and, in router state, the kind the
address named. `InvitationRoute` then draws `JoinScreen` or `ReceiveScreen`
unchanged. Reached from the menu with nothing waiting, it draws a field to
paste a link into instead. An installed app has no address bar and is not what
a link in a messenger opens -- the link opens the browser, signed in as nobody
or as somebody else -- so without the field, following an invitation from the
app would mean not using the app. `tokenOfLink` takes what follows the pasted text's last `#`, or the
text itself when only the token was copied; `kindOfLink` reads the kind from
the address in the pasted text, and failing that from the token's own `knd`
claim, which the server checks and the client reads only to choose which
preview to ask for. Under either screen is "Paste another link", which forgets
the invitation and returns to the field, and is the only way on from a link
that was refused: without it an invitation saved by an earlier visit would sit
on the menu's page for the rest of the session. The alerts carry no "Your
groups" or "Your characters" button, which would lead away from the page the
menu had just opened.

**A copy link is the second such link**, and it takes the same road rather
than a parallel one: `/characters/receive#<token>` lands on the same `routes/InvitationRoute.tsx`,
the token is stashed by the same `inviteToken.ts` under a key of its own (so it
and a pending group invitation cannot overwrite each other), and the signed-out
visitor gets `InvitePrompt` with its `character` wording. `ReceiveScreen` shows
the character's name, and accepting lands on the new sheet.

### Copying the invite link

A link nobody can copy is not a delivery mechanism, and the clipboard is the
second thing after passkeys to disappear outside a secure context: served over
plain HTTP on anything but localhost, `navigator.clipboard` is `undefined`.

Mantine's `CopyButton` is not the answer. Its `useClipboard` hook does notice
-- it sets an `error` -- but `CopyButton`'s render prop passes only
`{ copy, copied }` and drops it, so the button stays on "Copy link", nothing
reaches the clipboard, and nothing says so: working on production and failing
in dev, which is the worst arrangement of those two outcomes.

`lib/clipboard.ts` returns **whether it worked**: the modern
API when there is one, `document.execCommand('copy')` over a throwaway
selection when there is not -- deprecated, and the only thing that predates the
secure-context rule -- and `false` when neither is possible. The sheet then
says so and selects the link, so there is something the keyboard can still do.
Never a control that quietly does nothing.

## Signed out and signed in, one build

There is one hostname, one bundle and one route table. `shell/RootGate.tsx`
branches on the auth state and picks the chrome: a loader while the session is
still unknown, `LandingShell` when anonymous, `RootShell` when signed in. At
`/`, `routes/HomeRoute.tsx` makes the same choice about the content.

Signed out, that content is a carousel of three panels -- build a character,
join a group, run an adventure. Those three are the whole of what this app means
to be, in the order you meet them.

Two of the captions in `routes/LandingPage.tsx` describe what the app does; `Run sessions` describes
intent, because the battle tracker is not built. That is the one to keep honest
-- a landing page promising it would be the only thing on easydnd.org that did.

It is also the one place in the project that uses **session** for a sitting at a
table. Everywhere else that is a *game*, and a session is being signed in -- the
distinction the README draws and the app's own navigation keeps. The landing page
is read by somebody who has neither, and to them "session" is the evening being
described; the word stops at the sign-in boundary.

Two of Mantine's own defaults are overridden, and both for the same reason --
they are drawn for a carousel of photographs and this is a carousel of text on
paper. The indicators are white at `0.6` opacity, which over a pale panel on a
pale page is invisible; an invisible indicator is worse than no indicator,
because it says there is one panel. They are repainted in the primary colour,
which reads under `defaultColorScheme="auto"` where white does not. And the
controls are `44px` rather than the default `26px`: on the viewport that draws
them they are the only way through for a visitor not using the arrow keys, they
sit *over* a panel rather than beside it, and 26px is under every published
minimum for a pointer target.

They are drawn on desktop **only**. This is one of the few places outside a
`@/ui` primitive that calls `useIsDesktop`, and it asks about the *input*
rather than the layout: a phone has no pointer, so two 44px arrows covering the
panel they sit on would duplicate a swipe the screen already offers. Taking
them away removes a control, not a way through -- the swipe, the arrow keys and
the indicators all remain, and the indicators are what still say how many
panels there are.

**The arrow keys and the wheel move it too**, which is `ui/carouselGestures.ts`.
Mantine gives a carousel a swipe, a pair of arrow buttons and indicators that
answer the arrow keys *once one of them has focus* -- and none of that reaches
the visitor who has neither touched the page nor tabbed into it. On a laptop the
two obvious ways to move a carousel that fills the window are the arrow keys and
the wheel, and by default both do nothing.

Both are **borrowed rather than taken**, which is the whole of the hook. The
wheel is read on the axis the gesture is actually on, and claimed only when the
page has nowhere to scroll on that axis: a sideways trackpad swipe moves the
carousel when nothing scrolls horizontally, a plain mouse wheel moves it when the
page already fits, and the moment the page has scrolling of its own to do -- a
short landscape phone where the carousel hits its `320px` floor and the page
grows past the viewport -- the wheel goes straight back. A carousel that ate the
wheel unconditionally would be a page you cannot scroll. One gesture is one
slide: a flick fires dozens of events, so the rest of a gesture is read and
thrown away for 400ms, and `preventDefault` still runs during that window so a
sideways swipe cannot trigger the browser's back-navigation.

The keys are borrowed on the same terms. Focus inside the carousel is left
alone, because the indicators are a roving tabindex that already answers arrows
there and handling them twice moves two slides per press; a field being typed in
keeps its own arrows. The listener is on the window rather than the carousel,
because "the carousel is what this page is" is the case it exists for, and the
effect only binds once there is an engine -- so a page without a carousel binds
nothing. It lives in `ui/` and returns `getEmblaApi` as a prop to spread, so
`routes/LandingPage.tsx` never has to name `EmblaCarouselType`: only `ui/` may
import the engine. `ui/TabDeck` deliberately does **not** use it -- it sits on
pages that scroll, which is exactly the case the guard hands back.

None of the three panels is a link, and not because two of them lead nowhere --
`/groups` is real. It is that all three live behind the sign-in boundary, so a
panel that navigated would bounce a signed-out visitor straight back to this
page; the header's "Log in" stays the only control, and it carries them where
they were going.

### A fourth panel, on a phone, that you can press

There is a fourth panel below the breakpoint: a d20 you can pick up and throw
-- grab it, fling it, and it caroms off the edges of the panel and settles on a
number. It is the only thing on this page that does anything, and that is the
argument for it.

**The panel carries its heading and no caption.** The scene takes the whole panel
and the heading floats over it, so the die rolls behind its own name; there is
nothing to say about a toy that throwing it does not say. Stacking the two
would cost the die a heading's height on the one viewport where
the panel is narrowest, and a d20 with less room to travel is a duller throw. The
heading is `pointer-events: none`, without which it would be a dead strip across
the top of a toy that is grabbed and flung with pointer events on the canvas.
The panel is named by that heading the way the other three are, and the
padding is on the heading rather than on the panel so the scene still reaches
the edges it bounces off.

Every section of the app is behind sign-in, so a curious visitor has nothing to
try -- and a die is the one piece of this product that works with no account,
no character and no table. It is a toy and not a preview: it rolls, it says
what it rolled, and it keeps nothing.

Phone only, and for the same *kind* of reason the arrows are pointer only --
this asks about the input, not the layout. A die is a thumb toy. On a desktop
it would be a large ornament clicked with a mouse, on the page where somebody
is deciding whether to sign up, competing for that decision with the three
panels that say what the app is for.

It goes last rather than among the three. The panels are the app's pitch in the
order you meet it, and a toy wedged into that sequence interrupts an argument
to offer a distraction. `LandingPage.test.tsx` pins the order on a phone for
that reason, and pins the die's absence on a wide screen.

The test `offers nothing to press on the slides that describe the app` is
worded that way on purpose. Its claim is that no panel is a *door* -- a
carousel where one panel navigates and two do not teaches the wrong thing --
and a die leads nowhere, so the claim holds on the three it is about.

Each panel is named by its own heading, through `aria-labelledby`, rather than
carrying a second copy of the words in an `aria-label`: two spellings of one
name is only how they come to disagree. Reachability *by name* is the
accessibility contract -- a slide that announces itself as "slide 2 of 3" is
unusable without sight -- and `LandingPage.test.tsx` pins the wiring and not merely the name, because an id
that stops resolving turns every panel back into "Carousel slide" without
failing a test that only looked one up.

### A sheet for the content to sit on

The pattern is a background, so anything laid straight onto it competes with it:
a table's header rule and its row separators are hairlines, and hairlines are the
first thing a busy ground eats. `ui/Panel` is the answer -- the `Paper` the
character list's folders use, generalised so a screen asks for one
by name instead of spelling out three props. It fills with
`--mantine-color-body`, so the pattern stops at its border in both colour schemes
with nothing said about either.

Screens take one for their content: the group list, the game list, a group's tabs, a game's
roster, and the whole of character creation. What stays outside it is the page's
heading, trail and actions -- they say where you are, and where you are is not
part of what you are looking at, which is also why this is not something `ui/Page`
does for every screen.

### The pattern behind every page

`ui/backdrop.ts` tiles a sheet of hand-drawn marginalia -- dice, swords, scrolls,
a dragon -- behind the main box of all three chromes, washed down to almost
nothing. Without it the app is flat theme colour everywhere except the landing
carousel, and signing in is a cut from three photographs to a blank sheet.

A photograph is the wrong kind of picture for the job:
it has a subject, and a subject behind a table of characters competes with it. A
pattern has none. It is texture, it repeats, and one 170KB tile serves every page
at every size -- where a photograph has to cover a viewport.

The wash is `--mantine-color-body` at 88% via `color-mix`, not a pair of `rgba()`
literals, and that is what makes one declaration serve both colour schemes: the
variable is white in the light one and near-black in the dark, so the drawing is
lightened or darkened towards whichever page it sits on and every foreground
colour keeps the contrast it was chosen against. In the dark scheme it inverts --
dark lines on a ground a shade lighter than the page -- which is the same drawing
and reads the same way. The tile is 1024px square drawn at 512, small enough to
read as texture rather than illustration.

It goes on `AppShell.Main` only. The header and the desktop navbar keep their own
flat grounds: they are chrome sitting over the content, and one sheet of pattern
running under all three would read as one surface.

**Every page takes it, and there are no exceptions** -- not the die's screen
at `/roll`, though a canvas with its own lit floor is already a picture, and
not the landing carousel, though it is three photographs. One ground under
every page is worth more than either argument, and at an 88% wash what shows
through behind a canvas or between two slides is texture rather than a second
picture. So the three shells spread `PAGE_BACKDROP` directly, with no lookup
by path and nothing for them to keep in step.

`background-attachment` is `scroll` rather than `fixed`: iOS Safari sizes a fixed
background against the document rather than the viewport, and a repeating tile
has no framing to hold still anyway.

### The art in the panels

Each panel is filled by a photograph of its own -- `src/assets/landing-valley.webp`
behind "Build a character", `landing-ship.webp` behind the group,
`landing-volcano.webp` behind the adventure. One picture per panel rather than
one behind the carousel, because the art belongs to the thing the panel is about
and should travel with it as you swipe. The die keeps its own background -- its
scene is a canvas, and a photograph behind that is two pictures in one box.

**Nothing is laid over the picture. The contrast is on the letters instead.** A
scrim defeats what the art is for -- a photograph behind a sheet of translucent
white is not a photograph. The answer is a halo: each
panel's words carry a three-stop `text-shadow` in the opposite colour, which is
as wide as the letters and touches nothing else. The stops run tight to loose,
because the tight pair is what separates a stroke from a busy background (a lava
flow is high-frequency, and one soft shadow washes across it without ever getting
dark at the edge of a letter) while the loose one carries the block of words.

There are two variants and a panel names the one its picture wants -- `INK`,
selected by each slide's `ink`. `onDark` is white in a black halo and is what the
valley and the volcano take; `onLight` is black in a white halo, for the ship.
Chosen per picture rather than derived from the colour scheme, because a
photograph is neither light nor dark, and these are not even the same kind of
picture.

Darkening the photograph under the words was tried on the valley and reverted.
It is the opposite trade -- the words are left alone and the picture pays -- and
the picture is what the panel is for. The halo stays: it is as wide as the
letters and the photograph is untouched everywhere else.

The pair is set once on the stack, which is also what keeps the caption's usual
`dimmed` grey -- unreadable over any of them -- off the panels that have a
picture. It stays on the ones that do not.

**Where the words sit on the picture.** The block starts a quarter of the way
down the panel, and on a wide screen a third of the way across it: dead centre
puts a paragraph over the middle of the photograph, which is where its subject
is. The two fractions differ because they answer different questions -- the
horizontal one is about where the subject stands in these pictures, the vertical
one about a block that grows downward from its heading and should not end up in
the lower half.

The vertical quarter is a *fixed* `flex-basis: 25%` spacer above the block, which
is what puts every heading in the carousel on one line. The panel carries no
padding of its own at either width -- the words hold their own inset instead --
so that quarter is a quarter of the panel rather than a quarter of what is left
inside its padding, which is not the same fraction on a phone as on a laptop.

Below the breakpoint the spacer is a sixth instead. A phone's panel is a tall
column holding one block of text, so a quarter of it is a screenful of picture
before the first line -- and the top edge itself leaves the heading nothing to
sit against. Sharing the leftover space between two flexible spacers would
measure from the middle of a block whose height is the length of its own
caption, so the three headings would sit at three heights and jump as you
swipe. `flex-basis` in percent
is read against the panel's height, unlike a percentage padding, which resolves
against width even when it is `padding-top`. The horizontal third is still a 1:2
pair of flexible spacers, where nothing has to line up between panels. Below the
breakpoint the block spans the panel instead -- a third of 390px is not a margin,
it is a column too narrow to set type in.

Caption and heading are both ranged left, on one edge. Justified, at four or
five words to a line the word spaces stretch visibly, no two lines set the same
colour, and over a photograph every gap shows a different piece of picture. A
ragged right edge is the cheaper thing to look at, and a centred heading reads
as a heading belonging to something else. A fourth photograph takes whichever
variant suits it; the upgrade, if two ever stop being enough, is sampling the
image rather than adding a third.

Each is `cover` and pinned to the **top** edge, not centred: the panel is taller
than the pictures' aspect at most sizes, so something is always cropped, and
these three are framed to be read downward from the top. Centring, the default
worth having for a photograph nobody composed for this box, takes the crop off
both ends instead.

The files are 1920px wide WebP, around a quarter of a megabyte each, imported
rather than dropped in `public/` so Vite content-hashes them and the immutable
cache rule in `deploy/nginx/easydnd.conf` applies. One size for every viewport;
a `<picture>` with a narrow variant is the upgrade if transfer size on a phone
ever becomes a complaint. The art is decorative and carries no accessible name:
the headings say what the page is.

**The wordmark is a link home**, in all three shells, which is what a logo in
that corner has meant since there were corners. It leads to `/` on both sides of
the boundary -- the carousel signed out, the character list signed in -- and it
carries no link styling, because the mark and the word are its appearance and a
blue underlined "easydnd" would be the browser's default showing through.

It is why `/legal` has no "Back to easydnd" button. That would be a second way
home drawn on exactly one page, sitting in the corner `SignInActions` keeps for
the way *in* rather than the way out; a licence notice should not need chrome
of its own to get out of. `SignInActions` offers a signed-in visitor no "Log
in" there.

The carousel fills
`AppShell`'s main content box -- `100dvh` less the header offset, the footer
offset and twice the shell padding -- with a `320px` floor, because `height` on
a carousel is a hard height and a short landscape phone would otherwise squeeze
three panels into a couple of hundred pixels. Every term is
`AppShell`'s own custom property, so the header height and the footer height in
`shell/LandingShell.tsx` are each stated once. One trap worth naming: Mantine's
`rem()` passes a length through untouched only when it begins `calc(` or
`clamp(`, so the whole expression is wrapped in `calc(...)` rather than being a
bare `max(...)`, which would be split on its spaces and mangled.

There is no "this browser cannot use passkeys" warning, here or on `/login`:
the passkey card is simply absent there, and what is left on the page -- a provider, a guest -- is
what that browser can actually do. An alert explaining an option nobody was
offered apologises for a hole the visitor cannot see.

### The footer says whose this is

The landing chrome has a footer, and only the landing chrome does. It exists
for one of the three things it carries: the SRD 5.1 data this app is built on is
CC-BY-4.0, and that licence expects its attribution *in the product*. `/legal`
is that attribution, the footer is how anybody reaches `/legal`, and
the source link and the build are the two things that belong beside it once
there is a line to put them on.

It is a `Group` and not a `<nav>`, which is load-bearing rather than an
oversight. `LandingShell.test.tsx` pins that the logged-out chrome exposes no
navigation landmark -- while nobody is signed in there is nowhere in the app to
navigate to -- and two links to static documents are not the app's navigation.
The `<footer>` that `AppShell.Footer` renders supplies `contentinfo`, which is
the landmark this content actually wants.

The version is the release identifier as plain text: a tag on a release, a
short commit SHA on anything else. It is provenance rather than a diagnostic,
and it links nowhere -- there is nowhere for it to lead, and linking it would
promise a page for an arbitrary commit that nothing serves.

The cost is worth stating rather than pretending away: the signed-in shells have
no footer, so an account holder -- the person actually reading SRD-derived
material on a character sheet -- has no link to `/legal` from anywhere. Nothing
in the layout forbids it: the `AppShell.Footer` slot is free in both signed-in
shells. So it is a decision nobody has made rather than a thing that cannot be
done -- recorded in [licensing.md](licensing.md#known-gaps) rather than
quietly carried.

Branching rather than redirecting is what keeps the address bar honest: an
unauthenticated visit to a deep link does not bounce anywhere, so a link shared
with someone who has not signed in yet still works -- the moment the state
flips, the same route renders its real content.

The one navigation in the whole flow is deliberate. There are three ways in and
choosing between them needs room to say what each costs, so they live on
`/login` rather than in a header corner. The page opens straight on the cards,
with no lead sentence counting the ways in: the visitor can see the cards, and
each of them already says what it costs. Three, not four: signing in with a
passkey and signing up with one are one button -- see
[One button means both halves](#one-button-means-both-halves) -- and sign-up
asks for no display name.

`shell/SignInActions.tsx` draws a single "Log in" link that carries the
current location in router state, and `features/auth/LoginScreen.tsx` returns
the visitor there on success -- which is what preserves the deep-link property
when the way in is a page. A signed-in visitor who reaches `/login` is
redirected to `/`, since a login page inside the signed-in shell is nonsense.

The state itself lives in `lib/auth`, which asks `GET /v1/auth/me` once on
mount. The session is an `HttpOnly` cookie, so that request is the *only* way
the client can find out who is using it; nothing is inferred from storage.

It asks `GET /v1/auth/providers` alongside it. Which external sign-in buttons
exist is a property of the deployment rather than of who is looking, so it is
fetched once and never refetched -- and a failure there is swallowed, because
the passkey buttons still work and the worst case is one fewer option.

`AuthStatus` has four values, and the fourth is the one that matters:
`offline` is kept distinct from `anonymous` because being unable to ask is not
the same as being told no. Only an explicit 401 signs someone out. Treating a
failed request as a sign-out would eject people whenever the network dropped,
and would do it on every launch of an installed PWA opened offline.

Mind the vocabulary trap that leaves behind. `AuthStatus: 'anonymous'` predates
guest sessions and means **signed out**. A guest is `'authenticated'` with
`user.anonymous` set: they have a working session, it just names no account.
The two senses never meet in one expression, but they do meet in `lib/auth`.

## One button means both halves

The passkey option is a single button, and pressing it either signs you in or
signs you up. That is not a shortcut, it is the only honest arrangement: **the
browser will not tell a page whether a passkey exists for it.**
`NotAllowedError` covers "cancelled", "timed out" and "no matching passkey"
alike, deliberately, so offering "Sign in" and "Create an account" as two
presses would ask the visitor a question the platform refuses to answer -- and
strand whichever of the two they turned out to be.

So `signInOrRegister` runs the sign-in ceremony, and if the picker ends without
an assertion it runs the registration ceremony instead. Both live inside **one**
`runAuth` attempt. Chaining two would be wrong twice over: `runAuth`'s catch
would swallow the sign-in failure and report it before registration ever ran,
and `busy` would drop to false between the prompts -- a button that stops
spinning in the exact gap where the operating system is about to ask something.
One attempt means one spinner, and one message from whichever half failed last.
The sign-in failure on the way past is discarded rather than shown, because the
registration failure is the actionable one.

The trigger is `isCeremonyDismissed` from `lib/webauthn`, not a name-check on
the exception: `AuthProvider` should not be reciting spec terms, and this way
the caveat lives next to the switch that causes it. Anything that is *not* a
dismissed picker -- a 500, a dropped connection, a misconfigured relying party
-- is rethrown, because registration would fail the same way and would report a
sign-up problem to somebody who asked to sign in.

The cost, stated plainly: **a deliberate cancel is followed by a create-passkey
prompt**, since the two are the same signal. The mitigation is the card's own
copy, which says so before the button is pressed -- a confirmation dialog would
charge every first-time visitor a click to save a returning visitor a single
Escape. The server-side half of the bargain, including why a second account on
the same device is reachable, is in
[backend.md](backend.md#authentication).

Nothing here can be tested without an authenticator, so `test/webauthn.ts`
fakes one: a real class installed as `PublicKeyCredential` (the ceremony code
checks `instanceof` before trusting what came back) and a
`navigator.credentials` defined onto the existing navigator rather than stubbed
wholesale, which would break `userEvent`. It carries no `parse*OptionsFromJSON`
statics on purpose -- adding them would route the tests around the hand-rolled
decoding every real happy-dom run uses.

A guest session is one POST rather than a ceremony, so `AuthProvider` shares the
busy/error/unmounted plumbing with it through `runAuth` and lets the flows
differ only in what they await. Everything that offers account management
has to check the flag: `features/account/AccountScreen` says "You are playing
as a guest" in place of an account's ways in, and `shell/AccountActions.tsx`
*names* the control "End guest session" rather than borrowing a word that
implies you can come back. Those words are the control's accessible name and
its tooltip rather than button text, and they matter: a logout glyph is
identical either way and the difference is whether pressing it destroys
somebody's only copy.

There is no "add a passkey" flow, on either side of the wire: an account's
passkeys are the ones it was created with. `/account` therefore lists them and
offers no button -- and, having no button, drops the section entirely for an
account that has none, since a heading that can never fill is a heading over
nothing. Redundancy is a matter of connecting a provider -- see
[No recovery](backend.md#no-recovery).

`lib/webauthn` is the browser half of the ceremony: base64url conversion, the
`navigator.credentials` calls, turning a `DOMException` into a sentence worth
showing someone, and -- in `isCeremonyDismissed` -- judging which of those
sentences means "there was nothing to sign in with". It prefers the spec's own
`parse*OptionsFromJSON` where a browser has them and falls back to hand-rolled
decoding, which is the path the tests exercise -- happy-dom has neither.

## Signing in with Google is a navigation, not a request

`ssoStartUrl()` returns a **URL**, and `signInWith()` hands it to
`window.location.assign`. It is deliberately not routed through
`lib/api/client.ts`: fetching it would follow the redirect as an XHR, land
Google's consent page in a JavaScript string, and set no cookie anywhere.

The round trip means the SPA is torn down and rebuilt, so nothing survives it
except what the server sealed into a cookie. Two consequences:

- **Where to come back to** travels in the sealed flight, put there from
  `window.location.pathname` at the moment the button is pressed.
- **A failure** comes back as `/?auth_error=<code>`, because the API has no
  HTML to render. `AuthProvider` reads it in an effect, maps the code through a
  table to a sentence, and scrubs it from the URL with `history.replaceState`
  so a reload cannot resurrect it. The table is why an unrecognised code
  becomes the generic message rather than reaching the screen: text rendered
  from a query parameter is a way to put chosen words on somebody else's page.

The button itself lives on `/login`, one card per configured provider, drawn
from `providers` on the auth state. It renders first when it is offered: it is
the only way in that both keeps your characters and works on a browser with no
WebAuthn. Nothing renders when the deployment configured none, because a button
for a provider that is not there is a dead end -- the server answers the
redirect with "unknown sign-in provider".

The provider button carries no `loading` state, unlike its neighbours. It
leaves the page rather than resolving, so a spinner would spin until the
browser navigated away and then come back on a fresh mount.

The Go side is in [backend.md](backend.md#authentication).

## Two views, one codebase

The chrome has exactly one viewport branch, in `shell/RootShell.tsx`: a persistent
navbar on desktop, and on mobile a single row of chrome whose sections live in
a dropdown beside the mark. Below it, screens are viewport-agnostic.

The phone chrome is one row, not a header *and* a thumb-reachable bottom tab
bar. The argument for the bar -- that the top of a phone is the hardest place
to reach one-handed -- is true, and is how this app gets used at a table. It
loses to a bigger one. Two rows of chrome cost 108px of an 844px screen to draw
two things, one of which is pressed rarely: you are usually already in the
section you want. Folding the sections into a dropdown buys the content back a
whole row, gives each section its full name instead of a tab bar's four
characters, makes them real links rather than imperative `useNavigate`, and
costs the same whether there are two sections or six -- which a `Tabs.List
grow` does not.

The `AppShell.Footer` slot is empty on a phone as on a desktop; see
[licensing.md](licensing.md#known-gaps) for the thing that wants it.

Two smaller rules the three shells share, both of them fixes for something that
looked wrong on screen rather than preferences:

- **One header height**, `HEADER_HEIGHT` in `shell/chrome.ts`. The landing
  chrome at 56 and the phone chrome at 52 would move the whole page up four
  pixels on sign-in -- a flinch at the moment somebody first sees the
  app. Three shells sharing a corner need to share its dimensions, or the
  seam between logged-out and logged-in is visible.
- **The desktop navbar narrows to a rail of glyphs.** A wide screen has room
  for 240px of navbar only while the content is whatever width the window gives
  it, and every page is capped at `CONTENT_MAX_WIDTH` -- on a 1280px laptop, 240 of navbar plus
  1024 of content plus the shell's padding does not fit, and the thing that
  gives way is the table you were reading.

  **It narrows rather than disappears, and that is what fixes the control.** The
  problem with a menu that vanishes is not the hiding, it is that the way back
  has nowhere to live. Put it in the header and it crowds the wordmark, and a
  `Burger` there, open, draws a close cross left of the mark and before the
  app's own name, so it reads as a way to dismiss something. A rail always has room, so the
  control keeps one address in both states. It costs 64px of the 240 it gives
  back; what it buys is a navigation that never leaves the screen.

  **The control sits directly under the sections, behind a rule**, and is drawn
  as a `NavLink` like they are -- so it inherits their geometry instead of being
  aligned to them by hand, and it is dimmed so it does not read as another
  place to go. Not at the foot of the navbar, where a sidebar's chrome
  conventionally goes: this navbar is as tall as the window and holds a handful
  of items, so the control would end up hundreds of pixels below the last thing
  anybody had looked at.

  Two numbers are pinned rather than left to the content, both because the
  difference showed up on screen. **A row is `ROW_HEIGHT` in both states**: it
  measures 41px with a word in it and 34px without, so collapsing would
  otherwise shorten every row and shunt the list upward while you watched. And **a row
  carries an explicit radius**, because Mantine's `NavLink` ships with none --
  square corners are barely noticeable on a 219px-wide highlight and read as a
  cramped box on a 43px one, so the rail's active item is the same shape as the
  menu's, just narrower.

  The section glyphs earn their keep twice here -- on the rail they *are* the
  navigation, so each keeps its name in a tooltip and in the link's accessible
  name.

  **It is not remembered.** A remembered state would be a
  setting no page lists, which is how you get "the menu is gone" from somebody
  who collapsed it once, a month ago.

  A note for whoever reaches for `AppShell`'s `collapsed` prop: it is read only
  when Mantine considers the layout to be in its desktop mode, and
  `breakpoint: 'never'` opts out of having modes -- so `collapsed` silently does
  nothing here. It also passes every test in
  `RootShell.test.tsx` while doing nothing, because the suite runs with `css: false` and the width
  arrives from a generated `<style>` element. Resizing `width` is what this
  shell does instead, and it needs no breakpoint at all.

Where a layout genuinely has to differ, it differs inside a `@/ui` primitive
rather than at the call site:

| Primitive | Desktop | Mobile |
|---|---|---|
| `ModalSheet` | centred modal | bottom drawer |
| `DataList` | table | a card: name, marks, one dimmed line of facts, a `⋮` menu |
| `TabDeck` | tab strip, and the active panel | tab strip over a carousel of every panel |
| `TabRow` | tab strip, wrapping onto a second line | tab strip scrolled sideways, ends faded |
| `ChoiceDetails` | inline, under the row it was opened from | a full-screen view with Back |
| `BlockList` | a list of blocks, one open | the same |

Every dialog in the app is a `ModalSheet`, so what a sheet looks like on a phone
is decided once. Two things are, there: **`title` is required**, because a
sheet without one opens as an empty header band over an unnamed form (the game
tracker's edit sheet is titled with the entry's name); and the
**body keeps its own top padding** in both renderings -- the phone's drawer
and the desktop's modal -- which Mantine zeroes under a
header -- fine when the header is the panel's colour, flush against a band's
edge when it is not.

### A dropdown inside a sheet stays inside it

Mantine portals a `Select`'s dropdown to `document.body`, and a `Drawer` closes
when a tap lands on its overlay. On a phone those two facts meet: the dropdown
is outside the sheet, so opening a picker closes the sheet under it and leaves
the list floating against the bottom of a page it no longer belongs to.

Every `Select` inside a `ModalSheet` therefore passes `SHEET_COMBOBOX`
(`comboboxProps={{ withinPortal: false }}`), which keeps the dropdown in the
sheet so the tap lands where the person tapping thinks they are. It is a
constant rather than a theme default because it is only right there: a `Select`
on an ordinary page may sit inside something with `overflow: hidden` -- the
build screen's carousel does -- where an un-portalled dropdown is clipped
instead.

The sheet's own height cap is `85svh` rather than `85dvh` for a neighbouring
reason. `dvh` is defined to change as dynamic browser UI appears, and
`interactive-widget=resizes-content` makes the soft keyboard resize the layout
viewport too -- so a cap in `dvh` chases every one of those resizes.

### A write refreshes the screen; it does not reload it

`useResource` offers both, and the difference is not cosmetic.

`reload` blanks: it puts the resource back to `loading`, which on these screens
means `ui/Page` swaps the whole body for a spinner. That is honest for a first
load or a retry after a failure, where the screen genuinely has no answer.
`refresh` re-fetches behind what is already drawn.

Every `act()` here follows a write **the server has already confirmed**. The
screen knows what happened; it is catching up on what else changed. Calling
`reload` there would tear the list down and rebuild it under whoever was reading
it -- and on a phone it is worse than untidy: a row action or a bottom sheet's
close unmounts the portal it was dispatched from mid-transition, and the page
flashes.

So: `reload` stays on `pageState`'s `onRetry`, which is the case it was written
for, and every post-write path calls `refresh`.

### A card is not a table row with the headers moved

`DataList`'s mobile rendering does not re-label each cell -- a bold line for
the name, then `Class: --`, then `Level: 0`, then the actions on a line of
their own. That spends a 390px line on one word, repeatedly, and leaves the
actions nowhere to go.

A card is the shape `features/characters/FolderPanel` has: a bordered `Paper`,
a `wrap="nowrap"` row with a `flex: 1; minWidth: 0` name, marks beside it, and
every action behind one `⋮`. Under the name goes **one dimmed line of values**,
joined with `·` and carrying no headers at all.

Four things follow, and each is a rule rather than a detail:

- **A fact with nothing to say is dropped**, including the literal `--`.
  A *table* prints two dashes for nothing, because a blank cell in a grid of
  them reads as a rendering fault -- `domain/classLine` returns them and two
  screens write `level || '--'` by hand. A card has no column to keep straight,
  so it says nothing, and a character who is still being built gets their name
  and no second line rather than "Class: -- · Level: 0".
- **`DataList` styles the name, and the caller supplies it as a string.** A
  call site wrapping its own name in `<Text size="sm">` is how a heading comes
  out the same size as the fields under it. `text` is also
  what names each of the row's actions.
- **A column says where it goes on a phone.** `badge` rides beside the name,
  because a `<Badge>` in a run of dot-separated text reads as neither; `block`
  gets a full-width line of its own, which the event log's `Detail` needs
  because it is a stack of `<Code>` elements rather than a value. Marks that
  never had a column at all -- "Yours", "You", "Guest" -- are the `badges` prop
  instead, and they ride with the name at *both* widths.
- **A list keeps one right edge.** If any row in the list can be acted on, the
  rows that cannot reserve the gutter anyway. Otherwise a roster whose owner row
  has no menu runs 44px wider than its neighbours, and a ragged edge down a list
  reads as a bug.

`BlockList`'s two renderings are **identical markup**, and `TabRow`'s differ in
one style property: the list wraps on a wide screen. The others genuinely swap
components at the breakpoint; `TabRow` is a `ScrollArea type="never"` that is
simply inert at a width the tabs fit in, so there is no second tree to keep
working.

### A scrolling strip rests on a tab, and hides the one it cuts

Two things that only matter once the tabs do not fit, which on the sheet is
always: seven labels are 657px and a phone viewport is 369.

**It rests on a tab's left edge.** Stopping the moment the active tab's *right*
edge clears the viewport is the least it could do and leaves whatever tab straddles the left edge cut in half. Landing on
the tab's own left edge cannot leave a fragment, because a boundary is where a
tab begins.

**The rule under the tabs spans the tabs.** `Tabs.List` is a block inside the
scroller, so left alone it takes the *viewport's* width -- 369px against 657px
of tabs -- and its bottom rule stops a third of the way along while the tabs
themselves overflow it. From a scrolled position that draws as a stray dash
beside the first tab you can see. Under `md`, `width: max-content` makes the
list as wide as what is in it.

**The end that is cut fades, and the tab under the fade stays visible.** Hiding
the cut tab outright -- on the argument that a legible fragment reads as a
broken word rather than as "there is more this way" -- misses where the strip
rests. Rule one lands it on a tab's own left edge, so the tab straddling the
*far* edge is cut by however much of it happens to fit, and hiding the cut
hides the entire next tab: the strip ends in clean space and reads as having
no more tabs. The fragment is the only evidence there is more.

So the fragment is kept and the fade is a constant 44px at any end with strip
behind it. A tab dissolving into the edge is the signal every scrolling strip
uses, and it cannot be read as a typo because it is visibly unfinished -- a
shorter ramp leaves the far half of a fragment at 80-90% opacity, where it
reads as finished text. An end with nothing behind it is drawn hard, because
that is what says the strip stops there.

What that measures is one boolean per end -- whether the scroller has
anything left that way -- rather than the geometry of whichever tab lies across
the edge. It is the
one thing in this component the suite cannot press: happy-dom computes no layout, so
every strip there is 0px wide, never overflows, and never draws a mask. What the
tests hold is that the absence is identical at both viewports. The active tab
is brought into view by setting `scrollLeft`, not
by `scrollIntoView`, which scrolls every scrollable ancestor -- it would drag
the document as well as the strip, and happy-dom does not implement it. A stack of
bordered disclosures needs no branch either: it is right at 390px and at
1440px, and the only difference is padding the spacing scale already handles.

## A dialog is a form, so the keyboard's Go key works

Every dialog in this app that asks for a name passes `onSubmit` to
`ui/ModalSheet`, which wraps its children in a real `<form>` and lets the confirm
button be a `type="submit"`.

Without it there is a bug you can only find on a phone. A soft keyboard offers
a Go key on the strength of the browser seeing a form with a submit button in
it -- so in a dialog that is a bare `TextInput` beside a `Button onClick`, you
type a name, press the obvious key, and nothing happens at all: no error and
no hint.

Putting it on the wrapper is what stops that recurring: a dialog
opts in with one prop, and the button and the key press cannot drift apart
because there is one handler rather than two.

**A dialog with no field must not become a form.** Delete-this-group,
hand-over-ownership and the pickers pass no `onSubmit`, and `ModalSheet` wraps
nothing -- a stray submit in a confirmation would fire on a key press nobody
aimed at anything. `ui/ModalSheet.test.tsx` holds both halves.

`NameForm` keeps its Enter handler and is the remaining odd one out: it is not
in a dialog, it is the first thing anybody sees on the build screen, and its
button is the screen's own. Worth folding in the next time that screen is opened.

## The keyboard resizes the page, not just the view

`index.html`'s viewport meta carries `interactive-widget=resizes-content`, and
it is there for the bottom sheet.

By default a soft keyboard resizes only the *visual* viewport: the layout
viewport stays the full height of the screen, so an element anchored to its
bottom -- which is what `ModalSheet` becomes below `md` -- is left sitting behind
the keyboard. Every rename and create dialog puts its field there, so typing into
one would mean typing into something you cannot see. `resizes-content` shrinks the
layout viewport instead, which puts the sheet on top of the keyboard and makes
the sheet's height cap mean what it says.

The sheet's body also scrolls (`overflowY: 'auto'`), for the case the cap still
bites: the New folder dialog carries a paragraph above its field, and on a short
viewport with the keyboard up that is taller than the space left.

## One button size, in one place -- and one CSS rule beside it

Every `Button` is `xs`, at every width, set once in `ui/theme.ts` as a
`defaultProps` override rather than passed at each call site. Passed at each
call site, the result is three sizes -- a header "Log in" at `compact-sm`, a
`/login` page of default-`sm` buttons, inline retries at `xs` -- and pressing
one and landing on the other reads as two designs, which is what a size decided
fifteen times eventually looks like.

A call site may still pass `size` where it genuinely means something
different. `ACTION_ICON_SIZE` is a separate constant because a glyph is not a
control.

### The one thing a theme value could not say

`ui/app.css` is the app's only stylesheet, and its first rule is the one a
theme value could not say: a phone's input text is 16px. (The rest is the
Markdown body's block structure and the AI Wizard chat's animations.)

That is a browser fact, not a size. **iOS Safari zooms the whole page when a
field smaller than 16px takes focus**, and every field here is Mantine's `sm`,
which is 14px -- so without it every rename box in the app lurches the page when
you tap it. It is set as `font-size` on `.mantine-Input-input` rather than through a
variable because Mantine writes `--input-fz` as an *inline* custom property from
its `varsResolver`, and an inline declaration cannot be reached from a
stylesheet. One rule covers all five field types; they each render an `Input`
underneath.

`ui/AppTheme.tsx` imports Mantine's `styles.layer.css` rather than `styles.css`,
so every Mantine rule sits inside `@layer mantine` and this one beats it whatever
the selectors weigh -- no specificity contest, no `!important`, and no dependence
on which file the bundler emits first. `ui/theme.test.ts` asserts no
`!important` appears there, because one showing up is the first sign the layered
import was swapped back. It also pins the breakpoint byte-for-byte against
`DESKTOP_MEDIA_QUERY`, since `postcss-preset-mantine`'s `smaller-than` mixin
would have written 61.9375em -- a pixel from where `useIsDesktop` changes its
mind, and close enough that nothing would look wrong while one width disagreed
with itself.

**`app.css` is the only stylesheet, and `AppTheme` the only file that may import
one.** `scripts/check-layers.mjs` enforces both, and for that it sees
side-effect imports too: a pattern requiring a `from` leaves every
`import '@mantine/core/styles.css'` invisible to the vendor rule that exists to
keep Mantine inside `src/ui`.

### What was tried and reverted: 44px controls on a phone

This file briefly grew *every* control to the 44px touch target below the
breakpoint, with `theme.ts` naming which Mantine size each control wore and
`app.css` saying what those names measured at each width. The argument was good
and the result was not: at 390px this app is mostly controls -- a heading row of
three, a tab strip, a pair of add buttons under every folder -- and inflating all
of them turned a dense screen into a scroll. It is recorded rather than quietly
dropped, because the touch-target argument is correct in the abstract and
somebody will make it again.

What survived is the part that was a fact rather than a judgement, which is the
16px rule above. `TOUCH_TARGET` stays in `theme/tokens.ts` as the floor for what
a thumb has to hit *precisely* -- `ScoreAssignment`'s drag targets -- and not
as the size of every control.

## Personal appearance settings

`/account` offers an Appearance card for account holders and guests. Color theme
chooses Dragon, Parchment, Midnight or Moss; Display mode chooses Light, Dark or
System. Changes apply immediately without remounting the page. While loading or
saving account appearance the selectors are disabled; a failed save restores
the previous preference
and shows a translated error. System follows the device's current color scheme.

`lib/appearance` owns the preference and persistence, below the UI layer.
`AppearanceProvider` sits inside `AuthProvider` and wraps `AppTheme`, which builds
Mantine's accent ramp and surface variables from the active palette. Mantine's
separate localStorage color scheme manager is disabled so there is one authority.
Default appearance is Dragon with System mode.

Account preferences load through `GET /v1/appearance` after login and session
refresh, and save through `PUT /v1/appearance`, replacing the complete resource.
Appearance is independent of authentication response DTOs. A failed read keeps
the cached theme and offers Retry; successful reads are authoritative.
There is no background polling or offline write queue. Confirmed values are
cached in localStorage under `easydnd.appearance.account.<id>`, with the last
account ID in `easydnd.appearance.account` to restore appearance while the session
loads or is offline. Sign-out restores the browser preference and clears that
pointer. Switching accounts cancels old saves and keeps caches separate.
Guests and signed-out visitors use `easydnd.appearance.browser`; guest choices
are never uploaded at sign-in. Invalid or unavailable storage falls back safely.

The palettes remain framework-free data in `theme/palettes.ts`. Each defines
both light and dark surfaces. `PALETTE_NAME` in `theme/tokens.ts` remains the
default brand palette for generated icons, the PWA manifest, logos and dice
artwork. Personal settings do not regenerate those assets. Runtime browser
`theme-color` follows the selected palette's brand color.

**The binding uses semantic CSS variables and surface aliases.** `createTheme` takes
colour *ramps* and has no way to say "the page's background"; `cssVariablesResolver`
in `ui/theme.ts` is the lever, and it takes light and dark separately. Mantine's
components also read ramp colors directly, so light white and
gray border shades and dark surface/border shades receive palette-specific
aliases. Paper's background binds to the semantic surface variable in the theme.
This makes cards, inputs, tables and panels follow the active palette.

**Computed colors need a browser check.** Vitest runs with `css: false`
and happy-dom lays nothing out, so no test can read a computed colour. That leaves
the data as the only surface to hold, and `theme/palettes.test.ts` holds it:
ten valid steps, a brand colour drawn from its own ramp, both schemes complete,
and -- the one that earns its keep -- text-on-background contrast of at least
4.5:1 in both. That is the failure invisible to whoever makes it, because they
were looking at the scheme they designed. Dimmed text is held to 3:1 rather than
4.5:1 on purpose: Mantine's own default dimmed is 3.3:1 on white, and holding a
palette to AA body text there would fail the framework's own choice.

## The icons come from the palette

The brand colour has one source, reached three different ways. Written out in
each place it is needed -- `tokens.ts`, `favicon.svg`, `index.html`, the PWA
manifest -- it drifts.

**`vite.config.ts` and `index.html` are not generated -- they import.** The
config is TypeScript compiled by Vite, so it imports `PALETTE` directly and the
manifest's `theme_color` and `background_color` become expressions with nothing
to diff. `index.html`'s `theme-color` meta is filled by a `transformIndexHtml`
plugin in the same config, at dev *and* build; the browser paints its own chrome
from that tag before a line of React runs, so it cannot read a token the way a
component does. Rewriting a hand-edited config from a generator would invite a
conflict every time either side moved; an imported value cannot drift at all.
The plugin throws if the tag is missing, because a rewrite that quietly finds
nothing to rewrite is not a gate.

Watch the shape of that check, which is the bug it was written with: it tests
for the tag's *presence*, not for whether the HTML changed. `String.replace`
hands back an identical string when the value it writes is the value already
there, so "did anything change?" reports the correct case as the missing one.

**`scripts/gen-icons.mjs` reads the TypeScript.** Plain Node, no flag, no
dependency: `.nvmrc` is 24, Node has stripped types unflagged since 22.18, and
`tsconfig.app.json` already sets `erasableSyntaxOnly` -- so every file under
`src/` is *already* constrained to exactly the syntax the stripper accepts.
(`theme/tokens.ts` imports its neighbour with an explicit `.ts` extension for
this reason, and nothing else in `src/` does; Node's ESM resolver does not guess
at one.) The alternative was a `.js` palette beside a hand-written `.d.ts` --
two files to keep in step, and `check-layers.mjs` only walks `.ts`/`.tsx`, so
`theme/` would have gained a file nothing checked.

It owns `public/favicon.svg` and the four PNGs.
`make web/icons` regenerates them; `make web/icons/check` fails on drift and is
part of `make verify`, beside `pack/check`.

**The check compares decoded pixels, not file bytes.** `deflateSync` is
deterministic for a given zlib but is not promised to be stable across Node
versions, so a byte diff would be a gate that goes red on a laptop whose Node
differs from CI's -- failing for a reason that has nothing to do with the icons.
Decoding is trivial because the encoder only ever writes filter type 0.

**The one footgun.** The icon set is generated bytes that live in git, so
switching `PALETTE_NAME` to look at something and switching back leaves
`web/public` matching whichever palette you last generated, and `make verify`
will say so. `git checkout web/public` is the way out, and the failure message
says as much.

## Dependency rule

```
theme -> lib -> ui -> shell -> features -> routes
```

Imports point left, and **only `src/ui/` may import `@mantine/*` or
`@tabler/*`** -- everything else imports from `@/ui`, which re-exports what it
needs. An icon set is a *look* the same way a component library is, so a
feature reaching past `@/ui` for a glyph is the same leak with a smaller blast
radius. `npm run lint:layers` enforces both, the same way `make lint/layers`
does for the Go packages: a convention nobody can run is a convention that
rots. The list of packages it guards lives in `scripts/check-layers.mjs` as a
list rather than one name, because `@tabler/` arrived through the hole a single
hard-coded `@mantine/` left open.

The same rule covers a second vendor: **only `src/lib/i18n/` may import
`i18next` or `react-i18next`**, and everything else imports `useT` from
`@/lib/i18n`. A translator that sixty files reach for directly is a translator
nothing can ever replace, and the re-export is also where the key type lives.
The guarded directory is `lib/i18n` rather than `lib`, because it is a
directory inside a layer and the rest of `lib/` has no business importing
i18next either.

`src/domain/` sits beside `theme/` at the bottom: pure rules, no framework, no
transport -- and no prose: the nouns are message keys in
`features/character/labels.ts`, and what is here is what is genuinely a rule.

## Adding a feature

Types and calls in `web/src/lib/api/`, screens in
`web/src/features/<aggregate>/`, a route in `web/src/routes/index.tsx`. Shared
visuals belong in `web/src/ui/`, never inline in a feature. The API's error
envelope is decoded exactly once, into `ApiError`, by `lib/api/client.ts`.

A **new top-level section** is one more entry in `ui/sections.ts` -- a path, a
label and a glyph. Both shells map over `SECTIONS` and neither needs touching,
and so does every breadcrumb: the desktop navbar, the phone dropdown and the
first crumb of every trail all build themselves from it.

It lives in `ui/` rather than `shell/`, and that is forced: a trail begins at the section it is in, so *screens*
need the same label and glyph the navbar draws, and `features/` may not import
`@/shell`. `lib/` could not hold it either, being denied the icon package. `ui/`
is the one layer both the chrome and the screens can see.

`sectionFor` decides which section a path belongs to, and a `Section` carries
two different things for a reason. `to` is where it *links*; `owns` is what it
*claims*. Characters is why: its list is `/`, and `/` as a prefix matches the
entire app, so matching on `to` alone would mean a character sheet lit nothing and
the phone's dropdown fell back to the word `Menu`. Splitting the two jobs lets
`/characters/*` belong to Characters while `/` still matches only itself -- the
one property that function is pinned on. The trailing slash in the prefix test
is load-bearing too: without it `/groupsfoo` answers Groups.

The dropdown keeps the `Menu` fallback, because it is one control and the only
thing naming the current place on a phone, and a path nothing names -- a 404
-- still needs a word.

Watch the names when a feature's API types meet the design system. `@/ui`
already exports Mantine's `Group` layout primitive and every screen uses it, so
`lib/api/groups.ts` exports `GroupDetail`, `GroupSummary` and `GroupMember` and
deliberately nothing called `Group` -- a type of that name would shadow the
component in exactly the files that need both.

The **brand mark** is the d20 in `web/public/favicon.svg`: the tab icon, the
PWA icons (`npm run icons` regenerates them from `scripts/gen-icons.mjs`) and
the header wordmark. `shell/Wordmark.tsx` reaches it with an `<img>` because
the browser has already fetched that file for the tab.

`ui/ProficiencyMark` is an inline SVG component, and the reasons behind how it
is drawn are what generalise. It is **announced**. It is drawn in
`currentColor`, because it sits inline in a row of text and has to dim when the
row dims; a literal would make it the one thing on the row ignoring both the
row and the colour scheme. And it is named with `aria-label` rather than a
`<title>` child, because a `<title>` is a text node: eighteen of these share
the skills panel, whose rows are read as text, and "Stealth DEX +7" must not
come back with a sentence about proficiency bonuses in the middle of it. The
rule: a mark riding inside text takes the text's colour and stays out of its
content.

There is a rule for choosing a source. The brand mark is generated and lives
in `public/`, because it is the app's own. A **UI affordance** -- an account, a way out,
a chevron, a tick -- comes from `@tabler/icons-react`, re-exported one glyph at
a time through `@/ui`. Hand-drawing those is how two "delete" controls end up
different shapes, and the re-export list doubles as the app's icon inventory:
adding one is a decision somebody makes in `ui/index.ts` rather than an import
nobody reviews. They cost the production bundle little, because each
is its own ES module and the package sets `sideEffects: false` -- named imports
only, never the deep `dist/esm/...` paths.

Those icons are **decorative**, which inverts the mark above: the control
around them carries the accessible name, and a named glyph inside a named
button says it twice. That is the generalisable half -- a mark is announced
when it is the only thing saying what it says, and silent when something else
already does.

`/account` is where both inventories live -- passkeys and connected providers --
and where connecting and disconnecting happen. It shows each of them only when
there is something to show or something to do: a Google-only account is not
told it has no passkeys, and a deployment with no provider configured is not
told so under a heading of its own. The exception is the account that most
needs it -- a passkey-only account has nothing to list, and still gets the
connect card, because connecting is the whole of its recovery. A guest gets
neither, and no alert about it: the subtitle says the session is a guest's, and
a paragraph under it listing what a guest session lacks would be the page
scolding somebody for the way they came in, on the one screen where there is
nothing they can do about it. Both account holders and guests have the
Appearance card.

**The way in is a row in the navigation, and it is still not a section.** The
desktop navbar draws it under the same rule that separates the collapse control
from the sections; the phone's dropdown draws it under a `Menu.Divider` at the
bottom of the same list. Nothing about it comes from `ui/sections.ts`: no trail
starts at `/account` and `sectionFor` answers null there; the phone's trigger
names it from `MobileShell`'s own `ACCOUNT` constant.

It is a row rather than a glyph in the header's top-right corner. The
navigation lists the parts of the app while the account is who is looking at
them -- which is exactly why the row sits below the rule rather than among the
sections -- but a corner glyph can only name itself when hovered, and the
corner already holds its share. A menu row has a word.

**What stayed in the corner is the way out**, built once in
`shell/AccountActions.tsx` and used by both signed-in shells, beside the
language: signing out is not somewhere to go, so it is not in a list of places.

It is a glyph, not a display name and a text button. A display name is
arbitrary length in the narrowest row this app has, sharing it with a mark, the
word "easydnd" and a button reading "End guest session" -- and the thing that
overflows first is the control that ends the session.

So the words are the control's accessible name and tooltip: `Sign out`, or `End guest session` for a guest. The header
ends in the way out whether or not there is an account behind the session.

**Signing out lands on `/`**, rather than leaving the URL where it was. Staying
put reads as a bug at every address that is not `/`: the chrome swaps to the
logged-out one underneath you, and the deep link you were on -- a sheet, a group,
`/account` -- either bounces through the gate or sits there naming a thing you
can no longer open. `/` is the one address that means something on both sides of
the boundary, so it is where the session ends. The navigation happens after the
request rather than beside it, and `signOut` drops the local session even when
the request fails, so it is reached either way.

The header therefore says nothing about whose session it is. `/account` does:
its profile card prints the display name beside the avatar.

A tooltip cannot be the name. Mantine's wires `aria-describedby`, and only
while it is open; the label is not in the DOM at all when it is closed. So the
`aria-label` is the name and the tooltip is the sighted equivalent, both built
from one string -- duplicated deliberately rather than by accident.

`/` is the one page both sides of the sign-in boundary share: the three panels
signed out, the character list signed in. It carries nothing else -- system
status is a deploy question rather than something either audience came to `/` to
read.

There is no `/status` page: the comparison it would show -- the bundle's
version beside the API's -- is made by the client itself on every response.
`curl`ing `/version.json` against `/v1/version` is the manual check, and it is
what the deploy pipeline does.

`/legal` is the one route outside `RootGate` entirely: it renders in
`LandingShell` for everybody, signed in or out, because needing an account to
read the licence of the material you are being shown would be backwards. It is a
document rather than a section, so it stays out of `ui/sections.ts`, and it is
reached from the footer the landing chrome carries. The wordmark is a
signed-in visitor's way back; the landing header offers them no second
invitation to sign in.

`/roll` is a third of the same kind, private rather than public: a page that is
a die and nothing else. Out of the section table for the same reason the two
above are -- it owns no paths, lights nothing and starts no trail -- and
reached only from the phone menu, below a divider, because the die is a
thumb's. See [The die is a page, not a
dialog](#the-die-is-a-page-not-a-dialog).

Routes added under `/` render inside `RootGate`, which only picks the chrome:
a screen that needs a session is wrapped in `Private`, and a route that must
stay public -- `/login` -- is not. Either way, guard the *data* on the server
rather than assuming the route is unreachable.

The Go side of the same feature is in
[backend.md](backend.md#adding-a-feature).

## Localization

The client speaks English and Russian. **No user-facing word appears anywhere
under `src/`**: every caption lives in `web/locales/en.json` and
`web/locales/ru.json`, reached by key. That is the whole design goal, and the
reason for most of what follows -- translating this app has to mean editing a
data file, never opening a component.

```tsx
const t = useT()
<Title order={2}>{t('login.title')}</Title>
<Text>{t('folders.newCharacterIn', { name })}</Text>
<Text>{t('choice.language', { count })}</Text>
```

**`i18next` + `react-i18next`, and only `src/lib/i18n/` may import them.**
`npm run lint:layers` enforces that, for the reason `@/ui` exists: a vendor
every layer may reach for is a vendor nothing can ever replace. Screens import
`useT` from `@/lib/i18n`, which also carries the key type -- `useT` is typed
from `en.json`, so a key the catalogue does not define will not compile.

**Fallback is per key, not per file.** A Russian catalogue that has translated
a button and not the paragraph beside it shows the button in Russian and the
paragraph in English. That partial state is what a growing locale actually
looks like, so it is the case that has to work well -- and it is the same rule
the Go catalogue applies to SRD prose, described in
[dnd.md](dnd.md#localization). That is the runtime's rule; the gate is stricter
-- see [Keeping the catalogue honest](#keeping-the-catalogue-honest).

**Plurals are keys, not code.** Russian has four plural forms where English has
two, so `n === 1 ? 'x' : 'xs'` is wrong before the word order is. A plural is
`foo_one` / `foo_other` in the catalogue, called as `t('foo', { count })`, and
i18next picks between the forms with `Intl.PluralRules`. Russian adds `_few`
and `_many` in its own file and nothing in `src/` changes.

That is also why **counts are digits rather than words**: the build screen
reads "2 more languages", not "Two more languages". A Russian
numeral agrees in gender with the noun it counts -- два языка, две черты -- so a
shared table of spelled numbers is the same composition bug one level down.

**Nothing is glued together.** A label built as `` `${whose}Spell attack bonus` ``
is English word order written in TypeScript: a translator handed the fragments
cannot reorder them, because the code already did. Every such phrase is one
message with named arguments in it.

### What is not translated

- **`src/features/legal/attribution.ts` and the notices on `/legal`.** The SRD
  5.1 attribution is pinned to `data/pack/srd-5.1/ATTRIBUTION.md` by a test, and a
  translated licence notice is a different notice. See
  [licensing.md](licensing.md).
- **`index.html`'s `<title>` and `<meta description>`, and the PWA manifest.**
  They are static, baked at build time, and stay English. This is not a
  preference: the browser and the OS read all three *before* the bundle is
  parsed -- they are what the install dialog, the home-screen label and the
  browser tab are drawn from -- so nothing in `src/` could swap them per
  locale whatever machinery existed there.
- **The product's name.** `easydnd.org` is a name, not a word: it is what
  people type, what the certificate says and what an issue is filed against.
  Translating it would produce a product nobody can find.

- **Language names in the switcher.** "English" and "Русский" are what each
  language calls itself, which is what somebody looking for one is looking for.

The static ones -- `index.html` and the manifest block in `vite.config.ts` --
carry an `i18n-exempt` comment naming the reason. Grep for it before extracting
strings: a marked string is one to leave exactly where it is.

### Choosing a language

`LocaleActions` sits in the header, left of the account and sign-out controls,
and is drawn for guests and signed-out visitors too -- somebody who cannot read
the landing page should not have to make an account before they can.

Each row in its menu carries a flag emoji beside the name (`LOCALE_FLAGS` in
`lib/i18n/locales.ts`). A language is not a country, so the flag is `aria-hidden`
decoration and the name it sits beside is what actually identifies the row.
Emoji rather than artwork: Windows ships no flag glyphs, so Chrome there draws
the two-letter code instead -- still the right pair of letters.

The choice is autodetected from the browser on first load and kept in
`sessionStorage` under `easydnd.locale`. **Not on the account**, so a guest has
one; **not in `localStorage`**, so it is this visit's business -- the same trade
`features/groups/inviteToken.ts` makes. The consequence is worth knowing rather
than discovering: a second tab autodetects again rather than inheriting the
choice.

Detection is fifteen hand-rolled lines in `lib/i18n/instance.ts` rather than
`i18next-browser-languagedetector`. That leaves `i18n.language` as the raw
tag, so `ru-RU` produces a locale of "ru-RU" that every consumer has to
normalise again, and it reads `sessionStorage` unguarded, so a private-mode
browser that refuses storage throws before the app renders.

The chosen language rides on **every** API request as `?locale=`, because the
server negotiates per request and a page cannot rewrite its own
`Accept-Language`. `lib/api/locale.ts` holds it; `src/test/setup.ts` resets it,
because the suite shares one module registry. The catalogue cache in
`lib/api/catalog.ts` is keyed by locale for the same reason -- without that, a
switch would go on serving English entries for the life of the tab.

### Keeping the catalogue honest

Two checks, both in `make web/lint`:

- **The compiler.** `useT` is typed from `en.json`, so a key that is not in the
  catalogue is a type error at the call site.
- **`npm run check:messages`.** The compiler cannot see the other direction --
  a key the catalogue defines that nothing renders any more -- and has nothing
  to say about Russian. The script fails on an unused key, on a Russian key
  with no English counterpart, and on **an English key Russian does not
  translate**.

  That last one is a failure rather than a printed percentage, although a
  partial locale is the normal state of a growing one. The per-key fallback is
  why: with a whole screen's keys missing it does exactly what it promises and
  serves English, so nothing looks broken enough to notice until somebody reads
  the screen. A fallback that good is what stops a gap surfacing on its own,
  which makes a number nobody reads the wrong instrument. Adding an English
  caption means adding its Russian in the same change.

  Keys are compared on their base name, so Russian keeps the `_few` and
  `_many` forms English has no use for and is never asked for a suffix its
  grammar does not want. The runtime fallback stays either way -- it still
  saves a user from a blank screen if a key slips through -- it just does not
  decide what "finished" means.

### The suite renders in English

Around six hundred assertions match visible copy --
`getByRole('button', { name: 'New group' })`. They hold with the captions in
`web/locales/` because `src/test/render.tsx` is the single seam: it wraps every render in a `LocaleProvider` pinned to English.
`renderAt(viewport, ui, 'ru')` is how a test asks for the other one.

The instance is built per render rather than shared, and that is not tidiness.
The suite runs with `isolate: false`, so a language set on a module-level
singleton by one file would be inherited by every file that ran after it, in
whatever order they happened to run -- which is the same hazard `vi.mock` is
banned for. A module of pure functions that needs words takes a `Translate` as
an argument instead; `features/character/settled.ts` and `options.ts` are the
worked examples, and `src/test/i18n.ts` is the translator their tests hand over.

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
[backend.md](backend.md#deployment).

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
   [backend.md](backend.md#what-a-release-is-called-and-where-it-lives).
2. **A bad bundle goes live silently.** The API-side health gate cannot see the
   frontend, so `deploy.sh` checks the bundle exists *before* the symlink swap.
   Without that, a bundle that unpacked badly would go live as a blank site and
   would not roll back.

[mantine]: https://mantine.dev

## Builder choice behavior

Creation and level-up use the same tabs and server prompts. Cantrips and
Spells have separate tabs; all equipment choices live in the final Equipment
tab, and nothing else does. **That tab is drawn on a visit that was asked a
starting-kit question, and stays for the rest of that visit** -- otherwise
answering the kit would take the tab away mid-creation, and the answer with it. The kit is
asked once -- by the first class, at level 1 -- and its answers seed the
inventory; after that the character's things are edited on the sheet's
Equipment and Items tabs, so a visit that opens with the kit already answered
draws no tab: Edit and Level up never show one. There is no stored "finished"
flag, so "creating" is exactly "this visit saw a kit prompt"; leaving a
half-built character and coming back with the kit answered hides the tab too.
The spell tabs' Next and save land on the tab after them with work or stay put.

**Editing is the creation screen, locked where it is final.** The Rules tab of
an existing character draws the same ruleset form and the same pack buttons as
creation, disabled, with "This choice is final" -- not a summary card and a
text list, which read as a different screen. Reopening the level-1 class choice
re-poses the kit, and the tab comes back with it. A selection
changes a card in place: choosing, confirming, reopening, or replacing it must
not sort the wizard's question cards. Spell choices have an explicit selected
list above their filters; other option lists retain their original order. Answered choices use the original
prompt identity; other events use their persistent ID. New questions append to
their level group. Sequence numbers address writes, not visual identity.

All displayed equipment and spell names resolve through the localized catalogue.
Saved answers use the API's resolved `selections`, which retain quantities and
fixed bundle components. A crossbow and twenty bolts therefore remain two named
items with the ammunition count, including in Russian. Catalogue requests show
loading and retry states; stale responses cannot overwrite a newer language.
Selections remain in hand while option details load or the language changes.
Large spell lists load in bounded catalogue batches rather than exceeding the
API slug limit. Failed requests cannot be
confirmed as if the catalogue were complete.

Starting-equipment cards show complete descriptions plus armor and weapon facts,
weight and pack contents for the selected option. Bundles describe each
component. Equipment-category choices support their full membership, and a
multi-item category permits repeated copies. Confirmation still requires the
requested number of items. Conditional cleric gear stays visible but disabled
until its proficiency requirement is satisfied. Fixed equipment is granted by
the rules; choosing equipment puts it in the backpack, without automatically
wearing it. Starting wealth shopping is not part of this screen.

A pack's contents are a list under the selected option, one item per line, with
a count only where it is more than one. "×1" is printed nowhere: one of a thing
is the thing.

The tab is its questions and nothing else. There is no inventory list, no coin
fields, no custom items and no way to add an item here: what the character
starts with is decided by the choices (and the fixed grants behind them), and
it is read, worn, counted and added to on the sheet's Equipment and Items tabs.
A list of the projected inventory under the questions would repeat every answer
a second time and invite editing a starting kit by hand.

An option is not drawn until its catalogue entry has loaded. Drawn early it is
named by its slug and then renamed in place -- "Scholars Pack" becoming
"Scholar's Pack" with its source tags -- which read as the card flashing.

A class kit card is **titled by the slot it fills** -- Body, Main hand, Off
hand, Backup weapon, Pack, Focus, Instrument -- because that is how a player
fills a sheet, and the book's "(a) chain mail or (b) leather armor, a longbow
and 20 arrows" reads as word soup when it is joined into a title. The slot is
the server's, on the choice (docs/dnd.md), so the card is named before its
options have loaded; a kit question with no slot -- a background's, a pack
built before slots -- is still **titled by what it offers**, "Component pouch
or one of: Arcane Foci", and one that is only a category reads "Also one of:
Simple Weapons", which tells it from the card above that offers the same
category as one of two options. Armor and weapons are separate choices in the
catalogue. A weapon bundle's label leaves its ammunition unsaid -- "Longbow",
the arrows being what a bow comes with -- and
the detail under the picked option still lists them. No price is shown on an
option: starting equipment is granted, not bought.

A picked option and its description are one box, in the picked colour; the
description is beside the button rather than inside it only because it is
Markdown. The box owns the horizontal padding, so the name and the description
start on one left edge, and the description is smaller so it reads as
being about the option rather than as another one -- not dimmed, because grey
on the picked colour is not legible. The focus ring is drawn round the whole
box (`.picked-option` in `ui/app.css`), not round the button, which framed the
name and left the description outside. An optional block gets no accent
border: that border means "still has to be answered". An item's numbers (damage,
range, weight, cost) are one line joined by middle dots, not a paragraph each;
a bundle heads each component with its name in bold. One of a category is named by the category: "Arcane Foci", not
"1 × Arcane Foci".

A held trait, feat, fighting style or proficiency cannot be selected again where
it would duplicate a benefit. Expertise offers proficient skills/tools that have
not already received Expertise. Explicitly repeatable choices, including ability
improvements and copies of equipment, keep their repetition behavior. Personality
is free text, not a mechanically deduplicated list.

Spell cards identify cantrips, spells known, spellbook additions, preparation,
replacement and special grants. The builder shares the spells page's search and
filters: level (including cantrips), school, class, casting time, concentration,
ritual and no material
components. The wizard's ‘Available for your character’ filter is enabled by
default and uses the union of the server's eligible options, including racial
and subclass grants. Turning it off loads the wider catalogue for the current
Cantrips or Spells tab and allows any catalogue spell to be added, including
spells beyond the normal class, level and count limits. A pick that fits a
normal remaining allowance uses it; other picks are stored under the explicit
custom-spells source. Custom picks remain marked after saving and when the
filter is enabled again. They do not satisfy or inflate class/racial allowances.
Reset restores the eligibility filter. Browsing never changes saved or draft picks. Each spell row shows its icon,
level, concentration/ritual tags, school, casting time and components.

Opening a spell previews it without adding it. Desktop previews expand inside the same spell box, below a horizontal divider,
with no repeated icon or badges and only the row’s right-hand Add/Remove button. On phones, the same complete spell details open in a
full-screen view with Add and Back controls; Back returns without changing the
answer. Adding returns to the list with its filters intact. The detail view and
the standalone spell page share a Markdown renderer, including emphasis, lists,
tables and higher-level rules. Catalogue HTML is not executed.

Every description the client shows goes through that one renderer: spell
details, and the detail under a picked option in the builder (a background, a
feat, an item). `desc` and `blocks.*` arrive as an array with one line-level
block per element -- a table is one row per element -- so they are never joined
by hand: `joinProse` (`web/src/ui/prose.ts`) puts a blank line between blocks
and a single newline between adjacent rows or list items, which is the
difference between a table and a column of pipes. `name` and `fields.*` are
plain text and are rendered as written. The contract is in
`docs/packs.md#prose-formats`. An option's detail sits below its button, not
inside it, because a table is not something a button may contain.

The Cantrips and Spells tabs each show one combined selected list immediately,
without collapsed question boxes or a per-level choice selector. Only the
selected list is grouped by spell level. Available spells form one flat,
list sorted by ascending spell level, then localized name, below the filters. The pool is the union of the
server's eligible options, so class lists, subclass grants, racial cantrips and
current class-level limits constrain normal choices. Turning off character availability
enables explicitly marked custom choices. Stored legacy forget answers are
not learning slots and never increase the displayed capacity.

New picks share one draft and one total, including saved selections: a character
with two saved cantrips and one newly granted slot sees `2 / 3`. A matching
algorithm assigns new picks to the underlying source allowances, reassigning
flexible picks when necessary so click order cannot strand a racial or feature
choice. Ordinary class learning uses the current-level pool for every allowance.
Add is disabled when no legal assignment exists. Next sends the
filled prompts in one revision-guarded append batch, preserving their server
identities and class levels. A level-up extends the current selection by the
allowance the rules grant; it does not reset previously saved spells.

Removing a saved selection loads its read-only edit prompt automatically. All
previous selections remain editable while other edits or acquisitions are in
progress. Each changed answer keeps its own retained picks, while new picks are
assigned to the remaining eligible slots. Nothing is written on Add or Remove.
Multiple replacements and new acquisitions are committed atomically through the
batch revision API; an invalid choice or stale revision leaves the entire saved
log unchanged. Dependent changes still use the dry-run confirmation. Leaving
the tab discards unconfirmed drafts; filters and localization do not.

Each spell box contains its preview control and its Add or Remove button as
separate, keyboard-accessible buttons inside one border. On phones, compact
plus/trash controls with accessible spell-specific names and 44-pixel touch
targets leave more room for the name and tags; the row uses reduced padding
and a smaller icon. The full-screen detail view retains labeled Add/Remove
and Back controls. A single Next button
sits immediately after the selected list beside the count, followed by filters
and available results. There is no second Next button at the bottom. With no
draft, Next navigates directly; with a valid draft it saves first, then advances
from Cantrips to Spells or from Spells to the next tab with work. Invalid or failed saves
keep the draft and the current tab in place.
Selected spells stay visible under every filter. Add remains inside each box,
and a full selection still allows previews. Confirmation is blocked while
catalogue data is loading or has failed. Filters never change the compendium's
URL filters. Learning uses
the server's exact count; preparation accepts a nonempty subset up to the
displayed capacity.
Leaving preparation open does not prevent finishing. The saved events retain the class level that grants each selection,
including intermediate levels of a character created above level one. Cantrip
purposes and legacy cantrip prompt IDs route to Cantrips; other spell choices
route to Spells. Both saved choices and outstanding prompts use that mapping. The sheet displays spell ownership per source, and lets a preparing class change its prepared list without the builder; see [the sheet's spells](backend.md#the-sheet-arrives-resolved).

Editing saved spells reads the question at the original event's
position through `GET /characters/:id/prompts?before=:seq`, with the current
class level and subclass used for spell eligibility. The selected list is
seeded from that event's stored picks, including their order. Loading details or
changing language does not reset the draft. Opening, closing or abandoning this
editor does not delete or change the saved answer. Next uses the
replacement preview and revision guard; dependent answers are only
revalidated against the replacement when it is submitted.

Other saved choices use the existing revision/preview flow. Where a question
must be reopened by removing its answer, the dependent-answer preview describes
what would be lost. Changing level or abilities can require new preparation choices. There is no
automatic selection of spells, and the builder does not simulate resting,
spellbook-copying purchases or combat spell use.


### Current-level spell totals

Above the selected list, the wizard shows one compact explanation
per source, using the server's `spellRules`. Equivalent acquisition allowances
are added together, and spellbook/preparation totals for one class share the same
line. The class label uses its current level. Ordinary class choices identify
the owning class rather than inferring extra classes from spells shared by
multiple lists. Racial/feature choices, automatic grants and custom exceptions
keep their own sources.

The wizard uses current-level totals, with no replacement workflow or history
of when a spell was learned. A level-five sorcerer can select six known spells
from spell levels one through three, including at most two level-three spells,
then edit that list directly. The highest-level count is base acquisitions since
that spell level unlocked plus replacement opportunities in that interval;
replacement opportunities never increase the total. Lower spell levels have no
separate quotas. Per-level answer IDs remain internal storage addresses; all ordinary
learning allowances draw from the class's current spell-level pool. Increasing
the class level adds the newly granted count. Historical spell answers can be
edited against today's class-level pool, without rewriting the level history.

There are no forget selectors, replacement counters, or “Save and review
replacements” step. Old saved swap events still project for compatibility,
and previously forgotten spells stay out of the selected list. The simplified
selection policy intentionally does not simulate the stricter 2014 sequence of
spell acquisition and one replacement per class level. Casting slots remain
separate resources, not per-spell-level quotas for spells known.

Turning off character availability continues to allow explicit custom choices
beyond these normal totals. They retain their custom source and a compact “Custom choice” badge, including
after saving and reopening. The allowance panel includes the selected/total
counter (including custom selections) at bottom left and gray minus/plus
“Change spell limit” controls at bottom right. The extra allowance cannot go
below zero; decreases, including decreases to previously saved extras, are saved
on Next together with increases and spell edits. Adjustments persist as a separate extra
allowance; they do not change class totals or highest-level caps. With character
availability enabled, extras can be picked from the class pool and are explicitly
custom. Availability off still permits unrestricted custom exceptions, even if
the counter exceeds its normal-plus-extra denominator.

Selected cards use blue highlighting. Both lists use compact 32px icons, reduced
padding and one row action; mobile keeps a 44px action target. Available results
start with 20 rows; “Load more” appends the next 20, matching the spells page.
Changing filters resets the loaded range, while adding a spell or taking a
draft pick back keeps the expanded list; removing a *saved* spell asks again,
because the server is what leaves saved picks out. **The list is searched,
sorted, counted and paged by the server**: the panel posts the offer -- the
options its open prompts list, plus the levels and class lists an extra
allowance lets in, minus what is saved -- to `…/catalog/spells/search` and is
sent twenty spells at a time. A draft pick is hidden locally rather than
excluded by the server, so that adding a spell is not a request; the count
subtracts it. The description is fetched for the one row that is open. The selected cantrip list has one “Cantrips”
caption without a redundant level-zero heading. Selected leveled spells retain
their level headings; available results remain sorted by level and localized name.
Each allowance source, including additional allowance, has a caption with gray
details on a separate line.

## Import workspace

The **AI Wizard** section (`/ai-wizard`) is a single assistant chat; the old
`/characters/import` addresses redirect to it (`routes/LegacyImportRedirect.tsx`).
`AgentImportScreen` is a chat of message bubbles: the assistant's on the left,
the owner's on the right, with a message's attachments inside it. Each
`progress` event is a bubble of its own, built by `progressEntry`
(`agentProgress.ts`): the field as a bold caption, its value beneath. It is
never folded.
Source labels and internal assumptions are omitted.

The page has no control for an [unattended session](agent.md#unattended-sessions);
the request field and the CLI flag are the ways to ask for one.

The screen is nothing but the transcript, with the composer as its last
element. The assistant asks for the rules as one button per pack, the press
becomes a player bubble, and the assistant then asks for a sheet or a
description. The composer is open from that first question and carries a
borderless "Attach a sheet" `FileButton` bottom left until a file replaces
it: a line of text with a clip, flush with the words above it. Its
background is held transparent in every state -- disabled, it would get the
grey slab every disabled button gets, which with no padding to sit in reads
as a button drawn wrong.

The rules question is answered by pressing or by writing. `send` on an opened
chat with no rules yet asks `namedPack` (`agentChat.ts`) which pack the text
names and chooses it first; a text that is only the pack's name stops there.
A text that names none -- or a sheet with no text -- is sent as the first
message regardless, and the server takes the deployment's packs: the `rules`
event then carries `assumed`, and `Conversation` draws it as the assistant's
line (`agent.rulesAssumed`) under the message instead of as the player's
answer above it.

Replies are buttons: a message's prepared answers -- the question's `options`,
nothing when it offered none -- of which only the latest message's are live. The composer is always rendered, under
the `log`, and is open for writing whenever there is a
session; `myTurn` enables only Send. On
`review` the last message's buttons are View, Edit, Finish and Delete. With no
session id the screen takes the owner's latest session that is not
`finished` as its own (`resumed`, state rather than a redirect), and opens a
new one (`openAgentSession`) when there is none -- so there is always a
session, and the whole transcript, the opening's two questions included, is
drawn from its events by `Conversation` and by nothing else; Finish posts
the `finish` control before navigating, and a finished session renders
without composer or actions. The composer has no attach
control: files go with the first message, through the opening's "Attach a
sheet", and that first message is `startAgentSession`. The opening's rules
question (the `opening` event) is the same kind of buttons, one per pack,
resolved through `resolvePacks` and sent with `chooseAgentRules`; they stay
pressable until the first message.

Each `progress` event is its own bubble: `agent.progress.imported` ("Imported
field: …") in bold over the value from `progressEntry`. Assistant bubbles have
a fixed width (85%); the player's fit their text.

The chat sits on a bordered card, its own plain ground over the page's
pattern, but has no scroll of its own: the transcript is as long as it is and
the **page** scrolls, as in any chat application. The composer is a second
block straight under the card -- outside the `log`, in normal flow, not
`sticky` -- so the two can never cover each other; a pinned composer hides the
end of the newest message whenever the page is a few pixels short of the
bottom. **The composer is in the same place whether the chat is empty or
long**: the two blocks are a flex column at least as tall as the window
leaves, the card takes what the composer does not, and a chat that has
outgrown the window scrolls to the same foot. How tall that is is measured
(`frame`, in a layout effect, again whenever the page resizes) and not
written as a sum: a `calc(100dvh - 360px)` on the `log` is a guess at what
stands above it, and cannot be right on a phone, where the heading is gone and
the padding under the page is the safe area's. What is
above the chat is the box's own offset; what is under it is `main`'s bottom
padding. The page
follows the conversation until the reader scrolls **up**, and
picks it up again when they come back within 80px of the end, send a message,
or press the round arrow that appears at the foot of the window while they are
away.
Distance from the end alone does not stop it: a tall bubble arrives in one
step, and the smooth scroll after it reports every position on the way down,
which must not read as the reader having left. A `ResizeObserver` on the chat keeps the page at the end as well:
text is laid out after it is rendered (Markdown, fonts, wrapping), which moves
the end without any state having changed -- without it a freshly opened
chat stops short of its last message. The arrow rides the foot of the window in a
zero-height `sticky` row.

View, Edit, Finish (on `review`) and Delete are also the page's `actions`,
shown whenever the session has a character. A paused or failed
session adds a Resume or Retry bubble. 

The page's name in the trail is `chatName(session.created, session.id)` --
the local time the chat was opened and eight characters of its id -- and is
drawn once the session has arrived, so it never begins as one thing and
becomes another.

`/ai-wizard/:sessionId` identifies a chat and is the only page of it. View and
Edit navigate to `/characters/:id` and `/characters/:id/build`: the chat writes
to a real character from its first message, so there is no preview page, no
draft editor and no Save. `BuildScreen` takes no draft id and `characterPath`
has one shape.

Questions and unresolved choices are handled in chat with suggested reply
buttons and free-text input. Sending waits for the current assistant run to end;
there is no Stop control. One [poll](polling.md) a second brings
turn status and new messages, without making the composer busy: the screen
sends the revision and last event id it holds, applies the answer through one
`merge` function, and asks again. It stops while the tab is hidden, for a
finished chat, and on a 404; a failed request shows the reconnecting notice and
is retried after a second.

**What arrives is not what is drawn.** The server answers in bursts: one model
response is a batch of writes and the question that follows them, and all of
it lands in a single poll. Drawn as it landed, that is a wall of bubbles at
once. `useReveal` (`features/characters/useReveal.ts`) stands between the
session and `Conversation` and hands the events on the way a messenger would:
each "imported ..." line after a 300 ms beat, the assistant's words typed out a
word at a time (no message taking longer than 2.4 s). Only a backlog of more
than twenty-five is hurried, and only to twice the pace. The player's own messages, and
text the model already streamed chunk by chunk, are not delayed. While
anything is still to come the screen treats the turn as the assistant's --
typing dots, Send and the reply buttons disabled -- so nobody answers a
question that is still being written. The message box itself is never closed:
the next message can be written while the assistant works, and sent when it
has finished. Two things skip the pacing: the transcript that was there when
the chat was opened, and a hidden tab. Reduced motion does not -- a pause is
not motion -- and changes only the CSS in `ui/app.css`, where the bubble's
slide and the dots' bounce become plain fades. Tests switch the pacing off
through the hook's exported `pacing.on`. New messages
scroll into view unless the user has scrolled up.

Private catalogue names appear in the shared `SheetBody`, and so do a
character's notes, on its Custom tab, so copies and shared sheets retain
custom content. Captions/errors have English
and Russian translations. See [agent.md](agent.md) for the tool/question
contract and the session lifetime.

### No "imported values"

The builder has no panel of "imported values" under its tabs -- every printed
value an import laid over the build, as a row with an Edit button -- because
the import writes what the builder writes: an imported character's scores are the ability-scores card and open
`AbilityScoresForm`, its traits are the Personality tab's written cards, its
level is the Level card. What an import still lays over the build (inventory,
coins) is shown by the sheet's Equipment and Items tabs.

### No picker writes a custom entry

No picker ends with **Custom…**, a way to write a race, class, background,
item or spell the rules do not have: offered on every question, it reads as one
more answer to it. Custom items have a flow of their own, a page reached from
the sheet and from a game (see [Equipment is what is worn; Items is what is
carried](#equipment-is-what-is-worn-items-is-what-is-carried)). Catalogue entries flagged `manual` are still offered in the
list with a Custom badge.

`CustomOptionsPanel` remains for entries a character already has -- an AI Wizard
import writes them for anything it could not match -- and draws them as
`BlockList` blocks inside the tab's panel, above Next, with the form to edit
one. It has no add button, and nothing else opens it. It does not draw
**notes**: those are the Custom tab's.

### Custom is what the player writes unasked

A character has things the rules have no field for -- a backstory, a boon a DM
granted, a debt. **Custom** is the last tab of both the build screen and the
sheet, and holds any number of them, each a **title and a text**.
`features/character/CustomNotes` is the one component both mount.

- **An item is a custom entry of kind `note`** -- a name and a description
  the server stores and gives no meaning to (docs/backend.md). So a note an
  old import left on a character is here too, where it can be read,
  rewritten or deleted.
- **The text is drawn as typed**, `pre-wrap`, never as Markdown: the player
  wrote it, and a stray asterisk is not formatting. The personality fields
  are drawn the same way for the same reason.
- **Nothing opens a dialog but the question before a delete.** Add turns into
  the form where the button stood; Edit replaces the item with its form.
- **A write reads the log's head first** (`getEvents`), as the sheet's other
  edits do, then posts or deletes through `/custom-options` and refreshes its
  screen. Neither screen hands a revision down, which is what lets one
  component serve both.
- **Read-only without `characterId`.** The owner has the tab even when it is
  empty -- it is where the first item is written; a sheet shared with a table
  has it only when there is something to read, and nothing to press on it.
- **In the builder it is not a `StagePanel`**, which would say "Nothing to
  answer yet" about a tab that never asks, and **Next never leads to it**:
  `stageAfter` follows tabs with open questions, so the last question's Next is
  still Finish and Custom is reached by its tab. It is not drawn until the
  character exists, since there is nothing to attach a note to.

## Admin is two tables, for one kind of account

`/admin` (`features/admin/`) lists every account and every character for a
superadmin: two tabs in one `TabRow`, **Players** and **Characters**, each a
`DataList` under its filters with a count and **Load more**.

- **It is drawn for whoever `GET /v1/auth/me` marks `admin`**, and only in the
  desktop navbar. It is the `SECTIONS` entry with `desktopOnly` and
  `adminOnly`; `DesktopShell` filters on the second, `MobileShell`
  drops the first. "Desktop only" means *not linked on a phone*: the
  URL still answers there, and `DataList` falls back to its cards.
- **The tab and every filter are the URL** (`?tab=characters&owner=...`), so a
  filtered table is a link. Switching tabs drops the other tab's filters. Text
  filters commit after a 300 ms pause.
- **A player row opens that player's characters** -- the Characters tab with
  `owner` set to their id. **A character row opens `/shared/:id`**, the
  read-only sheet a group member gets; the server lets a superadmin read any.
- **Characters filter by owner, id and visibility only.** Name, level and class
  are shown but are folded from the log on the server for the rows on screen,
  so there is nothing to search them by; see
  [backend.md](backend.md#a-superadmin-reads-everything-and-writes-one-thing).
- **A player row has one action where a private pack is installed: Private
  packs**, a sheet of one checkbox per installed pack, saved as the whole list.
  The action is not drawn on a server with none, rather than opening onto
  nothing to tick. With one, the table gains a **Private packs column** naming
  what each player has, read from the listing itself (`packs` on a row) and
  re-read after a save. It is the only thing on the screen that writes.
- `usePaged` is the offset/limit paging `SpellsScreen` has, written once for
  both tables: a new filter set refetches from the top while the old rows stay
  dimmed, and a page that answers after the filters moved is dropped.

## Homebrew

The Homebrew pages list built-in, owned and group-shared packs. The editor
uses a recursive typed form driven by `/v1/packs/schema`, including reference
pickers, ordered rows, expressions and locale maps. Closed sections are expanded
lazily so large imported catalogues do not render every field at once. Saving
keeps an incomplete draft; validating and publishing use the server compiler.
Import accepts JSON or a ZIP containing one pack directory; export uses JSON. Dependency replacement is explicit and
preserves external version constraints. Every caption ships in English and Russian.

The builder's first Rules tab shows the rules edition followed by compatible
pack selection, using the same collapsible cards and Confirm/Clear actions as
race selection. Confirming packs preserves the previously confirmed edition.
Confirmed pack selections are final in the builder. Existing characters show
their pinned releases read-only; there are no migration controls.

The standalone spell browser searches all accessible published packs. Pack and
Book/source multiselects live beside the other spell filters, apply immediately,
and preserve their selections in the URL. The latest accessible release is the
default; packs with older versions offer a version selector in the filter area.
Changing one filter preserves the others and resets pagination. Book choices
follow selected packs/releases; unavailable book selections are removed. Exact
release contexts travel into spell detail links, including existing `packs=` URLs.

Character spell and cantrip selectors reuse those filters over their pinned
catalogue without changing rules or eligibility. Filtering never discards picks.
Pack/book tags appear once on catalogue and choice rows and in details. Source
tags use the theme’s light blue variant; full book names and release versions are
available on hover. Selected choices use a light background with readable text.
Book multiselect labels use compact codes and pack names (for example
`PH / D&D 2014`), and a selected value replaces the empty placeholder without
forcing a second input line.
Character-sheet summaries remain compact. All source names use the catalogue's
locale fallback. Packs without book metadata still show their owning pack.

Catalogue functions accept an explicit request scope. Builder descendants receive
the character scope through React context; spell pages receive a URL selection.
Private catalogue reads bypass the public collection cache so access is checked
on each request. Shared and owned character sheets load their pinned catalogue.
A group has a **Private packs** tab (`features/packs/GroupPacks`), drawn for
whoever has a pack they may grant: it grants a private pack to the table and
takes the grant back.

The prompts response also supplies the selected core's build policy: score
bounds, standard array, point-buy prices/budget and maximum level. Builder forms
consume that policy; legacy responses retain the SRD defaults. The sheet's level-up
button uses the same maximum rather than a fixed level 20.

Character creation treats confirmed rules and rule packs as final: the Rules tab
shows the pinned pack selection read-only, with no migration controls. Final
choices use neutral borders; red highlights indicate outstanding choices only.
Choice rows place their source badges on the right beside the name.
Spell *lists* are the exception and carry no source badges -- not on a spell
choice row, not in the compendium's list. A list is dozens of rows and the pack
and book were the same two badges on every one of them; the compendium's Source
*filter* stays, its options reading pack first and then the book inside it
("SRD 5.1 extended v2.0.0 / PHB"), the order the two filters stand in. The badges are in one place for a spell: inside its description
box (`features/spells/SpellDetails`), as the last of its facts, which is where
"which book is this from" is asked about one spell. On a spell's own page that
box also opens with its level and school (in the builder's preview the row
above already says both).
Homebrew has no entry in the navigation at either width; its direct routes
(`/homebrew`, `/homebrew/:id`) remain.
The custom background is listed last among the backgrounds -- it is the way
out of the list rather than one more entry in it -- and the server is what
orders it so (`customLast` in the catalogue handler; collections are otherwise
in slug order, so a pack file's own order decides nothing). Builder options
for equipment carry no artwork: a list of thirty weapons was thirty 66px tiles.
The builder's **Finish** -- both of them, the header's and the last tab's
Next, which share one `finish` handler so that neither leaves a character with
nothing on -- asks the server to dress
the character
(`POST /v1/characters/{id}/auto-equip`) before it opens the sheet: a build
equips nothing on its way, so that is the one moment a new character gets
anything on, and a failure there costs an empty paperdoll rather than the way
out of the builder. On the
sheet's Actions tab every row is a box of its own (`BlockList`'s `outlined`),
since inside a folding group an unoutlined row is a line of text adrift.
A pick keeps the picked option in view and does not jump to Confirm, which in
a long list would throw the page to its foot on every click. The spell filters share
one fixed width, so the row does not re-wrap when a value is chosen or the
language changes.

### Portraits

Character portraits have their own choice on the Personal tab. The shared avatar
editor also appears in Account's Profile section. JPEG, PNG and WebP uploads up
to 5 MiB open a rounded-square crop preview with dragging, position sliders and zoom.
Confirming produces a 256 × 256 WebP; cancelling keeps the saved image. New
character portraits remain drafts until creation; existing portraits save
independently of the name and retain drafts on failure.

The shared avatar displays character and account portraits as 48 px rounded squares with
a 1 px black border and the spell tiles’ slate background (`#252c3b`) and 8 px corners, including placeholders. Character headers, lists, group
rosters, game trackers and pickers use it. Names remain visible without redundant
Name column headings or tracker labels. Captions and errors are localized.


Default portraits are generated WebP emblems bundled in `web/public/avatars/`,
using the spell artwork's luminous illustrated style: soft colored shading,
bright highlights and restrained edge glow. Readability comes from one large
class symbol with broad structural shapes, generous gaps and little ornament,
checked at 48 px; the shading and highlights remain part of the artwork.
A character's starting
class selects the emblem, including multiclass characters; pack-qualified class
slugs use their final class name. Missing or unsupported character classes use the person placeholder. NPC copies
retain the starting class for their emblem; classless NPCs select a stable random
avatar from their entry ID. Accounts and classless NPCs share 24 generated icons
in `web/public/avatars/random/`, including animal heads and fantasy objects.
Their IDs determine a stable selection; the pool and its order remain fixed
to preserve assignments. Entries from older API versions also get this fallback.
Tracker players can use classes from the game's existing roster.

Defaults are display-only and never written into `image`. Uploads override them;
removing an upload reveals the default without offering removal of the default
itself. The app makes no generation requests at runtime. Assets are 128 × 128,
transparent WebPs. Generation prompts are in `docs/avatar-prompts.json` and
`docs/random-avatar-prompts.json`. Placeholders use a centered, integer-sized
SVG glyph inside the same tile.

The public `/avatar-gallery` page shows all 12 class emblems and 24 random icons
in separate responsive grids with localized names and larger rounded-square previews.


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
