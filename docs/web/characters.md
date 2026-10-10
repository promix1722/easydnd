# Characters: folders, sharing and the AI Wizard

Part of the [web client documentation](../web.md).

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
[docs/backend.md](../backend.md#folders).

The drag itself is hand-rolled over the native events, the way
`features/character/ScoreAssignment` is, and for the same two reasons: there is
no drag library below `@/ui`, and a native drag fires on neither a touchscreen
nor happy-dom. So it is never the only way to do something -- **Move up** and
**Move down** in each folder's menu are the real path, and the one the tests
press. Four folder actions is also the case `@/ui` blesses a `Menu` for, rather
than the spelled-out buttons a table row gets.

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

## Import workspace

The **AI Wizard** section (`/ai-wizard`) is a single assistant chat; the old
`/characters/import` addresses redirect to it (`routes/LegacyImportRedirect.tsx`).
`AgentImportScreen` is a chat of message bubbles: the assistant's on the left,
the owner's on the right, with a message's attachments inside it. Each
`progress` event is a bubble of its own, built by `progressEntry`
(`agentProgress.ts`): the field as a bold caption, its value beneath. It is
never folded.
Source labels and internal assumptions are omitted.

The page has no control for an [unattended session](../agent.md#unattended-sessions);
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
there is no Stop control. One [poll](../polling.md) a second brings
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
and Russian translations. See [agent.md](../agent.md) for the tool/question
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
carried](sheet.md#equipment-is-what-is-worn-items-is-what-is-carried)). Catalogue entries flagged `manual` are still offered in the
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
