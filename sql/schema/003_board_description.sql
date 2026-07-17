-- +goose Up
ALTER TABLE boards
ADD description VARCHAR(250);

-- +goose Down
ALTER TABLE boards
DROP COLUMN description;
