# JSON rule packs

The server runs characters against immutable, versioned packs. `pack` and
`addon` mean the same artifact. The generated SRD is the base pack; configured
addons pass through the same decoder, dependency resolver and compiler.
The original design discussion is in [packs-plan.md](packs-plan.md).

This delivery covers files, the core evaluator, character locks, revisions,
resource events and migration APIs. Pack editing, JSON uploads, publishing,
private storage, and group sharing are available through the Homebrew section;
see Homebrew authoring below. Character imports can also compile temporary,
session-scoped private definitions; see [agent.md](agent.md#custom-content).
These releases use the same validator and immutable locks, never enter the
default catalogue, and are stored in `private_releases`. The
existing browser still uses its original ability editor; its resource displays
read `resources.pools` and `resources.parameters`.

## Pack artwork

Equipment and magic items share a pack-local asset library. Each definition
may set `"icon": "sword"`; any number of rows in either collection can use
that label. Labels are stable IDs, independent of item names and translations.
Directory and ZIP manifests declare `"icons/items/sword": "item-icons/sword.webp"`.
Portable JSON stores each image once in
`"icons": { "items": { "sword": "<base64 WebP bytes>" } }`.
An item's label must exist in its own pack; it cannot point into another pack.
Missing references, invalid labels and malformed artwork fail validation.
Artwork remains optional for older and custom packs.

Item icons are transparent 128×128 WebPs. The SRD uses original pixel artwork,
with shared silhouettes for variants. The artwork lives in
`data/pack/srd-5.1/item-icons/<label>.webp` and the assignment is the `icon`
field on each equipment and magic-item row, edited in place; the loader
rejects a label with no file behind it. A row without a label has no icon --
the non-SRD items reuse an existing label where one fits and go without where
none does. New artwork and assignments require a new release.

To convert approved PNGs from `output/imagegen/item-samples/` and
`output/imagegen/item-icons/`, run `node web/scripts/item-icons.mjs` after
installing the web dependencies. This uses nearest-neighbor resizing and
lossless WebP, checks dimensions and transparency, and preserves source PNGs.
Then run `make pack/check`: the loader finds the files by name, so a new WebP
under `item-icons/` is in the pack the moment a row names it.

Item detail, collection, and search responses expose an optional `icon` data
URL, just as spells do. The UI displays item artwork at 88×88 alongside its
name in inventory, equipped slots, equip selectors and starting choices.

### Spell artwork

Artwork is optional and belongs to an immutable release. A directory laid out
by convention holds `spell-icons/<local-spell-id>.webp` and says nothing more;
a manifest that names its files declares them, using local spell IDs:

```json
"files": {
  "icons/spells/guiding-mark": "spell-icons/guiding-mark.webp"
}
```

Portable JSON carries the same bytes as base64:

```json
"icons": { "spells": { "guiding-mark": "<base64 WebP bytes>" } }
```

Only valid 128×128 WebPs naming spells defined in that pack are accepted.
Artwork counts toward the existing 64 MiB pack limit, survives imports,
exports, draft editing, forks, publishing and sharing, and participates in the
release digest. Updating an icon requires a new release version. Existing
packs without icons retain their digests and remain valid. Existing catalog
summary/detail routes expose an optional `icon` data URL; no image routes or
separate image storage are used.

The SRD pack's version is the `version` field of
`data/pack/srd-5.1/pack-manifest.json` (now `2.0.0`), bumped by hand with the
content it describes. Preserve
`data.pack_archive` when deploying so characters pinned to an earlier release
continue to use the archived bytes; startup refuses an archived release whose
bytes have changed under the same version. An explicit
`data.default_packs.srd-2014` pin must be updated to select the new version.

An `equipment` or `magic-items` row may carry `slot`: where the item is worn or
wielded, one of `head`, `neck`, `back`, `body`, `arms`, `waist`, `feet`,
`ring`, `main-hand`, `off-hand`. Gloves and gauntlets are `arms`, as bracers
are; `hands`, the name that slot had before release 1.5.0, is still accepted
and read as `arms`. Left out, the loader derives it from what the item is --
armor `body`, a shield `off-hand`, a weapon or focus `main-hand`, a magic ring
`ring`, a wand/staff/rod `main-hand` -- and anything else is only carried.
Write it for a wondrous item that is worn (a cloak, a belt) or to override the
shape; any other unknown value fails the load.

The committed `data/pack/srd-5.1/spell-icons/` directory holds one WebP per
spell, SRD and non-SRD alike. `make spell-icons` fills whatever is missing
through the image API and never reads an external checkout.

## Load and export

```sh
# Validate the SRD pack and print its exact lock -- what `make pack/check` runs.
go run ./cmd/pack -in data/pack/srd-5.1

# Validate the base with the private descriptions overlay installed beside it.
go run ./cmd/pack -in data/pack/srd-5.1,../easydnd-2014/pack

# Export the first input; supply its dependencies too. Destinations must be new.
go run ./cmd/pack -in ../easydnd-2014/pack,data/pack/srd-5.1 -out /tmp/personal.json
go run ./cmd/pack -in data/pack/srd-5.1 -out /tmp/srd-pack -directory
```

### Prose formats

A locale entry is `{name, desc, fields, blocks}`, and each of those is one of
two formats. The format belongs to the field; nothing on the wire marks it.

| Field | Format |
|---|---|
| `name`, every `fields.*` value, and the name-only bundles (`terms`, `resources`) | **plain text**: one line, shown as written |
| `desc[]` and every `blocks.*[]` | **Markdown**, one array element per line-level block |

A Markdown element is exactly one of: a paragraph, a `##### ` heading, one
`- ` list item, or one table row. A table is a run of elements, each with outer
pipes, whose second element is the `|---|---|` separator. Inline, `***Name.***`
opens a named paragraph and `**bold**` is bold. There is no HTML and there are
no links; the client renders with `skipHtml`.

One block per element rather than one string per table is what keeps a
translation aligned with its source path by path -- `/desc/7` is the same row
in every locale. The client puts the document back together with `joinProse`
(`web/src/ui/prose.ts`): a blank line between blocks, a single newline between
adjacent rows or adjacent list items, which is what GFM needs to see one table
rather than a column of paragraphs.

### Linting the prose

`cmd/pack` proves a pack loads; it says nothing about whether its text reads.
`cmd/packlint` reads `i18n/<locale>/*.json` of a pack *directory* and reports
what a loader cannot see:

- converter markup that leaked into a sentence (`{@item ...}`, a bare `phb`),
  punctuation debris, stray whitespace;
- breaches of the formats above -- `malformed-table` for a row without outer
  pipes or a table without its separator, `markdown-in-plain-field` for
  Markdown in a name or a `fields.*` value;
- for every locale other than the default one, a coverage table plus what is
  missing, what was copied instead of translated, and descriptions whose dice
  or distances disagree with the source -- the cheap sign that a translation
  was attached to the wrong entry;
- with `-glossary`, source terms whose preferred translation the entry never
  uses (`glossary-term`).

```sh
make data/lint                                   # the SRD, report only
make data/lint PACK=../easydnd-2014/pack         # any pack directory
make data/lint/check                             # the gate `make verify` runs
go run ./cmd/packlint -in <dir> -check values-differ   # every finding of one check
```

It exits 0 unless `-fail` names a check with findings. `data/lint/check` is
that list for the SRD: the checks that have been driven to zero and are held
there. `glossary-term` is not in it and is not meant to be -- it matches word
stems, so it is a reading list, not a verdict. The Russian-only checks
(Cyrillic dice, metric units, `[english]` in brackets, `nonstandard-abbreviation`)
encode the rules in `data/locale-terms-locked/README.md`.

Configure the server through the existing YAML file:

```yaml
data:
  srd_dir: data/pack/srd-5.1
  pack_files:
    - ../easydnd-2014/pack
  autoload_packs:
    - path: /path/to/another-pack-repository
      id: another-core # optional namespace override
  default_packs:
    srd-2014: "2.0.0"
    easydnd-2014-personal: "1.0.0"
  pack_archive: .pack-archive
```

`pack_files` installs additional files or directories. `default_packs` selects
roots; dependency ranges are resolved from installed releases only. When omitted,
the explicitly configured inputs become exact default roots. That lock, whole,
is the default: it is what a new character is pinned to and what the catalogue
manifest reports as `defaultRules` -- every root in it, not only the base, so a
pack listed in `pack_files` reaches new characters without anybody picking it
(`Authoring.Default`). An archive retains
portable releases by digest and makes older locks loadable after restart. Keep
it outside a deployment's release directory. Changing content under an archived
ID/version is an error. Bump the version instead. The archive never supplies
new default roots by itself, and startup makes no network requests.

Every pack other than the base SRD has its slugs qualified with its ID at load
(`dnd-2014/wizard`), and so has every field that *refers* to one. Two references
are qualified by shape rather than by key, because the key means other things
elsewhere: an option set of kind `equipment-category` has its `category`
qualified (a weapon's `category` is "simple" and is left alone), and an ideal's
`alignments` are qualified like any slug list. Both are checked: a category or
an alignment that does not exist fails the load. Left bare, "any arcane focus"
asked a namespaced pack for `arcane-foci`, found only `pack/arcane-foci`, and
offered the player nothing to choose.

`autoload_packs` installs additional common packs at startup without adding them
to the default roots. Each entry's `path` names a pack directory, or a repository
whose `pack/` child is the pack directory. A manifest at the configured root
takes precedence; the loader does not recursively scan vendor or source folders.
Missing folders and invalid packs fail startup. Restart after changing files;
there is no file watcher. These packs appear in Homebrew, the pack selector and
spell browsing alongside SRD, and need no database import or publication step.

When `autoload_packs` points at a repository containing `pack/`, matching
`spell-icons/<local-spell-slug>.webp` files beside `pack/` are adopted into that
release. Manifest-declared icons take precedence. The same 128×128 WebP and
pack-size validation applies; artwork travels in exports and archives. No
files are written back to the source repository.

An optional `id` overrides the source manifest ID and rewrites internal references
using the same rules as pack import. It preserves the source version, translations
and attribution, and computes a new digest. The source files remain untouched.
This lets a replacement core whose source ID is `srd-2014` coexist with the built-in
SRD as a separately selectable pack. Dependencies on another renamed pack must
already declare its installed ID. Independent cores are selected separately;
combining two core providers still fails validation. Autoloaded releases use the
same immutable archive policy as `pack_files`.

### Common and private disk packs

A pack installed from disk belongs to one of two groups, and which one is
decided by the key that names it:

| | Key | Who has it |
|---|---|---|
| **common** | `data.pack_files`, `data.autoload_packs` | every session, guests included |
| **private** | `data.private_pack_files` | a **superadmin**, any account a superadmin has handed it to, and the members of any group a superadmin has shared it with |

A private pack is installed and compiled at startup like any other, and then
kept out of everything that is the same for everybody: it is never a default
root, so it is not in the default lock, the global `/v1/catalog` or a
character created without choosing rules. For an account that does not have it
the pack does not exist -- not in `GET /v1/packs`, the pack selector, the spell
browser or `export`, and naming it in a lock is refused exactly as a stranger's
homebrew pack is. The choke point is the one that already existed,
`Service.available` in `internal/usecase/pack`: an unowned record marked
`Restricted` is allowed only for a superadmin or an account it was granted
to, and the group-share loop below it grants it to a table.

**A superadmin** is an account named in `auth.superadmins`, by the *verified*
email of a linked Google account or by account id. An unverified email is
anybody's to claim and never matches; a passkey-only account has no email and
can only be named by id, which is how `config.dev.yaml` names `dev:master`.
The same list opens the admin listings and every sheet for reading; see
[backend.md](backend.md#a-superadmin-reads-everything-and-writes-one-thing).

**Granting** reuses pack sharing. On a group they belong to, a superadmin sees
a *Private packs* tab and shares a release with the table through
`POST /v1/groups/:id/packs`; its members then find the pack in their own pack
selector. Removing the share (`DELETE`, the contributor or a DM) takes it away
from new characters. Two things follow from how sharing already worked:

- A character already built on the pack keeps it -- its lock is retained, as
  for any unshared homebrew pack -- and anybody who may read that character's
  sheet, through a group or a public link, reads it whole.
- A granted member selects and reads the pack but cannot `export` it; only a
  superadmin takes a copy.
- A member who was granted the pack may publish homebrew that depends on it,
  but cannot share that homebrew with another table: a restricted dependency
  is passed on only by a superadmin (`pack.dependencyPrivate`).

**Granting to one account** needs no table. On the admin Players tab a row's
*Private packs* action ticks the packs that account has
(`PUT /v1/admin/players/:id/packs`), and they appear in that player's pack
selector. The same three rules hold -- a character built on it keeps it after
the tick is removed, the player cannot export it, and cannot share it or
homebrew built on it with a table -- and the grant follows the pack id, so it
survives the pack being replaced on disk by a newer version.

Privacy is by **pack ID**, taken from the folders configured at this startup.
With `data.pack_archive` set, drop a path from `private_pack_files` and its
archived releases load as ordinary installed packs -- remove them from the
archive too.

Neither committed config names a private pack, because the path is a directory
on one machine: it is `EASYDND_PRIVATE_PACK_FILES` in the env file. In
development that is `~/config/easydnd/dev.env`, pointing at the
`easydnd-2014` checkout -- the descriptions-only overlay
(`easydnd-2014-personal`, see [Prose overlays](#prose-overlays)). In production
the pack is copied by hand, from a machine that has it:

```sh
deploy/push-private-pack.sh ~/projects/easydnd-2014/pack   # [ssh-host], default `easydnd`
```

It compiles the pack against this checkout's SRD, copies it to
`/opt/easydnd/private-packs/<pack id>/` (`root:easydnd`, outside `releases/`,
so no deploy replaces it and no prune removes it) and restarts the service,
putting the previous copy back if the service does not answer. The **first**
push restarts nothing and prints the line to add to `/etc/easydnd/prod.env`:
a path that does not exist is a startup error, so the pack has to be there
before the server is told about it.

The archive preserves packs, **not characters**. Character logs, checkpoints,
folders, shares and games are in PostgreSQL when `db.url` is set, and in
memory otherwise.

## File contract

A portable file has `manifest`, `entities`, `mechanics` and `locales`. The test
fixture [tactician.json](../internal/adapter/catalog/file/testdata/tactician.json)
introduces a subclass, race, spell, tool, feature, numeric modifier, scaling
dice pool and an action that spends it -- every kind of thing an addon can
say, which is why the loader's tests are built on it. It is a test input, not
shipped content.

```json
{
  "manifest": {
    "schemaVersion": 1,
    "id": "my-pack",
    "version": "1.0.0",
    "edition": "2014",
    "semantics": "1",
    "requires": ["resources.v1"],
    "dependencies": [{"id": "srd-2014", "version": "^1.0.0"}],
    "defaultLocale": "en"
  },
  "entities": {},
  "mechanics": {},
  "locales": {"en": {}}
}
```

A directory contains `manifest.json` (the SRD uses `pack-manifest.json` to
coexist with its old catalogue index) and either a `files` map or a layout.
The map names each logical key's file:

```json
{
  "entities/subclasses": "subclasses.json",
  "mechanics": "mechanics.json",
  "locales/en/subclasses": "i18n/en/subclasses.json",
  "icons/spells/guiding-mark": "spell-icons/guiding-mark.webp"
}
```

A manifest that names no files is read by convention instead: each collection
in `MechanicsFiles()` as `<collection>.json` at the root, `mechanics.json`,
`provenance.json`, every `i18n/<tag>/<bundle>.json`, every
`spell-icons/<id>.webp` and `item-icons/<label>.webp`
(`conventionalFiles`, `internal/adapter/catalog/file/pack_codec.go`). That is
what a hand-maintained pack needs: the SRD's map had grown to 530 lines, one
per icon, and a list that long goes stale with the next WebP or the next
language. A map that is present wins, so a portable export -- which
`SavePackDirectory` writes with an explicit map, in this same layout -- is
read exactly as written. ZIP imports follow the same rule.

File paths are relative and confined to the root, including symlink resolution.
Input is bounded to 64 MiB and nesting depth 64. Duplicate keys, unknown typed
fields, unsupported capabilities, unresolved references, missing dependencies,
dependency cycles and conflicting definitions are errors. JSON Schema validates
the envelope; strict typed decoding and semantic checks validate the contents.
The wire types in `internal/adapter/catalog/file` are the contract: whatever
writes a pack -- `cmd/pack`, the homebrew editor -- produces them, and the
loader is the one reader.

Pack IDs are lowercase letters/digits/hyphens starting with a letter. Local
entity, rule, resource and action IDs contain lowercase letters/digits/hyphens.
References in files may be local (`subclass:tactician`) or canonical
(`srd-2014:class:fighter`). Cross-pack references require an explicit dependency.
Versions are in the lock, never in an entity ID. The HTTP compatibility encoding
is `kind:pack/local`; base refs retain `kind:local` aliases. Display names do not
identify anything. Classes and races discover addon children from reverse
indexes, so adding a subclass/subrace does not edit a parent release.

`mechanics` supports:

| Field | Purpose |
| --- | --- |
| `core` | One provider: score limits/generation metadata, proficiency/modifier and HP formulas, base AC/save DC, senses, multiclass slots, recovery |
| `rules` | Owner, minimum level, optional condition, choices, grants and numeric effects |
| `resources` | Spendable pools or derived parameters, thresholds/expressions, recovery and sharing |
| `casting` | Explicit class or subclass shared/independent profiles; contribution numerator/denominator, start level and floor/ceil rounding per class |
| `actions` | Part of the turn (`kind`), optional owner, condition, resource costs and manual outcomes |
| `overrides` | Version-guarded replacement of a dependency's complete rule/resource definition |

Expressions are integer ASTs: `constant`, `read`, `add`, `subtract`, `multiply`,
`min`, `max`, `floor-div`, `ceil-div`, `eq`, `gte`, `lte`, `and`, `or`, `not`.
They have bounded depth/arity, exact arithmetic and JSON-safe integer limits.
Inputs include `level`, `proficiency`, `class:<id>`, `ability:<id>` and
`modifier:<id>`. Core formulas additionally receive `score` or `hitDie` in their
respective environments. Dice are descriptions; evaluation never rolls them.
A new operation needs a new supported engine capability.

Two more inputs say what is worn: `equipped:armor` and `equipped:shield`, each
1 or 0. They exist for rules worded "while you are not wearing armor", and are
readable **only in the value of an effect on something other than an ability
score**. That restriction is the order of projection rather than caution: a
rule's `when` and its ability effects are settled before the starting kit is
granted and the log's equipment changes are applied, so there they could only
ever read "nothing equipped". A pack that reads them anywhere else is refused
at load. The base data's Unarmored Defense is the example:

```json
{"id": "barbarian-unarmored-defense", "owner": "feature:barbarian-unarmored-defense", "minimumLevel": 1,
 "effects": [{"op": "add", "target": "status.armorClass", "value": {"op": "multiply", "args": [
   {"op": "not", "args": [{"op": "read", "ref": "equipped:armor"}]},
   {"op": "max", "args": [{"op": "constant", "value": 0}, {"op": "read", "ref": "modifier:con"}]}]}}]}
```

Characters retain exactly STR, DEX, CON, INT, WIS and CHA. Packs may modify
these scores, but cannot introduce additional characteristics. The example pack
uses a Wisdom bonus; version 2.0.0 removes its former demonstration Luck score.

Numeric effects support `add`, `max`, `set` on ability scores, AC, initiative,
passive Perception, HP maximum and movement speeds. Grants support features,
traits, feats, spells, proficiencies, languages and equipment. Rules can expose
choices through the existing prompt/answer grammar. A class's
`startingEquipmentOptions` carry a `slot` apiece -- `body`, `main-hand`,
`off-hand`, `backup`, `pack`, `focus` or `instrument` -- which titles the
builder's card and equips nothing: every kit item is carried until the
character is dressed at the end (see docs/dnd.md); a choice without one is titled by its options. Ability dependencies are
ordered and cyclic reads rejected. Mixed combination operations or multiple
`set` effects on one target are rejected. DM change events remain final overrides.
Rules with no automated effect must explicitly declare `manual: true`.

Overrides require a canonical `target`, compatible `version`, and exactly one
replacement `rule` or `resource`. They retain the target identity and existing
localized name. Two replacements of the same target fail; file order never
selects a winner. Arbitrary JSON patches and whole-entity replacement are not
supported operations.

## Prose overlays

A pack may ship prose for entities it does not define. A locale bundle's key
is normally a local slug; it may instead be a canonical reference into a
declared dependency -- `srd-2014:spell:fireball` in `locales/en/spells` -- and
then names that pack's entity (`foreignProseKey`,
`internal/adapter/catalog/file/pack_semantics.go`). The key's kind has to match
the bundle's collection, and the pack it names has to be a dependency; a bare
slug in a collection the pack does not define is still an error, as is a
canonical key into a pack it never declared.

At compile the prose is merged **fill-empty**: the overlay's fields land in
whatever the base left blank, and a field both packs set -- a name, a
description, one `fields` or `blocks` key -- fails the load with
`localized identity <collection>/<id>: desc set twice` (`fillProse`,
`locale.go`). That is the same rule overrides follow: load order never picks a
winner. A field counts as set once the locale *or its fallback* supplies it, so
an overlay cannot slip a Russian description under an English one the base
already has. A canonical key naming no compiled entity fails too
(`localized identity without entity`), because a typo there would otherwise
vanish without a trace.

This is what makes a **descriptions-only pack** possible: no entities, no
mechanics, no icons, just bundles keyed into its base. The private
`easydnd-2014` repository is one -- `easydnd-2014-personal`, depending on
`srd-2014` 2.x -- and it carries the text the base deliberately does not ship
(see [licensing.md](licensing.md)). Loading it adds descriptions to the
entries that lack them and duplicates nothing; leaving it out leaves those
entries with a name, their mechanics and an empty description. Whole-entity
replacement is still not a thing a pack can do; an overlay only fills blanks.

## Resources and temporal events

A resource declares `id`, `owner`, `minimumLevel`, `kind` (`pool` or `parameter`),
`input`, and `rows` or `capacity`. Threshold rows use `from`, `capacity`, optional
`dice`, `slotLevel`, exact `rational`, `boolean` and `text`; the greatest matching threshold applies. A capacity
expression can accompany rows to compute uses independently of die/slot scaling.
`sharedKey` plus `combine: max|sum` explicitly merges capacities; recovery policies
must agree. Unrelated resources with identical display names stay separate.

The base data includes Pact Magic, an oath-gated Paladin Channel Divinity grant,
Cleric sharing, ability-based Bardic Inspiration and conditional recovery.
Limited-use features are pools too, not prose: Second Wind, Wild Shape uses,
Lay on Hands (a capacity expression, paladin level times five), Divine Sense,
Cleansing Touch, Arcane Recovery, Signature Spells, Divine Intervention, Stroke
of Luck, Eldritch Master, the Land, Open Hand and Fiend subclass uses, and two
trait-owned ones, Breath Weapon and Relentless Endurance. A pool with no limit
is declared with capacity 9999, which the browser reads as unlimited.
Ordinary spell slots and Hit Dice retain family instance IDs. `resources.pools`
is authoritative for usage; `resources.parameters` separates damage/scaling
values from consumables. The older slot/class arrays remain compatibility views.

The usage events below are the character's own record. The browser's game
tracker does not write them: it keeps a spent count per pool on the game entry,
so a use spent at one table is not spent at another. See
[backend.md](backend.md#active-game-entries).

```json
{"type":"resource.spent","resource":"example/combat-dice","amount":1}
{"type":"rest.completed","trigger":"short-rest"}
{"type":"action.used","ref":"example:action:maneuver"}
{"type":"rest.completed","trigger":"long-rest","allocations":{"hit-dice/fighter":2,"hit-dice/wizard":1}}
```

Each event is validated at its position in the log. Later levels cannot legalize
an earlier overspend. Capacity changes preserve spent uses; available uses are
`max(0, maximum - used)`. Recovery operations are `all`, `amount` or `budget`;
shared budgets require explicit recorded allocation. Conditional recovery uses
`when`. An action verifies ownership and all costs before charging any pool.
Its rolls and other outcomes remain manual. The sheet exposes localized
`packActions`, affordability, `manualRules` and contributions with rule/owner
and originating event identity where directly attributable.

## Action tags

The character sheet's action list is whatever the pack says it is. There are
two ways to put something on it, and no third in code.

**Tag an entry.** A feature, trait, feat, equipment item or magic item may carry
an `action` object:

```json
{"slug": "second-wind", "class": "fighter", "level": 1,
 "action": {"kind": "bonus-action", "uses": "second-wind"}}
```

`kind` is `action`, `bonus-action`, `reaction` or `free-action` -- the last for
something that rides on another action, like Action Surge or a smite. `uses` is
optional and names the resource pool the action spends; it is a slug field, so
it is namespaced and reference-checked like any other. The entry's own `name`
and `desc` are the action's, which is why the tag is two fields: there is no
second copy of the prose to keep in step. A character has the action while it
holds the entry; for an item that means equipped.

**Declare a standalone action** in `mechanics.actions`, for what is not an
entry. `kind` defaults to `action`. `owner` is optional: without one the action
belongs to every character and may carry no `minimumLevel`, `when` or `costs`,
because there is nothing to be eligible for or to afford. Its `actions` locale
entry needs a `name` and may carry a Markdown `desc`.

The SRD pack uses both. Its fourteen unowned actions -- Attack through Use an
Object, Grapple, Shove, Two-Weapon Fighting, Opportunity Attack -- are in
`data/pack/srd-5.1/mechanics.json` with their text in
`i18n/<locale>/actions.json`. Its tags sit on the entries themselves, the
`action` field of a feature, trait or item row: the upstream dump said "as a
bonus action" only in prose, so every tag was authored by hand, and now that
the pack is edited in place the tag lives where it applies rather than in a
side table that had to be merged in.

Where a feature exists once per tier (`bardic-inspiration-d6`, `-d8`, ...),
only the first is tagged, or a tenth-level bard would list it three times.

Equipped weapons need no tag; their attack is derived from `weapon`. Unowned
actions are not in `packActions` -- that list is what can be *spent*.

## Revisions and migrations

Characters keep editable ordered build choices. Events have stable IDs and a
schema version; sequence is their current position. The record's `revision`
increases on every write, including a same-length edit. HTTP writes send
`expectedRevision`; `expectedSeq` remains a positional check. Existing clients
without a revision field fall back to their sequence, which safely refuses a
write once the values diverge. Refresh and send the returned revision.

The init event pins a `RulesLock`: edition, evaluator semantics, and every exact
pack ID/version/SHA-256 digest. Lists, sheets, prompts, copies and shared-game
reads load that context. An installed update never changes an old character.
Missing releases or unsupported semantics fail closed. The catalogue manifest
returns the default lock, and `GET /v1/characters/:id/catalog/:collection` serves
an owned character's pinned content.

`POST /v1/characters/:id/rules?dryRun=true` accepts `expectedRevision`, `rules`
(the proposed lock), and optional `entities`, `prompts`, `options`, `paths` mappings. Resource references in usage
and allocation events follow explicit entity mappings; path mappings handle
renamed score/stat paths.
It replays without writing and reports before/after sections and invalid event
identities. Remove `dryRun=true` to revalidate and commit with revision CAS; a
checkpoint retains the original log/lock. No invalid choices are silently dropped.
`POST /v1/characters/:id/rules/restore?dryRun=true` takes `expectedRevision` and
`checkpoint` index. Applying it saves the current build as another checkpoint.
Later edits therefore remain recoverable.

The existing foreign-sheet importer imports a **snapshot** under the configured
context and reports unresolved content. It does not claim to reconstruct a
historical event log. Unsupported event/pack schema versions fail rather than
being guessed. Version 0 events in existing memory fixtures are upgraded to 1 at
the repository boundary.

## Release and localization policy

Edit the base pack in place -- mechanics in `data/pack/srd-5.1/mechanics.json`,
entities in the collection files, translations in `i18n/<locale>/` -- and bump
`version` in `pack-manifest.json` for a new published artifact; `make
pack/check` is the gate. SemVer communicates compatibility:
new optional content normally increments minor; renamed/removed IDs or changed
choice contracts increment major; compatible corrections increment patch.
All released bytes remain immutable, including translation changes.

The digest is SHA-256 of the logical portable document after canonical JSON
serialization (RFC 8785). Physical paths and entity-collection ordering are
excluded. Ordered choices/progression rows remain semantic. A digest detects
content changes; it is not a publisher signature. No remote installation or
executable pack code is supported.

Locales use BCP 47 tags and separate prose from mechanics. Resolution follows
requested tag, parent tags, then the owning pack's default, per field. Every
entity/resource/action needs a default name. A missing translation falls back;
explicit empty map fields/blocks can clear those values. HTTP negotiates the
installed content locales; the browser's own chrome currently remains en/ru.
Lock plus locale identifies a compiled context, preventing cross-version cache
mixing. Configured packs are operator-installed public catalogue content. Private releases
use the authoring service access model described below.

Research underlying these contracts: [SemVer](https://semver.org/),
[JSON Schema 2020-12](https://json-schema.org/draft/2020-12),
[RFC 8785](https://www.rfc-editor.org/rfc/rfc8785),
[RFC 4647](https://www.rfc-editor.org/rfc/rfc4647), and the
[official SRD 5.1](https://media.wizards.com/2023/downloads/dnd/SRD_CC_v5.1.pdf).

## Builder selection policy

Casting profiles may set `selection` (`known`, `prepared`, `spellbook`),
`prepareDivisor`, `bookStart`, `bookPerLevel`, `replaceKnown` and
`expandedSubclass`. These control acquisition and preparation independently of
slot progression.

A profile is keyed by a class, or by a **subclass** that casts when its class
does not (Arcane Trickster, Eldritch Knight). A subclass profile must also set
`list`, the class whose spell list it draws on, and `ability`, its
spellcasting ability -- a class's profile needs neither, because the class has
both. The subclass's `class-levels` rows carry its `spellSlots`,
`cantripsKnown` and `spellsKnown`, one row per level it casts at. A profile
keyed by something that is neither a class nor a subclass, or a subclass
profile missing either field, is refused at load. `spellBenefits` attach automatic spells or counted spell
choices to an owner at a minimum level, with class/any-list/spellbook eligibility
and a separate purpose such as Arcanum or mastery. Racial benefits can declare
their casting ability. Casting profiles may be keyed by a subclass to use its
advancement rows at the parent class's level. A selected subclass profile takes
precedence over the class profile. Subclass profiles require `list` (a spell-list
class) and `ability` (one of the six standard scores); class profiles may also
use these overrides. Spell acquisition and single-caster slots use the owner's
advancement table, while multiclass slots use the profile's contribution and
rounding. Choice IDs and sources belong to the subclass, so changing an
archetype cannot reuse its previous spell choices.

Expressions can read `equipped:armor` and `equipped:shield` as 0/1 flags for
catalog body armor and shields currently equipped. Backpack items do not set
these flags.

`choiceRequirements` gate conditional equipment offers
on any of a list of proficiencies. Referenced owners, classes, spells and
proficiencies must resolve in the pinned catalogue.

The SRD defines these in `data/pack/srd-5.1/mechanics.json`, edited in place.
Older profiles that omit selection policy keep their
previous behavior. Character rules locks remain authoritative, so installing a
new policy does not silently migrate existing pinned characters.

## Homebrew authoring

`/homebrew` manages personal drafts and immutable published releases. Guests use
exactly the same authoring endpoints; their stored guest identity owns the packs,
so access still depends on retaining that guest session. PostgreSQL persists packs
and group shares when configured. Development without a database uses memory.

The visual editor is described by the existing typed wire definitions, including
all entity collections, locale bundles and recursive mechanics. It writes the
same portable JSON as the CLI. Drafts may be incomplete; saving uses an expected
revision, while publishing requires validation and compilation in every supplied
locale. Published versions cannot be overwritten. To publish another version,
edit the draft's manifest version. Archiving hides a pack from new selections;
its release bytes are retained for existing characters and checkpoints.

SRD 5.1 is the default selection, independently of additional operator-installed
packs. The first character tab and the spell browser accept multiple compatible
roots. Dependencies are resolved automatically; one core provider is required.
An independent core can define the same six standard scores, whose identities
remain global; additional ability scores are still rejected. The character's
exact lock governs choices, spells, names and shared-sheet catalogues. Changing
packs on an existing character previews and commits through the rules migration
API. Copies require current access to their releases; existing characters retain
access for progression and restoration after a share is removed.

Any group member may share a release they own. A share records its exact
transitive closure and does not advance when another version is published. The
contributor or an owner/DM may remove it. A contributor leaving does not remove
it. A private dependency owned by somebody else must already be available to the
group before it can be included. Group members can export accessible releases
and import independent copies; sharing is not a restriction on redistribution.

Browser import accepts **one portable JSON document or one ZIP containing a
pack directory**. Export remains portable JSON. A ZIP may put the manifest and
referenced files at its root or inside one enclosing folder; package the pack
folder itself, not a repository containing several packs. `pack-manifest.json`
takes precedence over `manifest.json` in the same root. Archives have a 64 MiB
upload/expanded-content limit and at most 4,096 entries. Absolute/traversal paths,
symlinks, duplicate paths, missing referenced files and multiple pack roots are
rejected. Files are read directly from the archive, never extracted.

Import creates an owned draft with a new ID and version 1.0.0, rewrites
self-references, and preserves translations and source attribution. External
dependencies remain declarations. Import dependencies separately, then use
mapping to point at their new identities and adjust version ranges as needed.
Missing dependencies block publication, not draft saving. Failed imports do not
create partial drafts. No remote fetching or executable content is added.

For an existing directory pack, the CLI can still produce the JSON upload:

```sh
go run ./cmd/pack -in /path/to/pack -out /tmp/my-pack.json
```

Supply dependency paths after the first input when required; the output must be
new. A standalone replacement core needs only its own directory.

The authoring API is `/v1/packs`: list/create, `schema`, `import`, `resolve`,
`catalog[/collection]`, and per-ID `draft`, `validate`, `publish`, `archive`,
`export` operations. Group sharing uses `/v1/groups/:id/packs`. Catalogue selection
uses `packs=id@version,id@version`; resolution returns a complete rules lock.
Every request checks current access before compiling or returning cached content.
Drafts and all private authoring/catalogue responses use `Cache-Control: no-store`.
Validation has localized reason codes, document locations, and expandable compiler
details for diagnosing unsupported mechanics.

Three counts are limited (see [Limits](backend.md#limits)): an owner holds at
most 30 unarchived packs, a pack at most 100 published versions, and a group at
most 20 shared packs. Archived packs are not counted, because a pack cannot be
deleted and counting them would leave an owner at the limit with no way back
under it. Sharing another version of a pack the group already has replaces it
and is never refused.

## Class ability priority

A class may carry `abilityPriority`: all six abilities, most important first --
where its best score goes, then its second best, down to the one it can spare.

```json
{"slug": "wizard", "abilityPriority": ["int", "wis", "con", "dex", "cha", "str"]}
```

It is advice, not a rule: nothing is derived from it. The builder's **Use
recommended** deals the ability scores out by it (docs/web.md), and the server
passes it on only when it names every ability exactly once -- a list of four,
or one with a repeat, is not an order six scores can be dealt by and is
ignored. The base pack's thirteen classes each have one. They follow the
usual advice for each class -- the casting or attacking ability first,
Constitution high, the dump stat last -- and are a starting point to edit, not
a ruling.

## Pack and book provenance

`manifest.title` optionally provides a release display name. `manifest.sources`
maps pack-local source IDs to default display names; IDs use lowercase letters,
digits, hyphens and dots (for example `srd-5.1`). Localized names live in
`locales.<locale>.sources.<id>.name`, with normal locale fallback.

The optional `provenance` field maps collection names to local entity IDs to
arrays of source IDs. A directory references it as `files.provenance`:

```json
{
  "manifest": {"title": "My 2014 books", "sources": {"phb": "Player’s Handbook", "xge": "Xanathar’s Guide to Everything"}},
  "provenance": {"spells": {"example-spell": ["phb", "xge"]}}
}
```

This is an excerpt, not a complete pack. The SRD pack itself keeps its sources
to bare ids -- `{"srd-5.1": "SRD 5.1", "phb": "PHB", "xge": "XGE", "tce": "TCE"}`
-- and ships no `sources` bundle: a tag, not a title, is all a row needs to
say where it came from (`web/src/ui/sourceAbbreviation.ts` renders the
abbreviation). Source mappings must name existing entities and declared
sources. Membership ordering is not semantic. Provenance
is part of the immutable release digest, survives JSON/directory/ZIP round trips,
and requires a version bump when changed. Importing a copy retains book
memberships but derives its owning identity from the new pack. Legacy spells'
`source` field supplies a fallback; other untagged entities expose only their pack.
No book is inferred merely from a matching display name.

Catalogue entries return `provenance` with `packId`, `packTitle`, `version`,
`digest`, and `sources` (`id`, `name`). Source IDs are scoped as `pack-id:book-id`.
Player-facing races, subraces, classes, subclasses, features, traits, backgrounds,
feats, equipment, magic items and spells receive provenance. Progression rows
and structural constants do not get visible tags.

`GET /v1/packs/spell-filters` returns accessible published packs and versions,
book options, schools, classes and unavailable releases. `GET /v1/packs/spells`
searches across those releases, independently resolving each dependency closure;
incompatible cores are never combined. **The exception is the default rules**:
a release that is in the server's default lock is loaded under that whole lock
rather than alone, and its rows carry that lock as `catalogPacks`. This is what
lets an **overlay** reach the compendium -- a pack of prose over another pack's
entities that defines none of its own. Alone, the base pack has no description
for a spell the overlay describes, and the overlay's own row lists nothing,
because no spell is *its* spell; so a spell beyond the SRD opened with its
mechanics and an empty box. Default packs are browsed the way a new character
meets them, together. The latest accessible semantic version
of each non-archived pack is the default. `versions=id@version,...` overrides it.
`pack=id,...` and `source=pack:book,...` narrow results (OR within a field, AND
between fields and other spell filters). Existing spell query and pagination
parameters apply. Scoped catalogue searches accept these filters too.

Aggregate spell rows include `catalogPacks`, the exact dependency versions for
their detail link. Entries are emitted once from their owning release, so shared
dependencies do not duplicate results. Access is checked on every request and
responses use `no-store`. An unavailable release is reported rather than
silently replaced; explicitly requested inaccessible versions return an error.

Pack confirmations reuse decoded immutable documents by a hash of their actual
bytes and compiled catalogues by exact lock and locale. Installed release JSON
is encoded once at startup. Available releases and permissions are still checked
on every resolve; cached content cannot restore revoked access. The same compiled
context is reused when the player confirms the name and creates the character.
