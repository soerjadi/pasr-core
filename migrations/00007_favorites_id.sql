-- +goose Up
-- listFavorites pages newest first on a keyset cursor, so each favorite needs a stable, ordered id.
-- Existing rows get ids in creation order; the composite primary key (user_id, product_id) stays.
ALTER TABLE favorites ADD COLUMN id BIGINT;
CREATE SEQUENCE favorites_id_seq OWNED BY favorites.id;
UPDATE favorites f SET id = o.n
FROM (SELECT user_id, product_id, row_number() OVER (ORDER BY created_at, user_id, product_id) AS n FROM favorites) o
WHERE f.user_id = o.user_id AND f.product_id = o.product_id;
SELECT setval('favorites_id_seq', COALESCE((SELECT max(id) FROM favorites), 0) + 1, false);
ALTER TABLE favorites
    ALTER COLUMN id SET DEFAULT nextval('favorites_id_seq'),
    ALTER COLUMN id SET NOT NULL,
    ADD CONSTRAINT favorites_id_key UNIQUE (id);
-- The list filters by user and orders by id DESC.
CREATE INDEX favorites_user_id_idx ON favorites (user_id, id DESC);

-- +goose Down
DROP INDEX favorites_user_id_idx;
ALTER TABLE favorites DROP COLUMN id;
