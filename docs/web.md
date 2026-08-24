# Frontend

The engineering doc for the [easydnd.org](https://easydnd.org) browser client:
layout, layer rules, and how it ships. For the Go API it talks to, see
[backend.md](backend.md); for the game model behind both, see [dnd.md](dnd.md).

Status: **real**. Sign-in (passkeys and Google), the party list, character
creation, the build loop, the account screen and the sheet are all built and
tested. The battle tracker is not.

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
  features/   screens
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
independent lifetimes, and here one screen owns one character. Revisit it if a
party view ever renders six.

The compendium is immutable for the life of the server process, so
`lib/api/catalog.ts` memoises each collection's *promise* for the session: two
components mounting in the same tick make one request, not two.

Net new dependencies for the whole character feature: zero.

## The build screen is a loop, not a wizard

`features/character/BuildScreen` reads `/prompts`, renders the first open one,
posts the answer and reads again. It is a loop rather than an N-step wizard
because prompts nest -- answering the "two skills" branch of a rogue's
Expertise is what brings the two-skill prompt into existence -- so the total
number of steps is not knowable until the last one is answered. Progress is
shown by stage instead.

Nothing in it decides what an answer *means*. The prompt says which event
carries it and the screen copies that verbatim, so the browser never learns
that a first level is a `class` event and a fourth is a `level` one. Option
keys come from the server for the same reason: a bundle of a shortbow and
twenty arrows has no slug of its own.

One `PromptCard` renders every kind of prompt rather than one component per
kind, because the server synthesises "which race?" into the compendium's own
grammar instead of a second vocabulary. What the kinds change is the wording.

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

Nothing on the page writes. Dropping a suffix of the log is the Back button on
the build screen, where the thing being undone is in front of you.

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

## Signed out and signed in, one build

There is one hostname, one bundle and one route table. `shell/RootGate.tsx`
branches on the auth state and picks the chrome: a loader while the session is
still unknown, `LandingShell` when anonymous, `RootShell` when signed in. At
`/`, `routes/HomeRoute.tsx` makes the same choice about the content.

Signed out, that content is the dragon mark and nothing else -- no headline, no
tagline, and since system status moved to `/status` alone, nothing under it
either. With the page otherwise empty, the mark sits in the middle of the
window -- the window itself, not the space left under the header, so it reads
as centred rather than as nudged down by the chrome. The `Center` in
`routes/LandingPage.tsx` starts below the header, so it is given a height of
the viewport less *twice* that offset and stops short of the bottom by what the
header takes at the top. Both terms are `AppShell`'s own custom properties, so
the header height and the shell padding are still stated in one place. The
chrome around it already does the work a pitch would: the header carries the
name and the "Log in" button. What the old
copy promised about the rules and level-up is either invisible until you are
inside or better said on `/login`, which is one press away and has room to say
what each way in costs.

The "this browser cannot use passkeys" warning went with the copy rather than
being kept. `features/auth/LoginScreen.tsx` renders the same alert on `/login`,
where the guest button that still works actually lives; a second copy on the
landing page would only be a sentence read twice on the way to the same place.

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
offers no button, and redundancy is a matter of connecting a provider -- see
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

There are deliberately **two brand marks**, and they are served two different
ways. The d20 in `web/public/favicon.svg` is the tab icon, the PWA icons
(`npm run icons` regenerates them from `scripts/gen-icons.mjs`) and the header
wordmark; `shell/Wordmark.tsx` reaches it with an `<img>` because the browser
has already fetched that file for the tab. The dragon in `ui/DragonMark.tsx` is
the hero mark on the signed-out landing page, and it is an inline SVG
component: nothing has pre-fetched it, so a `public/` asset would be a round
trip before the page has anything to show, and there is no `vite-plugin-svgr`
here to turn a `.svg` into a component without a new dependency. Neither is a
mistake to be tidied into the other -- one is drawn to survive 16px, the other
to carry a page.

`DragonMark` is also the client's first inline SVG, so it sets the convention:
colours as literals rather than `currentColor` (it carries its own field, which
is what makes it read under `defaultColorScheme="auto"`), and `role="img"` with
a real accessible name rather than `aria-hidden`, because on a page with no
text the mark is the only thing naming the app.

`/account` is where both inventories live -- passkeys and connected providers --
and where connecting and disconnecting happen. It is reached from the header,
not from `shell/nav.ts`: the navigation lists the parts of the app, and the
account is who is looking at them, so both shells put an "Account" link in the
top right beside the button that ends the session.

`/` is the one page both sides of the sign-in boundary share: the landing pitch
signed out, the party list signed in. It carries nothing else -- system status
is a deploy question rather than something either audience came to `/` to read.

`/status` answers that question, and it is the one route outside `RootGate`
entirely: it renders in `LandingShell` for everybody, signed in or out. Public
because needing to sign in to check whether a deploy landed would be backwards;
outside the signed-in chrome and absent from `shell/nav.ts` because it is a
diagnostic rather than a part of the app to navigate around. The landing header
offers a signed-in visitor the way back rather than a second invitation to sign
in.

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
