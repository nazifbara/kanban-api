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