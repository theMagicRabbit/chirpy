-- +goose Up
CREATE TABLE chirps (
    id uuid UNIQUE NOT NULL,
    created_at timestamp NOT NULL,
    updated_at timestamp NOT NULL,
    body text NOT NULL,
    user_id uuid NOT NULL,
    CONSTRAINT pk_chirps PRIMARY KEY (id),
    CONSTRAINT fk_chirps_user_id FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE chirps;

