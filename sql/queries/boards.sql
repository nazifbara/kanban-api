-- name: UpdateBoard :one
UPDATE boards SET name = $1 WHERE id = $2 RETURNING *;

-- name: DeleteBoard :one
DELETE FROM boards WHERE id = $1 RETURNING *;

-- name: GetBoard :one
SELECT * FROM boards WHERE id = $1 FOR SHARE;

-- name: GetUserBoards :many
SELECT * FROM boards WHERE creator_id = $1 ORDER BY created_at DESC;

-- name: CreateBoard :one
INSERT INTO boards (id, creator_id, name, description, created_at, updated_at)
VALUES (
    gen_random_uuid(),
    $1,
    $2,
    $3,
    NOw(),
    NOW()
)
RETURNING *;
