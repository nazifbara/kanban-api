-- name: DeleteBoard :one
DELETE FROM boards WHERE id = $1 RETURNING *;

-- name: GetBoardByID :one
SELECT * FROM boards WHERE id = $1;

-- name: GetAllBoards :many
SELECT * FROM boards ORDER BY created_at DESC;

-- name: CreateBoard :one
INSERT INTO boards (id, name, created_at, updated_at)
VALUES (
    gen_random_uuid(),
    $1,
    NOw(),
    NOW()
)
RETURNING *;