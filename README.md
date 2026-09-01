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
They live in PostgreSQL and survive a restart.

A group is no longer only people. Any member may **share** a character with it,
and that grants a read: everybody at the table can open the sheet, and only its
owner can ever change it.

**Games** are the third main section, beside Characters and Groups. A game is
one sitting run by a DM, played at one group's table, with a roster of the
characters that group has shared. They are listed together across every table
you sit at rather than being reached through a group -- the group is a fact
about a game, not the way in to one. Note the split, which is surprising: the
group and its members are in PostgreSQL, while the characters shared with it and
the games run from them are **in memory and die with the characters they name**,
because a character id does not outlive the process. See
[Ownership, and membership](docs/backend.md#ownership-and-membership).

A **group**, a **game** and a **folder** are different things and the words are
not interchangeable. A group is people, shared, with ranks. A game is one
sitting at that group's table, with the characters a DM seats at it -- never
called a *session*, which here means being signed in. A folder is one account's
private shelf for its own characters, shared with nobody.

## Documentation

| Doc | Covers |
| --- | --- |
| [docs/dnd.md](docs/dnd.md) | The game model: catalogue entities, the event-sourced character, and the SRD terminology the code follows |
| [docs/backend.md](docs/backend.md) | The Go service: layout, layer rules, configuration, deployment |
| [docs/web.md](docs/web.md) | The browser client: layout, layer rules, how it ships |
| [docs/seo.md](docs/seo.md) | Search-engine and answer-engine discovery, submission, and monitoring |
| [docs/licensing.md](docs/licensing.md) | MIT for the project's own code, and the SRD 5.1 attribution the data carries |
