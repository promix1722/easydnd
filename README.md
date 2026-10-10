# easydnd

A D&D 5e character builder and table tracker, live at
[easydnd.org](https://easydnd.org). It targets the **2014 rules** on SRD 5.1,
extended with the mechanics and names of the other 2014 books and none of
their text.

A Go HTTP API and a React client, with everything stored in PostgreSQL.

## What it does

- **Characters.** Create one, answer the choices each level opens, level up
  from the sheet. A character is an event log, replayed into a sheet.
- **AI Wizard.** Upload a character sheet and a model builds the character
  from it, using tools the server supplies.
- **Groups.** A table of people with three ranks -- owner, DM, player -- who
  join by an invitation link.
- **Games.** One sitting at a group's table: a roster of shared characters and
  NPCs, with hit points, initiative, rests, items and coins tracked per game.
- **Rule packs.** The compendium is a versioned pack. Homebrew packs are
  authored in the browser, shared with a group, and imported or exported as
  JSON.
- **Accounts.** A passkey or a Google account, or a guest session; no
  passwords.

Three words are not interchangeable. A **group** is people. A **game** is one
sitting at a group's table, never called a *session*, which here means being
signed in. A **folder** is one account's private shelf for its own characters.

## Run it

```sh
make dev      # Postgres, the API and the web client, on this worktree's ports
make verify   # everything CI checks; run it before every commit
```

`make dev` seeds three accounts -- **master**, **player1**, **player2** -- with
a shared group, two games and a few characters. Open `/login` and pick one.
Secrets and the AI Wizard's key come from `~/config/easydnd/dev.env`; see
`easydnd.example.env`.

Deploying is a tag: `git tag -a vX.Y.Z && git push origin vX.Y.Z`.

## Documentation

| Doc | Covers |
| --- | --- |
| [docs/backend.md](docs/backend.md) | The Go service: layout, layer rules, API, configuration, deployment |
| [docs/web.md](docs/web.md) | The browser client: layout, layer rules, design decisions, how it ships |
| [docs/dnd.md](docs/dnd.md) | The game model: catalogue, the event-sourced character, SRD terminology |
| [docs/packs.md](docs/packs.md) | Rule packs: format, versions and locks, resources, homebrew authoring |
| [docs/agent.md](docs/agent.md) | The AI Wizard: tools, chat workspace, private content, resumability |
| [docs/polling.md](docs/polling.md) | How the AI Wizard page follows a running import |
| [docs/known-caveats.md](docs/known-caveats.md) | Limits that are known and deliberate |
| [docs/seo.md](docs/seo.md) | Search and answer-engine discovery |
| [docs/licensing.md](docs/licensing.md) | MIT for the code, and the SRD 5.1 attribution the data carries |

`CLAUDE.md` holds the working rules for changing this repository.
