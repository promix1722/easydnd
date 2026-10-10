-- +goose Up
-- A restricted disk pack a superadmin has handed to one account. The pack has
-- no row in rule_packs to point at (see 00010), so pack_id is bare text.
CREATE TABLE user_rule_packs (
 user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 pack_id text NOT NULL,
 PRIMARY KEY(user_id, pack_id)
);

-- +goose Down
DROP TABLE user_rule_packs;
