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
default catalogue, and live in memory. The
existing browser still uses its original ability editor and resource displays;
the new metadata/read contracts are available for that later UI work.

## Load and export

```sh
# Validate the complete installation and print its exact lock.
go run ./cmd/pack -in data/srd_5.1,data/packs/examples/tactician.json

# Export the first input; supply its dependencies too. Destinations must be new.
go run ./cmd/pack -in data/packs/examples/tactician.json,data/srd_5.1 -out /tmp/tactician.json
go run ./cmd/pack -in data/packs/examples/tactician.json,data/srd_5.1 -out /tmp/tactician-pack -directory
```

Configure the server through the existing YAML file:

```yaml
data:
  srd_dir: data/srd_5.1
  pack_files:
    - data/packs/examples/tactician.json
  default_packs:
    srd-2014: "1.0.0"
    example: "2.0.0"
  pack_archive: .pack-archive
```

`pack_files` installs additional files or directories. `default_packs` selects
roots; dependency ranges are resolved from installed releases only. When omitted,
the explicitly configured inputs become exact default roots. An archive retains
portable releases by digest and makes older locks loadable after restart. Keep
it outside a deployment's release directory. Changing content under an archived
ID/version is an error. Bump the version instead. The archive never supplies
new default roots by itself, and startup makes no network requests.

The archive preserves packs, **not characters**. Character logs, checkpoints,
folders, shares and games still live in memory. Durable character storage is a
separate dependency before replay across application restarts can be promised.

## File contract

A portable file has `manifest`, `entities`, `mechanics` and `locales`. The checked
example [tactician.json](../data/packs/examples/tactician.json) introduces a
subclass, race, spell, tool, feature, numeric modifier, scaling dice
pool and an action that spends it. It is illustrative custom content and is not
installed by default.

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

A directory contains `manifest.json` (the generated SRD uses
`pack-manifest.json` to coexist with its old catalogue index) and a `files` map:

```json
{
  "entities/subclasses": "entities/subclasses.json",
  "mechanics": "mechanics.json",
  "locales/en/subclasses": "i18n/en/subclasses.json"
}
```

File paths are relative and confined to the root, including symlink resolution.
Input is bounded to 64 MiB and nesting depth 64. Duplicate keys, unknown typed
fields, unsupported capabilities, unresolved references, missing dependencies,
dependency cycles and conflicting definitions are errors. JSON Schema validates
the envelope; strict typed decoding and semantic checks validate the contents.
The wire types in `internal/adapter/catalog/file` are shared with the generator.

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
| `casting` | Explicit shared/independent profiles; contribution numerator/denominator, start level and floor/ceil rounding per class |
| `actions` | Owner, condition, resource costs and manual outcomes |
| `overrides` | Version-guarded replacement of a dependency's complete rule/resource definition |

Expressions are integer ASTs: `constant`, `read`, `add`, `subtract`, `multiply`,
`min`, `max`, `floor-div`, `ceil-div`, `eq`, `gte`, `lte`, `and`, `or`, `not`.
They have bounded depth/arity, exact arithmetic and JSON-safe integer limits.
Inputs include `level`, `proficiency`, `class:<id>`, `ability:<id>` and
`modifier:<id>`. Core formulas additionally receive `score` or `hitDie` in their
respective environments. Dice are descriptions; evaluation never rolls them.
A new operation needs a new supported engine capability.

Characters retain exactly STR, DEX, CON, INT, WIS and CHA. Packs may modify
these scores, but cannot introduce additional characteristics. The example pack
uses a Wisdom bonus; version 2.0.0 removes its former demonstration Luck score.

Numeric effects support `add`, `max`, `set` on ability scores, AC, initiative,
passive Perception, HP maximum and movement speeds. Grants support features,
traits, feats, spells, proficiencies, languages and equipment. Rules can expose
choices through the existing prompt/answer grammar. Ability dependencies are
ordered and cyclic reads rejected. Mixed combination operations or multiple
`set` effects on one target are rejected. DM change events remain final overrides.
Rules with no automated effect must explicitly declare `manual: true`.

Overrides require a canonical `target`, compatible `version`, and exactly one
replacement `rule` or `resource`. They retain the target identity and existing
localized name. Two replacements of the same target fail; file order never
selects a winner. Arbitrary JSON patches and whole-entity replacement are not
supported operations.

## Resources and temporal events

A resource declares `id`, `owner`, `minimumLevel`, `kind` (`pool` or `parameter`),
`input`, and `rows` or `capacity`. Threshold rows use `from`, `capacity`, optional
`dice`, `slotLevel`, exact `rational`, `boolean` and `text`; the greatest matching threshold applies. A capacity
expression can accompany rows to compute uses independently of die/slot scaling.
`sharedKey` plus `combine: max|sum` explicitly merges capacities; recovery policies
must agree. Unrelated resources with identical display names stay separate.

The base data includes Pact Magic, an oath-gated Paladin Channel Divinity grant,
Cleric sharing, ability-based Bardic Inspiration and conditional recovery.
Ordinary spell slots and Hit Dice retain family instance IDs. `resources.pools`
is authoritative for usage; `resources.parameters` separates damage/scaling
values from consumables. The older slot/class arrays remain compatibility views.

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

Edit base mechanics in `data/rules/2014/`, translations in `data/translations/`,
and bump `data/rules/2014/release.json` for a new published artifact. Run
`make data/srd`; never hand-edit its output. SemVer communicates compatibility:
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
slot progression. `spellBenefits` attach automatic spells or counted spell
choices to an owner at a minimum level, with class/any-list/spellbook eligibility
and a separate purpose such as Arcanum or mastery. Racial benefits can declare
their casting ability. `choiceRequirements` gate conditional equipment offers
on any of a list of proficiencies. Referenced owners, classes, spells and
proficiencies must resolve in the pinned catalogue.

The SRD input policy defines these in `data/rules/2014/mechanics.json`; regenerate
with `make data/srd`. Older profiles that omit selection policy keep their
previous behavior. Character rules locks remain authoritative, so installing a
new policy does not silently migrate existing pinned characters.

## Homebrew authoring

`/homebrew` manages personal drafts and immutable published releases. Guests use
exactly the same authoring endpoints; their stored guest identity owns the packs,
so access still depends on retaining that guest session. PostgreSQL persists packs
and group shares when configured. Development without a database uses memory.
Character logs remain memory-only.

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

Browser import/export supports **one portable JSON document**, with no dependency
bundles or directory uploads. Import creates an owned draft with a new ID and
version 1.0.0, rewrites self-references, and preserves translations and source
attribution. External dependencies remain declarations. Import missing packs
separately, then use dependency mapping to point at their new identities; adjust
version ranges in the manifest when necessary. Missing dependencies block
publication, not draft saving. No remote fetching or executable content is added.

The authoring API is `/v1/packs`: list/create, `schema`, `import`, `resolve`,
`catalog[/collection]`, and per-ID `draft`, `validate`, `publish`, `archive`,
`export` operations. Group sharing uses `/v1/groups/:id/packs`. Catalogue selection
uses `packs=id@version,id@version`; resolution returns a complete rules lock.
Every request checks current access before compiling or returning cached content.
Drafts and all private authoring/catalogue responses use `Cache-Control: no-store`.
Validation has localized reason codes, document locations, and expandable compiler
details for diagnosing unsupported mechanics.
