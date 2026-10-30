-- +goose Up
-- The seller's order list pages a store's seller-orders on id DESC (keyset) within a tab.
-- One status (the NEW tab) walks the status index in order; the large DONE tab can walk the store
-- index and filter, so neither has to sort the store's whole history.
CREATE INDEX seller_orders_store_status_id_idx ON seller_orders (store_id, status, id DESC);
CREATE INDEX seller_orders_store_id_idx ON seller_orders (store_id, id DESC);

-- +goose Down
DROP INDEX seller_orders_store_id_idx;
DROP INDEX seller_orders_store_status_id_idx;
