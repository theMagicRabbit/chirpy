-- +goose Up
CREATE TABLE users (
    id uuid UNIQUE NOT NULL,
    created_at timestamp NOT NULL,
    updated_at timestamp NOT NULL,
    email text UNIQUE NOT NULL,
    CONSTRAINT pk_users PRIMARY KEY (id)
);

-- +goose Down
DROP TABLE users;
