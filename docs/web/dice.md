# The 3D die

Part of the [web client documentation](../web.md).

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
