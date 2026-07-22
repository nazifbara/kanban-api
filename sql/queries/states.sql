-- name: DeleteState :exec
DELETE FROM states WHERE id = $1;

-- name: CreateState :one
INSERT INTO states (id, title, created_at, updated_at, board_id)
VALUES (
    gen_random_uuid(),
    $1,
    NOW(),
    NOW(),
    $2
) RETURNING *;

-- name: GetStates :many
SELECT * from states WHERE board_id = $1 ORDER BY created_at DESC;