# Signing in, sessions and the admin section

Part of the [web client documentation](../web.md).

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
[backend.md](../backend.md#authentication).

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
[No recovery](../backend.md#no-recovery).

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

The Go side is in [backend.md](../backend.md#authentication).

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
  [backend.md](../backend.md#a-superadmin-reads-everything-and-writes-one-thing).
- **A player row has one action where a private pack is installed: Private
  packs**, a sheet of one checkbox per installed pack, saved as the whole list.
  The action is not drawn on a server with none, rather than opening onto
  nothing to tick. With one, the table gains a **Private packs column** naming
  what each player has, read from the listing itself (`packs` on a row) and
  re-read after a save. It is the only thing on the screen that writes.
- `usePaged` is the offset/limit paging `SpellsScreen` has, written once for
  both tables: a new filter set refetches from the top while the old rows stay
  dimmed, and a page that answers after the filters moved is dropped.
