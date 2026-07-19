-- +goose Up
ALTER TABLE boards ADD state_positions UUID[] DEFAULT '{}';

-- +goose Down
ALTER TABLE boards DROP state_positions position;