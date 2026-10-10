# easydnd developer tasks. Recipe lines must be TAB-indented.

BINARY      := easydnd
MODULE      := github.com/promix1722/easydnd
CMD         := ./cmd/$(BINARY)
SRD_DIR     := data/pack/srd-5.1
DEV_CONFIG  := config.dev.yaml

# The release identifier: the tag on a tagged commit, a short SHA anywhere
# else. Delegated to a script rather than spelled out here because CI needs the
# same answer, and the comment that used to say "keep in lockstep with the
# build step in deploy.yml" was an instruction to a human where a shared file
# does the job. Invoked through `bash` so that a lost execute bit -- an export,
# a fresh clone on a filesystem that drops it -- cannot break every build.
VERSION     ?= $(shell bash deploy/release-version.sh)
VERSION_PKG := $(MODULE)/internal/buildinfo
LDFLAGS     := -s -w -X $(VERSION_PKG).Version=$(VERSION)

GOLANGCI_VERSION ?= v2.6.2

# Machine-level development settings (PUBLIC_HOST, PUBLIC_PORT_BASE), then
# per-worktree ones. Both optional and both gitignored; without either, the
# defaults below describe a laptop with nothing in front of it.
-include $(HOME)/.config/easydnd/dev.mk
-include local.mk

# A slot is one worktree's share of the machine. Every port the local stack
# binds derives from it, so two worktrees can run at once without agreeing on
# anything in advance -- `make dev` claims one by probing, and records it in
# .dev-slot. See docs/backend.md#running-more-than-one-worktree.
#
# Each family starts past its own unclaimed default on purpose. Had the slots
# begun at the old constants, an unclaimed worktree and a slot-0 worktree would
# publish the same Postgres port and the second one up would quietly talk to
# the first one's database -- the failure this whole arrangement exists to
# remove. 5432 stays clear for a Postgres the machine already has.
SLOT_COUNT    ?= 10
WEB_PORT_BASE ?= 8080
API_PORT_BASE ?= 18080
PG_PORT_BASE  ?= 5440

# Read, never probed: every target except `dev` follows the claim rather than
# making one, which is what stops `make db/down` reaching into a neighbour.
SLOT ?= $(strip $(shell cat .dev-slot 2>/dev/null))

# PUBLIC_HOST is the name a browser reaches this machine on when a proxy sits
# in front of the dev server. Empty means there is none and the dev server is
# itself the origin.
PUBLIC_HOST      ?=
PUBLIC_PORT_BASE ?= 8880

ifeq ($(SLOT),)
# Unclaimed: the constants docs/backend.md and docs/web.md have always quoted.
WEB_PORT        := 5173
API_PORT        := 8080
PG_PORT         := 5433
COMPOSE_PROJECT := easydnd
WEB_PUBLIC_URL  :=
RP_ID           := localhost
else
WEB_PORT        := $(shell expr $(WEB_PORT_BASE) + $(SLOT))
API_PORT        := $(shell expr $(API_PORT_BASE) + $(SLOT))
PG_PORT         := $(shell expr $(PG_PORT_BASE) + $(SLOT))
COMPOSE_PROJECT := easydnd-$(SLOT)
WEB_PUBLIC_URL  := $(if $(PUBLIC_HOST),http://$(PUBLIC_HOST):$(shell expr $(PUBLIC_PORT_BASE) + $(SLOT)))
RP_ID           := $(if $(PUBLIC_HOST),$(PUBLIC_HOST),localhost)
endif

DEVSLOT_FLAGS := -count $(SLOT_COUNT) -web $(WEB_PORT_BASE) -api $(API_PORT_BASE) \
                 -pg $(PG_PORT_BASE) -public-host "$(PUBLIC_HOST)" -public-base $(PUBLIC_PORT_BASE)

# This worktree's Postgres, from deploy/local/docker-compose.yml. Production is
# RDS over TLS; this is sslmode=disable because a throwaway container has no CA.
TEST_DATABASE_URL ?= postgres://easydnd:easydnd@127.0.0.1:$(PG_PORT)/easydnd?sslmode=disable

# What a committed config cannot carry: secrets, and anything true of this
# machine only. One file for every worktree, outside all of them, loaded into
# the API's environment by the targets that run it -- see easydnd.example.env
# for the names. Optional: without it the stack still runs, with the AI Wizard
# off.
DEV_ENV ?= $(HOME)/config/easydnd/dev.env

# The origins a browser may reach this worktree on. The public one first when
# there is one, because the first is where Google sign-in sends people back to.
comma := ,
DEV_ORIGINS := $(if $(WEB_PUBLIC_URL),$(WEB_PUBLIC_URL)$(comma))http://localhost:$(WEB_PORT)

# dev_env: the environment a development API runs in -- DEV_ENV, then this
# worktree's slot laid over it. $(1) port, $(2) origins, $(3) database URL.
# The slot is passed rather than written down: config.dev.yaml is the same file
# in every worktree, and internal/config reads these four over it.
define dev_env
set -a; \
	if [ -f "$(DEV_ENV)" ]; then . "$(DEV_ENV)"; else echo "no $(DEV_ENV) -- AI Wizard off, see easydnd.example.env"; fi; \
	EASYDND_HTTP_PORT='$(1)'; EASYDND_RP_ID='$(RP_ID)'; EASYDND_RP_ORIGINS='$(2)'; \
	$(if $(3),EASYDND_DB_URL='$(3)';) \
	set +a
endef

# llm_key: the same key for the command-line tools that spend OpenAI credit.
define llm_key
if [ -f "$(DEV_ENV)" ]; then . "$(DEV_ENV)"; fi; export OPENAI_API_KEY="$${OPENAI_API_KEY:-$$EASYDND_AGENT_API_KEY}"
endef

# `make preview`: the built bundle and the API on one origin, behind TLS.
#
# Fixed ports rather than a slot, and outside the 808x slot range so they
# collide with no worktree. nginx maps 8890 -> 8090 with the real hton.cloud
# certificate, which is what makes this a secure context -- and a secure
# context is the whole point: service workers, the install prompt and passkeys
# are all unavailable on the plain-HTTP 888x ports.
#
# One pair of ports means ONE PREVIEW AT A TIME across every worktree. That is
# deliberate; a preview is a verification pass, not somewhere to live.
PREVIEW_PORT        := 8090
PREVIEW_PUBLIC_PORT := 8890
PREVIEW_URL         := $(if $(PUBLIC_HOST),https://$(PUBLIC_HOST):$(PREVIEW_PUBLIC_PORT),https://localhost:$(PREVIEW_PUBLIC_PORT))

.DEFAULT_GOAL := help

## help: list available targets
# -h because MAKEFILE_LIST holds the optional dev.mk/local.mk includes too, and
# grep prefixes every match with the file it came from once it has more than one.
help:
	@grep -hE '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'

## build/release: build exactly what CI ships (linux/amd64, static)
build/release:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

## test/unit: run the test suite (~60s cold, ~3s with the test cache warm)
# No -race here, and that is a deliberate trade rather than an oversight: the
# detector multiplies the cold minute several times over. A gate slow enough
# to be worth skipping stops being a gate, and since nothing runs on main this
# is the only one there is.
#
# The detector is not gone, it has moved off the path everybody walks. Run
# `make test/race` before tagging. See docs/backend.md#tests.
test/unit:
	go test ./...

## test/race: the whole suite under the race detector (minutes) -- not in `verify`
# atexit_sleep_ms=0 takes back a second per test binary: the race runtime
# sleeps a full second at the exit of each one by default, which across the
# test packages here is a quarter of a minute of an idle machine. What the
# sleep buys is a last chance to check a goroutine still running when main
# returns; nothing here leaves one, because the HTTP tests drive httptest
# in-process and synchronously and internal/app builds its server without
# listening. A race *during* a test is reported exactly as it was before,
# which is what this target is for. The catalogue adapter's package is most
# of the cost: its tests run in parallel, and the detector slows each by
# roughly ten times.
test/race:
	GORACE=atexit_sleep_ms=0 go test -race ./...

## db/up: start this worktree's Postgres, for development and the adapter tests
# -p is what keeps worktrees apart: it overrides the compose file's own `name`,
# so each slot gets its own project, network and containers.
db/up:
	EASYDND_PG_PORT=$(PG_PORT) docker compose -p $(COMPOSE_PROJECT) \
	  -f deploy/local/docker-compose.yml up -d --wait

## db/down: stop this worktree's Postgres and delete its data
db/down:
	EASYDND_PG_PORT=$(PG_PORT) docker compose -p $(COMPOSE_PROJECT) \
	  -f deploy/local/docker-compose.yml down -v

## db/psql: a psql shell on this worktree's Postgres
# The container has no fixed name -- that would be one global name for every
# worktree to collide on -- so compose resolves the service instead.
db/psql:
	EASYDND_PG_PORT=$(PG_PORT) docker compose -p $(COMPOSE_PROJECT) \
	  -f deploy/local/docker-compose.yml exec postgres psql -U easydnd -d easydnd

## test/db: run the suite against the local Postgres (needs `make db/up`)
# Without TEST_DATABASE_URL the Postgres adapter tests skip, which is what
# keeps `make verify` working on a machine with no Docker. This target is how
# they actually run.
#
# -p 1 because two packages reach the same database and both TRUNCATE it:
# internal/adapter/repository/postgres between subtests, and
# internal/api/http's durability test before it registers. Run in parallel,
# one wipes the other's account mid-test and the failure lands in whichever
# package lost the race -- which is not where the problem is. One database and
# a global TRUNCATE mean the packages have to take turns.
#
# **CI runs this target too**, and for exactly that reason. It used to run
# `test/unit` with TEST_DATABASE_URL set, which is `go test ./...` without the
# -p 1 -- the one combination this flag exists to prevent, and the only place
# in the world it occurred, since on a laptop those tests skip. It failed the
# v1.0.1 release with both packages red at once, which is what the race looks
# like from the outside. So the rule is: anywhere TEST_DATABASE_URL is set,
# this is the target to run.
test/db:
	TEST_DATABASE_URL=$(TEST_DATABASE_URL) go test -p 1 ./...

## preview: serve the BUILT bundle and the API on one TLS origin, for PWA testing
# What the dev server cannot do. `make web/dev` has no service worker at all
# (devOptions.enabled is false in vite.config.ts, so a worker cannot shadow the
# module graph), and its origin is plain HTTP, so `beforeinstallprompt` never
# fires and passkeys are unavailable. This target answers both: it serves the
# real production bundle, and it does it behind the TLS port.
#
# No Vite, and so no HMR: the whole point is to exercise the artifact that
# ships rather than a development approximation of it. Rebuild by restarting.
#
# Needs the 8890 server block in /etc/nginx/conf.d/z-dev-ports.conf; see
# docs/web.md.
preview:
	@go run ./cmd/devslot claim $(DEVSLOT_FLAGS) >/dev/null
	@$(MAKE) preview/up

# One port, because Go is serving the bundle as well as the API, and an https
# origin, because middleware.SameOrigin compares auth.rp_origins against the
# browser's Origin byte for byte and the browser will say https here.
preview/up: db/up web/build
	@echo "preview  $(PREVIEW_URL)  (127.0.0.1:$(PREVIEW_PORT))"; \
	 trap 'exit 0' INT TERM; \
	 trap '$(MAKE) --no-print-directory db/down' EXIT; \
	 $(call dev_env,$(PREVIEW_PORT),$(PREVIEW_URL),$(TEST_DATABASE_URL)); \
	 go run -ldflags "$(LDFLAGS)" $(CMD) -config $(DEV_CONFIG) -web web/dist

## run/db: run the API in development mode against this worktree's Postgres
run/db:
	@$(call dev_env,$(API_PORT),$(DEV_ORIGINS),$(TEST_DATABASE_URL)); \
	 go run -ldflags "$(LDFLAGS)" $(CMD) -config $(DEV_CONFIG)

## dev: this worktree's whole stack -- Postgres, the API and the web client
# Claims a slot first, then re-enters make: SLOT is resolved when the Makefile
# is parsed, so the recipe that uses it has to be in a second parse that can
# see the .dev-slot the claim just wrote.
dev:
	@go run ./cmd/devslot claim $(DEVSLOT_FLAGS) >/dev/null
	@$(MAKE) dev/up

# Three processes, one Ctrl-C, and nothing left behind.
#
# INT and TERM are trapped, not just EXIT: a shell killed by a signal it does
# not trap dies without running its EXIT trap, so the cleanup below would be
# skipped by the very Ctrl-C that is supposed to trigger it. The handlers exit,
# which is what reaches EXIT -- with 0, because Ctrl-C is how this target is
# meant to end and a red "Error 130" would suggest something went wrong.
#
# `kill 0` signals the whole process group, which is what stops a `go run`
# binary or a Vite server outliving the make that started it and holding the
# ports against the next `make dev`. It also ends this shell, so anything meant
# to run afterwards never would -- which is why the database comes down first.
#
# Every `make dev` therefore starts on an empty schema. That is the point of
# it: a disposable stack. To keep accounts across restarts, use the three
# targets it composes -- `make db/up` once, then `make run/db` and
# `make web/dev` -- and take it down with `make dev/down` when you are done.
dev/up: db/up
	@echo "web  http://127.0.0.1:$(WEB_PORT)$(if $(WEB_PUBLIC_URL),  -> $(WEB_PUBLIC_URL),)"; \
	 echo "api  http://127.0.0.1:$(API_PORT)   pg 127.0.0.1:$(PG_PORT)   compose $(COMPOSE_PROJECT)"; \
	 trap 'exit 0' INT TERM; \
	 trap '$(MAKE) --no-print-directory db/down; kill 0' EXIT; \
	 $(MAKE) run/db & \
	 $(MAKE) web/dev & \
	 wait

## dev/down: take this worktree's stack down and delete its database
# `make dev` already does this on the way out. This is for when it could not --
# a closed terminal, a SIGKILL -- and for a stack started from db/up and run/db
# separately. The listing afterwards is the check that worked: this worktree's
# row should read "idle".
dev/down: db/down
	@go run ./cmd/devslot list $(DEVSLOT_FLAGS)

## slots: which slot each worktree on this machine is holding
slots:
	@go run ./cmd/devslot list $(DEVSLOT_FLAGS)

## ports: this worktree's port allocation
ports:
	@echo "slot $(if $(SLOT),$(SLOT),<unclaimed>)  web $(WEB_PORT)  api $(API_PORT)  pg $(PG_PORT)  compose $(COMPOSE_PROJECT)"
	@echo "open $(if $(WEB_PUBLIC_URL),$(WEB_PUBLIC_URL),http://127.0.0.1:$(WEB_PORT))"

## pack/check: load the hand-maintained SRD pack through the real loader
# The pack is edited by hand, so there is nothing to regenerate and diff; the
# gate is the same validation the server runs at startup -- schema, every
# reference, every choice -- plus the item and spell icons it names.
pack/check:
	@go run ./cmd/pack -in $(SRD_DIR) >/dev/null && echo "srd pack loads"

## data/lint: report suspicious prose in a pack (PACK=dir, default the SRD)
# See docs/packs.md#linting-the-prose.
PACK ?= $(SRD_DIR)
LINT_GLOSSARY ?= data/locale-terms-locked/ru.glossary.json
data/lint:
	go run ./cmd/packlint -in $(PACK) -glossary $(LINT_GLOSSARY)

## data/lint/check: fail if the SRD prose regresses on a check that is at zero
# A check joins this list when its last finding is fixed, not before: a gate
# that starts red is a gate people learn to skip.
LINT_GATED ?= markup-leftover,source-code-leak,stray-punctuation,space-before-punctuation,double-space,html,edge-whitespace,empty-string,malformed-table,markdown-in-plain-field,cyrillic-dice,metric-units,inline-table,heading-glued-to-paragraph,english-in-brackets,nonstandard-abbreviation,untranslated-words,same-as-default-locale,slug-not-in-default-locale,missing-name,missing-desc,missing-fields,missing-blocks,paragraph-count-differs,values-differ
data/lint/check:
	@go run ./cmd/packlint -in $(SRD_DIR) -samples 0 -fail "$(LINT_GATED)" >/dev/null \
	  || { go run ./cmd/packlint -in $(SRD_DIR) | sed -n '/^[a-z][a-z] /,$$p'; echo "PROSE LINT: a gated check has findings; see above"; exit 1; }
	@echo "srd prose clean"

## fmt: format all Go source
fmt:
	gofmt -s -w .

## fmt/check: fail if any file is unformatted (mirrors CI)
fmt/check:
	@test -z "$$(gofmt -l cmd internal)" || { gofmt -l cmd internal; exit 1; }

## vet: run go vet (mirrors CI)
vet:
	go vet ./...

## lint: run golangci-lint without adding it to go.mod (mirrors CI)
lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION) run ./...

## web/deps: install frontend dependencies (clean, lockfile-exact)
web/deps:
	cd web && npm ci

## web/dev: run the Vite dev server; it proxies /v1 to this worktree's API
# Keep the commit visible for diagnostics. Development release checks are
# disabled by Vite's build mode: a long-running Vite process and a rebuilt API
# can report different commits, and reloading cannot change Vite's startup env.
web/dev:
	cd web && EASYDND_WEB_PORT=$(WEB_PORT) \
	          EASYDND_WEB_PUBLIC_URL=$(WEB_PUBLIC_URL) \
	          VITE_APP_VERSION=$(VERSION) \
	          EASYDND_API_ORIGIN=http://127.0.0.1:$(API_PORT) npm run dev

## web/lint: typecheck, lint, layer-check and message-check the frontend -- no tests
# Split out from web/check so CI's Check stage can run static analysis without
# paying for the test suite; the Test stage runs web/test.
web/lint:
	cd web && npm run typecheck && npm run lint && npm run lint:layers && npm run check:messages

## web/test: run the frontend suite once, no watch
web/test:
	cd web && npm run test -- --run

## web/check: typecheck, lint, layer-check and test the frontend (mirrors CI)
web/check: web/lint web/test

## web/build: production build into web/dist
# VITE_APP_VERSION is required -- vite.config.ts refuses to build without it,
# so that a bundle can never ship silently reporting "dev".
web/build:
	cd web && VITE_APP_VERSION=$(VERSION) npm run build

## web/icons: regenerate the favicon and the PWA icon set from the palette
web/icons:
	cd web && npm run icons

## web/icons/check: fail if the committed icons differ from the generator
# Not a `diff -rq`, and for a specific reason: the PNG
# encoder's zlib output is deterministic for a given zlib but is not promised
# to be stable across Node versions, so a byte diff would go red on a machine
# whose Node differs from CI's -- failing for a reason that has nothing to do
# with the icons. --check decodes and compares pixels instead. See the note in
# web/scripts/gen-icons.mjs.
web/icons/check:
	@cd web && npm run icons -- --check || { \
	  echo "ICON DRIFT: web/public does not match the palette in src/theme/tokens.ts."; \
	  echo "  Run 'make web/icons' -- or, if you only switched PALETTE_NAME to look"; \
	  echo "  at something, run 'git checkout web/public' and switch it back."; \
	  exit 1; }

## web/release: build exactly what CI ships to the server
web/release: web/build
	tar -czf web.tar.gz -C web/dist .

## spell-icons: generate the per-spell icons -- manual, costs OpenAI credit
# Three steps: build the prompts from the SRD, generate 1024px PNGs into a
# cache outside the repo (the expensive artifact, so it survives worktrees and
# reruns), downscale to the 128px WebPs owned by the SRD pack. Every step skips
# what already exists, so an interrupted run resumes for free; rerolling one
# icon means deleting its webp here and its PNG in the cache. Never part of
# `verify` -- icons are art, and art has no drift check.
SPELL_ICON_CACHE := $(HOME)/.cache/easydnd/spell-icons
spell-icons:
	@$(llm_key); test -n "$$OPENAI_API_KEY" || { \
	  echo "no LLM key: set EASYDND_AGENT_API_KEY in $(DEV_ENV)."; exit 1; }
	node web/scripts/spell-icons.mjs prompts $(SPELL_ICON_CACHE)/prompts.json
	$(llm_key); go run ./cmd/llm images -in $(SPELL_ICON_CACHE)/prompts.json \
	  -out $(SPELL_ICON_CACHE)/png -quality low -background transparent
	node web/scripts/spell-icons.mjs convert $(SPELL_ICON_CACHE)/png

## translate/ru: re-translate the Russian spell prose -- manual, costs OpenAI credit
# `-preserve name` is what makes this a reroll rather than a no-op: -existing
# points at the output file, so without it every leaf already there counts as
# done and the run translates nothing while exiting successfully. Naming the
# leaves to keep re-requests every description and keeps the hand-checked names.
#
# Model and reasoning effort are pinned and explicit because
# data/locale-terms-locked/ru.sources.json records both, and a record that says
# "whatever the alias meant that day" is not a record. Override either on the
# command line to compare settings:
#
#   make translate/ru TRANSLATE_FLAGS=-dry-run        # counts only, no key, no spend
#   make translate/ru TRANSLATE_REASONING=high
#
# Never part of `verify`: it costs money and hits the network.
TRANSLATE_MODEL     ?= gpt-5.4-2026-03-05
TRANSLATE_REASONING ?= medium
TRANSLATE_FLAGS     ?=
translate/ru:
	@$(llm_key); test -n "$$OPENAI_API_KEY" || test -n "$(findstring -dry-run,$(TRANSLATE_FLAGS))" || { \
	  echo "no LLM key: set EASYDND_AGENT_API_KEY in $(DEV_ENV)."; exit 1; }
	$(llm_key); go run ./cmd/llm translate \
	  -in $(SRD_DIR)/i18n/en/spells.json \
	  -out data/pack/srd-5.1/i18n/ru/spells.json \
	  -existing data/pack/srd-5.1/i18n/ru/spells.json \
	  -preserve name \
	  -glossary data/locale-terms-locked/ru.glossary.json \
	  -model $(TRANSLATE_MODEL) \
	  -reasoning $(TRANSLATE_REASONING) \
	  -to ru $(TRANSLATE_FLAGS)

## lint/layers: fail if the inner layers reach for transport or storage
lint/layers:
	@! go list -deps ./internal/domain/... ./internal/usecase/... \
	  | grep -E 'gin-gonic|^net/http$$|^database/sql$$|jackc/pgx|pressly/goose' \
	  || { echo "LAYER VIOLATION: inner layers must not import transport or storage"; exit 1; }
	@echo "layers clean"

## tidy: sync go.mod and go.sum
tidy:
	go mod tidy

## verify: everything CI checks, locally -- two jobs, longest first
# The only gate this project has, so what it costs decides whether it gets run.
# It used to be one serial chain, which meant the Go side's time was added to
# the frontend's rather than spent inside it. CI has always run those two as
# separate jobs; this is the same arrangement locally.
#
# The order of the goals is the schedule. `make -j` starts them left to right
# as slots come free, so `web/test` -- about forty seconds -- is named first
# and everything small happens inside its shadow. Left at the end it would land
# in the last slot and `verify` would cost its length plus everything that ran
# before it, which is most of what the serial version was paying for.
#
# `test/unit` runs AFTER that group, alone, and that is measured rather than
# tidy. Both it and vitest now use every core -- the heavy Go packages run
# their tests in parallel -- and side by side they thrash: a cold suite that
# takes 60s alone took 100s beside vitest, and vitest's 40s became 167s, for
# 184s in all. One after the other is 40s + 60s cold and 40s + 3s with a warm
# test cache, and the second number is the one a developer sees most.
#
# -j2 rather than a bare -j: one of those two jobs is vitest, which forks
# `availableParallelism - 1` workers of its own, so two make jobs is already the
# whole of a four-core machine. VERIFY_JOBS is the knob for a worktree sharing
# the box with a neighbour.
#
# --output-sync=target holds each target's output and prints it whole. Without
# it a failing assertion arrives interleaved with whatever else was mid-run,
# which for the one gate there is would be a bad trade for the seconds. The
# cost is that nothing prints until a target finishes.
#
# `web/check` is left alone: it is what a person runs by hand, and CI's frontend
# job runs `web/lint` and `web/test` as separate steps.
VERIFY_JOBS ?= 2
verify:
	@$(MAKE) --no-print-directory -j$(VERIFY_JOBS) --output-sync=target \
	  web/test web/build web/lint vet build/release pack/check data/lint/check \
	  web/icons/check fmt/check lint/layers lint
	@$(MAKE) --no-print-directory test/unit

## clean: remove build artefacts
clean:
# Not .dev-slot: that is this worktree's identity, and deleting it would move
# the address you reach it on.
	rm -rf $(BINARY) web.tar.gz web/dist web/dev-dist

.PHONY: help build/release run/db test/unit test/race \
        dev dev/up dev/down slots ports \
        preview preview/up \
        db/up db/down db/psql test/db \
        pack/check data/lint data/lint/check \
        fmt fmt/check vet lint lint/layers tidy verify clean \
        web/deps web/dev web/lint web/test web/check web/build web/release \
        web/icons web/icons/check spell-icons
