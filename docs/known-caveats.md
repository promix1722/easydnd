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

## The AI Wizard scales horizontally now

This used to be the first caveat: the character an import built and the packs
it compiled lived in one process's memory, so a second API process could read
the chat and claim its turn but not find its character. Both are in PostgreSQL
now (`characters`, `private_releases`), and nothing in the wizard needed to
change. It is kept here so that the reasoning is not lost: a turn claimed by
any process finds the character by id, loads the private release by its lock
from the store on a cache miss, and writes back under the character's
revision.
