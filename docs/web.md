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

**A test runs at one viewport unless the tree branches on width.** Exactly six
components do: `Columns`, `DataList`, `ModalSheet`, `SectionDeck`, `SheetBody`
and `RootShell`.

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

## Layout

A single responsive SPA in `web/`, served by nginx from the same release
directory as the binary. React 19 + TypeScript + Vite, with
[Mantine][mantine] as the component library and a PWA manifest so it installs
to a phone home screen.

```
web/src/
  theme/      framework-free design tokens (breakpoints, the palette, the content cap)
  ui/         the design system -- the only place Mantine is imported, and the section table
  lib/        API client, data hooks, WebAuthn plumbing and the auth context; no UI
  shell/      the chrome: RootGate picks it, RootShell picks the viewport
  features/   screens -- one directory per aggregate (characters/, groups/, ...)
  routes/     the route table, one tree for both viewports
  domain/     pure display helpers; the Go model in dnd.md owns the rules
```

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
own table under it, and its own **New character** and **Import** beneath that.
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
rename, leave, delete. It is not where you add to it.

**A row acts on that row.** Rename and delete sit in the row whose entity they
edit, with an icon each -- and whether they are drawn is the caller's rank at
*that* row's table, not at whichever one happens to be first. A DM at one group
and a player at another must not get a Delete on the second because they have
one on the first. Row actions are spelled out rather than folded into a menu,
and each carries its row's name as its accessible name: a column of buttons all
called "Delete" is ambiguous to a screen reader and to a test alike.

**Leaving is not editing.** It comes before rename and delete rather than
between them, because sitting in the middle of that pair reads as though it
were one of them.

Every one of these controls is drawn at `ACTION_SIZE` with an `ACTION_ICON_SIZE`
glyph, both from `@/ui`. They are constants rather than literals because the
sizes had already drifted once: a row's buttons were `compact-xs`, a heading's
were the default `md`, and the icons were a mix of 14, 16 and the icon
package's own 24 -- so the same three actions were drawn three different sizes
depending on which screen you were looking at. Small on purpose: a table is the
content and its controls are not.

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

**Below `md`, the section is dropped from the page.** The phone's one row of
chrome already carries a control naming the section you are in and opening the
others, and it draws that section's glyph beside the name -- so a crumb
repeating the word, or a list screen whose heading *is* the word, spends a 390px
line restating what sits an inch above it. On a detail page the section crumb
goes; on a section root the heading goes with it. Deeper crumbs stay at both
widths, because a group on a shared character's sheet is a real parent rather
than a restatement of the chrome.

Two things to know about that. It is done with `visibleFrom` rather than a
branch -- the first use of a responsive visibility prop in this client -- so
`Page` still renders one tree at every width and stays off the list of four
components that swap markup. And it has a real cost: below `md` a section root
has no heading in the accessibility tree at all, and the thing naming it is a
button rather than an `h2`. The alternative was printing the same word twice on
the narrowest screen the app supports.

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
inert below 1024px. The list of components that genuinely swap markup at the
breakpoint stays at four; `Page.test.tsx` pins that by comparing the two
renderings byte for byte, the way `TabRow.test.tsx` does.

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

Out of scope, deliberately: `/account`, `/login`, `/legal`, `/status`, the 404
and the join flow. None is in a section, none has a trail, and `AccountScreen`
renders without a router at all.

## Sharing is reading, and it is one component

A group screen has two tabs -- Members and Characters -- because both are the
same table seen two ways and neither is a page of its own. `TabRow` already
existed for this. **Games are deliberately not a third tab**: see below.

The **Characters** tab is what the group's members have shared with each other.
Sharing grants a read and only a read, and the panel says so by what it does not
draw: there is no edit control anywhere on it, no build link, no event log. That
is not the client hiding things it could offer -- there is no route behind any of
them for anybody but the owner, so a button would come back 404. The only action
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

New character and Import carry their own folder through as `?folder=`: there is
a pair of them under every folder's table, so where you press is where the next
character lands.

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
answered on one is exactly what `/prompts` returned for it. Two surfaces draw
those questions: the build tab, as blocks that open, and
`features/character/OutstandingChoices`, which is the character sheet's
read-only statement of what is left -- and the level-up page's, when there is
one. Both read the same response and name it through the same
`features/character/promptNames`, so there is deliberately no second notion of
"outstanding" anywhere in this client, and no second vocabulary for it. What
differs is only that one is a list of ways in and the other is a list.

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
a key it has not seen sorts to the end. Answering a question therefore adds
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

The trail reads `Characters / Ada / Creation`, which is what the screen does;
the route stays `/build`, because a URL somebody has open is not worth breaking
over a word. The tabs are capitalised for the same reason a heading is -- a tab
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
whatever the list was showing is where the next character lands. Import does the
same. Absent, the server resolves the account's default.

### The Stub button is a development build's third button

Beside Import there is a **Stub**, and it is there only in a development build.
It posts to `/v1/characters/stub` and lands on a finished level-3 rogue -- the
character in `docs/reference_hexsheet/` -- so that working on the sheet, the log
page or the character list does not begin with a walk through five tabs.

It goes to **the sheet**, and that is the one place it differs from Import,
which goes to the build screen. An import answers no prompts, so there is always
something left to decide; a stub is finished, so the sheet is the thing worth
looking at. It carries `?folder=` like both its neighbours.

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
what was decided -- "Race chosen" -- and never by the category alone, and empty
copy is "Nothing left here" rather than "nothing left in race". It is a small
rule with a large payoff: `getByText('race')` means one thing on this page, and
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
six abilities to put numbers on. Dragging is the obvious gesture on a mouse and
does not exist on a phone, so a number can equally be picked up with a tap or
the keyboard and put down with a second one -- the same operation, reachable
without a pointing device. Dropping onto a taken ability swaps the two; putting
a number back where it came from returns it to the pool. Nothing can be
confirmed until all six are placed, because six numbers and five decisions is
not an answer. Both halves are drawn at least 44px square: a number waiting to
be placed used to be a badge, which is a few millimetres of target for the one
gesture the whole surface exists for.

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

### On a phone the sheet is a deck, not an accordion

One row of tabs under the character's name, one section on screen, and a swipe
between them. **Nothing opens and nothing closes**, which is the change: the
carousel decides what is visible, so a section has no shut state to be in.

What it replaces is a `Columns` accordion, and the two answer different
questions. An accordion is right for a page of two or three panels where the
answer is usually in the first and the rest are detail -- `/status` is exactly
that, and still uses it. A character sheet is not that shape. It is six things
a player leafs between at a table, none of them subordinate to the others, and
an accordion made reading one of them a gesture: open it, and possibly shut
three others first. It also put the headline numbers -- identity, the ability
cards, the vitals -- above the accordion where they were never reachable except
by scrolling past them. As slides they are tabs like any other.

The tab strip is `ui/TabRow`, unchanged, because six tabs do not fit across a
390px screen and a strip that scrolls sideways is the whole of what it is. It
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

The captions in `routes/LandingPage.tsx` are **sample copy** and are marked as
such where they live: the right length and the right shape, not the words this
page ships with. Two of them describe what the app does; `Run an adventure`
describes intent, because the battle tracker is not built. That is the one to
keep honest -- a landing page promising it would be the only thing on
easydnd.org that did.

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

None of the three panels is a link, and not because two of them lead nowhere --
`/groups` is real. It is that all three live behind the sign-in boundary, so a
panel that navigated would bounce a signed-out visitor straight back to this
page; the header's "Log in" stays the only control, and it carries them where
they were going. What the old copy promised about the rules and level-up is
still either invisible until you are inside or better said on `/login`, which is
one press away and has room to say what each way in costs.

Each panel is named by its own heading, through `aria-labelledby`, rather than
carrying a second copy of the words in an `aria-label`. While the panels were
empty the label was all there was, and a slide that announces itself as "slide 2
of 3" is unusable without sight; now that the words are on screen, two spellings
of one name is only how they come to disagree. Reachability *by name* is still
the accessibility contract the mark used to hold up -- moved, not dropped -- and
`LandingPage.test.tsx` pins the wiring and not merely the name, because an id
that stops resolving turns every panel back into "Carousel slide" without
failing a test that only looked one up.

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
being kept. `features/auth/LoginScreen.tsx` renders the same alert on `/login`,
where the guest button that still works actually lives; a second copy on the
landing page would only be a sentence read twice on the way to the same place.

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

The version is the short SHA as plain text. `/status` is where a version is a
*diagnostic*: the full SHA, beside the API's, to be compared against it. Here it
is provenance, in a form that fits a 390px footer, and linking it would promise
a page for an arbitrary commit that nothing serves.

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
`/login` rather than in a header corner. Three, not four: signing in with a
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
| `DataList` | table | labelled cards |
| `Columns` | side-by-side panels | accordion |
| `SectionDeck` | full-width blocks, then side-by-side panels | a tab strip over a carousel |
| `TabRow` | tab strip, actions right | the same, scrolled sideways |
| `BlockList` | a list of blocks, one open | the same |

`Columns` and `SectionDeck` are the same idea answering two different questions,
and both are kept rather than one winning. `Columns` collapses: a section is a
disclosure, which is right where a page has two or three panels and the answer
is usually in the first -- `/status` is that page. `SectionDeck` leafs: nothing
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

`TabRow` and `BlockList` are the ones whose two renderings are **identical
markup**. The others genuinely swap components at the breakpoint; `TabRow` is a
`ScrollArea type="never"` that is simply inert at a width the tabs fit in, so
there is no second tree to keep working and a test at one width is a real test
of the other. The active tab is brought into view by setting `scrollLeft`, not
by `scrollIntoView`, which scrolls every scrollable ancestor -- it would drag
the document as well as the strip, and jsdom does not implement it. A stack of
bordered disclosures needs no branch either: it is right at 390px and at
1440px, and the only difference is padding the spacing scale already handles.

## One button size, in one place

Every `Button` is `xs`, and that is set once in `ui/theme.ts` as a `defaultProps`
override rather than passed at each call site. It used to be passed at each call
site, and the result was three sizes: the header's "Log in" was `compact-sm` at
26px, the `/login` page it leads to answered with four default-`sm` 36px ones,
and the inline retry buttons sat between them at `xs`. Pressing one and landing
on the other read as two designs, which is what a size decided fifteen times
eventually looks like.

A call site may still pass `size` where it genuinely means something different --
`defaultProps` sets a floor, not a ceiling. But the drift started with a second
size in three files, so a new one wants a reason beyond the button looking better
on the screen being worked on.

There is exactly one such call site: the phone header's section dropdown is
`sm`. The reason is a tap target rather than taste -- it is the whole of the
app's navigation on a phone, and `xs` is 30px, under every guideline there is.
`ActionIcon` keeps Mantine's own default and gets no `defaultProps` entry of
its own, because the three call sites that already use one rely on it and a new
theme default would silently resize them.

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
neither, only the alert saying there is no account behind the session. It is
reached from the header,
not from `shell/nav.ts`: the navigation lists the parts of the app, and the
account is who is looking at them, so both shells put the way in at the top
right beside the button that ends the session.

**The account is two icons**, built once in `shell/AccountActions.tsx` and used
by both signed-in shells -- a profile mark linking to `/account`, and the one
that ends the session.

It was a display name and a text button, and the name *was* the link, on the
reasoning that the header has to say whose session this is and that a button
labelled "Account" beside the account's own name said it twice. The first half
of that still holds; what broke it is the phone. A display name is arbitrary
length in the narrowest row this app has, sharing it with a mark, the word
"easydnd" and a button reading "End guest session" -- and the thing that
overflowed first was the control that ends the session.

So the name moved out of the header's *text* and into the controls' accessible
names and tooltips: `Account: Alice` and `Sign out`, or `End guest session` for
a guest. The header still says whose session this is; it says it on demand
rather than spending a phone's chrome on it unprompted. The cost is real and
worth naming rather than glossing: a sighted visitor now hovers the mark, or
opens the page it leads to, which names the account at its top. The empty-name
fallback survives as plain `Account`, since a control with no accessible name
is a control nobody can find, and a null user renders neither -- the pair is
right-pushed, so the header ends in the way out whether or not there is an
account to link to.

A tooltip cannot be the name. Mantine's wires `aria-describedby`, and only
while it is open; the label is not in the DOM at all when it is closed. So the
`aria-label` is the name and the tooltip is the sighted equivalent, both built
from one string -- duplicated deliberately rather than by accident.

`/` is the one page both sides of the sign-in boundary share: the three panels
signed out, the character list signed in. It carries nothing else -- system
status is a deploy question rather than something either audience came to `/` to
read.

`/status` answers that question, and it is one of two routes outside `RootGate`
entirely: it renders in `LandingShell` for everybody, signed in or out. Public
because needing to sign in to check whether a deploy landed would be backwards;
outside the signed-in chrome and absent from `shell/nav.ts` because it is a
diagnostic rather than a part of the app to navigate around. The landing header
offers a signed-in visitor the way back rather than a second invitation to sign
in.

`/legal` is the other, on adjacent grounds: needing an account to read the
licence of the material you are being shown would be backwards in the same way.
It is a document rather than a section, so it stays out of `shell/nav.ts` too,
and it is reached from the footer the landing chrome carries.

Routes added under `/` render inside `RootGate`, so they are only reached when
somebody is signed in. A route that must stay public -- `/login`, which would be
unreachable otherwise -- still renders, so guard the *data* on the server rather
than assuming the route is unreachable.

The Go side of the same feature is in
[backend.md](backend.md#adding-a-feature).

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
   build without it.
2. **A bad bundle goes live silently.** The API-side health gate cannot see the
   frontend, so `deploy.sh` checks the bundle exists *before* the symlink swap.
   Without that, a bundle that unpacked badly would go live as a blank site and
   would not roll back.

[mantine]: https://mantine.dev
