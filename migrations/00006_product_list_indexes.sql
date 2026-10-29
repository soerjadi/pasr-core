-- +goose Up
-- listProducts pages on id DESC (keyset). These partial indexes let the planner walk the filter
-- and the order together instead of sorting: by category, by seller, and the featured shelf.
CREATE INDEX products_category_id_idx ON products (category_id, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX products_store_id_idx ON products (store_id, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX products_featured_id_idx ON products (id DESC) WHERE featured AND deleted_at IS NULL;

-- +goose Down
DROP INDEX products_featured_id_idx;
DROP INDEX products_store_id_idx;
DROP INDEX products_category_id_idx;
