# AI Wizard

The AI Wizard creates **one character** from a description or uploaded sources.
The character is real from the first message: it is created with the session,
it is in the owner's character list while the assistant is still working, and
every tool call is committed to it. There is no draft and no Save. The session
holds the attachments, the conversation, tool results and the id of that
character. A model never receives an owner, character or session ID parameter
with which to target another record.

It used to be the opposite -- a private draft served through a parallel copy
of the character API, published by an explicit Save -- and everything awkward
about the wizard came from there: View and Edit were pages of the chat rather
than of a character, the editor needed a second repository to pretend a draft
was one, and what the import wrote did not have to look like what the builder
writes because nothing but the wizard's own panels ever read it.

## User flow

The desktop and mobile menu has **AI Wizard / AI Помощник**. Folder actions link
to the same workspace while preserving the destination folder. The server
needs `agent.api_key` and `agent.model` configured to process uploads. Without
the provider, imports report that AI import is not configured; the workspace
does not fall back to the legacy HexSheet JSON screen.

**The assistant leads and the player answers.** The conversation opens with
two questions asked before any session exists:

1. *Which rules?* -- one prepared answer per rule pack, like any other
   question's; pressing one is the player's reply. What the pack depends on
   comes with it. The lock travels with the first message and is final from
   then on; the HTTP API still requires it. It used to be the builder's pack
   form dropped into a bubble -- toggles, Confirm, Clear -- which was a form
   in a conversation.
2. *A sheet to attach, or a description?* -- asked, and answered by writing:
   the text field opens with this question. Attaching is a quiet control
   inside the field, bottom left, which the attached file replaces. There are
   no "attach" and "describe" buttons; describing is just typing.

Both stay in the log: the session's first event is `rules`, carrying the
confirmed packs, and the client draws it as that same exchange. It used to be
a card above the chat that disappeared the moment the session began.

**Every turn of the assistant's ends in buttons.** It says what it did, then
offers prepared answers (`ask_user`, up to ten -- all of a choice's options
when they fit, never one or two when more exist) or finishes
(`prepare_review`); it is instructed never to end on a plain message that
waits for a reply. A turn that ends that way all the same is sent back by the
server, once per message of the owner's, with a note to keep working or ask
properly. The app invents no answer of its own -- it used to put a
**Continue** button under such a message, which is a choice with nothing to
choose. The model is likewise told not to report a leftover and stop, nor to
ask anything whose only answer is "go on": it resolves what it can, and asks
about a concrete problem with its concrete ways out, "Skip it" among them.
**The text field is always the last thing in
the conversation**, under the latest message, and is open for writing only
when the assistant has asked something: disabled while it works and before
the opening's questions are answered. (For one round it was hidden behind a
"Tell what to do…" answer; a field that is always there and says by its state
whose turn it is turned out simpler.) A paused or failed run adds a Resume or
Retry button.

**Nothing in the log folds away.** Answers already given stay under the
question that offered them, no longer pressable. Every write the assistant
makes is a message of its own: "Imported field: Name" in bold, the value on
the line beneath. The assistant's messages are one column of a fixed width.

**View the sheet, Edit, Finish and Delete.** They sit in the page header, top
right, whenever there is a character, and the same four, drawn the same, close
every turn of the assistant's that did not end in a question of its own --
finished, stopped short or failed alike -- so no conversation ends on nothing
to press. Finish is offered whenever the assistant has stopped, not only when
it calls the character ready. Delete deletes the character
with the chat. **Finish is what closes a chat**: it marks the session
`finished` (control action `finish`, accepted at any revision -- it was first
checked like the other controls and silently refused from a page one revision
behind) and goes to the sheet. A finished chat opened from the sheet's
history link is a record: no text field, no answers, no Finish or Delete. The character is
already real, so nothing is saved by it.

**The wizard reopens the chat that was left unfinished.** `/ai-wizard` with no
session lists the owner's sessions and shows the latest (`created`) that is
not `finished`, in place; a new conversation starts only when there is none. Unfinished
means Finish was not pressed -- a character the assistant has declared ready
is still an open chat, which is the difference from `review`. The sheet's
header links to its chat as **AI Wizard history**, beside Level up and Edit.

**The assistant cannot finish over unanswered questions without asking.**
`prepare_review` is refused once while any prompt is still open -- optional
ones included: alignment, personality traits, ideals, bonds, flaws -- and
returns them with the instruction to offer the owner *fill them in for me*,
*one by one*, or *leave them blank*. It stays refused until an `ask_user` has
actually followed -- calling again is not consent, and neither is
`allow_incomplete`, which is the model's word that the owner chose and was
taken at that word until a model set it on its own. The builder's extra-spell
questions (`custom/spell/*`) are not counted: they are always open.
This is enforced by the tool rather than left to the
prompt because the prompt alone was not followed -- and could not have been:
the written questions were filtered out of the open prompts the model is
shown, so it never knew they were unanswered. They are listed now, each with
the `identity.*` path that answers it.

**A sheet is attached to the first message and to no other.** Attaching is one
of the two answers to the opening's second question; later messages are text.
(`POST /v1/agent-sessions/:id/files` remains on the server and has no caller
in the client.)

What the build leaves open is settled the same way, by the assistant asking.
Once everything the sheet or the description determines is in the build, it
asks once how to deal with the rest -- *pick them for me*, *one by one*, or
*leave them open* -- and, one by one, asks each open choice in turn with its
options as suggested replies. A request such as "a level 6 character who
fights with their hands" is built as far as it determines and then met with
that same question, rather than decided in silence. This is instruction in the
system prompt over the existing `ask_user` and `answer_choices` tools, not new
machinery.

Upload PDF, PNG, JPEG, WebP, JSON or UTF-8 text and optionally describe what needs
attention. Multiple files belong to the same character and are sent together
with the first message. Filenames identify sources; new attachments must not
reuse a filename. The server checks the content type rather than trusting the
browser's filename or MIME header.

Chat is one chronological log of **messages**: the assistant's on the left,
the owner's on the right, each in its own bubble. Factual progress comes from
validated writes -- a class imported, a name set -- and a run of it is one
assistant bubble listing each field and its value, as described above.
Catalogue searches and internal tool labels are hidden. Each user message
contains its own attachments and text; filenames are not a separate global row.
Source/page evidence and assumptions remain internal metadata, not transcript
labels or a preview warning panel. The compact composer sits at the bottom;
Enter sends and Shift+Enter inserts a line break. Scrolling up stops following.

The assistant replies in **the language of the owner's latest message**, and
writes its questions and suggested answers in it too. The interface locale is
only the fallback, for a first message that is an attachment and nothing else.

The route is `/ai-wizard/:sessionId`. Old `/characters/import` routes redirect,
retaining the session and query, and legacy `?session=...` links still open.
**View and Edit are links to the character**: `/characters/:id` and
`/characters/:id/build`, the ordinary sheet and the ordinary builder. They sit
in the chat header from the first message and inside assistant messages where
relevant. The sheet links back to the chat. An edit made in the builder is an
ordinary edit of an ordinary character; the assistant notices it on its next
tool call (see [Session lifetime](#session-lifetime)).

Unresolved choices belong in chat. `ask_user` records a question and up to six
suggested text answers. The latest unanswered question shows clickable replies;
the composer always permits a different free-text answer. A reply is an ordinary
user message, and the assistant applies it using validated choice tools. The
model must ask a direct question instead of narrating that it is paused.

`prepare_review` ends the import. It refuses while a required choice is
open, returning the open choices with their options, and accepts a partial
draft only when the model explicitly sets `allow_incomplete` after the user has
chosen to leave choices incomplete. This flag is a model assertion of that
conversation, not a separate server-verified consent record. It also holds a
**checklist** the model wrote in `plan_import` against the draft -- the
entries the sheet puts on the character, down to class features and racial
traits, each covered by the character having it, however it got there -- and that part
is a reminder rather than a lock: entries not in the draft are listed once, and
the same list a second time passes. It used to block until every entry was
satisfied, and a model that had worded one in a way nothing could satisfy
responded by inventing custom content until the list went quiet.

There is no interruption control in the UI, and no composer while the
assistant is running. Terminal status snapshots bring the composer back and
focus it. A three-second snapshot
refresh recovers a missed end-of-turn stream update without disabling input.
Resume continues a paused conversation; Retry continues after a failure.
The internal stop API remains for cancellation and lifecycle handling.
Waiting for a reply holds no worker. **Discard deletes the character with the
chat** -- it is what discarding the draft used to mean -- while leaving the
chat any other way leaves the character where it is. A character whose
remaining choices are open is finished in the ordinary builder.

## Session lifetime

State is **in memory in one server process**, including attachments and private
packs. Browser reload or SSE reconnection recovers the session from the URL's
session ID (or legacy `session` query parameter). The import screen also lists unfinished sessions for the
selected folder. Server restart loses this state, as it currently loses ordinary
characters. PostgreSQL account storage does not change that guarantee.

The coordinator in `internal/usecase/character/agent.go` owns the session map and
its mutation lock. It deliberately has no database or durable job framework in
this release. A fixed number of goroutines run independent sessions, with one
active model request per session. Model I/O runs outside the lock; tool
mutations run inside it. Waiting, review and paused states do not consume
workers.

**Reads do not take the coordinator's lock.** A response's tool calls hold it
for seconds, and `Get`/`List` -- the page opening, its three-second poll,
every SSE tick -- used to queue behind them, which is what made the app appear
to hang when the wizard was opened mid-import. Readers are served from
`views`, a published copy of each session under its own read-write lock,
republished after each tool call, streamed chunk and control.

The session's log is the agent's **working copy** of the character, not the
character. Around every tool call the coordinator reads the stored character
and, if the call changed anything, commits it back at the revision it read
(`pull`, `call`, `push` in `agent.go`). A revision that moved in between means
the owner edited in the builder. The edit wins: the working copy is replaced,
the call is answered "not run: the player edited the character", and the model
re-reads the build context before going on -- only `get_build_context` itself
is let through. The builder is therefore never locked while the assistant
runs. A character the owner deleted fails the session.

Every stop, user message, file addition or direct edit advances a generation.
A late response from an older generation cannot publish text or change the
draft. Revision checks reject edits based on an older snapshot. Complete model
responses enter the server transcript before tool execution. Partial function
arguments are never executed. Tool-call IDs have an argument hash and recorded
outcome; replay returns that outcome and reuse with different arguments fails.

SSE records have increasing event IDs. Reconnection sends a coherent snapshot
with its cursor, then newer events. In development, EventSource carries the tab cookie selector as `devSession`
in its URL because native streams cannot set the development session header.
It is not a credential; authentication still validates the selected HttpOnly
cookie. Production ignores the selector. The browser deduplicates event IDs and
rejects older snapshots. A heartbeat keeps the proxy connection open. Closing
the browser only closes the stream, not the import.

## Tools and rules

The OpenAI adapter supplies function schemas; the character usecase implements
the tools. The vendor boundary is the small `AgentModel` port, not an agent SDK
woven into the rules engine. The initial adapter uses the official Go Responses
SDK, native PDF/image input, streamed output, `store: false`, and encrypted
reasoning state when supplied by the provider. Provider state stays server-side.
Files are included again on each request; there is no OCR/extraction cache yet.

| Tool | Responsibility |
| --- | --- |
| `get_build_context` | The draft by fact path, open prompts **with their options**, the answers given so far, custom entries, differences from the sheet's printed numbers, checklist entries not yet covered |
| `read_source` | Text/JSON source contents, or reference to an attached image/PDF |
| `plan_import` | Transcribe the sheet in one typed call: final ability totals, level, hit points, armor class, speed, every skill and save bonus, coins, inventory and spells, plus a checklist of what else it documents |
| `import_facts` | Race, subrace, class with its level, subclass, background and feats **by printed name**; printed values at a path. Per-fact errors with candidates |
| `assign_skills` | Distribute the sheet's proficient skills over the prompts that grant skills, and its expertise over the expertise prompts |
| `assign_spells` | Distribute the sheet's cantrips and spells over the build's spell prompts, and keep the ones past the build's count as spells known |
| `answer_choices` | Answer open prompts in one batch, by option key or printed name, through the existing character validator. Rejections name the pick and the rule |
| `revise_choice` | Replace a prior choice and report invalidated dependent entries |
| `list_choice_options` | Page through a prompt with more than 60 options |
| `set_inventory` | Items by printed name, count and placement; unmatched names come back with candidates |
| `search_catalog` | Ranked identities across supported locales within the pinned rules lock |
| `get_option_details` | Exact catalogue mechanics and, when available, a pack wire example |
| `upsert_custom_option` | Keep content the rules lack as an editable typed definition. Refuses to copy what the pack or the build already has |
| `ask_user` | Ask a blocking question and release the worker |
| `prepare_review` | End the import: check the name and required choices, drop overrides the build reproduces |

A response may carry several tool calls; they run in order, up to 32. Once a
call ends the turn (`ask_user`, a successful `prepare_review`) the rest of that
response is answered "not run" rather than executed. A rejected call returns
the message together with whatever makes the next attempt different: the
`fields` a validator named -- which pick, which rule -- and the `candidates` a
name could have meant. At debug level every call is logged with its arguments
and result (`AI wizard tool call`), which is the only record of what a model
actually asked for; the arguments are a player's character sheet, so it is
debug-only.

Session creation -- which is also character creation -- is an application
operation, not a model tool. Source
text, catalogue descriptions and files are explicitly treated as untrusted data
in the model instructions. There are no shell, network-browsing, unrelated
character-editing or publishing tools.

### Names, not slugs

Every tool that takes a catalogue entry takes it the way a sheet prints it, in
any locale the pinned packs ship, and one function (`resolve`) turns that into
a canonical ref or into an error listing the entries it could have meant. A
model never has to produce a slug, which matters more than it sounds: a pack
installed under its own id namespaces every slug (`dnd-2014/stealth`), so the
`skills.stealth.bonus` and `equipment.backpack.dagger` a model naturally
writes address nothing. Paths are resolved the same way, and everything the
model reads back is in local names.

Matching is identity, not spelling distance alone. Names are compared as sets
of singular words, so "Rope, Hempen" is "Hempen Rope", "Arrows" is "Arrow" and
"Rations" finds "Rations (1 day)". A sheet that says more than the catalogue --
"Path of the Totem Warrior" for "Totem Warrior" -- matches only inside a
closed scope: a subclass among its class's, a subrace among its race's, a pick
among its prompt's options. That scope is also a correctness requirement, not
a convenience: "Wild Magic" is exactly the barbarian's path and only loosely
the sorcerer's origin, and unscoped it picks the wrong one. Outside a scope
the looser match would turn "Fire Shield" into "Shield". Mass/Greater/Lesser
variants are never normalized into their ordinary counterparts, and Fire Bolt
and Fireball stay distinct.

An exact or reordered name wins outright; between two spellings of one name
the entry that carries mechanics wins (the development pack has some items
twice). Anything weaker has to be the only plausible reading, or the tool
answers with candidates instead of guessing.

An identity match does **not** authorize choosing a spell. Open prompts and
the normal validator enforce its source, class, purpose, counts and held
options.

### Rebuilding the sheet

The import rebuilds the character the sheet describes: catalogue entries, then
the build's own choices, answered through the validator a player's answers go
through. See [dnd.md](dnd.md#imported-builds-and-final-ability-totals) for why
it no longer records a sheet as overrides.

**The transcription comes first and is typed.** `plan_import` has a slot for
each thing a sheet prints -- six scores, level, hit points, armor class,
speed, a bonus per skill and per save, coins, items. A number asked for one
fact at a time is a number a model skips; a form with an empty field is
harder to leave half done. Level, coins and items are written to the
character. The six scores are held by the session and *solved into* it (see
below). The derived numbers are kept as a reference and never written.

**The server does the arithmetic and the matching.** From the printed bonuses
it reads which skills and saves are proficient (a bonus is the modifier plus
the proficiency bonus, once or twice) and returns the list. `assign_skills`
then distributes those skills over the prompts that grant them. That is a
matching problem -- Sleight of Hand is not on the sorcerer's list, so it must
be the half-elf's, which leaves Insight for the sorcerer -- and a model given
it picks whatever fills the slot. `assign_spells` is the same solver over the
spell prompts: an Arcane Trickster's Mage Hand is already granted, its other
two cantrips fill its own prompt, and the high elf's cantrip is then a
question the sheet does not answer. Prompts that must be answered are filled
first and larger ones before smaller, so the one left open is the one worth
asking the owner about; spells the sheet lists past what the prompts take are
kept in the builder's extra-spell answers. All of this used to be left to the
model, which tried Mage Hand in each prompt in turn.

Both tools replace the picks of their kind made so far, and remove them
outright rather than through `Revise`: nothing but another pick of the same
kind depends on one, and `Revise` would re-judge every later answer.

**Printed numbers are a reference, not an instruction.** Every write reports
`differences`: where the draft computes something other than the sheet
prints. They tell the model a skill is unassigned or the level is wrong, and
they go in its summary. Nothing is pinned to make them agree, because a number
read off a scan is the least reliable thing in an import -- a model that reads
the armor class box as hit points will also confirm it when asked. A number is
written over the build only on purpose (`import_facts` with a path, when the
user asks to keep one), and is dropped again at review and at save if the
build computes the same value.

**Prompts are judged against the build alone.** A value written over the
build would otherwise make every skill it mentions "already held" and refuse
the pick that explains it, so prompts and answers are computed on the draft
with such values lifted off.

### The builder's own entries

Whatever the builder has a question for, the import answers the way the
builder's own form does, so that an imported character opens in the builder as
the cards a hand-built one has and nowhere as a list of "imported values":

- **Name, then rules.** The character opens with the two entries a new
  character gets: `init` with the name, and a change setting
  `identity.ruleset`. They used to be one entry, which the builder files under
  Rules, where the ruleset is final -- so the name sat on the Rules tab and
  could not be changed.
- **Level.** `identity.desiredLevel`, as an ordinary change. The class entry
  is at level 1 and the subclass entry at the level its class chooses one,
  because an entry's level is the level its decision belongs to and that is
  how the Class tab is grouped. The level printed beside the class is the
  level the character is built towards -- a question of its own. Only a
  second class keeps levels on the class entries, since one desired level
  cannot say how they are shared out.
- **Ability scores.** One entry: `abilities.method = manual` and the six base
  scores, which is what the ability-scores form writes and reads back. The
  sheet prints *totals*, and the bonuses that turn a base into a total arrive
  in any order, so the session keeps the totals and after every tool call
  solves the bases that reach them, rewriting that one entry in place
  (`settleScores`). An edit by the owner ends that: their numbers win. Only a
  build no base score can reach -- a custom rule that is not additive -- keeps
  a `finalAbilities` total pinned over it.
- **Personality traits, ideals, bonds, flaws, alignment.** Ordinary changes in
  the Personality group, one per path, rewritten in place when stated again.

What stays an `Observed` entry is what the builder has no question for: the
structural entries (race, class, subclass, background, feat), which a sheet
states outright, and the inventory, coins, languages and tool proficiencies.
The Equipment tab shows the inventory and coins through its ordinary summary;
they are changed through the chat. A structural entry that is
named again replaces its observation, and one that changes -- a different
race -- is revised, so answers given to the old one do not replay as picks of
the new one.

The character is created with the session through the repository's atomic
`CreateWithLog`, so there is never an empty character behind a chat. The name
and the pruning of overrides the build reproduces are checked at
`prepare_review`.

### What is not written

A blank is not an answer: `import_facts` refuses an empty value (or the "…"
placeholder) at an `identity.*` path, because written it closes the question
with nothing in it. A character with no name is not an error to work around
either -- `prepare_review` answers that the name must be read off the sheet or
asked for, with suggestions. And `upsert_custom_option` keeps only what a
character is built from that the rules lack -- class, subclass, race, subrace,
background, cantrip, spell, item. A feature, trait, feat or note the catalogue
does not know is refused: those were what a model wrote to quiet the checklist
("Versatile", "Languages") or to record that a field was left blank, and the
owner found them as junk on the sheet. For the same reason the checklist no
longer lists features and traits as missing; nothing imports one by name.

### A stated feat, and the sheet's own words

A feat is the one structural entry that also answers a question. A sheet names
it outright, so it is written as a fact before the improvement that grants a
feat is open; when that question is then answered with the same feat, the fact
gives way to the answer. It used to be refused as already held, and the model
picked some other feat to fill the slot. Inventory placement is read in the
sheet's words too: wielded, held and worn are equipped, carried is the pack.

### Custom content

Custom content is for what the selected rules lack. `upsert_custom_option`
first looks for the entry among what the build already holds, then in the
pack, and makes nothing custom of what it finds: a feature the class grants is
left alone, a subclass or background the pack has is imported as itself, a
catalogue item goes into the inventory, and a catalogue spell beyond the
class's count goes into the builder's own extra-spell answers
(`custom/spell/known`, `custom/spell/cantrip`). A sheet prints everything a
character has, and each of those arriving as a custom entry is how an imported
rogue ended up with a custom Sneak Attack beside the catalogue's. A `ref` or
`parent` that does not resolve is an error with candidates, not something
dropped in silence.

Custom classes, races, subraces, backgrounds, subclasses, spells, cantrips,
equipment, feats, features and traits are stable definitions in the character
log. Names, descriptions, provenance and explicitly known numeric details are
editable in the ordinary builder, for wizard-made and hand-made characters alike. Unknown
mechanics remain unknown; an unknown class hit die does not generate HP or dice.

Definitions overlay only that character's catalogue. Canonical references can
retain spell identity even when a source exceeds normal eligibility or counts.
Selected custom spells retain their source, ability and known/prepared/granted/spellbook
status; equipment retains its quantity and location. Definitions survive save,
copy, reopen and later edits. Imported observations keep their editor group and
are not discarded merely because they do not reconstruct historical choices.
Changing a parent reports invalid child selections through normal revision
handling. Older import metadata gains editor groups on read; old manual notes
remain available as editable custom definitions without automatically selecting
them. Direct name edits merge the opening fields, preserving bundled legacy
facts.

**A custom entry is an answer to a question the builder already asks**, never
a control of its own. The builder offers six: a custom **race**, **class** and
**background** as the last option of those pickers, a custom **item** as the
last option of an equipment choice, and a custom **cantrip** or **spell** as
the last entry of the spell tabs' list. Each opens the entry's form with its
kind already said. No tab has a "Custom…" button under it -- the Personal tab
used to end with one that made a note. Entries written earlier are offered in
their list beside the catalogue's, marked Custom.

A custom entry on a character is a block on its tab like any other decision
-- what it is, its name, a Custom mark -- above the tab's Next, and pressing
it opens the same form, where it can be deselected to return to a catalogue
choice. That includes kinds the builder does not offer to create (a subclass,
a feature, a note): the assistant may still record one when a sheet has
something the rules lack, and it has to be visible and editable where it lands.

The backend retains compatibility with complete private pack definitions through
the existing strict compiler. The standard model tool schema favors typed
definitions instead of asking the model to manufacture a pack. Compilation
and projection must succeed before a complete definition changes the lock.

Private releases never enter the default catalogue. Older versions remain
available for older locks; copied/shared characters retain the attached lock
and receive localized private names/descriptions in their projection. Migration
cannot attach another character's private release. The registry is still
process-local; this is not a general pack editor, upload library or publisher.

## HTTP surface

All routes require the same authenticated session as ordinary character routes.
Owners are taken from authentication, never from a request body.

| Method and route | Input / result |
| --- | --- |
| `GET /v1/agent-capabilities` | Whether the provider is configured |
| `POST /v1/agent-sessions?folder=...` | Multipart required `rules` JSON, optional `files` and `instructions`; at least a description or file is required |
| `GET /v1/agent-sessions` | Owner's import sessions |
| `GET /v1/agent-sessions/:id` | Session snapshot, including `characterId` |
| `GET /v1/agent-sessions/:id/events` | SSE snapshots, updates and cursor recovery |
| `POST /v1/agent-sessions/:id/files` | Multipart files, revision and optional instructions |
| `POST /v1/agent-sessions/:id/control` | Revision plus `message`, `stop`, `resume`, `retry` or `discard` action; `discard` deletes the character too |


Normal characters expose GET/POST `/v1/characters/:id/custom-options`.
GET returns `{revision, options}`; POST accepts `{revision, option}` and returns
the updated sheet and revision. Writes check ownership and optimistic revision.
Definitions cannot be erased by generic note replacement.

The character itself is read and edited through the ordinary
`/v1/characters/:id` routes. There is no draft surface.

The existing `POST /v1/characters/import` contract is unchanged.

## Configuration and deployment

Add this to the YAML file selected by `EASYDND_CONFIG`. Keep the actual API key
out of version control. No provider model is silently selected for operators.

```yaml
agent:
  api_key: "YOUR_API_KEY"
  model: "YOUR_RESPONSES_MODEL_WITH_PDF_IMAGE_AND_TOOL_SUPPORT"
  workers: 4
  max_turns: 40
  max_sessions: 100
  request_timeout: 2m
```

The key's absence disables AI import. Limits default to the values above;
workers are bounded at 32, turns at 200, sessions at 1000 and request timeout at
10 minutes. The session count includes finished conversations, so capacity
must be sized for this memory-first deployment.

Additional fixed bounds are 8 files / 20 MiB per session, 256 KiB per text/JSON
file, 16,000 UTF-8 bytes per message, 128 KiB per tool argument payload, 12,000
output tokens per response and two SDK retries. Conversation growth is bounded
by item/event counts and a 2 MiB transcript threshold. Turns pause at the run
limit; manual Resume starts another bounded run. These are request/run limits,
not a billed-token accounting system.

The checked-in nginx configuration raises `/v1/`'s body limit to 21 MiB. **Deploying
a release does not install nginx configuration**: apply that file separately.
SSE uses `X-Accel-Buffering: no`, heartbeats and a request-specific write deadline
override, leaving normal HTTP timeouts in place.

## Validation and later milestones

Tests cover translated/fuzzy spell identity, printed-name resolution and its
scopes, a scripted end-to-end import that ends with no custom entry and nothing
pinned (on the SRD and on a namespaced pack), skill distribution, the
one-time checklist reminder, calls after a turn has ended, worker bounds,
cancellation, owner isolation, HTTP uploads/SSE recovery, score preservation
and rebasing, the builder's own entries on the stored character (name apart
from rules, class at 1 and subclass at its level, one manual ability-scores
entry, nothing printed pinned over an identity field), discard deleting the
character, private definition isolation/versioning, the Responses streaming
adapter, and desktop/mobile chat, View and Edit opening the real character,
rules preselected inside the chat, explicit questions and suggested answers,
required-choice review guards and missed terminal status recovery.
The provider adapter is tested against a local HTTP fixture; a live provider
smoke test requires deployment credentials and a configured model.

Durable restart recovery is deliberately later. It must persist attachments,
transcript/provider state, operation outcomes, revision/generation, draft log,
private releases and final character linkage together, with transactional
claims/leases and a unique finalization key. Persisting only messages cannot
resume this runtime correctly. Text-only character creation uses the same bounded tools. A future MCP adapter
can reuse them.


## Opt-in PDF regression

The real-provider regression is deliberately outside default tests and CI.
Run it on request or after deep changes to the AI Wizard. It needs an already
configured development API, the source PDF and the expected source snapshot:

```sh
node scripts/check-ai-wizard.mjs --pdf /tmp/Arya.pdf \
  --expected testdata/agent/arya.expected.json \
  --api http://localhost:18083/v1 --origin http://localhost:8083
```

It sends one neutral instruction, answers a question with its first suggested
option, and then checks two things. That the draft **is the sheet**: name,
scores, class and level, hit points, armor class, spells, inventory counts,
coins, skill and save bonuses. And that it is **a build**: `subclass` and
`background` are catalogue entries, no custom option exists outside
`allowedCustom`, no printed value is pinned over the build outside
`allowedOverrides`, and no required choice is open outside `openPrompts`. It
then reopens the character, edits the name and, where one exists, a custom
definition, and verifies that facts survive. Test edits are restored; the
imported character remains.

The three expectation files are written for the development pack
(`dnd-2014`), which has the subclasses and backgrounds the sheets use; against
the bare SRD they fail by design, because there those are not in the rules.
`itemAlternatives` names the second slug of items that pack carries twice. The
run uses the configured provider and can incur provider charges. No provider
credentials or session cookies are stored by the script. The PDFs themselves
are not committed.

What the regression cannot make deterministic is the model's reading. The
tooling removes the errors a server can catch -- unknown names, illegal
picks, a wrong skill split, a misread number pinned over a correct build --
but a line the model skips in an equipment list is simply not imported, and a
small model does skip one now and then. When a run fails that way and the tool
log shows no rejection, `agent.model` is the lever.
