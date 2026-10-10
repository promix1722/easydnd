# Known caveats

Limits that are known and, for now, deliberate. Each says what breaks, why,
and what would lift it.

## A guest's characters are never deleted

A guest session is one click and no account, and it owns the characters it
makes like any account does: they are rows in `characters`, with the guest's
id as the owner. The session expires; the rows do not. Nothing can reach them
again -- a guest id is never issued twice -- and nothing sweeps them, so the
table grows by whatever guests make and leave behind.

That is a size, not a correctness problem: no read lists them, no join reaches
them, and the one index on `owner_id` is what every query uses. It is left as
it is because the honest fix is a question the product has not answered yet:
how long a guest's work should outlive the guest.

**What would lift it:** a sweep like the wizard's, deleting characters whose
owner is a guest id older than the guest session's lifetime, and the folders
with them. `ponytail:` in `internal/app/app.go` marks the spot.

## Limits are counted, not reserved

The numbers in [Limits](backend.md#limits) stop a list growing by accident.
Three things they do not stop:

**A guest starts again at zero.** Limits are per owner, a guest session is one
click, and a new one is a new owner. That includes `WizardRunsPerDay`, the one
limit that costs money to get around. *What would lift it:* a rate limit by
address on `POST /auth/anonymous`, which this service does not have, or an
account requirement for the wizard.

**Discarding a wizard chat gives its run back.** The 24-hour count is read
from `agent_sessions`, and Discard deletes the row. A chat opened more than a
day before its first message is not counted either, since the row's time is
when it was opened. *What would lift it:* a table of `(owner, started_at)`
written at the first message; `ponytail:` on `Agent.room` marks the spot.

**Two creates at once can both pass.** Every limit but a game's roster is a
count followed by an insert, so requests racing at the limit can leave a list
a few over it. The roster is exact because it is checked under the row lock
`MutateEntries` already takes. *What would lift it:* the guarded insert the
wizard's `max_sessions` uses (`postgres/agent.go`, `Create`). Counting an
owner's packs also reads every pack; a count over `rule_packs_owner` would
replace it.

## Choices the builder does not ask

Every pick the 2014 rules leave to the player is meant to be a prompt (see
[Picks a feature owns](dnd.md#picks-a-feature-owns)). These are not, yet:

- **Tasha's optional class features** -- Deft Explorer, Favored Foe, the
  Versatility features. There are no rows, and they *replace* a feature, which
  nothing in the grammar says. *What would lift it:* a "replaces" field on a
  feature and a choice between the pair.
- **A Beast Master's companion.** There is no monster data, so both
  companion features are granted and neither is chosen.
- **Kensei weapons.** The tool is asked; the weapons are not. A kensei weapon
  is not a proficiency -- a monk may name a weapon they are already proficient
  with -- so it needs its own option rows.
- **A favored enemy of "two races of humanoid".** Only the type is offered.
- **Replicate Magic Item** does not ask which item, and **Elemental Adept**
  cannot be taken a second time for a second damage type.
- **Spell Sniper** offers every cantrip on the chosen list, not only those
  with an attack roll.

Two things restrict a choice and are not enforced, so the builder offers
too much without computing anything wrong: **feat prerequisites** (a feat is
offered to everybody; "Intelligence or Wisdom 13" is stored as two conditions
that would both have to hold, and `feature:spellcasting` matches no class's
own spellcasting row, so enforcing the data as written would be wrong more
often than not) and the **Eldritch Knight and Arcane Trickster school
limits**.

Two things a chosen option should bring and does not:

- **Martial Adept's superiority die** and **Metamagic Adept's two sorcery
  points.** The pools belong to the Battle Master and the sorcerer, so a
  character with only the feat knows the maneuver and has nothing to spend.
- **A subclass feature's fixed proficiencies** -- a Life cleric's heavy armor,
  a Mastermind's disguise kit. The row is prose; only the part the player
  chooses is granted. *What would lift it:* one grant rule per feature, in the
  shape `telekinetic-spell-mage-hand` already has.

There is no step for **replacing** a maneuver, invocation or infusion on
level-up, on purpose: prerequisites are read at the character's current level
and every earlier answer can be edited, so the replacement is an edit.

A question a rule poses for a *picked* feature -- Superior Technique's
maneuver, a favored enemy's language -- is drawn above first level on the
class tab, because the option's row carries no level of its own.

## The AI Wizard scales horizontally now

This used to be the first caveat: the character an import built and the packs
it compiled lived in one process's memory, so a second API process could read
the chat and claim its turn but not find its character. Both are in PostgreSQL
now (`characters`, `private_releases`), and nothing in the wizard needed to
change. It is kept here so that the reasoning is not lost: a turn claimed by
any process finds the character by id, loads the private release by its lock
from the store on a cache miss, and writes back under the character's
revision.
