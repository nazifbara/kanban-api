-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY,
    CONSTRAINT fk_identities FOREIGN KEY (id) REFERENCES identities(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL UNIQUE,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

-- +goose Down
DROP TABLE users;
