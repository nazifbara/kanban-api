-- name: CreateTask :one
INSERT INTO tasks (id, board_id, column_id, title, description, position, created_at, updated_at)
VALUES (
    gen_random_uuid(),
    $1,
    $2,
    $3,
    $4,
    $5,
    NOW(),
    NOW()
) 
RETURNING *;

-- name: GetColumnTasks :many
SELECT * FROM tasks WHERE column_id = $1 AND board_id = $2;

-- name: UpdateTaskPosition :exec
UPDATE tasks SET position = $2 WHERE id = $1;