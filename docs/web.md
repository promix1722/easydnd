# Frontend

The engineering doc for the [easydnd.org](https://easydnd.org) browser client:
layout, layer rules, and how it ships. For the Go API it talks to, see
[backend.md](backend.md); for the game model behind both, see [dnd.md](dnd.md).

Status: **real**. Sign-in (passkeys and Google), the character list with its
folders, character creation, the build loop, the account screen and the sheet
are all built and tested. **Level-up is not**, and this client does not offer
it -- see [Level-up is not offered](#level-up-is-not-offered). Neither is the
battle tracker: `/games` is a section in the navigation whose page says so.

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
worker picks up share **one** module registry and **one** jsdom rather than
forking a fresh pair per file. Rebuilding the Mantine, embla and React module
graph 48 times over cost 36s of imports and 78s of jsdom construction out of
229s total; sharing both took the run to 64s without a single assertion
changing. Passing `delay: null` to user-event (see `src/test/user.ts`) took it
to 38s from there, and the two rules below took it to 24s. On a four-core
machine, where vitest forks `availableParallelism - 1` workers of its own, the
whole suite is about fourteen seconds.

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

There used to be a second, isolated vitest project for the one file that mocked,
and a `MOCKING_TESTS` list to keep in step with it. It cost **2.4s of every
run**: vitest schedules an isolated project ahead of everything else and nothing
overlaps it, so 52 files took 13.8s and all 53 took 16.3s. One optional prop
bought that back and deleted the list, the second project and the rule that
policed them.

## Three rules about writing a test here

**A test runs at one viewport unless the tree branches on width.** Exactly seven
components do: `Columns`, `DataList`, `ModalSheet`, `SectionDeck`, `TabDeck`,
`SheetBody` and `RootShell`.

It stayed seven when the controls grew a per-width size, and stayed seven when
that was reverted -- worth a sentence because "make the theme responsive" is the
obvious wrong turn both times. Anything that must differ by width and is not a
*layout* belongs in `ui/app.css`, so nothing branches in JavaScript and nothing
re-renders at the breakpoint. It also means no test here can assert a rendered
size: the suite parses no CSS and jsdom evaluates no `@media`.

`ui/Page` is deliberately **not** a seventh, and its own test proves it rather
than asserting it in prose: the last case there compares the two renderings byte
for byte, the way `TabRow.test.tsx` does. The cheapest way to "fix" a future
layout problem in a shared page component would be to reach for `useIsDesktop`,
and that test is what goes red when somebody does. Nothing else
can, because the suite runs without CSS -- a `SimpleGrid cols={{ base: 2, sm: 3 }}`
renders one DOM whatever the width is -- so `describe.each(['mobile','desktop'])`
around a tree that reaches none of those six is the same assertion run twice
against byte-identical markup. `src/ui/TabRow.test.tsx` proves the point
directly: its last test compares the two renderings and they are equal.

That was 72 of the suite's 433 cases, weighted toward the slowest files --
`BuildScreen.test.tsx` alone ran 48 cases where 26 say the same thing. Where a
block runs at one width, the comment above it names this rule, so the next
reader knows it was a decision. Where a block still runs at both -- the group
screens, `ModalSheet`, `Columns` and `SectionDeck` themselves, and the rows of
`CharacterListScreen` -- it is because the swap is what the test is about.
`SheetBody` is there twice over: the deck it hands its sections to draws a
different tree at each width, and the sheet itself puts the two halves of its
first section in a different order on a phone.

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

Both of the update path's real bugs were found here rather than in production,
which is the argument for having it: the browser reusing a cached `sw.js`, and
the reload firing before the new worker had installed. See
[Why registerType is 'prompt'](#why-registertype-is-prompt).


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
it sounds -- see "Two caches decide what a returning visitor sees".

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

The character list is now the one screen that holds two resources -- its folders
and its characters -- which is the case that sentence used to leave open. It
still does not need a library. The two reads are independent and unkeyed: the
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

They are two surfaces now: the landing page, and `ui/SectionDeck` -- which is
what the character sheet becomes on a phone. That does not re-open the decision,
it settles it. The +10 kB is amortised over the page a visitor meets the app on
and the page they spend the most time on, and the second use is the one that
justifies a real carousel rather than a `scroll-snap` flexbox, because it is the
one a thumb is actually on at a table. The peer-range warning above is
unchanged, and is still the thing to look for in a lockfile diff.

## The die is real 3D, and it is paid for in one chunk

`ui/D20Scene.tsx` draws a genuine icosahedron with three.js and throws it
around a cannon-es physics world. It is by a wide margin the largest thing this
app depends on -- **158 kB gzipped**, against the 10 kB the landing carousel
cost and which was, until now, the whole of this project's dependency budget.
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
2. `vite.config.ts` gives the chunk a stable name through `manualChunks` and
   then names that chunk in workbox's `globIgnores`. Without this the service
   worker would have precached it on first visit -- it globs `**/*.js` -- and
   every visitor would have downloaded three.js in the background regardless
   of step 1. This is the step that is easy to miss and silent when wrong.
3. `scripts/check-layers.mjs` lists `three` and `cannon-es` as `ui/`-only
   vendors, so a feature cannot import either directly and quietly undo the
   split.

The measured result: `index-*.js` is 260 kB gzipped and contains no three.js at
all; `d20-scene-*.js` is 158 kB and is not in the precache manifest. Both are
worth re-checking after a dependency bump, because nothing fails if they merge.

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
ordinary phone chrome. It was a full-screen dialog first, and the dialog was
wrong twice over. It covered the header, so the menu you had opened the die
from was unreachable until you dismissed it. Cutting it back to sit *below* the
header fixed that and left it half-drawn over the page beneath, still carrying
a close button -- a second way out of a place you can already leave through the
menu, and one more thing on a panel that is supposed to hold nothing but a die.

A page needs none of it. The chrome is simply there, every section is one press
away, the back gesture takes you off the die rather than out of the app, and
leaving is navigating instead of dismissing.

It is a page but **not a section**. `/roll` is absent from `ui/sections.ts`, so
it owns no other paths, lights nothing in the navbar and starts no breadcrumb
-- and the desktop rail never offers it, because the section table is what both
shells map over and the die is a phone's. `shell/MobileShell.tsx` links to it
directly, below a divider, as the one entry in that menu that is not a section.

Being outside the table does not mean being nameless, though, and that
distinction cost a round. The phone's trigger is the only thing on screen
saying where you are, and it takes its label from `sectionFor` -- which answers
nothing for `/roll`, so the control fell through to the word "Menu" on a page
that this very menu links to *by name*. Naming a page after the control you
reached it through is worse than the fallback existing at all. So `MobileShell`
holds a one-entry `DIE` constant shaped like a `Section`, and the trigger, the
glyph and the tick all read `active ?? die`, with no second code path. The
fallback stays where it belongs: `/account` and a 404 have genuinely no name to
give, and inventing one would put a fourth entry in a menu that has three.

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
- **The live region is now the die's whole accessible surface.** The number is
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
pointer is simpler and was the first draft, but pressing anywhere in a tall
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
WebGL die is untestable in jsdom and asserting on a mock of one would be
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
the sign of that area now, which is the only thing standing between the die and
a backwards 7.

`embla-carousel` itself is now in `check-layers.mjs`'s `UI_ONLY_PACKAGES`.
`@mantine/carousel` was always guarded by the `@mantine/` entry, but the engine
underneath it ships its own types and nothing stopped a feature importing
`EmblaCarouselType` directly -- the same hole `@tabler/` came through, one
package wider.

It costs something in the test suite too. embla constructs a `ResizeObserver`
and an `IntersectionObserver` unconditionally, and jsdom implements neither, so
`test/setup.ts` installs inert stubs. They deliberately never fire: jsdom has no
layout, so anything they reported would be fiction, and a test that leaned on one
would be testing the stub. A carousel is therefore asserted on its structure --
its panels named, in order, in a named region, and whatever the call site does
about height -- and never on which panel is scrolled into view. That is why
`LandingPage.test.tsx` pins a height expression that still mentions both shell
offsets, and why `SectionDeck.test.tsx` pins the exact opposite: that
`--carousel-height` is never set at all.

It is also why the deck's tab strip reads React state rather than embla's
`selectedScrollSnap()`. Pressing a tab is therefore observable in jsdom and
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

`CharacterListScreen` used to draw one table with a `Select` above it reading
"All characters", a **Manage folders** dialog beside that, and a Folder column
so the rows could be told apart once you switched back to all of them. All
three are gone, and none of them was removed for tidiness: each was left with
nothing to do.

Every folder is now a bordered `FolderPanel` -- a heading you can collapse, its
own table under it, and its own **New character** beneath that. (**Import** sat
beside it and is withheld for now; `ImportCharacterScreen` and its route are
untouched, so putting the button back is one line in `FolderAdditions`.)
That change is worth four consequences:

- **There is nothing to filter.** A folder you can see the edges of is not a
  narrowing of a list; it *is* the list. The `Select` went, and `ui/Page`'s
  `filters` slot went with it, having lost its only caller.
- **There is nothing for the Folder column to say.** Inside a folder every row
  would carry the same word as the heading two lines above it.
- **`?folder=` is now a fact about where you pressed** rather than about what a
  control was set to. Every add button carries its own folder, which is also
  why each one is named for it -- three buttons reading "New character" is the
  same ambiguity as a column of "Delete"s.
- **Both resources drive the page's state.** A character carries a folder *id*,
  so a folder listing that will not load leaves nothing to head the panels
  with. The second, inline alert this screen used to keep for the folders
  request went with the filter it belonged to: while a folder was a filter,
  drawing the rows anyway was right; now there are no rows to draw.

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
nor jsdom. So it is never the only way to do something -- **Move up** and
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
Finish spent a while against the last tab instead, in a slot `TabRow` had for
it -- which said it was a control on the tabs, and left a button competing with
the strip for a 390px line. It is neither of those things.

**A row acts on that row.** Rename and delete sit in the row whose entity they
edit, with an icon each -- and whether they are drawn is the caller's rank at
*that* row's table, not at whichever one happens to be first. A DM at one group
and a player at another must not get a Delete on the second because they have
one on the first.

**A row's actions are data, not markup.** A screen hands `ui/DataList` a
`rowActions` list -- a label, an icon, a colour, a handler -- and `DataList`
draws it twice: **spelled out as buttons on a desktop**, where a table has the
room and a row's actions should be visible without opening anything, and
**folded behind one `⋮` menu on a phone**, where they have nowhere to go. A list
may ask for the menu at both widths with **`menuActions`**, and the character
list does: three buttons on every row made a shelf of characters read as a
control panel, and the folder above them already carries its own actions behind
the same glyph. That split is the whole of the difference between the two
renderings, and it is the
reason the rule below is now kept by construction rather than by twenty call
sites remembering it: each action carries its row's name as its accessible name
("Delete Ada"), because a column of buttons all called "Delete" is ambiguous to
a screen reader and to a test alike. Three of the eight lists used to spell that
out by hand and five did not.

It went the way `ColumnsSection`'s `aside` and `TabRow`'s `actions` went, and
for the opposite reason: those were `ReactNode` slots that lost their callers,
this was a `ReactNode` slot that could only ever be drawn one way. A cluster of
`<Button>`s cannot become menu items; a list of descriptions can be either.

Four is where a desktop row stops being able to lay them out and folds into the
same menu -- the threshold `FolderPanel` had already settled on for its own
header, and the reason it has one.

**Leaving is not editing.** It comes before rename and delete rather than
between them, because sitting in the middle of that pair reads as though it
were one of them.

Every one of these controls draws an `ACTION_ICON_SIZE` glyph from `@/ui`. It is
a constant rather than a literal because the sizes had already drifted once: the
icons were a mix of 14, 16 and the icon package's own 24, so the same three
actions were drawn three different sizes depending on which screen you were
looking at.

There used to be an `ACTION_SIZE` beside it, passed at every one of these call
sites. It is gone: the size is one theme default for the whole app, so a row's
buttons cannot drift from a heading's by being written down separately.

The heading's controls are capped to `MAX_TABLE_WIDTH` too, so they land on the
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
`ui/Page`. It used to be `MAX_TABLE_WIDTH` and it used to live inside
`DataList`, where it capped tables and nothing else -- so the two detail screens
that wanted their heading to line up with the table beneath it copied the number
by hand, and the character sheet, which is not a table, was never capped at all.
It is a fact about the page, so it is a layout token in `theme/tokens.ts`.

## Every page is the same page

Characters, Groups and Games do the same job -- a list of things you own or sit
at, each row opening onto a detail page -- and for a while no two of them were
built the same way. Games replaced the whole screen while loading where the
other two drew a spinner above the table; Characters wrapped its title in a
layout row that held nothing else; two detail screens hand-copied the content
width onto their heading; the character sheet capped nothing. `ui/Page` is the
one shape they now share.

**The last crumb is the heading, and the whole trail is one line at one size.**
There is no separate title line: the trail runs `Groups / Wednesday Night` with
the page's own name last, and every part of it is drawn the same. The section's
name used to be a heading on its list screen and a small crumb on everything
below it -- the same word in two sizes, shrinking as you walked in -- and that
is the thing this fixed.

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

That line is **smaller on desktop than on a phone**, which is the way round it
sounds wrong until you see why: desktop is where it carries the most -- a
section, a separator, a page name, a badge, an action cluster -- because the
phone drops the section entirely. It is a responsive value, not a branch.

**A section root renders no `<nav>` at all.** With an empty `trail` there is one
crumb, it is the heading, and there is nothing above it to navigate to -- which
is what lets "the trail replaces the title" need no special case for the three
list screens.

**Below `md`, the section's *word* is dropped -- not its crumb.** The phone's
one row of chrome already carries a control naming the section you are in and
opening the others, with that section's glyph beside it. So a crumb spelling the
word out again spends a 390px line restating what sits an inch above it, and on
a section root the heading *is* that word.

The way back is not a restatement, though, and an earlier version dropped it
along with the word: from a group's page there was no route to Groups but the
browser's own Back button. So on a subpage the section crumb stays as **its
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

**`loading` and `failed` replace the body, never the header.** This is what
retired the early returns: you still know where you are when the thing you came
for will not load, and "Try again" sits beneath a trail rather than alone on a
blank page. `pageState` adapts a `useResource` in one call. The loading line
takes an override for the two screens whose word says something the generic one
does not -- "Projecting the sheet...", "Reading the log..." -- rather than
imposing one word everywhere or letting four drift apart again.

**`Page` does not branch on viewport, and must not.** The actions wrap under the
heading on a narrow screen because the row is allowed to wrap, and the cap is
inert below 1024px. `Page.test.tsx` pins that by comparing the two renderings
byte for byte, the way `TabRow.test.tsx` does. What may branch on the width is
the handful of components that genuinely have to -- `Columns`, `DataList`,
`ModalSheet`, `SectionDeck`, `TabDeck`, `RootShell` and `SheetBody`.
`ScoreAssignment` was briefly one of them and is not: its gesture is the same at
every width now.

### What stayed different, and why

Unifying is not flattening. Two things survived because each does real work:

| Kept | Why |
| --- | --- |
| Groups' `TabRow` | Members and Characters are two views of one table. |
| Games' gate on **New game** | You can only open a game at a table you run. |

Two more used to be here and are not any more, both from Characters, and both
because the thing they were defending went rather than because the rule caught
up with them. The **folder filter** had a named `filters` slot on `Page`; when
folders became the page's structure there was nothing left to filter, so the
slot lost its only caller and was deleted. The **second error alert** existed
because a filter that would not load was no reason to refuse to draw the rows
it was going to narrow -- but a panel needs its folder's *name*, so now both
resources drive the page's state and there is one alert again. See
[A folder is the structure of the page](#a-folder-is-the-structure-of-the-page-not-a-filter-on-it).

Out of scope, deliberately: `/login`, `/legal`, the 404 and the join
flow. None of them is in a section and none is behind the signed-in chrome.

`/account` **is** in scope, and was the one screen that had drifted out of it.
It drew its own `Title` over its own dimmed line, so the profile page's heading
was a different size from the character list's, started at a different height,
and was capped by nothing at all on a wide monitor. It wears `Page` now. Being
in no section is not the same as having no shape: `sectionFor` answers null for
`/account`, which is exactly the case `Page` already draws as a heading with no
breadcrumb above it -- and the right one here, the phone's chrome having no
word for this place either.

## Sharing is reading, and it is one component

A group screen has two tabs -- Members and Characters -- because both are the
same table seen two ways and neither is a page of its own. `TabRow` already
existed for this. **Games are deliberately not a third tab**: see below.

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

## Games are a section, not a corner of a group

The header enforces it now. A game's trail is `Games / Thursday night`, and the
group it is played at is a **subtitle, never a crumb** -- a trail reading
`Groups / Wednesday Night / Thursday night` would say the opposite of this
section and would disagree with the navbar, which lights Games. A *shared
character* is the other way round, and consistently so: its trail is
`Groups / <group> / <character>`, because a shared sheet really does hang off
the group, which is what grants the read.

One wrinkle, recorded because it is a gap rather than a decision: that subtitle
points at the group without naming it, because `GameDetail` carries `group_id`
but no `group_name` -- only `GameSummary`, which the list screen uses, does.
Closing it means either the field on the detail response or an extra `getGroup`
on every game page for one word.

Games get their own `NAV_ITEMS` entry and live at `/games` and `/games/:id`,
beside Characters and Groups rather than inside one.

The reason is the same one that keeps folders out of Groups: **a game belongs
to a group the way a character belongs to a folder** -- the group is a fact
about the game, not the route to it. Somebody who plays at three tables wants
one list of their games, and making them open a group first is asking them to
remember where Thursday's game lives in order to find it. `GET /v1/games`
answers that in one request, and each row carries `group_name` so the list can
say which table without a request per row.

A game screen offers two ways to fill a roster, and they are shaped differently
because the things behind them are shaped differently.

**Add character from group** is a flat list. A game is played at exactly one
group, so there is one set of shared characters and nothing to branch on.

**Add my characters** is a tree of your folders, collapsed. A folder is how its
owner already thinks about their characters -- somebody with three campaigns'
worth of them knows which shelf tonight's is on -- and a flat list would make
them read every name to find it. It fetches everything up front rather than per
branch, because a character listing carries its own folder: one request covers
every shelf, and a request per shelf would be slower for no benefit.

Only characters that are not already seated are offered, and a folder with
nothing left to offer is left out of the tree entirely.

`GamesScreen` offers **New game** only to somebody who runs at least one table,
because a player has nowhere to put one and a dialog with an empty picker
teaches nothing. The picker is where the group is chosen, which is why creation
is the one call that names a group at all.

A shared character's sheet stays at `/groups/:id/characters/:character`, and
that asymmetry is deliberate: sharing *is* a group's doing and the group is what
grants the read, so the URL says so.

Two things on that screen are not cosmetic:

- **The default folder's menu has no delete control.** It is the folder an
  account is guaranteed to have, and the API refuses to delete it. Rename is
  offered, because what an account cannot lose is the folder, not its name.
- **The delete-folder confirmation states the character count.** Deleting a
  folder deletes the characters in it, and characters live in memory, so there
  is no undo and no backup. A dialog that only named the folder would be
  describing a smaller action than the one about to happen.

New character carries its own folder through as `?folder=`: there is one under
every folder's table, so where you press is where the next character lands.

### A row's dialogs belong to the screen, not to the row

Move, Copy and Delete used to be a `RowActions` component rendered inside a
table cell, which owned the open/closed state of two `ModalSheet`s and rendered
them from there. That stopped being possible when a row's actions became data:
`ui/DataList` renders no children, so there is nowhere inside a row to put a
dialog.

It should not have been there anyway, and the reason is worth keeping. A dialog
mounted in a table cell disappears when its row does -- and a successful **Move**
is precisely the moment the row leaves for another folder's table. It survived
only because the reload happened to arrive after the dialog had already closed
itself.

`useCharacterActions` is what replaced it: a hook that returns the three actions
to hand to each folder's list and the two sheets to render once at the bottom of
the screen, beside the folder dialogs that were already there. Copy has no
dialog, because it asks nothing.

## The build screen is a loop, not a wizard

`features/character/BuildScreen` reads `/prompts`, `/events` and `/sheet`, and
draws six tabs. It is still a loop rather than an N-step wizard, and it has to
be: prompts nest -- answering the "two skills" branch of a rogue's Expertise is
what brings the two-skill prompt into existence -- so the total number of steps
is not knowable until the last one is answered. The tabs are not steps. They
are the fixed set of *categories* a question can belong to, which is the
server's own `Prompt.Group` and not a taxonomy this client invented, in the
order `domain/stages.ts` states: identity, class, abilities, race, background,
personality. Class first after the name, because it is the choice the most
other choices hang off -- and the scores straight after it, because they are
what the class was picked *for*: a barbarian wants the 15 in Strength, and
deciding that while the class is still the last thing you looked at is the
difference between building a character and filling in a form. Personality is
last and is the only tab that asks nothing about the rules -- see
[below](#who-the-character-is-is-its-own-tab-and-its-own-words).

### The tabs are a deck, so a phone can swipe between them

The five tabs are `ui/TabDeck`. **On a phone** it is a strip over a carousel of
all five panels, every one mounted, one on screen: pressing a tab scrolls the
carousel to it and swiping the panel reports the tab it landed on, and neither
can drive the other in a loop -- scrolling to the slide embla already holds does
nothing, and the deck only reports a slide that is not the one the caller asked
for.

**On a wide screen there is no carousel at all**, only the strip and the panel
that is showing. The carousel answers a phone and nothing else: there the panel
is the biggest thing on screen and a swipe across it is the cheapest gesture
available, where reaching back up to a 60px tab is the dearest. With a mouse the
tabs are one click away, a drag across the page is how you select text, and
mounting five panels to show one is five times the work for a gesture nobody
makes. That is the branch, and it is why `TabDeck` is on the list of components
whose two renderings differ rather than the list whose renderings match.

**A press scrolls, a swipe is left alone, and anything else jumps.** Sliding
from one tab to the next answers a press -- it shows which way you went. The
same slide arriving unasked is the page moving while you are reading it, which
is what a cold load of the build screen did until `TabDeck` told the two apart:
it opens on the first unanswered category, so every load began on *identity* and
slid sideways off it.

A swipe is the third case and it took a bug to notice: embla selects the slide
the moment the gesture decides and is still settling on to it, so a deck that
"synced" to that selection cut its own animation short. The deck read as jumpy
next to the sheet's, which nobody had swiped hard enough to see. It now checks
`selectedScrollSnap()` and does nothing when the carousel is already where it is
being asked to go.

This is the same object the character sheet's phone rendering already was, and
that is why it moved into `ui/`: five stage tabs a player leafs between are
seven sheet sections under a different name, and two copies of a two-way embla
sync are two copies of the one thing in it that goes subtly wrong.
`ui/SectionDeck` is now the *desktop* half of a sheet -- where a section knows
whether it is a bare row or a bordered panel -- and hands the phone half here.

Two things follow from every panel being mounted, and both are worth knowing.
Where a block sits is remembered per tab rather than for the screen as a whole
(see [One block per choice](#one-block-per-choice)): with all five drawn at
once, one shared memory would let the place a dropped answer vacated under
*class* be claimed by whatever arrived next under *background*. And a test
about what a tab holds has to say which tab -- `BuildScreen.test.tsx` scopes
those queries to a slide, because a query against the whole document now sees
all five categories at once.

The deck does not swipe while the character does not exist. A tab press there
*creates* one rather than moving anywhere, so a swipe would be a gesture the
screen answers by refusing to move, and a deck that snaps back is worse than
one that never gives.

Keyboard focus now walks all five panels rather than stopping at the end of
one, and the strip follows it: that is embla's own `watchFocus`, which scrolls
a focused slide into view and emits the same `select` a swipe does. Nothing in
`TabDeck` implements it, which is worth saying because the obvious hand-rolled
version is a focus handler that fights the one already there.

The screen opens on the first category with something required outstanding,
and that is the whole of the help it offers: **answering does not move you**.
A tab changes when a tab is pressed, and never on the way back from a write --
answering one question is not a request to be asked another, and a player who
has just chosen barbarian is usually looking at what barbarian brought with
it. That has no exception for the first answer either, which for a long time it
did: see
[Creating is answering the first question](#creating-is-answering-the-first-question).
There is no Next in the tab row for the same reason: the order is the player's,
and a control that walks the tabs in the server's order is a wizard's stride in
a screen that is not a wizard. `Finish` sits against the last tab rather than
across the row from it, because it is the thing to do after them.

A `Next` does appear **under the list, once a tab has nothing left to answer**,
and only then. That is not navigation -- the tabs are always there -- it is the
end of a piece of work saying where the next piece is, at the moment when that
is the only thing left to say. It goes to the next category with something
*required* still open, wrapping round, and it names none of them: a finished
character always has an optional prompt somewhere, and a Next that walked to
that would never let anybody stop.

Three requests, because they answer three different questions: `/prompts` says
what is still open, `/events` says what was decided and in which entry, and
`/sheet` says what all of it adds up to. The sheet cannot be folded from the
log in the browser -- an ability score improvement's increments are derived at
projection time -- so it is fetched rather than computed.

**Nothing can be answered before it is asked.** Every tab is freely clickable,
because a tab is a place to look as well as a place to answer, but what can be
answered on one is exactly what `/prompts` returned for it. One surface draws
those questions -- the build tab, as blocks that open -- and it is the only one
in the client. There used to be a second: `OutstandingChoices` listed the same
prompts above the character sheet, read-only, with a button beneath it. It is
gone, and what replaced it is smaller than a list. See [what the sheet says
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
and twenty arrows has no slug of its own.

The exception, and its bounds, is the character's **inputs**: a name, an
alignment and the six ability scores. They settle a value on the sheet rather
than naming a catalogue entry or answering a grant, so each is stored as an
addressed change -- and the prompt, which says the entry is a `change`, has
nowhere to say to which path. `BuildScreen`'s `INPUTS` is that table and is
deliberately the only place a path is written down. It is worth knowing why it
exists: an alignment is namespaced `character/alignment` exactly like
`character/race`, this screen read the namespace as the shape, and the
`change` event it posted -- naming an alignment, changing nothing -- was
accepted by a server that could attribute it to no prompt. The alignment
simply never saved, with a 200 to say so.

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
there. It used to run along the same line as the name, cut to 120 characters,
which gave a list of six draconic ancestries six half-sentences and no whole
one. Under the name there is room for what the compendium actually wrote, and
under *only the picked one* the list stays a list -- the description is shown
where it is being decided about.
Three questions are genuinely not that shape and get a form each: a name
(`NameForm`), the six ability scores (`AbilityScoresForm`, below) and the four
roleplaying lines (`WrittenForm`). `StagePanel` chooses between the four by the
kind of the prompt, and that is the only place in the client mapping a prompt
kind to a control -- which is what let `PromptCard` stay exactly as it was
while three new kinds arrived.

None of the four says what the question is, and there is no exception. The
block they open inside is headed by the choice's own name, and a surface that
repeated it -- "Two more languages", then "Choose 2 more languages" -- would be
asking twice. `NameForm` used to be the exception, on the argument that "What
are they called?" is the first line anybody reads in this application: it
carried that heading *and* a field label under it, so the first screen anybody
sees said "A name", "What are they called?" and "Name", three times over one
empty box. The block says it; the field carries an `aria-label` for whoever
cannot see where it sits.

### One block per choice

A tab is one list, and every choice on it is a block that opens onto its own
answering surface. There used to be three places for one thing: what had been
decided in a card at the top, what was left in a card under it, and -- detached
at the bottom -- whichever question was in hand. Answering meant reading a name
in one card and finding its options in another.

A decided choice and an open one are the same object at two moments, so they
are one list rather than two sections: the choice of a race *is* the question
"which race?" once it has an answer. `features/character/blocks` merges them,
sorted by level with everything un-levelled first, which reads as the story it
is -- took rogue at 1, still owes two skills at 1, gained a level at 2. What
tells the two apart is that an open block is drawn to stand out, not where it
sits.

**Nothing opens itself**, with one exception below. The screen used to open the
first open question of the tab; it has no way of knowing which of five a player
came here to make, and a surface that opens itself is one they have to close.
One block is open at a time, and answering closes it rather than advancing to
the next question.

**The list grows; it does not rearrange.** Level order decides where a block
goes the first time it is drawn, and after that it stays there:
`features/character/blocks` keeps a `BlockOrder` of where everything sits, and
a key it has not seen sorts to the end. There is one `BlockOrder` **per tab**,
because where a block sits is a fact about the tab it sits on and every tab is
drawn at once now -- see [the tabs are a
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

The trail reads `Characters / Ada`, and ends there. It used to carry a third
crumb naming what the screen does -- "Creation" -- which a screen full of
questions was not being asked, and which cost a line of a 390px trail to say it.
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
pre-empted, and a front door whose one row is shut reads as broken. There is no separate create screen because
there was never a second thing being done -- and creation used to carry the
score method and all six numbers, which meant eight selections in one log entry
and nothing a player could point at and change. The scores are an ordinary open
choice now, answered on the abilities tab and written as their own entry.

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

`/characters/new?folder=` carries the folder the character list was filtered to, so
whatever the list was showing is where the next character lands. Import did the
same while its button was offered. Absent, the server resolves the account's
default.

### The Stub button is a development build's second button

Beside New character there is a **Stub**, and it is there only in a development
build.
It posts to `/v1/characters/stub` and lands on a finished level-3 rogue -- the
character in `docs/reference_hexsheet/` -- so that working on the sheet, the log
page or the character list does not begin with a walk through five tabs.

It goes to **the sheet**, and that is the one place it differs from Import,
which goes to the build screen. An import answers no prompts, so there is always
something left to decide; a stub is finished, so the sheet is the thing worth
looking at. It carries `?folder=` like its neighbour.

Finished means the build screen's "still to choose" panel is **empty**, not
merely that the rules call the character complete. Seven of its prompts are
optional -- acolyte's language and holy symbol, and the five questions about
who the character is -- and a stub that left them would have shown seven
untouched rows to anybody opening the build screen to look at one. The four
written ones are answered as changes rather than picks, which is what they are
now; the stub is a log the build screen could have written, so it writes what
that screen would. The only row that remains is the standing offer of a level.

The gate is `import.meta.env.DEV`, not a runtime check on a version or a
feature flag, and the difference is the point: Vite replaces it with a literal,
so a production build **drops the branch and everything behind it** rather than
shipping code it merely never reaches. The server does the same on its side --
the route is not registered outside `development` -- so neither half relies on
the other to stay hidden. See
[backend.md](backend.md#the-stub-builds-a-character-it-does-not-import-one) for
why it builds the character rather than importing it.

That elimination is why the button is **its own component**, `StubButton.tsx`,
rather than a few lines inline in the character list. A hook cannot sit inside a
branch, so inline the `useAction(createStubCharacter)` would have to be called
unconditionally -- and an unconditional call keeps the whole path reachable, so
the bundle would ship it and merely never draw it. Behind its own module the
one reference folds away with the branch and the module goes with it. The check
is a grep: `characters/stub` appears zero times in `dist/`, where
`characters/import` beside it appears twice. What the character list keeps is one
`useState` holding an error nothing ever sets, because that too is a hook.

It renders under Vitest, since `DEV` is set there, which is what makes the two
tests in `CharacterListScreen.test.tsx` possible. That a production bundle omits
it is not something a test in that environment can observe.

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

An answer to a *nested* prompt -- a rogue's Expertise, a half-elf's ability
bonuses -- cannot be re-posed directly, because the options that made it up
arrived with a prompt the server stopped emitting the moment it was answered.
Opening one of those blocks therefore drops the entry, which reaches the same
place from the other side: the question comes back outstanding, and
`reclaimPlace` holds the block's own place for it, so what returns is where
what went was. The player is shown none of that -- the same press, the same
outcome, a moment longer -- and it is asked about on the same rule as
everything else: only if another answer cannot survive it.

The question that comes back is also **open**. `done` takes the key of a block
that does not exist yet, and the reread is what brings it into being; without
that the row you pressed turned back into a shut question and wanted pressing
again, which is the same gesture twice for one intention. The key is knowable
in both directions: a dropped entry names the prompt it answered, and a *nested
option's key is its inner prompt's slug*, so answering "a martial melee weapon"
rather than the greataxe beside it says exactly which question is about to
arrive.

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

The two that deal out a set share `ScoreAssignment`: a pool you take from and
six abilities to put numbers on. Two gestures, one operation: drag a number, or
tap it and tap where it goes. The tap is not a fallback -- it is the keyboard's
path and the one a screen reader can follow, since every half of the surface is
a real `<button>`.

**The drag is pointer events, not HTML5 drag-and-drop**, and that is what makes
it a gesture a finger can make. `draggable` + `dragstart` is a mouse protocol
that no mobile browser fires for touch, so for as long as this surface used it,
the instruction "drag a number onto an ability" did nothing whatsoever on the
device the app is used on at a table -- and there is one implementation now
rather than two, because a single pointer API covers finger, mouse and stylus.
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
phone a drag towards Strength swiped to the next tab, and even a tap on a 44px
square pressed with a thumb often did nothing at all. `TabDeck` passes
`watchDrag` a predicate instead of `true`, and a surface marked with the
attribute keeps its own gestures: the deck is still swiped from anywhere else on
the slide, and the tabs never stopped working. `ScoreAssignment` marks the whole
surface rather than each control, because the drift that has to be tolerated
happens between a press and its release and the release is not always over what
was pressed.

The number follows the pointer as a ghost, and it is drawn in a `Portal` for a
reason worth writing down: `position: fixed` is positioned against the nearest
*transformed* ancestor rather than against the viewport, and the carousel's
track is translated on every frame. Drawn in place it was laid out against that
track and clipped by its overflow -- the drag landed and nothing appeared to
move. For the same class of reason the gesture's state is read from the render
its handler was made in rather than through a `setState` updater: an updater
must be pure, placing a number is not, and StrictMode invokes it twice to prove
the point.

The suite drives the drag by hand, with `document.elementFromPoint` stubbed:
jsdom computes no layout, so the browser's one contribution to the gesture is
the one thing a test has to supply. The threshold, the swap and the click that
must not undo the drop are the real code.

Dropping onto a taken ability swaps the two. **Dropping one anywhere that is not
an ability returns it to the pool** -- undoing a placement is half of dealing six
numbers out, and its only gesture used to be tapping the number and tapping it
again, which is not what anybody tries on a surface that says "drag". Tapping a
placed number twice still works. Nothing can be confirmed until all six are placed,
because six numbers and five decisions is not an answer. Both halves are drawn
at least 44px square: a number waiting to be placed used to be a badge, which is
a few millimetres of target for the one gesture the whole surface exists for.

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
that method actually produced six. Switching to it from an unplaced array used
to show six zeros -- under its own stated minimum of 1 -- because an ability
nobody has dealt to reads as a 0, and confirming then stored six 10s. The log
and the screen disagreed about what had been entered, with nothing on screen to
say so.

The dice live in `domain/abilities` too, and take the die as a parameter: a
test that cannot say what was rolled can only assert that six numbers came
back, and "between 3 and 18" is not a test of dropping the lowest. The
algorithm is the SRD's rule verbatim -- roll four d6, total the highest three,
six times -- and SRD 5.1 has no re-roll-if-unplayable clause, so neither does
this.

### Who the character is is its own tab, and its own words

`personality` is the last tab and the only one that asks nothing about the
rules: a personality trait, an ideal, a bond, a flaw and an alignment. They are
the *background's* questions -- it is the acolyte entry that suggests what an
acolyte tends to believe -- and they used to sit under background for exactly
that reason, which put five questions nobody has to answer in front of the one
required question on that tab. A group of their own is the server's change, not
this client's: `domain/stages.ts` gains a line, and nothing else here had to
learn the tab exists.

The four are **written, not picked**. SRD 5.1 prints eight of each and the
compendium carries them, and the prompt used to *be* that menu -- eight options
and no way to say anything else about a character who is yours. The state
behind them was free text the whole time (`State.Identity.PersonalityTraits` is
`[]string`), so what changed is only that the prompt stopped pretending
otherwise. The suggestions are still in the compendium for anybody who wants to
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
asked now is one answer in the player's own words, which is a sentence and
sometimes several. Nothing written is the same as not answering, and these are
optional, so the button simply stays disabled.

It is a fixed three rows rather than an autosizing one. Mantine's autosize is
`react-textarea-autosize`, which measures through a listener jsdom has no
element to attach -- the field could not even be focused under test -- and
`vi.mock` is not available to paper over it. Three rows and a scrollbar is a
smaller loss than a control the suite cannot drive.

The prompt is told apart from a menu the same way the six starting scores are
told apart from a level-up improvement: **by whether it offers anything to pick
between**. That is the server's own statement of what may be picked here, not a
slug this client has memorised.

### Level-up is not offered

The server poses "gain a level in which class?" and everything that follows
from it under the `advance` group, and this client filters that group out of
everything it draws: no block, no answering surface, no control.
Taking a level does not work -- the event the client would post is recorded as
a no-op -- and a question that appears answerable and silently changes nothing
is worse than a question that is not asked. It is the same judgement that took
the "Level up" button off the sheet.

Levels a character already *has* stay visible as settled blocks on the class
tab, and stay read-only -- blocks with nothing to open, which is why
`ui/BlockList` draws one as a statement rather than as a control that refuses. They are facts about the character -- an imported one may
well have several -- rather than controls, and editing one would drive the same
machinery that cannot take one. `domain/stages.ts` is the single line that
reverses all of this on the day it works.

## The sheet decides what order things come in

Six sections, in one list, in the order a player reads them: who the character
is and the abilities everything else is derived from, the body's state, then the
skills, the proficiencies, the traits and the gear. `features/character/SheetBody`
is that list, and `ui/SectionDeck` draws it -- across the page on a wide screen,
and as a deck of tabs on a phone.

### What the sheet says about an unfinished character

A badge on the name reading **Unfinished**, and only when `/prompts` comes back
with something in it. The badge is the whole message: the sheet does not
enumerate what is open, because enumerating it put the build screen's work on
the page nobody came to build on -- an alert, a list of five questions, and the
sheet itself pushed below the fold on a phone. The screen that answers a
question is the screen that lists it.

The mark used to be the *button*, which appeared and disappeared with the same
prompts. That made one control say two things -- here is the way in, and there
is work left -- so a finished character had no way back to the build screen at
all. **Edit** is on every sheet now, and the news is a badge, where a rank and
"Read only" already go.

Two things went with that list. `features/character/OutstandingChoices` had no
other caller and is deleted rather than kept for a level-up page that does not
exist. And the **Event log** link that used to be the sheet's only action is
gone from the page for now -- `/characters/:id/log` still serves it, and is
still the unabridged record, but nothing in the client links there.

A `/prompts` that *failed* draws no badge. The request is deliberately
survivable -- a sheet is worth drawing with a second request down, which is the
same bargain the compendium lookups make -- and the honest reading of "I do not
know what is open" is to say nothing rather than to guess. Edit does not depend
on that answer and is drawn either way.

### On a phone the sheet is a deck, not an accordion

One row of tabs under the character's name, one section on screen, and a swipe
between them. **Nothing opens and nothing closes**, which is the change: the
carousel decides what is visible, so a section has no shut state to be in.

What it replaces is a `Columns` accordion, and the two answer different
questions. An accordion is right for a page of two or three panels where the
answer is usually in the first and the rest are detail. A character sheet is
not that shape. It is six things
a player leafs between at a table, none of them subordinate to the others, and
an accordion made reading one of them a gesture: open it, and possibly shut
three others first. It also put the headline numbers -- identity, the ability
cards, the vitals -- above the accordion where they were never reachable except
by scrolling past them. As slides they are tabs like any other.

The strip and the carousel are `ui/TabDeck`, which the build screen's five
stage tabs also draw -- see [the tabs are a
deck](#the-tabs-are-a-deck-so-a-phone-can-swipe-between-them). Under it is
`ui/TabRow`, unchanged, because six tabs do not fit across a 390px screen and a
strip that scrolls sideways is the whole of what it is. It
scrolls away with the page rather than pinning under the header: that is one
fewer row of chrome on a screen this app has already spent an argument buying
back (see [Two views, one codebase](#two-views-one-codebase)), and a swipe
changes section from anywhere on the slide, so the strip is not the only way
through.

**A slide is as tall as the tallest slide**, and a section sits at the top of
its own rather than being stretched down it. Eighteen skill rows therefore leave
a screen of blank space under the three rows of proficiencies. The alternative
is to measure whichever slide is showing and size the viewport to it, and that
is a `ResizeObserver` reading a layout -- which jsdom does not compute, so the
suite could neither exercise it nor catch it breaking. The honest cost of that
plus the non-sticky strip: scroll to the foot of Skills, swipe, and you are a
long way down a mostly empty Identity with the tabs off-screen above.

The tab and the panel heading are one string, written once in `SheetBody` --
which is what [a category's word appears exactly
once](#a-categorys-word-appears-exactly-once) asks for here. On the phone the
heading is not drawn at all: the tab is on screen naming the section, and the
slide repeating it underneath would be the word twice in two inches.

The first tab is **`Main`**, and it is the one label here that names a place
rather than its contents. The section holds two things -- the identity table and
the ability cards, merged because they were the two thinnest slides on the sheet
and neither filled a screen alone -- so a tab naming either would send a reader
looking for the other one somewhere else, and a tab naming both is the longest
thing in a strip of six. A place is what it actually is: the tab you are on when
you open a sheet. Note what is *not* renamed: `Vitals` stays `Vitals`, and the
abilities keep no mention of saves, which would put back the two-list vocabulary
that merging the saves into the cards deleted.

### What a phone spends its height on

Three of the sheet's measurements are tighter on a phone than on a wide screen,
and they are the ones a phone can afford least:

- **The gap between blocks in a slide is `md`, where the wide layout stacks them
  `lg` apart.** It only has to say "different block", and it still does, because
  the cards *inside* a block are `xs` apart. On a wide screen the same gap
  separates things that sit beside other things, so tightening it there buys
  nothing anybody sees.
- **The card grids gap at `xs` from `base` and `sm` from `sm` up** -- the ability
  row, the two vitals rows and the identity table's own columns.
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

**On a phone the main section leads with the ability cards, not with the
identity table**, which is the reverse of the wide screen and the one thing on this sheet
whose order depends on width. A wide screen shows both at once, so it reads in
the order a sheet is written in: who the character is, then what everything
about them is derived from. A phone shows one slide, and the first thing on the
one you land on should be the thing reached for mid-turn -- six modifiers, not a
background. It is swapped **in the document**, by the one `useIsDesktop` call in
this feature, rather than with a `column-reverse` that would leave the page
saying one order and the screen showing another. Two static blocks would survive
that mismatch; a habit of it would not, and a test can assert a document order
where it cannot assert a cascaded style.

Two of the six -- Main, Vitals -- are `desktop: 'full'` and
are drawn bare on a wide screen, no border and no heading, exactly as they
always were. Their titles exist only to name a tab, which is also why the merge
costs the wide screen nothing: two bare sections stacked and one holding both
are the same page. The other four are `desktop: 'panel'` and land in one
two-column grid, which is the same page the two stacked `Columns` grids drew:
`SimpleGrid` sizes each row independently and the row gap is the `lg` the
`Stack` between them used.

Both sheet screens get this, because there is only one of them to get:
`SheetBody` is drawn by the owner's sheet and by the one a group member opens
for a character shared with their table, and the whole point of that split is
that the two cannot disagree about what a sheet is.

`features/character/CharacterSheetScreen` prints ability scores and saving
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

That merge changed what a *missing* ability means, and the change is worth
stating because the older rule said the opposite. Scores and saving throws are
two projections and neither promises all six keys. The cards used to be driven
by the scores alone, and an ability with no score drew nothing at all -- a
blank card claiming a score that is not there being worse than a row of five.
Now that the card is the only place either projection is drawn, dropping it
would swallow a save the server did send. So the grid walks the **union** of
the two (`abilitiesOnSheet`), and a card with no score prints a dash where the
modifier goes and still prints its save. It claims nothing it was not sent, and
hides nothing it was.

Above them, `features/character/IdentityTable` says who the character is as
labelled pairs, in **four columns of two**: name over level, race over subrace,
class over subclass, background over experience. The sheet used to say this in
one dimmed line under the name ("Elf · Wizard 1"), which reads well and answers
badly: a line has no room for the subrace or the subclass, and a reader looking
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
page. Names come from the compendium, five session-cached collections flattened
into one map keyed `"<collection>:<slug>"`; keyed by collection as well as slug
because two collections may share one, and a bare slug map would let a
background rename a class. Without it the table falls back to title-casing, and
"half-elf" becomes "Half Elf" rather than "Half-Elf".

Under the cards is a second headline row, `features/character/Vitals`: passive
Perception, the spellcasting numbers, speed, vision and Hit Dice. Four of those
five were being projected and drawn nowhere at all -- speed and senses were on
every sheet the server sent and on none it showed, and Hit Dice sat in
"Resources and gear" beside the backpack, which is the wrong shelf: it is a
fact about the body, not about the kit. It is drawn in one place now, not two.

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

The skills beside them are a different case, and they used to be alphabetical
for a reason that has since expired. `features/character/SkillsPanel` draws
**all eighteen**, ordered by how trained the character is — Expertise, then
proficient, then half, then untrained, alphabetical within each block. When the
panel listed only the six a character was proficient in, the alphabet was the
only sequence a reader could search. Eighteen rows of which six matter is a
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

What the compendium *is* asked for is each skill's **name and governing
ability**, fetched with the session-cached `getCollection('skills')`. The name
matters twice: it is in the negotiated locale, and it is the only spelling that
gets "Sleight of Hand" right, where title-casing the slug capitalises the "Of".
That request failing costs the ability tags and falls the names back to the
slug; it does not stop the panel drawing, on the same reasoning as the prompts
fetch above.

Training level is carried by a mark — `ui/ProficiencyMark`, one glyph filling
in across the four levels, with Expertise ringed rather than merely fuller
because it is the bonus counted twice rather than more training. The mark is
what separates the rows, and the dimming of untrained ones is a **second**,
redundant channel: a panel distinguishing eighteen rows by a shade of grey
reads to nobody on a monochrome print and to nobody who cannot tell the two
greys apart. **There is no filter over the top of it**, and there used to be: a
"Hide untrained" toggle collapsed the panel to its trained rows. It was
answering the question the eighteen rows exist to answer -- what do I roll for a
skill nothing trained -- by taking those rows away, and it was the only control
on the whole sheet. The two channels above already separate the trained from the
untrained without anything to press first.

Skills stays at half width rather than spreading now that the saving throws
have moved up into the ability cards: its rows are name, ability and bonus, and
a full page width would set the bonus so far from the name that the eye travels
back along an empty line to pair them. **`features/character/ProficienciesPanel`
takes the half beside it** -- everything the character is trained with that is
not a skill or a save, grouped into Tools, Weapons and Armor. It used to be one
comma-joined paragraph at the foot of "Traits and features", which is a
sentence to be read rather than a list to be searched, and which filed a tool a
player rolls with beside a racial trait they never touch again. It is drawn in
one place now, not two.

**"Traits and features" and "Resources and gear" are lists for the same
reason.** Both were the arrangement that argument was made against and kept it
one panel longer: a dimmed label with everything under it comma-joined onto one
line, so the twelfth trait and the first were the same visual object and finding
one meant reading the sentence. They are rows now -- one item to a line, in
`ProficienciesPanel`'s own grid, one column on a phone and two from `lg` where a
panel is half the page. There is no third spelling of this on the sheet.

**Every list on the sheet is marked the same way.** Skills and proficiencies are
marked by `ui/ProficiencyMark`, whose lowest level is an empty ring; the four
lists that have no training level to report are marked by `ui/Bullet`, which is
that same ring at the same diameter, weight and indent, and nothing else. Two
lists side by side whose items began at different indents would read as two
kinds of thing, when what they are is one kind of thing with and without a
number attached.

`Bullet` is a separate component rather than `ProficiencyMark level="none"`, and
the reason is what that component *says*: it names itself "Not proficient" and
carries a tooltip explaining proficiency bonuses. Drawn beside "Darkvision" that
is a false statement about a racial trait rather than a decoration, so `Bullet`
is `aria-hidden` and says nothing at all -- the same convention every other
inline glyph here follows when it sits beside a label that already names the
row. What that split costs is two copies of one ring, and `Bullet.test.tsx`
pays it: it renders both and compares the circle attribute for attribute, so
the day one of them changes diameter the other fails rather than quietly
drifting.

The resources panel gained its labels in the same change. A class pool and a
spell slot used to be loose lines with nothing over them, sitting above two
labelled groups; every group is named now, so the panel is four lists rather
than two lists after some sentences. **A group that does not apply is left out
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
and the *type* that decides which rows get it comes from the compendium, via
the same session-cached `getCollection` the skill names come from.

The stat row above leads with hit points, then armor class, initiative and
proficiency. Hit points are the one number that moves between one glance and
the next and the one reached for mid-turn, where armor class is settled at the
start of a fight; at two columns on a phone, first position is the only one
visible without the eye travelling.

## The log has its own page, and it never asks for the sheet

**Nothing links to it at the moment.** The sheet's Event log button came off the
page (see [what the sheet says about an unfinished
character](#what-the-sheet-says-about-an-unfinished-character)), so the route is
reached by typing it. The screen and its route are kept whole rather than
deleted: what was wanted was one fewer control on the sheet, not the loss of the
one page that can answer "why do I have this proficiency?" when the projection
has gone wrong.

Its breadcrumb trail is `Characters / Event log` -- two crumbs, where every
other detail page has three and names the thing it is about. That is the same
rule, applied one step further: the character's name lives on the sheet, and
fetching it for a crumb would reintroduce exactly the dependency this page
exists without. The trail says less instead. `/characters/:id/build` is the
asymmetry worth noting -- it already holds the sheet, so it gets the full
`Characters / Ada / Build`.

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

## Importing shows the report before the character

`features/characters/ImportCharacterScreen` uploads a sheet exported from
another tool, and it is deliberately two steps rather than one: the file is
posted, and then the **report** is shown before anything navigates away.

An import is lossy by construction -- SRD 5.1 publishes one background and one
feat, so a sheet from a tool with the full rules always leaves something behind
-- and going straight to the new character would make the import look lossless
and let the player find out otherwise from a wrong number weeks later. So the
screen names what did not come across, says how many choices are still
outstanding, and offers a way on.

That way on is the **build screen**, not the sheet. An import answers no
prompts (see [dnd.md](dnd.md#importing-a-foreign-sheet)), so an imported
character always has something left to decide.

The file's bytes are the request body, sent through `request`'s existing
`rawBody` option: the route takes the export itself, not a wrapper object, and
re-encoding JSON that is already JSON buys nothing.

## Private routes branch, they do not redirect

`routes/Private.tsx` renders the landing page to a signed-out visitor instead
of navigating away, which is the same rule `HomeRoute` follows: the URL never
changes on account of who is looking, so a shared deep link to a character
survives being opened by someone who has not signed in yet.

### An invitation link is the deep link that arrives at a stranger

Every other private route is followed by somebody who already has an account.
An invitation is the opposite: it is *sent to people who do not*, which makes
it the one link that must survive a whole sign-up before it can be used.

The token rides in the **URL fragment** -- `/groups/join#<token>` -- because a
fragment is never sent to any server, so it stays out of nginx's access log,
out of `Referer`, and out of any link unfurler that fetches the URL. It is then
posted in a request body, never in a query string. That is the safest place to
carry it and the least durable one, and three things used to lose it:

1. **`<Private>` branches, so the screen never mounts.** Whatever `JoinScreen`
   did to save the token ran only for visitors who were already signed in --
   precisely the ones who did not need it. `/groups/join` is therefore *not*
   wrapped in `Private`: `routes/JoinRoute.tsx` captures the token first and
   branches afterwards, the way `HomeRoute` does for `/`.
2. **`/login` is a different URL.** Returning to `from.pathname` dropped the
   search and the fragment, so an invitation link came back as a bare
   `/groups/join`. `LoginScreen` now rebuilds `pathname + search + hash`, and
   ignores a `from` that is not a path of ours -- history is attacker-reachable.
3. **Google leaves the origin entirely.** Nothing in a URL survives that, and
   it cannot: `currentPath()` sends `pathname + search`, and the server refuses
   any `return_to` containing `#` outright (`SafeReturnTo`). So the token is
   also copied into `sessionStorage` by `features/groups/inviteToken.ts` --
   session, not local, because an invitation is one visit's business and one
   left behind in a shared browser is somebody else's group.

A signed-out visitor gets `InvitePrompt` rather than the bare landing page.
That is a deliberate exception to "the landing page has no copy on it": they
followed a link somebody sent them on purpose, and a dragon with no explanation
does not tell them the thing they came for is one button away. It cannot name
the group -- previewing needs a session, and opening that up so a stranger
could read a group's name off a link is not a trade worth one sentence.

### Copying the invite link

A link nobody can copy is not a delivery mechanism, and the clipboard is the
second thing after passkeys to disappear outside a secure context: served over
plain HTTP on anything but localhost, `navigator.clipboard` is `undefined`.

The first version used Mantine's `CopyButton`. Its `useClipboard` hook does
notice -- it sets an `error` -- but `CopyButton`'s render prop passes only
`{ copy, copied }` and drops it, so the button stayed on "Copy link", nothing
reached the clipboard, and nothing said so. It worked on production and failed
in dev, which is the worst arrangement of those two outcomes.

`lib/clipboard.ts` replaces it and returns **whether it worked**: the modern
API when there is one, `document.execCommand('copy')` over a throwaway
selection when there is not -- deprecated, and the only thing that predates the
secure-context rule -- and `false` when neither is possible. The sheet then
says so and selects the link, so there is something the keyboard can still do.
Never a control that quietly does nothing.

Passkey and guest sign-in are `fetch` calls that never leave the page, so they
would need none of the above; Google is the one that does. One mechanism
covering all three is less to get wrong than two.

## Signed out and signed in, one build

There is one hostname, one bundle and one route table. `shell/RootGate.tsx`
branches on the auth state and picks the chrome: a loader while the session is
still unknown, `LandingShell` when anonymous, `RootShell` when signed in. At
`/`, `routes/HomeRoute.tsx` makes the same choice about the content.

Signed out, that content is a carousel of three panels -- build a character,
join a group, run an adventure. Those three are the whole of what this app means
to be, in the order you meet them, and committing to that shape before
committing to the words for it was the cheap order to do the work in: the
layout, the swipe and the accessible names settled first, against panels that
were literally empty, and the copy arrived after.

The captions in `routes/LandingPage.tsx` are the page's own copy now; the sample
text they replaced had done its job, which was to hold the shape while the layout
was settled. Two of them describe what the app does; `Run sessions` describes
intent, because the battle tracker is not built. That is the one to keep honest
-- a landing page promising it would be the only thing on easydnd.org that did.

It is also the one place in the project that uses **session** for a sitting at a
table. Everywhere else that is a *game*, and a session is being signed in -- the
distinction the README draws and the app's own navigation keeps. The landing page
is read by somebody who has neither, and to them "session" is the evening being
described; the word stops at the sign-in boundary.

Those three panels are also the three sections a signed-in visitor gets, and
since `/games` joined the navigation the third has somewhere to lead. What is
behind it says it is not built, which is the honest middle between a promise
with nothing under it and a section hidden until the day it works.

They also paid for a piece of the design to be removed. While the panels were
empty, `slideSize` was under 100% so the neighbours peeked: three identical
blank rectangles at full width read as one rectangle, and a swipe between them
appeared to do nothing. A panel that says something is already distinguishable
from the panel beside it, so the peek went and the carousel shows one panel at a
time. The border stayed, because a panel still wants an edge.

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
the wheel, and both did nothing.

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
they were going. What the old copy promised about the rules and level-up is
still either invisible until you are inside or better said on `/login`, which is
one press away and has room to say what each way in costs.

### A fourth panel, on a phone, that you can press

There is a fourth panel below the breakpoint: a d20 you can pick up and throw
-- grab it, fling it, and it caroms off the edges of the panel and settles on a
number. It is the only thing on this page that does anything, and that is the
argument for it.

**The panel carries its heading and no caption.** The scene takes the whole panel
and the heading floats over it, so the die rolls behind its own name; there is
nothing to say about a toy that throwing it does not say. Stacking the two -- the
first arrangement -- cost the die a heading's height on the one viewport where
the panel is narrowest, and a d20 with less room to travel is a duller throw. The
heading is `pointer-events: none`, without which it would be a dead strip across
the top of a toy that is grabbed and flung with pointer events on the canvas. It carried no words at all for a while, and was named
by an `aria-label` for exactly that reason -- the one exception to "label a panel
by its own heading" this page allowed. The exception is gone with the emptiness:
a panel whose name is on screen is named the way the other three are, and the
padding is on the heading rather than on the panel so the scene still reaches the
edges it bounces off.
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

One existing test changed shape rather than being deleted: `offers nothing to
press on the slides themselves` is now `... on the slides that describe the
app`. Its claim was that no panel is a *door* -- a carousel where one panel
navigates and two do not teaches the wrong thing -- and a die leads nowhere, so
the claim survives intact on the three it was written about.

Each panel is named by its own heading, through `aria-labelledby`, rather than
carrying a second copy of the words in an `aria-label`. While the panels were
empty the label was all there was, and a slide that announces itself as "slide 2
of 3" is unusable without sight; now that the words are on screen, two spellings
of one name is only how they come to disagree. Reachability *by name* is still
the accessibility contract the mark used to hold up -- moved, not dropped -- and
`LandingPage.test.tsx` pins the wiring and not merely the name, because an id
that stops resolving turns every panel back into "Carousel slide" without
failing a test that only looked one up.

### A sheet for the content to sit on

The pattern is a background, so anything laid straight onto it competes with it:
a table's header rule and its row separators are hairlines, and hairlines are the
first thing a busy ground eats. `ui/Panel` is the answer -- the `Paper` the
character list's folders have always used, generalised so a screen asks for one
by name instead of spelling out three props. It fills with
`--mantine-color-body`, so the pattern stops at its border in both colour schemes
with nothing said about either.

Five screens take one: the group list, the game list, a group's tabs, a game's
roster, and the whole of character creation. What stays outside it is the page's
heading, trail and actions -- they say where you are, and where you are is not
part of what you are looking at, which is also why this is not something `ui/Page`
does for every screen.

### The pattern behind every page

`ui/backdrop.ts` tiles a sheet of hand-drawn marginalia -- dice, swords, scrolls,
a dragon -- behind the main box of all three chromes, washed down to almost
nothing. Before it the app was flat theme colour everywhere except the landing
carousel, so signing in was a cut from three photographs to a blank sheet.

A photograph was tried here first and is the wrong kind of picture for the job:
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

**Every page takes it, and there are no exceptions.** There used to be two. The
die's screen at `/roll` opted out because a canvas with its own lit floor is
already a picture; the landing carousel opted out because it is three
photographs, and a washed-out fourth showing through the gap between two panels
is the page arguing with itself. Both are gone: one ground under every page is
worth more than either argument, and at an 88% wash what shows through behind a
canvas or between two slides is texture rather than a second picture.

That deleted `backdropFor(pathname)` with them. It existed only to name the
exceptions, and a function returning the same value for every path is a lookup
pretending to be a decision -- so the three shells spread `PAGE_BACKDROP`
directly and there is nothing left for them to keep in step. `LandingShell` no
longer needs `useLocation` at all.

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
scrim was tried first and is exactly what the art is for -- a photograph behind a
sheet of translucent white is not a photograph. What replaced it is a halo: each
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
before the first line -- and the top edge itself, which this tried in between,
leaves the heading nothing to sit against. Sharing the leftover
space between two flexible spacers -- the first attempt -- measures from the
middle of a block whose height is the length of its own caption, so the three
headings sat at three heights and jumped as you swiped. `flex-basis` in percent
is read against the panel's height, unlike a percentage padding, which resolves
against width even when it is `padding-top`. The horizontal third is still a 1:2
pair of flexible spacers, where nothing has to line up between panels. Below the
breakpoint the block spans the panel instead -- a third of 390px is not a margin,
it is a column too narrow to set type in.

Caption and heading are both ranged left, on one edge. The caption was justified
for a while, hyphenated by `documentElement.lang` (which `LocaleProvider` keeps
current), and it cost more than it bought: at four or five words to a line the
word spaces stretch visibly, no two lines set the same colour, and over a
photograph every gap shows a different piece of picture. A ragged right edge is
the cheaper thing to look at. The heading was centred before that, which read as
a heading belonging to something else. A fourth photograph takes whichever
variant suits it; the upgrade, if two ever stop being enough, is sampling the
image rather than adding a third.

Each is `cover` and pinned to the **top** edge, not centred: the panel is taller
than the pictures' aspect at most sizes, so something is always cropped, and
these three are framed to be read downward from the top. Centring, the default
worth having for a photograph nobody composed for this box, takes the crop off
both ends instead.

Both files are 1920px wide WebP, around a quarter of a megabyte each, imported
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

It replaced `/legal`'s "Back to easydnd" button. That was a second way home drawn
on exactly one page, sitting in the corner `SignInActions` keeps for the way *in*
rather than the way out; a licence notice should not need chrome of its own to
get out of. `SignInActions` now draws nothing at all for a signed-in visitor
there, and `auth.backToApp` is gone from both catalogues.

The dragon mark went with the emptiness. The old `Center` existed because a lone
mark on a blank page should read as optically centred against the *window* --
hence a box of the viewport less twice the header offset, stopping short of the
bottom by what the header took at the top. With a footer below and structure in
the middle there is no longer a figure to centre, and a hero mark stacked above a
carousel is two heroes competing for one glance. `ui/DragonMark.tsx` stays: it is
the app's mark rather than the landing page's, and its test keeps alive the
inline-SVG conventions it set.

So the height calc changed shape and not merely terms. The carousel fills
`AppShell`'s main content box -- `100dvh` less the header offset, the footer
offset and twice the shell padding -- with a `320px` floor, because `height` on
a carousel is a hard height and a short landscape phone would otherwise squeeze
three panels into a couple of hundred pixels. That floor is the old
`min-height`-not-`height` argument in its new home. Every term is still
`AppShell`'s own custom property, so the header height and the footer height in
`shell/LandingShell.tsx` are each stated once. One trap worth naming: Mantine's
`rem()` passes a length through untouched only when it begins `calc(` or
`clamp(`, so the whole expression is wrapped in `calc(...)` rather than being a
bare `max(...)`, which would be split on its spaces and mangled.

The "this browser cannot use passkeys" warning went with the copy rather than
being kept, and `/login` no longer carries one either: the passkey card is
simply absent there, and what is left on the page -- a provider, a guest -- is
what that browser can actually do. An alert explaining an option nobody was
offered apologises for a hole the visitor cannot see.

### The footer says whose this is

The landing chrome now has a footer, and only the landing chrome does. It exists
for one of the three things it carries: the SRD 5.1 data this app is built on is
CC-BY-4.0, and that licence expects its attribution *in the product*. Until now
`data/srd_5.1/ATTRIBUTION.md` had been travelling in the release tarball to a
directory nginx does not serve, which `licensing.md` recorded against itself as
an open gap. `/legal` closes it, the footer is how anybody reaches `/legal`, and
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
material on a character sheet -- has no link to `/legal` from anywhere. That
used to be a layout that forbade it, `MobileShell` spending its only
`AppShell.Footer` slot on the tab bar. Since the tab bar became a dropdown in
the header, the slot is free and both signed-in shells could carry a footer.
So it is now a decision nobody has made rather than a thing that cannot be
done -- still recorded in [licensing.md](licensing.md#known-gaps) rather than
quietly carried.

Branching rather than redirecting is what keeps the address bar honest: an
unauthenticated visit to a deep link does not bounce anywhere, so a link shared
with someone who has not signed in yet still works -- the moment the state
flips, the same route renders its real content.

The one navigation in the whole flow is deliberate. There are three ways in and
choosing between them needs room to say what each costs, so they live on
`/login` rather than in a header corner. The page opens straight on the cards:
the lead sentence that used to count the ways in ("Two ways in. One of them
keeps your characters; the other does not.") was counting cards the visitor can
see, and each of them already says what it costs. Three, not four: signing in with a
passkey and signing up with one are one button -- see
[One button means both halves](#one-button-means-both-halves) -- and the display
name that sign-up used to ask for was the last text this client ever collected.

`shell/SignInActions.tsx` is now a single "Log in" link that carries the
current location in router state, and `features/auth/LoginScreen.tsx` returns
the visitor there on success -- which is what preserves the deep-link property
now that the way in is a page. A signed-in visitor who reaches `/login` is
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
decoding every real jsdom run uses.

A guest session is one POST rather than a ceremony, so `AuthProvider` shares the
busy/error/unmounted plumbing with it through `runAuth` and lets the flows
differ only in what they await. Everything that offers account management
has to check the flag: `features/auth/PasskeyNotice.tsx` renders a "nothing is
saved" notice for a guest and nothing at all for an account, and both shells
still *name* the control "End guest session" rather than borrowing a word that
implies you can come back. Those words are now the control's accessible name
and its tooltip rather than button text -- see
[the account is two icons](#adding-a-feature) -- which is the one thing that
had to survive the change intact, because a logout glyph is identical either
way and the difference is whether pressing it destroys somebody's only copy.

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
decoding, which is the path the tests exercise -- jsdom has neither.

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

There is exactly one viewport branch, in `shell/RootShell.tsx`: a persistent
navbar on desktop, and on mobile a single row of chrome whose sections live in
a dropdown beside the mark. Below it, screens are viewport-agnostic.

The phone chrome used to be a header *and* a thumb-reachable bottom tab bar, on
the argument that the top of a phone is the hardest place to reach one-handed
-- which is true, and is how this app gets used at a table. It lost to a bigger
one. Two rows of chrome cost 108px of an 844px screen to draw two things, one
of which is pressed rarely: you are usually already in the section you want.
Folding the sections into a dropdown buys the content back a whole row, gives
each section its full name instead of the tab bar's four characters, turns
imperative `useNavigate` into real links, and costs the same whether there are
two sections or six -- which a `Tabs.List grow` does not. Note what is *not*
the reason: three tabs would have fit. Adding one is what made the question
worth asking, not what answered it.

It also freed the `AppShell.Footer` slot that the tab bar owned. Nothing fills
it; see [licensing.md](licensing.md#known-gaps) for the thing that wants to.

Two smaller rules the three shells share, both of them fixes for something that
looked wrong on screen rather than preferences:

- **One header height**, `HEADER_HEIGHT` in `shell/chrome.ts`. The landing
  chrome drew 56 and the phone chrome 52, so signing in on a phone moved the
  whole page up four pixels -- a flinch at the moment somebody first sees the
  app. Three shells sharing a corner need to share its dimensions, or the
  seam between logged-out and logged-in is visible.
- **The desktop navbar narrows to a rail of glyphs.** This is a reversal: the
  navbar used to have nothing to touch it, on the argument that a wide screen
  has room for 240px and no reason to spend it. That was true while the content
  was whatever width the window gave it. It stopped being true when every page
  got capped at `CONTENT_MAX_WIDTH` -- on a 1280px laptop, 240 of navbar plus
  1024 of content plus the shell's padding does not fit, and the thing that
  gives way is the table you were reading.

  **It narrows rather than disappears, and that is what fixes the control.** The
  problem with a menu that vanishes is not the hiding, it is that the way back
  has nowhere to live. Put it in the header and it crowds the wordmark -- which
  is what the first attempt did, and what a `Burger` defaulting to open did
  before that: it drew a close cross, left of the mark and before the app's own
  name, so it read as a way to dismiss something. A rail always has room, so the
  control keeps one address in both states. It costs 64px of the 240 it gives
  back; what it buys is a navigation that never leaves the screen.

  **The control sits directly under the sections, behind a rule**, and is drawn
  as a `NavLink` like they are -- so it inherits their geometry instead of being
  aligned to them by hand, and it is dimmed so it does not read as a fourth
  place to go. It was at the foot of the navbar first, which is where a
  sidebar's chrome conventionally goes; but this navbar is as tall as the window
  and holds three items, so the control ended up hundreds of pixels below the
  last thing anybody had looked at, and was missed.

  Two numbers are pinned rather than left to the content, both because the
  difference showed up on screen. **A row is `ROW_HEIGHT` in both states**: it
  measures 41px with a word in it and 34px without, so collapsing used to
  shorten every row and shunt the list upward while you watched. And **a row
  carries an explicit radius**, because Mantine's `NavLink` ships with none --
  square corners are barely noticeable on a 219px-wide highlight and read as a
  cramped box on a 43px one, so the rail's active item is the same shape as the
  menu's, just narrower.

  The section glyphs earn their keep twice here -- on the rail they *are* the
  navigation, so each keeps its name in a tooltip and in the link's accessible
  name.

  **It is not remembered.** There is no `localStorage` in this client at all,
  and the sheet's "Hide untrained" toggle is already deliberately unpersisted --
  a second unpersisted toggle is consistent, where a persisted one would be the
  first stored preference here and would have to earn that. It would also be a
  setting no page lists, which is how you get "the menu is gone" from somebody
  who collapsed it once, a month ago.

  A note for whoever reaches for `AppShell`'s `collapsed` prop: it is read only
  when Mantine considers the layout to be in its desktop mode, and
  `breakpoint: 'never'` opts out of having modes -- so `collapsed` silently does
  nothing here. An earlier attempt shipped that way and passed every test in
  `RootShell.test.tsx`, because the suite runs with `css: false` and the width
  arrives from a generated `<style>` element. Resizing `width` is what this
  shell does instead, and it needs no breakpoint at all.

Where a layout genuinely has to differ, it differs inside a `@/ui` primitive
rather than at the call site:

| Primitive | Desktop | Mobile |
|---|---|---|
| `ModalSheet` | centred modal | bottom drawer |
| `DataList` | table | a card: name, marks, one dimmed line of facts, a `⋮` menu |
| `Columns` | side-by-side panels | accordion |
| `SectionDeck` | full-width blocks, then side-by-side panels | a `TabDeck` |
| `TabDeck` | tab strip, and the active panel | tab strip over a carousel of every panel |
| `TabRow` | tab strip | the same, scrolled sideways, ends faded |
| `BlockList` | a list of blocks, one open | the same |

### A dropdown inside a sheet stays inside it

Mantine portals a `Select`'s dropdown to `document.body`, and a `Drawer` closes
when a tap lands on its overlay. On a phone those two facts meet: the dropdown
is outside the sheet, so opening the invite picker closed the sheet under it and
left the list floating against the bottom of a page it no longer belonged to.
The two then fought, which is what the flashing was.

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
viewport too -- so a cap in `dvh` chased every one of those resizes.

### A write refreshes the screen; it does not reload it

`useResource` offers both, and the difference is not cosmetic -- its own doc
comment has said so all along, and every list screen was calling the wrong one.

`reload` blanks: it puts the resource back to `loading`, which on these screens
means `ui/Page` swaps the whole body for a spinner. That is honest for a first
load or a retry after a failure, where the screen genuinely has no answer.
`refresh` re-fetches behind what is already drawn.

Every `act()` here follows a write **the server has already confirmed**. The
screen knows what happened; it is catching up on what else changed. Calling
`reload` there tore the list down and rebuilt it under whoever was reading it --
and on a phone it was worse than untidy: a row action or a bottom sheet's close
unmounted the portal it was dispatched from mid-transition, and the page flashed.
Renaming a group, changing somebody's rank and creating an invitation all did it.

So: `reload` stays on `pageState`'s `onRetry`, which is the case it was written
for, and every post-write path calls `refresh`.

### A card is not a table row with the headers moved

`DataList`'s mobile rendering used to re-label each cell: a bold line for the
name, then `Class: --`, then `Level: 0`, then whatever the actions column
happened to render, on a text line of its own. Every list in the app looked like
that, and three things were wrong with it -- only the first of them cosmetic.

- **It spent a 390px line on one word, repeatedly.** Two facts about a character
  took two full rows and printed the column header twice to do it.
- **It was invalid markup.** Each cell went inside a `<Text>`, which renders a
  `<p>`, and nearly every name cell rendered a `<Group>` of the name and its
  badges, which is a `<div>`. The browser closes the paragraph early, which is
  part of why the vertical rhythm was wrong.
- **The actions had nowhere to go.** `FolderPanel` said so in a comment long
  before this was fixed: *"four buttons in a row is what `DataList`'s mobile
  card rendering already cannot lay out."*

What replaced it is the shape `features/characters/FolderPanel` already had, and
which was the one list in the app that read well on a phone: a bordered `Paper`,
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
- **`DataList` styles the name, and the caller supplies it as a string.** Every
  call site used to wrap its own name in `<Text size="sm">`, which is exactly
  why the heading came out the same size as the fields under it. `text` is also
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

`Columns` and `SectionDeck` are the same idea answering two different questions,
and both are kept rather than one winning. `Columns` collapses: a section is a
disclosure, which is right where a page has two or three panels and the answer
is usually in the first. `SectionDeck` leafs: nothing
collapses, and a swipe or a tab decides what is on screen, which is right where
the sections are peers and a reader moves between them rather than down them.
The character sheet is the second kind; see [the sheet is a deck, not an
accordion](#on-a-phone-the-sheet-is-a-deck-not-an-accordion).

A section handed to `SectionDeck` says where it sits on a wide screen --
`'full'` for its own bare row, `'panel'` for a bordered card in the grid -- and
that is the primitive's whole knowledge of the screens above it. It is the same
kind of thing as `Columns`' `cols`: a layout hint, not a fact about a character
sheet. Consecutive `'panel'` sections share one grid, so the order of the list
is the order of the page.

Neither takes a control of its own. A section is a title and its content, and
that is all -- there is nothing on a sheet panel to press. `ColumnsSection` used
to carry an **`aside`**, a control belonging to the panel as a whole drawn on
its title line, and the skills filter was the one thing that ever used it. It
went with the filter rather than being kept as a slot nothing fills: on the
phone accordion it needed a genuinely subtle arrangement -- `Accordion.Control`
*is* a button, so an aside nested inside it would be a button within a button,
invalid markup with the outer control swallowing the press -- and carrying that
subtlety for no caller is how it comes to be wrong the day somebody needs it.

`TabRow`'s **`actions`** went the same way and for the same reason, once its one
caller -- the build screen's Finish -- moved to the heading line where it
belonged. What that slot carried was a `flex: 0 0 auto` and a paragraph
explaining why the strip beside it was the only thing allowed to give way. Both
are gone with it.

`TabRow` and `BlockList` are the ones whose two renderings are **identical
markup**. The others genuinely swap components at the breakpoint; `TabRow` is a
`ScrollArea type="never"` that is simply inert at a width the tabs fit in, so
there is no second tree to keep working and a test at one width is a real test
of the other. `TabDeck` was on this list for a while and is not any more -- see
below, which is the argument for why.

### A scrolling strip rests on a tab, and hides the one it cuts

Two things that only matter once the tabs do not fit, which on the sheet is
always: seven labels are 657px and a phone viewport is 369.

**It rests on a tab's left edge.** Bringing the active tab into view used to
stop the moment its *right* edge cleared the viewport, which is the least it
could do and leaves whatever tab straddles the left edge cut in half. Landing on
the tab's own left edge cannot leave a fragment, because a boundary is where a
tab begins.

**The rule under the tabs spans the tabs.** `Tabs.List` is a block inside the
scroller, so it took the *viewport's* width -- 369px against 657px of tabs --
and its bottom rule stopped a third of the way along while the tabs themselves
overflowed it. From a scrolled position that draws as a stray dash beside the
first tab you can see, which is what it was reported as. `width: max-content`
makes the list as wide as what is in it.

**The end that is cut is hidden, not faded.** The far end of the strip is the
one place the first rule cannot win -- the browser clamps the scroll wherever
the arithmetic puts it, which on the sheet's last tab is 36px into
*Proficiencies*. A gradient across that fragment was tried at 24px and again at
32px and failed both times for the same reason: the far half of it sits at
80-90% opacity and reads as a word. So the mask is transparent for the
fragment's measured width and ramps up over the 16px after it, where the next
whole tab starts. An end resting exactly on a boundary hides nothing and keeps
the ramp, because an edge drawn hard says the strip ends there.

Both are measured from the tabs' own geometry, which is the one thing in this
component the suite cannot press: jsdom computes no layout, so every strip there
is 0px wide, never overflows, and never draws a mask. What the tests hold is
that the absence is identical at both viewports. The active tab is brought into view by setting `scrollLeft`, not
by `scrollIntoView`, which scrolls every scrollable ancestor -- it would drag
the document as well as the strip, and jsdom does not implement it. A stack of
bordered disclosures needs no branch either: it is right at 390px and at
1440px, and the only difference is padding the spacing scale already handles.

## A dialog is a form, so the keyboard's Go key works

Every dialog in this app that asks for a name now passes `onSubmit` to
`ui/ModalSheet`, which wraps its children in a real `<form>` and lets the confirm
button be a `type="submit"`.

This was a bug you could only find on a phone. A soft keyboard offers a Go key
on the strength of the browser seeing a form with a submit button in it -- so in
the eight dialogs that were a bare `TextInput` beside a `Button onClick`, you
typed a name, pressed the obvious key, and nothing happened at all. There was no
error and no hint; the app simply ignored you.

Three mechanisms had grown where there should have been one. The two folder
dialogs wrote out their own `<form>` (and so were the only two that worked),
`features/character/NameForm` listens for Enter on the input, and the other eight
did nothing. Putting it on the wrapper is what stops that recurring: a dialog
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
one meant typing into something you could not see. `resizes-content` shrinks the
layout viewport instead, which puts the sheet on top of the keyboard and makes
the `85dvh` cap mean what it says.

The sheet's body also scrolls (`overflowY: 'auto'`), for the case the cap still
bites: the New folder dialog carries a paragraph above its field, and on a short
viewport with the keyboard up that is taller than the space left.

## One button size, in one place -- and one CSS rule beside it

Every `Button` is `xs`, at every width, set once in `ui/theme.ts` as a
`defaultProps` override rather than passed at each call site. It used to be
passed at each call site, and the result was three sizes: the header's "Log in"
was `compact-sm` at 26px, the `/login` page it leads to answered with four
default-`sm` 36px ones, and the inline retry buttons sat between them at `xs`.
Pressing one and landing on the other read as two designs, which is what a size
decided fifteen times eventually looks like.

A call site may still pass `size` where it genuinely means something different.
There is exactly one: the phone header's section dropdown is `sm`, because it is
the whole of the app's navigation on a phone and `xs` is 30px.

**`ACTION_SIZE` is gone.** It was `'xs'`, passed at some twenty call sites, and
said exactly what the theme default already said -- so "the row-action size" and
"the app button size" were pinned together by coincidence rather than by
construction. `ACTION_ICON_SIZE` stays: a glyph is not a control.

### The one thing a theme value could not say

`ui/app.css` is the app's only stylesheet and holds a single rule: a phone's
input text is 16px.

That is a browser fact, not a size. **iOS Safari zooms the whole page when a
field smaller than 16px takes focus**, and every field here is Mantine's `sm`,
which is 14px -- so every rename box in the app lurched the page when you tapped
it. It is set as `font-size` on `.mantine-Input-input` rather than through a
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
one.** `scripts/check-layers.mjs` enforces both. Closing that hole meant teaching
the layer check to see side-effect imports at all: its pattern required a `from`,
so every `import '@mantine/core/styles.css'` had been invisible to the vendor
rule that exists to keep Mantine inside `src/ui`.

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
a thumb has to hit *precisely* -- `ScoreAssignment`'s drag targets, which worked
the number out for themselves before it was a token -- and not as the size of
every control.

## The palette is one line in theme/tokens.ts

The app's colour is data, in `theme/palettes.ts`, and which palette it wears is
one constant:

```ts
export const PALETTE_NAME: PaletteName = 'dragon'
```

**It is a development tool, not a setting.** There is no picker, no environment
variable and nothing to strip from a production build. Change the word, Vite
repaints, and you are looking at another skin. A user never chooses one, because
a user was never the audience: the point is to be able to try the app four ways
while designing it without editing forty files.

A `Palette` is a ten-step accent ramp, the one step that is *the* brand colour,
the mark's light field, and two `Scheme`s of five surfaces each -- background,
surface, text, dimmed, border. Four ship: `dragon` (the deep red the app has
always worn, and the default), `parchment`, `midnight` and `moss`.

**Every palette defines both schemes**, and that is not tidiness.
`AppTheme` runs `defaultColorScheme="auto"`, so the app never gets to choose
which scheme a visitor sees; one that defined only `light` would be unreadable
to half the people who opened it.

**The binding is five CSS variables, not a stylesheet.** `createTheme` takes
colour *ramps* and has no way to say "the page's background"; `cssVariablesResolver`
in `ui/theme.ts` is the lever, and it takes light and dark separately. Because
those are Mantine's *own* variable names, every `Card`, `Table`, `Paper`,
`Alert`, `Modal` and `Drawer` follows with no per-component override -- and this
repo still has no CSS file, which only `ui/AppTheme.tsx` would have been allowed
to import.

**Nothing in the suite can assert on any of it.** Vitest runs with `css: false`
and jsdom lays nothing out, so no test can read a computed colour. That leaves
the data as the only surface to hold, and `theme/palettes.test.ts` holds it:
ten valid steps, a brand colour drawn from its own ramp, both schemes complete,
and -- the one that earns its keep -- text-on-background contrast of at least
4.5:1 in both. That is the failure invisible to whoever makes it, because they
were looking at the scheme they designed. Dimmed text is held to 3:1 rather than
4.5:1 on purpose: Mantine's own default dimmed is 3.3:1 on white, and holding a
palette to AA body text there would fail the framework's own choice.

## The icons come from the palette

The brand red used to be written in five places, and they had drifted:
`tokens.ts` and `favicon.svg` said `#99051d` while `index.html` and the PWA
manifest said `#7a1f2b`, a colour that appeared nowhere else. There is now one
source, reached three different ways.

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

It owns `public/favicon.svg` (previously hand-authored) and the four PNGs.
`make web/icons` regenerates them; `make web/icons/check` fails on drift and is
part of `make verify`, beside `data/srd/check`.

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

The same rule now covers a second vendor: **only `src/lib/i18n/` may import
`i18next` or `react-i18next`**, and everything else imports `useT` from
`@/lib/i18n`. A translator that sixty files reach for directly is a translator
nothing can ever replace, and the re-export is also where the key type lives.
The guarded directory is `lib/i18n` rather than `lib`, because it is a
directory inside a layer and the rest of `lib/` has no business importing
i18next either.

`src/domain/` sits beside `theme/` at the bottom: pure rules, no framework, no
transport -- and now no prose. It used to hold three tables of English nouns,
which were the one thing in that directory that could not survive a second
language; they are message keys in `features/character/labels.ts` now, and what
stayed behind is what is genuinely a rule.

## Adding a feature

Types and calls in `web/src/lib/api/`, screens in
`web/src/features/<aggregate>/`, a route in `web/src/routes/index.tsx`. Shared
visuals belong in `web/src/ui/`, never inline in a feature. The API's error
envelope is decoded exactly once, into `ApiError`, by `lib/api/client.ts`.

A **new top-level section** is one more entry in `ui/sections.ts` -- a path, a
label and a glyph. Both shells map over `SECTIONS` and neither needs touching,
and so does every breadcrumb: the desktop navbar, the phone dropdown and the
first crumb of every trail all build themselves from it.

It lives in `ui/` rather than `shell/`, where it started life as `shell/nav.ts`,
and the move is forced: a trail begins at the section it is in, so *screens*
need the same label and glyph the navbar draws, and `features/` may not import
`@/shell`. `lib/` could not hold it either, being denied the icon package. `ui/`
is the one layer both the chrome and the screens can see.

`sectionFor` decides which section a path belongs to, and a `Section` carries
two different things for a reason. `to` is where it *links*; `owns` is what it
*claims*. Characters is why: its list is `/`, and `/` as a prefix matches the
entire app, so matching on `to` alone meant a character sheet lit nothing and
the phone's dropdown fell back to the word `Menu`. Splitting the two jobs lets
`/characters/*` belong to Characters while `/` still matches only itself -- the
one property that function is pinned on. The trailing slash in the prefix test
is load-bearing too: without it `/groupsfoo` answers Groups.

The dropdown keeps the `Menu` fallback, because it is one control and the only
thing naming the current place on a phone, and `/account` and `/login` belong to
no section. It just fires far less often than it used to.

Watch the names when a feature's API types meet the design system. `@/ui`
already exports Mantine's `Group` layout primitive and every screen uses it, so
`lib/api/groups.ts` exports `GroupDetail`, `GroupSummary` and `GroupMember` and
deliberately nothing called `Group` -- a type of that name would shadow the
component in exactly the files that need both.

There are deliberately **two brand marks**, and they are served two different
ways. The d20 in `web/public/favicon.svg` is the tab icon, the PWA icons
(`npm run icons` regenerates them from `scripts/gen-icons.mjs`) and the header
wordmark; `shell/Wordmark.tsx` reaches it with an `<img>` because the browser
has already fetched that file for the tab. The dragon in `ui/DragonMark.tsx` is
the app's hero mark -- it carried the signed-out landing page until the carousel
replaced it, and it is currently unplaced -- and it is an inline SVG component:
nothing pre-fetches it, so a `public/` asset would be a round trip before the
page has anything to show, and there is no `vite-plugin-svgr` here to turn a
`.svg` into a component without a new dependency. Neither is a mistake to be
tidied into the other -- one is drawn to survive 16px, the other to carry a
page.

`DragonMark` is also the client's first inline SVG, so it sets the convention:
colours as literals rather than `currentColor` (it carries its own field, which
is what makes it read under `defaultColorScheme="auto"`), and `role="img"` with
a real accessible name rather than `aria-hidden`, because on a page with no
text the mark is the only thing naming the app.

`ui/ProficiencyMark` is the second, and it keeps the half of that convention
worth keeping while inverting the other two -- which is the useful thing to
know about drawing an SVG here, because the reasons are what generalise, not
the choices. It is **announced**, like DragonMark. But it is drawn in
`currentColor`, because it sits inline in a row of text and has to dim when the
row dims; a literal would make it the one thing on the row ignoring both the
row and the colour scheme. And it is named with `aria-label` rather than a
`<title>` child, because a `<title>` is a text node: eighteen of these share
the skills panel, whose rows are read as text, and "Stealth DEX +7" must not
come back with a sentence about proficiency bonuses in the middle of it. The
rule behind all three: a mark carrying a page keeps its own colours and can
afford a `<title>`; a mark riding inside text takes the text's colour and stays
out of its content.

There is now a **third** source, and a rule for choosing between them. A brand
mark is drawn by hand and lives in `ui/` or `public/`, because there are two of
them and they are the app's own. A **UI affordance** -- an account, a way out,
a chevron, a tick -- comes from `@tabler/icons-react`, re-exported one glyph at
a time through `@/ui`. Hand-drawing those is how two "delete" controls end up
different shapes, and the re-export list doubles as the app's icon inventory:
adding one is a decision somebody makes in `ui/index.ts` rather than an import
nobody reviews. Four icons cost the production bundle about 2KB, because each
is its own ES module and the package sets `sideEffects: false` -- named imports
only, never the deep `dist/esm/...` paths.

Those icons are **decorative**, which inverts both marks above: the control
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
neither, and no longer gets an alert about it either: the subtitle says the
session is a guest's, and a paragraph under it listing what a guest session
lacks was the page scolding somebody for the way they came in, on the one screen
where there is nothing they can do about it.

**The way in is a row in the navigation, and it is still not a section.** The
desktop navbar draws it under the same rule that separates the collapse control
from the sections; the phone's dropdown draws it under a `Menu.Divider` at the
bottom of the same list. Nothing about it comes from `ui/sections.ts`: no trail
starts at `/account`, `sectionFor` answers null there, and the phone's trigger
still falls back to `Menu` -- the list of *places* stays three long.

It was a glyph in the header's top-right corner before that, on the reasoning
that the navigation lists the parts of the app while the account is who is
looking at them. That reading is still true and is exactly why the row sits
below the rule rather than among the sections -- but a corner glyph can only
name itself when hovered, and it was the third one in a corner that already had
two. A menu row has a word.

**What stayed in the corner is the way out**, built once in
`shell/AccountActions.tsx` and used by both signed-in shells, beside the
language: signing out is not somewhere to go, so it is not in a list of places.

It was a display name and a text button, and the name *was* the link, on the
reasoning that the header has to say whose session this is. What broke that is
the phone. A display name is arbitrary length in the narrowest row this app has,
sharing it with a mark, the word "easydnd" and a button reading "End guest
session" -- and the thing that overflowed first was the control that ends the
session.

So the name moved out of the header's *text* and into the control's accessible
name and tooltip: `Sign out`, or `End guest session` for a guest. The header
ends in the way out whether or not there is an account behind the session.

**Signing out lands on `/`**, rather than leaving the URL where it was. Staying
put reads as a bug at every address that is not `/`: the chrome swaps to the
logged-out one underneath you, and the deep link you were on -- a sheet, a group,
`/account` -- either bounces through the gate or sits there naming a thing you
can no longer open. `/` is the one address that means something on both sides of
the boundary, so it is where the session ends. The navigation happens after the
request rather than beside it, and `signOut` drops the local session even when
the request fails, so it is reached either way.

The cost is worth naming rather than glossing, because it has since grown: the
display name is now printed **nowhere in the client**. `/account` used to carry
it in its subtitle and no longer does -- what the page is for is the inventory
of ways in, and a line naming the account above it was a label on a page you
reached by pressing the account's own row. The server still knows the name and
`GET /v1/auth/me` still sends it; nothing draws it.

A tooltip cannot be the name. Mantine's wires `aria-describedby`, and only
while it is open; the label is not in the DOM at all when it is closed. So the
`aria-label` is the name and the tooltip is the sighted equivalent, both built
from one string -- duplicated deliberately rather than by accident.

`/` is the one page both sides of the sign-in boundary share: the three panels
signed out, the character list signed in. It carries nothing else -- system
status is a deploy question rather than something either audience came to `/` to
read.

There used to be a `/status` page answering it, showing the bundle's version
beside the API's. It was removed: the comparison it existed to show is now made
by the client itself on every response, and a page nobody opened is worse than
no page. `curl`ing `/version.json` against `/v1/version` is what remains, and it
is what the deploy pipeline was doing all along.

`/legal` is the one route outside `RootGate` entirely: it renders in
`LandingShell` for everybody, signed in or out, because needing an account to
read the licence of the material you are being shown would be backwards. It is a
document rather than a section, so it stays out of `shell/nav.ts`, and it is
reached from the footer the landing chrome carries. The landing header offers a
signed-in visitor the way back rather than a second invitation to sign in.

`/roll` is a third of the same kind, private rather than public: a page that is
a die and nothing else. Out of the section table for the same reason the two
above are -- it owns no paths, lights nothing and starts no trail -- and
reached only from the phone menu, below a divider, because the die is a
thumb's. See [The die is a page, not a
dialog](#the-die-is-a-page-not-a-dialog).

Routes added under `/` render inside `RootGate`, so they are only reached when
somebody is signed in. A route that must stay public -- `/login`, which would be
unreachable otherwise -- still renders, so guard the *data* on the server rather
than assuming the route is unreachable.

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
[dnd.md](dnd.md#localization). `ru.json` being incomplete is never a build
failure.

**Plurals are keys, not code.** Russian has four plural forms where English has
two, so `n === 1 ? 'x' : 'xs'` is wrong before the word order is. A plural is
`foo_one` / `foo_other` in the catalogue, called as `t('foo', { count })`, and
i18next picks between the forms with `Intl.PluralRules`. Russian adds `_few`
and `_many` in its own file and nothing in `src/` changes.

That is also why **counts are digits rather than words**. The build screen used
to read "Two more languages"; it reads "2 more languages" now. A Russian
numeral agrees in gender with the noun it counts -- два языка, две черты -- so a
shared table of spelled numbers is the same composition bug one level down.

**Nothing is glued together.** A label built as `` `${whose}Spell attack bonus` ``
is English word order written in TypeScript: a translator handed the fragments
cannot reorder them, because the code already did. Every such phrase is one
message with named arguments in it.

### What is not translated

- **`src/features/legal/attribution.ts` and the notices on `/legal`.** The SRD
  5.1 attribution is pinned to `cmd/srdgen`'s constant by a test, and a
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

Each of those carries an `i18n-exempt` comment in the source naming its
reason. Grep for it before extracting strings: a marked string is one to
leave exactly where it is.
- **Language names in the switcher.** "English" and "Русский" are what each
  language calls itself, which is what somebody looking for one is looking for.

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
`i18next-browser-languagedetector`, which was tried and removed. It left
`i18n.language` as the raw tag, so `ru-RU` produced a locale of "ru-RU" that
every consumer had to normalise again, and it read `sessionStorage` unguarded,
so a private-mode browser that refuses storage threw before the app rendered.

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
  to say about Russian. The script fails on an unused key and on a Russian key
  with no English counterpart, and *reports* Russian coverage without ever
  failing on it.

### The suite renders in English

Around six hundred assertions match visible copy --
`getByRole('button', { name: 'New group' })`. They went on passing unchanged
when the captions moved into `web/locales/`, because `src/test/render.tsx` is
the single seam: it wraps every render in a `LocaleProvider` pinned to English.
`renderAt(viewport, ui, 'ru')` is how a test asks for the other one.

The instance is built per render rather than shared, and that is not tidiness.
The suite runs with `isolate: false`, so a language set on a module-level
singleton by one file would be inherited by every file that ran after it, in
whatever order they happened to run -- which is the same hazard `vi.mock` is
banned for. A module of pure functions that needs words takes a `Translate` as
an argument instead; `features/character/settled.ts` and `options.ts` are the
worked examples, and `src/test/i18n.ts` is the translator their tests hand over.

## Offering the install, and clearing the notch

The app has been installable since the first commit and said so to nobody. On
HTTPS Chrome offers its own omnibox install icon regardless; on iOS nothing
appears at all, because iOS has no install API and never has. `ui/InstallAction`
is the offer, and `lib/install` is the one bit of state behind it.

**A button, not a banner**, and that is [web.dev's guidance][promote] rather
than taste: *"Don't show banners on initial page load or out of context"*, and
*"keep promotions outside of the flow of your user journeys"*. It also settles a
question this client would otherwise have had to answer -- a button has nothing
to dismiss, so nothing has to be remembered, and the rule about there being no
`localStorage` here stays where it is.

It renders `null` unless there is something to offer, so an installed app and a
browser that cannot install both get the chrome exactly as it was.

**It is a glyph in the header, left of the language.** It spent a while
floating in the bottom left corner, `position: fixed`, on the argument that an
offer does not belong in the row that says where you are and how to leave. What
that bought was a control sitting on top of the page's own content at every
width -- over the foot of a table, over the landing footer -- which is worse
than being one more glyph in the corner the rest of the chrome already shares.
So no shell mounts it any more: `shell/AccountActions` draws it for the two
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

`index.html` has always said `viewport-fit=cover`, which tells iOS to hand the
page the whole display -- including the strip under the status bar and the one
under the home indicator. The matching half, `env(safe-area-inset-*)`, was used
nowhere, so the header simply sat beneath the notch. In a browser tab Safari's
own chrome covers it and nothing looks wrong; installed, it is the first thing
anybody sees. Shipping an install button without this would have been an
invitation to a broken header.

`shell/chrome.ts` carries the tokens and the three shells apply them, the way
`HEADER_HEIGHT` already solved the same class of problem -- there are no CSS
files in this repo and there is not about to be one. `HEADER_BOX` grows the bar
upward so it paints behind the status bar, while `paddingTop: SAFE_TOP` on the
same element keeps the row of controls where it was. The `0px` fallbacks in
those `env()` calls are load-bearing: an `env()` a browser does not know is
invalid, and an invalid value inside `calc()` poisons the declaration, so the
header would lose its height rather than gain nothing.

**None of this can be exercised in `make web/dev`.** `beforeinstallprompt` needs
a registered service worker and a secure context, and the dev server has neither
-- see `devOptions` below. A production build reached over `localhost` is the
only way to see the button; the notch needs a real device, because Chrome's
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

`src/lib/version` watches for a deploy. Two signals feed it:

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
out -- the app is unusable until the server is. `/status` used to be the page
you could still reach to see the two versions disagreeing, and it was removed;
diagnose that case with `curl https://easydnd.org/version.json` against
`/v1/version`, which is what the deploy pipeline does anyway.

### Why registerType is 'prompt'

It was `'autoUpdate'`, and that was half a mechanism rather than a choice.
`'autoUpdate'` sets `skipWaiting` and `clientsClaim` in the generated worker,
but nothing here imports `virtual:pwa-register`, so `injectRegister` fell back
to `'script'` and the emitted `registerSW.js` was a bare `register()` call with
no update listener in it. A deploy then did this: the new worker installed,
skipped waiting, claimed tabs that were still running the previous release's
JavaScript, and `cleanupOutdatedCaches()` deleted the precache holding the
chunks those tabs would ask for next. Nothing reloaded them.

`'prompt'` leaves `skipWaiting` off, so a new worker waits instead of seizing
live tabs, and Workbox's template emits a `message` listener for
`{type: 'SKIP_WAITING'}`. `reload.ts` calls `registration.update()`, sends that
message to the waiting worker, and reloads on `controllerchange` -- with a
five-second fallback, because the button has to do something even if the worker
is wedged.

**`update()` resolving is not the install finishing**, and reading that wrong is
what made the dialog's button need pressing twice. The promise settles once the
script has been fetched and the install job is running: `registration.waiting` is
empty and `registration.installing` is the worker that matters. Reading only
`waiting` fell through to the plain reload -- four milliseconds after the new
worker began precaching, in the trace that found this -- and the old worker,
still in control, answered the navigation from its own precache with the same
page the dialog was complaining about. The second press worked because the first
press's install had finished by then. `reload.ts` now waits for an installing
worker to reach `installed` before sending `SKIP_WAITING`, and treats
`redundant`, `activating` and `activated` as "nothing to skip" -- a plain reload,
which is right for each of them. `reload.test.ts` pins the ordering.

That trace also showed the preview server answering the worker's precache fetch
of `/index.html` with a 301 to `./`: both `http.FileServer` and `http.ServeFile`
redirect that name. Every install spent a redirect on it and stored a response
marked `redirected`, which a browser may refuse to hand to a navigation. nginx
serves the file, so `internal/api/http/static.go` opens it and hands it to
`ServeContent` instead. It is written against `navigator.serviceWorker` rather than
`virtual:pwa-register` so that it stays ordinary TypeScript: the virtual module
resolves only through the plugin, which would make it a build-time dependency of
every test that touches the file.

The worker is disabled in `make web/dev` (`devOptions.enabled: false`) -- it
would otherwise shadow the dev server's module graph and serve stale chunks
after every edit. So **none of this runs in the dev server**, and a dev bundle
reports `WEB_VERSION === 'dev'`, which the watch ignores outright. Without that
guard every `make dev` session would open the dialog on its first request, since
the API it proxies to reports a real identifier.

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

The icons were the one violation: `max-age=2592000` with no revalidation on a
filename that never changes, so a palette change was invisible to a returning
browser for up to thirty days. Installed clients never saw it, because Workbox
refetches every revisioned precache entry with `cache: 'reload'` -- which is
precisely why it went unnoticed.

`manifest.webmanifest` had no rule at all, and stock nginx `mime.types` has no
`webmanifest` entry, so it was served as `application/octet-stream`.

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
   commit rather than the word "dev"; it must equal what the binary reports, or the update
   dialog above would fire against a version that disagrees with it for reasons
   that have nothing to do with a deploy. See
   [backend.md](backend.md#what-a-release-is-called-and-where-it-lives).
2. **A bad bundle goes live silently.** The API-side health gate cannot see the
   frontend, so `deploy.sh` checks the bundle exists *before* the symlink swap.
   Without that, a bundle that unpacked badly would go live as a blank site and
   would not roll back.

[mantine]: https://mantine.dev
