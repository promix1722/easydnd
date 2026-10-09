# easydnd

@README.md

## Working here

- `make verify` before any commit. There is **no CI on `main`** -- the workflow
  runs on `v*` tags only -- so this is the only gate.
- **Update the docs in the same change that alters logic.** Not for a rename or
  a refactor that keeps behaviour, but whenever a change moves a rule, a route,
  a config key, a layer boundary or a deploy step, the matching page in `docs/`
  is part of that change and not a follow-up: `docs/dnd.md` for the game model,
  `docs/backend.md` for the service, `docs/web.md` for the client,
  `docs/licensing.md` for anything about the SRD data's terms. These documents
  explain *why*, so a stale one is worse than a missing one -- it argues for a
  design the code no longer has.
- **`data/pack/srd-5.1/` is the compendium, and it is hand-maintained.** There
  is no generator and nothing to regenerate: edit the entity JSON, the
  `i18n/<locale>/` bundles and `mechanics.json` in place. The gates are
  `make pack/check` -- the real pack loader, the same validation the server
  runs at startup -- and `make data/lint/check` for the prose. Rows whose
  `provenance.json` entry names `phb`, `xge` or `tce` are owned by the private
  `easydnd-2014` repo's `make export` and are re-applied from there; edit
  those in that repo. The manifest names no files: the loader reads the layout.
- **Pack translations live in the pack, UI captions do not.** A locale is
  `data/pack/srd-5.1/i18n/<tag>/`, one file per collection; adding a language
  is `mkdir` plus files, and a slug the English bundle does not define fails
  the load. The translators' glossary and provenance record are in
  `data/locale-terms-locked/`; see its README.
- **No user-facing English in `web/src/`.** Every caption lives in
  `web/locales/{en,ru}.json` and is reached by key, so translating the client
  never means opening a component. `npm run check:messages` fails on a key
  nothing renders **and on an English key Russian does not translate**, so a
  caption and its Russian land in the same change; `useT` is typed from
  `en.json`, so a key the catalogue does not define will not compile. The Go
  side sends no prose either -- errors travel as a `reason` slug, see
  `docs/backend.md#errors-are-keys-not-sentences`.
- **Never hand-edit `web/public/favicon.svg` or `web/public/icons/`.** They are
  generated from the palette by `web/scripts/gen-icons.mjs`; change
  `PALETTE_NAME` (or the mark) and run `make web/icons`. `make web/icons/check`
  compares the committed tree against the generator and fails on drift -- it
  compares decoded pixels rather than bytes, because zlib output is not stable
  across Node versions.
- Layer rules are enforced, not advisory: `make lint/layers` for Go,
  `npm run lint:layers` for the web client. Inner layers import no framework. The standalone `internal/usecase/spellicon`
  generator owns outbound HTTP; it is excluded only from the HTTP dependency
  restriction and must not be imported by other usecases.
- Configuration is one YAML file (`EASYDND_CONFIG`); individual settings cannot
  be overridden from the environment. See `docs/backend.md#configuration`.
- Deploying is a tag: `git tag -a vX.Y.Z && git push origin vX.Y.Z`. A tag
  ending `-notest` skips CI's Test stage -- both suites, and only the suites --
  for a release whose `make verify` you have just run yourself. The two version
  assertions run in Build and are never skipped. See
  `docs/backend.md#deployment` for what that gives up.
- **`deploy/release-version.sh` is the only place that decides what a release is
  called** -- the tag when the deploy pipeline builds it, a short SHA for
  everything else, a local build of that same tagged commit included. The
  `Makefile` and every job in the deploy workflow call it, because the binary,
  the bundle and the four gates on them all compare that one string. A release
  still *lives* in a directory named by commit SHA; the two are deliberately
  different, and `deploy.sh` bridges them through `releases/<sha>/VERSION`.
- **`deploy/nginx/easydnd.conf` is not applied by a deploy.** It carries the
  whole HTTP caching policy and the live copy is installed by hand; changing it
  here and tagging a release changes nothing on the server.
- **Anywhere `TEST_DATABASE_URL` is set, run `make test/db`, never
  `make test/unit`.** The DB-touching packages TRUNCATE one shared database, so
  they need that target's `-p 1` to take turns. Without it they wipe each other
  and the red lands on whichever lost the race.
