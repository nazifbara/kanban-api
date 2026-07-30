-- +goose Up
CREATE TABLE tasks (
    id UUID PRIMARY KEY,
    board_id UUID NOT NULL,
    column_id UUID NOT NULL,
    title VARCHAR(300) NOT NULL,
    description TEXT,
    position INT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,

    CONSTRAINT fk_tasks_board FOREIGN KEY (board_id) REFERENCES boards(id) ON DELETE CASCADE,
    CONSTRAINT fk_tasks_column FOREIGN KEY (column_id) REFERENCES columns(id) ON DELETE CASCADE
);
CREATE INDEX idx_tasks_board_column ON tasks (board_id, column_id);
CREATE INDEX idx_tasks_board ON tasks (board_id);

-- +goose Down
DROP TABLE tasks;