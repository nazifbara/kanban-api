-- name: GetBoardTasks :many
SELECT * FROM tasks WHERE board_id = $1 ORDER BY created_at DESC;

-- name: DeleteTask :exec
DELETE FROM tasks WHERE id = $1; 

-- name: ShiftTasksBetween :exec
UPDATE tasks
SET position = position + sqlc.arg(delta)
WHERE position > sqlc.arg(after) AND position < sqlc.arg(before) AND column_id = $1;

-- name: ShiftTasksFrom :exec
UPDATE tasks
SET position = position + sqlc.arg(delta)
WHERE position >= sqlc.arg(start) AND column_id = $1;

-- name: GetTask :one
SELECT * FROM tasks WHERE id = $1;

-- name: GetTaskForShare :one
SELECT * FROM tasks WHERE id = $1 FOR SHARE;

-- name: GetTaskForUpdate :one
SELECT * FROM tasks WHERE id = $1 FOR UPDATE;

-- name: CountTasks :one
SELECT COUNT(*) from tasks WHERE column_id = $1;

-- name: UpdateTask :one 
UPDATE tasks
SET
    column_id = COALESCE(sqlc.narg(column_id), column_id),
    title = COALESCE(sqlc.narg(title), title),
    description = COALESCE(sqlc.narg(description), description),
    position = COALESCE(sqlc.narg(position), position),
    updated_at = NOW()
where id = $1
RETURNING *;

-- name: CreateTask :one
INSERT INTO tasks (id, creator_id, board_id, column_id, title, description, position, created_at, updated_at)
VALUES (
    gen_random_uuid(),
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    NOW(),
    NOW()
) 
RETURNING *;

-- name: GetColumnTasksForUpdate :many
SELECT * FROM tasks WHERE column_id = $1 ORDER BY position ASC FOR UPDATE;

-- name: GetColumnTasks :many
SELECT * FROM tasks WHERE column_id = $1 ORDER BY position ASC;

-- name: UpdateTaskPosition :exec
UPDATE tasks SET position = $2 WHERE id = $1;
