-- +goose Up
-- +goose StatementBegin

-- A play group: the people at one table.
--
-- Characters are deliberately not attached to one and no column here
-- anticipates them. Characters still live in memory behind process-local ids
-- that do not survive a restart, so a foreign key to one would be dangling by
-- morning -- and a schema written against an unfinished feature is a migration
-- nobody can revise later.
--
-- The id is minted by the usecase rather than by the database, and is opaque
-- for the same reason an account id is: it travels in invite links people
-- paste to each other, so a sequence there would say how many groups exist and
-- let a stranger address the next one.
CREATE TABLE groups (
    id         text        NOT NULL,
    name       text        NOT NULL,
    created_by text        NOT NULL,
    created_at timestamptz NOT NULL,

    CONSTRAINT groups_pkey PRIMARY KEY (id),
    CONSTRAINT groups_name_len CHECK (char_length(name) BETWEEN 1 AND 64),

    -- created_by is history, not authority: ownership lives in group_members
    -- and moves when it is transferred. Nothing may consult this column to
    -- decide what somebody is allowed to do.
    CONSTRAINT groups_created_by_fkey
        FOREIGN KEY (created_by) REFERENCES users (id) ON DELETE CASCADE
);

-- One person's seat at one table.
--
-- user_id is a real foreign key into users, guests included. That is what the
-- usecase's ensureStored exists to make true: a guest is otherwise minted
-- inside a session token and written nowhere, and a roster other people read
-- cannot be built out of ids with nothing behind them. Storing the name here
-- instead would have been the alternative, and it would have been a second
-- copy of a fact that is already recorded once.
--
-- Anonymity is likewise not a column. It is read off the `anon:` prefix on the
-- id -- see user.AnonymousIDPrefix -- because a stored copy of a derivable
-- fact is one more thing that can disagree with the id beside it.
--
-- The primary key is (group_id, user_id) rather than a surrogate: it makes
-- "somebody is in a group at most once" the database's rule rather than a Go
-- map's, and its leading column gives the roster read its index for free.
CREATE TABLE group_members (
    group_id  text        NOT NULL,
    user_id   text        NOT NULL,
    role      text        NOT NULL,
    joined_at timestamptz NOT NULL,

    CONSTRAINT group_members_pkey PRIMARY KEY (group_id, user_id),
    CONSTRAINT group_members_role_check CHECK (role IN ('owner', 'dm', 'player')),
    CONSTRAINT group_members_group_id_fkey
        FOREIGN KEY (group_id) REFERENCES groups (id) ON DELETE CASCADE,

    -- ON DELETE CASCADE is unreachable today: there is no path that deletes an
    -- account. Whoever writes one should read this first -- deleting an
    -- account silently removes their owner row and strands every group they
    -- ran, which is a decision that belongs to that change and not to this one.
    CONSTRAINT group_members_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

-- EXACTLY ONE OWNER PER GROUP, enforced here and not only in Go.
--
-- A partial unique index rather than a CHECK, because the rule spans rows. The
-- usecase refuses a second owner too, but the usecase is not what a second
-- process racing it obeys.
--
-- This forces the ORDER of a transfer, and getting it backwards is a bug that
-- only ever appears against real Postgres. A unique index is checked as each
-- statement runs and cannot be deferred, so TransferOwnership must DEMOTE the
-- outgoing owner first and PROMOTE the incoming one second. The intermediate
-- state is then zero owners, which this index allows; the other order is two,
-- which it rejects.
CREATE UNIQUE INDEX group_members_one_owner_idx
    ON group_members (group_id) WHERE role = 'owner';

-- ListFor reads every membership of one person. group_id leads the primary
-- key, so that direction is already indexed and this one is not: without it
-- the group list is a sequential scan of every membership in the system.
CREATE INDEX group_members_user_id_idx ON group_members (user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE group_members;
DROP TABLE groups;
-- +goose StatementEnd
