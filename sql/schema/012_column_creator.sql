-- +goose Up
ALTER TABLE columns ADD COLUMN creator_id UUID NOT NULL;
ALTER TABLE columns ADD CONSTRAINT fk_users FOREIGN KEY(creator_id) REFERENCES identities(id) ON DELETE CASCADE;

-- +goose Down
ALTER TABLE columns DROP COLUMN creator_id;
