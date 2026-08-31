-- name: ShiftColumnsBetween :exec
UPDATE columns
SET position = position + sqlc.arg(delta)
WHERE position > sqlc.arg(after) AND position < sqlc.arg(before) AND board_id = $1;

-- name: ShiftColumnsFrom :exec
UPDATE columns
SET position = position + sqlc.arg(delta)
WHERE position >= sqlc.arg(start) AND board_id = $1;

-- name: CountColumns :one
SELECT COUNT(*) from columns WHERE board_id = $1;

-- name: GetColumn :one
SELECT * FROM columns WHERE id = $1 FOR SHARE;

-- name: UpdateColumn :one
UPDATE columns
SET 
    position = COALESCE(sqlc.narg(position), position),
    title = COALESCE(sqlc.narg(title), title),
    description = COALESCE(sqlc.narg(description), description),
    updated_at = NOW() 
WHERE id = $1
RETURNING *;

-- name: UpdateColumnPosition :exec
UPDATE columns SET position = $2 WHERE id = $1;

-- name: DeleteColumn :exec
DELETE FROM columns WHERE id = $1;

-- name: CreateColumn :one
INSERT INTO columns (id, title, created_at, updated_at, board_id, position, creator_id)
VALUES (
    gen_random_uuid(),
    $1,
    NOW(),
    NOW(),
    $2,
    $3,
    $4
) RETURNING *;

-- name: GetColumns :many
SELECT * from columns WHERE board_id = $1 ORDER BY position ASC;
