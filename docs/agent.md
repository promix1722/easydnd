# AI Wizard

## Standalone agent CLI

`go run ./cmd/llm agent -pack /path/to/easydnd-2014/pack`
runs the same wizard locally, without a web server or database. It reads
`agent.model`, `agent.reasoning_effort`, `agent.max_turns` and
`agent.request_timeout` from `config.dev.yaml` (`-config` names another file)
and the key from `EASYDND_AGENT_API_KEY`, so export your env file first:
`set -a; . ~/config/easydnd/dev.env; set +a`. `-locale` defaults to `en`;
`-timeout` defaults to `5m` for each start, message or resumption. The supplied
pack is loaded as an immutable rules lock. Nothing is written to the pack.

Send one JSON object per line on stdin. Keep stdin open while the agent works:

```json
{"action":"start","files":["/home/me/tmp/dnd-sheets/character.pdf"],"text":"Import everything you can, do not ask me anything"}
{"action":"inspect"}
{"action":"message","revision":12,"text":"Leave them blank"}
{"action":"resume","revision":15}
{"action":"stop","revision":20}
{"action":"finish","revision":21}
```

Use the revision from the latest response or status, not these example numbers.
`"unattended": true` on `start` begins an [unattended session](#unattended-sessions).
`retry` and `discard` are also available; controls use the same revision rules
as the HTTP wizard. Only one session is active in a process. After `discard`,
a new session can start. EOF or Ctrl-C closes the process and cancels its work;
sessions are not persisted. Send `inspect` and save its response before exiting.
Attachments have the same supported MIME types and size limits as HTTP uploads;
paths identify local files, and duplicate basenames are rejected.

Stdout is JSON lines: `result` responses (with `action`, `session`, `sheet`,
`log`, `prompts`, `rules`), `event` entries, `status` updates, `model` timings,
token `usage` (`input`, `cached`, `output`) and complete returned tool calls, `error` responses, and `timeout` when a run
is stopped at its deadline. Stderr contains debug tool arguments and results.
The inspection's sheet, log, prompts and rules are the existing Go domain JSON
shape, with capitalized field names; session and event fields use their existing
lowercase JSON names. Timings cover the complete model request, including
provider streaming and decoding. An input error leaves the process available
for another command. A deadline pauses the session; it does not answer questions
or declare the character complete.

The AI Wizard creates **one character** from a description or uploaded sources.
The character is real from the first message: it is created when that message
is sent -- the chat itself is older, opened with the page -- it
is in the owner's character list while the assistant is still working, and
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
has no other importer to fall back to.

**The assistant leads and the player answers.** The page opens a session the
moment it is arrived at (`Open`, status `opening`), and the conversation
begins with two questions that are that session's events:

1. *Which rules?* -- the `opening` event, with one prepared answer per rule
   pack, like any other question's; pressing one is the player's reply (the
   `rules` control, recorded as a `rules` event). What the pack depends on
   comes with it. It can be answered again -- the last answer stands, and is
   the one shown -- until the first message, which makes the lock final. It
   used to be the builder's pack form dropped into a bubble -- toggles,
   Confirm, Clear -- which was a form in a conversation.

   It can also be answered **by writing**, like every later question. There
   is no model to read the reply yet -- the rules are what it will be given --
   so the page reads it (`namedPack`): the one pack whose title or id the text
   contains, or whose button the text is a piece of ("srd", "dnd 2014"). A
   message that says only that is the answer and nothing more. And it can go
   **unanswered**: a first message sent before any rules -- a sheet dropped
   into the box -- starts the chat under the deployment's own packs
   (`data.default_packs`), which `start` records as a `rules` event marked
   `assumed`, after the message, and the page draws as the assistant saying
   which rules it took rather than as something the player picked. The text
   field used to be disabled until a button was pressed, which made the first
   thing on the page a box that could not be typed into.
2. *A sheet to attach, or a description?* -- asked, and answered by writing:
   the text field opens with this question. Attaching is a quiet control
   inside the field, bottom left, which the attached file replaces. There are
   no "attach" and "describe" buttons; describing is just typing.

Both are the log: a session's first events are `opening` and `rules`, and the
page draws them through the one function that draws every later message.
They used to be drawn by the page before any session existed and drawn again
from the `rules` event once one did, and the swap showed: the opening
flickered as the first message was sent. Before that it was a card above the
chat that disappeared the moment the session began.

An opened chat has no character. It is the chat the wizard reopens, so
arriving twice does not make two; one that is walked away from is a row that
the daily sweep removes, and until then it counts towards `max_sessions`.

**Every turn of the assistant's ends in buttons.** It says what it did, then
offers prepared answers (`ask_user`, up to ten -- all of a choice's options
when they fit, never one or two when more exist) or finishes
(`prepare_review`); it is instructed never to end on a plain message that
waits for a reply, and the request carries `tool_choice: required`, so the
provider does not allow one either. A response with no tool call can then only
be one cut short, and it ends the run `paused`, with Resume -- never `waiting`,
which would show a text field under a question nobody asked. (It used to be
sent back once with a note and then left `waiting`; two imports in twenty-one
ended that way, with nothing to answer.) The app invents no answer of its own -- it used to put a
**Continue** button under such a message, which is a choice with nothing to
choose. The model is likewise told not to report a leftover and stop, nor to
ask anything whose only answer is "go on": it resolves what it can, and asks
about a concrete problem with its concrete ways out, "Skip it" among them.
**The text field is always the last thing in
the conversation**, under the latest message, and is always open for writing,
from the opening question on; what waits for the assistant to stop is
sending. A question that offers buttons is answered by pressing one or by
writing, whichever the player likes. (For one round the field was hidden
behind a "Tell what to do…" answer, and for another it was disabled until
the rules were chosen; a field that is always there turned out simpler.) A paused or failed run says so in a line of the
assistant's -- it stopped, nothing imported is lost -- with Resume or Retry
under it, ahead of the four buttons. It used to add the button and no words,
which read as a chat that had ended on nothing. Why a turn failed is in the
server log (`AI wizard model request failed`), not in the chat.

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
`prepare_review` is refused while any open prompt is one the owner has not
been asked about -- optional ones included: alignment, personality traits,
ideals, bonds, flaws -- and returns those with the instruction to offer the
owner *fill them in for me*, *one by one*, or *leave them blank*. It stays
refused until an `ask_user` has actually followed -- calling again is not
consent, and neither is `allow_incomplete`, which is the model's word that
the owner chose and was taken at that word until a model set it on its own.
The builder's extra-spell questions (`custom/spell/*`) are not counted: they
are always open. This is enforced by the tool rather than left to the prompt
because the prompt alone was not followed -- and could not have been: the
written questions were filtered out of the open prompts the model is shown,
so it never knew they were unanswered. They are listed now, each with the
`identity.*` path that answers it.

**Nothing is asked twice.** Every question is kept with its answer
(`AgentSession.questions`, stored with the session): its text and options,
the open prompts it was *about*, and what the owner replied. A question is
about the prompts it names (`ask_user`'s `about`), the ones whose own options
it offers as answers -- a question about a blank alignment offers alignments,
whatever it says -- and, when it follows a refused review, everything that
refusal listed. Three things read that record:

- **The open prompts themselves.** Every list of them a tool returns -- after
  each write, in `get_build_context`, in a refused review -- carries, on a
  prompt the owner was asked about, the question and their answer (`asked`),
  with the instruction to do as they said and not ask again.
  `get_build_context` also returns the whole record as `userAnswers`. That
  list is where a model decides whether to ask, and it used to say only that
  the prompt was still open.
- **The review.** What the owner was asked about is not held against the
  draft, whatever they answered and whenever they were asked. It used to
  count only if the question came *after* the refusal: an owner asked about a
  blank alignment who said "leave it blank" was asked again one message
  later, because the review that followed found the alignment unanswered and
  did not know they had just been asked. The model was then told to ask, and
  did.
- **`ask_user` itself.** The same answers, offered again over the same open
  prompts to an owner who has just pressed one of them and written nothing
  since, is the question they answered however it is reworded; it is refused
  with their answer. A reply in the owner's own words is not caught by this
  -- asking again after "what is the difference?" is not asking twice -- and
  neither is a different question about the same prompt: "one by one" is
  followed by exactly that.

**An answer to one of the written questions goes to its path, whichever tool
it came by.** A
model answers every open prompt with `answer_choices`, these included, and
for a while that wrote a pick under `character/alignment` -- an entry the
build accepted and never read. The question stayed open under its answer, the
model answered it again ("alignment is still showing as blank"), and the
builder drew one decided block per answer and an undecided one, all under one
key, none of which opened. `answer` now writes such an answer as the change it
is (`promptPaths`), and the build refuses the pick form outright -- see
[Choices](dnd.md#choices). Two things keep the working door from being a
way round the rule above. `answer_choices` refuses these prompts until the
owner has been asked something, because before that the only one deciding is
the model; and the prompt's `how` no longer says only how to write the value
but whose it is to give: what a source states is transcribed, what none
states waits for `ask_user`. Told only how, a model wrote an alignment "from
the description" for a box the sheet left blank.

### Unattended sessions

An owner who does not want to be asked says so when the chat begins:
`unattended=true` in the multipart request, `"unattended": true` on the CLI's
`start`. The web client no longer offers it -- its checkbox under the first
message was removed -- so this is the CLI's and the corpus runs' mode. The
session then imports what the
sources state and leaves the rest open:

- `ask_user` is refused, with the reason.
- `prepare_review` no longer refuses a blank name, an open required choice or
  an unanswered optional one. Its checklist reminder and the requirement that
  `plan_import` ran still apply. The review event carries the prompts left
  open in its `data.open`.
- The instructions gain a paragraph saying the same, so the model does not
  spend a turn finding out.

Nothing is picked or invented on the owner's behalf; the character is finished
in the builder. The flag belongs to the session and is set only when it is
created. It is deliberately not something the model can assert -- the
sentence "do not ask me anything" in a message does not set it -- because a
model's word that the owner chose to leave things open is exactly what
`allow_incomplete` was, and that was set by models on their own.

Before this existed the instruction could not be followed at all: with any
prompt open, review was unreachable without a question, and all twenty-one
sheets of the first corpus run stopped to ask one -- nine of them for a name.

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

The assistant replies in **the language the owner chose for the interface**,
and writes its questions and suggested answers in it too. Nothing else moves
it: not the language a sheet is written in, and not the language of a
message. It used to follow the owner's latest message with the interface
locale as the fallback for an attachment sent alone -- and for exactly that
case, a Russian sheet in an English interface, the chat turned Russian.

**Which language that is is settled by the first message** (`start`, from the
`?locale=` of the request that sends it), and not when the chat is opened. A
chat is opened by arriving at the page and is the chat reopened on every
later visit, so the language it was opened in can be days old: one opened in
English and begun after the owner switched to Russian was answered in English
to the end. From the first message on it does not change, whatever the
interface is switched to. The `message` control carries no locale, so a first
message sent by that door keeps the language of `Open`; the page sends its
first message through `/files`.

The transcript's own lines follow the same language. A `progress` event names
a catalogue entry -- a race, an item, an alignment, the languages in a list --
as the chat's locale does (`recordProgress`), not by the name the tool
answered the model with, which is the one that matched and so the sheet's
language: an English sheet in a Russian chat used to read "Race: Tiefling".

The route is `/ai-wizard/:sessionId`. Old `/characters/import` routes redirect,
retaining the session and query, and legacy `?session=...` links still open.
The page is **named by when the chat was opened and the head of its id** --
`2026-02-01-17:20:32-364665a0`, the reader's own time (`chatName`) -- where it
used to be named by the id alone, thirty-two hex digits that say nothing.
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

**Stop** takes Send's place while a turn is running or queued. It sends the
`stop` control, which ends the turn `paused` with the usual Resume: everything
imported so far is kept. A stop is honoured from a stale revision while a turn
is in flight, because a running turn moves the revision with every tool call
and the page is usually a step behind; one that arrives after the turn ended
does nothing. The message box stays open while
the assistant is running, but nothing can be sent from it: the reply
buttons wait for the turn to end. Terminal status snapshots enable them and
focus it. The page learns of them by [polling](polling.md), which
never disables input.
Resume continues a paused conversation; Retry continues after a failure.
Waiting for a reply holds no worker. **Discard deletes the character with the
chat** -- it is what discarding the draft used to mean -- while leaving the
chat any other way leaves the character where it is. A character whose
remaining choices are open is finished in the ordinary builder.

## Session lifetime

A session is **stored in PostgreSQL**, with its attachments: `agent_sessions`
(one row a chat), `agent_events` (what the page shows, appended to) and
`agent_files` (the sources). Any API process answers for any chat, a restart
keeps it, and browser reload recovers it from the URL's session ID (or legacy
`session` query parameter). A process without a database -- development with
no `db.url`, the CLI, the tests -- keeps the same sessions in memory behind
the same `Store` port.

**The character is stored beside it, not with it.** The character is an
ordinary row in `characters`, and the private packs an import writes are rows
in `private_releases`, so a chat, its character and the rules it compiled all
survive a restart together and any API process can pick the turn up. The
assistant still checks that the character is its owner's before writing to
it, and fails the session if the character has been deleted; the chat list
leaves such a chat out, so the wizard does not reopen one whose character is
gone.

**A chat nobody has used for a day is deleted**, whatever its status, by a
sweep every API process runs every ten minutes. "Used" is a write, a turn, or
the page being opened; an idle poll does not count. Only the chat goes: the
character stays where it is, and its *AI Wizard history* link then says the
chat is no longer kept.

### Who is doing what

A session's `status` moves only along one table (`transitions` in
`state.go`), and `setStatus` refuses anything else:

| From | To | By |
| --- | --- | --- |
| `opening` | `queued` | the owner's first message, once the rules are chosen |
| `queued` | `running` | a worker claiming the turn |
| `running` | `queued` | the turn going on to its next request; a process that is stopping, giving the turn back |
| `running` | `waiting`, `review`, `paused`, `failed` | the turn ending |
| `waiting`, `review`, `paused`, `failed` | `queued` | the owner: a message, Resume, Retry, more files |
| any but `opening` | `paused` | the owner: stop, Finish |

Who may make a move is decided by the store and not by a lock in a process:

- **A lease.** Claiming a turn writes `lease_owner` -- a random name the
  process gave itself at start -- and `lease_until`. The claim is one
  `UPDATE ... FOR UPDATE SKIP LOCKED`, so every worker of every process can
  ask at once and each is handed a different session or none. A `CHECK`
  holds the table to it: a row is `running` exactly when it has a lease.
- **A generation.** Every control from the owner advances it.
- **Every write a turn makes names both** (`WHERE lease_owner = me AND
  generation = mine`). A turn that was stopped, answered or taken over
  writes nothing: the store reports the refusal and the worker drops what it
  was doing. A streamed chunk is such a write, so a turn stopped from
  another process learns of it at its next chunk.

**Taking over.** A lease runs for the request timeout plus thirty seconds and
is not renewed -- a turn cannot outlive its own deadline. A process that
stops puts its turns back to `queued`, and the next process, or the same one
restarted, claims them at once. A process that dies leaves its turns
`running` under a lease that runs out, after which they are claimed like
queued ones; that wait, two and a half minutes by default, is the price of
having no heartbeat. Each process looks for such turns once a second.

**The stored transcript is always sendable.** The session is saved after the
model's response and after each tool call. A response's calls that have not
run yet are stored as answered "not run: the turn was interrupted", because
the provider rejects a transcript with an unanswered call -- so whoever
continues from a turn cut short, by a stop or by a dead process, continues
from something it can send. Tool-call ids already have a recorded outcome
(below), so a call seen twice is not run twice.

The wizard is its own use case package, `internal/usecase/agent` (imported as
`agentuc`), beside the character one it writes through. It is a second writer
of the same character rather than a client of the builder's operations, so it
takes from `charuc.Service` what a writer needs -- the repository, the
catalogue source, the pack authorisation port, the logger -- and from the
package the same validators the builder's own writes pass
(`ValidateAndAttribute`, `ValidateImported`, `Revise`, `UpsertCustom`). The
dependency runs one way: nothing in `usecase/character` imports the agent.

The coordinator in `internal/usecase/agent/agent.go` holds no session. A
fixed number of goroutines each claim a turn, work on their own copy of the
session -- the lease is what keeps every other writer out, so there is no
lock -- and save it. One model request is active per session. Waiting, review
and paused states do not consume workers.

**Reads wait for nothing.** `Get`, `List` and `Poll` -- the page opening and
every poll it sends after -- read the store. They used to queue behind a
mutex a response's tool calls held for seconds, which made the app appear to
hang when the wizard was opened mid-import, and then were served from a
published copy of each session; with the session in the store neither is
needed.

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

Events have increasing IDs, and the browser follows a session by polling
with the pair it holds -- the session's revision and its last event ID. It is
answered with the whole session when the revision has moved and with only the
newer events otherwise; it deduplicates event IDs and rejects older snapshots.
Closing the browser only ends the polling, not the import. The transport is
described in [polling.md](polling.md).

## Tools and rules

The OpenAI adapter supplies function schemas; the character usecase implements
the tools. The vendor boundary is the small `AgentModel` port, not an agent SDK
woven into the rules engine. The initial adapter uses the official Go Responses
SDK, native PDF/image input, streamed output, `store: false`, and encrypted
reasoning state when supplied by the provider. Provider state stays server-side.
Files are included again on each request; there is no OCR/extraction cache yet.

| Tool | Responsibility |
| --- | --- |
| `get_build_context` | The draft by fact path, open prompts **with their options** and what the owner has already said about each, the answers given so far, the owner's replies to every question (`userAnswers`), custom entries, differences from the sheet's printed numbers, checklist entries not yet covered |
| `read_source` | Text/JSON source contents, or reference to an attached image/PDF |
| `plan_import` | Transcribe the sheet in one typed call: name, alignment, personality traits, ideals, bonds and flaws, final ability totals, level, hit points, armor class, speed, every skill and save bonus, coins, inventory, the attacks table's names and spells, plus a checklist of what else it documents |
| `import_facts` | Race, subrace, class with its level, subclass, background and feats **by printed name**; printed values at a path. Per-fact errors with candidates |
| `assign_skills` | Distribute the sheet's proficient skills over the prompts that grant skills, and its expertise over the expertise prompts |
| `assign_spells` | Distribute the sheet's cantrips and spells over the build's spell prompts, and keep the ones past the build's count as spells known |
| `assign_abilities` | Answer the open Ability Score Improvements and chosen racial ability bonuses, which a sheet never itemises, without moving a printed total |
| `answer_choices` | Answer open prompts in one batch, by option key or printed name, through the existing character validator. Rejections name the pick and the rule |
| `revise_choice` | Replace a prior choice and report invalidated dependent entries |
| `list_choice_options` | Page through a prompt with more than 60 options |
| `set_inventory` | Items by printed name, count and placement; an equipment pack is carried as its contents; unmatched names come back with candidates |
| `search_catalog` | Ranked identities across supported locales within the pinned rules lock |
| `get_option_details` | Exact catalogue mechanics and, when available, a pack wire example |
| `upsert_custom_option` | Keep content the rules lack as an editable typed definition. Refuses to copy what the pack or the build already has |
| `ask_user` | Ask a blocking question and release the worker. `about` names the open prompts it puts to the owner; a question just answered is refused |
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
variants are never normalized into their ordinary counterparts, in English or
Russian (`Множественное`, `Высшая`), and Fire Bolt and Fireball stay distinct.
Russian `ё` and `е` are one letter.

`assign_spells` has every spell in the rules as its candidates, so it does not
use the looser match at all. A printed name that does not resolve is tried
once more without what sheets add to one -- a parenthesised note ("Detect Magic
(R)") and the owner's name the SRD drops ("Tasha's Hideous Laughter") -- and
is otherwise reported as unknown. "A catalogue name inside the printed one"
would have turned Cause Fear into Fear.

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
harder to leave half done. The name, alignment and the four personality
lists have slots for that reason: asked for afterwards as separate facts, a
field's second paragraph was what got left out. The name slot is nullable, and
a value that is only the box's caption ("Character Name") is refused, since a
blank box still prints one. Level, coins, items and those identity fields are
written to the character. **`attacks` is the name in each row of the attacks
table, and it is a slot of its own because a sentence asking for those weapons
among the items was not enough**: a sheet attacking with a longsword listed a
rapier in its equipment box and no sword, and the model copied the box. The
server resolves each name, keeps the ones that are catalogue weapons, and puts
them at the head of the inventory as equipped -- moving up the box's own line
for one it lists too -- so with one item to a slot the sword is in hand and
the rapier carried. A row that is no weapon (a breath weapon) is not an item. The six scores are held by the session and *solved into* it (see
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

Three things keep a correctly named spell from being lost on the way:

- **A prompt the sheet does not fill keeps its spells.** A prompt takes exactly
  its count, so four of a wizard's six first spells cannot answer it. It stays
  open, and the four are kept as known rather than dropped with it.
- **The top spell level is rationed before matching.** A class learns only so
  many spells of the highest level it can cast, counted over all its prompts
  together. A third-level wizard with three second-level spells (one scribed)
  has the third kept as known, not every prompt holding one refused.
- **A prepared caster's overflow is `unprepared`, not an extra spell.** A
  cleric prepares from the whole class list, so a listed spell past the
  preparation limit is already the character's to prepare another day. Which
  ones are prepared is what `plan_import`'s optional `prepared` list says the
  sheet marks; a sheet that marks none gets the first few by name.

`assign_abilities` is the third of the family, for the ability-score prompts
no sheet answers: an Ability Score Improvement, a half-elf's two +1s. A sheet
prints six totals and never how they were reached. Left open, such a prompt
keeps the whole total in the base score, and the owner who answers it later in
the builder gets the improvement twice -- a level 10 fighter's printed Strength
20 becomes 22. The tool answers them, taking each point from the highest base
score left (never under 8), and the totals do not move because `settleScores`
holds them. Where the points went is an inference, and the tool's result tells
the model to say so in its summary. Feats the sheet names are imported first,
so an improvement still open by then is the scores.

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
  (`settleScores`). An edit by the owner that changes an ability total ends
  that: their numbers win. Any other edit -- a rename -- does not, because
  forgetting the totals there put the next race or improvement on top of bases
  that already included it. Only a
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
kind already said. No tab has a "Custom…" button under it. Notes are the one
exception to all of this: they answer no question, so they have a tab of their
own, Custom, where the player adds, rewrites and deletes them
(docs/web.md#custom-is-what-the-player-writes-unasked). The assistant still
may not write one. Entries written earlier are offered in
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
cannot attach another character's private release. The releases are stored in
`private_releases` and cached per process; this is not a general pack editor,
upload library or publisher.

## HTTP surface

All routes require the same authenticated session as ordinary character routes.
Owners are taken from authentication, never from a request body.

| Method and route | Input / result |
| --- | --- |
| `GET /v1/agent-capabilities` | Whether the provider is configured |
| `POST /v1/agent-sessions?folder=...` | No form: opens a chat -- status `opening`, one `opening` event, no character. With a multipart form (required `rules` JSON, optional `files`, `instructions`, `unattended`): the older single request that also chooses the rules and sends the first message |
| `GET /v1/agent-sessions` | Owner's import sessions |
| `GET /v1/agent-sessions/:id` | Session snapshot, including `characterId`. With `?revision=R&after=N` it is a [poll](polling.md): answered at once with the session, the newer events, or `204` |
| `POST /v1/agent-sessions/:id/files` | Multipart `revision`, optional `files` and `instructions`. On an opened chat it is the first message -- text, files or both -- and creates the character; afterwards it adds source files |
| `POST /v1/agent-sessions/:id/control` | Revision plus `rules` (with a `rules` lock; an opened chat only), `message`, `stop`, `resume`, `retry`, `finish` or `discard` action; `discard` deletes the character too |


Normal characters expose GET/POST `/v1/characters/:id/custom-options`.
GET returns `{revision, options}`; POST accepts `{revision, option}` and returns
the updated sheet and revision. Writes check ownership and optimistic revision.
Definitions cannot be erased by generic note replacement.
`DELETE /v1/characters/:id/custom-options/:option?revision=` erases one, and
only a `note`: see docs/backend.md.

The character itself is read and edited through the ordinary
`/v1/characters/:id` routes. There is no draft surface.

The HexSheet JSON importer that used to sit beside this at `POST
/v1/characters/import` is gone: the AI Wizard is the only way a foreign sheet
comes in.

### An import wears one item to a slot

A sheet lists every weapon it has as wielded, and taking that at its word
seated a barbarian's net, longsword, four javelins and handaxe in one hand.
`set_inventory` now gives a slot to the **first** thing the sheet calls worn
and carries everything after it for that slot, and every further copy of it:
"Dagger x2, equipped" is one in hand and one in the pack. The same item on a
second page keeps the seat it has. It still equips, rather than carrying
everything, for one reason: the armor class a sheet prints is only something
the build can reproduce while the armor is on, and a number it cannot reproduce
is pinned as a fixed value that then ignores what the character later takes
off. Finishing the chat calls the character service's `AutoEquip`, which
dresses a character the import left with nothing on and otherwise does
nothing.

## Configuration and deployment

The settings are in the committed configs, `config.dev.yaml` and
`config.prod.yaml`. **The key is not**: it is `EASYDND_AGENT_API_KEY` in the
env file -- `~/config/easydnd/dev.env` in development, `/etc/easydnd/prod.env`
in production -- and a server without it starts with the AI Wizard off. See
[backend.md](backend.md#configuration). No provider model is silently selected
for operators.

```yaml
agent:
  model: "YOUR_RESPONSES_MODEL_WITH_PDF_IMAGE_AND_TOOL_SUPPORT"
  reasoning_effort: low
  workers: 4
  max_turns: 40
  max_sessions: 100
  request_timeout: 2m
```

`reasoning_effort` is passed to the provider as written and defaults to
`low`; `default` sends none, for a model that takes no such setting. An
import is transcription and matching, and the server does the arithmetic, so
more effort mostly buys latency. Every request also carries the session id as
`prompt_cache_key`: the sources, instructions and tool schemas are resent
unchanged each time, and that prefix is what the provider can cache. Token
usage per request is logged at info (`AI wizard model request`).

### Which model

Use **`gpt-6-luna`**. On 2026-10-05 three models imported the same eleven
sheets from the corpus, unattended, at `reasoning_effort: low`:

| Model | Sheets fully matched | Median seconds | Dollars per import |
| --- | ---: | ---: | ---: |
| `gpt-6-luna` | 6 of 11 | 55 | 0.004 |
| `gpt-6.1-sol` | 8 of 11 | 71 | 0.076 |
| `gpt-5.4-mini` (the model until then) | 1 of 11 | 56 | 0.043 |

`gpt-5.4-mini` read the gold box as zero on nine of the eleven; neither newer
model did once. `gpt-6.1-sol` matched two more sheets than `gpt-6-luna`, which
on eleven is within noise, and was sixteen seconds slower at eighteen times the
price. It is the one to switch to if accuracy is ever worth that. Five of each
newer model's mismatches were on one sheet (the Tabaxi bard), and the rest were
mostly skill bonuses on sheets both got wrong -- the server's skill assignment
or the expectations, not the reading. The harness that measured this is not in
the repository; the CLI's `model` records carry the token usage a new
comparison needs.

The key's absence disables AI import. Limits default to the values above;
workers are bounded at 32, turns at 200, sessions at 1000 and request timeout at
10 minutes. `workers` is per API process. `max_sessions` counts every stored
chat, finished ones included and across all processes; the daily sweep is
what brings it back down.

Additional fixed bounds are 8 files / 20 MiB per session, 256 KiB per text/JSON
file, 16,000 UTF-8 bytes per message, 128 KiB per tool argument payload, 12,000
output tokens per response and two SDK retries. Conversation growth is bounded
by item/event counts and a 2 MiB transcript threshold. Turns pause at the run
limit; manual Resume starts another bounded run. These are request/run limits,
not a billed-token accounting system.

One limit is per person: an owner may start 20 chats in 24 hours
(`WizardRunsPerDay`, see [Limits](backend.md#limits)). It is checked at a
chat's first message, which is also where its character is made and so where
the owner's character limit is checked; an opened chat nobody has written in
costs nothing and is not counted. The refusal is 400 `limit.wizardRuns`. The
count is read from the chats still stored, so discarding one gives its run
back -- see [known-caveats.md](known-caveats.md#limits-are-counted-not-reserved).
A tool call that would take a character past a per-character limit is refused
to the model as a tool error and the log is left as it was.

The checked-in nginx configuration raises `/v1/`'s body limit to 21 MiB. **Deploying
a release does not install nginx configuration**: apply that file separately.
A poll is an ordinary request answered at once, so it needs nothing from
nginx and leaves the normal HTTP timeouts in place.

## Validation and later milestones

Tests cover translated/fuzzy spell identity, printed-name resolution and its
scopes, a scripted end-to-end import that ends with no custom entry and nothing
pinned (on the SRD and on a namespaced pack), skill distribution, the
one-time checklist reminder, calls after a turn has ended, worker bounds,
cancellation, owner isolation, HTTP uploads, the opened chat answered in steps and poll recovery, score preservation
and rebasing, the builder's own entries on the stored character (name apart
from rules, class at 1 and subclass at its level, one manual ability-scores
entry, nothing printed pinned over an identity field), discard deleting the
character, private definition isolation/versioning, the Responses streaming
adapter, and desktop/mobile chat, View and Edit opening the real character,
rules preselected inside the chat, explicit questions and suggested answers,
required-choice review guards and the end of a turn arriving by poll.
The provider adapter is tested against a local HTTP fixture; a live provider
smoke test requires deployment credentials and a configured model.

Sessions, attachments, the transcript with its provider state, operation
outcomes and the revision and generation are stored together, with turns
claimed under a lease (see [Session lifetime](#session-lifetime)). The
character and the private releases are stored by their own adapters, so a
restart loses nothing. `RunAgentStore` runs one contract over the PostgreSQL
and in-memory stores -- one winner among eight claims, a fenced turn that
cannot save or append, an expired lease taken over, the sweep -- and the use
case tests run two agents over one store: a turn released by a stopping
process and finished by another.
Text-only character creation uses the same bounded tools. A future MCP adapter
can reuse them.

