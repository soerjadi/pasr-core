-- +goose Up
-- The operator's order list pages every store's seller-orders on id DESC, optionally of one
-- status; this index serves the status filter without sorting all seller-orders of that status.
CREATE INDEX seller_orders_status_id_idx ON seller_orders (status, id DESC);

-- +goose Down
DROP INDEX seller_orders_status_id_idx;
