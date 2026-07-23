-- name: GetColumnById :one
SELECT * FROM columns WHERE id = $1;

-- name: UpdateColumn :one
UPDATE columns
SET position = $1, title = $2, description = $3, updated_at = NOW() 
WHERE id = $4
RETURNING *;

-- name: UpdateColumnPosition :exec
UPDATE columns SET position = $2 WHERE id = $1;

-- name: DeleteColumn :exec
DELETE FROM columns WHERE id = $1;

-- name: CreateColumn :one
INSERT INTO columns (id, title, created_at, updated_at, board_id, position)
VALUES (
    gen_random_uuid(),
    $1,
    NOW(),
    NOW(),
    $2,
    $3
) RETURNING *;

-- name: GetColumns :many
SELECT * from columns WHERE board_id = $1 ORDER BY position ASC;