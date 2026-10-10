# The D&D model

What easydnd models, and the words it uses. Targets the **2014 rules** and
**SRD 5.1**; see [reference_srd_5.1/](reference_srd_5.1/README.md) for where the
data comes from, [backend.md](backend.md) for the Go architecture around it and
[web.md](web.md) for the browser client.

The **catalogue** is a compiled context of immutable JSON rule packs. A character
pins exact releases and is derived from an editable ordered log. Pack-defined
resources, casting profiles, grants, conditions, stat effects and action costs
run through the same projector as the generated base rules. See
[packs.md](packs.md) for the implemented contract. The legacy formulas and pool
arrays described below remain compatibility paths; installed contexts use the
base pack's explicit policy and `resources.pools`/`resources.parameters`.

Usage events are temporal: spend/rest/action events validate against the build
at their position, and later levels preserve spent uses. They are not how a
sitting is tracked -- the game tracker counts spent uses per game entry and
never writes the log; see [backend.md](backend.md#active-game-entries). Rules upgrades are
explicit migrations with previews and rollback checkpoints. A new pack version
never changes an existing build implicitly.

Ability scores are fixed to STR, DEX, CON, INT, WIS and CHA. Addons may grant
bonuses, features and actions, but cannot add custom characteristics.

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
| campaign / session (as an app-level thing) | **game** | Neither is an SRD word: "campaign" is a DMG term and a "session" is a calendar entry. A group is who is at the table; a game is what they are playing |

**A group is people; a party is characters.** A **group** is the account-level
thing the app stores: the people at one table, with a rank each — owner, DM,
player. A **party** is the in-fiction band of adventurers, which is characters,
and the SRD uses it that way throughout ("the party", "party treasure"). They
are deliberately not modelled as the same thing. That is why the account-level
feature is a *group* and not a *party*: "party" is a fiction word naming
nothing this app stores, and a set of people with ranks is not a party in any
sense the SRD uses.

The client used to lean on this the other way round, short-labelling the
Characters section "Party" on a phone. It no longer does — the section is
"Characters" everywhere — and the argument above is unchanged, because it never
depended on that label. What changed is that "party" is now a word in the
fiction and in this document only: no screen, route, field or type in either
half of the app is named one, so there is nothing left for "group" to collide
with.

A group is no longer only people. Its members may **share** characters with it,
and what that grants is a read: every member can open a shared character's
sheet, and only its owner can ever change it. The shared characters are not a
party -- they are everything the table has to hand.

A **folder** is the third word and the smallest of them: one account's private
shelf for its own characters. It is not people and not in-fiction -- nothing is
shared through it and no rule reads it. A **game** is the fourth: one sitting at
a group's table, with a roster drawn from what that group shares -- which is
where a party is finally modelled. Group, game, party, folder: people, what they
play, characters in the fiction, characters on a shelf.

A game is played *at* a group but is not part of one: it is its own section of
the app, listed with every other game you are in. The group is a fact about a
game in the way a folder is a fact about a character. It is called a game and
never a *session*, because that word is spent several times over on the thing
that proves a request belongs to an account.

A group, its members, the characters shared with it and the games run from it
all live in PostgreSQL and survive a restart. The rows that name a character
carry no foreign key to it, on purpose; see
[backend.md](backend.md#ownership-and-membership).

An active game has an ordered list of player characters and NPCs. Player
base stats follow their original sheets; current HP, temporary HP, rolled
initiative and text tags belong to the game alone. Tags are temporary notes,
including conditions, without applying mechanical effects to either sheet.
Players can edit their own unlocked entries. The master (a group owner or DM)
can edit all entries, lock player edits, and move or sort the list. Initiative
is a reported roll total, initially unset; sorting is explicit and stable for
ties, with unset entries last.

NPCs are independent private copies of the master's characters, or stubs
named NPC starting at 10/10 HP with editable stats. Copying one grants no access to the original sheet.
Copies retain their starting class for their portrait. Players see NPC names,
portraits and their place in the order; the master sees stats
and tags. Removing an NPC affects only the game, and copying the same source
again creates another independent creature.

The last row is different in kind from the others, and worth flagging rather
than letting the table's authority stretch over it. The rest correct a wrong
word against SRD evidence — counts of what the document itself says. "Game"
names a **product** concept the SRD has no word for at all, so there is no
count to appeal to; it is a choice, made here so that it is made once.

Two of those are more than renaming:

**Traits and features are different collections.** A **trait** comes from a race
or subrace (Darkvision, Fey Ancestry); a **feature** comes from a class or
subclass (Sneak Attack, Cunning Action). The SRD keeps them in two files —
38 traits, 407 features — and so does the model. A merged bucket could not answer
"what did my race give me?".

Dragonborn choose their draconic ancestry once. Breath Weapon and Damage
Resistance are automatic racial traits; the selected ancestry determines their
damage type without another confirmation. The legacy catalogue stores the
ancestry's breath attack in a single-option `breathWeapon` choice-shaped payload
for compatibility. This is ability data, not a player choice, so the prompt
builder never offers it. Spell and subtrait choices remain real prompts.

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

Twenty-two collections in `data/pack/srd-5.1/`, hand-maintained. Every entry
has a slug and a name; most have a description, and the ones that do not are
missing it on purpose (see [below](#sources)). References between entries are
always slugs, never pointers, which keeps the data acyclic and lets one loaded
catalogue be shared immutably across requests.

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

### Sources

Every player-facing entry carries a **source** through the pack's
`provenance.json` -- `srd-5.1`, or `phb`, `xge` and `tce` for what the SRD
leaves out. The pack is *SRD 5.1 extended*: the mechanics and the name of
every 2014 spell, subclass, feat, background and the artificer ship here,
because dice, levels, components and prerequisites are facts; their text does
not, because everything outside the SRD is unlicensed WotC copyright. So a
Xanathar's spell in this catalogue has its level, school, range and class
lists above an empty description, and that is the honest state of what may be
shipped. The descriptions come from a separate, private pack that depends on
this one and fills only what is blank ([packs.md](packs.md#prose-overlays));
nothing is duplicated, and the public site never has it. The source is what
lets both kinds of row sit in one collection, and what the sheet's attribution
badge groups by.

### Choices

The SRD asks the player to choose constantly — two skills from a class list, a
martial weapon *or* a shield, +1 to any two abilities — and those prompts nest.
The model transcribes that grammar as a recursive `Choice`: choose N from an
option set, where an option may itself be a choice, a bundle, a reference, an
ability bonus, a damage expression or plain text.

**Transcribing is not the same as asking**, and one case makes the difference
visible. The rogue's first Expertise is printed as "two of your skill
proficiencies, or one of your skill proficiencies and your proficiency with
thieves' tools", and the compendium says exactly that: choose 1 of two
branches. It is one question — choose 2 from your skills plus thieves' tools —
and the same feature at sixth level, and both of the bard's, are already flat
in the data. `oneList` reconciles it, in the domain rather than in the
generator, so the compendium goes on saying what the book says. Both sides go
through it: `Prompts` asks with it and `Project` reads with it, because
projection resolves an answer by walking the catalogue's own shape and a
flattened question with a nested reader would silently lose the answer. The
guard is narrow — every branch over the same list of references, every branch
worth the same number of picks — so the monk's "an artisan's tool or a
musical instrument", two branches over different lists, is left alone.

**`Repeatable` is on the choice, and only one choice has it.** A level's
Ability Score Improvement is "+2 to one ability, or +1 to two", so the same
ability picked twice is a legal answer and the points are summed. A half-elf's
is "+1 to two *different* scores" — the same choice kind, over the same
ability-bonus options — so a rule read off the kind let the half-elf spend both
on one score. The prompt says it instead. Nothing in the compendium sets it;
the domain does, on the improvement it synthesises.

Every prompt carries a **stable id** (`fighter/starting-equipment/body`). That id is
what a character's stored answer points at, so it must survive a data
regeneration — otherwise reloading a character silently loses its choices.

Every *option* carries a stable **slug** for the same reason, and it is derived
from what the option is rather than from where it sits: a bundle of a shortbow
and twenty arrows is `shortbow+arrow`, a martial weapon with a shield is
`martial-weapons+shield`, a draconic ancestry's breath is `acid`. So an answer
reads back as what was taken — in a log row, in a hand-written fixture, in a
support question — and a person can write one and be right. Options used to
fall back to a positional `#0` where they carried no slug, which was stable
only until somebody reordered a class's equipment list, and legible never.

What position did guarantee is that two options in one set cannot collide, and
a derived slug cannot promise that on its own. That guarantee is a check over
the whole compendium instead (`TestOptionKeysAreTotalAndUniquePerPrompt`),
which fails the build rather than letting an ambiguous answer resolve to
whichever option came first. One case has no derivable name at all — the
monk's "one artisan's tool or one musical instrument" is two inline lists of
the same kind, and the words telling them apart are SRD prose the compendium
does not carry — so a branch that lists its options inline falls back to its
own prompt id, which is unique by construction.

Not every question the SRD *prints* as a table is posed as one. A background's
personality traits, ideals, bonds and flaws are d8 tables in the book and are
carried in the catalogue as `Choice`s of text options, but a character is asked
for them **in their own words**: the state behind them has always been free text,
and a menu of eight makes a character's own line the compendium's. They are
posed as prompts with an empty option set and answered by the change that
settles them — `identity.personalityTraits` and its three siblings — which is
the same shape an alignment and the six ability scores travel in. The tables
remain in the compendium as suggestions to read.

**A pick never answers one of the character's own questions.** Everything
under `character/` — which race, which class, the alignment, the four written
lines — is settled by an entry that *is* the answer or by a change at a path,
and the projection reads it from there. A pick filed under one of those ids
(`choices: [{prompt: "character/alignment", picks: [...]}]`) names a real
option of a real open question, so it used to pass validation, and it settled
nothing: the question stayed open with an answer under it for good. It is
refused now (`field.answer.notAPick`, in `validateAnswer`), which is the only
honest thing to do with an answer nothing will ever read.

### Localization

Mechanics are language-neutral and prose is not, so they live in separate files:
`spells.json` has the level, range and damage; `i18n/en/spells.json` has the name
and description. Adding Russian never touches a mechanics file.

Fallback is **per key, not per entry**: a locale that has translated a spell's
name but not its description gets the translated name and the English text. That
partial state is what a growing locale actually looks like, so it is the case
that has to work well. `en` is complete; every other locale is as far as
somebody has got.

Translations live in the pack, beside what they translate:
`data/pack/srd-5.1/i18n/ru/spells.json`, hand-edited and checked in. Adding a
language is adding a directory -- the loader reads whatever locales are present
rather than a list in code -- and a slug that is not in the English bundle
fails the load with the collection and the slug named, because a mistyped key
is otherwise a word nobody ever sees.

A locale directory holds **only what has been translated**. It is not a merged
copy of English: the loader merges at read time, and writing the merge out
would put a megabyte of untouched English into every language's diff. See
[data/locale-terms-locked/README.md](../data/locale-terms-locked/README.md).

Rule strings like `"1 action"` and `"Up to 1 minute"` are *mechanics*, and are
stored structured rather than as text — otherwise a Russian sheet would read
"90 feet", and "which spells can I cast as a bonus action?" would be a substring
search.

## The character

**Event sourcing.** The log is the source of truth; the sheet is a projection.
That is what makes level-up reversible, makes "why do I have this proficiency?"
answerable, and lets a character survive a catalogue regeneration — the events
record what was *chosen*, not what it evaluated to.

**A folder is not part of this model.** Characters are filed into folders — see
[backend.md](backend.md#folders) — and a folder records where a record is kept,
not anything about the character. It is neither the group above, nor the game,
nor the party:
no rule reads it, nothing is shared through it, and it never appears in a log or
on a sheet. Look for it in the service, not here.

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

**The rule is enforced, in two places, so that no writer can break it.** It
used to be a convention the build screen kept and nothing checked, and the
development seeds did not keep it: a rogue's class was one entry carrying its
skills, its Expertise and three kit picks. The editor draws one box per entry
on the tab the entry belongs to, so that character's whole first level was one
box under Class, with the boxes its other answers should have had simply
missing.

- **The log itself** (`Log.Validate`, which every repository write and every
  projection runs) refuses an entry whose answers belong to more than one
  question. A branch and the picks made inside it are one question -- the
  improvement's "two scores" and which two -- which is why a nested prompt's id
  sits under its parent's, and an answer nested under the entry's first answer
  travels with it. Anything else is a second question. This holds for an
  import, a migration and a repository handed a whole log, not only for the
  service.
- **The service** (`oneSelection` in `usecase/character/validate.go`, on
  append and on revise) refuses the same thing with a field error a client can
  point at, and one shape more that only it can see: an entry that *selects*
  something -- a race, a class, a subclass, a background, the answer to "which
  one?" -- and also carries answers. What a selection opens is asked next and
  answered in entries of its own; whether an event is a selection depends on
  which prompt is open, which the log alone cannot know.

Tests that transcribe a character as one literal per step keep doing so and
split it on the way in (`oneQuestionEach` in the domain's fixtures).

Every event has the **same field structure** — one struct with a `Type`
discriminator, not a sealed interface. Fields a given type does not use are zero.

| Type | Carries |
| --- | --- |
| `init` | The opening state — for a character created here, the name. Always first, exactly once |
| `change` | An arbitrary addressed mutation — the escape hatch for a DM ruling or homebrew, and how the ability scores are answered |
| `race`, `subrace`, `background`, `class`, `subclass` | A catalogue entry plus the answers to its prompts |
| `level` | What a level granted, answered — an improvement, an Expertise, a feature's pick |
| `feat` | A feat taken |
| `note` | A player's annotation; changes nothing. One carrying a custom entry of kind `note` is a titled text the player wrote, shown on the Custom tab |

An event names a catalogue entry by typed reference, records `Choices` as
answers keyed by prompt id, and — for `init` and `change` only — carries
`Changes`: a path, an operator (`set`, `increment`, `add`, `remove`) and a
typed value.

It also records a **`source`**: the group of the prompt it answers — `identity`,
`abilities`, `race`, `background`, `class`, `advance` or `personality`, the same
vocabulary `Prompts` groups its questions by. Grouping the log is then a fact the server
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
| `Identity` | Name, alignment, race, background, classes, personality, experience |
| `Base` | Hit points, speeds, senses, size, languages, exhaustion, death saves, inspiration |
| `Abilities` | The six scores. Modifiers are computed, never stored |
| `Skills` | Training per skill — an enum, because Expertise doubles and Jack of All Trades halves. **Every** skill is present, the untrained ones at the bare ability modifier |
| `SavingThrows` | Autocalculated |
| `Status` | Armor class, initiative, proficiency bonus, passive Perception, and a spellcasting summary **per class** — a multiclassed cleric/wizard has two |
| `Equipment` | Equipped, backpack, loot, purse. Homebrew items are first-class |
| `Resources` | Spell slots, Hit Dice, and class resources |
| `Spells` | Cantrips, known, prepared, ability |
| `Actions` | See below |
| `Feats`, `Traits`, `Features`, `Conditions` | Slug lists |

**Experience is recorded, not acted on.** `Identity.Experience` is a number the
log can set and nothing reads: a character is third level because three level
events say so, not because the total crossed 900. That keeps one answer to
"what level is this character" -- the log -- where deriving a level from XP as
well would give two, and they would disagree the moment a table awarded a
milestone. A group playing milestones leaves it at zero and loses nothing; a
group counting XP has somewhere to keep the count. Set with a change event, on
`identity.experience`, like every other value no rule computes.

**The desired level *is* the level.** `Identity.DesiredLevel` is the level the
player said they are building towards, set with a change event on
`identity.desiredLevel` (1--20), and for a single-class character `Project`
makes it the class's level (`advanceToDesiredLevel`). Nothing takes a level
one entry at a time, because with one class there is no question about which
class a level goes into, and a question with one answer is not a question. So
levelling up is raising the declaration, creation at level N is declaring N,
and going back down is revising the entry that declared it -- which prices
what those levels bought, exactly like any other revision.

What the declaration opens is what the player is actually asked: `Prompts`
walks every level from 1 to that one and adds what each grants -- the
archetype at its due level, the Ability Score Improvements, a feature's own
picks -- to the same list race, background and abilities come in.

**Multiclassing is not offered**, which is what this rests on. `Identity.Classes`
is still a slice and `applyClasses` still walks it -- a log with two classes
projects correctly, and the 2014 rules for what a second class grants are in
`classGrant` -- but nothing poses the question that would create one. Turning
it back on means posing "which class does this level go into?" again and
stopping `advanceToDesiredLevel` from applying; the two go together.

**The ruleset is recorded on the character.** `Identity.Ruleset`, set once
with a change event on `identity.ruleset` and held by validation to the
compendium's own ruleset -- "2014", today the only one served. It exists so
that a future 2024 compendium meets characters that already say which rules
they meant, and with one compendium it makes the choice final.

**The skills map holds every skill, not only the trained ones.** A character
proficient in six of the eighteen still projects all eighteen; the other twelve
carry `NotProficient` and a bonus that is just the governing ability's
modifier. This is a deliberate cost — twelve entries per sheet that a grant
never touched — paid because the question a skill list is read to answer is
usually about a skill nothing trained, and because the alternative is every
client adding an ability modifier itself. A second implementation of a rule is
a second implementation to disagree, and it would disagree the day Jack of All
Trades starts halving a bonus.

Two consequences worth knowing. **Passive Perception depends on it**: the value
is `10 +` the Perception bonus read straight off the map, so before every skill
was present it read a missing key for an untrained character and silently
dropped their Wisdom modifier — the sheet said exactly 10 whatever the score.
And **presence in the map no longer means trained**, which is a trap the
projector fell into once: whether a skill may take Expertise is a question
about its training level, not about whether the key exists.

**Actions have two provenances**, and the model says which. A *derived* action is
recomputed on every projection, so editing one has no effect. A *manual* action
is stored in the log outright, for things no rule derives. Each carries an
`Origin` naming what produced it: the rogue's bonus-action Hide comes from
`feature:cunning-action`.

**The pack decides what is on the list; the code names no class.** `deriveActions`
(`internal/domain/character/actions.go`) fills `State.Actions` from three places:

- **An equipped weapon** gives its attack, with no tag needed. The modifier is
  Strength, Dexterity for a ranged weapon, the better of the two for a finesse
  one; the proficiency bonus is added when one of the character's proficiencies
  *references* the weapon or a category holding it. References are followed
  rather than slugs compared, so it holds under a pack whose slugs are
  namespaced. That is all it applies: no fighting style, no magic bonus, no
  Martial Arts, no off-hand rule -- none of those is data yet.
- **A tagged entry.** Any feature, trait, feat, item or magic item may carry
  `"action": {"kind": "bonus-action", "uses": "second-wind"}`. If the
  character holds the entry (an item: has it equipped), it is an action, named
  and described by the entry's own prose. See
  [packs.md](packs.md#action-tags).
- **A standalone action** from the pack's `mechanics.actions`. One with no
  `owner` is open to everybody -- the SRD's Dash, Hide, Opportunity Attack --
  and is filed as `basic`; an owned one is the pack action it always was.

Each action also carries a `Category` -- `basic`, `equipment` or `feature` --
which is the reason the character has it and what a player filters by. Spells
are not on this list; they are castings, and they have their own.

Action, Bonus Action and Reaction are **siblings**, not a hierarchy. A turn
grants one of each, and spending one does not spend another.

## Importing a foreign sheet

> The HexSheet JSON importer this section was written for has been removed
> (`internal/adapter/sheet/hexsheet` and `POST /v1/characters/import`). What
> it describes still explains two things that outlived it: the override-tier
> `skills.<skill>` and `savingThrows.<ability>` paths, and
> `ValidateImported`, which the AI Wizard's tools and custom options still
> call. Read "an import" below as history.

A sheet exported from another tool is a **state**, not a **history**. It says
what the character is; it does not say what was chosen to get there. That is
the exact inverse of the log, and the gap is not closable: "proficient in
Stealth" could have come from the class, the background or a racial trait, and
the export does not say which.

So an import does not reconstruct choices. The export's final state becomes the
character's *opening* state — an `init` event carrying the numbers, plus typed
`race`, `class` and `subclass` events and a declared `identity.desiredLevel`,
naming what the export states outright so that traits, features and
level-scaled values attach. A multiclassed export loses its later classes,
reported as unresolved rather than folded into the first. **No prompt
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

- Castings from prepared spells, and anything about a weapon attack beyond the
  ability modifier and proficiency: fighting styles, magic bonuses, Martial
  Arts, two-weapon and two-handed grips.
- Jack of All Trades, a class feature whose mechanics the compendium records
  only as prose.

Unarmored Defense used to be on that list. It is derived now, and not by
`armorClass`: that function knows what armor does, and Unarmored Defense is a
class feature. It is two pack rules owned by the barbarian's and the monk's
feature, each an `add` effect on `status.armorClass` whose value reads what is
worn -- Constitution while no armor is worn, Wisdom while neither armor nor a
shield is. See [packs.md](packs.md#file-contract) for the two inputs,
`equipped:armor` and `equipped:shield`, and why only an effect's value may read
them. The bonus is never negative: a character may always use the plain
`10 + DEX`.

Dwarven Toughness is the same kind of rule, owned by the trait: an `add` on
`base.hitPoints.max` whose value reads `level`. Until it existed a hill dwarf
was built one hit point short per level, which nothing noticed until imported
sheets were compared with what they print. Draconic Resilience is two such effects on one
feature: the same hit point per level, and three armor class while no armor is
worn. Both read the character's level, which is the sorcerer's own only while
the character has one class.

Spell class lists were corrected against the SRD 5.1 text when the data was
derived: the upstream dump got five of them wrong (Faerie Fire was not on the
bard's list there). A class whose list lacks a spell is never offered it, so a
wrong list is a wrong build, not a wrong label -- which is why a list is worth
checking against the book whenever a spell row is edited.

Worn armor counts whether it was equipped as a list entry
(`equipment.equipped` add `leather-armor`) or as a counted stack
(`equipment.equipped.leather-armor` set `1`). The second is how an import
writes inventory, because a sheet prints quantities; both are applied before
armor class is derived. A counted write edits the stack where it already is
-- the sheet draws a list in its order, and "one fewer" must not move the row. A third path, `equipment.custom` set to one slug or
none, records which equipped item sits in the sheet's Custom slot -- see
[below](#items-carry-their-slot); it is the one placement the projection
cannot derive, and it is forgotten on its own once the item is no longer
equipped, whichever path took it off.

## Builder choices under the 2014 rules

The target is [SRD 5.1](https://www.dndbeyond.com/attachments/39j2li89/SRD5.1-CCBY4.0License.pdf),
using the vendored text for Equipment, each class's Spellcasting/Pact Magic,
Fighting Style, Expertise and the SRD subclass features. Rule policy lives in
`data/pack/srd-5.1/mechanics.json`, beside the entities it governs.

Equipment categories are expanded by the catalogue before either validation or
projection. The expanded set retains its category identity so existing branch
answers still resolve. Unknown items and items outside the offered category are
rejected. Two martial weapons may be two copies of one weapon; two proficiencies
or two fighting styles cannot duplicate the same benefit. Class-specific names
for the same fighting style count as one style. Collection choices such as feats
and languages are also validated against explicit catalogue membership.

**A class kit is asked slot by slot**, written by hand on each class row's
`startingEquipmentOptions` rather than transcribed from the book's
"(a) … or (b) …" pairs. The book pairs things the way it sells them; a player fills a sheet
by its slots, and "a martial weapon and a shield, or two martial weapons"
asked as one question was the card nobody could read. Each choice carries a
`slot` -- `body`, `main-hand`, `off-hand`, or the kit-only `backup`, `pack`,
`focus`, `instrument` -- which is its prompt id (`fighter/starting-equipment/off-hand`),
the card's title, and for the three worn slots a rule. **A kit card is one
list and never a sub-choice**: "a shield or any martial weapon" is written
with the category and generated as the shield followed by every martial
weapon, less any item the slot already names on its own (five javelins beside
"any simple melee weapon" is one javelin option, the five). The rule: **nothing
a kit grants is equipped** -- chosen or fixed, whatever slot the card was for,
it lands in the backpack. The slot titles the question; it no longer wears the
answer. It used to: the item chosen for a worn slot was equipped, which read
well for one sword and compounded badly -- a second weapon in the off hand was
a fighting style nobody had chosen, and fixed items still needed an explicit
change to be worn, so a kit was half dressed either way.

A character is dressed **once, at the end**, by `AutoEquip`
(`domain/character/autoequip.go`): one suitable thing from the backpack for
each slot, written as an ordinary change entry. The main hand takes the weapon
with the biggest damage die, the body the armor with the best base AC, every
other slot the first thing made for it; the off hand takes only something made
for the off hand -- a shield -- and nothing beside a two-handed weapon. It does
nothing for a character with anything already on, so it is a start and never a
correction. Proficiency is not consulted: a wizard carrying chain mail gets it
put on, and can take it off.
The fighter's and paladin's weapons are *main hand: any martial weapon* and
*off hand: a shield or any martial weapon*. Some choices deliberately depart
from the SRD to keep each card about one equipment type: the
ranger's "two shortswords or two simple melee weapons" is two independent
picks, which allows a shortsword beside a handaxe the book does not. The
fighter chooses chain mail or leather armor for Body and can choose a longbow
with arrows as its backup weapon, independently of armor. The barbarian's
off-hand handaxe option grants one handaxe.
Ammunition and its quiver may accompany one weapon; carried stacks such as
the fighter's two backup handaxes and the paladin's five javelins remain.
Catalogue tests enforce one equipment type per class-kit option and one
matching item per worn slot.

The generator repairs omissions against the SRD: ranger quivers, the rogue's
quiver-bearing bow bundle, and the acolyte's five incense blocks, vestments,
prayer-book/wheel choice and 15 gp. The rogue bundle retains its historical key
`shortbow+arrow` although its resolved contents now include the quiver. A bundle
can carry an explicit identity for precisely this kind of source correction.
Cleric warhammer and chain-mail choices require appropriate proficiency. **An
equipment pack is granted as its contents**: a dungeoneer's pack is a backpack,
a crowbar, ten torches and the rest in the backpack, never a row called
"Dungeoneer's Pack", because the contents are what a player reaches for and the
pack is only how the book sells them together. The wizard still names the
pack, and a settled answer reads it back as one. The AI Wizard's
`set_inventory` opens a pack a sheet names in the same way, and a line the
sheet has of its own for something in it -- seven torches left of the ten --
stands over the pack's count. A bare counted write
(`equipment.backpack.dungeoneers-pack` set 1) is still not unpacked: a count
is set, and contents can only be added.

### Items carry their slot

The catalogue says what an item *is* -- armor, weapon, a gear category -- and
**where it is worn**: every item and magic item carries a `slot`, one of the
DMG's "Wearing and Wielding Items" set (`head`, `neck`, `back`, `body`, `arms`,
`waist`, `feet`, `ring`, `main-hand`, `off-hand`), or none when it is only
carried. The DMG's bracers and gloves are one slot here, `arms`: the sheet
draws one card for the forearm and the hand on the end of it. `hands` was a
slot of its own until SRD release 1.5.0; the loader still reads it, as `arms`,
because archived releases and the private 2014 pack wrote it. The shape
decides where an item can go: armor is `body`, a shield `off-hand`, a weapon or
focus `main-hand`, a magic ring `ring`, a wand, staff or rod `main-hand`. That
default is applied once, by the catalogue loader, so a pack writes a slot only
where the shape cannot tell -- the SRD rows do so for the clothes, the amulet and
reliquary, the magic shields and every wondrous item whose name says where it
goes (a cloak is `back`, boots are `feet`). A wondrous item without one -- a
bag of holding, an ioun stone -- is carried, not worn.

Whether an item is *used up* the catalogue still does not say; the client's
Consumables group is guessed from the item's category and a short slug list
(`web/src/domain/equipment.ts`). The character has one `equipped` list and
**one** piece of per-slot state, `Equipment.Custom`: the slug in the sheet's
Custom slot, the slot that takes a wearable of any shape. Every other placement is
derived from the list and the catalogue on every read, because the shape
decides it; Custom is stored because nothing else could decide it, and it only
ever names something equipped.

### Spell acquisition and preparation

Spell sources retain their own cantrips, known spells, spellbook, prepared spells,
Arcanum, mastery choices and preparation capacity. Aggregate cantrip/known/prepared
fields remain for compatibility. Identical spells from separate classes retain
separate ownership. Eligibility follows the individual class's level and spell
list; combined multiclass slots do not unlock higher-level spells to learn.

A selected subclass can supply its own casting profile and advancement table.
Eldritch Knight and Arcane Trickster in the D&D 2014 pack learn from the Wizard
list and cast with Intelligence, while their acquisition totals and single-class
slots follow their subclass rows at the parent class's level. Multiclass slots
use the profile's fraction and rounding. Spell-choice IDs and sources belong to
the subclass, preventing an archetype change from carrying its old selections.

Pack expressions can read whether body armor or a shield is equipped. The
starting kit's worn items are seated first, then the equipped changes apply,
before rule conditions -- so a sheet's whole-list write replaces the kit's
choice rather than being topped up by it; backpack and loot changes follow grants.
This allows packs to implement equipment-dependent features such as Unarmored
Defense without interpreting their prose.

| Casting mode | Acquisition | Preparation |
| --- | --- | --- |
| Bard, ranger, sorcerer, warlock | Current table total, using the current class-level spell pool | Known spells are available |
| Wizard | Six entries initially plus two per later wizard level, using the current class-level pool | Spellbook subset, Intelligence modifier + wizard level, minimum one |
| Cleric, druid | Cantrips follow the class table | Class-list subset, Wisdom modifier + class level, minimum one |
| Paladin | Begins at level two | Class-list subset, Charisma modifier + half paladin level rounded down, minimum one |

| Arcane Trickster, Eldritch Knight | Begins at level three, from the **subclass's** table and the wizard's list | Known spells are available |

A class is not the only thing that casts. A rogue does not and an Arcane
Trickster does, so a casting profile may be keyed by a **subclass** instead of
a class. Such a profile names the class whose spell list it draws on and its
spellcasting ability, since the subclass has neither; its cantrips known,
spells known and slots are read from the subclass's own advancement rows, and
its prompts are named after it (`arcane-trickster/spell/known/3`). Everything
that asks how a class casts -- slots, the multiclass caster level, the
spellcasting summary, the spell prompts -- asks one function, `castingFor`,
which answers with the subclass's profile when it has one and the class's
otherwise. A rogue with any other archetype is exactly the non-caster it was.

The school limits are **not enforced**: an Arcane Trickster is offered the
whole wizard list rather than enchantment and illusion plus the free picks at
levels 3, 8, 14 and 20, and an Eldritch Knight likewise for abjuration and
evocation. The limit is prose the profile does not carry. That is a builder
that offers too much, not one that computes wrong; a profile field for the
schools, and spell benefits for the unrestricted picks, is where it would go.

Required acquisitions must be answered before the build is complete. Preparation
is optional and may use less than the maximum, and is the one spell choice the
sheet itself edits: its owner prepares and unprepares from the Spells tab, which
answers the same prompt the builder would. Its choices are revalidated when
levels or ability modifiers change. The wizard permits direct editing within
current totals rather than simulating 2014 retraining restrictions; the policy
and its intentional simplification are described below.

Pack spell benefits express exceptions explicitly: Life and Devotion spells are
always prepared without consuming capacity; Fiend spells expand eligibility;
Land spells depend on the chosen terrain. The Life domain's missing Guardian of
Faith is restored by its benefit policy. Land's extra cantrip, Magical Secrets,
Pact of the Tome/Chain, Mystic Arcanum, Spell Mastery, Signature Spells and Infernal
Legacy are separate grants or choices. The high elf's existing cantrip choice is
projected. Arcanum remains separate from Pact Magic slots. These selections do
not implement spell casting, copying costs or rest tracking.

Existing logs need no event rewrite: newly required unanswered spell choices
appear as open prompts. Existing pinned pack releases retain their own policy;
changing a character's pinned rules still uses the explicit pack migration flow.


### Current-level spell selection and custom choices

The wizard intentionally simplifies 2014 spell acquisition: it calculates the
class's total number of known spells, cantrips or spellbook entries from the
current class level, and allows ordinary spells from any spell level currently
available to that class. It does not simulate when each spell was learned or
require a forget/replacement operation. For example, a level-five sorcerer may
choose six known spells from levels one through three, with at most two level-three
spells: one base acquisition plus one replacement opportunity at class level five.
The highest available spell level alone has a count quota: base class acquisitions
since unlocking that level, plus one replacement per subsequent class level where
the profile allows replacement, capped by the total. At sorcerer level six this
is four level-three spells within seven known spells. Lower spell levels have no
individual quotas. Spellbook acquisition uses the same base count with no
replacement bonus; preparation and separate feature/racial grants do not use this
learning quota. The class total never increases because of replacement opportunities.
This is a product simplification of the SRD's stricter advancement history,
not a claim that those distributions all result from RAW level-by-level play.
Class boundaries, total counts, preparation rules, special feature grants and
racial allowances remain distinct. Spell slots are casting resources, not
quotas of spells to learn at each spell level. Old recorded swaps still replay
for compatibility but are not offered as new wizard actions.

The wizard permits deliberate custom choices when character availability is
disabled. These belong to an explicit custom source, separate from ordinary
class and racial allowances, and may exceed class, level and count limits.
Unknown spells and duplicate entries remain invalid. The UI explains exceptions
alongside the normal grants; it never interprets them as extra casting slots.

The player's explicit additional spell limit is a separate persisted allowance,
not a change to the class progression. Spells beyond the ordinary class allowance
remain custom picks, even when covered by this extra count. Unlimited explicit
custom choices remain possible with character availability disabled.

## Imported builds and final ability totals

An agent import **rebuilds the character**: catalogue race, class, subclass and
background, then the build's own choices -- class skills, expertise, languages,
spells, metamagic -- answered through the same validator a player's answers go
through. It used to do the opposite, recording the sheet's skills, saves, hit
points and armor class as overrides and leaving every choice open on the
grounds that a sheet is not a history. The result was a character the builder
could not edit, pinned to numbers a model had read off a scan. A sheet is not a
history, but it does determine a build, and the build is what the product is
for.

Two kinds of event are still written without a prompt asking for them, and
both are marked `Observed`:

- The structural entries -- race, subrace, class, subclass, background, feat.
  A sheet states these outright, so they are facts rather than answers. They
  open exactly the prompts an ordinary entry opens, and carry the level an
  ordinary entry would: 1 for the class, the level the class chooses one at
  for the subclass.
- `finalAbilities.<ability>`, an explicit source total, **only where no base
  score reaches it**. Projection installs it before dependent calculations and
  avoids adding race, ASI or pack ability bonuses again. Ordinarily it never
  reaches the log: the import holds the printed totals and after every write
  inverts the additive bonuses into base scores, verifying that reprojection
  preserves the totals, and records them as the ability-scores question's own
  answer (`abilities.method = manual`). Non-invertible custom rules retain the
  explicit override; a later ordinary base score assignment clears it.

Everything else the builder asks about -- name, desired level, personality,
alignment -- is written as the builder's own answer, not as an observation.
See [agent.md](agent.md#the-builders-own-entries).

The sheet's *derived* numbers -- hit points, armor class, skill and save
bonuses -- are not written at all. They are held beside the draft as a
reference: the server reads the proficiencies off the bonuses, reports where
the draft computes something else, and writes nothing to make it agree. A
number is pinned over the build only when it is imported on purpose, and even
then it is dropped at review and at save if the build computes the same value
(`pruneAgentOverrides`), so an imported character carries no override that
merely repeats its own arithmetic. See [agent.md](agent.md#rebuilding-the-sheet).

Unknown mechanics are manual notes rather than executable guesses. Complete
custom definitions use the existing immutable pack lock. Both manual source
notes and private definitions remain readable on copied/shared projections.
See [agent.md](agent.md) for import reconciliation and review boundaries.

## Selecting homebrew rules

A character starts with SRD 5.1 and can select compatible accessible add-ons, or
another complete core pack, on the first build tab. One core provider is required;
dependencies and conflicts are checked by the existing pack compiler. Core packs
may define the six standard ability scores, whose identities remain STR, DEX, CON,
INT, WIS and CHA. Packs cannot introduce a seventh score.

Creation pins the complete resolved release lock. Subsequent selection changes
use migration preview and revision-checked application. Published pack updates
never change an existing character implicitly. If group access to a pack ends,
already pinned characters remain playable and can gain levels, while new
characters and copies require current release access. Group shares are exact
versions and advance only when explicitly replaced.

### Portraits

An optional portrait is an identity input, represented by an `identity.image`
string set in the character log alongside the name. Empty means no portrait.
It has no effect on rules or completion and is projected onto sheets and
summaries. Portrait and Name choices save independently through the existing revision
mechanism, preserving the other identity fields.
