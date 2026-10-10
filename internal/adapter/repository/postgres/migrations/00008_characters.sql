-- +goose Up

-- Characters, the folders they are filed in, the group pools they are shared
-- into and the games run from them. Until this migration all four lived in
-- the memory of one process; 00003_groups.sql explains why nothing could
-- refer to a character then. That reason is gone: a character id is now drawn
-- from a sequence here, so it never names a different character after a
-- restart.
--
-- Three things deliberately have no foreign key:
--
--   * owner_id, on folders and characters, because a guest may own either and
--     a guest is a users row only once they ask to be named in a group -- the
--     same reason agent_sessions.owner_id has none.
--   * characters.folder_id, because the port says the store does not verify
--     the folder; that is the usecase's authorization question, and the
--     in-memory adapter cannot verify it either. Deleting a folder deletes its
--     characters first, in the usecase.
--   * shared_characters.character_id and the character ids inside a roster,
--     because the usecases already order their cascades (unshare, then
--     delete) and every read skips an id that is gone. A key here would make
--     the SQL adapter refuse a share the in-memory one accepts, and the two
--     are held to one contract.
--
-- group_id does reference groups, as group_rule_packs does: a group is only
-- ever durable, and when it goes its pool and its games go with it.
--
-- A log is json rather than jsonb for the reason agent_sessions.document is:
-- it is never queried, and jsonb refuses the \u0000 a transcribed source can
-- carry. The same goes for a roster, which carries a copy of a monster's
-- sheet.

CREATE SEQUENCE folders_id_seq;
CREATE TABLE folders (
    id          text        PRIMARY KEY
                            DEFAULT 'fld_' || lpad(nextval('folders_id_seq')::text, 6, '0'),
    owner_id    text        NOT NULL,
    name        text        NOT NULL,
    is_default  boolean     NOT NULL DEFAULT false,
    position    integer     NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX folders_owner_idx ON folders (owner_id);
-- One default per owner, held by the database so that two first requests
-- from a new account cannot both make one.
CREATE UNIQUE INDEX folders_one_default_idx ON folders (owner_id) WHERE is_default;

CREATE SEQUENCE characters_id_seq;
CREATE TABLE characters (
    id          text        PRIMARY KEY
                            DEFAULT 'chr_' || lpad(nextval('characters_id_seq')::text, 6, '0'),
    owner_id    text        NOT NULL,
    folder_id   text        NOT NULL,
    revision    integer     NOT NULL DEFAULT 0,
    log         json        NOT NULL,
    checkpoints json,
    commands    json,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX characters_owner_idx ON characters (owner_id);

CREATE TABLE shared_characters (
    group_id     text        NOT NULL
                             CONSTRAINT shared_characters_group_id_fkey
                             REFERENCES groups (id) ON DELETE CASCADE,
    character_id text        NOT NULL,
    owner_id     text        NOT NULL,
    shared_at    timestamptz NOT NULL,
    CONSTRAINT shared_characters_pkey PRIMARY KEY (group_id, character_id)
);
CREATE INDEX shared_characters_character_idx ON shared_characters (character_id);

CREATE TABLE games (
    id         text        PRIMARY KEY,
    group_id   text        NOT NULL
                           CONSTRAINT games_group_id_fkey
                           REFERENCES groups (id) ON DELETE CASCADE,
    name       text        NOT NULL,
    created_by text        NOT NULL,
    created_at timestamptz NOT NULL,
    roster     json        NOT NULL DEFAULT '[]'
);
CREATE INDEX games_group_idx ON games (group_id);

-- The rule packs an AI Wizard import compiles for content the rules lack.
-- They are pinned by the character's rules lock and never listed, so they
-- are not rule_packs rows; see docs/agent.md.
CREATE TABLE private_releases (
    id      text  NOT NULL,
    version text  NOT NULL,
    digest  text  NOT NULL,
    data    bytea NOT NULL,
    PRIMARY KEY (id, version)
);

-- +goose Down
DROP TABLE private_releases;
DROP TABLE games;
DROP TABLE shared_characters;
DROP TABLE characters;
DROP SEQUENCE characters_id_seq;
DROP TABLE folders;
DROP SEQUENCE folders_id_seq;
