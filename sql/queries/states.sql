-- name: CreateState :one
INSERT INTO states (id, title, created_at, updated_at, board_id)
VALUES (
    gen_random_uuid(),
    $1,
    NOW(),
    NOW(),
    $2
) RETURNING *;
