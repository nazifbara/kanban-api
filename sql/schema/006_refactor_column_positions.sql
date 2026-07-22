-- +goose Up
ALTER TABLE boards DROP COLUMN column_positions;
ALTER TABLE columns ADD position INT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE boards ADD column_positions UUID[] DEFAULT '{}';
ALTER TABLE columns DROP COLUMN position;
