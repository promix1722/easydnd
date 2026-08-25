# easydnd developer tasks. Recipe lines must be TAB-indented.

BINARY      := easydnd
MODULE      := github.com/promix1722/easydnd
CMD         := ./cmd/$(BINARY)
BIN_DIR     := bin
SRD_DIR     := data/srd_5.1
DEV_CONFIG  := config.dev.yaml

# Keep in lockstep with the build step in .github/workflows/deploy.yml.
VERSION     ?= $(shell git rev-parse HEAD 2>/dev/null || echo dev)
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

# Written by config/dev: a whole development config for this worktree's slot.
# Gitignored, never edited by hand -- edit config.dev.yaml, or make a
# config.local.yaml, instead.
DEV_RUN_CONFIG := config.dev-run.yaml

.DEFAULT_GOAL := help

## help: list available targets
# -h because MAKEFILE_LIST holds the optional dev.mk/local.mk includes too, and
# grep prefixes every match with the file it came from once it has more than one.
help:
	@grep -hE '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'

## build/server: build the API binary into bin/
build/server:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) $(CMD)

## build/release: build exactly what CI ships (linux/amd64, static)
build/release:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

## run/server: run the API in development mode, no database
# Unclaimed, this is config.dev.yaml exactly as it always was. Once the
# worktree holds a slot it needs that slot's port and origin instead, so it
# runs the generated config.
run/server: $(if $(SLOT),config/dev)
	go run -ldflags "$(LDFLAGS)" $(CMD) -config $(if $(SLOT),$(DEV_RUN_CONFIG),$(DEV_CONFIG))

## test/unit: run the test suite (~4s)
# No -race here, and that is a deliberate trade rather than an oversight: the
# detector costs roughly 9s against 4s, and it used to cost 46s against 10s
# before the compendium sharing below. A gate slow enough to be worth skipping
# stops being a gate, and since nothing runs on main this is the only one there
# is.
#
# The detector is not gone, it has moved off the path everybody walks. Run
# `make test/race` before tagging. See docs/backend.md#tests.
test/unit:
	go test ./...

## test/race: the whole suite under the race detector (~9s) -- not in `verify`
# atexit_sleep_ms=0 is most of why this is nine seconds and not twenty-five.
# The race runtime sleeps a full second at the exit of every test binary by
# default, which across sixteen test packages is sixteen seconds of an idle
# machine. What the sleep buys is a last chance to check a goroutine still
# running when main returns; nothing here leaves one, because the HTTP tests
# drive httptest in-process and synchronously and internal/app -- which owns
# the only real server lifecycle -- has no tests at all. A race *during* a test
# is reported exactly as it was before, which is what this target is for.
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
# a global TRUNCATE mean the packages have to take turns; only this target
# pays for it, because everywhere else those tests skip.
test/db:
	TEST_DATABASE_URL=$(TEST_DATABASE_URL) go test -p 1 ./...

## config/dev: write the generated development config for this worktree
# Written whole rather than appended to config.dev.yaml, because a slot needs
# http.port and auth.rp_origins -- and a second `auth:` block in one file is a
# duplicate mapping key, which the loader rejects outright. What it leaves out
# the loader defaults for; data.srd_dir is already data/srd_5.1.
#
# No auth.session_secret: development invents one per process and says so,
# which is honest given that a restart also empties the character store.
config/dev:
	@{ printf 'env: development\n'; \
	   printf 'log:\n  format: text\n  level: debug\n'; \
	   printf 'http:\n  port: "%s"\n' '$(API_PORT)'; \
	   printf 'auth:\n  rp_id: %s\n  rp_origins:\n    - http://localhost:%s\n' '$(RP_ID)' '$(WEB_PORT)'; \
	   $(if $(WEB_PUBLIC_URL),printf '    - %s\n' '$(WEB_PUBLIC_URL)';) \
	   $(if $(DEV_DB_URL),printf 'db:\n  url: %s\n' '$(DEV_DB_URL)';) } > $(DEV_RUN_CONFIG)
	@chmod 600 $(DEV_RUN_CONFIG)
	@echo "wrote $(DEV_RUN_CONFIG)"

## run/db: run the API in development mode against this worktree's Postgres
# config.local.yaml wins if you have made one (it is gitignored), which is the
# hook for anything the generated file cannot carry -- auth.google, say. Its
# ports and origins are then yours to keep correct: nothing rewrites it.
run/db: DEV_DB_URL := $(TEST_DATABASE_URL)
run/db: $(if $(wildcard config.local.yaml),,config/dev)
	@if [ -f config.local.yaml ]; then \
	  echo "using config.local.yaml -- its ports and origins are yours to keep correct"; \
	  go run -ldflags "$(LDFLAGS)" $(CMD) -config config.local.yaml; \
	else \
	  go run -ldflags "$(LDFLAGS)" $(CMD) -config $(DEV_RUN_CONFIG); \
	fi

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

## test/cover: run tests and summarise coverage
test/cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

## data/srd: regenerate data/srd_5.1 from the vendored SRD dump
data/srd:
	go run ./cmd/srdgen

## data/srd/check: fail if the committed data differs from srdgen's output
data/srd/check:
	@tmp=$$(mktemp -d); \
	 go run ./cmd/srdgen -out $$tmp >/dev/null || { rm -rf $$tmp; exit 1; }; \
	 if ! diff -rq $(SRD_DIR) $$tmp >/dev/null; then \
	   diff -rq $(SRD_DIR) $$tmp || true; \
	   rm -rf $$tmp; \
	   echo "DATA DRIFT: $(SRD_DIR) does not match srdgen; run 'make data/srd'"; \
	   exit 1; \
	 fi; \
	 rm -rf $$tmp; \
	 echo "srd data current"

## fmt: format all Go source
fmt:
	gofmt -s -w .

## fmt/check: fail if any file is unformatted (mirrors CI)
fmt/check:
	@test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }

## vet: run go vet (mirrors CI)
vet:
	go vet ./...

## lint: run golangci-lint without adding it to go.mod
lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION) run ./...

## web/deps: install frontend dependencies (clean, lockfile-exact)
web/deps:
	cd web && npm ci

## web/dev: run the Vite dev server; it proxies /v1 to this worktree's API
web/dev:
	cd web && EASYDND_WEB_PORT=$(WEB_PORT) \
	          EASYDND_WEB_PUBLIC_URL=$(WEB_PUBLIC_URL) \
	          EASYDND_API_ORIGIN=http://127.0.0.1:$(API_PORT) npm run dev

## web/lint: typecheck, lint and layer-check the frontend -- no tests
# Split out from web/check so CI's Check stage can run static analysis without
# paying for the test suite; the Test stage runs web/test.
web/lint:
	cd web && npm run typecheck && npm run lint && npm run lint:layers

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

## web/release: build exactly what CI ships to the server
web/release: web/build
	tar -czf web.tar.gz -C web/dist .

## lint/layers: fail if the inner layers reach for transport or storage
lint/layers:
	@! go list -deps ./internal/domain/... ./internal/usecase/... \
	  | grep -E 'gin-gonic|^net/http$$|^database/sql$$|jackc/pgx|pressly/goose' \
	  || { echo "LAYER VIOLATION: inner layers must not import transport or storage"; exit 1; }
	@echo "layers clean"

## tidy: sync go.mod and go.sum
tidy:
	go mod tidy

## verify: everything CI checks, locally
verify: fmt/check vet lint/layers data/srd/check test/unit build/release web/check web/build

## clean: remove build artefacts
clean:
# Not .dev-slot: that is this worktree's identity, and deleting it would move
# the address you reach it on.
	rm -rf $(BIN_DIR) $(BINARY) coverage.out web.tar.gz web/dist web/dev-dist $(DEV_RUN_CONFIG)

.PHONY: help build/server build/release run/server run/db test/unit test/race test/cover \
        dev dev/up dev/down slots ports config/dev \
        db/up db/down db/psql test/db \
        data/srd data/srd/check \
        fmt fmt/check vet lint lint/layers tidy verify clean \
        web/deps web/dev web/lint web/test web/check web/build web/release
