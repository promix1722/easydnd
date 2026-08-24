# Frontend

The engineering doc for the [easydnd.org](https://easydnd.org) browser client:
layout, layer rules, and how it ships. For the Go API it talks to, see
[backend.md](backend.md); for the game model behind both, see [dnd.md](dnd.md).

Status: **real**. Sign-in (passkeys and Google), the party list with its
folders, character creation, the build loop, the account screen and the sheet
are all built and tested. **Level-up is not**, and this client does not offer
it -- see [Level-up is not offered](#level-up-is-not-offered). Neither is the
battle tracker.

## Quick start

```sh
make web/deps                       # once, per worktree -- node_modules is not shared
make web/dev                        # http://127.0.0.1:5173, proxies /v1 to :8080
make web/check                      # typecheck, lint, layer-check, tests -- mirrors CI
```

`make web/dev` proxies `/v1` to the API, so run `make run/server` alongside it
-- or `make dev` at the repo root, which starts both and a Postgres. `make
verify` at the repo root runs the frontend checks and the Go ones together.

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
  theme/      framework-free design tokens (breakpoints, palette)
  ui/         the design system -- the only place Mantine is imported
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

The party list is now the one screen that holds two resources -- its folders
and its characters -- which is the case that sentence used to leave open. It
still does not need a library. The two reads are independent, the character
read is keyed by the selected folder so changing the filter aborts the request
in flight rather than leaving the old folder's rows under the new heading, and
every mutation on that screen is followed by an explicit reload of the lists
that changed. There is no cache to go stale in between. Revisit it when
something needs a *third* component's copy of the same data to update by
itself.

The compendium is immutable for the life of the server process, so
`lib/api/catalog.ts` memoises each collection's *promise* for the session: two
components mounting in the same tick make one request, not two.

Net new dependencies for the whole character feature: zero.

The landing carousel is the exception that finally broke that run, and it is
recorded rather than slipped in: `@mantine/carousel`, `embla-carousel` and
`embla-carousel-react`, three packages for one page. Hand-rolling was the
default here and is wrong for this one thing -- a drag-and-snap carousel is
momentum physics, pointer capture, RTL and keyboard semantics, and "it is just a
flexbox with `scroll-snap`" stops being true the moment a thumb is involved. The
price is small and measured rather than assumed: +27 kB of JavaScript and +3 kB
of CSS minified, about +10 kB gzipped. The cost that outlives the decision is
the peer range -- `@mantine/carousel` pins `@mantine/core` to an *exact* version
rather than a range, so the four Mantine packages now move as a unit. A
lockfile diff that bumps `@mantine/core` is a UI-wide upgrade wearing a
carousel's clothes, and should be reviewed as one.

It costs something in the test suite too. embla constructs a `ResizeObserver`
and an `IntersectionObserver` unconditionally, and jsdom implements neither, so
`test/setup.ts` installs inert stubs. They deliberately never fire: jsdom has no
layout, so anything they reported would be fiction, and a test that leaned on one
would be testing the stub. The carousel is therefore asserted on its structure --
three panels named by their own headings, in a named region, and a height
expression that still mentions both shell offsets -- and never on which panel is
scrolled into view.

## Folders are filing, not sharing

A folder is a named place one account files its characters, and nothing else:
one owner, nothing shared with anybody, which is why the screen has no owner
column and no permissions on it. It is deliberately **not** called a group: the
Groups section next to it is the shared one, with people and ranks in it, and
the two words name genuinely different things. A folder lives inside the
Characters section and never appears in Groups.

`CharacterListScreen` carries the whole feature: a `Select` filters the list to
one folder, a **Manage folders** dialog creates, renames and deletes them, and
each row has a menu with Move, Copy and Delete. The filter is a `Select` rather
than tabs because it renders the same at both viewports and does not overflow
once an account keeps more than a few folders.

Two things on that screen are not cosmetic:

- **The default folder's row has no delete control.** It is the folder an
  account is guaranteed to have, and the API refuses to delete it. Rename is
  offered, because what an account cannot lose is the folder, not its name.
- **The delete-folder confirmation states the character count.** Deleting a
  folder deletes the characters in it, and characters live in memory, so there
  is no undo and no backup. A dialog that only named the folder would be
  describing a smaller action than the one about to happen.

New character and Import carry the selected folder through as `?folder=`, so
whatever the list was filtered to is where the next character lands.

## The build screen is a loop, not a wizard

`features/character/BuildScreen` reads `/prompts`, `/events` and `/sheet`, and
draws five tabs. It is still a loop rather than an N-step wizard, and it has to
be: prompts nest -- answering the "two skills" branch of a rogue's Expertise is
what brings the two-skill prompt into existence -- so the total number of steps
is not knowable until the last one is answered. The tabs are not steps. They
are the fixed set of *categories* a question can belong to, which is the
server's own `Prompt.Group` and not a taxonomy this client invented, in the
order `domain/stages.ts` states: identity, class, race, background, abilities.
Class first after the name, because it is the choice the most other choices
hang off.

Three requests, because they answer three different questions: `/prompts` says
what is still open, `/events` says what was decided and in which entry, and
`/sheet` says what all of it adds up to. The sheet cannot be folded from the
log in the browser -- an ability score improvement's increments are derived at
projection time -- so it is fetched rather than computed.

**Nothing can be answered before it is asked.** Every tab is freely clickable,
because a tab is a place to look as well as a place to answer, but what can be
answered on one is exactly what `/prompts` returned for it.
`features/character/OutstandingChoices` is that list, and it is one component
with three callers -- each build tab filtered to its own category, the
character sheet showing all of them, and the level-up page when there is one.
There is deliberately no second notion of "outstanding" anywhere in this
client, so there is no way for the sheet and the build screen to disagree
about it.

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

One `PromptCard` renders every kind of prompt rather than one component per
kind, because the server synthesises "which race?" into the compendium's own
grammar instead of a second vocabulary. What the kinds change is the wording.
Two questions are genuinely not that shape and get a form each: a name
(`NameForm`) and the six ability scores (`AbilityScoresForm`). `StagePanel`
chooses between the three by the kind of the prompt, and that is the only place
in the client mapping a prompt kind to a control -- which is what let
`PromptCard` stay exactly as it was while two new kinds arrived.

### Creating is answering the first question

`/characters/new` renders `BuildScreen` with no `:id`. The identity tab holds
the name, answering it creates the character with that name alone, and the URL
is replaced with the build one. There is no separate create screen because
there was never a second thing being done -- and creation used to carry the
score method and all six numbers, which meant eight selections in one log entry
and nothing a player could point at and change. The scores are an ordinary open
choice now, answered on the abilities tab and written as their own entry.

`/characters/new?folder=` carries the folder the party list was filtered to, so
whatever the list was showing is where the next character lands. Import does the
same. Absent, the server resolves the account's default.

### Changing anything is one mechanism

Every row under "already chosen" is exactly one log entry, and `[Change]`
replaces that entry: pick the new value, `PUT …?dryRun=true`, read what would
be dropped, commit the same `PUT` on confirmation. There is no
append-a-correction path and no Back button. The dialog names every dropped
entry and says the questions will be waiting outstanding in their own
categories, because they will be -- and it confirms even when nothing is
dropped, with a green affirmative rather than a red one, so that "this costs
nothing" is a thing the screen says rather than a thing you infer from its
silence.

An answer to a *nested* prompt -- a rogue's Expertise, a half-elf's ability
bonuses -- cannot be re-posed from here, because the options that made it up
arrived with a prompt the server stopped emitting the moment it was answered.
Those rows drop their entry instead, which reaches the same place from the
other side: the question comes back outstanding, under its own tab.

### A category's word appears exactly once

In its tab. Panel headings use the question's own wording, the two sections are
titled "already chosen" and "still to choose", and empty copy is "Nothing left
here" rather than "nothing left in race". It is a small rule with a large
payoff: `getByText('race')` means one thing on this page, and a test that
breaks does so for the reason it says.

### Level-up is not offered

The server poses "gain a level in which class?" and everything that follows
from it under the `advance` group, and this client filters that group out of
everything it draws: no outstanding row, no answering surface, no control.
Taking a level does not work -- the event the client would post is recorded as
a no-op -- and a question that appears answerable and silently changes nothing
is worse than a question that is not asked. It is the same judgement that took
the "Level up" button off the sheet.

Levels a character already *has* stay visible as settled rows on the class tab,
and stay read-only. They are facts about the character -- an imported one may
well have several -- rather than controls, and editing one would drive the same
machinery that cannot take one. `domain/stages.ts` is the single line that
reverses all of this on the day it works.

## The sheet decides what order the abilities come in

`features/character/CharacterSheetScreen` prints ability scores and saving
throws as a sheet does -- STR, DEX, CON, INT, WIS, CHA -- and it has to impose
that itself, because the API cannot say it. Both arrive as objects keyed by
slug and a Go map serialises its keys sorted, so a screen that walks the
response as it came prints CHA first. The order lives in `domain/format.ts` as
`ABILITY_ORDER`, and anything drawing more than one ability in sequence walks
it through `abilitiesInOrder` rather than walking the response -- the ability
scores form's six inputs included. Walking a fixed list against a projection that
may not match it decides two things: a slug the response omits draws nothing,
since a blank card claiming a score that is not there is worse than a row of
five; a slug the six do not cover is kept and drawn last rather than filtered
out, since an unrecognised ability means the server and this client disagree
about the game, which is a thing to see rather than a thing to hide. The skills
beside them stay alphabetical on purpose: there are eighteen in no traditional
order, so the alphabet is the only sequence a reader can search.

The stat row above leads with hit points, then armor class, initiative and
proficiency. Hit points are the one number that moves between one glance and
the next and the one reached for mid-turn, where armor class is settled at the
start of a fight; at two columns on a phone, first position is the only one
visible without the eye travelling.

## The log has its own page, and it never asks for the sheet

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
controls are `44px` rather than the default `26px`: they are the only way
through for a visitor with neither a touchscreen nor the arrow keys, they sit
*over* a panel rather than beside it, and 26px is under every published minimum
for a pointer target.

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
material on a character sheet -- has no link to `/legal` from anywhere.
`MobileShell` could not carry one without redesigning its tab bar, which owns
the only `AppShell.Footer` slot. That gap is recorded in
[licensing.md](licensing.md#known-gaps) rather than quietly carried.

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
say "End guest session" rather than borrowing a word that implies you can come
back.

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
navbar on desktop, a thumb-reachable bottom tab bar on mobile. Below it,
screens are viewport-agnostic. Where a layout genuinely has to differ, it
differs inside a `@/ui` primitive rather than at the call site:

| Primitive | Desktop | Mobile |
|---|---|---|
| `ModalSheet` | centred modal | bottom drawer |
| `DataList` | table | labelled cards |
| `Columns` | side-by-side panels | accordion |
| `TabRow` | tab strip, actions right | the same, scrolled sideways |

`TabRow` is the first of those whose two renderings are **identical markup**.
The others genuinely swap components at the breakpoint; this one is a
`ScrollArea type="never"` that is simply inert at a width the tabs fit in, so
there is no second tree to keep working and a test at one width is a real test
of the other. The active tab is brought into view by setting `scrollLeft`, not
by `scrollIntoView`, which scrolls every scrollable ancestor -- it would drag
the document as well as the strip, and jsdom does not implement it.

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

## Dependency rule

```
theme -> lib -> ui -> shell -> features -> routes
```

Imports point left, and **only `src/ui/` may import `@mantine/*`** -- everything
else imports from `@/ui`, which re-exports what it needs. `npm run lint:layers`
enforces both, the same way `make lint/layers` does for the Go packages: a
convention nobody can run is a convention that rots.

## Adding a feature

Types and calls in `web/src/lib/api/`, screens in
`web/src/features/<aggregate>/`, a route in `web/src/routes/index.tsx`. Shared
visuals belong in `web/src/ui/`, never inline in a feature. The API's error
envelope is decoded exactly once, into `ApiError`, by `lib/api/client.ts`.

A **new top-level section** is one more entry in `shell/nav.ts`; both shells map
over `NAV_ITEMS` and neither needs touching. Which entry is highlighted comes
from `activeNavPath` in the same file -- shared, because the two shells had
drifted and only the mobile one kept a section lit on its nested routes.

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

**The signed-in name is that link.** The header has to say whose session this
is, and a button labelled "Account" next to the account's own name said the
same thing twice -- so the name itself navigates to `/account`, drawn dimmed
and small rather than as a button, because it is still a label first. A session
whose name is empty falls back to the word "Account", since a link with no text
is a link nobody can press, and a null user renders neither: the pair sits in
its own right-pushed group so the header still ends in the sign-out control
when there is no name to draw.

`/` is the one page both sides of the sign-in boundary share: the three panels
signed out, the party list signed in. It carries nothing else -- system
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
