-- +goose Up
CREATE TABLE boards (
    id UUID PRIMARY KEY,
    name TEXT
);

-- +goose Down
DROP TABLE boards;
