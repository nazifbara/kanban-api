-- +goose Up
ALTER TABLE boards ADD COLUMN creator_id UUID NOT NULL;
ALTER TABLE boards ADD CONSTRAINT fk_users FOREIGN KEY(creator_id) REFERENCES identities(id) ON DELETE CASCADE;

-- +goose Down
ALTER TABLE boards DROP COLUMN creator_id;
