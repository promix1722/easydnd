# Known caveats

Limits that are known and, for now, deliberate. Each says what breaks, why,
and what would lift it.

## The AI Wizard does not scale horizontally

**Run one API process.** A second one behind the same address breaks the
wizard, and nothing in the service detects or prevents it.

Everything about an import lives in the memory of the process that started it
([agent.md](agent.md#session-lifetime)):

- the session itself -- transcript, status, revision;
- the files that were attached to it;
- the private packs it wrote;
- the worker goroutine running its current turn;
- the wake-up that answers a [long poll](long-polling.md) when the session
  changes.

With two processes, a request that lands on the one that does not hold the
session is answered `404 agent.notFound`. The page takes a `404` to mean the
chat was discarded and stops following it; a message sent there is rejected
the same way. Sticky routing would not be enough either: it would have to
hold across a deploy, and a restart loses every session regardless.

It is not only the wizard. Characters, folders and games are in memory too
(see the [README](../README.md)), so a second process would also show each
visitor a different set of characters depending on where a request landed.
The wizard is called out here because it is the part with work in flight: an
import is a running job, not just stored data.

**What would lift it:** sessions and attachments in a shared store, turns
claimed by one worker at a time across processes, and a cross-process
notification in place of the in-memory wake-up -- PostgreSQL `LISTEN`/`NOTIFY`
would do, since accounts already live there. None of it is built; one process
serves the load there is.
