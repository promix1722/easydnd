# Spells and homebrew packs

Part of the [web client documentation](../web.md).

## Pack transition

Build edits, deletes and level-up requests carry the server's `revision`
as `expectedRevision`, alongside their positional `expectedSeq`. This closes
the same-length-edit concurrency gap. Legacy test/API fixtures without a
revision fall back to their sequence.

The ability editor still uses the base-game presentation. Resources do not: the sheet and the game page read
the generic `resources.pools`/`parameters` and take their names from the pack,
so the client holds no table of resource labels and a homebrew pool shows up
without a client change. `packActions` is still unread. See
[packs.md](../packs.md) for the read/write contract.

## Spells are the compendium, browsable

A `SECTIONS` entry, at `/spells` and `/spells/:slug`. The section is a
reader over the catalogue rather than anybody's data: the rows belong to no
account, there is nothing to create and no row action, and a guest sees exactly
what an account does. It is still behind `Private` like every section, because
the catalog endpoints require a session -- the Go router marks the two lines to
move if a truly public browser is ever wanted.

It is also a **search-and-filter surface**, and the server
does the work: `searchSpells` in `lib/api/catalog.ts` sends the filters as
query parameters and gets back a `{spells, total}` page, filtered, sorted and
paged by `search.go` -- so the screen only says what it wants and appends
pages behind a **Load more** button, shown while fewer rows are loaded than
`total` says exist. The search box debounces 300ms before writing the URL,
because the URL is the request and firing one per letter would race four
requests to answer the word. The filter state lives in `useSearchParams`, not
component state, so a filtered list survives a reload and can be handed to
somebody as a URL, the same way the character list carries `?folder=`.

The detail page asks the same endpoint for the whole entry with `?slugs=`,
which is where the prose and the remaining rule values live.

**Item artwork uses shared pack labels.** Equipment and magic items reference
one pack-local `icons.items` library. Resolved API entries carry an optional
`icon` data URL. `ui/ItemIcon` renders the 128×128 assets at 66×66, one and
a half times the spell icon, with pixelated scaling beside the visible item name.
An equipment slot reserves that height whether or not anything is worn, and
cuts the worn item's facts and description to it, so equipping never moves the
cards below. Inventory rows, equipped slots
and starting-equipment choices use the same component; equipment
columns stack on narrow screens to make room. Missing images leave the name
and controls usable. Item artwork is decorative to screen readers. See
`docs/packs.md` for the authored mapping and conversion workflow.

**Spell artwork belongs to its pack release.** One WebP per spell lives in
`data/pack/srd-5.1/spell-icons/`, the non-SRD spells included, and the loader
picks them up by name. Existing catalog summaries and details expose an
optional `icon` WebP data URL. The same 44px slate rounded square is used in
spell lists, spell details and character spell choices. A small inset keeps
transparent artwork inside the tile, and the dark background makes glowing
runes readable on both light and dark pages. Packs without artwork show no
image. Changing packs or releases changes the artwork with the catalog.

**Adjusting a filter must not take the filters down.** `useResource` blanks
itself when its key changes, which is correct for a key naming a different
*thing* -- a different character, a different group -- and wrong for the one
key in this app that carries adjustable filter state. Were the search results
and the schools and classes filling the Selects a single resource keyed on the
search, every ticked checkbox would throw away the controls that ticked it, the
page-level state would take the whole screen down to a spinner and rebuild it
-- search box, filters, count and table -- to change which rows were in the
table, and typing would lose the caret on every pause.

So it is two resources. `spells:options` is keyed on nothing, loads once --
one small request for the packs, books, schools and classes there are to filter
by, `…/catalog/spell-filters`, never the spells themselves -- and is the only
one that gates the page. The search is keyed on its filters but gates the
results region alone through `PageBody`. A third piece finishes it: the screen
holds the last page that arrived, so a search in flight *dims* the rows already
on screen instead of emptying the table. They are the previous answer, not a
wrong one.

Artwork travels through the existing catalog response rather than Vite assets
or separate image routes. It is not included in the application bundle or PWA
precache. This supports packs imported after deployment without rebuilding the
website. The tradeoff is larger catalog JSON and no independent image cache;
the SRD artwork is about 8 MB before base64 encoding. Private and shared pack
artwork follows the existing catalog authorization and cache policy.

That tradeoff is why **the spell list is never downloaded**. Inline artwork
makes every spell about 30 KB, so "all the spells" is 11 MB, and the server
refuses the bare collection outright (see
[backend.md](../backend.md#spells-are-never-served-whole)). A screen gets spells
three ways and no others: the ones it names (`getEntries('spells', slugs)`),
a page of a search (`searchSpells`, `searchSpellOffer`), or resolved inside a
sheet. `getCollection('spells')` is a request the API answers with a 400.

**`features/spells/spellText.ts` is the piece the structured rule values were
waiting for**: the compendium stores casting time, range and duration as
`{kind, amount, unit}` precisely so a client can render "90 feet" per locale,
and this module is that rendering. It sits in `features/spells/` and *not* in
`src/domain/`, deliberately: `domain/` may not import `@/lib`, and these
functions want the typed `Translate` so every key they name is checked against
`en.json` at compile time -- the same reasoning that moved
`features/character/labels.ts` out of `domain/`. Other features import it from
here, which is precedented (`features/games` imports `../groups/roles`).

## Homebrew

The Homebrew pages list built-in, owned and group-shared packs. The editor
uses a recursive typed form driven by `/v1/packs/schema`, including reference
pickers, ordered rows, expressions and locale maps. Closed sections are expanded
lazily so large imported catalogues do not render every field at once. Saving
keeps an incomplete draft; validating and publishing use the server compiler.
Import accepts JSON or a ZIP containing one pack directory; export uses JSON. Dependency replacement is explicit and
preserves external version constraints. Every caption ships in English and Russian.

The builder's first Rules tab shows the rules edition followed by compatible
pack selection, using the same collapsible cards and Confirm/Clear actions as
race selection. Confirming packs preserves the previously confirmed edition.
Confirmed pack selections are final in the builder. Existing characters show
their pinned releases read-only; there are no migration controls.

The standalone spell browser searches all accessible published packs. Pack and
Book/source multiselects live beside the other spell filters, apply immediately,
and preserve their selections in the URL. The latest accessible release is the
default; packs with older versions offer a version selector in the filter area.
Changing one filter preserves the others and resets pagination. Book choices
follow selected packs/releases; unavailable book selections are removed. Exact
release contexts travel into spell detail links, including existing `packs=` URLs.

Character spell and cantrip selectors reuse those filters over their pinned
catalogue without changing rules or eligibility. Filtering never discards picks.
Pack/book tags appear once on catalogue and choice rows and in details. Source
tags use the theme’s light blue variant; full book names and release versions are
available on hover. Selected choices use a light background with readable text.
Book multiselect labels use compact codes and pack names (for example
`PH / D&D 2014`), and a selected value replaces the empty placeholder without
forcing a second input line.
Character-sheet summaries remain compact. All source names use the catalogue's
locale fallback. Packs without book metadata still show their owning pack.

Catalogue functions accept an explicit request scope. Builder descendants receive
the character scope through React context; spell pages receive a URL selection.
Private catalogue reads bypass the public collection cache so access is checked
on each request. Shared and owned character sheets load their pinned catalogue.
A group has a **Private packs** tab (`features/packs/GroupPacks`), drawn for
whoever has a pack they may grant: it grants a private pack to the table and
takes the grant back.

The prompts response also supplies the selected core's build policy: score
bounds, standard array, point-buy prices/budget and maximum level. Builder forms
consume that policy; legacy responses retain the SRD defaults. The sheet's level-up
button uses the same maximum rather than a fixed level 20.

Character creation treats confirmed rules and rule packs as final: the Rules tab
shows the pinned pack selection read-only, with no migration controls. Final
choices use neutral borders; red highlights indicate outstanding choices only.
Choice rows place their source badges on the right beside the name.
Spell *lists* are the exception and carry no source badges -- not on a spell
choice row, not in the compendium's list. A list is dozens of rows and the pack
and book were the same two badges on every one of them; the compendium's Source
*filter* stays, its options reading pack first and then the book inside it
("SRD 5.1 extended v2.0.0 / PHB"), the order the two filters stand in. The badges are in one place for a spell: inside its description
box (`features/spells/SpellDetails`), as the last of its facts, which is where
"which book is this from" is asked about one spell. On a spell's own page that
box also opens with its level and school (in the builder's preview the row
above already says both).
Homebrew has no entry in the navigation at either width; its direct routes
(`/homebrew`, `/homebrew/:id`) remain.
The custom background is listed last among the backgrounds -- it is the way
out of the list rather than one more entry in it -- and the server is what
orders it so (`customLast` in the catalogue handler; collections are otherwise
in slug order, so a pack file's own order decides nothing). Builder options
for equipment carry no artwork: a list of thirty weapons was thirty 66px tiles.
The builder's **Finish** -- both of them, the header's and the last tab's
Next, which share one `finish` handler so that neither leaves a character with
nothing on -- asks the server to dress
the character
(`POST /v1/characters/{id}/auto-equip`) before it opens the sheet: a build
equips nothing on its way, so that is the one moment a new character gets
anything on, and a failure there costs an empty paperdoll rather than the way
out of the builder. On the
sheet's Actions tab every row is a box of its own (`BlockList`'s `outlined`),
since inside a folding group an unoutlined row is a line of text adrift.
A pick keeps the picked option in view and does not jump to Confirm, which in
a long list would throw the page to its foot on every click. The spell filters share
one fixed width, so the row does not re-wrap when a value is chosen or the
language changes.

### Portraits

Character portraits have their own choice on the Personal tab. The shared avatar
editor also appears in Account's Profile section. JPEG, PNG and WebP uploads up
to 5 MiB open a rounded-square crop preview with dragging, position sliders and zoom.
Confirming produces a 256 × 256 WebP; cancelling keeps the saved image. New
character portraits remain drafts until creation; existing portraits save
independently of the name and retain drafts on failure.

The shared avatar displays character and account portraits as 48 px rounded squares with
a 1 px black border and the spell tiles’ slate background (`#252c3b`) and 8 px corners, including placeholders. Character headers, lists, group
rosters, game trackers and pickers use it. Names remain visible without redundant
Name column headings or tracker labels. Captions and errors are localized.


Default portraits are generated WebP emblems bundled in `web/public/avatars/`,
using the spell artwork's luminous illustrated style: soft colored shading,
bright highlights and restrained edge glow. Readability comes from one large
class symbol with broad structural shapes, generous gaps and little ornament,
checked at 48 px; the shading and highlights remain part of the artwork.
A character's starting
class selects the emblem, including multiclass characters; pack-qualified class
slugs use their final class name. Missing or unsupported character classes use the person placeholder. NPC copies
retain the starting class for their emblem; classless NPCs select a stable random
avatar from their entry ID. Accounts and classless NPCs share 24 generated icons
in `web/public/avatars/random/`, including animal heads and fantasy objects.
Their IDs determine a stable selection; the pool and its order remain fixed
to preserve assignments. Entries from older API versions also get this fallback.
Tracker players can use classes from the game's existing roster.

Defaults are display-only and never written into `image`. Uploads override them;
removing an upload reveals the default without offering removal of the default
itself. The app makes no generation requests at runtime. Assets are 128 × 128,
transparent WebPs. Generation prompts are in `docs/avatar-prompts.json` and
`docs/random-avatar-prompts.json`. Placeholders use a centered, integer-sized
SVG glyph inside the same tile.

The public `/avatar-gallery` page shows all 12 class emblems and 24 random icons
in separate responsive grids with localized names and larger rounded-square previews.
