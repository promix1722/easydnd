# Backend

The engineering doc for the [easydnd.org](https://easydnd.org) Go API:
architecture, layer rules, configuration and deployment. For the browser client
see [web.md](web.md); for the game model this serves, see [dnd.md](dnd.md).

Status: **the API is real.** The structure, wiring, deploy path, entity model,
SRD compendium, passkey and Google sign-in, and the rules math for creation and
level-up are built and tested. A character can be created, built and levelled
over HTTP.

## Quick start

```sh
make run/server                     # dev mode: text logs, debug level
curl localhost:8080/v1/health       # {"status":"ok"}
curl localhost:8080/v1/catalog      # the compendium's index
curl localhost:8080/v1/characters   # {"characters":[]}

make verify                         # everything CI checks, back and front, two jobs at once
```

`make run/server` needs no database: `config.dev.yaml` sets no `db.url`, so it
runs on the in-memory account store and says so. For durable accounts locally:

```sh
make db/up                          # throwaway Postgres on :5433
make run/db                         # the API against it, migrating on startup
make test/db                        # the suite including the Postgres adapter (-p 1: shared database)
make db/down                        # and delete it again
```

Or all of it at once, which is also the way to run more than one worktree:

```sh
make dev                            # Postgres, the API and the web client, one Ctrl-C
make ports                          # what this worktree claimed, and where to open it
```

`make dev` is a **disposable** stack: Ctrl-C takes the database down with the
servers, so every run starts on an empty schema and nothing is left behind.
When you want accounts to survive a restart, use the three targets it composes
instead -- `make db/up` once, then `make run/db` and `make web/dev` -- and
`make dev/down` when you are finished with them.

`make run/db` loads `config.local.yaml` if you have one (it is gitignored);
otherwise `make config/dev` writes `config.dev-run.yaml` for it, carrying this
worktree's ports and origins.

Without `TEST_DATABASE_URL` the Postgres adapter tests skip themselves, which is
what keeps `go test ./...` and `make verify` green on a machine with no Docker.
CI sets it against a service container, so they are not skipped there.

## Tests

```sh
make test/unit                      # the suite, ~4s
make test/race                      # the same suite under the race detector, ~9s
make test/db                        # including the Postgres adapter (needs make db/up)
```

**Where a database is involved, `test/db` is the only correct target**, and CI
runs it for that reason. Three packages reach the one database and each opens by
wiping it -- `internal/adapter/repository/postgres` for users and for groups, and
`internal/api/http`'s durability test. `go test ./...` runs packages in parallel,
so without `test/db`'s `-p 1` one truncates a table another has just written,
and the failure surfaces in whichever package lost the race rather than in
whichever caused it. Both go red at once, which is the signature to recognise.

CI used to run `make test/unit` with `TEST_DATABASE_URL` set -- the one place
that combination arose, because on a machine without the variable those tests
skip and the race cannot happen. It failed the v1.0.1 release and cost a manual
re-run of the whole job. The rule is in `CLAUDE.md` because it is easy to
reintroduce: set `TEST_DATABASE_URL`, run `test/db`.

`make verify` runs `test/unit`. It does **not** run `test/race`, and that is a
deliberate trade rather than an oversight.

The race detector used to be on by default, and it cost far more than it
looked: 46s against 10s. Two thirds of that gap was not even work. A `-race`
test binary sleeps a full second at exit -- `GORACE`'s `atexit_sleep_ms`,
defaulting to 1000 -- and this module has sixteen test packages, so every run
spent sixteen seconds on an idle machine. `make test/race` sets
`atexit_sleep_ms=0` and takes that back, which is why it now costs 9s rather
than 25s. What the sleep buys is a last chance to check a goroutine still
running when `main` returns, and nothing here leaves one: the HTTP tests drive
`httptest` in-process and synchronously, and `internal/app`, which owns the
only real server lifecycle, has no tests. A race *during* a test is reported
exactly as before.

That leaves 9s against 4s, and the reason the detector still sits outside
`verify` is what `verify` is for. Nothing runs on `main`, so it is the only
gate there is, and a gate slow enough to be worth skipping stops being a gate.
The detector moved onto the path worth taking before a `git tag` -- the point
where a missed race would otherwise ship. **Run `make test/race` before
tagging.** It found real races in the HTTP layer and the stores once, and
`make test/unit` will not find the next one.

### verify runs two jobs, longest first

`verify` used to be a serial chain, which meant the Go side's time was added to
the frontend's rather than spent inside it. It is now a `make -j2` over the same
leaf targets, and **the order they are named in is the schedule**: `make -j`
starts goals left to right as slots come free, so `web/test` -- fifteen seconds
against six for everything else put together -- has to go first. Left at the end
of the list it lands in the last slot and the run costs its length plus
everything before it. Named first, the Go lane, the frontend's typecheck and the
production build all happen inside its shadow, and `verify` costs about what
`web/test` costs.

Two jobs, not more. One of them is vitest, which forks
`availableParallelism - 1` workers of its own, so `-j2` is already the whole of
a four-core machine; `VERIFY_JOBS` is the knob for a worktree sharing the box.
`--output-sync=target` holds each target's output and prints it whole, so a
failure arrives as one block rather than interleaved with whatever else was
mid-run -- at the cost of nothing printing until a target finishes.

CI has the same two lanes, and since the pipeline was unchained it goes further:
its six check, build and test jobs all start at once. See
[The three checks run at once](#the-three-checks-run-at-once-and-nothing-is-cached).
`verify` cannot copy that -- one machine, not six -- so here the lanes are two
and the order they are named in does the scheduling.

The other reason the suite is fast is that each test package shares **one**
`catalogfile.Source`. `Source.Load` caches a converted `*catalog.Catalog` per
locale, and a `Catalog` is immutable, so one read of the 1.55 MB compendium
serves every test in the binary. Building a fresh `Source` per test threw that
cache away, and the suite was doing it about 120 times a run -- which was most
of its remaining runtime. `internal/domain/character` alone went from 3.0s to
0.06s. If you add a helper that needs the compendium, reach for the package's
existing `catalogSource` rather than calling `NewSource` again; the internal
and external test packages of one directory need one each, since a package-level
var cannot cross that line.

## Running more than one worktree

Every port the local stack binds used to be a constant, so exactly one checkout
could run at a time -- and worse, a second `make db/up` adopted the first one's
container and its database. Ports are now derived from a **slot**, one number
per worktree:

| | port | reached at |
| --- | --- | --- |
| Vite dev server | `8080 + slot` | `$PUBLIC_HOST:{8880 + slot}`, if a proxy is in front |
| the API | `18080 + slot` | loopback only; Vite proxies `/v1` to it |
| Postgres | `5440 + slot` | loopback only |
| compose project | `easydnd-{slot}` | its own network and containers |

Only the web server needs to be reachable from outside, so a ten-port proxy
range holds ten worktrees. A worktree that has claimed nothing keeps the old
constants exactly -- Vite `5173`, the API `8080`, Postgres `5433`, project
`easydnd` -- which is what the quick start above describes. Each family starts
*past* its unclaimed default on purpose: had slot 0 been Postgres `5433`, an
unclaimed worktree and a slot-0 worktree would publish the same port and the
second one up would quietly talk to the first one's database, which is the
failure this exists to remove.

`make dev` claims a slot by binding each candidate port to see whether it is
really free, and writes the answer to `.dev-slot` (gitignored). It prefers the
slot this worktree used last, so the address you bookmarked survives a restart;
if that one has been taken it says so and moves. Every other target *reads*
`.dev-slot` rather than probing, which is what stops `make db/down` and
`make test/db` reaching into a neighbour's stack.

```sh
make dev                            # claim, then bring the stack up
make dev/down                       # take it down and delete its database
make ports                          # this worktree: slot, ports, and the URL to open
make slots                          # every slot on the machine and who holds it
make db/psql                        # a shell on this worktree's database
```

`make dev` cleans up after itself: it traps `INT` and `TERM` as well as `EXIT`,
because a shell killed by a signal it does not trap dies *without* running its
`EXIT` trap -- which would skip the cleanup on the very Ctrl-C meant to trigger
it. `make dev/down` is for when it could not clean up anyway: a closed
terminal, a `SIGKILL`, or a stack started from `db/up` and `run/db` separately.
It prints the slot table afterwards, and this worktree's row reading **idle**
is the proof that the ports came back.

`cmd/devslot` is the prober. It binds the exact address a server will use,
because connecting instead would call a bound-but-not-accepting socket free.
Claims are recorded under `$XDG_RUNTIME_DIR/easydnd-devslots` so worktrees can
see each other's, and a claim with nothing listening behind it ages out after a
minute -- long enough to cover the gap between claiming a slot and binding it.
Two `make dev` in the same second can still pick the same slot; the loser fails
loudly on the port bind, and re-running fixes it.

### Reaching it from another machine

If a reverse proxy fronts this machine, tell the Makefile once, in
`~/.config/easydnd/dev.mk` (gitignored, read by every worktree):

```make
PUBLIC_HOST      := dev.example.org
PUBLIC_PORT_BASE := 8880
```

That is the whole configuration. `make dev` then puts
`http://dev.example.org:{8880 + slot}` into the generated config's
`auth.rp_origins` and hands it to Vite, which needs it for two things of its
own -- see [web.md](web.md#one-dev-server-per-worktree).

Two consequences of reaching the app over plain HTTP on a name that is not
`localhost`, both by design rather than breakage:

- **Passkeys are unavailable on the `888x` ports.** WebAuthn requires a secure
  context, so `window.PublicKeyCredential` is undefined and the sign-in page
  draws no passkey card at all. The guest session is the way in. They do work at
  `http://localhost:{port}`, which browsers treat as secure, and they work in
  `make preview` -- see [web.md](web.md#make-preview-is-the-only-secure-origin),
  which exists because service workers and the install prompt are blocked by
  this same rule.
- **`env: development` is doing real work.** It is what clears the cookie
  `Secure` flag and the `__Host-` prefix; a production-mode cookie would never
  be sent over such a connection. The generated config always sets it.
- **`navigator.clipboard` is undefined**, for exactly the same reason as
  `PublicKeyCredential`. The invite sheet falls back to a selection copy and,
  if even that is refused, says so and selects the link -- see
  [web.md](web.md#copying-the-invite-link). Worth knowing because the first
  version used Mantine's `CopyButton`, which drops the error its own hook
  reports, so the button silently did nothing here and worked on production.

## Layout

Directory convention follows [golang-standards/project-layout][layout]; the
internals follow clean architecture.

```
cmd/easydnd/          process entrypoint: flags, config, logger, signals
cmd/srdgen/           converts the vendored SRD dump into data/srd_5.1/
cmd/devslot/          hands each worktree its own development ports
cmd/llm/              dev-machine OpenAI tool: batch image generation, JSON translation
internal/
  app/                composition root -- the only package that knows every layer
  buildinfo/          Version, stamped by the linker
  config/             env-driven configuration
  logging/            slog constructor + request-scoped logger on context
  types/              transport-agnostic error vocabulary
  domain/rules/       shared value objects: slugs, dice, coins, choices  (layer 1)
  domain/catalog/     the SRD compendium and its Source port             (layer 1)
  domain/character/   the event-sourced character aggregate              (layer 1)
  domain/user/        the account aggregate and its passkeys             (layer 1)
  domain/auth/        the ceremony and token-signing ports               (layer 1)
  usecase/            application services                              (layer 2)
  adapter/catalog/    reads the compendium off disk                     (layer 3)
  adapter/repository/ outbound adapters: memory, and postgres for accounts (layer 3)
  adapter/sheet/      reads character sheets exported by other tools     (layer 3)
  adapter/webauthn/   runs the WebAuthn ceremonies                       (layer 3)
  adapter/oidc/       exchanges authorization codes with Google          (layer 3)
  adapter/token/      signs the session and ceremony cookies             (layer 3)
  api/http/           inbound adapter, gin                              (layer 3)
web/                  browser client (React + TypeScript, Vite); see web.md
data/srd_5.1/         generated SRD data, read at startup
deploy/               server-side release activation and the nginx site
docs/reference_srd_5.1/   vendored SRD 5.1 source data (reference only)
docs/reference_hexsheet/  a real exported character sheet, used as a shape reference
```

`docs/reference_srd_5.1/` is the *input*: an untouched vendored dump. `data/srd_5.1/`
is the *output*: this project's own format, generated by `cmd/srdgen` and read at
runtime. Nothing outside the generator reads the vendored dump.

`cmd/llm` is a development-machine tool and no part of the service: it calls
the OpenAI API (key from `OPENAI_API_KEY`) to batch-generate entity artwork
and to translate JSON files, writing plain files that a developer then moves
wherever they belong -- it knows nothing of the repo's layouts, because where
images will live (database, S3) is not decided yet. `llm images -in
prompts.json -out art/` turns a flat name-to-prompt object into `<name>.png`
files, skipping ones that already exist so an interrupted batch resumes by
rerunning; `llm translate -in data/srd_5.1/i18n/en/spells.json -out
spells.ru.json -to ru` writes a same-shaped copy with every string leaf
translated, keys and `{{placeholders}}` verified untouched. `-dry-run` on
either shows the work without the key or the spend.

## The API

The resource routes below, plus the sign-in routes in
[Authentication](#authentication). Every resource route is at most **one level
deep**: a sub-resource under an addressed parent, never a sub-resource of
that.

| Method | Path | Returns |
|---|---|---|
| `GET` | `/v1/health` | liveness |
| `GET` | `/v1/version` | the release identifier -- a deploy contract, see below |
| `GET` | `/v1/catalog` | the compendium's index: ruleset, locales, collections and counts |
| `GET` | `/v1/catalog/{collection}` | one collection; `?slugs=a,b` narrows it. Spells and magic items list as *summaries*, and `?slugs=` returns full fidelity. Spells alone also answer search parameters (`q`, `level`, `school`, `class`, `castingTime`, `concentration`, `ritual`, `material`, `limit`, `offset`) with a filtered, level-then-name-sorted, paged `{spells, total}` envelope -- the filter itself is `domain/catalog.SpellFilter`; with no search parameter the route serves the bare array from its byte cache, unchanged |
| `GET` | `/v1/characters` | summaries |
| `POST` | `/v1/characters` | create: a name (and an alignment, if there is one) |
| `POST` | `/v1/characters/import` | import a sheet exported by another tool |
| `POST` | `/v1/characters/stub` | **development only** -- build the reference character in one call |
| `GET` | `/v1/characters/{id}` | the log |
| `DELETE` | `/v1/characters/{id}` | |
| `GET` | `/v1/characters/{id}/sheet` | the projection |
| `GET` | `/v1/characters/{id}/prompts` | what must be decided next |
| `GET` | `/v1/characters/{id}/events` | the log |
| `POST` | `/v1/characters/{id}/events` | append; returns the new sheet |
| `DELETE` | `/v1/characters/{id}/events` | truncate: `?after=N&expectedSeq=M` |
| `PUT` | `/v1/characters/{id}/events/{seq}` | replace one entry: `{expectedSeq, event}`, `?dryRun=true` |
| `DELETE` | `/v1/characters/{id}/events/{seq}` | remove one entry: `?expectedSeq=M`, `?dryRun=true` |
| `PUT` | `/v1/characters/{id}/folder` | file it elsewhere |
| `POST` | `/v1/characters/{id}/copy` | duplicate it, log and all |
| `GET` | `/v1/folders` | the account's folders, default first, then in their owner's order |
| `POST` | `/v1/folders` | create: a name |
| `PUT` | `/v1/folders/order` | the whole order: every movable folder, in sequence |
| `PATCH` | `/v1/folders/{id}` | rename |
| `DELETE` | `/v1/folders/{id}` | **deletes the characters in it too** |
| `GET` | `/v1/groups` | the groups you are in, with your role in each |
| `POST` | `/v1/groups` | create; you become its owner |
| `GET` | `/v1/groups/{id}` | the group and its whole roster |
| `PATCH` | `/v1/groups/{id}` | rename |
| `DELETE` | `/v1/groups/{id}` | owner only |
| `POST` | `/v1/groups/{id}/invites` | mint a link: `{"role":"dm"\|"player"}` |
| `PATCH` | `/v1/groups/{id}/members` | change a rank: `?user=U`, `{"role":...}`; `owner` hands the group over |
| `DELETE` | `/v1/groups/{id}/members` | remove: `?user=U`; your own id is how you leave |
| `POST` | `/v1/invites/preview` | read a link without acting on it |
| `POST` | `/v1/invites/accept` | redeem a link |
| `GET` | `/v1/groups/{id}/characters` | the group's table: what its members have shared |
| `POST` | `/v1/groups/{id}/characters` | share one of your own: `{"character_id":"chr_x"}` |
| `DELETE` | `/v1/groups/{id}/characters` | unshare: `?character=C`; **takes it out of every game too** |
| `GET` | `/v1/games` | every game at every table you sit at, newest first |
| `POST` | `/v1/games` | open one: `{"group_id":"grp_x","name":"..."}`. DM or owner |
| `GET` | `/v1/games/{id}` | the game, your rank, and its roster |
| `PATCH` | `/v1/games/{id}` | rename. DM or owner |
| `DELETE` | `/v1/games/{id}` | DM or owner; the characters stay on the table |
| `POST` | `/v1/games/{id}/characters` | seat some: `{"character_ids":[...]}`; your own land on the table too |
| `DELETE` | `/v1/games/{id}/characters` | unseat one: `?character=C` |
| `GET` | `/v1/shared/{id}/sheet` | a shared character's sheet, read-only |

Three of those need a word about their shape.

**Nothing about a game hangs off `/v1/groups`.** A game is a section of its
own, not a corner of a group: somebody at three tables wants one list of their
games, and the group is a *field* on a game rather than the way in to one. So
the listing is `GET /v1/games` with no group to name, and creation carries the
group in the body -- it is the only operation here that has to say which table,
and putting it in the path would make the other six look reachable that way too.

The depth rule points the same way. Hung under its group, a game's own roster
would be `/v1/groups/{id}/games/{gid}/characters`, which is three levels where
the convention above allows one.

**`/v1/shared/{id}/sheet` hangs off nothing at all**, and that is the honest
shape: what grants the read is "some group we are both in", so naming any one
of them in the URL would be a lie about why it was allowed. Note also what is
absent — there is no `/v1/shared/{id}`. `/v1/characters/{id}` is a character's
*log*, the record of every decision its owner made and the order they made them
in, and that is not the table's business. A table sees what a character **is**.

**Seating takes a list, always**, so there is one request shape whether it is
one character or nine, and "everyone at this table" is the client sending the
list it already has on screen.

**Seating your own character shares it.** Everything on a roster has to be
readable by every member -- a game carrying a name nobody but its owner may
open would be a leak the DM caused by accident -- so a character that is not on
the table yet is put there, provided it belongs to the caller. Somebody else's
unshared character is still a 400: a DM runs the table, but does not get to
publish another player's character on their behalf.

The two `events/{seq}` routes address a *member* of the log by position, which
is what `Seq` means. That is not a third level: `events` is the sub-resource,
`{seq}` names one of them, and there is no route below it.

A group's members are addressed the other way, by `?user=`. Either would have
been consistent with the rule above; the query parameter is what
`DELETE /v1/characters/{id}/events?after=N` already does, and a member is named
by an opaque account id rather than by position.

`PATCH` arrives with groups, as `PUT` does with the log entry routes above:
everything older than both is `GET`, `POST` or `DELETE`.

The invite routes are a separate tree rather than sitting under `/v1/groups`
because somebody redeeming a link is **not in the group yet and cannot name
it** -- the token carries the id, so there is no addressed parent to hang them
off. Both take the token in the **body** and never in the URL: our own access
log records the route pattern, but nginx in front of it logs the whole request
line, and an invite token is usable for a day. The browser keeps it in a URL
*fragment*, which is never sent to any server at all.

`GET /v1/characters` takes `?folder=` to narrow the listing, and `POST
/v1/characters` takes a `folder` in the body. `POST /v1/characters/import` takes
`?folder=` instead, because its body is the exported sheet itself.

### Importing states, not histories

`POST /v1/characters/import` takes a sheet exported from another tool -- today
HexSheet -- as the request body, and answers with a character plus a **report**
of what did not survive.

The route exists in tension with everything above it. easydnd stores *choices*
and derives the sheet; an exported sheet is the opposite, a set of finished
numbers with no record of where they came from. It says the character is
proficient in Stealth, not whether the class, the background or a racial trait
granted it.

The importer does not try to bridge that by reconstructing the choices.
Recovering them means solving a matching problem -- six proficient skills
across a class prompt taking four from a restricted list and a trait prompt
taking two from all eighteen -- and then presenting one of several answers that
fit as the one the player made. So instead:

- the export's final state becomes the character's **opening** state, as an
  `init` event carrying the numbers;
- typed `race`, `class`, `level` and `subclass` events name what the export
  states outright, so traits, features and level-scaled values attach -- each
  level before the subclass it makes due, so that the log the importer writes
  is one the build flow could have written itself and one a later replacement
  will keep;
- **no prompt is answered.** Every choice stays open, and the client sends the
  player to the build screen rather than the sheet;
- anything with no home in the model is named in the report rather than
  dropped.

One consequence is worth stating because it looks like a bug. Ability scores
are input tier, applied *before* racial bonuses, so the init event records the
export's scores **minus the race's fixed bonuses** -- a half-elf's Charisma 14
is stored as 12 and projects back to 14. The race's *optional* bonuses are not
subtracted and not guessed: the export does not record where they went, so that
prompt simply stays open, which is why an imported character can look a point
or two light until it is answered.

`internal/adapter/sheet/hexsheet` also holds the codebase's only name-to-slug
lookup. Everywhere else a reference is already a slug; an import is the one
place a display name is all there is.

### Creating a character takes a name

`POST /v1/characters` takes a name, and an alignment if the player already has
one in mind. It used to take the generation method and the six base scores as
well, and the init event it seeded carried all eight -- which is precisely why
the name and the scores were the two things a build screen could not offer to
revisit. The log is **one entry per selection**; a selection with no entry of
its own is a selection nobody can point at. See
[dnd.md](dnd.md#log-and-events).

So the scores are an ordinary open choice now. A freshly created character has
`character/abilities` outstanding, answered with a `change` event carrying the
six `abilities.<ability>` paths, and the generation method travels with that
answer rather than with creation. The bound on a score -- 1 to 30, wide enough
for a DM's ruling and narrow enough to reject a typo -- moved with them, and is
checked where they now arrive.

### The stub builds a character; it does not import one

`POST /v1/characters/stub` makes the level-3 half-elf rogue of
`docs/reference_hexsheet/` in one call. It is a development convenience --
reaching a character worth looking at otherwise means five tabs and a dozen
answers, and the character store is in memory, so a restart costs that walk
again.

It would have been one line to point it at the importer, and that would have
been wrong. An import records what a character *is* rather than what was
chosen, so an imported log answers no prompts and arrives with every choice
still open -- see [Importing states, not
histories](#importing-states-not-histories). That is the honest way to carry a
foreign sheet across and the exact opposite of what a stub is for. So the stub
is *built*: `Create` writes the opening entry and one `Apply` puts the eight
selections through `validateAndAttribute`, the same path an append from the
build screen takes. Every entry is checked against the prompts open at that
moment and stamped with the group of the prompt it answers, which is what keeps
the stub a log the build flow could have written rather than a shape only this
endpoint can produce.

Two consequences of going through that path, both of which the tests pin:

- **The level comes before the subclass it makes due.** A rogue is offered a
  Roguish Archetype *because* they have reached level 3, so nothing offers
  `subclass:thief` until the level granting it is in the log. Answering them the
  other way round is rejected outright. The domain's own fixture in
  `fixtures_test.go` has them the other way round and is not wrong to: it calls
  `Log.Append`, which checks the shape of an entry and never asks whether
  anything was offering it.
- **One entry carries no source.** Equipping the leather armor a rogue's kit
  contains answers no prompt and closes none, because no rule says a kit is
  worn. It is therefore unattributed, exactly as a DM's ruling is, and sits in
  no build-screen tab.

The stub answers its **optional** prompts too, and that is worth saying because
"complete" and "finished" are not the same thing. Seven prompts here are
optional -- the half-elf's third language and all six acolyte poses -- so the
character counted as complete while the build screen still listed seven rows
under "still to choose". Those seven answers are the only part of the log that
is *invented* rather than transcribed: the export names no third language, and
its background is Urchin, which SRD 5.1 does not publish. `character/level` is
left open on purpose, being the standing offer every complete character carries.

One of the seven records an answer and buys nothing, and it is a gap in the
model rather than in the stub. `acolyte/starting-equipment/0` draws its options
from an equipment *category* (`rules.OptionsFromEquipmentCategory`, "any item in
holy-symbols"), and nothing outside the catalogue DTO reads that option-set
kind. `rules.OptionKeys` returns no keys for it, so `validateAnswer` skips the
membership check and **any slug at all is accepted** -- a nonexistent one, or a
rapier as a holy symbol -- and `Project` materialises none of them. The prompt
closes, so the build screen is right to show it settled; the item simply never
reaches the sheet. Every category-drawn equipment prompt in the compendium
behaves this way, and `TestStubOptionalAnswersReachTheSheet` asserts the gap so
that closing it fails loudly.

The route exists **only when `env` is `development`**, which is why there is no
check inside the handler: a guard in two places is a guard that can disagree
with itself, and the routing table already answers "does this endpoint exist?".
In production the path is not registered, so it answers 405 rather than 404 --
`GET`/`DELETE /v1/characters/{id}` already claim that shape, and to gin "stub"
is a character id there. Which of the two refusals gin picks is a routing
detail; that no handler runs is the point.

Gating it needed **no new configuration key**. `env` already distinguishes
development from production and already defaults to production, which matters
because unknown keys are fatal and a new one would have had to stage across two
releases -- see [Configuration](#configuration).

The stub's character is a second transcription of the one in
`internal/domain/character/fixtures_test.go`, and they deliberately do not share
a definition: that file is `package character` in the domain, so importing the
application layer from it would be a cycle. They also differ in one place worth
knowing when reconciling them -- the fixture carries the six ability scores in
its `init` event, which is what an imported log looks like, whereas creation no
longer bundles them and the stub answers `character/abilities` as its own entry.

### Creation and level-up are one flow

`GET /v1/characters/{id}/prompts` answers "what does this character still have
to decide?", and it is the only endpoint a build screen needs. It returns
prompts in the compendium's own `Choice` grammar -- the ones the compendium
poses, verbatim, and synthetic ones in the same shape for the questions it does
not pose ("which race?", "which class do you gain a level in?").

A finished character's remaining prompt is `character/level`. Answering it
appends a level event, which opens that level's own prompts. So there is no
level-up endpoint, because there is no separate question: creation is the first
pass through the same loop.

Three fields make the client mechanical rather than knowledgeable:

- **`event`** says what the answer must be posted as. The first level in a
  class is a `class` event and later levels are `level` events; a client that
  decided this itself would be reimplementing the rules in the browser.
- **`optional`** says whether a character is complete without it. Without the
  distinction the flow deadlocks -- an unpicked personality trait would mean
  the character is never finished, so the prompt offering a level is never
  reached.
- **`held`** lists options the character already has. Prompts are never
  narrowed by what is held, because narrowing would make the question depend on
  the order it was answered in; the client greys them out and the server
  rejects them. `heldOnly` inverts it for Expertise, where being proficient is
  the precondition rather than the conflict.

A prompt whose option set is **explicit and empty** is a question the player
answers in their own words. Three exist: a name, and the four roleplaying lines
in the `personality` group -- a personality trait, an ideal, a bond and a flaw.
SRD 5.1 prints eight of each and the compendium still carries them, but they
are offered as prompts no longer: a trait is the one line on a sheet that is
nobody's but the player's, and a menu of eight makes it the compendium's. The
state behind all four was always free text (`State.Identity.PersonalityTraits`
is a `[]string`), so what changed is that the prompt stopped pretending
otherwise.

Those four are posed the way `character/alignment` is, and for the same reason:
there is no option set to compare an answer against, so "answered" is a
question about the sheet rather than about the log. `promptBuilder.personality`
emits each only while its value is unset, and the change that sets it is what
closes it -- which is also what attributes the entry, through the same
`closedGroup` path the six ability scores go down. The projector no longer
seeds any of them from a picked suggestion; it used to, which meant choosing a
background *after* writing a trait silently overwrote it.

### Writing to a character

Every write states the sequence it expects the log to end at. The whole log is
one record, so without that check two clients would read, modify and write the
same blob and the later write would discard the earlier silently.

Every write returns the freshly projected sheet. That makes a build step one
round trip instead of two, and it is why the client needs no cache
invalidation: the response *is* the invalidation.

Every write also records a **`source`**: the group of the prompt the entry
answers -- `identity`, `abilities`, `race`, `background`, `class`, `advance` or
`personality`, the same vocabulary `/prompts` groups its questions by. The
**server** writes
it, from the prompt the event was matched against, and it is ignored if a
request carries one. That is not distrust for its own sake: the client already
posts what a prompt told it to post, so the server knows which prompt that was,
and a client-supplied source would be a second vocabulary for the same fact,
free to disagree with the one the rules produce. Entries the server cannot
attribute -- an imported log, a DM's `change` -- carry no source. `GET
/characters/{id}/events` remains the unabridged record either way.

`DELETE /events?after=N&expectedSeq=M` is the undo primitive: dropping a suffix,
which is what un-taking a level is. Nothing in the web client calls it any
more -- changing a pick goes through the replace route below -- but it is
working, tested API, and removing it would be a breaking change made as a side
effect of a UI decision.

An earlier version of this page said that changing a pick needs no undo,
because answers fold last-write-wins and re-answering a prompt is a plain
append. **That was false.** `promptBuilder.add` stops emitting a prompt the
moment it is fully answered, so posting the same prompt again is rejected as a
prompt the character does not have open. Last-write-wins is what lets a *later*
entry answer an *earlier* entry's question -- a trait's prompt does not exist
until the race is chosen -- and it was never a way to change an answer. The
route below is what changing an answer needs.

#### Replacing an entry

```
PUT    /v1/characters/{id}/events/{seq}[?dryRun=true]   {expectedSeq, event}
DELETE /v1/characters/{id}/events/{seq}[?dryRun=true]   ?expectedSeq=M
```

One mechanism for changing anything a player chose: replace the entry that
carries the choice and revalidate what follows. `PUT` because the body is a
complete replacement entry; `DELETE` because removing a level has nothing to
put back. `expectedSeq` guards the whole log exactly as it does on an append.

**Seq 1 is replaceable when the replacement is also an init event** -- that is
how a name is changed. `Log.Validate` already requires init to be first and to
appear exactly once, so this is a type check rather than a prohibition: what it
refuses is a log that could not be read back. Anything else at seq 1, and an
init event anywhere else, is a field error on `seq`.

The algorithm, in `internal/usecase/character/revise.go`:

1. the prefix before the target is kept untouched;
2. a replacement is validated **strictly**, exactly as an append is, so a
   rejection writes nothing and the stored log is byte-identical afterwards;
3. every entry after the target is replayed **in order**, each judged against
   the log rebuilt *so far* -- before it is applied. Not against the old log,
   which is gone, and not against the finished new one, which would make an
   entry's legality depend on entries that come after it;
4. the rebuilt log is renumbered by `character.Rebuild`, validated, and
   projected.

Two invariants make this something a player can trust.

**Revalidation is never stricter than the predicate that accepted the entry,
except where that predicate was wrong.** The replay checks each entry against
exactly the prefix it will sit on, which is the same thing the append checked
against. The exception is deliberate and is the whole reason the bug below had
to be closed first: an entry that only ever got in because nothing checked it
does not survive a replay.

**An entry carrying a `Ref` is never dropped merely because its answers died.**
A rogue whose race change invalidated one of their four class skills is still a
rogue: the class entry stands, keeps every other answer it carries -- the
Expertise, the starting weapon -- and the invalidated question comes back
outstanding under its own group. Deleting the entry would take the class, the
answers that were still fine and every level built on it, and a revalidation
that silently eats a player's choices is worse than the truncation it replaces.

The granularity is one **answer**, not one pick. An answer is what a prompt was
asked for, so half of one answers nothing: four skills picked together stand or
fall together, and the prompt is asked again.

**The response** is the ordinary `WriteResponse` plus `dropped[]`. Each entry
names its **original** seq -- the position the client last saw, not the one
after the rebuild -- its type, ref, level and source, a `reason`, and the
answers that were lost in the `rule` vocabulary a rejected append already
speaks. The three reasons are different events for a player: `not-offered`
means the entry itself is gone, `empty` means it had nothing left once its
answers went, and `answers-dropped` is *not* a deletion -- the entry stands,
minus some answers.

**The dry run is the same function with `commit=false`.** It loads, validates,
replays, renumbers, validates the rebuilt log and projects it, then skips
exactly one line: `repo.Rewrite`. A separate preview route would be two paths
to drift, and a preview that disagrees with its commit is worse than none. A
stale preview cannot be committed silently either, and that costs nothing
extra: the commit re-runs the replay, and if the log moved in between,
`expectedSeq` makes it the ordinary sequence conflict.

`Repository.Rewrite` is the port method behind it -- neither an append nor a
truncation, because replacing one entry can drop entries after it and the
stored log comes back a different length. The in-memory adapter is the only
implementation; the Postgres adapter holds accounts only, so there is no
migration and no backfill for `source` either.

#### Two questions about a reference

`validateRef` asks **does this entry exist in the compendium?**
`answersAnOpenPrompt` asks **was the character offered it?** Only the first
used to be asked, and that is a bug this change closes on the way: `POST
.../events {"type":"subrace","ref":"subrace:hill-dwarf"}` was accepted for a
half-elf, because `subrace:hill-dwarf` resolves perfectly well and nothing
looked at whether anything had asked for a subrace at all. The projector then
applied it. Revalidation cannot work until that is closed -- a replay with no
notion of "was this offered?" has no way to notice an entry the new prefix
orphaned -- so the two are now asked together, in that order, on every
structural event.

`answersAnOpenPrompt` matches on the prompt's own `event` block -- the same
three fields a client copies into the body -- and then on one of two shapes:
the prompt selects the entry itself ("which race?"), and the event names an
option it offers; or the prompt hangs off an entry the character already holds,
in which case it states the `ref` to post with, and matching that ref is what
keeps a race's own follow-up entries alive. It is also the function that yields
the entry's `source`, so an entry's group and its legality are decided by one
match rather than two that can disagree.

One consequence worth stating: a `feat` event is no longer acceptable, because
no prompt offers one. The Ability Score Improvement's feat branch is answered
as a `level` event -- that is what the prompt says to post -- and no other
prompt asks for a feat at all. The projector still knows the type; "nothing can
be answered before it is asked" simply now applies to it like everything else,
and a prompt that wants one has to say so.

### Folders

A folder is a named place one account files its characters. That is the whole
of it: one owner, nothing shared, no rule in the game reads it. It is **not** a
group of players -- that word is reserved, and kept out of this feature's
types, routes and screens on purpose, so the two cannot be confused when the
other one arrives.

**Every account always has one.** The default folder is created by the first
read that needs it -- `GET /v1/folders`, or creating a character with no folder
named -- so "a character is always somewhere" is true without a nullable column
and without a migration that walks every account that already exists.
Materialising it is `FolderRepository.EnsureDefault`, and it is a repository
method rather than a get-or-create in the usecase for one reason: two requests
arriving together for a new account would otherwise both find no default and
both make one, leaving that account two folders it can never delete. The store
holds the lock, so the store holds the invariant.

The default folder can be renamed and cannot be deleted. What an account cannot
lose is the folder, not the word on it.

**The order is the account's, and the default folder is not in it.** A folder
carries a `Position`, and `FolderRepository.List` sorts the default first, then
by `Position`, with the identifier breaking a tie so the order is total rather
than merely mostly-decided. The default leads whatever anybody rearranges: it
is the one folder an account is guaranteed to have, and a list whose first entry
wanders is a list nobody can point at. Sorting it in by name would have made
where it lands depend on what its owner renamed it to; sorting it in by
`Position` would make it move.

A new folder lands last. That is the only position that needs no decision from
whoever made it -- they asked for a folder, not for a place in the list.

**Reordering is a `PUT` of the whole run, not a move.** `PUT /v1/folders/order`
takes every folder the account owns *except* the default, in the order wanted.
Three properties follow, and they are the reason for the shape:

- **It is idempotent.** Sending it twice leaves the same order, so a client
  unsure whether a drag landed can simply send it again.
- **It cannot half-apply.** A "move this one up" arriving against a listing
  that changed since it was drawn moves the wrong folder. A complete order
  either matches the account's set or is refused; the store compares the two as
  sets and rewrites every position under one write lock.
- **It needs no version on a row.** The set comparison *is* the concurrency
  check: an order naming a folder that has since been deleted is a set
  mismatch, which is a 400 rather than a silent partial write.

Naming the default folder is a **400**, for the same reason deleting it is: it
exists, the caller owns it, and the honest answer is that this particular folder
does not move. Naming somebody else's is a **404**, from the same `ownedFolder`
choke point every move and rename goes through.

The `Position` is deliberately **not** on the wire. `GET /v1/folders` already
returns the folders in order, and a number beside a list that is already in
order gives a client a second source of truth to disagree with the first.

**Membership is a field, not an event.** `Character.Folder` sits beside
`Character.Owner` and outside the log, because neither is a fact about the
character -- they are facts about the record: who it belongs to, and where its
owner filed it. Moving a character to another folder is not something that
happened to them in the fiction and has no business appearing in their history.

**Deleting a folder deletes the characters in it.** There is no undo, and
characters live in memory, so there is not even a backup behind it. A client
that offers the button owes the player a confirmation that says how many
characters are about to go; the web client's does. The cascade runs in the
usecase, not the store -- two aggregates, two stores, and a repository that
wrote to both would be two repositories sharing a name. Characters go first and
the folder last: there is no transaction across the two, so the order is chosen
for what a crash half way leaves behind. This one leaves a folder holding fewer
characters, which the application already understands. The other would leave
characters filed in a folder that no longer exists, which nothing can list.

**Why `PUT /v1/characters/{id}/folder` and not `PATCH /v1/characters/{id}`.**
The folder is the one thing about a stored character that changes without an
event. A general PATCH on the character would read as an invitation to patch a
name, a level or a score -- and the log is the only way any of those can
change. A route named after the single mutable field cannot be misread.

**Copying** is `POST /v1/characters/{id}/copy`: a new character in the same
folder unless another is named, carrying the source's whole log, with its name
suffixed `(copy)`. That rename arrives as one more appended event rather than
as an edit of the init event it was duplicated from -- otherwise the copy would
be the one record in the system that broke the log's invariant.

### Ownership, and membership

There are two authorization models here, and the difference between them is
the difference between a character and a group.

**A character belongs to one account.** The owner is resolved in exactly one
function, `handler.owner` in `internal/api/http/v1/character/handler.go`, from
the account `middleware.RequireSession` put on the request; a handler reached
without that middleware gets the zero `OwnerID`, which owns nothing, so a
mis-wiring shows up as an empty list rather than as somebody else's.

Enforcement lives in the usecase, not the handler: every read and write goes
through `Service.owned`, which refuses a character to anyone but its owner --
and refuses it as a **404, not a 403**, because a 403 on somebody else's id
confirms that the id exists and turns a guessable identifier into an
enumeration oracle.

**A folder belongs to one account too**, and is on this side of the split
rather than the membership side: it is one person's private filing, it has no
ranks, and nothing is ever shared through it. The choke point is
`Service.ownedFolder`, which is `owned` for the other aggregate down to the
refusal being a 404. That is why naming a folder you do not own returns 404
rather than an empty listing -- an empty listing would say the folder is there
and happens to be empty. Both ends of a move are checked, because without the
folder half an account could file its own character into somebody else's
folder, where it would vanish from its own listing.

**A group belongs to several people at three ranks.** `owner` > `dm` >
`player`, and every rule is a comparison between two of them. The choke point
is `Service.member` in `internal/usecase/group/service.go`, which is to a group
what `owned` is to a character: the only way any read or write reaches one, and
the only place a caller's rank is established.

| | owner | dm | player | not a member |
|---|---|---|---|---|
| read the group and its roster | yes | yes | yes | **404** |
| rename | yes | yes | 403 | **404** |
| delete | yes | 403 | 403 | **404** |
| invite (as `dm` or `player`) | yes | yes | 403 | **404** |
| remove a player or a DM | yes | yes | 403 | **404** |
| remove or demote the **owner** | **403** | 403 | 403 | **404** |
| promote or demote between dm and player | yes | 403 | 403 | **404** |
| hand the group over | yes | 403 | 403 | **404** |
| leave | **403** | yes | yes | **404** |

Two things in that table are worth saying in prose.

**404 and 403 mean different things, and the split is deliberate.** A
non-member gets 404 for the same enumeration reason a character does. A member
who lacks a right gets **403**, because they are standing in the group with the
roster on their screen -- hiding it from them would leak nothing and teach them
nothing. The predicate that decides 404 (`member`) and the one that decides 403
are different functions returning different error types, which is what stops
the two from being confused.

**Exactly one owner, always.** A group is created with one and only ever
changes owner through a transfer, which demotes the outgoing owner to `dm` in
the same step. That is why an owner may not leave: they must hand the group on
first, or delete it -- both exist, so nobody is ever trapped. The invariant is
enforced three times over, in the usecase, in the repository's statements, and
in a partial unique index (`group_members_one_owner_idx`), because the last of
those is the only one a second process racing the first obeys.

### A third way a character is reached

Sharing a character with a group is the first thing in this codebase that lets
one account read another's character, and it needed a third chokepoint rather
than a loosening of either existing one.

`character.Service.owned` asks **"is this yours"** and grants a read *and* a
write. `game.Service.readable` asks **"is it on a table you sit at"** and grants
a read and nothing else. Neither was widened to accommodate the other: they are
different functions, in different packages, over different stores, and the write
paths still go only through the first. There is no route anywhere that writes to
a character through the second, which is why "read-only" here is a property of
the API's shape rather than a rule somebody has to remember.

Both refuse with **404**, and for the reason `owned` does: a character id is a
short counter, so a 403 on one that is not yours confirms it exists. A character
that was never shared, one unshared a moment ago and one that never existed are
indistinguishable from outside.

| | its owner | group owner | dm | player | not a member |
|---|---|---|---|---|---|
| see the group's table | — | yes | yes | yes | **404** |
| read a shared sheet | yes | yes | yes | yes | **404** |
| read a character *not* shared here | yes | **404** | **404** | **404** | **404** |
| share your own character | yes | yes | yes | yes | **404** |
| share somebody else's | **404** | **404** | **404** | **404** | **404** |
| unshare | yes | yes | yes | **403** | **404** |
| edit or delete a shared character | yes | **404** | **404** | **404** | **404** |
| open, rename or delete a game | — | yes | yes | **403** | **404** |
| see a game and its roster | — | yes | yes | yes | **404** |
| seat or unseat a character | — | yes | yes | **403** | **404** |
| seat your own, not yet shared | — | yes | yes | **403** | **404** |
| seat somebody else's, not yet shared | — | **400** | **400** | **403** | **404** |

Two rows are worth saying in prose. **A player may share** — that is the whole
of what a player does at a table, and it is the half of a group that was missing
until now. **A DM may unshare somebody else's character**, which looks like a
reach into another account and is not: a guest's session expires and cannot be
recovered, so without it their character would sit on the table forever with
nobody able to take it down.

**Your character is always yours to delete.** Being on somebody's table does not
make it theirs, so nothing consults a group before agreeing — the character comes
off every table first, then out of the store. Deleting a group does the mirror
image: its games go, then its table, and the characters themselves are untouched
because they were never the group's.

Both of those cascades run through a port rather than an import:
`character.Sharing` and `group.Tables`, each one method, each satisfied by the
game service and wired in `internal/app`. The arrows point outward from the
thing being deleted, so a character still knows nothing about groups and a group
still knows nothing about games.

**Neither store is in Postgres, deliberately.** Every row on a table or a roster
names a character id, and a character id is the process-local counter — the same
argument `00003_groups.sql` makes for why the groups schema refuses to name one.
So a group and its members survive a restart and the characters shared with it
do not. That split is surprising and it is the price of having the feature
before characters are durable; the two move to Postgres together or not at all.

Invitations are stateless. A link is a signed token naming a group and a rank,
valid for 24 hours, **reusable and not revocable** -- there is no invites table
and nothing to revoke against. The trade is written down beside the type in
`internal/domain/group/invite.go`; the upgrade, if it ever stops being
acceptable, is a stored invite whose id rides in the token.

One trap worth naming, because it is invisible until somebody hits it: every
token port reports failure as a `*types.UnauthenticatedError`, which renders as
**401**, and a 401 is what tells the client its session is gone. A stale invite
link must therefore *not* surface as one, or clicking yesterday's invitation
would sign out the perfectly signed-in person who clicked it. `openInvite`
translates it into a `*types.ValidationError` -- a 400 -- and there is a test
for it.

### Dependency rule

```
cmd/easydnd -> internal/app -+-> internal/api/http/**        (inbound adapter)
                             +-> internal/adapter/repository (outbound adapter)
                                        |
                          both depend inward on
                                        v
                             internal/usecase/**  ->  internal/domain/**  ->  internal/types
```

Imports point inward, never outward:

- **`internal/domain/**`** imports the standard library only. No gin, no
  `net/http`, no `database/sql`, and no JSON or database struct tags --
  serialization and persistence belong to adapters. The domain packages may
  import each other, one way only: `character` reads `catalog`, both read
  `rules`, `group` reads `user`, and `game` reads `character`, `group` and
  `user` — it is the one package that exists because two aggregates meet, and
  it is the only one that names more than one of them. Nothing points back, so
  a character still knows nothing about who is allowed to look at it.
- **`internal/usecase/**`** imports the domain and `internal/types`. It never
  sees a `*gin.Context`; handlers pass `c.Request.Context()` and plain values
  inward.
- **`internal/types`** carries no HTTP status codes. The error-to-status table
  exists exactly once, in `internal/api/http/helpers/errors.go`. That is what
  makes `types` safe for the domain to import.
- **`internal/app`** is the only package importing across all layers.

Two mechanical checks back this up: `make lint/layers` greps the dependency
graph of the inner layers, and a `depguard` rule in `.golangci.yml` denies the
same imports at lint time.

The frontend has its own layer rule and its own checker; see
[web.md](web.md#dependency-rule).

### Package naming conventions

| Situation | Convention |
|---|---|
| `internal/api/http` | package is named `httpapi`, so files can import `net/http` unaliased |
| domain packages | the aggregate is imported as `domain` (e.g. `domain "…/internal/domain/character"`); `rules` and `catalog` keep their own names |
| usecase packages | imported as `<aggregate>uc` (e.g. `charuc "…/internal/usecase/character"`) |
| handler files | one exported handler per file, named after the action |
| request DTOs | `<Action>Params`, declared beside their handler |

## Configuration

All configuration lives in **one YAML file**. The app finds it via the
`EASYDND_CONFIG` environment variable, or a `-config <path>` flag which takes
precedence; there is no default location and the file is **mandatory in every
environment**. `EASYDND_CONFIG` is the only environment variable the app reads —
individual settings cannot be overridden from the environment, so what the file
says is what the process runs.

`config.dev.yaml` is committed and is what `make run/server` loads.
`deploy/config.example.yaml` is the production template; the live copy is
installed by hand, once:

```sh
sudo install -d -o root -g easydnd -m 751 /etc/easydnd
sudo install -o root -g easydnd -m 640 deploy/config.example.yaml /etc/easydnd/config.yaml
sudo -e /etc/easydnd/config.yaml        # fill in auth.session_secret
```

Mode `640 root:easydnd` is the point: the service account can read the signing
key and nothing else on the host can. The app warns at startup if the file is
world-readable, and logs which file it loaded.

The directory is `751`, not `750`: `deploy/deploy.sh` runs as `deploy` and
checks this file exists before swapping the release symlink, and testing a path
needs execute permission on every parent directory. A `750` directory fails that
check with `EACCES` while the file itself is perfectly fine — which is how the
v0.5.0 deploy failed. `751` grants others `--x`, so the path can be traversed
but the directory cannot be listed and the `640` file still cannot be read.

Every key is optional — the defaults below apply to anything the file omits —
but an **unknown key is a startup error**, so `rp_origin` for `rp_origins` fails
loudly instead of silently leaving production on a default nobody chose.
Malformed values (a bad duration, an unknown log level) are likewise fatal
rather than quietly defaulted.

| Key | Default | Notes |
|---|---|---|
| `env` | `production` | `development` or `production`; drives gin mode |
| `http.host` | `127.0.0.1` | loopback on purpose -- a reverse proxy fronts the API |
| `http.port` | `"8080"` | a string; must match `deploy/deploy.sh`'s health check and the nginx `proxy_pass` |
| `http.read_timeout` | `10s` | |
| `http.read_header_timeout` | `5s` | |
| `http.write_timeout` | `15s` | |
| `http.idle_timeout` | `60s` | |
| `http.shutdown_timeout` | `5s` | must stay below supervisor's `stopwaitsecs` (default 10s) |
| `http.max_header_bytes` | `1048576` | |
| `http.trusted_proxies` | `[127.0.0.1, "::1"]` | gin trusts `0.0.0.0/0` by default; narrowed here |
| `log.level` | `info` | `debug`, `info`, `warn`, `error` |
| `log.format` | `json` | `json` or `text` |
| `data.srd_dir` | `data/srd_5.1` | read at startup; a missing or malformed directory is a fatal error, by design. Absolute in production, through `current/` so it follows the symlink swap |
| `db.url` | *(none)* | **required in production**; libpq URL for the account store. Say `sslmode=verify-full` -- an omitted `sslmode` means libpq's `prefer`, which is unauthenticated and permits a plaintext fallback. Unset in development falls back to the in-memory store with a warning. The example file's placeholder password is rejected by name |
| `db.max_conns` | `10` | pgxpool size |
| `db.connect_timeout` | `5s` | bounds the startup ping; must fit inside `deploy.sh`'s 15s health gate alongside migrating and binding |
| `db.migrate_on_start` | `true` | apply pending migrations before the listener binds. Set `false` only to stage a migration by hand with `easydnd -migrate=up` |
| `auth.session_secret` | *(none)* | **required in production**; signs the session cookie. `openssl rand -base64 48`, quoted. Read as base64, taken literally if it is not valid base64; must decode to at least 32 bytes. The example file's placeholder is rejected by name |
| `auth.rp_id` | `easydnd.org` / `localhost` | **a one-way door** -- see below. `localhost` in development |
| `auth.rp_name` | `easydnd` | what the operating system's passkey prompt calls us |
| `auth.rp_origins` | `[https://easydnd.org]` / `[http://localhost:5173]` | a list; entries carry scheme and port, unlike the RP id. Also the CSRF allow-list: `middleware.SameOrigin` compares the `Origin` header on every non-safe request against it, so an instance reached on any origin not listed here rejects every write |
| `auth.session_ttl` | `168h` | how long a session cookie lasts |
| `auth.guest_session_ttl` | `24h` | how long an anonymous session lasts. Deliberately its own key, and shorter: a guest token cannot be revoked and names nothing recoverable, so the only thing bounding a leaked one is how soon it expires |
| `auth.ceremony_ttl` | `5m` | how long a begin/finish pair stays valid; also bounds an in-flight SSO redirect |
| `auth.google.client_id` | *(none)* | omitting the whole `auth.google` block means Google sign-in is **not offered**, which is a supported deployment |
| `auth.google.client_secret` | *(none)* | must be set together with the id; half a configuration is a startup error. The example file's placeholder is rejected by name |
| `auth.google.redirect_url` | `https://easydnd.org/v1/auth/sso/google/callback` / `http://localhost:5173/v1/auth/sso/google/callback` | must match a URI registered with Google byte for byte. The development default is the **Vite dev server**, not this process |

Cookie `Secure` and the `__Host-` / `__Secure-` name prefixes are derived from
`env`, not configured: the Vite dev server is plain HTTP, and a `Secure`
cookie there is simply never sent. In development an unset
`auth.session_secret` generates one for that process and logs a warning;
production refuses to start without it — which is why `config.dev.yaml` carries
no secret at all.

`-version` is handled before the config is loaded, so `./easydnd -version` works
in CI where no config file exists.

## Authentication

Three ways in, one account model. There is no password and no email of our own.

- **Passkeys.** A visitor signs in with a fingerprint, a face or a device PIN,
  and the browser picks the passkey -- sign-in asks for nothing at all, because
  the credential is *discoverable* and carries the account handle on the
  authenticator. **Sign-up asks for nothing either.** `register/begin` takes no
  body; the account id is minted here, and the display name the operating
  system's passkey prompt needs to label the passkey with is the fixed word
  **`easydnd`** -- every passkey account is called that. The label's job is to
  say which site the passkey opens when somebody scrolls their credential
  manager months later, and a name invented per account answered a different
  question. Sharing one name costs nothing: `users.display_name` is neither
  unique nor indexed, and no lookup anywhere goes through it. So there is no
  username, no email and no client-supplied text anywhere in this API's auth
  surface: every string in `users.display_name` is either that constant or a
  provider's claim. See `PasskeyDisplayName` in `internal/usecase/auth` -- a
  constant rather than the configured `auth.rp_name`, because the usecase layer
  imports no configuration and `make lint/layers` enforces it.
- **Google**, over OpenID Connect. Optional configuration: with no client id
  and secret the provider is simply not offered, and everything else works
  unchanged.
- **A guest session**, which has no account behind it at all. See
  [Anonymous sessions](#anonymous-sessions) below.

The first two are properties of an account, and it carries both --
`Credentials []Credential` and `Identities []Identity` in
`internal/domain/user`. Either signs its owner in, which is the closest thing
to account recovery this design has, and it is why linking exists. A guest has
neither, so it has no way in to lose and nothing to link.

**An account's passkeys are fixed at sign-up.** There is no endpoint that adds
one to an existing account: `Repository` can `Create` an account with its
initial credentials and `TouchCredential` one that has just been used, and that
is the whole of it. Redundancy therefore comes from linking a provider, not
from a second passkey -- see [No recovery](#no-recovery).

**Registration cannot exclude the passkeys you already have, and this is
load-bearing.** `BeginRegistration` builds `excludeCredentials` from the
candidate account's own credentials, and the candidate is brand new, so that
list is always empty. It cannot be anything else: excluding somebody's existing
passkeys means knowing whose they are, and identifying them is precisely what
just failed -- the client falls back to registration exactly when the sign-in
picker came back empty-handed. The consequence is that a visitor who dismisses
a picker listing their own account is offered a new account rather than told
they already have one, and both passkeys keep working afterwards with no way to
merge them. What makes this cheap rather than alarming is that
**`register/begin` stores nothing**: the candidate rides inside the sealed
ceremony cookie and reaches `repo.Create` only once an attestation verifies, so
an abandoned fallback leaves no record at all. The browser half of this bargain
is in [web.md](web.md#one-button-means-both-halves).

| Route | Guard | Does |
|---|---|---|
| `POST /v1/auth/register/begin` | none | creation options for a new, server-named account; no body; sets the ceremony cookie |
| `POST /v1/auth/register/finish` | ceremony cookie | verifies, stores the account, sets the session cookie |
| `POST /v1/auth/login/begin` | none | request options; no body, names no account |
| `POST /v1/auth/login/finish` | ceremony cookie | verifies and sets the session cookie |
| `POST /v1/auth/anonymous` | none | issues a guest session; stores nothing |
| `POST /v1/auth/logout` | none | clears the session cookie |
| `GET /v1/auth/providers` | none | which external buttons to draw |
| `GET /v1/auth/sso/{provider}/start` | none | 302 to the provider; sets the flight cookie |
| `GET /v1/auth/sso/{provider}/callback` | flight cookie | exchanges the code and sets the session cookie |
| `GET /v1/auth/sso/{provider}/link` | session | same, but attaches to the signed-in account |
| `POST /v1/auth/sso/{provider}/unlink` | session | disconnects an external account |
| `GET /v1/auth/me` | session | the signed-in account, or 401 |

### Sign in with Google

Authorization Code + PKCE, run **server-side**. The browser leaves for Google
as a top-level navigation and comes back to `/callback`; no Google JavaScript
is loaded, the client secret never leaves the process, and the frontend gained
zero dependencies -- the button is a link.

```
GET /v1/auth/sso/google/start
    mint state, nonce, PKCE verifier, returnTo -> Signer.Seal -> flight cookie
    302 -> accounts.google.com

GET /v1/auth/sso/google/callback?code=&state=
    open the flight, constant-time state compare
    exchange the code, verify the ID token and its nonce
    resolve the account, set the session cookie
    302 -> returnTo   (or /?auth_error=<code>)
```

Three things about it are load-bearing and quiet when wrong:

1. **The flight cookie is `SameSite=Lax`, and must stay that way.** The
   callback is a top-level GET arriving from `accounts.google.com` -- cross-site
   by every definition a browser uses. Lax is sent on exactly that; `Strict`,
   which the ceremony cookie beside it uses quite correctly, is withheld.
   "Tidying" the two to match would break every Google sign-in with *no sign-in
   is in progress* and nothing in the log to say why. There is a test named for
   this.
2. **`middleware.SameOrigin` exempts safe methods**, so both routes pass the
   CSRF guard as GETs. `state` and PKCE are therefore not decoration -- they
   are the only thing binding the callback to the attempt that started it.
3. **The failure path is a redirect, not an error body.** The API has no HTML,
   and JSON at the end of a top-level navigation would replace the application
   with a page of braces. Failures land on `/?auth_error=<code>`, and the code
   is looked up in a table in the client rather than rendered: text taken from
   a query parameter is a way to put chosen words on somebody else's page.

The `returnTo` path rides inside the **sealed** cookie, never in the query
string, and is still re-validated as a site-relative path on the way out.

**Accounts are matched by the provider's subject, never by email.** An address
can be released and reassigned, and a passkey account has no email to match
against anyway. So a Google sign-in resolves to an existing account only if
that exact subject was linked before; otherwise it creates one.

### Linking

Connecting Google to an existing account is a deliberate act from `/account`,
never a guess made at sign-in time. `/link` is guarded, and it seals *whose*
account into the flight -- deciding that at the callback instead, from whichever
session happened to be open, would let a stray sign-in absorb somebody's Google
account. The callback additionally requires the live session to be that same
account.

A subject already linked to a **different** account is refused rather than
moved. Moving one is an explicit unlink and relink.

Unlinking **refuses to remove the last way in**. An account with no passkey and
no identity can never be signed into again and nothing here can restore it, so
`Service.Unlink` checks `SignInMethods() > 1` first. That rule lives in the
usecase rather than the repository because it spans both kinds of proof.

Five direct dependencies arrive with this: `go-webauthn/webauthn` (the only
maintained Go relying-party implementation), `golang-jwt/jwt/v5` (already in
go-webauthn's own graph, so free), `coreos/go-oidc/v3` and `x/oauth2` for the
Google exchange, and `fxamacker/cbor/v2`, used **only by tests** -- `internal/adapter/webauthn/roundtrip_test.go` builds a software
authenticator with real ES256 keys and drives a full register-then-sign-in
against the real library, which is the one test that would notice the adapter
agreeing with itself but not with the specification.

### Anonymous sessions

A guest session is the same signed token in the same `HttpOnly` cookie as any
other, carrying one extra private claim, `anon`. It rides in the token because
there is nothing to look it up in.

A guest used to have no row anywhere at all. **Groups ended that, but only
just**: a guest who joins somebody else's table has to be nameable in a roster
other people read, and `group_members.user_id` is a real foreign key. So the
group usecase writes a `users` row for a guest the first time they create or
join a group -- `EnsureGuest`, idempotent, and called on those two paths and
nowhere else. A guest who never touches a group is still stored nowhere.

Which raised the question of what to call them. "Guest" was legible while a
guest could only see their own things and useless the moment three shared a
roster: nobody could tell which one to remove. A guest is now "Guest" plus four
characters of the id they already carry -- `guestName`, a pure function of the
session's subject.

Derived rather than stored or claimed, which is the whole point: there is no
extra claim to add, no row to keep in sync, no migration, and every cookie ever
issued renders correctly through it. It is also the same judgement
`PasskeyDisplayName` makes in the other direction -- a name should answer the
question actually being asked. On a roster that question is "which of these
people", and four characters answer it; an invented two-word name would answer
"who are they", which a session with no account behind it cannot honestly claim
to know.

The `anon` claim is load-bearing in exactly one place. `Session()` -- the usecase
behind `RequireSession`, and the only database read on the authenticated
request path -- short-circuits on it and rebuilds the identity from the claims
instead of calling `repo.ByID`. Without that, every guest request would answer
401 "session no longer identifies an account", which is what the account path
correctly reports for a token naming a deleted account and exactly the wrong
thing to say about a session working as designed.

Guest ids carry an `anon:` prefix. The prefix is *not* what makes a session
anonymous -- the token says that, and the token is the authority -- so forging
the prefix into an account id buys nothing. What it buys us is that `:` sits
outside the base64url alphabet `newUserID` draws from, so the two id spaces
cannot collide even by accident.

Everything downstream then works unchanged, because nothing downstream reads
the account store: the character handlers use only the owner id, and the
catalog handlers ignore the user entirely. A guest therefore owns characters
with no schema change. They live in the in-memory character store and die with
the process, which is honest for a session that cannot be signed back into.

Two consequences worth stating plainly:

- **A guest cannot become an account.** There is no conversion path, in either
  direction: the row `EnsureGuest` writes carries no credential and no identity,
  and there is no method that would add one. It is an account nobody can ever
  sign in to. Every surface that shows a guest session is obliged to say that
  nothing is being kept.
- **A guest can own a group, and that is a known hazard.** A guest id is minted
  fresh per sign-in and expires with the session, so a guest who creates a group
  owns it permanently and stops existing within a day. Nobody can then delete
  that group or hand it on -- only an owner may, and the owner is unreachable.
  The intended fix is a **scheduled job that reaps guest rows and everything
  they own once their session lifetime has passed; it is not implemented**.
  Until it is, orphaned groups accumulate and only a hand-written statement
  removes one.
- **`POST /v1/auth/anonymous` is unauthenticated and has no rate limit.**
  Signing is cheap and stateless, so the tokens are not the concern; the
  character store each one can then fill is bounded by nothing. There is no
  rate limiter anywhere in this service yet. When one arrives, this route wants
  it first.

### Why the layers look the way they do

`go-webauthn`, `golang-jwt`, `go-oidc` and `x/oauth2` all reach `net/http`,
which `depguard` and `make lint/layers` forbid in `internal/domain` and
`internal/usecase` -- and `lint/layers` is transitive, so a library three hops
from `net/http` still trips it. All of them therefore sit behind ports declared
in `internal/domain/auth` -- `Ceremony`, `Signer`, `Federation` -- and are
implemented under `internal/adapter`. The application layer trades in plain
strings, bytes and domain types and never sees a protocol type.

`Federation` is one interface per provider rather than one with a provider
argument, so endpoints and credentials are settled when the adapter is built
instead of being re-decided per call. The usecase holds a map of them, and an
unconfigured provider is simply absent from it.

OIDC discovery is **lazy**, behind a `sync.Once` in the adapter. Doing it in
`app.New` would make the process refuse to boot whenever `accounts.google.com`
was unreachable -- which `deploy.sh`'s health gate would read as a bad release
and roll back.

### Sessions

The session is a stateless HS256 JWT in an `HttpOnly` cookie. Nothing
server-side records that it exists, which has two consequences worth stating
plainly:

- **Logging out clears the cookie and nothing else.** A token someone already
  captured stays valid until it expires.
- **Rotating `auth.session_secret` is the only revocation lever**, and it
  revokes everything at once. It is unusually cheap here: no passkey is lost,
  because credentials live in the account store rather than in the token, and
  everyone simply clicks "Sign in" again.

The in-flight ceremony rides in a second short-lived cookie carrying the
sealed challenge, so a begin/finish pair needs no server-side map either. An
in-flight Google redirect rides in a third, on the same `Seal`/`Open`
primitive, for the same reason. Each envelope names its own kind and refuses
the other's, so a value minted by one flow cannot be fed to the other's finish
endpoint and land somewhere surprising.

An **invite link is a fourth kind**, signed by the same key from the same
adapter. That is safe only because of the kind claim, and this is the sharpest
illustration of why it exists: without it, the session cookie every signed-in
visitor already holds would verify perfectly well as an invitation to any group
whose id they could guess -- and an invite link, which is meant to be forwarded
to strangers, would verify as somebody's session. `internal/adapter/token` is
still the only package that knows any of these are JWTs; the group usecase
sees an `Inviter` port trading in strings and domain types.

CSRF is covered three ways, in `middleware.SameOrigin`: `SameSite` on the
cookie, an `Origin` check against `auth.rp_origins`, and a required
`X-Request-Id` header -- which `web/src/lib/api/client.ts` already sends on
every call, and which an HTML form cannot set at all.

That `Origin` check is why `auth.rp_origins` is not only a passkey setting. It
is the list of addresses this instance will accept a write from, so a
development instance reached on some other host has to have that host in it or
every POST comes back "request origin is not allowed" -- which is what
`make dev` generates it for.

### `auth.rp_id` is permanent

The relying-party id is burned into every passkey when it is created. Changing
it orphans all of them with no migration path. It is the **apex** domain on
purpose: a passkey registered against `easydnd.org` keeps working on a future
`app.easydnd.org`, and the reverse is impossible. The `www` → apex redirect in
`deploy/nginx/easydnd.conf` is what keeps this true, and is why the session
cookie can use the `__Host-` prefix.

Development uses `localhost`, so a passkey made in development will never work
in production, and vice versa. That is two disjoint identities, and it is
correct.

### Where accounts and groups live

Accounts, their passkeys, their linked external identities and the groups they
play in are stored in PostgreSQL -- AWS RDS in production -- by
`internal/adapter/repository/postgres`. Five tables:

| Table | Holds |
|---|---|
| `users` | the account id, display name and creation time |
| `user_credentials` | one row per registered passkey |
| `user_identities` | one row per linked external account |
| `groups` | a group's id, name and who made it |
| `group_members` | one row per seat: who, in which group, at which rank |

Characters are **not** among them. They still live in the in-memory store and
die with the process, which is why nothing in `groups` refers to one: a
character id is a process-local counter, so a foreign key to it would be
dangling by the next restart, and a schema written against an unfinished
feature is a migration nobody can revise later.

`users` is the only place a display name is stored, and a roster is a join
rather than a copy -- so a rename shows up in every group at once or in none.
That is also why a guest gets a row there when they join something: see
[Anonymous sessions](#anonymous-sessions).

`group_members` keys on `(group_id, user_id)`, which makes "a person is in a
group at most once" the database's rule and gives the roster read its index.
`created_by` on `groups` is **history, not authority**: ownership lives in the
member rows and moves when the group is handed on, so nothing may consult that
column to decide what anybody is allowed to do.

The one-owner rule is a **partial unique index**, `group_members_one_owner_idx`
`ON group_members (group_id) WHERE role = 'owner'`. It forces the order of a
transfer and this is easy to get wrong: a unique index is checked as each
statement runs and cannot be deferred to commit, so a transfer must **demote
the outgoing owner first and promote the incoming one second**. The
intermediate state is then zero owners, which the index permits; the other
order is two, which it rejects. The in-memory adapter writes them in the same
order deliberately, so that it cannot pass a test the real one fails.

The credential id is the **primary key** of `user_credentials` rather than a
surrogate. That is what gives `ByCredentialID` -- the lookup every usernameless
sign-in makes -- its index, and it enforces "a credential belongs to exactly one
account" in the database rather than in a map only one process can see.

`user_identities` keys on `(provider, subject)` for the same two reasons, and it
is **composite** because a subject is only unique within its issuer: keyed on
the subject alone, one provider's subject could resolve to an account linked
through another, which is a sign-in as the wrong person. `email` is stored but
is deliberately not unique and never a lookup key -- an address can be released
and reassigned, so matching on one would eventually hand somebody else's characters
to a stranger.

`sign_count` is a `bigint` because the domain's `SignCount` is a `uint32` and
Postgres has no unsigned types; a `CHECK` keeps it inside the range so the
narrowing cast on the read path cannot wrap. Times are `timestamptz` -- a bare
`timestamp` would be reinterpreted on a server whose zone differs from the
writer's. Note that `timestamptz` is microsecond-precision and pgx decodes into
the local zone, so **compare stored times with `time.Time.Equal`, never `==` or
`reflect.DeepEqual`.**

#### Two adapters, one contract

`user.Repository` still has two implementations. The in-memory one is the
development fallback: with no `db.url` the server logs a warning and runs
on it, so `make run/server`, `go test ./...` and `make verify` all work with no
Postgres installed. `config.validate` refuses that combination in production.

Both run the same test suite, `internal/adapter/repository/repotest`. That is
not tidiness. `internal/api/http/helpers` maps a `*types.ValidationError` to 400
and a `*types.NotFoundError` to 404 exactly once, so two implementations that
disagree about which error a bad call produces are two different ports wearing
one name -- and only one of them can be right.

The sharpest instance: Postgres evaluates a unique constraint **before** it
fires a foreign key trigger. `AddCredential` against a missing account whose
credential id is already claimed therefore reports the duplicate, where the
in-memory adapter reports the missing account -- so the SQL adapter probes for
the account explicitly first. `repotest` has a case for it.

### Migrations

The schema ships inside the binary. `internal/adapter/repository/postgres/migrations`
embeds numbered `.sql` files and [goose][goose] applies them **at startup,
before the listener binds**, holding a Postgres advisory lock so two processes
racing `up` cannot both run the same migration.

There is no migration step in the deploy, and nothing to forget. A failed
migration fails startup, the health gate in `deploy.sh` never sees the new
release answer, and the previous release is restored automatically.

> **Migrations must be expand-only.** That rollback runs against the schema
> that was just applied, so the *previous* binary has to work on it. Add tables
> and nullable columns; never drop or rename a column in the same release as the
> code that stops using it. Split a rename into "add, backfill, dual-write" and
> "drop", two releases apart.

Never renumber or delete a migration once it has been applied -- goose errors on
a database version it cannot find in the embedded files.

`easydnd -migrate=status|up|down` runs one command and exits without starting
the server, for the operator who set `DB_MIGRATE_ON_START=false` to stage a
risky change. `-migrate=down` in production additionally requires
`-migrate-force`: it drops passkeys, and a passkey cannot be reissued.

[goose]: https://github.com/pressly/goose

### Connecting to RDS

`db.url` is a libpq URL and **should say `sslmode=verify-full`**. The
Amazon RDS CA bundle is compiled into the binary, so there is no certificate
file to ship alongside a release -- which matters because `deploy.sh` swaps
releases by symlink and prunes old ones.

Two pgx defaults are worth knowing, because the adapter exists partly to undo
them:

- An **omitted** `sslmode` means `prefer`, which sets `InsecureSkipVerify` *and*
  appends a plaintext fallback. The adapter upgrades that to `verify-full` and
  clears the fallback.
- `sslmode=verify-full` on its own leaves the root pool empty, meaning the
  system store -- which does not carry the Amazon RDS CAs, so it does not merely
  fail to protect anything, it fails to connect. The embedded bundle is what
  makes it work.

An explicit `sslrootcert=` is left alone, and `sslmode=disable` is honoured so a
local container works.

### No recovery

There is still no reset link and no support address. What exists is redundancy:
a linked Google account beside the passkey the account was created with. The
service has no way to add a passkey to an existing account, so linking is the
only redundancy on offer, and `/account` is where both inventories live and
where linking happens.

Unlinking still refuses to remove the last way in. Losing that one way in --
every device holding the passkey, or the Google account itself -- loses the
easydnd account permanently.

## Deployment

One pipeline ships the API, the SRD data and the frontend together, and it is
**driven entirely by tags: a push to `main` runs nothing at all.**

| Event | Runs |
|---|---|
| push to `main` | *nothing* -- no build, no tests |
| push a `v*` tag | gofmt, vet, tests, build, version-injection check, then deploy |
| push a `v*-notest` tag | the same minus the two suites; the version assertions still run |
| push a `ci/*` tag | everything except Deploy and Restart -- a dry run that cannot ship |
| manual run on a `v*` tag | the same -- how you re-run a tag that failed halfway |
| manual run on a branch | builds and tests, but will not deploy |

Because nothing runs on `main`, **`make verify` locally before tagging is the
only thing standing between a mistake and a tagged release.**

`-notest` is for the release where you have just run it. Paying for the same
checks twice buys nothing, so a tag named for the exemption skips both test
jobs. Naming the tag is the whole mechanism: the workflow reads it off the ref,
and `git tag` therefore shows for ever which releases went out untested.

What it gives up is the two suites, and now nothing else. The pair of version
assertions -- that `./easydnd -version` and the bundle's `version.json` both
report this release -- used to live in the Test stage and go with it. They are
in Build now, beside the artifact each is about, so a `-notest` release still
proves the identifier landed. That matters more than it sounds: an unfound `-X`
symbol is a *silent* no-op, and without the assertion the same mistake still
gets caught, but by `deploy.sh`'s health gate -- a rollback and a red `restart`
job minutes later, with nothing on the run page saying why.

The suffix reaches one further thing, and only in what the release calls itself:
the identifier is the tag, so `v1.0.4-notest` ships reporting `v1.0.4-notest`.
Left alone rather than trimmed back, because a release that skipped its suites
should say so wherever anyone reads its version. Every path on the server is
still keyed by the SHA, so a `-notest` release is byte-identical to any other
where it is stored.

To release:

```sh
git tag v0.1.0
git push origin v0.1.0
```

That builds a static `linux/amd64` binary, a `web.tar.gz` of the frontend and a
tarball of `data/srd_5.1/`, ships all three into `/opt/easydnd/releases/<sha>/`,
and runs `deploy/deploy.sh`: unpack the bundle, atomic symlink swap, supervisor
restart, health gate, automatic rollback on failure, prune to the last 5
releases.

```
/opt/easydnd/releases/<sha>/easydnd        supervisor runs this
/opt/easydnd/releases/<sha>/data/srd_5.1/  the API reads this at startup
/opt/easydnd/releases/<sha>/web/           nginx serves this
/opt/easydnd/releases/<sha>/VERSION        what this release calls itself
/opt/easydnd/current -> releases/<sha>     all four follow this symlink
```

A release is therefore a **directory, not a file** -- the compendium is read
from disk rather than embedded, and the frontend is static files. `deploy.sh`
checks for all three before the swap, so a partial upload fails *before* going
live rather than after. That ordering matters most for the frontend: nginx
serves `current/web`, so a bundle that unpacked badly would go live as a blank
site that the API-side health gate cannot see and will not roll back.

Because all three sit behind one symlink they swap together, so a rollback
reverts the UI, the API and its data as a unit.

The database is the exception, and the only piece of state that does **not**
swap with a release. That is what makes the expand-only rule above binding: a
rollback puts the previous binary in front of the schema the failed release
applied.

### Exercising the pipeline without shipping

```sh
git tag ci/whatever && git push origin ci/whatever
```

That runs Check, Build and Test exactly as a release does and stops there.
Nothing in the workflow was taught about it: Deploy and Restart already ask
`startsWith(github.ref, 'refs/tags/v')`, and a `ci/` ref fails that, so they skip
on their own.

It works only because the trigger and the deploy gate stopped being the same
condition. They both used to say "starts with `v`" -- the trigger as the glob
`v*`, the gate as `startsWith(…, 'refs/tags/v')` -- so every tag that could
start the pipeline could also ship from it, and there was no way to run CI on a
tag without a release at the end of it.

**A dry-run tag must not begin with `v`.** `v*` is a glob and not a version
pattern: `vtest` and `verify` both match the trigger *and* pass the deploy gate,
so either would ship whatever it points at to easydnd.org. `ci/` cannot be
mistyped into that, which is the whole reason for the prefix.

A dry run also gets its own concurrency lane rather than sharing `deploy`, so a
test tag can never hold a real release in the queue behind it.

What it does **not** prove is the deploy gating itself. `deploy-backend` skips
here for the ref, not because it weighed `check-backend`'s result -- so a `ci/`
run says nothing about whether a red Check would stop a release. Only a real
`v*` tag exercises that clause, which is why it is worth reading rather than
testing.

### The three checks run at once, and nothing is cached

Check, Build and Test are six jobs with no dependencies between them -- three
per lane, all starting together. They used to be a chain per lane, and the chain
cost more than its contents: the Go module graph was compiled from scratch in
each of the three backend jobs, one after another, 31s then 38s then 39s of a
202s release. Nothing in Check produces anything Build or Test reads. The only
real tie was the version assertions, which needed a built artifact -- so they
moved into the job that builds it.

The one thing that arrangement costs is that **Check no longer gates anything by
being upstream of it.** The two Deploy jobs name `check-*` in `needs` and test
its result explicitly. They have to: their `if` already lifts the implicit
`success()` gate so that `-notest` can work, so a missing clause there would not
fail loudly -- it would ship a release whose gofmt, vet, layer and drift checks
were red.

**Nothing is cached, and that is not an oversight.** A GitHub Actions cache is
readable only from the ref that wrote it or from the default branch. This
workflow runs on `v*` tags and nothing else, so every run was a new ref: it
missed, wrote ~120 MB under its own tag, and left it for nobody. Twenty-five
entries and 2.9 GB had accumulated without a single read; the logs said `Cache
is not found` on every run sampled. Switching it back on would take something
running on `main` so the cache lands on the default branch, and by the rule at
the top of this section nothing runs on `main`. Until that trade is made
deliberately, `cache: false` is the honest setting and saves the upload.

### Provisioning the database

The binary migrates its own schema but cannot create the instance it migrates.
Once, by hand:

1. Create an RDS **PostgreSQL 16 or 17** instance. `db.t4g.micro` is ample.
2. The VPS is outside AWS, so it needs **Publicly accessible = Yes** -- and then
   a security group with a *single* inbound rule, TCP 5432 from the VPS public
   IPv4 as a `/32`. "Publicly accessible" plus a permissive security group is
   how account databases end up on the internet.
3. Keep the default certificate authority; it is in the bundle compiled into the
   binary. Note its expiry somewhere: refreshing that bundle is a code change.
4. Attach a parameter group with **`rds.force_ssl = 1`**, so the server refuses
   plaintext and a mis-set `sslmode` fails loudly instead of downgrading.
5. Create the database and a non-superuser role. Postgres 15 and later revoke
   `CREATE` on `public` from `PUBLIC`, and goose needs to create both its
   version table and ours:

   ```sql
   CREATE DATABASE easydnd;
   CREATE ROLE easydnd LOGIN PASSWORD '<generated>';
   GRANT CONNECT ON DATABASE easydnd TO easydnd;
   \c easydnd
   ALTER SCHEMA public OWNER TO easydnd;
   GRANT CREATE, USAGE ON SCHEMA public TO easydnd;
   ```

6. **Turn on automated backups.** This is the actual durability guarantee, not
   the schema. There is no password, no email and no account recovery, so a lost
   `users` table orphans every passkey in every user's password manager
   permanently.
7. Put `db.url` in `/etc/easydnd/config.yaml`, alongside `auth.session_secret`.
   That file is `640 root:easydnd` precisely because it now holds two
   credentials. **Install it before deploying the release that needs it** --
   the new binary refuses to start without `db.url`, which the health gate
   would turn into a rollback. Editing it while the old release is live is
   safe: the old binary reads the same file and simply ignores a `db:` section
   it does not know.

Two server-side configs are applied **by hand**, not by the pipeline, and both
are mirrored in the repo as the source of truth for what the live copy should
say:

- `deploy/supervisor/easydnd.conf` -- the API process. It must set
  `data.srd_dir` to an absolute path under `current/`, because the config
  default is *relative* and resolves against `directory=`, not against the
  release. Getting this wrong costs a deploy: the binary dies on every restart
  with `load SRD data from data/srd_5.1: ... no such file or directory`, the
  health gate times out, and the release rolls back. It is written that way in
  the committed copy for exactly that reason.
- `deploy/nginx/easydnd.conf` -- the routing: `/v1/` to the Go process,
  everything else to the bundle with an SPA fallback, plus the whole of the
  HTTP caching policy. Its cache rules changed with the release-identifier
  work -- `/icons/`, `/manifest.webmanifest`, `/favicon.svg`, the Workbox
  runtime chunk and the source maps -- and **a tag deploy does not carry any of
  that**. The live copy has to be replaced by hand or the new rules are simply
  not in effect, silently, with nothing failing to say so.

> **Apply the nginx config before the first tagged deploy carrying a
> frontend.** Until nginx serves `/opt/easydnd/current/web`, the workflow's
> public `version.json` check has nothing to read and the deploy job fails --
> after the release has already activated successfully, which reads as a far
> more alarming failure than it is. Applying it also means adding `www-data`
> to the `easydnd` group and *restarting* nginx; see the header of
> `deploy/nginx/easydnd.conf`.

### What a release is called, and where it lives

Two different things, and keeping them apart is what makes the rest of this
section work.

**A release is named by its tag; being in the pipeline is what earns one.**
`deploy/release-version.sh` is the only place that decides: a `refs/tags/v*`
push reports its tag (`v1.0.4`), and **everything else reports a short commit
SHA** -- a `ci/*` dry run, a `workflow_dispatch` from a branch, `make dev`,
`make verify`, a hand-run `make build/release`. One script, called by the
`VERSION` variable in the `Makefile` and by every job in the workflow that needs
the answer, because the binary, the bundle and the four checks on them all
compare this string and a second opinion would be a bug rather than a nuance.

It deliberately does **not** ask `git describe` whether the local commit happens
to carry a tag. Building `v1.0.4`'s commit on a laptop reports `c15fdec`, not
`v1.0.4`, and that is the point: the working tree may differ from the tag,
nothing has been through CI, and a build that answers `v1.0.4` while being none
of the things a release is makes every later bug report ambiguous. `GITHUB_REF`
is how the script knows the difference. It also sidesteps the fact that a CI
checkout is shallow and cannot be relied on to have the tags at all.

So a dev build says what it honestly is -- the commit it came from. `make dev`
passes the same value to the Vite dev server, so the footer reads `c15fdec`
rather than the word "dev", and the bundle and the API it talks to agree. They
have to agree: disagreeing is exactly what opens the update dialog.

A `v*-notest` tag reports itself in full, `-notest` and all. That is deliberate:
a release that skipped its suites should say so wherever anyone reads its
version.

**A release lives in a directory named by commit SHA.** `releases/<sha>/`, as it
always has. A tag can be moved; a commit cannot, so the SHA is what guarantees
two builds never land on top of each other. On a tag push `GITHUB_SHA` is the
commit the tag points at, so nothing about the directory layout changed.

Because those two are no longer the same string, `deploy.sh` cannot derive the
one from the other, and it needs the identifier twice -- once for the release it
is activating and once for whatever it rolls back to. So the deploy job writes
`releases/<sha>/VERSION` beside the binary, and `deploy.sh` reads it. Releases
built before this existed fall back to their directory name, which is exactly
what those binaries report.

Tagging a commit already on `main` re-runs the build -- the tag is the deploy
trigger, not a shortcut past CI.

### Three contracts

All are easy to break silently:

1. **`GET /v1/version` must contain the literal `"version":"<release>"`.**
   `deploy.sh` gates a release by matching that string and the workflow matches
   the public endpoint the same way. Neither parses JSON, so the field name, the
   quoting and the absence of a space after the colon are all part of the
   contract -- which is what `encoding/json` emits.

   The match is anchored on the field, and with `grep -F`, because both matter
   once the identifier is a tag: `v1.0.4` is a substring of `v1.0.40`, and to a
   regex the `.` in it matches any character at all. A loose match would call a
   release healthy that is not.
2. **The build version is injected into `internal/buildinfo.Version`.** `-X`
   against a symbol the linker cannot find is a *silent no-op*, so the workflow
   asserts `./easydnd -version` equals the identifier immediately after
   building. Without that check a wrong package path ships a binary reporting
   `dev`, fails the health gate, and rolls back with no obvious cause.
3. **The frontend build must be given `VITE_APP_VERSION`.** It writes
   `dist/version.json`, which the workflow reads through the public URL to
   prove nginx is serving this release rather than a cached `index.html`. An
   unset variable is the same silent no-op as a wrong `-X` path, so
   `web/vite.config.ts` refuses to build without it.

Keep the `-X` path in the `Makefile` in step with `internal/buildinfo`.

### Every response says which release answered it

`middleware.AppVersion` puts the identifier on every response as
`X-App-Version`, from the global chain rather than from a route -- so it lands
on a 401 and on the 404 from `NoRoute` as readily as on a success.

It is there for the browser, not for us. A tab that has been open since before a
deploy is running code that no longer exists on the server, and the header is
how it finds out without polling for it: the client compares it against the
version baked into its own bundle at the single point every request passes
through, so any request it was going to make anyway is the check. See
`docs/web.md`, "Two caches decide what a returning visitor sees".

`/v1/version` itself is `no-store`. It answers "which release is live", an
answer that can be held is not an answer to that question, and it has two
readers -- the deploy gate and the browser -- who would both be misled by a
stale one. It carried no cache header at any layer until that was noticed, and
was safe only because nginx happens to have no `proxy_cache`.

## Adding a feature

1. Entity and its `Repository` port in `internal/domain/<aggregate>/`.
2. `Service` with the injected port in `internal/usecase/<aggregate>/`.
3. Adapter implementing the port under `internal/adapter/repository/`, plus a
   goose migration under `internal/adapter/repository/postgres/migrations/` if
   it is stored. Never renumber a migration that has been applied, and keep it
   expand-only.
4. Handlers in `internal/api/http/v1/<resource>/`, one action per file.
5. Register routes in `internal/api/http/router.go` and wire the graph in
   `internal/app/app.go`. **Anything belonging to a person goes behind
   `middleware.RequireSession`** -- a resource route added one line above that
   group has no authentication at all. The one route that cannot be guarded is
   the SSO callback, because it is the request that establishes a session; it
   is guarded by the sealed flight cookie and its `state` instead, and the
   comment above it says so.

Errors travel as `internal/types` values and are rendered exactly once, by
`helpers.FormatError`. Handlers never build an error body themselves. **No
prose leaves this service** -- see [Errors are keys, not
sentences](#errors-are-keys-not-sentences).

If the thing belongs to more than one person, add a sixth step: put the
authorization in **one function in the usecase** that every read and write goes
through, the way `character.owned`, `group.member` and `game.readable` do, and decide
deliberately which refusals are 404 and which are 403 -- see [Ownership, and
membership](#ownership-and-membership). Two adapters means the shared contract
suite in `internal/adapter/repository/repotest/` runs the identical assertions
against both, which is the only thing that keeps them able to stand in for one
another.

The frontend side of the same feature is in
[web.md](web.md#adding-a-feature).

## Errors are keys, not sentences

The error envelope carries no message:

```json
{"error":{"code":"validation_error","reason":"group.name.required","request_id":"01J…"}}
```

`code` is one of seven and decides the status. `reason` is a stable slug the
client turns into a sentence out of `web/locales/*.json`, with `args` for
anything that has to be interpolated. Field errors carry the same pair beside
`field` and `rule`.

The reason is why: **the words a person reads are in the language that person
chose**, and this service has no idea what that is beyond an `Accept-Language`
header it would then have to hold a translation table for. The client already
holds one, for every other caption in the app.

### The English is not lost

It moved to the log. `types.NewValidationError("character %q is at sequence %d,
not %d", …)` still says exactly that, and `helpers.FormatError` now logs **every**
refusal rather than only the 5xx, tagged with the request id the browser is
holding. So "why did that fail" is still one `grep` away, and the person who hit
it is no longer shown a sequence number.

### Most errors do not need a slug

Of the couple of hundred raise sites here, most are saying something only a
developer wants: a value kind the wire should never have carried, a ceremony
envelope that would not decode. Those keep their English message for the log and
answer the client with the generic sentence for their code, which is all a
person could have done with them anyway.

The ones somebody is meant to read say so, with `Because`:

```go
types.NewFieldValidationError("a group needs a name",
	types.FieldError{Field: "name", Rule: "required"}).
	Because("group.name.required")

// A limit travels as an argument, so the two catalogues cannot drift from the
// constant the day it changes.
types.FieldError{Field: "name", Rule: "max"}.
	Because("field.maxChars", types.Args{"max": domain.MaxNameLen})
```

That keeps the vocabulary a translator has to cover down to the set somebody
actually reads -- about fifty -- and adding to it later is one call at the raise
site rather than a schema change.

**Never put an opaque id in `Args`.** "character %q not found" with `chr_9f2a`
spliced in reads worse in every language than "that character is not there", and
a visitor can do nothing with it. The id belongs in the log next to the request
that mentioned it. `Args` is for things a person can act on: a length limit, a
role name, a count.

### What the client does with an unknown reason

Falls back, in order: `error.<reason>`, then `error.code.<code>`. The server may
grow a reason before the browser does, and a vaguer sentence beats a bare slug
on screen. `lib/api/errors.ts` checks each key against the English catalogue
before using it, so that fallback is a real branch rather than a hope.

## Changing the SRD data

Never edit `data/srd_5.1/` by hand -- `make verify` regenerates it into a
temporary directory and fails on any difference, so a hand-edit survives only
until the next run. Change `cmd/srdgen` and run `make data/srd` instead.

**A translation is not a change to the generator.** Those go in
`data/translations/<locale>/`, which is hand-edited by design; see
[data/translations/README.md](../data/translations/README.md) and
[dnd.md](dnd.md#localization).

The generator does four things the raw dump does not:

1. **Splits mechanics from prose.** Language-neutral data lives in the directory
   root; translatable text lives under `i18n/<locale>/`, keyed by the same slug.
   A partial locale falls back to English *per key*, so a translated name with an
   untranslated description works.
2. **Merges translations from `data/translations/`.** Each locale is a
   subdirectory named by its language tag, and `srdgen` reads what is there
   rather than a list in code -- so adding a language is `mkdir`, and
   `rules.SupportedLocales()` is the only place a tag has to be legal. A slug the
   English bundle does not define is a warning, and `maxWarnings` is zero, so a
   typo fails the build with the file and the slug printed rather than being
   dropped into a file nothing reads. The generated locale directory holds only
   what was translated: the loader merges at read time, and writing the merge out
   would put a megabyte of untouched English into every language's diff.
3. **Normalises rule strings.** `"1 action"`, `"90 feet"` and `"Up to 1 minute"`
   are mechanics wearing prose clothing; they become structured values and are
   re-rendered per locale.
4. **Types every cross-reference** as `kind:slug`, using the upstream URL --
   `skill:acrobatics` and `proficiency:skill-acrobatics` are different things
   with confusingly similar names.

The wire format is defined once, in `internal/adapter/catalog/file/wire.go`, and
imported by both the generator that writes it and the loader that reads it. That
is what makes them impossible to drift apart.

Optionality comes from the Zod schemas in
`docs/reference_srd_5.1/data/5e-database-2014-en/schemas/`, not from inspecting
the data. Guessing is how a level-up calculator reads "this cantrip has no
scaling table" as "this cantrip does zero damage".

[layout]: https://github.com/golang-standards/project-layout
