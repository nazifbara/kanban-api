-- +goose Up
ALTER TABLE tasks ADD COLUMN creator_id UUID NOT NULL;
ALTER TABLE tasks ADD CONSTRAINT fk_users FOREIGN KEY(creator_id) REFERENCES identities(id) ON DELETE CASCADE;

-- +goose Down
ALTER TABLE tasks DROP COLUMN creator_id;
