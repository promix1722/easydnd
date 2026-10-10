-- +goose Up
CREATE TABLE rule_packs (
 id text PRIMARY KEY,
 owner_id text NOT NULL REFERENCES users(id),
 revision integer NOT NULL CHECK (revision > 0),
 document jsonb NOT NULL
);
CREATE INDEX rule_packs_owner ON rule_packs(owner_id);
CREATE TABLE group_rule_packs (
 group_id text NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
 pack_id text NOT NULL REFERENCES rule_packs(id),
 document jsonb NOT NULL,
 PRIMARY KEY(group_id, pack_id)
);
-- +goose Down
DROP TABLE group_rule_packs;
DROP TABLE rule_packs;
