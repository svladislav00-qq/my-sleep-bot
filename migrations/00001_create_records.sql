-- +goose Up
CREATE TABLE records (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL,
    fell_sleep TIMESTAMPTZ NOT NULL,
    woke_up TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE records;