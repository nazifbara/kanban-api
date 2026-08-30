-- name: CreateIdentity :one
INSERT INTO identities (id, password_hash, email, created_at, updated_at)
VALUES(gen_random_uuid(), $1, $2, NOW(), NOW()) RETURNING *; 
