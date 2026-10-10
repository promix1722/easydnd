# The build screen

Part of the [web client documentation](../web.md).

## The build screen is a loop, not a wizard

The screen is one component and the parts it is made of, all in
`features/character/`. `BuildScreen.tsx` is the orchestration and the JSX.
`useBuildView` reads prompts, events and the sheet; `useBuildDrafts` holds
unsaved inputs; `useBuildStages` decides which tab and block are open;
`useBuildActions` owns every write and the drop preview. `buildModel.ts` and
`answers.ts` are the view types and the pure functions between an answer and
the event that records it, and `DropPreviewSheet.tsx` is the "this would
drop" dialog.

`features/character/BuildScreen` reads `/prompts`, `/events` and `/sheet`, and
draws up to eleven tabs -- Cantrips and Spells only for a caster, Equipment only
on a visit that was asked about a starting kit, Custom once the character
exists, see [Builder choice
behavior](#builder-choice-behavior). It is a loop rather than an N-step wizard, and it has to
be: prompts nest -- answering the "two skills" branch of a rogue's Expertise is
what brings the two-skill prompt into existence -- so the total number of steps
is not knowable until the last one is answered. The tabs are not steps. They
are display categories in the order `domain/stages.ts` states: rules, personal
(the server's `identity` group), class, abilities, race, background, cantrips, spells, equipment, personality,
custom. Spell and equipment
choices use their kind to select a tab; other choices use the server's group.
Saved answers use the same mapping, with prompt IDs as a fallback for older
events without choice metadata. Rule ownership remains class/race/background.
Class comes first after the name, because it is the choice the most other
choices hang off -- and the scores straight after it, because they are
what the class was picked *for*: a barbarian wants the 15 in Strength, and
deciding that while the class is still the last thing you looked at is the
difference between building a character and filling in a form. Personality is
the last tab that asks anything, and asks nothing about the rules -- see
[below](#who-the-character-is-is-its-own-tab-and-its-own-words). **Custom** is
after it and asks nothing at all -- see
[Custom is what the player writes unasked](characters.md#custom-is-what-the-player-writes-unasked).

### The tabs are a deck, so a phone can swipe between them

The tabs are `ui/TabDeck`. **On a phone** it is a strip over a carousel of
all the panels, every one mounted, one on screen: pressing a tab scrolls the
carousel to it and swiping the panel reports the tab it landed on, and neither
can drive the other in a loop -- scrolling to the slide embla already holds does
nothing, and the deck only reports a slide that is not the one the caller asked
for.

**The strip scrolls on a phone and wraps on a wide screen.** `ui/TabRow` keeps
its tabs on one line under `md`, where a swipe finds the ones off the edge and
the strip comes to rest on the active tab. Above `md` there is no swipe, and
that same rest position would hide the *first* tabs: a caster's builder in
Russian is ten or eleven tabs, wider than the page's content column, and
opened on Spells it would show the row from "Race" on with nothing to say
there was more to the left. So on a wide screen the list wraps onto a second line instead. It
costs a row of height and hides nothing; the scroller is still there and has
nothing to scroll, so the edge fade never draws.

**On a wide screen there is no carousel at all**, only the strip and the panel
that is showing. The carousel answers a phone and nothing else: there the panel
is the biggest thing on screen and a swipe across it is the cheapest gesture
available, where reaching back up to a 60px tab is the dearest. With a mouse the
tabs are one click away, a drag across the page is how you select text, and
mounting every panel to show one is many times the work for a gesture nobody
makes. That is the branch, and it is why `TabDeck` is on the list of components
whose two renderings differ rather than the list whose renderings match.

**A press scrolls, a swipe is left alone, and anything else jumps.** Sliding
from one tab to the next answers a press -- it shows which way you went. The
same slide arriving unasked is the page moving while you are reading it, which
is what a cold load of the build screen would otherwise do: it opens on the
first unanswered category, so every load would begin on the first tab and
slide sideways off it.

A swipe is the third case: embla selects the slide the moment the gesture
decides and is still settling on to it, so a deck that "synced" to that
selection would cut its own animation short. The deck checks
`selectedScrollSnap()` and does nothing when the carousel is already where it is
being asked to go.

This is the same object as the character sheet's deck, and that is why it
lives in `ui/`: stage tabs a player leafs between are sheet sections under a
different name, and two copies of a two-way embla sync are two copies of the
one thing in it that goes subtly wrong. The sheet has no desktop half of its
own either: it is the same tabs at every width, and this primitive draws both.

Two things follow from every panel being mounted, and both are worth knowing.
Where a block sits is remembered per tab rather than for the screen as a whole
(see [One block per choice](#one-block-per-choice)): with every tab drawn at
once, one shared memory would let the place a dropped answer vacated under
*class* be claimed by whatever arrived next under *background*. And a test
about what a tab holds has to say which tab -- `BuildScreen.test.tsx` scopes
those queries to a slide, because a query against the whole document sees
every category at once.

The deck does not swipe while the character does not exist. A tab press there
*creates* one rather than moving anywhere, so a swipe would be a gesture the
screen answers by refusing to move, and a deck that snaps back is worse than
one that never gives.

Keyboard focus walks all the panels rather than stopping at the end of
one, and the strip follows it: that is embla's own `watchFocus`, which scrolls
a focused slide into view and emits the same `select` a swipe does. Nothing in
`TabDeck` implements it, which is worth saying because the obvious hand-rolled
version is a focus handler that fights the one already there.

The screen opens on the first category with something required outstanding --
and, for a character with nothing required left, on the first tab -- not the
first tab with *anything* open: a custom spell is on offer to every character
for ever, and Edit would open a finished fighter on Cantrips. That is the whole of the help it offers: **answering does not move you**.
A tab changes when a tab is pressed, and never on the way back from a write --
answering one question is not a request to be asked another, and a player who
has just chosen barbarian is usually looking at what barbarian brought with
it. That has no exception for the first answer either: see
[Creating is answering the first question](#creating-is-answering-the-first-question).
There is no Next in the tab row for the same reason: the order is the player's,
and a control that walks the tabs in the server's order is a wizard's stride in
a screen that is not a wizard. `Finish` is on the trail's line, in `Page`'s
`actions`, because it acts on the character rather than on the tabs.

A `Next` does appear **under the list, once a tab has nothing left to answer**,
and only then. That is not navigation -- the tabs are always there -- it is the
end of a piece of work saying where the next piece is, at the moment when that
is the only thing left to say. It goes to the next later category with anything
still open, optional included -- that is how a player reaches Personality --
and wraps round only for *required* work. The standing offer of a custom
spell (`purpose: custom`) is not followed: it is open on every character, and
Next would walk a fighter through Cantrips and Spells because of it. **With nowhere left to go and
nothing required open, Next is Finish**: it lands on the sheet, the same as
the button on the trail. Until the character is complete the last tab has no
Next. The build screen prints no line saying everything required is
answered; the filled Finish and a Next that leaves say it.

Three requests, because they answer three different questions: `/prompts` says
what is still open, `/events` says what was decided and in which entry, and
`/sheet` says what all of it adds up to. The sheet cannot be folded from the
log in the browser -- an ability score improvement's increments are derived at
projection time -- so it is fetched rather than computed.

**Nothing can be answered before it is asked.** Every tab is freely clickable,
because a tab is a place to look as well as a place to answer, but what can be
answered on one is exactly what `/prompts` returned for it. One surface draws
those questions -- the build tab, as blocks that open -- and it is the only one
in the client. The character sheet does not list them. See [what the sheet says
about an unfinished character](sheet.md#what-the-sheet-says-about-an-unfinished-character).

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
and twenty arrows has no slug of its own, and the server composes it one --
`shortbow+arrow` -- so a settled row can read it back as "Shortbow, Arrow"
without asking the compendium what the answer meant.

The exception, and its bounds, is the character's **inputs**: a name, an
alignment, the six ability scores, the desired level and the ruleset. They
settle a value on the sheet rather than naming a catalogue entry or answering
a grant, so each is stored as an addressed change -- and the prompt, which
says the entry is a `change`, has nowhere to say to which path. `INPUTS` in `features/character/answers.ts` is that table and is
deliberately the only place a path is written down. It is worth knowing why it
exists: an alignment is namespaced `character/alignment` exactly like
`character/race`, this screen read the namespace as the shape, and the
`change` event it posted -- naming an alignment, changing nothing -- was
accepted by a server that could attribute it to no prompt. The alignment
simply never saved, with a 200 to say so. (The server refuses that shape now,
`field.answer.notAPick`: the AI Wizard made the same mistake later, and two
such entries beside the still-open question are three blocks under one key.)

A settled input reads back as what it is. An alignment is shown by the
compendium's name for it, asked for alongside the entries' references
(`alignment:<slug>`), and not as its slug in title case -- English in every
language, and under a rule pack "Dnd 2014/neutral Good". The four written
lines are printed as written even when they are stored as a list, which is
how the wizard stores them: printed the way a list of slugs is, "кто-нибудь"
comes out "кто Нибудь".

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
there. Run along the same line as the name and cut to 120 characters, it gives
a list of six draconic ancestries six half-sentences and no whole one. Under
the name there is room for what the compendium actually wrote, and
under *only the picked one* the list stays a list -- the description is shown
where it is being decided about.

Some questions are genuinely not that shape and get a form each: a name
(`NameForm`), the six ability scores (`AbilityScoresForm`, below), the four
roleplaying lines (`WrittenForm`), the desired level (`DesiredLevelForm`) and
the ruleset (`RulesetForm`). `StagePanel` chooses between them and `PromptCard`
by the kind of the prompt, and that is the only place in the client mapping a
prompt kind to a control.

None of them says what the question is, and there is no exception. The
block they open inside is headed by the choice's own name, and a surface that
repeated it -- "Two more languages", then "Choose 2 more languages" -- would be
asking twice. `NameForm` is not the exception, though "What are they called?"
is the first line anybody reads in this application: with that heading *and* a
field label under it, the first screen anybody sees would say "A name", "What
are they called?" and "Name", three times over one empty box. The block says
it; the field carries an `aria-label` for whoever
cannot see where it sits.

### One block per choice

A tab is one list, and every choice on it is a block that opens onto its own
answering surface -- not what has been decided in one card, what is left in
another, and the question in hand detached at the bottom.

A decided choice and an open one are the same object at two moments, so they
are one list rather than two sections: the choice of a race *is* the question
"which race?" once it has an answer. `features/character/blocks` merges them,
sorted by level with everything un-levelled first, which reads as the story it
is -- took rogue at 1, still owes two skills at 1, gained a level at 2 -- and
`groupByLevel` then cuts the ordered list into one section per level, so the
panel draws a "Level N" heading over each run instead of a tag on every card.
What tells the two apart is that an open block is drawn to stand out, not
where it sits.

A question a **feat** opens carries the level the feat was taken at, and the
server is what says so (`featLevel` in `domain/character/prompts.go`): Slasher
taken at a fighter's fourth level asks "+1 to Strength or Dexterity" as part of
what fourth level asked. Without a level this list would read it as "belongs
to none" and draw it above Level 1, ahead of the improvement that opened it.

**Nothing opens itself**, with one exception below. The screen does not open
the first open question of the tab: it has no way of knowing which of five a player
came here to make, and a surface that opens itself is one they have to close.
One block is open at a time, and answering closes it rather than advancing to
the next question.

**The list grows; it does not rearrange.** Level order decides where a block
goes the first time it is drawn, and after that it stays there:
`features/character/blocks` keeps a `BlockOrder` of where everything sits, and
a key it has not seen sorts to the end. There is one `BlockOrder` **per tab**,
because where a block sits is a fact about the tab it sits on and every tab is
drawn at once -- see [the tabs are a
deck](#the-tabs-are-a-deck-so-a-phone-can-swipe-between-them). Answering a question therefore adds
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

The trail reads `Characters / Ada`, and ends there: a third crumb naming what
the screen does -- "Creation" -- answers nothing a screen full of questions
is asked, and costs a line of a 390px trail to say it.
The route stays `/build` either way, because a URL somebody has open is not
worth breaking over a word. The tabs are capitalised for the same reason a heading is -- a tab
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
pre-empted, and a front door whose one row is shut reads as broken. There is no
separate create screen because there was never a second thing being done. Nor
does creation carry the score method and the six numbers: that would be eight
selections in one log entry and nothing a player could point at and change. The
scores are an ordinary open choice, answered on the abilities tab and written
as their own entry.

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

`/characters/new?folder=` carries the folder whose **New character** was
pressed, so that is where the next character lands. Absent, the server resolves the account's
default.

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

An answer that carries *picks* -- a rogue's Expertise, a starting weapon --
cannot be rebuilt from the answer alone, because the options that made it up
arrived with a prompt the server stopped emitting the moment it was answered.
So the screen asks for it: `GET /prompts?before=<seq>` returns the questions as
they stood at that entry's position, `repose` finds the one the entry answered,
and the block opens **on its answer** -- as deep as the answer went, so a focus
picked out of "an arcane focus" opens on the list of foci with that one pressed.
Opening writes nothing; a change replaces the entry in place.

Dropping the entry instead and waiting for the question to come back would
reach the same question, blank: a player who opened a card to see what they
had picked would see nothing picked, and would in fact have just unpicked it. The drop
remains only where the question cannot be found again -- an entry that bundles
several questions, or a prompt the server no longer poses there -- and then
`reclaimPlace` holds the block's own place for what returns, and it is asked
about on the same rule as everything else: only if another answer cannot
survive it.

The question that comes back is also **open**. `done` takes the key of a block
that does not exist yet, and the reread is what brings it into being; without
that the row you pressed turned back into a shut question and wanted pressing
again, which is the same gesture twice for one intention.

A **subrace and a subclass** take the same route, for a different reason:
their options are narrowed by the entry above them. The server offers a
half-elf their subraces and a rogue their archetypes, and this screen cannot
know either list -- rebuilding the question from the answer's own kind, which
is right for a race or a class, offered Berserker and Champion to a rogue.
`NARROWED` in `answers.ts` is that list, and `reask` returns nothing for it.

### A choice inside a choice is answered where it was asked

Some options are themselves questions. "A martial weapon and a shield, or two
martial weapons" makes the second branch a choice; so does the improvement a
level grants, whose first branch is "raise two of your scores". Confirming
such a pick, posting it, and drawing the branch the server then poses as a
**second block** further down the tab would turn a question the player is in
the middle of answering into two questions in two places.

`PromptCard` resolves it in place. Two things make that possible: the branch's whole inner choice ships inside the option it is
(`Option.choice`), and the server validates a batch of answers one at a time
against a log that grows as each lands -- so a branch answer and the answer it
opens travel in one event, the second legal because the first arrived. So
picking a branch draws its options in the same card, with nothing fetched and
nothing posted, and Confirm sends all of it. `opens` is the client's mirror of
the domain's `addOpened`, bundles included, because "a martial weapon **and** a
shield" is a bundle whose first item is a question.

A branch that is the only branch is not a question, and `settle` takes it on
arrival rather than making it a click: the improvement opens straight onto the
six abilities, with "or a feat" still there to change to.

`loadEntries` reaches into branches for the same reason -- an inner option's
name has to be there before anything is posted, and a branch drawing on a whole
collection ("or a feat") pulls that collection in the same pass.

One entry carries the branch and its contents, so the settled row would
read "Expertise, Skill Stealth, Skill Acrobatics" -- the choice named once as
itself. `leafAnswers` drops the branch answers: an answer whose id is extended
by another answer's in the same entry (`X` and `X/0`) said only which way the
question went. Every branch id in the compendium extends its parent's, and so
does the improvement's, which the server synthesises. The log screen reads
through the same function.

### A category's word appears exactly once

In its tab. A block is headed by the choice's own name -- "A race" -- or by
what was decided -- "Race chosen" -- and never by the category alone, and the
one line a tab with nothing to ask still prints is "Nothing to answer yet"
rather than "nothing to answer in race". (A tab whose questions are all
*answered* prints nothing: the blocks are still there to read, so a line
announcing their absence was announcing something that is not absent.) It is a
small rule with a large payoff: `getByText('race')` means one thing on this page, and
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

**Use recommended** deals the scores out for the player, and is there only
when the server has advice to give: the ability-score prompt carries
`recommended`, the character's class's abilities in order of importance, read
from the class's `abilityPriority` in the pack (docs/packs.md). The advice is
an order rather than six numbers so that it fits every method: the dealt
numbers -- printed or rolled -- go out best-first by it, and point buy and
manual, which have no numbers of their own, get the standard array placed the
same way (point buy only where its budget can buy that array). No class yet,
or a pack with no priority for it, means no button rather than a guess. It
fills the form and stops; Confirm is still the player's.

The two that deal out a set share `ScoreAssignment`: a pool you take from and
six abilities to put numbers on. Two gestures, one operation: drag a number, or
tap it and tap where it goes. The tap is not a fallback -- it is the keyboard's
path and the one a screen reader can follow, since every half of the surface is
a real `<button>`.

**The drag is pointer events, not HTML5 drag-and-drop**, and that is what makes
it a gesture a finger can make. `draggable` + `dragstart` is a mouse protocol
that no mobile browser fires for touch, so on it the instruction "drag a
number onto an ability" does nothing whatsoever on the device the app is used
on at a table -- and a single pointer API covers finger, mouse and stylus with
one implementation.
Three things the native protocol did for free are done here instead: the drop
target is found with `elementFromPoint` over a `data-ability` attribute, the
gesture is held with `setPointerCapture` so events keep arriving once the finger
has left what it started on, and `touch-action: none` on anything you can pick
up stops the browser scrolling the page and cancelling the drag on the first
millimetre. A press that never travels `DRAG_THRESHOLD` is not a drag at all and
falls through to the click, which is what keeps the tap exact.
`features/character/scoreDrag` holds the arithmetic and the lookup, in its own
module because a file that exports a component may export nothing else without
losing fast refresh.

The carousel had to be told to keep its hands off first, which is what
`ui/swipe.ts`'s **`NO_SWIPE`** is for. Embla takes the pointer down on any
element that is not a field, and from a few pixels of sideways drift it both
scrolls the deck and swallows the `click` that was about to land -- so on a
phone a drag towards Strength would swipe to the next tab, and even a tap on a
44px square pressed with a thumb would often do nothing at all. `TabDeck` passes
`watchDrag` a predicate instead of `true`, and a surface marked with the
attribute keeps its own gestures: the deck is still swiped from anywhere else on
the slide. `ScoreAssignment` marks the whole
surface rather than each control, because the drift that has to be tolerated
happens between a press and its release and the release is not always over what
was pressed.

The number follows the pointer as a ghost, and it is drawn in a `Portal` for a
reason worth writing down: `position: fixed` is positioned against the nearest
*transformed* ancestor rather than against the viewport, and the carousel's
track is translated on every frame. Drawn in place it is laid out against that
track and clipped by its overflow -- the drag lands and nothing appears to
move. For the same class of reason the gesture's state is read from the render
its handler was made in rather than through a `setState` updater: an updater
must be pure, placing a number is not, and StrictMode invokes it twice to prove
the point.

The suite drives the drag by hand, with `document.elementFromPoint` stubbed:
happy-dom computes no layout, so the browser's one contribution to the gesture is
the one thing a test has to supply. The threshold, the swap and the click that
must not undo the drop are the real code.

Dropping onto a taken ability swaps the two. **Dropping one anywhere that is not
an ability returns it to the pool** -- undoing a placement is half of dealing six
numbers out, and tapping the number and tapping it again is not what anybody tries on a surface that says "drag". Tapping a
placed number twice still works. Nothing can be confirmed until all six are placed,
because six numbers and five decisions is not an answer. Both halves are drawn
at least 44px square: a badge is a few millimetres of target for the one gesture the whole surface exists for.

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
that method actually produced six. Otherwise switching to it from an unplaced
array would show six zeros -- under its own stated minimum of 1 -- because an
ability nobody has dealt to reads as a 0.

The dice live in `domain/abilities` too, and take the die as a parameter: a
test that cannot say what was rolled can only assert that six numbers came
back, and "between 3 and 18" is not a test of dropping the lowest. The
algorithm is the SRD's rule verbatim -- roll four d6, total the highest three,
six times -- and SRD 5.1 has no re-roll-if-unplayable clause, so neither does
this.

### Who the character is is its own tab, and its own words

`personality` is the last tab with questions -- after spells and equipment,
since none of it is required and the character can be played without it --
and the only one whose questions are not about the rules: a personality trait, an ideal, a bond, a flaw and an alignment. They are
the *background's* questions -- it is the acolyte entry that suggests what an
acolyte tends to believe -- but under background they would put five
questions nobody has to answer in front of the one required question on that
tab. The group of their own is the server's, not this client's:
`domain/stages.ts` has a line for it, and nothing else here knows the tab
exists.

The four are **written, not picked**. SRD 5.1 prints eight of each and the
compendium carries them, but a menu of eight leaves no way to say anything
else about a character who is yours, and the state behind them is free text
(`State.Identity.PersonalityTraits` is `[]string`). The suggestions are still in the compendium for anybody who wants to
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
asked is one answer in the player's own words, which is a sentence and
sometimes several. Nothing written is the same as not answering, and these are
optional, so the button simply stays disabled.

It is a fixed three rows rather than an autosizing one. Mantine's autosize is
`react-textarea-autosize`, which measures through a listener happy-dom has no
element to attach -- the field could not even be focused under test -- and
`vi.mock` is not available to paper over it. Three rows and a scrollbar is a
smaller loss than a control the suite cannot drive.

The prompt is told apart from a menu the same way the six starting scores are
told apart from a level-up improvement: **by whether it offers anything to pick
between**. That is the server's own statement of what may be picked here, not a
slug this client has memorised.

### Level-up is the desired level

Levelling up is one gesture: declaring the level the character is built
towards. The class tab asks it as `character/desired-level` -- a number
form, answered as the change event that settles `identity.desiredLevel` -- and
the sheet's **Level up** button raises the same declaration: it *replaces* the
entry that made it (the same revision the class tab performs, via
`features/character/desiredLevel.ts` so the two write the same change) and
lands on the class tab, where the choices the declaration opened are waiting.
The declaration *is* the level: the server raises the character to it and
poses what those levels open -- the archetype, the improvements, a feature's
picks -- as ordinary prompts in the ordinary list. So "level me to 9" is one
number followed by the build loop, not a wizard, and this screen has no code
that knows what a level is.

The rules tab asks `character/ruleset` once -- the rules edition of the
selected core pack, confirmed like any other choice -- and the answer is
final: the settled block is drawn locked (`blocksFor` marks it unchangeable), and the
server refuses any change to a value the compendium does not serve.

**All three identity questions are drawn before the character exists**, in the
order they are asked: the name, the rules, the level. Only the name can be
answered there -- it is what creates the character, and the other two are
answered against one -- so the other two are drawn with no body at all, which
`ui/BlockList` already renders as a statement rather than a control that would
fail. `NEW_INITIAL_PROMPTS` in `buildModel.ts` is the client's copy of what the
server poses a moment later, so confirming a name settles a row rather than
growing two new ones underneath it.

**Nothing here asks which class a level went into**, and nothing fills it in
either. There is no such question to ask: multiclassing is not offered, so a
level has one place to go, and the server does not pose it -- see
[backend.md](../backend.md#creation-and-level-up-are-one-flow) for what turning
it on costs.

Levels a character already *has* stay visible as settled blocks on the class
tab, under their own "Level N" headings (`groupByLevel`), and stay read-only:
`blocksFor` marks a bare level entry unchangeable, and `ui/BlockList` draws
one as a statement rather than as a control that refuses. They are facts about
the character. The way back down is the class tab's level, revised like any
other entry -- priced and confirmed, so what those levels bought is shown
before it goes.

What a level *granted* is a different thing wearing the same event type, and
stays changeable: an improvement, an Expertise, a feature's pick all arrive as
`level` events carrying answers, and locking them with the levels would take a
whole class of real decisions off the screen.

## Builder choice behavior

Creation and level-up use the same tabs and server prompts. Cantrips and
Spells have separate tabs; all equipment choices live in the final Equipment
tab, and nothing else does. **That tab is drawn on a visit that was asked a
starting-kit question, and stays for the rest of that visit** -- otherwise
answering the kit would take the tab away mid-creation, and the answer with it. The kit is
asked once -- by the first class, at level 1 -- and its answers seed the
inventory; after that the character's things are edited on the sheet's
Equipment and Items tabs, so a visit that opens with the kit already answered
draws no tab: Edit and Level up never show one. There is no stored "finished"
flag, so "creating" is exactly "this visit saw a kit prompt"; leaving a
half-built character and coming back with the kit answered hides the tab too.
The spell tabs' Next and save land on the tab after them with work or stay put.

**Editing is the creation screen, locked where it is final.** The Rules tab of
an existing character draws the same ruleset form and the same pack buttons as
creation, disabled, with "This choice is final" -- not a summary card and a
text list, which read as a different screen. Reopening the level-1 class choice
re-poses the kit, and the tab comes back with it. A selection
changes a card in place: choosing, confirming, reopening, or replacing it must
not sort the wizard's question cards. Spell choices have an explicit selected
list above their filters; other option lists retain their original order. Answered choices use the original
prompt identity; other events use their persistent ID. New questions append to
their level group. Sequence numbers address writes, not visual identity.

All displayed equipment and spell names resolve through the localized catalogue.
Saved answers use the API's resolved `selections`, which retain quantities and
fixed bundle components. A crossbow and twenty bolts therefore remain two named
items with the ammunition count, including in Russian. Catalogue requests show
loading and retry states; stale responses cannot overwrite a newer language.
Selections remain in hand while option details load or the language changes.
Large spell lists load in bounded catalogue batches rather than exceeding the
API slug limit. Failed requests cannot be
confirmed as if the catalogue were complete.

Starting-equipment cards show complete descriptions plus armor and weapon facts,
weight and pack contents for the selected option. Bundles describe each
component. Equipment-category choices support their full membership, and a
multi-item category permits repeated copies. Confirmation still requires the
requested number of items. Conditional cleric gear stays visible but disabled
until its proficiency requirement is satisfied. Fixed equipment is granted by
the rules; choosing equipment puts it in the backpack, without automatically
wearing it. Starting wealth shopping is not part of this screen.

A pack's contents are a list under the selected option, one item per line, with
a count only where it is more than one. "×1" is printed nowhere: one of a thing
is the thing.

The tab is its questions and nothing else. There is no inventory list, no coin
fields, no custom items and no way to add an item here: what the character
starts with is decided by the choices (and the fixed grants behind them), and
it is read, worn, counted and added to on the sheet's Equipment and Items tabs.
A list of the projected inventory under the questions would repeat every answer
a second time and invite editing a starting kit by hand.

An option is not drawn until its catalogue entry has loaded. Drawn early it is
named by its slug and then renamed in place -- "Scholars Pack" becoming
"Scholar's Pack" with its source tags -- which read as the card flashing.

A class kit card is **titled by the slot it fills** -- Body, Main hand, Off
hand, Backup weapon, Pack, Focus, Instrument -- because that is how a player
fills a sheet, and the book's "(a) chain mail or (b) leather armor, a longbow
and 20 arrows" reads as word soup when it is joined into a title. The slot is
the server's, on the choice (docs/dnd.md), so the card is named before its
options have loaded; a kit question with no slot -- a background's, a pack
built before slots -- is still **titled by what it offers**, "Component pouch
or one of: Arcane Foci", and one that is only a category reads "Also one of:
Simple Weapons", which tells it from the card above that offers the same
category as one of two options. Armor and weapons are separate choices in the
catalogue. A weapon bundle's label leaves its ammunition unsaid -- "Longbow",
the arrows being what a bow comes with -- and
the detail under the picked option still lists them. No price is shown on an
option: starting equipment is granted, not bought.

A picked option and its description are one box, in the picked colour; the
description is beside the button rather than inside it only because it is
Markdown. The box owns the horizontal padding, so the name and the description
start on one left edge, and the description is smaller so it reads as
being about the option rather than as another one -- not dimmed, because grey
on the picked colour is not legible. The focus ring is drawn round the whole
box (`.picked-option` in `ui/app.css`), not round the button, which framed the
name and left the description outside. An optional block gets no accent
border: that border means "still has to be answered". An item's numbers (damage,
range, weight, cost) are one line joined by middle dots, not a paragraph each;
a bundle heads each component with its name in bold. One of a category is named by the category: "Arcane Foci", not
"1 × Arcane Foci".

A held trait, feat, fighting style or proficiency cannot be selected again where
it would duplicate a benefit. Expertise offers proficient skills/tools that have
not already received Expertise. Explicitly repeatable choices, including ability
improvements and copies of equipment, keep their repetition behavior. Personality
is free text, not a mechanically deduplicated list.

Spell cards identify cantrips, spells known, spellbook additions, preparation,
replacement and special grants. The builder shares the spells page's search and
filters: level (including cantrips), school, class, casting time, concentration,
ritual and no material
components. The wizard's ‘Available for your character’ filter is enabled by
default and uses the union of the server's eligible options, including racial
and subclass grants. Turning it off loads the wider catalogue for the current
Cantrips or Spells tab and allows any catalogue spell to be added, including
spells beyond the normal class, level and count limits. A pick that fits a
normal remaining allowance uses it; other picks are stored under the explicit
custom-spells source. Custom picks remain marked after saving and when the
filter is enabled again. They do not satisfy or inflate class/racial allowances.
Reset restores the eligibility filter. Browsing never changes saved or draft picks. Each spell row shows its icon,
level, concentration/ritual tags, school, casting time and components.

Opening a spell previews it without adding it. Desktop previews expand inside the same spell box, below a horizontal divider,
with no repeated icon or badges and only the row’s right-hand Add/Remove button. On phones, the same complete spell details open in a
full-screen view with Add and Back controls; Back returns without changing the
answer. Adding returns to the list with its filters intact. The detail view and
the standalone spell page share a Markdown renderer, including emphasis, lists,
tables and higher-level rules. Catalogue HTML is not executed.

Every description the client shows goes through that one renderer: spell
details, and the detail under a picked option in the builder (a background, a
feat, an item). `desc` and `blocks.*` arrive as an array with one line-level
block per element -- a table is one row per element -- so they are never joined
by hand: `joinProse` (`web/src/ui/prose.ts`) puts a blank line between blocks
and a single newline between adjacent rows or list items, which is the
difference between a table and a column of pipes. `name` and `fields.*` are
plain text and are rendered as written. The contract is in
`docs/packs.md#prose-formats`. An option's detail sits below its button, not
inside it, because a table is not something a button may contain.

The Cantrips and Spells tabs each show one combined selected list immediately,
without collapsed question boxes or a per-level choice selector. Only the
selected list is grouped by spell level. Available spells form one flat,
list sorted by ascending spell level, then localized name, below the filters. The pool is the union of the
server's eligible options, so class lists, subclass grants, racial cantrips and
current class-level limits constrain normal choices. Turning off character availability
enables explicitly marked custom choices. Stored legacy forget answers are
not learning slots and never increase the displayed capacity.

New picks share one draft and one total, including saved selections: a character
with two saved cantrips and one newly granted slot sees `2 / 3`. A matching
algorithm assigns new picks to the underlying source allowances, reassigning
flexible picks when necessary so click order cannot strand a racial or feature
choice. Ordinary class learning uses the current-level pool for every allowance.
Add is disabled when no legal assignment exists. Next sends the
filled prompts in one revision-guarded append batch, preserving their server
identities and class levels. A level-up extends the current selection by the
allowance the rules grant; it does not reset previously saved spells.

Removing a saved selection loads its read-only edit prompt automatically. All
previous selections remain editable while other edits or acquisitions are in
progress. Each changed answer keeps its own retained picks, while new picks are
assigned to the remaining eligible slots. Nothing is written on Add or Remove.
Multiple replacements and new acquisitions are committed atomically through the
batch revision API; an invalid choice or stale revision leaves the entire saved
log unchanged. Dependent changes still use the dry-run confirmation. Leaving
the tab discards unconfirmed drafts; filters and localization do not.

Each spell box contains its preview control and its Add or Remove button as
separate, keyboard-accessible buttons inside one border. On phones, compact
plus/trash controls with accessible spell-specific names and 44-pixel touch
targets leave more room for the name and tags; the row uses reduced padding
and a smaller icon. The full-screen detail view retains labeled Add/Remove
and Back controls. A single Next button
sits immediately after the selected list beside the count, followed by filters
and available results. There is no second Next button at the bottom. With no
draft, Next navigates directly; with a valid draft it saves first, then advances
from Cantrips to Spells or from Spells to the next tab with work. Invalid or failed saves
keep the draft and the current tab in place.
Selected spells stay visible under every filter. Add remains inside each box,
and a full selection still allows previews. Confirmation is blocked while
catalogue data is loading or has failed. Filters never change the compendium's
URL filters. Learning uses
the server's exact count; preparation accepts a nonempty subset up to the
displayed capacity.
Leaving preparation open does not prevent finishing. The saved events retain the class level that grants each selection,
including intermediate levels of a character created above level one. Cantrip
purposes and legacy cantrip prompt IDs route to Cantrips; other spell choices
route to Spells. Both saved choices and outstanding prompts use that mapping. The sheet displays spell ownership per source, and lets a preparing class change its prepared list without the builder; see [the sheet's spells](../backend.md#the-sheet-arrives-resolved).

Editing saved spells reads the question at the original event's
position through `GET /characters/:id/prompts?before=:seq`, with the current
class level and subclass used for spell eligibility. The selected list is
seeded from that event's stored picks, including their order. Loading details or
changing language does not reset the draft. Opening, closing or abandoning this
editor does not delete or change the saved answer. Next uses the
replacement preview and revision guard; dependent answers are only
revalidated against the replacement when it is submitted.

Other saved choices use the existing revision/preview flow. Where a question
must be reopened by removing its answer, the dependent-answer preview describes
what would be lost. Changing level or abilities can require new preparation choices. There is no
automatic selection of spells, and the builder does not simulate resting,
spellbook-copying purchases or combat spell use.


### Current-level spell totals

Above the selected list, the wizard shows one compact explanation
per source, using the server's `spellRules`. Equivalent acquisition allowances
are added together, and spellbook/preparation totals for one class share the same
line. The class label uses its current level. Ordinary class choices identify
the owning class rather than inferring extra classes from spells shared by
multiple lists. Racial/feature choices, automatic grants and custom exceptions
keep their own sources.

The wizard uses current-level totals, with no replacement workflow or history
of when a spell was learned. A level-five sorcerer can select six known spells
from spell levels one through three, including at most two level-three spells,
then edit that list directly. The highest-level count is base acquisitions since
that spell level unlocked plus replacement opportunities in that interval;
replacement opportunities never increase the total. Lower spell levels have no
separate quotas. Per-level answer IDs remain internal storage addresses; all ordinary
learning allowances draw from the class's current spell-level pool. Increasing
the class level adds the newly granted count. Historical spell answers can be
edited against today's class-level pool, without rewriting the level history.

There are no forget selectors, replacement counters, or “Save and review
replacements” step. Old saved swap events still project for compatibility,
and previously forgotten spells stay out of the selected list. The simplified
selection policy intentionally does not simulate the stricter 2014 sequence of
spell acquisition and one replacement per class level. Casting slots remain
separate resources, not per-spell-level quotas for spells known.

Turning off character availability continues to allow explicit custom choices
beyond these normal totals. They retain their custom source and a compact “Custom choice” badge, including
after saving and reopening. The allowance panel includes the selected/total
counter (including custom selections) at bottom left and gray minus/plus
“Change spell limit” controls at bottom right. The extra allowance cannot go
below zero; decreases, including decreases to previously saved extras, are saved
on Next together with increases and spell edits. Adjustments persist as a separate extra
allowance; they do not change class totals or highest-level caps. With character
availability enabled, extras can be picked from the class pool and are explicitly
custom. Availability off still permits unrestricted custom exceptions, even if
the counter exceeds its normal-plus-extra denominator.

Selected cards use blue highlighting. Both lists use compact 32px icons, reduced
padding and one row action; mobile keeps a 44px action target. Available results
start with 20 rows; “Load more” appends the next 20, matching the spells page.
Changing filters resets the loaded range, while adding a spell or taking a
draft pick back keeps the expanded list; removing a *saved* spell asks again,
because the server is what leaves saved picks out. **The list is searched,
sorted, counted and paged by the server**: the panel posts the offer -- the
options its open prompts list, plus the levels and class lists an extra
allowance lets in, minus what is saved -- to `…/catalog/spells/search` and is
sent twenty spells at a time. A draft pick is hidden locally rather than
excluded by the server, so that adding a spell is not a request; the count
subtracts it. The description is fetched for the one row that is open. The selected cantrip list has one “Cantrips”
caption without a redundant level-zero heading. Selected leveled spells retain
their level headings; available results remain sorted by level and localized name.
Each allowance source, including additional allowance, has a caption with gray
details on a separate line.
