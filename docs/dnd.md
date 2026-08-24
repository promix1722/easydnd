# The D&D model

What easydnd models, and the words it uses. Targets the **2014 rules** and
**SRD 5.1**; see [reference_srd_5.1/](reference_srd_5.1/README.md) for where the
data comes from, [backend.md](backend.md) for the Go architecture around it and
[web.md](web.md) for the browser client.

There are two halves. The **catalogue** is the static compendium everyone shares
— races, classes, spells, equipment. A **character** is one player's, and is
event-sourced: an ordered log of what was chosen, from which everything visible
is derived.

## Terminology

The model uses the SRD's words, not near-synonyms. Several of these were wrong in
the first draft of this document, and the corrections are load-bearing rather
than cosmetic — the wrong word usually hides a wrong shape.

| Not this | This | Why |
| --- | --- | --- |
| attributes | **abilities** / ability scores | "attribute" appears **0×** in SRD 5.1; "ability score" 163× |
| proficiency (as a number) | **proficiency bonus** | Bare "proficiency" means *being proficient* with a skill or tool. Two different things, both needed |
| slots (for all class resources) | **resources**, with spell slots one kind | "spell slot" is spell-only. Ki points, rage uses, sorcery points and superiority dice have no umbrella term in the SRD |
| movement | **speed** | `walking speed` 30×; `movement speed` **0×**. Speeds are typed: walking, flying, climbing, swimming, burrowing |
| vision | **senses** | Darkvision is a sense. Stat blocks read `Senses darkvision 60 ft., passive Perception 11` |
| max HP | **hit point maximum** | `hit point maximum` 56×; `max HP` **0×** |
| spell casting | **spellcasting** | One word, 207× |
| dark vision | **Darkvision** | One word — and it is a *racial trait*, not a *class feature* |
| party (as an account-level thing) | **group** | See below — the two are not the same set of things |

**A group is people; a party is characters.** A **group** is the account-level
thing the app stores: the people at one table, with a rank each — owner, DM,
player. A **party** is the in-fiction band of adventurers, which is characters,
and the SRD uses it that way throughout ("the party", "party treasure"). They
are deliberately not modelled as the same thing and today only one of them is
modelled at all: a group holds no characters. The Characters tab is
short-labelled "Party" on mobile for exactly the SRD sense, which is why the
account-level feature could not also be called one.

Two of those are more than renaming:

**Traits and features are different collections.** A **trait** comes from a race
or subrace (Darkvision, Fey Ancestry); a **feature** comes from a class or
subclass (Sneak Attack, Cunning Action). The SRD keeps them in two files —
38 traits, 407 features — and so does the model. A merged bucket could not answer
"what did my race give me?".

**"Slots" became resources.** `Resources` holds `SpellSlots`, `HitDice` and a
generic keyed pool covering all thirty-two class-specific values the SRD defines.

Two things that read like errors but are not. `spell attack bonus` is used
verbatim in the SRD, so it stays (the rules also say "spell attack modifier";
either is fine). And `race` is correct **because** this project targets 2014 —
only the 2024 rules rename it to *species*.

One caveat: **`subclass` is a word the 2014 rules never use.** Each class names
its own — Roguish Archetype, Divine Domain, Sacred Oath, twelve in all. It is the
right term for the *model*; the term to put on *screen* comes from
`Subclass.Flavor`, and it is translatable prose.

## The catalogue

Twenty-two collections, 1,944 entries, generated from the vendored SRD dump into
`data/srd_5.1/`. Every entry has a slug, a name and a description; references
between entries are always slugs, never pointers, which keeps the data acyclic
and lets one loaded catalogue be shared immutably across requests.

| Group | Collections |
| --- | --- |
| Basics | abilities, skills, alignments, languages, conditions, damage types, magic schools, weapon properties, proficiencies, equipment categories |
| Ancestry | races, subraces, traits |
| Class | classes, class levels, subclasses, features |
| Origin | backgrounds, feats |
| Things | equipment, magic items |
| Magic | spells |

Monsters are deliberately **out of scope** for now; the vendored file stays
reference-only until the battle tracker gets its own pass.

### Choices

The SRD asks the player to choose constantly — two skills from a class list, a
martial weapon *or* a shield, +1 to any two abilities — and those prompts nest.
The model transcribes that grammar as a recursive `Choice`: choose N from an
option set, where an option may itself be a choice, a bundle, a reference, an
ability bonus, a damage expression or plain text.

Every prompt carries a **stable id** (`fighter/starting-equipment/1`). That id is
what a character's stored answer points at, so it must survive a data
regeneration — otherwise reloading a character silently loses its choices.

### Localization

Mechanics are language-neutral and prose is not, so they live in separate files:
`spells.json` has the level, range and damage; `i18n/en/spells.json` has the name
and description. Adding Russian never touches a mechanics file.

Fallback is **per key, not per entry**: a locale that has translated a spell's
name but not its description gets the translated name and the English text. That
partial state is what a growing locale actually looks like, so it is the case
that has to work well. `en` is complete; `ru` is currently a scaffold.

Rule strings like `"1 action"` and `"Up to 1 minute"` are *mechanics*, and are
stored structured rather than as text — otherwise a Russian sheet would read
"90 feet", and "which spells can I cast as a bonus action?" would be a substring
search.

## The character

**Event sourcing.** The log is the source of truth; the sheet is a projection.
That is what makes level-up reversible, makes "why do I have this proficiency?"
answerable, and lets a character survive a catalogue regeneration — the events
record what was *chosen*, not what it evaluated to.

### Log and events

The log is ordered. It is small, so it is stored as one database record holding
a JSON array — which is exactly why every write states the sequence number it
expects the log to end at. Without that check, two clients editing one
character read, modify and write the same blob, and the later write silently
discards the earlier.

It is **not** append-only, and saying that it was hid the reason it is safe.
The invariant is **append, drop a suffix, or replace one entry and revalidate
what follows**. What holds across all three is that *a stored answer's meaning
depends only on the entries before it*: dropping a suffix removes entries that
nothing earlier depends on, and a replacement leaves the prefix untouched, so
every earlier entry still means what it meant. What a replacement can
invalidate is the suffix, which is therefore re-checked entry by entry against
the log rebuilt so far. Editing an entry in the middle *without* that replay is
the thing that stays forbidden, and it is forbidden for the original reason: it
would leave answers standing that the new prefix never offered.

**One entry per selection.** Choosing a race is one entry. Setting the six
ability scores is one entry. Naming the character is one entry. Nothing bundles
several selections together, and that is not tidiness — a selection with no
entry of its own is a selection the player cannot change, because the only
thing they could point at is an entry that also carries five other decisions.
Creation used to seed the name, the generation method and all six scores into
the opening event, which is exactly why identity and abilities were the two
things a build screen could not offer to revisit.

Every event has the **same field structure** — one struct with a `Type`
discriminator, not a sealed interface. Fields a given type does not use are zero.

| Type | Carries |
| --- | --- |
| `init` | The opening state — for a character created here, the name. Always first, exactly once |
| `change` | An arbitrary addressed mutation — the escape hatch for a DM ruling or homebrew, and how the ability scores are answered |
| `race`, `subrace`, `background`, `class`, `subclass` | A catalogue entry plus the answers to its prompts |
| `level` | A level gained in a class |
| `feat` | A feat taken |
| `note` | A player's annotation; changes nothing |

An event names a catalogue entry by typed reference, records `Choices` as
answers keyed by prompt id, and — for `init` and `change` only — carries
`Changes`: a path, an operator (`set`, `increment`, `add`, `remove`) and a
typed value.

It also records a **`source`**: the group of the prompt it answers — `identity`,
`abilities`, `race`, `background`, `class` or `advance`, the same vocabulary
`Prompts` groups its questions by. Grouping the log is then a fact the server
wrote rather than something a reader infers from the type, and the difference
is not cosmetic: a `change` event carrying six numbers and a `change` event
carrying a DM's ruling have the same type and belong to different questions.
The server derives it from the prompt the event was matched against and never
reads it off a request, because a client-supplied source would be a second,
unverified vocabulary for the same fact. Entries the server cannot attribute —
an imported log, a DM's `change` — carry no source at all. They are still in
the log; they simply belong to no question.

**The trade this accepts.** The log records how a character is *constructed*,
not every state it passed through. "Why do I have this proficiency?" stays
answerable, because the entry that granted it is still there and still says
what it answered. "Was I ever a High Elf?" does not, because the entry that
said so was replaced rather than superseded. That is the right way round: the
first question is asked at the table every session, the second is asked by
nobody, and keeping a second history alongside the constructor would be a
second thing to hold consistent with the first.

Most of what a change addresses is a value nothing derives. Two are not:
`skills.<skill>` and `savingThrows.<ability>` state training the rules would
otherwise compute from whatever granted it. They exist for imported sheets,
which assert proficiencies without saying where they came from — see
[Importing a foreign sheet](#importing-a-foreign-sheet).

### The projected sheet

`Project(log, catalogue)` folds one against the other. It is pure: same log and
same catalogue, same result, no clock and no I/O.

| Section | Holds |
| --- | --- |
| `Identity` | Name, alignment, race, background, classes, personality |
| `Base` | Hit points, speeds, senses, size, languages, exhaustion, death saves, inspiration |
| `Abilities` | The six scores. Modifiers are computed, never stored |
| `Skills` | Training per skill — an enum, because Expertise doubles and Jack of All Trades halves |
| `SavingThrows` | Autocalculated |
| `Status` | Armor class, initiative, proficiency bonus, passive Perception, and a spellcasting summary **per class** — a multiclassed cleric/wizard has two |
| `Equipment` | Equipped, backpack, loot, purse. Homebrew items are first-class |
| `Resources` | Spell slots, Hit Dice, and class resources |
| `Spells` | Cantrips, known, prepared, ability |
| `Actions` | See below |
| `Feats`, `Traits`, `Features`, `Conditions` | Slug lists |

**Actions have two provenances**, and the model says which. A *derived* action is
recomputed on every projection — an equipped longsword produces its attack, a
prepared spell its casting — so editing one has no effect. A *manual* action is
stored in the log outright, for things no rule derives. Each carries an `Origin`
naming what produced it: the rogue's bonus-action Hide comes from
`feature:cunning-action`.

Action, Bonus Action and Reaction are **siblings**, not a hierarchy. A turn
grants one of each, and spending one does not spend another.

## Importing a foreign sheet

A sheet exported from another tool is a **state**, not a **history**. It says
what the character is; it does not say what was chosen to get there. That is
the exact inverse of the log, and the gap is not closable: "proficient in
Stealth" could have come from the class, the background or a racial trait, and
the export does not say which.

So an import does not reconstruct choices. The export's final state becomes the
character's *opening* state — an `init` event carrying the numbers, plus typed
`race`, `class`, `level` and `subclass` events naming what the export states
outright so that traits, features and level-scaled values attach. **No prompt
is answered.** An imported character arrives with every choice still open, and
finishing it is the ordinary build loop.

That is the honest representation. Guessing which prompt granted which
proficiency would put an invented history in the one place the model treats as
the truth, and every later projection would repeat it as fact.

Two consequences follow from the projector's ordering rather than from any
decision about imports:

- Ability scores are *input* tier, so racial bonuses are applied after them.
  An import records the export's scores **minus the race's fixed bonuses** — a
  half-elf's Charisma 14 is stored as 12 and projects back to 14. The race's
  *optional* bonuses are neither subtracted nor inferred, so that prompt stays
  open and the sheet can read a point or two light until it is answered.
- Skills and saving throws are *override* tier, applied after the bonuses are
  derived, so `skills.<skill>` and `savingThrows.<ability>` recompute the bonus
  they invalidate. A change that set only the training level would leave a
  sheet reading "Expertise" beside the number for plain proficiency.

Whatever cannot be expressed is named in an **import report** rather than
dropped: a background the SRD does not publish, a purse, spent Hit Dice, a
homebrew action. SRD 5.1 publishes one background and one feat, so a sheet from
a tool with the full rules always leaves something behind, and the report is
what makes that visible instead of silent.

## Status

The models, the on-disk format, the loader and the rules math for creation and
level-up are done and tested. `Project` folds a log into a sheet, and a
companion, `Prompts`, folds it into the questions still outstanding -- the two
together are what make creation and level-up one flow rather than two.

The golden test is the level-3 half-elf rogue in
[reference_hexsheet/](reference_hexsheet/): ability scores, proficiency bonus,
armor class, hit point maximum, passive Perception, six skills with Expertise
in two, and the saving throws all come out matching the real exported sheet.

**Still not derived**, and marked as such where it would go:

- Actions from equipment and prepared spells. `State.Actions` carries only
  what a change event put there, and the battle tracker is where the rest
  belongs.
- Spell selection. `Spells.Ability` is set; cantrips, known and prepared are
  not. Choosing spells needs a per-class list filter and prepared-versus-known
  rules, which is its own feature.
- Unarmored Defense and Jack of All Trades. Both are class features whose
  mechanics the compendium records only as prose.
