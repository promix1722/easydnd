-- +goose Up
-- A restricted disk pack is shared with a group like any other, and it has no
-- row in rule_packs to point at. Nothing deletes a rule_packs row either, so
-- the key protected no share and only refused these.
ALTER TABLE group_rule_packs DROP CONSTRAINT group_rule_packs_pack_id_fkey;

-- +goose Down
ALTER TABLE group_rule_packs ADD CONSTRAINT group_rule_packs_pack_id_fkey
 FOREIGN KEY (pack_id) REFERENCES rule_packs(id) NOT VALID;
