# Backend

The engineering doc for the [easydnd.org](https://easydnd.org) Go API:
architecture, layer rules, configuration and deployment. For the browser client
see [web.md](web.md); for the game model this serves, see [dnd.md](dnd.md).

Status: **the API is real.** The structure, wiring, deploy path, entity model,
SRD compendium, passkey and Google sign-in, and the rules math for creation and
level-up are built and tested. A character can be created, built and levelled
over HTTP.

## Rule pack runtime

Startup registers the base pack plus `data.pack_files` and
`data.autoload_packs`, resolves
`data.default_packs`, and compiles the selected contexts before readiness.
`data.pack_archive` retains immutable releases by digest outside deployment
folders. [packs.md](packs.md) documents the file format, CLI and
contextual catalogue routes. Autoloaded folders are additional public
catalogue choices, excluded from implicit default roots so replacement cores
can coexist. Each path accepts a pack directory or a repository with a `pack/`
child; an optional ID override gives replacement datasets their own namespace.
`data.private_pack_files` are installed the same way and then restricted to
`auth.superadmins` and the groups one of them grants a pack to; see
[packs.md](packs.md#common-and-private-disk-packs).
Missing or invalid configured packs fail startup. Changes require a restart.

### What the process holds

A release is read from disk once, and three things are kept for the life of
the process: the decoded document, its encoded JSON (what `export` and
resolution hand around), and its artwork as data URLs. Every catalogue
compiled from it *shares* those -- which matters, because the SRD pack is 19 MB
and 13 MB of that is spell icons.

- **A request copies none of it.** `Authoring.Builtins` hands out the same
  immutable bytes every time; nothing may write into a `Document.Data`.
  v1.1.0 cloned them on each call -- 22 MB per pack listing, three listings
  per spell-browser request -- and re-encoded the icons into each compiled
  catalogue, 19 MB apiece.
- **A lock of disk packs is compiled once**, at startup, by the base
  registry, and kept: one catalogue per lock and locale, bounded by what is
  installed.
- **A lock with a database pack in it** -- homebrew, a share, an import -- is
  compiled on first use and kept in a least-recently-used cache of
  `maxCompiledCatalogues` (8). An evicted one recompiles when next opened.
  Without the bound there is one catalogue per lock and locale anybody ever
  opened, forever.

Measured with the base pack and one private overlay: about 75 MB of live heap,
225 MB resident, and the same after 600 requests as after none.
`TestMemoryPackRequestsDoNotCopyOrLeak` holds the first two points and logs
the figures under `-v`; `TestCompiledCataloguesAreBounded...` holds the third.
Decoded documents of database packs (`Authoring.decoded`) are still kept
without a bound, one per published release.

The authoring service separately manages private drafts, published releases,
imports and group sharing.

All application character writes use repository revision CAS. `expectedSeq`
identifies positions; `expectedRevision` detects concurrent same-length edits.
Each init event pins a rules lock, and character/list/copy/shared-game reads use
that lock, which never changes afterwards. Characters, folders, shares and games are in
PostgreSQL whenever accounts are; see
[Where accounts and groups live](#where-accounts-and-groups-live).

## Quick start

```sh
make dev                            # Postgres, the API and the web client, one Ctrl-C
make ports                          # what this worktree claimed, and where to open it

make verify                         # everything CI checks, back and front, two jobs at once
```

Everything past `health` and `version` needs a session; in development
`POST /v1/dev/login` gives one, see
[Seeded development party](#seeded-development-party).

The server needs a database and does not start without `db.url`.
`config.dev.yaml` sets none, because the URL depends on the worktree's slot:
`make` passes it. The pieces `make dev` runs, one at a time:

```sh
make db/up                          # this worktree's throwaway Postgres
make run/db                         # the API against it, migrating on startup
make test/db                        # the suite including the Postgres adapter (-p 1: shared database)
make db/down                        # and delete it again
```

`make verify` needs no database: the tests run on the in-memory stores.

### An opened character is read by its link

A character has one switch its owner throws, `Public`, stored beside its
folder rather than in its log -- who may look at a character is not a fact
about the character. Open, **anybody signed in who has the link** may read the
sheet through the same `/v1/shared/{id}/...` routes a group's read uses,
whatever table they do or do not sit at; a guest session counts as signed in.
Hidden, which is the default, the character is its owner's and, in the groups
it is shared with, the DMs' and the group owner's -- not the other players'.

It is one line in one place: `readable` in `usecase/game/service.go`, the gate
every shared read already passes, answers yes for a public character before it
asks about groups -- and, last of all, yes for a superadmin, whatever the
switch says. Nothing about writing changes -- every write goes through
the character service, which asks who owns the character and nothing else. A
hidden character and a missing one both answer 404, so a link says nothing
about a character it does not open. Like the folder, the switch is a column
beside the log (`characters.public`), not an event in it.

### Seeded development party

Every API startup with `env: development` creates a ready-to-play group **Development party** and
three test accounts: **master**, **player1**, **player2**. The master owns the
group; both other accounts are players. Each owns one finished first-level
half-elf rogue, built through validated events rather than asserted stats.
The players' rogues are shared and seated in two games:

- **Training encounter**: both players are unlocked, with initiative rolls,
  temporary HP and a sample tag. Two private monsters demonstrate a character
  copy and a 10/10 HP stub. The master's original character stays private.
- **Locked encounter**: player2 is locked, so the master can test unlocking
  before that player edits game values.

Both games also seat two more shared characters, one per player: a fifth-level
paladin and a fifth-level cleric. They exist to show the
consumables tracker, which a first-level rogue cannot -- spell slots at several
levels, Channel Divinity, a pool too large for marks (Lay on Hands, 25), Hit
Dice. They are finished characters too -- a human acolyte each, with scores,
skills, a subclass, a fourth-level improvement, cantrips and prepared spells.

Every seed is a list of selections in `internal/app/dev_builds.go`, applied
entry by entry through the same validation an append from the build screen
takes, and `seedCharacter` refuses to return a character with a required
prompt still open. So a compendium edit that renames a prompt fails
start-up, naming the entry, rather than quietly seeding a sheet that opens as
"Unfinished" with no race and six tens.

`POST /v1/dev/login` with `{"account":"master"}` (or `player1`, `player2`)
issues the normal HttpOnly session cookie and returns the seeded `game_ids`.
It accepts only these three identities, keeps the `/v1` same-origin mutation
checks, and is never registered in production. No password or passkey is
required for these development accounts. The development client offers these
buttons on `/login` and an account switcher in the signed-in header.
Switching keeps the current seeded game when possible and reloads the page so
no previous identity's resource data or edit drafts remain.

Accounts, the group, the characters and the games are all reused when the API
restarts against an existing development database: the seed looks for
master's characters and builds nothing when they are there. Against a process
with no database it builds them every start. Signing in again within the same
run does not reseed or reset game changes. Open separate tabs and choose master, player1 and player2
in each: development shortcuts keep a random cookie selector in tab-local
`sessionStorage` and send it as `X-EasyDnD-Dev-Session`. It selects a signed
HttpOnly cookie and is ignored in production. Each successful switch
uses a new selector, including in duplicated tabs; a failed switch keeps the
previous identity.

All development auth cookie names also include a namespace derived from the
API listen address. Different worktree ports on the same browser hostname
therefore cannot overwrite or clear one another's sessions, passkey ceremonies
or SSO flight cookies. Production cookie names are unchanged. Restarting an
API without `auth.session_secret` still invalidates that API's sessions because
its signing key is generated per process. Because a browser does not isolate
cookies by port, a shared development hostname collects one such dead cookie
per switch and per restart, across every worktree's namespace, until the
`Cookie` header outgrows the proxy's 8 KB line limit and nginx answers
`400 Request Header Or Cookie Too Large` before the API sees anything. So in
development `RequireSession` also verifies every other session cookie of its
own namespace in the request and clears the ones that no longer work; cookies
of other namespaces are left alone, since another server's token cannot be
checked here and may be live. A jar that is already over the limit has to be
emptied in the browser once.

`make dev` is a **disposable** stack: Ctrl-C takes the database down with the
servers, so every run starts on an empty schema and nothing is left behind.
When you want accounts to survive a restart, use the three targets it composes
instead -- `make db/up` once, then `make run/db` and `make web/dev` -- and
`make dev/down` when you are finished with them.

Every one of those targets runs the API on the same committed
`config.dev.yaml`. What that file cannot say reaches the process through its
environment, in two layers that `make` puts there:

- **`~/config/easydnd/dev.env`** -- this machine's secrets, one file for every
  worktree and outside all of them (`DEV_ENV` overrides the path). Copy
  `easydnd.example.env` there, mode 600. **The AI Wizard's key is
  `EASYDND_AGENT_API_KEY` in this file**; the model is `agent.model` in
  `config.dev.yaml`. The file is optional: without it the server starts with
  the feature off and the target says so.
- **this worktree's slot** -- `EASYDND_HTTP_PORT`, `EASYDND_RP_ID`,
  `EASYDND_RP_ORIGINS` and, for `run/db`, `EASYDND_DB_URL`. They are passed,
  not written down: no config file is generated, so there is nothing per
  worktree to go stale or to hold a copy of a key.

`make spell-icons` and `make translate/ru` take their OpenAI key from the same
file. See [Configuration](#configuration) for the full list of variables.

Without `TEST_DATABASE_URL` the Postgres adapter tests skip themselves, which is
what keeps `go test ./...` and `make verify` green on a machine with no Docker.
CI sets it against a service container, so they are not skipped there.

## Tests

```sh
make test/unit                      # the suite, ~60s cold and ~3s with a warm test cache
make test/race                      # the same suite under the race detector, minutes
make test/db                        # including the Postgres adapter (needs make db/up)
```

**Where a database is involved, `test/db` is the only correct target**, and CI
runs it for that reason. Two packages reach the one database and each
wipes it -- `internal/adapter/repository/postgres` between subtests, and
`internal/api/http`'s durability test. `go test ./...` runs packages in parallel,
so without `test/db`'s `-p 1` one truncates a table another has just written,
and the failure surfaces in whichever package lost the race rather than in
whichever caused it. Both go red at once, which is the signature to recognise.

The rule is in `CLAUDE.md` because it is easy to reintroduce -- `make test/unit`
with `TEST_DATABASE_URL` set failed the v1.0.1 release in CI, the one place the
combination arose: set `TEST_DATABASE_URL`, run `test/db`.

`make verify` runs `test/unit`. It does **not** run `test/race`, and that is a
deliberate trade rather than an oversight.

The race detector used to be on by default, and it cost far more than it
looked. Part of that was not even work: a `-race` test binary sleeps a full
second at exit -- `GORACE`'s `atexit_sleep_ms`, defaulting to 1000 -- and this
module has a couple of dozen test packages, so every run spent that long on an
idle machine. `make test/race` sets `atexit_sleep_ms=0` and takes it back.
What the sleep buys is a last chance to check a goroutine still running when
`main` returns, and nothing here leaves one: the HTTP tests drive `httptest`
in-process and synchronously, and `internal/app`, which owns the only real
server lifecycle, builds its server without listening. A race *during* a test
is reported exactly as before. The rest is real: the detector slows the
catalogue adapter's CPU-bound tests about ten times over, so the whole suite
takes minutes under it.

That is the reason the detector still sits outside `verify`, which is what
`verify` is for. Nothing runs on `main`, so it is the only
gate there is, and a gate slow enough to be worth skipping stops being a gate.
The detector moved onto the path worth taking before a `git tag` -- the point
where a missed race would otherwise ship. **Run `make test/race` before
tagging.** It found real races in the HTTP layer and the stores once, and
`make test/unit` will not find the next one.

The heavy packages also run their tests in parallel: every top-level test in
`internal/adapter/catalog/file`, `internal/domain/character`,
`internal/usecase/character`, `internal/api/http` and `internal/app` calls
`t.Parallel()`, which is what lets the catalogue adapter's CPU-bound tests use
all four cores instead of one. `internal/usecase/agent` deliberately does not:
its tests wait on the agent settling under four-second deadlines, and under
load they starve. A new test in a parallel package calls `t.Parallel()` too,
unless it writes a package variable; `make test/race` is the check.

### verify runs two jobs, longest first

`verify` is a `make -j2` over the leaf targets other than `test/unit`, so the
Go side's small checks are spent inside the frontend's time rather than added
to it, and **the order they are named in is the
schedule**: `make -j` starts goals left to right as slots come free, so
`web/test` -- about forty seconds -- goes first, and the frontend's typecheck,
the production build and the small Go checks all happen inside its shadow.
Left at the end of the list it would land in the last slot and the run would
cost its length plus everything before it.

`test/unit` then runs **after** that group, alone. That is a measurement, not
tidiness: the heavy Go packages run their tests in parallel, so the Go
suite and vitest each want every core, and side by side they thrash -- a cold
Go suite that takes 60s alone took 100s beside vitest, vitest's 40s became
167s, and `verify` took 184s. One after the other it is 40s + 60s with a cold
test cache and 40s + 3s with a warm one, and the warm number is the one a
developer sees most.

Two jobs, not more. One of them is vitest, which forks
`availableParallelism - 1` workers of its own, so `-j2` is already the whole of
a four-core machine; `VERIFY_JOBS` is the knob for a worktree sharing the box.
`--output-sync=target` holds each target's output and prints it whole, so a
failure arrives as one block rather than interleaved with whatever else was
mid-run -- at the cost of nothing printing until a target finishes.

CI has the same two lanes and goes further: its six check, build and test jobs all start at once. See
[The three checks run at once](#the-three-checks-run-at-once-and-nothing-is-cached).
`verify` cannot copy that -- one machine, not six -- so here the lanes are two
and the order they are named in does the scheduling.

The other reason the suite is fast is that each test package shares **one**
`catalogfile.Registry`. It caches a compiled `*catalog.Catalog` per lock and
locale, and a `Catalog` is immutable, so one read of the compendium serves
every test in the binary. A `Registry` is dear to build: `NewRegistry` decodes the pack with
the strict two-pass decoder, re-marshals the whole document -- 13.5 MB of WebP
icons in base64 included -- to digest and validate it, then compiles every
locale, about eight CPU-seconds per build. So the tests share one through
a `sync.OnceValues` helper per package (`sharedRegistry`, `spellCatalog`,
`packBase`, `namespacedRegistry`), and reads of a registry are safe to share.
A test that *writes* into one -- `CompilePrivate` installs a release -- takes
a fresh one, and a test that changes a shared catalogue clones the map it
touches first. If you add a helper that needs the compendium, reach for
`filetest.SRD()` -- the SRD pack loaded the way the server loads it, once per
test binary -- or the package's shared registry rather than calling
`NewRegistry` again. The internal and external test packages of one directory need one each, since a package-level
var cannot cross that line.

## Running more than one worktree

Every port the local stack binds is derived from a **slot**, one number per
worktree, so more than one checkout can run at a time and a second
`make db/up` cannot adopt the first one's container and its database:

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

That is the whole configuration. `make dev` then passes
`http://dev.example.org:{8880 + slot}` to the API as the first of its
`auth.rp_origins` and hands it to Vite, which needs it for two things of its
own -- see [web.md](web/shipping.md#one-dev-server-per-worktree).

Three consequences of reaching the app over plain HTTP on a name that is not
`localhost`, all by design rather than breakage:

- **Passkeys are unavailable on the `888x` ports.** WebAuthn requires a secure
  context, so `window.PublicKeyCredential` is undefined and the sign-in page
  draws no passkey card at all. The guest session is the way in. They do work at
  `http://localhost:{port}`, which browsers treat as secure, and they work in
  `make preview` -- see [web.md](web/shipping.md#make-preview-is-the-only-secure-origin),
  which exists because service workers and the install prompt are blocked by
  this same rule.
- **`env: development` is doing real work.** It is what clears the cookie
  `Secure` flag and the `__Host-` prefix; a production-mode cookie would never
  be sent over such a connection. `config.dev.yaml` sets it.
- **`navigator.clipboard` is undefined**, for exactly the same reason as
  `PublicKeyCredential`. The invite sheet falls back to a selection copy and,
  if even that is refused, says so and selects the link -- see
  [web.md](web/shell.md#copying-the-invite-link).

## Layout

Directory convention follows [golang-standards/project-layout][layout]; the
internals follow clean architecture.

```
cmd/easydnd/          process entrypoint: flags, config, logger, signals
cmd/pack/             loads, validates and exports packs; `make pack/check` is this
cmd/packlint/         reads the pack's prose the way a player would and reports what is off
cmd/devslot/          hands each worktree its own development ports
cmd/llm/              dev-machine OpenAI tool: batch image generation, JSON translation
internal/
  app/                composition root -- the only package that knows every layer
  buildinfo/          Version, stamped by the linker
  config/             a committed YAML per environment, with a fixed env overlay
  logging/            slog constructor + request-scoped logger on context
  types/              transport-agnostic error vocabulary
  domain/rules/       shared value objects: slugs, dice, coins, choices  (layer 1)
  domain/catalog/     the SRD compendium and its Source port             (layer 1)
  domain/character/   the event-sourced character aggregate              (layer 1)
  domain/user/        the account aggregate and its passkeys             (layer 1)
  domain/group/       a table of people and their ranks                  (layer 1)
  domain/game/        one sitting at a group's table                     (layer 1)
  domain/pack/        rule packs, releases and locks                     (layer 1)
  domain/auth/        the ceremony and token-signing ports               (layer 1)
  usecase/            application services                              (layer 2)
  adapter/catalog/    reads the compendium off disk                     (layer 3)
  adapter/repository/ outbound adapters: memory and postgres, one contract suite (layer 3)
  adapter/agent/      the AI Wizard's model client                       (layer 3)
  adapter/webauthn/   runs the WebAuthn ceremonies                       (layer 3)
  adapter/oidc/       exchanges authorization codes with Google          (layer 3)
  adapter/token/      signs the session and ceremony cookies             (layer 3)
  api/http/           inbound adapter, gin                              (layer 3)
web/                  browser client (React + TypeScript, Vite); see web.md
data/pack/srd-5.1/    the compendium, hand-maintained, read at startup
data/locale-terms-locked/ the translators' glossary and provenance record
deploy/               server-side release activation and the nginx site
docs/reference_srd_5.1/   where the SRD data came from, and the licence question (notes only)
docs/reference_hexsheet/  a real exported character sheet, used as a shape reference
```

`data/pack/srd-5.1/` is a pack in this project's own format and nothing else:
there is no upstream dump and no generator between an edit and the server.
It was derived once from the `5e-bits/5e-database` dump and has been edited in
place since; see [Changing the SRD data](#changing-the-srd-data).

`cmd/llm` is a development-machine tool and no part of the service: it calls
the OpenAI API (key from `OPENAI_API_KEY`) to batch-generate entity artwork
and to translate JSON files, writing plain files that a developer then moves
wherever they belong -- it knows nothing of the repo's layouts. `llm images
-in prompts.json -out art/` turns a flat name-to-prompt object into
`<name>.png` files, skipping ones that already exist so an interrupted batch
resumes by rerunning; `-workers N` generates that many concurrently, safe
because a 429 self-throttles via its `Retry-After` (a free-tier cap of a few
images per minute makes more than 2 mostly queue). `llm translate -in
data/pack/srd-5.1/i18n/en/spells.json -out spells.ru.json -to ru` writes a
same-shaped copy with every string leaf translated, keys and
`{{placeholders}}` verified untouched. `-dry-run` on either shows the work
without the key or the spend.

The first consumer is the spell icons: `make spell-icons` chains a prompt
builder and a webp downscale (both `web/scripts/spell-icons.mjs`) around
`llm images`, caching the 1024px masters in `~/.cache/easydnd/spell-icons/`
and committing 128px WebPs to `data/pack/srd-5.1/spell-icons/`. The loader
picks those files up by name; see [packs.md](packs.md#spell-artwork).

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
| `GET` | `/v1/catalog/{collection}` | one collection; `?slugs=a,b` narrows it. Magic items list as *summaries*, and `?slugs=` returns full fidelity. **Spells are never served whole** -- see [below](#spells-are-never-served-whole): they answer `?slugs=`, or search parameters (`q`, `level`, `school`, `class`, `castingTime`, `concentration`, `ritual`, `material`, `limit`, `offset`) with a filtered, level-then-name-sorted, paged `{spells, total}` envelope -- the filter itself is `domain/catalog.SpellFilter` -- and the bare URL is a 400. `spell-filters` in place of a collection name returns what spells can be filtered by. `items` in place of a collection name is equipment and magic items **searched together**, `?q=&wearable=&category=&magic=&limit=&offset=`, as a name-sorted, paged `{items: [{slug, name, category, categoryName, cost, weight, magic}], total, categories}` envelope -- the sheet's Add pickers read it -- and like spells it is never served whole |
| `POST` | `/v1/catalog/spells/search` | the same search with a body, for an offer too long for a URL; also at `/v1/characters/{id}/catalog/spells/search` |
| `GET` | `/v1/characters` | summaries |
| `POST` | `/v1/characters` | create: a name (and an alignment, if there is one) |
| `POST` | `/v1/dev/login` | **development only** -- sign in as master, player1, or player2 in the seeded party |
| `GET` | `/v1/characters/{id}` | the log |
| `DELETE` | `/v1/characters/{id}` | |
| `GET` | `/v1/characters/{id}/sheet` | the projection, with what its slugs mean -- see [The sheet arrives resolved](#the-sheet-arrives-resolved) |
| `GET` | `/v1/characters/{id}/prompts` | what must be decided next |
| `GET` | `/v1/characters/{id}/events` | the log |
| `POST` | `/v1/characters/{id}/events` | append; returns the new sheet |
| `GET` | `/v1/characters/{id}/custom-options` | the character's custom entries, `{revision, options}` |
| `POST` | `/v1/characters/{id}/custom-options` | add or rewrite one, `{revision, option}`; an option without an `id` is new. Answers the new revision and sheet |
| `DELETE` | `/v1/characters/{id}/custom-options/{option}?revision=` | erase a custom entry of kind `note`; any other kind is a 400 -- see [below](#a-note-is-the-one-custom-entry-that-can-be-deleted) |
| `POST` | `/v1/characters/{id}/auto-equip` | dress a character who has nothing on: one suitable backpack item per slot; 204, and a no-op once anything is equipped |
| `GET` / `PUT` | `/v1/characters/{id}/visibility` | `{"public": bool}`: whether anybody signed in who has the character's link may read its sheet; owner only |
| `PUT` | `/v1/characters/{id}/events/{seq}` | replace one entry: `{expectedSeq, event}`, `?dryRun=true` |
| `DELETE` | `/v1/characters/{id}/events/{seq}` | remove one entry: `?expectedSeq=M`, `?dryRun=true` |
| `PUT` | `/v1/characters/{id}/folder` | file it elsewhere |
| `POST` | `/v1/characters/{id}/copy` | duplicate it, log and all |
| `POST` | `/v1/characters/{id}/copy-links` | mint a link whose holder may take a copy; owner only |
| `POST` | `/v1/copy-links/preview` | read a copy link: the character's name and class line |
| `POST` | `/v1/copy-links/accept` | take the copy, into your default folder |
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
| `PATCH` | `/v1/games/{id}/entries/{entry}` | patch game HP, temp HP, initiative, tags and spent uses (`{"used":{"spell-slots/1":2}}`); masters also lock players and edit monster base stats |
| `DELETE` | `/v1/games/{id}/entries/{entry}` | remove a player entry or monster; DM or owner |
| `POST` | `/v1/games/{id}/monsters` | private copy: `{"character_id":"..."}`; `{}` creates a stub; DM or owner |
| `POST` | `/v1/games/{id}/order` | stable sort: `{"by_initiative":true}`; move: `{"entry_id":"...","direction":-1}` (or `1`); DM or owner |
| `POST` | `/v1/games/{id}/rest` | a rest for the table; DM or owner. Long by default: every entry gets all of its spent uses back. `?kind=short`: only the pools a short rest refills |
| `GET` | `/v1/shared/{id}/sheet` | a shared character's sheet, read-only, resolved the same way |
| `GET` | `/v1/admin/players` | every stored account, newest first; `?q=` (name, email or id), `?kind=account\|guest`, `?limit=&offset=`. **Superadmin only**, 404 otherwise |
| `GET` | `/v1/admin/packs` | the private disk packs installed here. **Superadmin only** |
| `GET` | `/v1/admin/players/{id}/packs` | the private packs that account has been handed: `{"packs":["id"]}`. **Superadmin only** |
| `PUT` | `/v1/admin/players/{id}/packs` | replace that list whole. **Superadmin only** |
| `GET` | `/v1/admin/characters` | every character, newest first; `?owner=` (name, email or id), `?id=`, `?public=true\|false`, `?limit=&offset=`. **Superadmin only**, 404 otherwise |

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

**Seating your own player character shares it.** Every player character on a roster has to be
readable by every member -- a game carrying a name nobody but its owner may
open would be a leak the DM caused by accident -- so a character that is not on
the table yet is put there, provided it belongs to the caller. Somebody else's
unshared character is still a 400: a DM runs the table, but does not get to
publish another player's character on their behalf.

The two `events/{seq}` routes address a *member* of the log by position, which
is what `Seq` means. That is not a third level: `events` is the sub-resource,
`{seq}` names one of them, and there is no route below it.

A group's members are addressed the other way, by `?user=`. Either would have
been consistent with the rule above; a member is named by an opaque account
id rather than by position, so it travels as a query parameter.

The invite routes are a separate tree rather than sitting under `/v1/groups`
because somebody redeeming a link is **not in the group yet and cannot name
it** -- the token carries the id, so there is no addressed parent to hang them
off. Both take the token in the **body** and never in the URL: our own access
log records the route pattern, but nginx in front of it logs the whole request
line, and an invite token is usable for a day. The browser keeps it in a URL
*fragment*, which is never sent to any server at all. `/v1/copy-links` is the
same shape for the same two reasons.

`GET /v1/characters` takes `?folder=` to narrow the listing, and `POST
/v1/characters` takes a `folder` in the body.

### Spells are never served whole

Every spell carries its artwork inline, about 30 KB of it, so the spells
collection is 11 MB. Nothing a screen does needs all of it -- naming a dozen
spells, listing the books a filter offers, paging through a class's list --
so the route does not offer it: `GET …/catalog/spells` with neither `?slugs=` nor a search
parameter is a 400 (`field.limit.required`), on every scope the catalogue is
read through. A client that drifts back into fetching the list finds out on
the first request, not from a slow page.

What replaces it:

- **Named spells**, `?slugs=`, at full fidelity. Bounded at 200 per request.
- **A page of a search.** The query-string search above, or
  `POST …/catalog/spells/search` with the same filters and two things a URL
  cannot hold, `only` and `exclude`. `only` is an *offer* --
  `{slugs, fitting: [{minLevel, maxLevel, classes}]}`, the spells a character
  may pick, named or described; a spell is in it when either says so, and an
  empty offer matches nothing. `exclude` is what is already chosen. `limit` is
  required and at most 200. It is a POST because a bard's Magical Secrets
  offers every spell there is; it writes nothing.
- **`…/catalog/spell-filters`**: the packs, books, schools and classes the
  catalogue's spells can be filtered by, so that no screen reads them off the
  spells.
- **Resolved in a sheet**, below.

The manifest still counts the collection; it just cannot be fetched by that
name alone.

**`items` is held to the same rule for the opposite reason.** Equipment and
magic items are small enough to serve whole, and the plain collections still
are; but the one screen that *searches* them -- the sheet's Add pickers --
needs a page, not a download, and a client that pages never drifts into
holding the catalogue. So `GET …/catalog/items?q=&limit=&offset=` answers
both collections together, matched on name, sorted by name, in an
`{items, total, categories}` envelope with the `magic` flag telling the two
apart, and the bare `…/catalog/items` is the same 400. It is served on every
scope a collection is, so a character's picker sees the character's packs.

Three filters narrow it, and they are the search's own rather than the spell
parser's:

- `wearable=true|false` -- whether the item has a **slot**. That is the field
  the client splits the sheet's Equipment tab from its Items tab on, so each
  tab's picker asks for its own half and what it adds turns up on that tab.
- `category=<slug>` -- one equipment category.
- `magic=true|false` -- which of the two collections.

A hit carries what the picker's table shows and no more: the category **and
its name in the request's locale**, and for equipment its cost and weight (a
magic item has neither). `categories` is the category filter's options: every
category present under the `wearable` scope, **ignoring `q`, `category` and
`magic`** -- otherwise picking one would leave it the only option -- and sent
here so the client neither fetches `equipment-categories` nor derives the
list from a collection it is never given whole.

### The sheet arrives resolved

A projected sheet is slugs: `race: "half-elf"`, a list of features, the spells
a caster has. The two sheet reads -- `/v1/characters/{id}/sheet` and
`/v1/shared/{id}/sheet` -- send what those slugs mean in the same response,
through one function, `character.ResolvedSheetOf`:

- **`catalogNames`**, `"<collection>:<slug>"` to the localized name, for what a
  sheet only names: race, subrace, classes, subclasses, background, traits,
  features, feats, languages, and the source of each spell list. The projection
  already wrote the names of imported and custom entries there; those are kept.
- **`origins`**, what gave the character each trait and feature, both sides
  spelled as `catalogNames` keys: `features:maneuver-parry` to
  `features:maneuvers`, `features:second-wind` to `classes:fighter`. Worked out
  by `character.Origins` from the same catalogue, nearest source first -- the
  feature a pick was made under, then the owner of the rule that granted it,
  then the background or race, then the class or subclass on the entry's own
  row -- and the sheet draws it beside the name. An entry nothing accounts
  for, an imported one, has none.
- **`catalog`**, the entries a panel reads more than a name from, in the shapes
  the collection routes serve: the character's proficiencies, the items it
  holds (asked of both item collections, because a stack does not say which it
  came from), **its own spells with their artwork**, and all eighteen skills,
  which every sheet draws.
  Beside them, prose only: `actions`, keyed by each action's origin, and
  `traits`, `features` and `languages` -- the description of each one the
  character has, where the catalogue holds a description. The sheet opens a
  row onto it without asking again.

A slug the catalogue does not define is skipped, not an error: the client
title-cases it, as it always has.

The reason is the cost of the alternative. The server holds the character's
catalogue when it projects, so the lookup is a few map reads. Left to the
client it would be fourteen whole collections per sheet, the spell list with its
inlined artwork among them -- about 11 MB to print a dozen names. A sheet read
is one response, sized by the character rather than by the rules.

A write's echo of the sheet (`WriteResponse.sheet`) carries no `catalog`. It is
there to confirm the write; a screen that draws a sheet reads the sheet.

### Creating a character takes a name

`POST /v1/characters` takes a name, and an alignment if the player already has
one in mind. It takes neither the generation method nor the six base scores,
because the log is **one entry per selection**; a selection with no entry of
its own is a selection nobody can point at. See
[dnd.md](dnd.md#log-and-events).

That rule is refused, not just kept: an append or a revise whose entry selects
something *and* answers a question, or answers more than one question, is a
400 with the field rule `one-selection` (reason `field.answer.oneSelection`),
and the log's own validation refuses the second case for every other writer.
See [dnd.md](dnd.md#log-and-events). A batch may still carry several entries
-- a race, then what the race asked -- in one request.

So the scores are an ordinary open choice. A freshly created character has
`character/abilities` outstanding, answered with a `change` event carrying the
six `abilities.<ability>` paths, and the generation method travels with that
answer rather than with creation. The bound on a score -- 1 to 30, wide enough
for a DM's ruling and narrow enough to reject a typo -- travels with them, and is
checked where they arrive.

### Creation and level-up are one flow

`GET /v1/characters/{id}/prompts` answers "what does this character still have
to decide?", and it is the only endpoint a build screen needs. It returns
prompts in the compendium's own `Choice` grammar -- the ones the compendium
poses, verbatim, and synthetic ones in the same shape for the questions it does
not pose ("which race?", "which class?").

Advancement is one change event setting `identity.desiredLevel` (1--20,
bounded by validation with the reason slug `field.level.range`). For a
single-class character that number *is* the class's level -- `Project` raises
it in `advanceToDesiredLevel` -- and the levels it adds pose their own
questions here: the archetype at its due level, the Ability Score Improvements,
a feature's picks. So there is no level-up endpoint, because there is no
separate question: levelling up is raising the declaration, from the sheet's
Level up button or the identity tab, and creation is the first pass through
the same loop.

Nothing takes a level as an entry of its own: a bare `level` event -- one naming a class and carrying no answers -- is refused
on append, because no prompt offers it. The `level` event type is still in use
for what a level *grants*: an improvement, an Expertise, a feature's pick all
arrive as level events carrying answers.

**Multiclassing is not offered.** Nothing poses a question that would give a
character a second class; [dnd.md](dnd.md#log-and-events) says what the rule was. What
stays is everything that *reads* a multiclassed character: `Identity.Classes`
is a slice, `applyClasses` walks it, `classGrant` still knows a later class
grants no starting equipment, and the spellcasting summary is still one block
per casting class. Turning multiclassing back on is posing the question again
and stopping `advanceToDesiredLevel` from applying; the two go together, since
a declaration cannot say which class a level went into.

One consequence worth knowing: an **imported** multiclassed character loses
its later classes -- reported as unresolved rather than folded into the first,
which would give it levels in a class it never took.

Four fields make the client mechanical rather than knowledgeable:

- **`event`** says what the answer must be posted as. The class a character
  starts as is a `class` event, a subclass is a `subclass` event, and what a
  level grants is a `level` event; a client that decided this itself would be
  reimplementing the rules in the browser.
- **`optional`** says whether a character is complete without it. Without the
  distinction nothing is ever finished -- an unpicked personality trait would
  mean the character reads as unfinished forever.
- **`held`** lists options the character already has. Prompts are never
  narrowed by what is held, because narrowing would make the question depend on
  the order it was answered in; the client greys them out and the server
  rejects them. `heldOnly` inverts it for Expertise, where being proficient is
  the precondition rather than the conflict. It reaches inside branches, since
  the client draws a branch in the card that offered it.
- **`repeatable`**, on the choice rather than the prompt, says one option may
  be picked more than once — the picks are points to spend. Only a level's
  Ability Score Improvement says so: "+2 to one ability, or +1 to two". It is
  not read off the choice's *kind*, because a
  half-elf's two bonuses are the same kind over the same options and must go to
  two different scores.

An answer to a branch and the answer it opens arrive in **one event**, in that
order. `surviving` re-projects the prompts between each answer in a batch, so
the second is legal because the first landed — see
[web.md](web/builder.md#a-choice-inside-a-choice-is-answered-where-it-was-asked) for
why the client sends them together.

A prompt whose option set is **explicit and empty** is a question the player
answers in their own words. Three exist: a name, and the four roleplaying lines
in the `personality` group -- a personality trait, an ideal, a bond and a flaw.
SRD 5.1 prints eight of each and the compendium still carries them, but they
are not offered as prompts: a trait is the one line on a sheet that is
nobody's but the player's, and a menu of eight makes it the compendium's. The
state behind all four is free text (`State.Identity.PersonalityTraits` is a
`[]string`), and the prompt does not pretend otherwise.

Those four are posed the way `character/alignment` is, and for the same reason:
there is no option set to compare an answer against, so "answered" is a
question about the sheet rather than about the log. `promptBuilder.personality`
emits each only while its value is unset, and the change that sets it is what
closes it -- which is also what attributes the entry, through the same
`closedGroup` path the six ability scores go down. The projector seeds none
of them from a picked suggestion, so choosing a background *after* writing a
trait cannot overwrite it.

### Writing to a character

Every write states the sequence it expects the log to end at. The whole log is
one record, so without that check two clients would read, modify and write the
same blob and the later write would discard the earlier silently.

Every write returns the freshly projected sheet. That makes a build step one
round trip instead of two, and it is why the client needs no cache
invalidation: the response *is* the invalidation.

Every write also records a **`source`**: the group of the prompt the entry
answers -- `identity`, `abilities`, `race`, `background`, `class` or
`personality`, the same vocabulary `/prompts` groups its questions by. The
**server** writes
it, from the prompt the event was matched against, and it is ignored if a
request carries one. That is not distrust for its own sake: the client already
posts what a prompt told it to post, so the server knows which prompt that was,
and a client-supplied source would be a second vocabulary for the same fact,
free to disagree with the one the rules produce. Entries the server cannot
attribute -- an imported log, a DM's `change` -- carry no source. `GET
/characters/{id}/events` remains the unabridged record either way.

**Changing a pick is not a plain append**, even though answers fold
last-write-wins. `promptBuilder.add` stops emitting a prompt the
moment it is fully answered, so posting the same prompt again is rejected as a
prompt the character does not have open. Last-write-wins is what lets a *later*
entry answer an *earlier* entry's question -- a trait's prompt does not exist
until the race is chosen -- and it is not a way to change an answer. The
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
against. The exception is deliberate: an entry that only ever got in because nothing checked it
does not survive a replay.

**An entry carrying a `Ref` is never dropped merely because its answers died.**
A rogue whose race change invalidated one of their four class skills is still a
rogue: the class entry stands, keeps every other answer it carries -- the
Expertise, the starting weapon -- and the invalidated question comes back
outstanding under its own group. Deleting the entry would take the class, the
answers that were still fine and every level built on it, and a revalidation
that silently eats a player's choices is worse than none.

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
exactly one line: `repo.Commit`. A separate preview route would be two paths
to drift, and a preview that disagrees with its commit is worse than none. A
stale preview cannot be committed silently either, and that costs nothing
extra: the commit re-runs the replay, and if the log moved in between,
`expectedSeq` makes it the ordinary sequence conflict.

`Repository.Commit` is the port method behind it -- a whole-log write
rather than an append, because replacing one entry can drop entries after it
and the stored log comes back a different length. It is the only write the
port has for an existing log (`CreateWithLog` stores a new character with
one): the in-memory and Postgres adapters both implement it, and `source`
needed no migration or backfill because the log is one `json` column.

#### Saving several spell edits together

`POST /v1/characters/:id/events/revise[?dryRun=true]` accepts `expectedSeq`,
`expectedRevision`, `replacements: [{seq, event}]` and appended `events`. It uses
the same ownership and revision guards as a single replacement. Original
sequence numbers address every replacement throughout the replay, even if an
untouched event between them is dropped. Explicit replacements are validated
strictly against the rebuilt prefix; untouched suffix events retain the existing
survival rules. New acquisitions are validated after the replacements. Event
identities survive replacement.

The entire resulting log is projected before one repository commit. A failure in
any replacement or appended event writes nothing. Dry-run returns the final
sheet and dependent losses without saving; committing repeats those checks
against the same revision guard. The builder uses this to edit multiple saved
spells while adding new ones without partial writes or losing another draft.

#### Two questions about a reference

`validateRef` asks **does this entry exist in the compendium?**
`answersAnOpenPrompt` asks **was the character offered it?** The first alone is not enough: `POST
.../events {"type":"subrace","ref":"subrace:hill-dwarf"}` would be accepted for
a half-elf, because `subrace:hill-dwarf` resolves perfectly well, and the
projector would apply it. Revalidation needs the second too -- a replay with no
notion of "was this offered?" has no way to notice an entry the new prefix
orphaned -- so the two are asked together, in that order, on every
structural event.

`answersAnOpenPrompt` matches on the prompt's own `event` block -- the same
three fields a client copies into the body -- and then on one of two shapes:
the prompt selects the entry itself ("which race?"), and the event names an
option it offers; or the prompt hangs off an entry the character already holds,
in which case it states the `ref` to post with, and matching that ref is what
keeps a race's own follow-up entries alive. It is also the function that yields
the entry's `source`, so an entry's group and its legality are decided by one
match rather than two that can disagree.

One consequence worth stating: a `feat` event is not acceptable, because
no prompt offers one. The Ability Score Improvement's feat branch is answered
as a `level` event -- that is what the prompt says to post -- and no other
prompt asks for a feat at all. The projector still knows the type; "nothing can
be answered before it is asked" simply applies to it like everything else,
and a prompt that wants one has to say so.

### Folders

A folder is a named place one account files its characters. That is the whole
of it: one owner, nothing shared, no rule in the game reads it. It is **not** a
group of players -- that word is reserved, and kept out of this feature's
types, routes and screens on purpose, so the two cannot be confused.

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

**Deleting a folder deletes the characters in it.** There is no undo. A client
that offers the button owes the player a confirmation that says how many
characters are about to go; the web client's does. The cascade runs in the
usecase, not the store -- two aggregates, two stores, and a repository that
wrote to both would be two repositories sharing a name. Characters go first,
each through the same `Delete` a single character gets -- so one that was shared
with a group or seated at a game comes off those too -- and the folder last:
there is no transaction across the two, so the order is chosen for what a crash
half way leaves behind. This one leaves a folder holding fewer
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
write. `game.Service.readable` asks **"is it on a table you run, or opened by
its owner"** and grants
a read and nothing else. Neither was widened to accommodate the other: they are
different functions, in different packages, over different stores, and the write
paths still go only through the first. There is no route anywhere that writes to
a character through the second, which is why "read-only" here is a property of
the API's shape rather than a rule somebody has to remember.

#### What a table hands over

There is one exception, and it is not a widening of either function: four
routes under a game write to a seated character its actor does not own, and
they write a backpack count, a coin or one new custom item and nothing else.

| Route | Who | Does |
| --- | --- | --- |
| `POST /v1/games/{id}/entries/{entry}/items` `{item, count}` | DM or group owner | `count` more of `item` in the character's backpack |
| `POST /v1/games/{id}/entries/{entry}/custom-items` `{name, description, item}` | DM or group owner | one new custom item in the character's backpack; see "A custom item" below |
| `POST /v1/games/{id}/entries/{entry}/coins` `{unit, amount}` | DM or group owner | adds a signed `amount` to the purse; below zero is a 400 `coins.notEnough` |
| `POST /v1/games/{id}/entries/{entry}/give` `{item, count, to}` | the owner of `entry`'s character | moves `count` of `item` to the player entry `to` |

A table hands things over -- the DM gives out treasure, one player passes
another a potion -- and a rule that made the recipient type it into their own
sheet would be a rule nobody follows. What grants the write is the **game**:
the recipient is seated in it, and the actor either runs its table or is giving
up something of their own. So the routes hang off a game entry and not off
`/v1/characters`, whose every route is still the owner's alone, and
`character.Service.owned` is still untouched.

The write (`usecase/game/items.go`, `changeCharacter`) is an ordinary `change`
event with absolute `set` values on `equipment.backpack.<slug>` and
`equipment.purse.<unit>`, computed from the sheet as it stands and committed
against the character's revision -- the same event the owner's sheet sends, so
the log has one vocabulary and the owner can revise it like any other entry.
An owner's write landing between the read and the commit is a stale-revision
400, not a silent overwrite.

A give takes from the backpack, then from the loot, and never what is worn
(`item.worn`); more than is carried is `item.notCarried`. The item must exist
in the **recipient's** locked catalogue (`item.unknownToRecipient`): a custom
item, or one from a pack the recipient does not play with, has no slug there to
be counted under, and that is checked before anything leaves the giver. A grant
takes any slug the recipient's catalogue has, up to 100 at a time.

A give is **two commits, not one transaction** -- giver first, then recipient,
and the giver's is put back if the second fails. No repository method commits
two characters at once, so a crash between the two loses the item; it can never
duplicate it, and the DM can hand it back. Nothing records who gave what beyond
the two log entries.

Both refuse with **404**, and for the reason `owned` does: a character id is a
short sequence number, so a 403 on one that is not yours confirms it exists. A character
that was never shared, one unshared a moment ago and one that never existed are
indistinguishable from outside.

| | its owner | group owner | dm | player | not a member |
|---|---|---|---|---|---|
| see the group's table | — | yes | yes | yes | **404** |
| read a shared sheet, closed | yes | yes | yes | **404** | **404** |
| read a shared sheet, opened | yes | yes | yes | yes | yes |
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
| edit unlocked game values | yes | yes | yes | own character only | **404** |
| edit locked game values | **403** | yes | yes | **403** | **404** |
| lock/unlock, reorder, manage monsters, call a long rest | **403** | yes | yes | **403** | **404** |

A player still sees a closed character's row on the table -- name, class,
level -- because that is who is sitting there, not the sheet. The row carries
`public`, so the client draws the name as a link only for a reader the read
would admit.

Two rows are worth saying in prose. **A player may share** — that is the whole
of what a player does at a table. **A DM may unshare somebody else's character**, which looks like a
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

**Both stores are in Postgres, and neither has a foreign key to a character.**
`shared_characters` and `games.roster` name character ids, which are
drawn from a sequence that never hands an id out twice. The keys are left out
on purpose: the ports say the store does not verify the
character -- that is the usecase's authorization question -- and the in-memory
adapter cannot verify it either, so a key would make the two adapters answer
the same call differently. The cascades above are what keep the rows honest,
and every read already skips an id that is gone.

### A superadmin reads everything and writes one thing

An account named in `auth.superadmins` gets three reads beyond the private
packs, and one write: which private packs an account has been handed.

**Two listings**, `GET /v1/admin/players` and `GET /v1/admin/characters`,
behind `middleware.RequireSuperadmin`. The middleware runs after
`RequireSession`, whose account already carries its identities, so the check
costs no query; everybody else gets the **404** an unrouted path would, because
a 403 would tell any signed-in account that the surface exists. Both page with
`limit`/`offset` (default 50, at most 200) and answer `{players|characters,
total}`, the spell search's contract.

The listings are `Search` on the two repository ports, in both adapters. A
character's **name, level and class are not stored** -- they are folded from
its log -- so `usecase/admin` pages in SQL and runs `character.Summarize` on
the page it is about to return, and none of the three is a filter. What is
stored is filterable: the owner, the id, the public switch. The owner filter
is text a person types; the usecase resolves it to account ids through the
account `Search` (the first 200 matches) and adds the text itself as a literal
id, because a guest who never joined a group owns characters with no `users`
row to match by name. For the same reason **the players listing is the stored
accounts, not everybody who has played**: such a guest appears only as the
owner of their characters. "Last sign-in" is the newest `last_used_at` over an
account's passkeys and identities; nothing records later activity.

**Every sheet.** `game.Service.readable` asks a third question last, after
"is it yours" and "is it public or on a table you sit at": is the actor a
superadmin. So the client links an admin row to the existing
`/v1/shared/{id}/...` routes and there is no admin sheet endpoint. It is still
only `readable` -- `character.owned` was not touched, so a superadmin can
change nobody's character.

**A flag on the session.** `GET /v1/auth/me` carries `admin: true` for a
superadmin, so the client knows to draw the section. It grants nothing; the
routes above ask again on every request.

**The write: a private pack for one account.** `GET /v1/admin/packs` lists the
restricted disk packs installed here, and `GET`/`PUT
/v1/admin/players/{id}/packs` reads and replaces the ids one account has been
handed; the players listing carries the same ids on each row as `packs`. They
are rows in `user_rule_packs`, read by the same
`pack.Service.available` that already decided who has a pack, so a grant shows
up everywhere a pack does and nowhere new. It is by **pack id**, not by release
as a group share is: a private pack is replaced on disk by hand, and a grant
that pinned a version would silently end at the next copy. The usecase accepts
only an installed restricted pack and only a stored account -- a guest who
never joined a group has no row to hang it on. What a grant does and does not
give is in [packs.md](packs.md#common-and-private-disk-packs).

### A custom item

`POST /v1/characters/{id}/custom-options` takes an `item` on a custom entry of
kind `item`, in the catalogue's own item shape -- `category`, `slot`, `cost`,
`weight`, `weapon`, `armor` -- with `icon` being a pack icon's label. The
handler parses the words (`CustomItemOf`); `UpsertCustom` bounds the numbers
and checks every slug against the character's rules (`checkCustomItem`), and
answers a field error with reason `custom.item.invalid`. A write that carries
no `item` for an entry that has one keeps it, so the AI Wizard renaming what
it imported does not erase what a person filled in. What the model does with
it is in docs/dnd.md, "A custom item is an item".

The icons to choose from are the collection `item-icons`: label and data URL
for every item icon the character's packs carry, on the same
`.../catalog/{collection}` routes as the rest. It is not in the manifest, and
it is the one collection with artwork served whole -- about 1.7 MB -- because
a picker shows all of it; the client asks only when the picker is opened.

**A DM gives one at a game** through
`POST /v1/games/{id}/entries/{entry}/custom-items`, one of the hand-overs in
"What a table hands over" and held to the same rule as `GrantItem`: the caller
runs the table and the entry is a player's. `GrantCustomItem` appends exactly
one new definition, in the backpack, through the same `UpsertCustom` and the
same `CheckSheet` limits as the owner's route, and answers 204. It takes no
id, placement or count, so it cannot replace, move or remove anything.
`changeCharacter` projects against the character's custom overlay, so an item
that was given can be given on like any other.
The item search on the shared route, `/v1/shared/{id}/catalog/items`, leaves a
character's custom items out: it is what a DM hands out from, and those are
already that character's. Named outright they still answer, which is how a
shared sheet draws them.

A shared sheet and its catalogue (`/v1/shared/{id}/...`) are projected against
the character's custom overlay, as the owner's are, so a DM sees the item they
gave with its icon and numbers.

### A note is the one custom entry that can be deleted

A custom entry lives in the log as a `note` event carrying the definition, and
every kind but one may be something the character is built on: a custom class
it has levels in, a custom spell it prepared. Those are switched off through
their own `selected` flag and never removed, which is why
`DELETE …/events/{seq}` refuses a custom event outright.

Kind `note` is the exception because nothing can depend on it: it is a name
and a description the server gives no meaning to -- the player's own titled
text, drawn on the Custom tab of the sheet and the build screen. So
`DELETE /v1/characters/{id}/custom-options/{option}?revision=` drops that one
event and rebuilds the log, under the same ownership and revision checks as
the POST; the revision is in the query because a DELETE carries no body. A
non-note answers 400 `custom.notRemovable`, an unknown id 404. The AI Wizard's
tools still may not write a note, so what is on that tab is what the player
put there, plus whatever an older import left.

A title is 1-300 **bytes** and a text at most 16000, not characters: Cyrillic
gets about half. That is every custom entry's limit, not this route's.

### Active game entries

Game detail responses include ordered `entries`, separate from the legacy player
`characters` summaries used by character pickers. Each entry has a game entry
`id`, `kind`, `name`, optional portrait `image` and starting `class`, and caller-specific `can_edit`. Player entries additionally
carry `character_id`, `locked`, `hp`, `temp_hp`, optional `initiative`, `tags`, and
compact `stats`. The stats block contains `name`, `max_hp`, `armor_class`,
`spellcasting`, `speeds`, `senses`, and `abilities`; nested fields reuse sheet
shapes. An absent initiative is unset. PATCH accepts `initiative: null` to clear it.

Player base stats are projected live against their locked rules pack. HP and
other game values are initialized once, stored on the entry, and never written
to a character log. Newly seated characters are unlocked. Owners can edit only
their own unlocked entries; group owners and DMs can edit all entries.
`MutateEntries` runs the lock/ownership check and field patches against a deep
copy under the game repository mutex. Rejected changes leave storage untouched,
and independent field edits survive concurrent writes. The latest accepted
write wins when two requests change the same field.

**Consumables are game values too**, and they are sent only to the
character's owner and to whoever runs the table: another player's entry
arrives without `resources`, because what somebody has left to spend is theirs
to tell. A player entry carries `resources`: the
character's spendable pools -- spell slots by level, Pact Magic, every
pack-declared pool such as Channel Divinity or ki, and Hit Dice last -- each
`{id, name, group, max, used, dice?, slot_level?}`. Capacity is projected live
from the sheet like the other base stats; `used` is stored on the entry, per
game, and is what `PATCH .../entries/{entry}` sets through `used`, a map of
pool id to spent count merged key by key so two pools edited at once do not
overwrite each other. A count below zero, above the pool's capacity, or for a
pool the character does not have is a 400. It is one of the entry's game
values, so the same rule decides who may write it: the owner on their unlocked
entry, a DM or the group owner on any.

This is deliberately not an entry in the character's log, which holds build
decisions and nothing spent. A spent slot is a fact about one sitting, exactly as a hit point lost
is: the same character seated in two games has two independent counts, the
sheet always shows full pools, and a DM can correct a player's count without a
write path into somebody else's character. There are two recoveries, both `POST
/v1/games/{id}/rest` and both master-only, and neither touches HP. A **long
rest** clears every entry's spent uses -- Hit Dice included, where the rules
would return half. A **short rest** (`?kind=short`) clears only the pools whose
own recovery policy says a short rest refills them in full -- Second Wind,
Channel Divinity, a warlock's slots -- read from each player's sheet
(`ResourcePool.RestoredBy`). A policy with a condition counts as not applying:
no SRD pool has one, and evaluating it needs the catalogue.
A game can also spend a sheet's **plain-number scaling values** -- Extra
Attacks: 1, Maneuvers: 3 -- as pools of that many uses, under the id
`scaling/<parameter>` (`consumables` in `usecase/game/tracker.go`). A pack calls
them parameters because no rule spends them, but a table counts them off within
a turn, and the tracker is where counting is done. A value that is a die, a
fraction or a word has no number of uses and is not offered. A scaling value
has no recovery policy, so a short rest leaves it spent and only a long rest or
the row's plus gives it back.
The row's plus button is the undo for everything smaller. Monsters have
no pools.

NPCs hold private copies or editable default stats. New stubs are named NPC
and start with 10 current and maximum HP. The internal kind `monster` and
`/monsters` API route remain stable. Copying checks source
ownership and does not share the source. Multiple copies have separate IDs.
Copies retain their starting class for the default portrait. Players receive only
a monster's ID, kind, name, portrait image, starting class, and `can_edit: false`; private
fields and source links are omitted server-side. Source deletion/unsharing
removes linked player entries but does not remove copied monsters. Monster
`stats` patches update only supplied base fields and recalculate ability
modifiers; derived modifiers supplied by clients are ignored.

HP pools must be nonnegative integers; HP may exceed the sheet's maximum.
Initiative accepts signed integers. Tags are trimmed, deduplicated text, with
at most 20 tags of at most 100 characters. Tags have no rules effects. Monster
base values and movement ranges must be nonnegative; ability scores are 1–30.
Ordering is master-only: descending initiative, unset last, stable ties. Manual
menu moves exchange adjacent entries. Drag moves use `entry_id` and `before_id`
to place one entry before a stable target ID; an empty `before_id` appends it.
Missing source or target IDs reject the operation without changing the roster.
This atomic operation preserves entries added since the client's last view.
Sorting includes monsters without publishing
their initiative values. A game's roster, monster copies included, is one JSON
column on its row.

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

### Giving a character to somebody is giving them a copy

There is one way to hand a character to another person, and it does not change
who owns anything: the owner mints a **copy link**, and whoever opens it gets a
new character of their own with the same log. The original keeps its owner, its
folder, its group shares, its game seats and its AI Wizard chats, because
nothing about it was touched -- which is the whole reason this is a copy and
not a transfer. Moving `owner_id` would have to reconcile every one of those:
a folder that belongs to the old owner, a game seat whose stored owner decides
who may edit it, a chat that can still commit.

The link is the invite's design again (`internal/domain/character/copylink.go`):
a signed token naming the character and the owner who made it, 24 hours,
**reusable and not revocable**. That is a cheaper trade here than for a group,
since any number of redemptions cost the sender nothing. Redeeming loads the
character *as the owner in the token*, so a link dies with the character, and
the same 401-to-400 translation applies. Unlike the same-owner Copy there is no
`(copy)` suffix -- the recipient has no original to tell it from -- and the pack
check runs against the **recipient**: a character built on a pack they cannot
use is refused with `pack.unavailable` rather than handed over. A guest may
accept, as a guest may own a character.

### Limits

Everything a person can make more of has a number on it, and making one past
the number is refused. The numbers are one struct, `types.Limits` in
`internal/types/limits.go`, and `types.DefaultLimits` is its only value:

| Limit | Default | Counted | `reason` |
| --- | --- | --- | --- |
| `Characters` | 100 | per owner | `limit.characters` |
| `Folders` | 50 | per owner, the default folder included | `limit.folders` |
| `Groups` | 20 | groups an account owns now | `limit.groups` |
| `GroupMembers` | 50 | per group | `limit.groupMembers` |
| `GroupCharacters` | 200 | characters on one group's table | `limit.groupCharacters` |
| `GroupGames` | 50 | per group | `limit.groupGames` |
| `GroupPacks` | 20 | packs shared with one group | `limit.groupPacks` |
| `GameEntries` | 100 | one game's roster, characters and monsters | `limit.gameEntries` |
| `Packs` | 30 | an owner's unarchived packs | `limit.packs` |
| `PackReleases` | 100 | published versions of one pack | `limit.packReleases` |
| `CharacterItems` | 500 | item stacks on one character | `limit.characterItems` |
| `CharacterNotes` | 100 | Custom-tab texts on one character | `limit.characterNotes` |
| `CharacterCustomOptions` | 300 | custom entries of every kind on one character | `limit.characterCustomOptions` |
| `CharacterEvents` | 20000 | entries in one character's log | `limit.characterEvents` |
| `WizardRunsPerDay` | 20 | AI Wizard chats an owner started in 24 hours | `limit.wizardRuns` |

A refusal is `types.LimitReached`: a 400 `validation_error` whose reason is in
the table and whose `args.max` is the number, so the caption never repeats it.
It is a 400 and not a 429 because nothing about it is a rate -- waiting does
not help, deleting something does -- and it is the class `agent.capacity`
already used.

**It is a struct and not a block of YAML** for the reason `InviteTTL` is a
constant: an unknown config key stops the process at startup, so a key costs
two releases, and none of these numbers has needed to differ between
environments. Each service copies `DefaultLimits` when it is built and has a
`SetLimits` that the tests use; the AI Wizard reads the character service's.
The day a number has to vary, that setter is where the configuration lands.

Limits belong to whatever holds the list, which is not always a person. A game,
a shared character and a shared pack are a group's, so those are counted per
group and whoever adds the one too many is refused. `Groups` counts ownership
as it stands, so handing a group on frees a place.

**The per-character limits refuse growth, not size.** `character.CheckSheet`
compares the log before a write with the log after it and refuses only a count
that is over its limit *and* larger than it was. A character already past a
number -- the number was lowered, or an older import left it there -- can still
be edited, trimmed and levelled; it just cannot get bigger. It is called at
every place a log is written: `Apply`, `ReviseBatch`, `UpsertCustomOption`, and
the AI Wizard's tool calls, which write around all three and hand the refusal
to the model like any other tool error.

`CharacterEvents` is the catch-all. Personality traits, proficiencies and
conditions are all entries in the same log, so one
number bounds every list on a character that the struct does not name. It is
high because play adds to the log for as long as a campaign runs.

A character may be created four ways -- `Create`, a same-owner copy, a
redeemed copy link and the AI Wizard's import, which writes to the repository
directly -- and all four go through `Service.CheckCharacterLimit`. Both copies
share `copyTo`, which checks the limit of whoever receives the character, so a
copy link is refused when the *recipient* is full. A fifth must too.

Three things these limits do not do are in
[known-caveats.md](known-caveats.md#limits-are-counted-not-reserved).

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

Two mechanical checks back this up, and both are gates -- `make verify` and
the CI check job run each. `make lint/layers` greps the dependency graph of the
inner layers, transitively. `make lint` runs golangci-lint, whose `depguard`
rules deny the same imports one file at a time and add two things the grep
does not cover: no authentication library in the inner layers, and no adapter
importing another. The rest of what it enforces is in `.golangci.yml`:
unchecked errors, error wrapping, requests without a context, import grouping.
One rule is deliberately off -- a comment on every exported name -- for the
reason given beside it.

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

Configuration is **one committed YAML file per environment, plus an env file
for what must not be committed**:

| | Config -- committed, no secrets | Env file -- secrets, never committed |
|---|---|---|
| development | `config.dev.yaml` | `~/config/easydnd/dev.env`, one for every worktree, loaded by `make` |
| production | `config.prod.yaml`, shipped in each release as `config.yaml` | `/etc/easydnd/prod.env`, loaded by supervisor |

Analytics takes its token from `EASYDND_POSTHOG_TOKEN` in the existing env file,
entered by the operator and never committed. `analytics.host` remains in the
committed YAML as the HTTPS ingestion URL. With no env token, the committed
configs leave analytics disabled. Use the browser project token (`phc_`), not
a personal or secret API key. The public, uncached `GET /v1/analytics-config`
exposes only `environment`, `token`, and `host`, with environment taken from
the running server's `env`. It never serializes the full server config. See
[browser analytics](web/shipping.md#analytics) for setup and tracking behavior.

The app finds the YAML via the `EASYDND_CONFIG` environment variable, or a
`-config <path>` flag which takes precedence; there is no default location and
the file is **mandatory in every environment**. It logs which file it loaded.

The app never reads an env *file*. Whatever starts the process loads it, and
the loader lays a fixed list of variables over the parsed YAML -- a list, not a
naming rule, so a stray export cannot reach a key nobody meant to open:

| Variable | Sets | Where it comes from |
|---|---|---|
| `EASYDND_POSTHOG_TOKEN` | `analytics.token` | the env file, both environments |
| `EASYDND_AGENT_API_KEY` | `agent.api_key` | the env file, both environments |
| `EASYDND_SESSION_SECRET` | `auth.session_secret` | `prod.env` |
| `EASYDND_DB_URL` | `db.url` | `prod.env`; in development, `make` from the slot |
| `EASYDND_GOOGLE_CLIENT_ID`, `EASYDND_GOOGLE_CLIENT_SECRET` | `auth.google.*` | the env file, optional |
| `EASYDND_PRIVATE_PACK_FILES` (comma-separated) | `data.private_pack_files` | the env file -- a directory on that host, not a secret |
| `EASYDND_HTTP_PORT`, `EASYDND_RP_ID`, `EASYDND_RP_ORIGINS` (comma-separated) | `http.port`, `auth.rp_id`, `auth.rp_origins` | `make`, from the worktree's slot |

A set variable wins over the file; an unset or empty one leaves it alone. Two
tests keep the split honest: neither committed config may set a secret key, and
`config.prod.yaml` must be refused without the secrets and load with them.

`easydnd.example.env` is the template for both env files. In production it is
installed once, by hand:

```sh
sudo install -d -o root -g easydnd -m 751 /etc/easydnd
sudo install -o root -g easydnd -m 640 easydnd.example.env /etc/easydnd/prod.env
sudo -e /etc/easydnd/prod.env        # session secret, database URL, LLM key
```

Mode `640 root:easydnd` is the point: the service account can read the signing
key and nothing else on the host can.

The directory is `751`, not `750`: `deploy/deploy.sh` runs as `deploy` and
checks this file exists before swapping the release symlink, and testing a path
needs execute permission on every parent directory. A `750` directory fails that
check with `EACCES` while the file itself is perfectly fine — which is how the
v0.5.0 deploy failed. `751` grants others `--x`, so the path can be traversed
but the directory cannot be listed and the `640` file still cannot be read.

Because `config.prod.yaml` travels inside the release, the file a binary reads
is always the one it was tested against: a key added or removed in the loader
ships with the config that uses it, and a rollback takes both back together.
Only a *new required secret* still needs a hand step on the server before the
release that wants it.

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
| `data.pack_files` | `[]` | additional installed pack files/directories, common to everybody |
| `data.private_pack_files` | `[]` | pack directories only superadmins and granted groups can see; never a default root. Set from `EASYDND_PRIVATE_PACK_FILES` |
| `data.autoload_packs` | `[]` | additional public packs, each with a folder `path` and optional `id` override; root manifest or `pack/` child; does not add default roots |
| `data.default_packs` | `{}` | selected root IDs and version constraints; omitted means configured inputs |
| `data.pack_archive` | empty | optional persistent digest-addressed release directory |
| `data.srd_dir` | `data/pack/srd-5.1` | read at startup; a missing or malformed directory is a fatal error, by design. Absolute in production, through `current/` so it follows the symlink swap |
| `db.url` | *(none)* | **required in production**; libpq URL for the store that holds accounts, groups, characters, folders, games, packs and wizard chats. Say `sslmode=verify-full` -- an omitted `sslmode` means libpq's `prefer`, which is unauthenticated and permits a plaintext fallback. Unset in development falls back to the in-memory store with a warning. The template's placeholder password is rejected by name |
| `db.max_conns` | `10` | pgxpool size |
| `db.connect_timeout` | `5s` | bounds the startup ping; must fit inside `deploy.sh`'s 15s health gate alongside migrating and binding |
| `db.migrate_on_start` | `true` | apply pending migrations before the listener binds. Set `false` only to stage a migration by hand with `easydnd -migrate=up` |
| `auth.session_secret` | *(none)* | **required in production**; signs the session cookie. `openssl rand -base64 48`, quoted. Read as base64, taken literally if it is not valid base64; must decode to at least 32 bytes. The template's placeholder is rejected by name |
| `auth.superadmins` | `[]` | accounts that read private packs and grant them to groups or to single accounts, and that may list every account and character and read every sheet (see [A superadmin reads everything and writes one thing](#a-superadmin-reads-everything-and-writes-one-thing)): a **verified** Google email, or an account id |
| `auth.rp_id` | `easydnd.org` / `localhost` | **a one-way door** -- see below. `localhost` in development |
| `auth.rp_name` | `easydnd` | what the operating system's passkey prompt calls us |
| `auth.rp_origins` | `[https://easydnd.org]` / `[http://localhost:5173]` | a list; entries carry scheme and port, unlike the RP id. The first is where Google sign-in returns to. Also the CSRF allow-list: `middleware.SameOrigin` compares the `Origin` header on every non-safe request against it, so an instance reached on any origin not listed here rejects every write |
| `auth.session_ttl` | `168h` | how long a session cookie lasts |
| `auth.guest_session_ttl` | `24h` | how long an anonymous session lasts. Deliberately its own key, and shorter: a guest token cannot be revoked and names nothing recoverable, so the only thing bounding a leaked one is how soon it expires |
| `auth.ceremony_ttl` | `5m` | how long a begin/finish pair stays valid; also bounds an in-flight SSO redirect |
| `auth.google.client_id` | *(none)* | omitting the whole `auth.google` block means Google sign-in is **not offered**, which is a supported deployment |
| `auth.google.client_secret` | *(none)* | must be set together with the id; half a configuration is a startup error. The template's placeholder is rejected by name |
| `auth.google.redirect_url` | `/v1/auth/sso/google/callback` on the first of `auth.rp_origins` | must match a URI registered with Google byte for byte. In development that origin is the **Vite dev server**, not this process, and differs per worktree |
| `agent.model` | *(none)* | the AI Wizard's model; inert without `agent.api_key`. The other `agent.*` keys are in [agent.md](agent.md#configuration-and-deployment) |
| `agent.api_key` | *(none)* | **never in a committed file** -- `EASYDND_AGENT_API_KEY`. Unset means the AI Wizard is off |

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
is in [web.md](web/auth.md#one-button-means-both-halves).

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
| `GET /v1/appearance` | account session | read the current account's appearance |
| `PUT /v1/appearance` | account session | replace `{ "palette": "dragon", "color_scheme": "auto" }`; return saved appearance |

**Appearance is a separate resource for the current account.** Both methods
return the same representation, with a palette (`dragon`, `parchment`,
`midnight`, `moss`) and `color_scheme`
(`light`, `dark`, `auto`). The account aggregate and both repositories store
these fields; migration `00005_appearance.sql` defaults existing accounts to
Dragon/System and constrains the allowed values. The appearance endpoint requires
both fields, validates them in the appearance usecase, returns 400 for invalid input,
401 without a session and 403 for guests. GET and PUT use `Cache-Control: no-store`.
Appearance does not appear in authentication responses. PUT only changes the
current account's
appearance; credentials and identities remain intact. Guests store appearance
in their browser without creating an account row.

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

A guest has no row anywhere at all -- **until they touch a group**: a guest who joins somebody else's table has to be nameable in a roster
other people read, and `group_members.user_id` is a real foreign key. So the
group usecase writes a `users` row for a guest the first time they create or
join a group -- `EnsureGuest`, idempotent, and called on those two paths and on a
guest's first pack. A guest who does none of the three is stored nowhere.

A bare "Guest" is useless the moment three share a roster: nobody can tell
which one to remove. So a guest is "Guest" plus four
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
with no schema change. They are stored like anybody's, and nothing deletes them
when the guest session that could reach them expires; see
[known-caveats.md](known-caveats.md).

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
sees an `Inviter` port trading in strings and domain types. A **copy link is a
fifth**, behind `character.CopyLinks`; it reuses the invite's two claim slots,
so the kind claim is also all that keeps a group invitation from being redeemed
as a copy of whichever character shares its id.

CSRF is covered three ways, in `middleware.SameOrigin`: `SameSite` on the
cookie, an `Origin` check against `auth.rp_origins`, and a required
`X-Request-Id` header -- which `web/src/lib/api/client.ts` already sends on
every call, and which an HTML form cannot set at all.

That `Origin` check is why `auth.rp_origins` is not only a passkey setting. It
is the list of addresses this instance will accept a write from, so a
development instance reached on some other host has to have that host in it or
every POST comes back "request origin is not allowed" -- which is why
`make dev` passes it.

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

Accounts, their passkeys, their linked external identities, the groups they
play in, their characters and folders, and the pools and games at each table
are stored in PostgreSQL -- AWS RDS in production -- by
`internal/adapter/repository/postgres`. Ten tables (rule packs and AI Wizard
chats are kept there too, and are described with their own features:
[packs.md](packs.md), [agent.md](agent.md#session-lifetime)):

| Table | Holds |
|---|---|
| `users` | the account id, display name and creation time |
| `user_credentials` | one row per registered passkey |
| `user_identities` | one row per linked external account |
| `groups` | a group's id, name and who made it |
| `group_members` | one row per seat: who, in which group, at which rank |
| `folders` | one account's shelves; one is flagged the default |
| `characters` | one row per character: owner, folder, revision, and the whole log as `json` |
| `shared_characters` | one row per character a member has put on a group's table |
| `games` | a game and, as one `json` column, its whole roster |
| `private_releases` | the rule packs an AI Wizard import compiled, pinned by a character's lock and never listed |

Character and folder ids come from two sequences, `characters_id_seq` and
`folders_id_seq`, rendered in the same `chr_000001` / `fld_000001` shape the
in-memory store mints -- so nothing downstream can tell the adapters apart,
and, the point of it, an id never names a different character after a restart.
The rows that name a
character still carry **no foreign key** to it; the reason is under
[Ownership, and membership](#ownership-and-membership). `owner_id` on
`folders` and `characters` has none either, for the reason `agent_sessions`
gives: a guest may own one, and a guest is a `users` row only once they ask to
be named in a group.

A character's log is `json` rather than `jsonb`, as the wizard's document is:
it is never queried, and `jsonb` refuses the `\u0000` a transcribed source can
carry. The rules a write applies -- the revision guard and event stamping --
are the domain's (`Character.Commit`, `Log.Stamp`), so the SQL adapter is load-under-`FOR UPDATE`, call, store, and
cannot drift from the in-memory one; `repotest` runs both.

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

`user.Repository` has two implementations, as every store does. The server
runs on the Postgres one and refuses to start without `db.url`. The in-memory
one is what the tests run on, so `go test ./...` and `make verify` work with no
Postgres installed; `app.Options.InMemory` is the only way to select it, and
only this repository's tests set it.

Both run the same test suite, `internal/adapter/repository/repotest`. That is
not tidiness. `internal/api/http/helpers` maps a `*types.ValidationError` to 400
and a `*types.NotFoundError` to 404 exactly once, so two implementations that
disagree about which error a bad call produces are two different ports wearing
one name -- and only one of them can be right.

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
the server, for the operator who set `db.migrate_on_start: false` to stage a
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
| push a `v*` tag | gofmt, vet, lint, tests, build, version-injection check, then deploy |
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

What it gives up is the two suites and nothing else. The pair of version
assertions -- that `./easydnd -version` and the bundle's `version.json` both
report this release -- is in Build, beside the artifact each is about, so a `-notest` release still
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
tarball of `data/pack/srd-5.1/`, ships all three and `config.prod.yaml` into
`/opt/easydnd/releases/<sha>/`,
and runs `deploy/deploy.sh`: unpack the bundle, atomic symlink swap, supervisor
restart, health gate, automatic rollback on failure, prune to the last 5
releases.

```
/opt/easydnd/releases/<sha>/easydnd        supervisor runs this
/opt/easydnd/releases/<sha>/data/pack/srd-5.1/  the API reads this at startup
/opt/easydnd/releases/<sha>/web/           nginx serves this
/opt/easydnd/releases/<sha>/VERSION        what this release calls itself
/opt/easydnd/releases/<sha>/config.yaml    config.prod.yaml; EASYDND_CONFIG reads it
/opt/easydnd/current -> releases/<sha>     all five follow this symlink
/etc/easydnd/prod.env                      the secrets; not part of any release
```

A release is therefore a **directory, not a file** -- the compendium is read
from disk rather than embedded, and the frontend is static files. `deploy.sh`
checks for all three before the swap, so a partial upload fails *before* going
live rather than after. That ordering matters most for the frontend: nginx
serves `current/web`, so a bundle that unpacked badly would go live as a blank
site that the API-side health gate cannot see and will not roll back.

Because all three sit behind one symlink they swap together, so a rollback
reverts the UI, the API and its data as a unit.

The path of the data inside a release is part of the contract between the
tarball, `deploy.sh`'s existence check and the server's `data.srd_dir`. All
three travel with the tag -- `srd_dir` is in `config.prod.yaml`, inside the
release -- so moving the directory is one change and one deploy.

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

It works because the trigger and the deploy gate are different conditions: the
trigger also matches `ci/*`, the gate does not.

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
per lane, all starting together. A chain per lane cost
more than its contents: the Go module graph was compiled from scratch in each
of the three backend jobs, one after another. Nothing in Check produces
anything Build or Test reads; the only real tie is the version assertions,
which need a built artifact -- so they live in the job that builds it.

The one thing that arrangement costs is that **Check does not gate anything by
being upstream of it.** The two Deploy jobs name `check-*` in `needs` and test
its result explicitly. They have to: their `if` already lifts the implicit
`success()` gate so that `-notest` can work, so a missing clause there would not
fail loudly -- it would ship a release whose gofmt, vet, lint, layer and drift checks
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
7. Put `EASYDND_DB_URL` in `/etc/easydnd/prod.env`, alongside
   `EASYDND_SESSION_SECRET`. That file is `640 root:easydnd` precisely because
   it holds the credentials. **Install it before deploying the release that
   needs it** -- the binary refuses to start without a database URL, which the
   health gate would turn into a rollback. Adding a variable while an older
   release is live is safe: a binary ignores variables it does not read.

### What the server needs, in one place

A deploy carries the binary, the bundle, the compendium and `config.yaml`.
Everything below is set up once, by hand, and no tag changes it:

| Where | What |
|---|---|
| GitHub repository secrets | `SSH_HOST`, `SSH_PRIVATE_KEY`, `SSH_KNOWN_HOSTS` -- the deploy key's public half in `deploy`'s `authorized_keys`. No application secret is ever stored in GitHub |
| accounts | `deploy` (ships and activates releases), `easydnd` (runs the service), `www-data` in group `easydnd` |
| `/opt/easydnd` | owned `deploy:easydnd`, mode `2750` |
| `/etc/easydnd/prod.env` | from `easydnd.example.env`, `640 root:easydnd`, directory `751`: the session secret, the database URL, the LLM key, optionally the Google client |
| sudoers | `deploy` may run `/usr/bin/supervisorctl restart easydnd` without a password |
| `/etc/supervisor/conf.d/easydnd.conf` | from `deploy/supervisor/easydnd.conf`, **after** `prod.env` exists |
| `/var/log/easydnd/` | exists; supervisor writes `out.log` and `err.log` there |
| memory | a swap file (2 GB on the 1 GB host), and `GOMEMLIMIT` in the supervisor conf as a ceiling. v1.1.0 was OOM-killed a minute after its first sign-in; see [What the process holds](#what-the-process-holds) for why and for what it costs now |
| nginx and certbot | `deploy/nginx/easydnd.conf`, the `$connection_upgrade` map, the certificate |
| Postgres | the steps above |
| `/opt/easydnd/private-packs/` | optional: private rule packs, pushed by hand with `deploy/push-private-pack.sh` and named in `prod.env` |

`deploy.sh` checks before the swap that the release has its `config.yaml` and
that `prod.env` exists; it cannot read the latter, so a *missing variable* still
surfaces as a failed health gate and a rollback, with the reason in
`/var/log/easydnd/err.log`.

Two server-side configs are applied **by hand**, not by the pipeline, and both
are mirrored in the repo as the source of truth for what the live copy should
say:

- `deploy/supervisor/easydnd.conf` -- the API process. It loads
  `/etc/easydnd/prod.env` into the environment and points `EASYDND_CONFIG` at
  `/opt/easydnd/current/config.yaml`, so the config follows the symlink swap.
- `deploy/nginx/easydnd.conf` -- the routing: `/v1/` to the Go process,
  everything else to the bundle with an SPA fallback, plus the whole of the
  HTTP caching policy. **A tag deploy does not carry any of that**: after a
  change here the live copy has to be replaced by hand or the new rules are simply
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

**A release lives in a directory named by commit SHA.** `releases/<sha>/`. A tag can be moved; a commit cannot, so the SHA is what guarantees
two builds never land on top of each other. On a tag push `GITHUB_SHA` is the commit the tag points at.

Because those two are not the same string, `deploy.sh` cannot derive the
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
stale one.

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
not %d", …)` still says exactly that, and `helpers.FormatError` logs **every**
refusal rather than only the 5xx, tagged with the request id the browser is
holding. So "why did that fail" is still one `grep` away, and the person who hit
it is not shown a sequence number.

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

Every limit in [Limits](#limits) shares one family, `limit.<what>`, raised by
`types.LimitReached` with the number as `args.max`.

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

Edit `data/pack/srd-5.1/` in place. The generator that first produced it from a
vendored dump is retired: there is nothing to regenerate and no second copy to
keep in step.

What gates an edit is the loader itself: `make pack/check`
runs `cmd/pack` over the directory, which is the validation the server does at
startup -- schema, every cross-reference, every choice, every icon label --
and `make data/lint/check` holds the prose to the lint checks that are at zero.
Both are in `make verify`. A typo in a slug fails the load with the collection
and the slug named; a doubled space or an unclosed table row fails the lint.
Nothing tidies an edit silently, which is the point: a normaliser that rewrites
files behind the editor is a generator by another name.

The shape of the pack is what the generator established, and it still holds:

1. **Mechanics are split from prose.** Language-neutral data lives in the
   directory root; translatable text lives under `i18n/<locale>/`, keyed by
   the same slug. A partial locale falls back to English *per key*, so a
   translated name with an untranslated description works. A locale directory
   holds only what has been translated: the loader merges at read time, and
   writing the merge out would put a megabyte of untouched English into every
   language's diff. Adding a language is `mkdir`; the manifest names no files,
   the loader reads the layout (see [packs.md](packs.md#file-contract)).
   `rules.SupportedLocales()` is the only place a tag has to be legal, and
   nothing asks for a tag outside it.
2. **Rule strings are structured.** `"1 action"`, `"90 feet"` and
   `"Up to 1 minute"` are mechanics wearing prose clothing; they are stored as
   values and re-rendered per locale.
3. **What the books only say in prose is written as data.** `mechanics.json`
   holds the pools, rules and the actions open to everybody, with their English
   in `i18n/en/resources.json` and `i18n/en/actions.json`; a feature or trait
   that belongs in a character's action list carries an `action` tag on the
   row itself (see [packs.md](packs.md#action-tags)).
4. **Every cross-reference is typed** as `kind:slug` --
   `skill:acrobatics` and `proficiency:skill-acrobatics` are different things
   with confusingly similar names.

The wire format is defined once, in `internal/adapter/catalog/file/wire.go`.
The exported types are what a tool that writes a pack -- `cmd/pack`, a
homebrew editor -- has to produce, and the loader is the only reader.

**Every row is edited here.** `provenance.json` tags each row with its source,
and that is attribution, not ownership: the rows tagged `phb`, `xge` or `tce`
-- the non-SRD mechanics and names, converted once from a 5etools dump in
2026-10 -- are maintained in place like the SRD rows. What sets them apart is
what they lack: no prose, by design. Their text ships as a separate, private
overlay pack that is never deployed, see [packs.md](packs.md#prose-overlays)
and [licensing.md](licensing.md).

A translation is a change to the pack like any other: `i18n/<locale>/`, see
[data/locale-terms-locked/README.md](../data/locale-terms-locked/README.md)
and [dnd.md](dnd.md#localization).

[layout]: https://github.com/golang-standards/project-layout

## Builder choice responses

The existing prompts, events, append and revision routes serve equipment and
spell choices. No second spell-writing endpoint bypasses event validation.
Prompts add `purpose` (spell acquisition/preparation category), `upTo` (preparation
capacity) and `blocked` (conditional options requiring proficiency). Equipment
categories and supported collection choices are expanded into explicit legal
options, retaining category/collection metadata and stable option keys.

The events read endpoint adds read-only `choiceSource`, `choiceKind` and
`purpose` to name closed questions in the current locale, plus `selections`: resolved leaf options for
the event's answered choice trees, including item quantities and fixed components
of nested bundles. The normal catalogue option DTO is reused. Client-supplied
selection metadata is ignored; the server derives it from the character's log
and pinned catalogue. This prevents clients from parsing display text out of
bundle keys. Sequence/revision checks and mutation addresses are unchanged.

Sheet spell state adds `sources`, preserving source reference, class, casting
ability, cantrips, known spells, spellbook entries, preparation, special Arcanum
and mastery selections, and preparation limits. Existing aggregate fields remain.
Pack casting profiles describe learning mode, spellbook additions, preparation
and replacement policy. Profiles may belong to a selected subclass, with explicit
spell-list and ability overrides; parent class identity and level remain intact.
Repository autoload adopts matching sibling `spell-icons/` artwork into the
release, so exports and archives contain the same icons as catalog responses.
Source-specific spell benefits and conditional equipment requirements are validated with their referenced catalogue entries.

Append and revision use the same offered options and held/blocked checks. Replay
removes invalid dependent answers through the existing preview mechanism; it does
not silently grant unavailable equipment or spells. Catalogue option expansion
is shared with equipment projection, so validation and sheet reads use the
same definitions.


### Reading the original spell question

`GET /v1/characters/:id/prompts?before=N` returns the questions obtained by
projecting the event prefix strictly before saved event N. N must name an
existing event after the initial entry. This is a read-only operation with the
same ownership checks and pinned catalogue as the ordinary prompt read; it never
removes an event or writes a revision. The response retains the current head's
sequence and revision for concurrency checks, while its prompts and completion
flag describe the prefix. Spell editing uses this to obtain the original legal
pool without treating the saved picks as spells learned elsewhere. The client
seeds the editor from the saved event, then submits a normal event replacement;
replacement preview and suffix validation still apply.


### Explicit custom spells and wizard allowance metadata

`GET /v1/characters/:id/prompts` includes `spellRules`: source, class, acquisition
level, count, permitted spell-level range, shared class-list restrictions,
purpose, optionality, and any automatic spell grants. This is derived from the
same rules and catalogue as normal prompts, including already-answered choices.
It describes the current character and aggregates learning counts by source
and purpose. Spell-level bounds for ordinary learning use the current class
level. No new forget/replacement prompts or templates are exposed.

`before` reads the original answer prefix. For spell-only edits, a read-only
validation context adds the character's current levels and subclasses for classes already in
that prefix. This widens the spell pool to today's level while keeping the
original answer count and held selections. The same context is used by explicit
spell revisions, so a choice offered while editing is accepted when saved.
Synthetic context levels are never stored, never exceed the current character's
class levels, and do not affect non-spell revision validation.

Two optional prompts, `custom/spell/cantrip` and `custom/spell/known`, accept any
distinct catalogue spells of their respective spell-level category, independent
of class, character level and normal count limits. They use `change` events and
project into a separate `rule:custom-spells` source. They are explicit player
exceptions, not relaxation of ordinary spell validation. Unknown slugs,
duplicates and spells in the wrong category are rejected. Custom known spells
are also prepared; no casting ability is inferred for the custom source.
Empty answers remove a custom selection and reopen its optional prompt. Custom
prompts never prevent a character from being complete, and never satisfy an
unanswered class/racial prompt. Existing owner checks, revision guards and
atomic revision handling apply unchanged.

`spellRules.maxLevelCount` is present for ordinary class learning (known spells
and spellbook entries). It is the sum of actual acquisition counts at the current
maximum spell level plus eligible replacement opportunities, capped by the class
total. Feature grants stay separate. Append and atomic revision validate the
final projected class selection against this aggregate cap; checking individual
historical prompt counts alone would allow excess highest-level spells. Custom
sources are excluded. Existing logs still project for editing and correction.

`change` events at `spellLimits.known` and `spellLimits.cantrip` set or increment
an extra allowance (0–1000). These project independently from class rules and
return as optional `spellRules` entries with purpose `custom-limit`. The wizard
saves limit adjustments atomically with its spell selections. Custom spell
provenance remains in the event choices, so labels survive later editing.

## Character import sessions

The optional import agent uses a fixed Go worker pool and an OpenAI Responses
adapter behind the `AgentModel` port. Its authenticated `/v1/agent-sessions`
routes hold the conversation; the character it builds is an ordinary one in
the ordinary repository from the first message, committed to after every tool
call. Sessions and their source bytes are in PostgreSQL (`agent_sessions`,
`agent_events`, `agent_files`) behind the `Store` port declared beside
`AgentModel`; a turn is run by whichever API process claimed it under a
lease, and a process that stops gives its turn back to the queue. The wizard
also owns the service's only timer: once a second it looks for a turn queued
by another process, and every ten minutes it deletes the chats nobody has
used for a day. The character and the private packs a chat makes are stored
with everything else, so a restart keeps all three.
See [agent.md](agent.md) for tool contracts, lifecycle, bounds and the `agent`
YAML configuration. The nginx upload-limit change must be installed separately
from a release. The browser follows a session by
[polling](polling.md) once a second, which needs no proxy configuration; an idle
poll -- a `GET` answered `204` -- is logged at Debug rather than Info, since
every open wizard tab sends one a second. The wizard is also why the API
[runs as one process](known-caveats.md).

## User-authored rule packs

The pack usecase owns drafts, releases, and authorization. Its engine port reuses
`adapter/catalog/file` decoding, dependency resolution, normalization and compilation.
The adapter constructs immutable registry snapshots; character reads resolve exact
locks against built-in and retained database releases. The compiled cache is not
an authorization boundary. Catalogue selection, exports and new locks check access
on each request; character-owned reads can retain already pinned releases after
membership or sharing changes.

Migration 00004 adds `rule_packs` and `group_rule_packs`. Pack records store portable
release bytes and draft metadata together, with revision compare-and-swap writes.
Group shares store an exact dependency closure. Migration 00010 drops the
share's foreign key to `rule_packs`: a restricted disk pack is shared the same
way and has no row there. Deleting a group removes shares;
no release garbage collection is performed. Memory and PostgreSQL repositories
share concurrency and round-trip contract tests. A pack read never loads the
table: `ListFor` returns one owner's records plus the ones named by id -- the
packs the caller's groups share, or the ones a lock pins -- because the
availability check runs on every pack request and a document with artwork runs
to megabytes. Guest rows are materialized on
first pack creation using the same account repository operation as groups.

`POST /v1/characters` accepts an optional `rules` lock. Omitting it selects SRD
5.1. Event responses include
`rules`. Shared-sheet catalogue reads use `/v1/shared/:id/catalog/:collection` and
the same authorization as the shared sheet. Pack import is bounded to 64 MiB and
uses the existing strict JSON parser. `/v1/packs/schema` describes every editor
field from the wire types; it is presentation metadata, not a new pack format.

Private pack endpoints send no-store. General API failures retain reason slugs;
authoring validation additionally returns a document path and technical compiler
details, since authors need to diagnose unsupported rules and references.

The authoring adapter stores the import agent's generated private releases in
`private_releases` and keeps the ones it has seen in a per-process cache. They
stay outside pack listings and the default catalogue. Existing
imported characters and their copies resolve those exact definitions; copying
still checks current access to any ordinary homebrew releases in the same lock.

### Character portraits

Optional `image` values in character creation, sheet identity, summaries and game
rosters are inline raster data URLs. The existing log stores `identity.image` as
a string set; an empty string removes it. Editing the init event preserves fields
not included in the replacement, so a rename does not discard the portrait.
Copies retain it. Character portraits use the existing log storage.

Creation and event writes validate the image before committing: only JPEG, PNG
and WebP data URLs, at most 256 KiB including base64, with valid decoded pixels
and dimensions from 1 to 256 on each axis. URLs and SVG are rejected. Existing
character access checks also protect portraits. Monster entries inherit the
portrait when copied from a character and follow the existing privacy filtering.


Account portraits use the same validation. `PUT /v1/profile/image` accepts an
`image` string, including empty for removal, and updates only the signed-in
account. Guests cannot upload avatars. Migration 00006 adds the bounded image
column to users. Session and group-member responses include optional portraits.
