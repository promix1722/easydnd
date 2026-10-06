# Known caveats

Limits that are known and, for now, deliberate. Each says what breaks, why,
and what would lift it.

## The AI Wizard does not scale horizontally

**Run one API process.** A second one behind the same address breaks the
wizard, and nothing in the service detects or prevents it.

Less of an import lives in one process than used to. The session --
transcript, status, attachments -- is in PostgreSQL, a turn is claimed under
a lease by whichever process is free, and the page's [poll](polling.md) is
answered from the database by whichever process it reaches
([agent.md](agent.md#session-lifetime)). What is still in the memory of the
process that started the import:

- **the character it is building** -- with every other character, folder and
  game (see the [README](../README.md));
- **the private packs it wrote** for content the rules lack.

So with two processes the chat is readable from either, and its turn can be
claimed by either, but only the one that made the character can find it. A
turn claimed by the other fails the session at its first tool call, and the
chat is missing from the other's list of chats to reopen. The same is true of
one process across a restart: the chat is kept and its turn is claimed again,
and it fails because the character is gone.

That failure is deliberate rather than merely what happens. A character id is
a counter that starts again with the process, so the id a stored chat holds
will name some *other* character after a restart; the assistant checks that
the character is the one its own chat created before it writes to it.

**What would lift it:** characters, folders and private packs in the shared
store. Nothing in the wizard would then need to change.
