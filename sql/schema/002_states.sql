-- +goose Up
CREATE TABLE states (
    id UUID PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    board_id UUID NOT NULL,
    CONSTRAINT fk_board FOREIGN KEY (board_id) REFERENCES boards(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE states;