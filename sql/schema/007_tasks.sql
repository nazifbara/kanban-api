-- +goose Up
CREATE TABLE tasks (
    id UUID PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    column_id UUID NOT NULL,
    CONSTRAINT fk_column FOREIGN KEY (column_id) REFERENCES columns(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE tasks;