-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN salt text NOT NULL;
ALTER TABLE users ADD COLUMN password_hash text DEFAULT "unset" NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN password_has;
ALTER TABLE users DROP COLUMN salt;
-- +goose StatementEnd
