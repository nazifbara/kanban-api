-- +goose Up
ALTER TABLE states RENAME TO columns;
ALTER TABLE boards RENAME COLUMN state_positions TO column_positions;

-- +goose Down
ALTER TABLE columns RENAME TO states;
ALTER TABLE boards RENAME COLUMN column_positions TO state_positions;