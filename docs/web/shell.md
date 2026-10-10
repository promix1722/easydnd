# The shell: routes, viewports and the page frame

Part of the [web client documentation](../web.md).

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
done -- recorded in [licensing.md](../licensing.md#known-gaps) rather than
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
[One button means both halves](auth.md#one-button-means-both-halves) -- and sign-up
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
[licensing.md](../licensing.md#known-gaps) for the thing that wants it.

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
