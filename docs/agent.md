# AI Wizard

The AI Wizard creates **one character draft** from a description or uploaded sources.
It does not create a character record until the owner presses **Save character**.
The same session contains its attachments, conversation, tool results, draft,
custom definitions and eventual character ID. A model never receives an owner,
character or session ID parameter with which to target another record.

## User flow

The desktop and mobile menu has **AI Wizard / AI Помощник**. Folder actions link
to the same workspace while preserving the destination folder. The owner first
selects and confirms rules and pack releases; only then can they describe a
character or upload sources. The HTTP API also requires this pinned rules lock. The server needs `agent.api_key` and `agent.model` configured to process
uploads. Without the provider, imports report that AI import is not configured;
the workspace does not fall back to the legacy HexSheet JSON screen.

Upload PDF, PNG, JPEG, WebP, JSON or UTF-8 text and optionally describe what needs
attention. Multiple files belong to the same character. Files may also be added
later in the conversation. Filenames identify sources; new attachments must not
reuse a filename. The server checks the content type rather than trusting the
browser's filename or MIME header.

Chat is one chronological message log. Assistant text and factual progress
appear inline. Progress comes from validated writes, such as “Class imported:
Sorcerer, level 3” or “Name imported: Vas Pup”. Catalogue searches and internal
tool labels are hidden. Consecutive progress updates share one compact block.
View, Edit and Save buttons appear inside assistant messages when relevant. Each user message
contains its own attachments and text; filenames are not a separate global row.
Source/page evidence and assumptions remain internal metadata, not transcript
labels or a preview warning panel. The compact composer sits at the bottom;
Enter sends and Shift+Enter inserts a line break. Scrolling up stops following.

Routes are `/ai-wizard/:sessionId`, with `/character` and `/editor` child pages.
Old `/characters/import` routes redirect, retaining the session and query.
Breadcrumbs start at AI Wizard.
Legacy `?session=...` links still open. The preview uses the standard `SheetBody`;
the editor uses the complete standard `BuildScreen` against the same draft.
Opening a page does not save the character. Editor changes enter the transcript
and the assistant's context. Unsent chat input survives navigation within the
session. Neither page displays the internal assumptions list.

Unresolved choices belong in chat. `ask_user` records a question and up to six
suggested text answers. The latest unanswered question shows clickable replies;
the composer always permits a different free-text answer. A reply is an ordinary
user message, and the assistant applies it using validated choice tools. The
model must ask a direct question instead of narrating that it is paused.
`plan_import` records the six printed ability scores immediately and a checklist
of documented source facts. Unknown sheet
paths become custom-entry keys. `import_facts` writes batches and reports errors
for rejected facts; it publishes progress only for successful writes.
`prepare_review` refuses missing checklist entries even for a partial draft,
and returns unresolved required prompts without ending the run;
it accepts a partial draft only when the model explicitly sets `allow_incomplete`
after the user has chosen to leave choices incomplete. This flag is a model
assertion of that conversation, not a separate server-verified consent record.
Saving a partial character remains an explicit user action.

There is no interruption control in the UI. Sending and attaching files wait
while the assistant is running; typing the next reply remains possible. Terminal
status snapshots re-enable Send and focus the composer. A three-second snapshot
refresh recovers a missed end-of-turn stream update without disabling input.
Resume continues a paused conversation; Retry uses the same draft after a failure.
The internal stop API remains for cancellation and lifecycle handling.
Waiting for a reply holds no worker. Discard removes an unsaved session.
Save is an explicit owner action and may save a partial draft whose remaining
choices can be answered in the ordinary character editor. Saving makes the
conversation read-only. The saved sheet links back to that history.

## Session lifetime

State is **in memory in one server process**, including attachments and private
packs. Browser reload or SSE reconnection recovers the session from the URL's
session ID (or legacy `session` query parameter). The import screen also lists unfinished sessions for the
selected folder. Server restart loses this state, as it currently loses ordinary
characters. PostgreSQL account storage does not change that guarantee.

The coordinator in `internal/usecase/character/agent.go` owns the session map and
its mutation lock. It deliberately has no database or durable job framework in
this release. A fixed number of goroutines run independent sessions, with one
active model request per session. Model I/O runs outside the lock; draft/tool
mutations and finalization run inside it. Waiting, review and paused states do
not consume workers.

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
| `get_build_context` | Current projection, compact open prompts, rules lock, log and manual content |
| `read_source` | Text/JSON source contents, or reference to an attached image/PDF |
| `search_catalog` | Ranked identities across supported locales within the pinned rules lock |
| `get_option_details` | Exact catalogue mechanics and, when available, a pack wire example |
| `list_choice_options` | Available keys and choice constraints, paged 60 keys at a time |
| `plan_import` | Record documented source facts and custom-entry keys |
| `import_facts` | Batch observations, with errors for individual rejected facts |
| `resolve_import_facts` | Upsert an observed structural reference or supported sheet field |
| `answer_choice` | Apply an answer through the existing character validator |
| `revise_choice` | Replace a prior choice and report invalidated dependent entries |
| `upsert_custom_option` | Preserve an editable typed custom definition with known details |
| `reconcile_import` | Reproject the current draft for comparison with sources |
| `ask_user` | Ask a blocking question and release the worker |
| `prepare_review` | Mark the draft ready for the owner's review |

Session creation and saving are application operations, not model tools. Source
text, catalogue descriptions and files are explicitly treated as untrusted data
in the model instructions. There are no shell, network-browsing, unrelated
character-editing or publishing tools.

### Identity matching is separate from eligibility

Search returns canonical refs. Exact refs, normalized localized names and slugs
are ranked first, followed by Unicode edit distance and token overlap. Results
are merged across locales and deterministically ordered. Optional spell level
narrows candidates. Mass/Greater/Lesser variants are not normalized into their
ordinary counterparts. Fire Bolt and Fireball retain distinct identities.

The model can make a reasonable unambiguous match and record an assumption;
there is no confirmation dialog for every typo. An identity match does **not**
authorize choosing a spell. Open prompts and the normal validator enforce its
source, class, purpose, counts and held options. Known, prepared, spellbook and
granted spells continue to use the existing spell rules. Ambiguous or incomplete
content can remain manual rather than being silently replaced.

### Observations and choices

A foreign sheet describes a current state, not the player's historical choices.
Observed name, race, classes and totals are upserted without inventing a sequence
of level-up answers. Repeating the same observation replaces that observation.
Choices are only applied through the choice validator. Direct field edits are
later log entries, so updating an older observation does not erase them.

During import, `finalAbilities.<ability>` records an observed total before
modifier-dependent calculations. Race, ASI and pack ability bonuses cannot add
themselves to that total again. At save, additive cases are reconciled back to
ordinary base scores and checked by projection; subsequent progression then
works normally. A non-invertible custom rule retains its explicit total override
rather than changing the observed value. Setting an ordinary `abilities.*` base
score later removes that ability's override. Other imported numeric overrides
remain explicit log changes, as with the legacy importer.

Finalization validates the projection and name, checks folder ownership again,
and publishes the entire log through the repository's atomic `CreateWithLog`.
Repeated finalization returns the same character ID. No empty character is
created as an intermediate step.

### Custom content

Custom classes, races, subraces, backgrounds, subclasses, spells, cantrips,
equipment, feats, features and traits are stable definitions in the character
log. Names, descriptions, provenance and explicitly known numeric details are
editable in both AI drafts and the ordinary creation/edit wizard. Unknown
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

The normal wizard offers **Custom…** on applicable tabs and **Edit custom
option** for saved definitions. Observed fields have direct edit controls on
their corresponding tabs. Custom choices can be deselected to return to
catalogue selection.

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
| `GET /v1/agent-sessions/:id` | Session snapshot and projected sheet |
| `GET /v1/agent-sessions/:id/events` | SSE snapshots, updates and cursor recovery |
| `POST /v1/agent-sessions/:id/files` | Multipart files, revision and optional instructions |
| `POST /v1/agent-sessions/:id/control` | Revision plus `message`, `stop`, `resume`, `retry` or `discard` action |
| `POST /v1/agent-sessions/:id/edit` | Revision and ordinary event DTOs |
| `POST /v1/agent-sessions/:id/finalize` | Revision; idempotently saves the character |

Normal characters expose GET/POST `/v1/characters/:id/custom-options`.
GET returns `{revision, options}`; POST accepts `{revision, option}` and returns
the updated sheet and revision. Writes check ownership and optimistic revision.
Definitions cannot be erased by generic note replacement.

The standard editor also uses `/v1/agent-sessions/:id/draft`: GET `sheet`,
`events`, `prompts` and `custom-options`; POST `custom-options`, `events` and `events/revise`; PUT/DELETE
`events/:seq`. These routes reuse ordinary character handlers and validation with
a scoped draft repository. Reads/commits run under the coordinator lock, reject
foreign owners and stale revisions, and never create a normal character record.
Writes are rejected while the assistant is active or after saving. Unsupported
operations (copy, move, delete, rules migration) are not exposed.

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
10 minutes. The session count includes retained saved conversations, so capacity
must be sized for this memory-first deployment. Unsaved drafts can be discarded.

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

Tests cover translated/fuzzy spell identity, worker bounds, cancellation, owner
isolation, HTTP uploads/SSE recovery, score preservation and rebasing, repeated
Save, private definition isolation/versioning, the Responses streaming adapter,
and desktop/mobile chat, standalone preview/editor pages, composer preservation, draft
editor ownership/revision checks, explicit questions and suggested answers,
required-choice review guards, missed terminal status recovery, and read-only
saved history.
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

It checks source values, spell identities, inventory counts, custom content and
source discrepancies, then saves, reopens, edits the name and a custom definition
and verifies that facts survive. Test edits are restored; the imported character
remains saved. It uses the configured provider and can incur provider charges.
No provider credentials or session cookies are stored by the script. The PDF
itself is not committed.
