# easydnd

D&D 5e character creation, level-up and battle tracker. Go HTTP API plus a
responsive React client, deployed to [easydnd.org](https://easydnd.org) via
GitHub Actions. Targets the **2014 rules** and **SRD 5.1**.

Status: **character creation and level-up work end to end.** The architecture,
deploy path, entity model, SRD compendium, sign-in, the rules math and the
browser client are built and tested. The battle tracker is not.

An account is reached by **a passkey or a Google account**, either or both --
there is no password, no reset link and nothing to fill in: one button either
signs you in with the passkey you already have or creates an account around the
one your device makes, under a name the server picks. Connecting the second
method is the only recovery there is; the passkey an account is created with is
the only one it ever has. Google is optional configuration; without it the app
is passkeys only. Accounts, their passkeys and their linked accounts are
stored in PostgreSQL, with the schema migrating itself at startup, so a restart
no longer costs anybody their account. There is also a **guest session**: one
click, no account, nothing stored, and nothing that survives it. **Characters
still live in memory** and are wiped by a restart. See
[Authentication](docs/backend.md#authentication).

## Documentation

| Doc | Covers |
| --- | --- |
| [docs/dnd.md](docs/dnd.md) | The game model: catalogue entities, the event-sourced character, and the SRD terminology the code follows |
| [docs/backend.md](docs/backend.md) | The Go service: layout, layer rules, configuration, deployment |
| [docs/web.md](docs/web.md) | The browser client: layout, layer rules, how it ships |
| [docs/licensing.md](docs/licensing.md) | MIT for the project's own code, and the SRD 5.1 attribution the data carries |

