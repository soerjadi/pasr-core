-- +goose Up
CREATE TABLE categories (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT    NOT NULL,
    icon_key   TEXT    NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0
);

-- +goose Down
DROP TABLE categories;
