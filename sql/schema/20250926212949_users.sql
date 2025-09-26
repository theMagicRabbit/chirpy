-- +goose Up
ALTER TABLE users
ADD COLUMN hashed_password text NOT NULL,
ALTER COLUMN hashed_password SET DEFAULT 'unset';

-- +goose Down
ALTER TABLE users DROP COLUMN hashed_password;

