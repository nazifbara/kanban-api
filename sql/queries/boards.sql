-- name: CreateBoard :one
INSERT INTO boards (id, name, created_at, updated_at)
VALUES (
    gen_random_uuid(),
    $1,
    NOw(),
    NOW()
)
RETURNING *;