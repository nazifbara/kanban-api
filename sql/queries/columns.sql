-- name: DeleteColumn :exec
DELETE FROM columns WHERE id = $1;

-- name: CreateColumn :one
INSERT INTO columns (id, title, created_at, updated_at, board_id)
VALUES (
    gen_random_uuid(),
    $1,
    NOW(),
    NOW(),
    $2
) RETURNING *;

-- name: GetColumns :many
SELECT * from columns WHERE board_id = $1 ORDER BY created_at DESC;