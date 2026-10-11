-- +goose Up
-- An account can now be deleted (the superadmin's DELETE /v1/admin/players).
-- groups.created_by is history, not authority, and its ON DELETE CASCADE would
-- take a group away from whoever owns it today because the person who opened
-- it years ago is gone. The column stays; it simply stops pointing at a row,
-- the way characters.owner_id never did.
ALTER TABLE groups DROP CONSTRAINT groups_created_by_fkey;

-- +goose Down
ALTER TABLE groups ADD CONSTRAINT groups_created_by_fkey
    FOREIGN KEY (created_by) REFERENCES users (id) ON DELETE CASCADE NOT VALID;
