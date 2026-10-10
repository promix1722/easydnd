# Games and the tracker

Part of the [web client documentation](../web.md).

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
See [the seeded party](../backend.md#seeded-development-party) for the sample
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
[What a table hands over](../backend.md#what-a-table-hands-over). An entry's **consumables** are a dialog -- for its owner and a DM only; the
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
[backend.md](../backend.md#active-game-entries). Tags stay visible in the collapsed row; empty tag lists have no
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
A new NPC stub arrives from the server named NPC with 10/10 hit points;
character copies and existing NPCs keep their names and HP.

Dragging is one atomic `before_id` request: only the dragged entry moves. The
client and the API ship as one release, so neither call carries a fallback for
an older server.

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
