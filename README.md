# easydnd

D&D 5e character creation, level-up and battle tracker. Go HTTP API plus a
responsive React client, deployed to [easydnd.org](https://easydnd.org) via
GitHub Actions. Targets the **2014 rules** and **SRD 5.1**.

Status: **character creation works end to end.** The architecture, deploy path,
entity model, SRD compendium, sign-in, the rules math and the browser client
are built and tested. **Level-up is not**: the rules for it exist, but taking a
level does not stick, so the client does not offer it -- see
[docs/web.md](docs/web.md#level-up-is-not-offered). The battle tracker is not
built either.

An account is reached by **a passkey or a Google account**, either or both --
there is no password, no reset link and nothing to fill in: one button either
signs you in with the passkey you already have or creates an account around the
one your device makes, saved as `easydnd` in your credential manager so you can
find it there later. Connecting the second method is the only recovery there
is; the passkey an account is created with is the only one it ever has. Google is optional configuration; without it the app
is passkeys only. Accounts, their passkeys and their linked accounts are
stored in PostgreSQL, with the schema migrating itself at startup, so a restart
no longer costs anybody their account. There is also a **guest session**: one
click, no account, nothing stored beyond the group roster a guest asks to be
named in, and nothing that survives it. **Characters and the folders they are
filed in still live in memory** and are wiped by a restart. See
[Authentication](docs/backend.md#authentication) and
[Folders](docs/backend.md#folders).

**Groups** are the second main section: a table of people with three ranks --
owner, DM, player -- who invite each other with a link that works for 24 hours.
They live in PostgreSQL and survive a restart. A group holds people, not
characters; attaching characters to one is a later change, because a character
id does not outlive the process. See
[Ownership, and membership](docs/backend.md#ownership-and-membership).

A **group** and a **folder** are different things and the words are not
interchangeable. A group is people, shared, with ranks. A folder is one
account's private shelf for its own characters, shared with nobody.

## Documentation

| Doc | Covers |
| --- | --- |
| [docs/dnd.md](docs/dnd.md) | The game model: catalogue entities, the event-sourced character, and the SRD terminology the code follows |
| [docs/backend.md](docs/backend.md) | The Go service: layout, layer rules, configuration, deployment |
| [docs/web.md](docs/web.md) | The browser client: layout, layer rules, how it ships |
| [docs/licensing.md](docs/licensing.md) | MIT for the project's own code, and the SRD 5.1 attribution the data carries |

