# The character sheet and its log

Part of the [web client documentation](../web.md).

## The sheet decides what order things come in

Up to seven tabs, at every width: **Overview**, **Actions**, **Spells**,
**Resources**, **Equipment**, **Items**, **Custom**. Spells and Resources are
there when the character has any; Custom is there for the owner always and
for a reader when there is something in it.
`features/character/SheetBody` builds them and `ui/TabDeck` draws them -- a tab
row over the showing panel on a wide screen, the same row over a swiped deck on
a phone. They are named for what a player is doing rather than for a table of
the rulebook: looking the character up, taking a turn, casting, gearing up,
going through the pack.

- **Overview** is who the character is and the abilities everything else is
  derived from, the body's state, then skills, proficiencies, traits and
  personality as headed panels -- two abreast on a wide screen, stacked on a
  phone. Personality prints the trait, ideal, bond and flaw as the player wrote
  them, and says so when one has not been written.
- **Actions** lists what the server derived (`sheet.actions`) and nothing
  else. The list is
  `features/character/SheetActions.tsx`: one group per source -- **Equipment**,
  **Class and race**, **Basic** -- each a heading with a count that folds on
  its own, over a `BlockList` whose rows open onto their description. Only the
  groups a sheet has are drawn. A row is marked with its part of the turn only
  when that is not a plain action -- Bonus action, Reaction, No action -- since
  "Action" on most rows of a list called Actions says nothing.
  **Every group starts folded**: the tab opens as headings with their counts,
  and the player unfolds the one they came for. There is no
  search and no filter: either is more control than a dozen rows need, and the
  one question a player would ask of them, "hide the basic ones", is what a
  folded group already answers. What is on
  the list is the rule pack's decision
  (see [packs.md](../packs.md#action-tags)); the client knows no class. The prose
  arrives in the sheet response as `catalog.actions`, keyed by each action's
  `origin`, so opening a row is not a request. A row with no prose -- a mundane
  weapon -- has no body and is drawn as a fact, not a control. An action whose
  source the client does not know is filed under Class and race. A
  pool is shown by capacity, as everywhere on a sheet; spending is the game
  tracker's.
- **Spells** is drawn only for a character with a spell source.
- **Resources** is what a class hands out to spend, drawn only for a
  character with any: the pools ("Consumable slots"), and among them every
  scaling value that is a plain number -- Extra Attacks: 1, Maneuvers: 3 -- as
  that many marks, which a game spends like a pool (docs/backend.md). It grows
  with level rather than coming back on a rest, but a table still counts it off
  within a turn, and a number printed as "Extra Attacks: 1" offered nothing to
  count with. **There is no box of scaling values.** A value that is read
  rather than counted -- a Sneak Attack die -- is the size of a feature, so it
  is on Overview with the features: "Sneak Attack: 1d6" in place of "Sneak
  Attack", and a value no feature is named for follows them. A scaling value of
  zero is one the class has not reached yet and is not drawn.
- **Equipment** and **Items** are described
  [below](#equipment-is-what-is-worn-items-is-what-is-carried).
- **Custom** is described
  [below](characters.md#custom-is-what-the-player-writes-unasked).

The same handful of tabs at both widths is one layout to learn, and the cost
is deliberate: the phone's Overview is a scroll rather than
five slides.

### Equipment is what is worn; Items is what is carried

Both are `features/character/SheetEquipment`, one component each over the same
props, and they split the inventory by whether a thing *can* be worn rather than
by whether it is: a wearable in the backpack is on the Equipment tab, under the
doll it could go on.

**Equipment** is, on top, a **paperdoll** of twelve cards in three columns,
stacking on narrow screens: what is held -- Main hand, Off hand, Arms, Custom -- then what is
worn down the middle -- Head, Body, Belt, Legs -- then what hangs or is slipped
on -- Back, Amulet, Ring 1, Ring 2. Each occupied card shows the item's name,
available stats and Markdown description beside its icon. The labels are
the player's words for the catalogue's slots: Belt is `waist`, Amulet is
`neck`, Legs is `feet`, and the two ring cards are the one `ring` slot with
room for two. Arms is bracers *and* gloves, because the catalogue has one slot
for both (docs/dnd.md). **Custom** is the card that takes any wearable: a
second cloak, a third ring, a helm over a circlet. No portrait between the
columns; it
is in the page header already. Below the doll, the **Wearable** rows:
everything with a slot that is *not* worn, one row per entity however many
lists the server splits it across. A worn item is on its card and nowhere
else -- a row counts only the units still carried and is not drawn when there
are none -- so nothing is listed twice and "where is my armor" has one answer.

**Items** is the rest, read top-down with nothing to switch: the **Coins**
panel first, then **Consumables**, then **Other gear**, each a panel with the
same heading the Wearable rows have.

**Each tab ends with an Add button, and each adds from its own half of the
catalogue.** `AddItems` in `ItemPicker.tsx` is the button and the search it
turns into **in place, under the list it adds to** -- not a dialog, so what
was just added is visible above the search that added it. It is a name
search, a category select and a mundane / magic select over rows with **Add**
on each and **Load more** under them -- the spell selection's shape, because
like spells the items are never pulled whole (`searchItems` against
`…/characters/{id}/catalog/items`, docs/backend.md). **Close** puts the
button back.

- **Add equipment**, after the Wearable rows, lists only what has a slot;
  **Add item**, after Other gear, only what has none. That scope is the tab's
  and is not a filter the player can clear, so a thing added from a tab
  appears on that tab.
- **The category options arrive with the page.** The server names them and
  scopes them to the tab's half; nothing here derives them from data.
- **A row is pressed to read the item.** It opens to the same `ItemBody` the
  item page draws -- numbers, source, prose -- through `ChoiceDetails`, so it
  is inline on desktop and a full screen with Back and Add on a phone, as a
  spell's is. The description is asked for when the row opens
  (`getEntries` by slug): a search hit carries a row's worth and no prose.
- **One Add is one more in the backpack and one write** --
  `setTotal(equipment, slug, owned + 1)` through the same `onChange` every row
  menu uses -- and the search closes, as it does when a DM gives an item at a
  game: one thing, then back to the sheet. Staying open for the next would
  leave a player scrolling back up past a page of results to see whether
  anything had happened. An "Added" notice says so above the button and
  leaves after four seconds. It is shown when the sheet's own count of that
  item goes up, not when the button is pressed, so a write that failed is
  never announced as an addition. A row shows `×n` for what the character
  already has, read from the sheet.
- The buttons exist only with `onChange`, which is how a sheet says it is
  editable: the shared, read-only sheet has neither.

A custom item -- one the catalogue does not hold -- is written on a page of
its own, `features/character/CustomItemScreen`, and both tabs lead to it: an
**Add custom item** button beside each tab's Add, on its line and its equal
(`AddItems` takes it as `beside`), and the last entry of every
empty slot's menu, which is why that menu is never disabled.

**It is one screen for every way in, and a page rather than a dialog**, so its
trail is the way back. `/characters/:id/custom-item` writes on the owner's
sheet; `?slot=` (an empty slot's menu sets it) prefills the slot and, if the
item still goes there when saved, puts it on with the same `equip` changes the
slot's menu writes; `/characters/:id/custom-item/:option` changes an existing
one, reached from **Edit** on a custom item's own page. A game opens the same
screen at `/games/:id/characters/:character/custom-item` -- under `/games` so
the trail reads Games / the game / the character -- from **Add custom item** in a player row's menu,
shown to whoever runs the table, with the entry in `?entry=`; there it posts to
`/games/{id}/entries/{entry}/custom-items` and returns to the game, which says
what was given in the same corner notice the item search under the roster
uses -- the page passes it through the navigation's `state`. The form is the
name and description, an icon chosen from a grid of the packs' item icons
(fetched when the grid is opened, never before), category, slot, weight and
cost, and a kind -- neither, weapon, armor -- that reveals the weapon's or the
armor's fields. There is no delete: dropping the item takes it off the sheet.

**The sheet's tab is in the URL** (`?tab=`, `SheetBody`), which is what makes
the crumb back from that page -- and from an item's own -- land on the tab it
was opened from rather than on Overview.

The slot is the catalogue's: each item carries `slot`, written on the pack's
row or derived by the server from what the item is (see
docs/dnd.md). `domain/equipment.ts` reads it and nothing else, so a cloak is on
the back because the catalogue says so, not because the client knows the word.
The group is still derived here: *wearable* is anything with a slot, and
*consumable* is guessed from the category plus a short slug list, because the
catalogue has no "used up" field yet.

The server keeps one `equipped` list and one placement, `equipment.custom`, so
which item sits in which slot is derived too, bar that one: the Custom
occupant is seated first, then everything else goes by its shape, and a second
held item takes the off hand. Custom is stored rather than derived because
nothing about an item's shape could put it there -- a cloak in Custom while
Back stands empty is a choice, and the only way to keep a choice is to write
it down. Nothing is hidden: a slot worn past its capacity lists every
occupant, and an equipped item with no slot at all that nobody placed -- an
old log, an import, homebrew without the field -- is shown in an
**Elsewhere** card that appears only when something is in it -- it is not a
body part but "everything we could not place", so it is not a permanent slot.

**A weapon's numbers are captioned, not written out**, on the Actions tab and
the Equipment tab alike: `features/character/WeaponStats` draws **Damage**,
**Hit** and **Range**, in that order everywhere, each over its own caption and
the two that are rolled in colour. They are `StatColumns`, a stat block's
shape: the value large, because it is what is read, what it is small beneath,
a rule between one and the next -- and nothing in capitals, on any item
caption or badge. A dimmed sentence -- "+5 to hit · 1d8+3 · 5 ft." -- would be
three things read off mid-turn written as prose. Where a row is wide they are columns at its right, and on a
slot card they are columns beside the icon that wrap rather than clip; on a
phone's inventory row, which is too narrow for three columns beside a name,
they lead the row's list of captioned facts. The numbers come from the sheet's action for the weapon when it is
wielded (`weaponNumbers` in `options.ts`), so both tabs print the server's own
sum -- die plus modifier, and the bonus to hit; a weapon that is only carried
has no action, and shows the catalogue's die, type and range with no Hit --
*every* carried one, a second dagger beside the one in the hand included: the
list under the slots is never given the actions, so a carried dagger cannot
find the wielded one's by slug and print a bonus it does not have.
Properties, weight and thrown range are under
the name, each under its own caption; see the bubble below.

**Every item has a page of its own**, `features/character/ItemScreen`, opened
by **Details** in its menu -- on an inventory row and on a slot card alike, and
on a sheet that is only being read, where Details is the menu's one entry. It
is the spell page's shape: the icon beside the name, captioned facts (armor
class, damage, range, properties, weight, cost, the book it is from), then the
description. It reads the *character's* catalogue rather than the compendium's,
since an item on a sheet is that sheet's pack's item, homebrew included; the
two routes say whose -- `/characters/:id/items/:slug`, and the same under
`/groups/:id/characters/:character` for a shared sheet -- and the menu links
relatively, so it lands under whichever is showing.

That page is also why a **slot card says so little**. A card is a name
line -- the item, the slot it is in as a small badge, its menu -- over one
icon's height of columns: a weapon's three numbers, or armor's class and the
Strength it asks (`armorStats`; a shield's class is written as the `+2` it
adds), with a disadvantage on Stealth as one line beneath. None of the other
facts are there, and nothing worn says what it weighs -- weight is a fact about
carrying a thing. Properties, weight and the rest are one press away. An empty
slot is the same card with nothing in it: the slot's badge and the menu on the
name line, where a worn item's are, over the word. It keeps a full card's
height where cards sit side by side and is as short as its two lines on a
phone, where they do not.

**What could be worn is drawn as what is worn.** The Equipment tab's list
under the slots is the same card (`ItemCard` in `features/character/
Inventory`), three across on a wide screen and one on a phone, with a count
where the slot's badge would be: a dagger looks the same in the hand and in
the pack, and its other facts are on its page either way. As rows of captioned
facts, a Russian phone gives "Дистанция метания" half the row and breaks the
damage it is captioning over two lines. The fixed column widths that line
rows up are a wide screen's only; on a phone three of them are wider than the
screen.

Every inventory row on the Items tab is a **bubble** (`features/character/
Inventory`): the name, then the item's facts, **each under its own caption**
(`itemFactList` in `options.ts`, the list the item's page prints, less what
the row draws as columns and what the thing cost) -- not one grey sentence
("Thrown range: 20/60 ft. · Finesse, Light, Thrown · Weight: 1 lb."), three
kinds of thing run together, which on a phone wraps to three lines.
On a wide screen the properties are a line of their own and the rest are
caption-and-value pairs on the next; on a phone everything, the weapon's
numbers first, is one two-column list, captions down the left. The pack and
book badges are on one line in the bottom right corner on a wide screen, under
the numbers and the menu -- beside them they sat at a different place on every
row -- and **a phone's row has no badges**: which book a dagger is from is on
its page, and two pills wrapped under every row said it louder than the
dagger's damage. The columns of one row sit over the next row's:
in a list each is at least a fixed width (`Stat.width`), so "1d4 Piercing" and
"1d6 Bludgeoning" start at the same place; a slot card stands alone and takes
only what it needs. Nothing opens: everything a row has to say is on the
row. The words the line needs -- damage types, weapon properties -- arrive in
the sheet's `catalogNames`, resolved by the server like everything else on the
sheet (docs/backend.md#the-sheet-arrives-resolved).

Both tabs are **editable only when `SheetBody` is given `onEquipment`**. Only
the owner's `CharacterSheetScreen` passes it; `SharedSheetScreen` does not, so
a sheet shared with a table has nothing to press. With it, every row has a
**menu** on the right, and so does every worn item on its card -- the same
three dots, so there is one way to act on an item wherever it is drawn. A
card itself is never pressed -- that would make an empty rectangle a button.
An empty one has the three dots too, opening the **names of what is carried that fits that slot** --
nothing but names, since each has its card below -- and, last, **Add custom
item**, so the menu opens even when nothing carried fits. Naming an item is the row menu's Equip reached from the other end, through the
same `slotsFor` and `equip`, so the two cannot disagree about what goes where. A wearable's row menu has
**one Equip entry per slot it could go in** -- its own, Custom, and the off
hand for a held thing once the main hand is taken (`slotsFor` in
`domain/equipment.ts`; alone, a held item is seated in the main hand whatever
was asked, so the entry is not offered). A worn item's menu takes it off or
drops it. A row also has **Use** on a
consumable (one fewer, and nothing else yet -- no potion takes effect), and
Drop on anything, offered as *Drop one* and *Drop all* once there is more than
one. No count stepper: a dozen torches is still one row with one menu. The
purse is five fields, labelled with the coins' full names -- Copper, Gold --
because "cp" and "зм" are rulebook shorthand a new player does not read.
A reader of somebody else's sheet gets the same five fields, read-only, rather
than a line of text ("Gold: 15") that would be a second drawing of one fact.
Every edit is one `change` event on `equipment.*` paths, appended to the log.
Equipping writes the equipped list both whole and per slug -- see the comment
on `equippedChanges` for why the server needs both -- and into Custom, the
placement as well.

### What the sheet says about an unfinished character

A badge on the name reading **Unfinished**, and only when `/prompts` comes back
with `complete: false` -- not "with something in it": optional prompts stay
open for ever -- an alignment, a custom spell -- so that badge would never
come off a finished character, and the build screen and the sheet would
disagree about the same response. The badge is the whole message: the sheet
does not enumerate what is open, because enumerating it puts the build screen's work on
the page nobody came to build on -- an alert, a list of five questions, and the
sheet itself pushed below the fold on a phone. The screen that answers a
question is the screen that lists it.

The mark is not the *button*. One that appeared and disappeared with the
prompts would say two things -- here is the way in, and there is work left --
and a finished character would have no way back to the build screen at all.
**Edit** is on every sheet, and the news is a badge, where a rank and "Read
only" already go.

The **Event log** has no link on the page for now -- `/characters/:id/log`
still serves it, and is still the unabridged record, but nothing in the client
links there.

A `/prompts` that *failed* draws no badge. The request is deliberately
survivable -- a sheet is worth drawing with a second request down, which is the
same bargain the compendium lookups make -- and the honest reading of "I do not
know what is open" is to say nothing rather than to guess. Edit does not depend
on that answer and is drawn either way.

### On a phone the sheet is a deck, not an accordion

The same tabs under the character's name, one on screen, and a swipe
between them. **Nothing opens and nothing closes**: the carousel decides what is
visible, so a tab has no shut state to be in.

It is not an accordion, and the two answer different questions. An accordion
is right for a page of two or three panels where the answer is usually in the first and the rest are detail. A character
sheet is not that shape: its tabs are peers a player leafs between at a table.

The strip and the carousel are `ui/TabDeck`, which the build screen's stage tabs
also draw -- see [the tabs are a
deck](builder.md#the-tabs-are-a-deck-so-a-phone-can-swipe-between-them). It scrolls away
with the page rather than pinning under the header: one fewer row of chrome on a
screen this app has already spent an argument buying back (see [Two views, one
codebase](shell.md#two-views-one-codebase)), and a swipe changes tab from anywhere on
the slide, so the strip is not the only way through.

**The strip sits on a bar of its own, and nothing on a tab sits on the page.**
The sheet's tabs are the page's own rather than something inside a `Panel`, so
drawn plainly their captions lay straight on the backdrop's doodles with a
hairline under them -- and so did the Actions and Spells lists beneath. Build
and Group never showed it, only because their `TabRow` happens to be inside a
`Panel`. So the sheet passes `bar`, which `TabDeck` hands to `TabRow`: the strip
is drawn on the same bordered `Paper` a `Panel` is, and the list's grey rule is
dropped because the bar's border already draws that line. It is a flag rather
than the default, because a bar inside a `Panel` is a box in a box -- which is
why the Items tab's inner row does not pass it and sits in a `Panel` with
its rows and the purse instead. Every tab's content is in panels for the same
reason: two surfaces, the bar and then what it selects.

**The deck is as tall as the slide that is showing.** Every other slide is
given `height: 0` and left to overflow, so the viewport is sized by the one
slide with a height and clips the rest to it -- a neighbour is still drawn
sliding in during a swipe, cut at the foot of the panel being left. Nothing is
measured: the alternative, a `ResizeObserver` sizing the viewport, reads a
layout happy-dom does not compute, so the suite could neither exercise it nor catch
it breaking. Sized to the tallest slide instead, a swipe from the foot of
Overview would land a long way down a mostly empty Actions with the tabs
off-screen above.

**And exactly as wide as the deck.** Each slide carries `min-width: 0`. A flex
item does not shrink below its content, so a single row too wide for a phone
-- a weapon's name beside its three figures -- would widen its whole slide, and
embla, which centres a slide, would rest with both of the panel's edges cut off.
A row that does not fit is that row's problem: it wraps, as the weapon
row and a page's header actions do.

### What a phone spends its height on

Two of the sheet's measurements are chosen for a phone, which is where they
cost most:

- **The card grids gap at `xs` from `base` and `sm` from `sm` up** -- the ability
  row and the vitals rows.
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

**The main section reads the same at every width: who the character is, then
the ability cards.** A phone that opened on the six modifiers first -- on the
argument that a slide should open on what is reached for mid-turn -- would
make this the one place on the sheet whose order depended on width, and would
open a character's page on six numbers before saying whose they were.

Both sheet screens get this, because there is only one of them to get:
`SheetBody` is drawn by the owner's sheet and by the one a group member opens
for a character shared with their table, and the whole point of that split is
that the two cannot disagree about what a sheet is.

`features/character/SheetBody` prints ability scores and saving
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

That decides what a *missing* ability means. Scores and saving throws are two
projections and neither promises all six keys, and the card is the only place
either is drawn, so dropping a card with no score would swallow a save the
server did send. So the grid walks the **union** of
the two (`abilitiesOnSheet`), and a card with no score prints a dash where the
modifier goes and still prints its save. It claims nothing it was not sent, and
hides nothing it was.

Above them, `features/character/IdentityTable` says who the character is as
labelled pairs, in **four columns of two**: name over level, race over subrace,
class over subclass, background over experience. One dimmed line under the
name ("Elf · Wizard 1") reads well and answers badly: a line has no room for the subrace or the subclass, and a reader looking
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
page. Names come with the sheet, in `catalogNames`: one map keyed
`"<collection>:<slug>"`; keyed by collection as well as slug because two
collections may share one, and a bare slug map would let a background rename a
class. Without it the table falls back to title-casing, and "half-elf" becomes
"Half Elf" rather than "Half-Elf".

**The sheet asks the compendium for nothing.** The server has the character's
catalogue in hand when it projects the sheet, so it resolves every slug there
and sends the answer in the same response: `catalogNames` for what the sheet
only names, and `catalog` -- skills, proficiencies, equipment, magic items,
spells -- for the entries a panel reads more of, limited to what this character
references. See [the resolved sheet](../backend.md#the-sheet-arrives-resolved).
Opening a sheet is two requests, the sheet and its prompts, and a refresh after
an inventory edit is one.

Doing that lookup in the browser would mean fetching fourteen whole
collections, one of them every spell in the rules with its artwork inlined:
about 11 MB to print a dozen names. **No screen may download a
collection in order to name a few of its entries, and nothing may download the
whole spell list at all** -- a client gets the spells it names or one page of a
search. The sheet's own spells are the first case: they arrive resolved, icon
included, and the Spells tab (`features/character/SheetSpells`) draws them as
the build screen's rows -- icon, level, concentration and ritual tags, school,
casting time, components -- under a heading per spell level. What arrives is a
summary without the spell's text; opening a row fetches that one spell by slug
through the screen's catalogue scope, `/characters/:id/catalog` on the owner's
sheet and `/shared/:id/catalog` on the one a table reads, and shows it the way
the build screen does: inline below the row on a wide screen, as a full-screen
view with a Back button on a phone. The school and class names come from the
same scope's `spell-filters`, one small request per tab.

A class that prepares spells -- cleric, druid, paladin, wizard -- prepares them
on its owner's sheet. The tab finds the class's `prepared` question again the
way the build screen reposes a saved answer (the entry in the log that answers
`<class>/spell/prepared/<level>`, then `/prompts?before=` for the question as
it was asked; or the open prompt when it has never been answered) and draws
two lists: what is prepared, with Remove, and what could be, searched,
filtered and paged by the server from the question's options exactly as the
build screen's available list is, with Add. Spells the rules prepare on their
own (a domain's) are in the first list without a Remove. Every Add or Remove is
one write, as taking off a piece of armour is: an append the first time, a
replacement of the answering entry after that, and a delete when the last
prepared spell goes, because an empty up-to answer is one the server treats as
unanswered. There is no draft and no dry run -- nothing in the log depends on
a preparation answer, so there is never anything a change could drop. A shared
sheet shows the same rows and opens the same text, and nothing on it can be
pressed to change the character.

Under the cards is a second headline row, `features/character/Vitals`: passive
Perception, the spellcasting numbers, speed, vision and Hit Dice. Hit Dice are
here rather than beside the backpack because they are a fact about the body,
not about the kit.

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

The skills beside them are a different case. `features/character/SkillsPanel`
draws **all eighteen**, ordered by how trained the character is — Expertise,
then proficient, then half, then untrained, alphabetical within each block.
Eighteen rows of which six matter is a
different problem: the question is what the character is good at, and the
answer should not be scattered down a list of things nothing trained. The
alphabet still breaks ties, so each block stays stable and searchable.

It draws eighteen rows because the **server sends eighteen**, not because the
client fills in the gaps. The untrained skills arrive with their bonuses
already computed (see [dnd.md](../dnd.md#the-projected-sheet)); this panel adds
nothing up. Unioning the sheet against the compendium and adding an ability
modifier here would be the browser computing a rule, which
[`domain/format.ts`](../../web/src/domain/format.ts) exists to forbid — and it
would be wrong the day Jack of All Trades starts halving a bonus.

What the compendium *does* supply is each skill's **name and governing
ability**, which arrive in the sheet's own `catalog.skills` -- all eighteen,
because the panel draws all eighteen. The name matters twice: it is in the
negotiated locale, and it is the only spelling that gets "Sleight of Hand"
right, where title-casing the slug capitalises the "Of". A sheet without them
-- the one a write echoes back carries no `catalog` -- loses the ability tags
and falls the names back to the slug; it does not stop the panel drawing.

Training level is carried by a mark — `ui/ProficiencyMark`, one glyph filling
in across the four levels, with Expertise ringed rather than merely fuller
because it is the bonus counted twice rather than more training. The mark is
what separates the rows, and the dimming of untrained ones is a **second**,
redundant channel: a panel distinguishing eighteen rows by a shade of grey
reads to nobody on a monochrome print and to nobody who cannot tell the two
greys apart. **There is no filter over the top of it.** A "Hide untrained"
toggle would answer the question the eighteen rows exist to answer -- what do
I roll for a skill nothing trained -- by taking those rows away. The two
channels above already separate the trained from the
untrained without anything to press first.

Skills stays at half width rather than spreading: its rows are name, ability and bonus, and
a full page width would set the bonus so far from the name that the eye travels
back along an empty line to pair them. **`features/character/ProficienciesPanel`
takes the half beside it** -- everything the character is trained with that is
not a skill or a save, grouped into Tools, Weapons and Armor. A comma-joined
paragraph at the foot of "Traits and features" would be a sentence to be read
rather than a list to be searched, and would file a tool a player rolls with
beside a racial trait they never touch again.

**Consumables have their own panel.** Everything a character spends -- spell
slots by level, Pact Magic, Channel Divinity, ki -- is drawn by
`ResourcePools` as rows of marks in a "Consumable slots" section, present only when
the character has a pool. It is the same component the game page unfolds, here
without `onChange`, so it is read-only and always full: what has been spent is
a fact about one game.

**"Traits and features" is a list of rows that open**, one column at every
width, drawn by `ui/BlockList` as the Actions tab is. Not a grid: a grid row is
as tall as its tallest cell, and one name long enough to wrap -- "Wild Shape
(CR 1/4 or below, no flying or swim speed)" -- pushes its short neighbour away
from the rows around it, so the list reads as uneven.

**A row opens where it stands onto what the entry says.** A player who reads
"Fey Ancestry" on their sheet should not have to leave it to learn what that
is. The prose arrives in the sheet response as `catalog.traits`,
`catalog.features` and `catalog.languages` -- only the entries that have any --
so opening a row is not a request. A row with no prose is a statement, not a
control: a 2014 entry outside the SRD on a server without the private overlay
pack, or a scaling value no feature is named for. One description is open in
the panel at a time, not one per group. A statement and a closed row are the
same height, which is `BlockList`'s doing and holds on the Actions tab too.

**A group that does not apply is left out
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
and the *type* that decides which rows get it comes from the sheet's
`catalog.proficiencies`, resolved by the server beside the skill names.

The stat row above leads with hit points, then armor class, initiative and
proficiency. Hit points are the one number that moves between one glance and
the next and the one reached for mid-turn, where armor class is settled at the
start of a fight; at two columns on a phone, first position is the only one
visible without the eye travelling.

## The log has its own page, and it never asks for the sheet

**Nothing links to it at the moment** (see [what the sheet says about an
unfinished character](#what-the-sheet-says-about-an-unfinished-character)), so
the route is reached by typing it. The screen and its route are kept whole:
it is the one page that can answer "why do I have this proficiency?" when the
projection has gone wrong.

Its breadcrumb trail is `Characters / Event log`, where every other detail
page names the thing it is about. That is the same rule, applied one step
further: the character's name lives on the sheet, and fetching it for a crumb
would reintroduce exactly the dependency this page exists without. The trail
says less instead. `/characters/:id/build` is the asymmetry worth noting -- it
already holds the sheet, so it gets `Characters / Ada`.

`/characters/:id/log` draws the character's event log: one row per stored
event, in sequence, with the kind read back as prose, the time the server
stamped and the payload -- references, answers, changes -- as the log holds
them. It is the page [dnd.md](../dnd.md) implies by justifying event sourcing on
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
