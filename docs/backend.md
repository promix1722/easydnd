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

make verify                         # everything CI checks, back and front
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

- **Passkeys are unavailable.** WebAuthn requires a secure context, so
  `window.PublicKeyCredential` is undefined and the sign-in page draws no
  passkey card at all. The guest session is the way in. Passkeys still work at
  `http://localhost:{port}`, which browsers treat as secure.
- **`env: development` is doing real work.** It is what clears the cookie
  `Secure` flag and the `__Host-` prefix; a production-mode cookie would never
  be sent over such a connection. The generated config always sets it.

## Layout

Directory convention follows [golang-standards/project-layout][layout]; the
internals follow clean architecture.

```
cmd/easydnd/          process entrypoint: flags, config, logger, signals
cmd/srdgen/           converts the vendored SRD dump into data/srd_5.1/
cmd/devslot/          hands each worktree its own development ports
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

## The API

The resource routes below, plus the sign-in routes in
[Authentication](#authentication). Every resource route is at most **one level
deep**: a sub-resource under an addressed parent, never a sub-resource of
that.

| Method | Path | Returns |
|---|---|---|
| `GET` | `/v1/health` | liveness |
| `GET` | `/v1/version` | the release SHA -- a deploy contract, see below |
| `GET` | `/v1/catalog` | the compendium's index: ruleset, locales, collections and counts |
| `GET` | `/v1/catalog/{collection}` | one collection; `?slugs=a,b` narrows it |
| `GET` | `/v1/characters` | summaries |
| `POST` | `/v1/characters` | create: a name (and an alignment, if there is one) |
| `POST` | `/v1/characters/import` | import a sheet exported by another tool |
| `GET` | `/v1/characters/{id}` | the log |
| `DELETE` | `/v1/characters/{id}` | |
| `GET` | `/v1/characters/{id}/sheet` | the projection |
| `GET` | `/v1/characters/{id}/prompts` | what must be decided next |
| `GET` | `/v1/characters/{id}/events` | the log |
| `POST` | `/v1/characters/{id}/events` | append; returns the new sheet |
| `DELETE` | `/v1/characters/{id}/events` | truncate: `?after=N&expectedSeq=M` |
| `PUT` | `/v1/characters/{id}/events/{seq}` | replace one entry: `{expectedSeq, event}`, `?dryRun=true` |
| `DELETE` | `/v1/characters/{id}/events/{seq}` | remove one entry: `?expectedSeq=M`, `?dryRun=true` |

The last two address a *member* of the log by position, which is what `Seq`
means. That is not a third level: `events` is the sub-resource, `{seq}` names
one of them, and there is no route below it.

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

### Writing to a character

Every write states the sequence it expects the log to end at. The whole log is
one record, so without that check two clients would read, modify and write the
same blob and the later write would discard the earlier silently.

Every write returns the freshly projected sheet. That makes a build step one
round trip instead of two, and it is why the client needs no cache
invalidation: the response *is* the invalidation.

Every write also records a **`source`**: the group of the prompt the entry
answers -- `identity`, `abilities`, `race`, `background`, `class` or `advance`,
the same vocabulary `/prompts` groups its questions by. The **server** writes
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

### Ownership

A character belongs to the account that created it. The owner is resolved in
exactly one function, `handler.owner` in
`internal/api/http/v1/character/handler.go`, from the account
`middleware.RequireSession` put on the request; a handler reached without that
middleware gets the zero `OwnerID`, which owns nothing, so a mis-wiring shows
up as an empty party rather than as somebody else's.

Enforcement lives in the usecase, not the handler: every read and write goes
through `Service.owned`, which refuses a character to anyone but its owner --
and refuses it as a **404, not a 403**, because a 403 on somebody else's id
confirms that the id exists and turns a guessable identifier into an
enumeration oracle.

There is no role, group or sharing model. Authorization is one predicate.

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
  serialization and persistence belong to adapters. The three domain packages
  may import each other, one way only: `character` reads `catalog`, both read
  `rules`, and nothing points back.
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
other, carrying one extra private claim, `anon`. What it does not have is a
row: the id is minted, sealed into the token and never written anywhere.

That claim is load-bearing in exactly one place. `Session()` -- the usecase
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
  direction: a guest session has no row to attach anything to. Every surface
  that shows a guest session is obliged to say that nothing is being kept.
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

### Where accounts live

Accounts, their passkeys and their linked external identities are stored in
PostgreSQL -- AWS RDS in production -- by
`internal/adapter/repository/postgres`. Three tables:

| Table | Holds |
|---|---|
| `users` | the account id, display name and creation time |
| `user_credentials` | one row per registered passkey |
| `user_identities` | one row per linked external account |

The credential id is the **primary key** of `user_credentials` rather than a
surrogate. That is what gives `ByCredentialID` -- the lookup every usernameless
sign-in makes -- its index, and it enforces "a credential belongs to exactly one
account" in the database rather than in a map only one process can see.

`user_identities` keys on `(provider, subject)` for the same two reasons, and it
is **composite** because a subject is only unique within its issuer: keyed on
the subject alone, one provider's subject could resolve to an account linked
through another, which is a sign-in as the wrong person. `email` is stored but
is deliberately not unique and never a lookup key -- an address can be released
and reassigned, so matching on one would eventually hand somebody else's party
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
| push a `v*-notest` tag | the same minus the Test stage -- neither suite, neither version assertion |
| manual run on a `v*` tag | the same -- how you re-run a tag that failed halfway |
| manual run on a branch | builds and tests, but will not deploy |

Because nothing runs on `main`, **`make verify` locally before tagging is the
only thing standing between a mistake and a tagged release.**

`-notest` is for the release where you have just run it. The Test stage is
about ninety seconds of a four-minute deploy, and paying it twice for the same
checks buys nothing, so a tag named for the exemption skips both test jobs and
ships straight from Build. Naming the tag is the whole mechanism: the workflow
reads it off the ref, and `git tag` therefore shows for ever which releases
went out untested.

What it gives up is the pair of version assertions that live in the same stage
-- that `./easydnd -version` and the bundle's `version.json` both equal the
commit SHA. They cost seconds and they catch a silent ldflags no-op or a badly
tarred bundle *before* the symlink swap. Without them the same mistake still
gets caught, but by `deploy.sh`'s health gate: a rollback and a red `restart`
job minutes later, with nothing on the run page saying why. The suffix reaches
nothing else -- `VERSION` comes from `git rev-parse HEAD` and every path on the
server is keyed by the SHA, so a `-notest` release is byte-identical to any
other.

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
/opt/easydnd/current -> releases/<sha>     all three follow this symlink
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
  everything else to the bundle with an SPA fallback.

> **Apply the nginx config before the first tagged deploy carrying a
> frontend.** Until nginx serves `/opt/easydnd/current/web`, the workflow's
> public `version.json` check has nothing to read and the deploy job fails --
> after the release has already activated successfully, which reads as a far
> more alarming failure than it is. Applying it also means adding `www-data`
> to the `easydnd` group and *restarting* nginx; see the header of
> `deploy/nginx/easydnd.conf`.

Releases are keyed by commit SHA, not by tag: on a tag push `GITHUB_SHA` is the
commit the tag points at, so `/v1/version`, `releases/<sha>/` and the health
gate all work unchanged. Tagging a commit already on `main` re-runs the build --
the tag is the deploy trigger, not a shortcut past CI.

Three contracts tie the code to that pipeline. All are easy to break silently:

1. **`GET /v1/version` must contain the raw commit SHA.** `deploy.sh` gates a
   release with `curl … | grep -q "$SHA"` and the workflow substring-matches the
   public endpoint. Neither parses JSON, so never truncate the SHA or prefix it.
2. **The build version is injected into `internal/buildinfo.Version`.** `-X`
   against a symbol the linker cannot find is a *silent no-op*, so the workflow
   asserts `./easydnd -version` equals `$GITHUB_SHA` immediately after building.
   Without that check a wrong package path ships a binary reporting `dev`, fails
   the health gate, and rolls back with no obvious cause.
3. **The frontend build must be given `VITE_APP_VERSION`.** It writes
   `dist/version.json`, which the workflow reads through the public URL to
   prove nginx is serving this release rather than a cached `index.html`. An
   unset variable is the same silent no-op as a wrong `-X` path, so
   `web/vite.config.ts` refuses to build without it.

Keep the `-X` path in `.github/workflows/deploy.yml` and in the `Makefile` in
lockstep.

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
`helpers.FormatError`. Handlers never build an error body themselves.

The frontend side of the same feature is in
[web.md](web.md#adding-a-feature).

## Changing the SRD data

Never edit `data/srd_5.1/` by hand -- `make verify` regenerates it into a
temporary directory and fails on any difference, so a hand-edit survives only
until the next run. Change `cmd/srdgen` and run `make data/srd` instead.

The generator does three things the raw dump does not:

1. **Splits mechanics from prose.** Language-neutral data lives in the directory
   root; translatable text lives under `i18n/<locale>/`, keyed by the same slug.
   A partial locale falls back to English *per key*, so a translated name with an
   untranslated description works.
2. **Normalises rule strings.** `"1 action"`, `"90 feet"` and `"Up to 1 minute"`
   are mechanics wearing prose clothing; they become structured values and are
   re-rendered per locale.
3. **Types every cross-reference** as `kind:slug`, using the upstream URL --
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
