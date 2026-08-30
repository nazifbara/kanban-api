-- name: CreateUser :one
INSERT INTO users (id, first_name, last_name, email, created_at, updated_at)
VALUES($1, $2, $3, $4, NOW(), NOW()) RETURNING *; 
