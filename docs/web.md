# The web client

The engineering doc for the [easydnd.org](https://easydnd.org) browser client:
React and TypeScript, built with Vite, on a design system kept behind one
import (`@/ui`). For the Go API it talks to, see [backend.md](backend.md); for
the game model behind both, see [dnd.md](dnd.md).

This page is the way in: how to run the client, how the source is laid out,
the layer rule, and how to add a feature. Everything else is in `docs/web/`,
one file per part of the client. Those files explain *why* each part is the
way it is; read the one for the part you are about to change.

## Where everything else is

| Doc | Covers |
| --- | --- |
| [web/testing.md](web/testing.md) | How the suite runs, and the rules for writing a test |
| [web/shell.md](web/shell.md) | Routing, the two viewports, the page frame, dialogs, buttons, appearance, icons |
| [web/auth.md](web/auth.md) | Passkey and Google sign-in, a session ending, the admin section |
| [web/characters.md](web/characters.md) | Folders, sharing a character, and the AI Wizard workspace |
| [web/builder.md](web/builder.md) | The build loop, level-up, and how each kind of choice behaves |
| [web/sheet.md](web/sheet.md) | What the sheet shows and in what order, and the log page |
| [web/games.md](web/games.md) | The games section and the tracker |
| [web/dice.md](web/dice.md) | The die, and how its weight is kept out of the main bundle |
| [web/compendium.md](web/compendium.md) | The spell browser and homebrew packs |
| [web/localization.md](web/localization.md) | Captions, catalogues, and what is never translated |
| [web/shipping.md](web/shipping.md) | A dev server per worktree, caching, code splitting, the install prompt, deploy, analytics |

## Quick start

```sh
make web/deps                       # once, per worktree -- node_modules is not shared
make web/dev                        # http://127.0.0.1:5173, proxies /v1 to :8080
make web/check                      # typecheck, lint, layer-check, tests -- mirrors CI
make web/icons                      # after changing PALETTE_NAME or the mark
```

`make web/dev` proxies `/v1` to the API, so run `make run/db` alongside it
-- or `make dev` at the repo root, which starts both and a Postgres. `make
verify` at the repo root runs the frontend checks and the Go ones together, and
runs them **at the same time**: `web/test` is by far the longest thing in it, so
it is started first and the Go side happens inside its shadow. See
[backend.md](backend.md#tests).

## Layout

A single responsive SPA in `web/`, served by nginx from the same release
directory as the binary. React 19 + TypeScript + Vite, with
[Mantine][mantine] as the component library and a PWA manifest so it installs
to a phone home screen. It also ships a service worker, which matters more than
it sounds -- see
[Two caches decide what a returning visitor sees](web/shipping.md#two-caches-decide-what-a-returning-visitor-sees).

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
[Localization](web/localization.md#localization).

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
dialog](web/dice.md#the-die-is-a-page-not-a-dialog).

Routes added under `/` render inside `RootGate`, which only picks the chrome:
a screen that needs a session is wrapped in `Private`, and a route that must
stay public -- `/login` -- is not. Either way, guard the *data* on the server
rather than assuming the route is unreachable.

The Go side of the same feature is in
[backend.md](backend.md#adding-a-feature).
